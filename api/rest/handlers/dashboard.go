package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"gpu-orchestrator/core/models"
	"gpu-orchestrator/core/monitoring"
	"gpu-orchestrator/core/repository"
)

// DashboardHandler handles dashboard API requests
type DashboardHandler struct {
	jobRepo     *repository.JobRepository
	costTracker *monitoring.CostTracker
	clusterPool interface{} // TODO: Add cluster pool interface
}

// NewDashboardHandler creates a new dashboard handler
func NewDashboardHandler(
	jobRepo *repository.JobRepository,
	costTracker *monitoring.CostTracker,
) *DashboardHandler {
	return &DashboardHandler{
		jobRepo:     jobRepo,
		costTracker: costTracker,
	}
}

// GetCostMetrics returns cost metrics for dashboard
func (h *DashboardHandler) GetCostMetrics(w http.ResponseWriter, r *http.Request) {
	// Get query parameters
	userID := r.URL.Query().Get("user_id")
	startDate := r.URL.Query().Get("start_date")
	endDate := r.URL.Query().Get("end_date")

	// Parse dates (default to last 30 days)
	var start, end time.Time
	if startDate != "" {
		var err error
		start, err = time.Parse(time.RFC3339, startDate)
		if err != nil {
			http.Error(w, "Invalid start_date format", http.StatusBadRequest)
			return
		}
	} else {
		start = time.Now().AddDate(0, 0, -30)
	}

	if endDate != "" {
		var err error
		end, err = time.Parse(time.RFC3339, endDate)
		if err != nil {
			http.Error(w, "Invalid end_date format", http.StatusBadRequest)
			return
		}
	} else {
		end = time.Now()
	}

	// Get jobs in date range
	// TODO: Add date filtering to ListJobs
	jobs, _, err := h.jobRepo.ListJobs(userID, nil, 1000, "")
	if err != nil {
		http.Error(w, "Failed to fetch jobs: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Calculate metrics
	totalCost := 0.0
	runningCost := 0.0
	completedJobs := 0
	runningJobs := 0

	for _, job := range jobs {
		// Filter by date
		if job.CreatedAt.Before(start) || job.CreatedAt.After(end) {
			continue
		}

		if job.Status == models.JobStatusRunning {
			runningJobs++
			runningCost += h.costTracker.GetRunningCost(job.ID)
		}

		if job.Status == models.JobStatusCompleted {
			completedJobs++
			if job.CostRunningUSD > 0 {
				totalCost += job.CostRunningUSD
			} else if job.CostEstimatedUSD != nil {
				totalCost += *job.CostEstimatedUSD
			}
		}
	}

	response := map[string]interface{}{
		"period": map[string]interface{}{
			"start": start.Format(time.RFC3339),
			"end":   end.Format(time.RFC3339),
		},
		"costs": map[string]interface{}{
			"total_usd":     totalCost,
			"running_usd":   runningCost,
			"estimated_usd": totalCost + runningCost,
		},
		"jobs": map[string]interface{}{
			"completed": completedJobs,
			"running":   runningJobs,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetJobCosts returns cost breakdown by job
func (h *DashboardHandler) GetJobCosts(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("user_id")
	limit := 50
	if limitParam := r.URL.Query().Get("limit"); limitParam != "" {
		fmt.Sscanf(limitParam, "%d", &limit)
	}

	jobs, _, err := h.jobRepo.ListJobs(userID, nil, limit, "")
	if err != nil {
		http.Error(w, "Failed to fetch jobs: "+err.Error(), http.StatusInternalServerError)
		return
	}

	jobCosts := make([]map[string]interface{}, 0, len(jobs))
	for _, job := range jobs {
		cost := job.CostRunningUSD
		if job.Status == models.JobStatusCompleted && cost == 0 && job.CostEstimatedUSD != nil {
			cost = *job.CostEstimatedUSD
		}

		jobCosts = append(jobCosts, map[string]interface{}{
			"job_id":     job.ID,
			"name":       job.Name,
			"status":     job.Status,
			"cost_usd":   cost,
			"created_at": job.CreatedAt,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"items": jobCosts,
	})
}
