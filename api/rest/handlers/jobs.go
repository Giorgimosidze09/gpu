package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"gpu-orchestrator/core/models"
	"gpu-orchestrator/core/repository"
	"gpu-orchestrator/core/scheduler"
	"gpu-orchestrator/core/spec"

	"github.com/gorilla/mux"
)

// JobHandler handles job-related HTTP requests
type JobHandler struct {
	jobRepo        *repository.JobRepository
	allocationRepo *repository.AllocationRepository
	eventRepo      *repository.EventRepository
	artifactRepo   *repository.ArtifactRepository
	scheduler      *scheduler.Scheduler
}

// NewJobHandler creates a new job handler
func NewJobHandler(
	jobRepo *repository.JobRepository,
	allocationRepo *repository.AllocationRepository,
	eventRepo *repository.EventRepository,
	artifactRepo *repository.ArtifactRepository,
	sched *scheduler.Scheduler,
) *JobHandler {
	return &JobHandler{
		jobRepo:        jobRepo,
		allocationRepo: allocationRepo,
		eventRepo:      eventRepo,
		artifactRepo:   artifactRepo,
		scheduler:      sched,
	}
}

// SubmitJobRequest represents the request to submit a job
type SubmitJobRequest struct {
	Name     string `json:"name"`
	SpecYAML string `json:"spec_yaml"`
}

// SubmitJobResponse represents the response after submitting a job
type SubmitJobResponse struct {
	ID        string    `json:"id"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

// SubmitJob handles POST /v1/jobs
func (h *JobHandler) SubmitJob(w http.ResponseWriter, r *http.Request) {
	var req SubmitJobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Parse YAML spec
	job, err := spec.ParseJobSpec(req.SpecYAML)
	if err != nil {
		http.Error(w, "Invalid job spec: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Set user ID and name (TODO: Get from auth context)
	job.UserID = "default-user" // TODO: Extract from auth token
	job.Name = req.Name

	// Create job in database
	if err := h.jobRepo.CreateJob(job); err != nil {
		http.Error(w, "Failed to create job: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Enqueue job for scheduling
	h.scheduler.Enqueue(job)

	resp := SubmitJobResponse{
		ID:        job.ID,
		Status:    string(job.Status),
		CreatedAt: job.CreatedAt,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

// GetJob handles GET /v1/jobs/{id}
func (h *JobHandler) GetJob(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	jobID := vars["id"]

	job, err := h.jobRepo.GetJob(jobID)
	if err != nil {
		http.Error(w, "Job not found", http.StatusNotFound)
		return
	}

	// Get allocations
	allocations, _ := h.allocationRepo.GetAllocationsByJobID(jobID)

	// Build response
	response := map[string]interface{}{
		"id":             job.ID,
		"name":           job.Name,
		"status":         job.Status,
		"job_type":       job.JobType,
		"framework":      job.Framework,
		"execution_mode": job.Requirements.ExecutionMode,
		"allocations":    allocations,
		"cost": map[string]interface{}{
			"running_usd":   job.CostRunningUSD,
			"estimated_usd": job.CostEstimatedUSD,
		},
		"timestamps": map[string]interface{}{
			"created_at":  job.CreatedAt,
			"started_at":  job.StartedAt,
			"finished_at": job.CompletedAt,
		},
	}

	if job.SelectedProvider != nil {
		response["selected"] = map[string]interface{}{
			"provider":      *job.SelectedProvider,
			"region":        job.SelectedRegion,
			"backend":       job.SelectedBackend,
			"instance_type": allocations[0].InstanceType, // TODO: Handle multiple allocations
			"spot":          allocations[0].Spot,
			"count":         allocations[0].Count,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// ListJobs handles GET /v1/jobs
func (h *JobHandler) ListJobs(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	statusParam := r.URL.Query().Get("status")
	limit := 50 // Default limit
	if limitParam := r.URL.Query().Get("limit"); limitParam != "" {
		fmt.Sscanf(limitParam, "%d", &limit)
	}
	cursor := r.URL.Query().Get("cursor")

	var status *models.JobStatus
	if statusParam != "" {
		s := models.JobStatus(statusParam)
		status = &s
	}

	// Fetch jobs from database
	jobs, nextCursor, err := h.jobRepo.ListJobs("", status, limit, cursor)
	if err != nil {
		http.Error(w, "Failed to list jobs: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Build response items
	items := make([]map[string]interface{}, len(jobs))
	for i, job := range jobs {
		items[i] = map[string]interface{}{
			"id":         job.ID,
			"name":       job.Name,
			"status":     job.Status,
			"job_type":   job.JobType,
			"framework":  job.Framework,
			"created_at": job.CreatedAt,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"items":       items,
		"next_cursor": nextCursor,
	})
}

// CancelJob handles POST /v1/jobs/{id}/cancel
func (h *JobHandler) CancelJob(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	jobID := vars["id"]

	// Get current job status
	job, err := h.jobRepo.GetJob(jobID)
	if err != nil {
		http.Error(w, "Job not found", http.StatusNotFound)
		return
	}

	// Update job status to cancelled
	if err := h.jobRepo.UpdateJobStatus(job.ID, job.Status, models.JobStatusCancelled, "user_cancelled", nil); err != nil {
		http.Error(w, "Failed to cancel job: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// TODO: Trigger cleanup (terminate instances, etc.)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":     job.ID,
		"status": "cancelled",
	})
}

// GetJobEvents handles GET /v1/jobs/{id}/events
func (h *JobHandler) GetJobEvents(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	jobID := vars["id"]

	// Verify job exists
	_, err := h.jobRepo.GetJob(jobID)
	if err != nil {
		http.Error(w, "Job not found", http.StatusNotFound)
		return
	}

	// Fetch events
	events, err := h.eventRepo.GetJobEvents(jobID, 100)
	if err != nil {
		http.Error(w, "Failed to fetch events: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Build response items
	items := make([]map[string]interface{}, len(events))
	for i, event := range events {
		item := map[string]interface{}{
			"at":        event.At,
			"to_status": event.ToStatus,
			"reason":    event.Reason,
		}
		if event.FromStatus != nil {
			item["from_status"] = *event.FromStatus
		}
		items[i] = item
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"items": items,
	})
}

// GetJobArtifacts handles GET /v1/jobs/{id}/artifacts
func (h *JobHandler) GetJobArtifacts(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	jobID := vars["id"]

	// Verify job exists
	_, err := h.jobRepo.GetJob(jobID)
	if err != nil {
		http.Error(w, "Job not found", http.StatusNotFound)
		return
	}

	// Parse optional type filter
	var artifactType *models.ArtifactType
	if typeParam := r.URL.Query().Get("type"); typeParam != "" {
		t := models.ArtifactType(typeParam)
		artifactType = &t
	}

	// Fetch artifacts
	artifacts, err := h.artifactRepo.GetJobArtifacts(jobID, artifactType)
	if err != nil {
		http.Error(w, "Failed to fetch artifacts: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Build response items
	items := make([]map[string]interface{}, len(artifacts))
	for i, artifact := range artifacts {
		items[i] = map[string]interface{}{
			"type":       artifact.Type,
			"uri":        artifact.URI,
			"created_at": artifact.CreatedAt,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"items": items,
	})
}
