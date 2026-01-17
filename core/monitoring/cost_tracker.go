package monitoring

import (
	"context"
	"log"
	"sync"
	"time"

	"gpu-orchestrator/core/models"
	"gpu-orchestrator/core/repository"
)

// CostTracker tracks real-time costs for running jobs
type CostTracker struct {
	jobRepo      *repository.JobRepository
	jobCosts     map[string]*JobCost
	mu           sync.RWMutex
	updateTicker *time.Ticker
}

// JobCost tracks cost for a single job
type JobCost struct {
	JobID       string
	StartTime   time.Time
	RunningCost float64
	Allocations []models.Allocation
	LastUpdate  time.Time
}

// NewCostTracker creates a new cost tracker
func NewCostTracker(jobRepo *repository.JobRepository) *CostTracker {
	return &CostTracker{
		jobRepo:      jobRepo,
		jobCosts:     make(map[string]*JobCost),
		updateTicker: time.NewTicker(1 * time.Minute), // Update every minute
	}
}

// Start starts the cost tracking worker
func (ct *CostTracker) Start(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-ct.updateTicker.C:
			ct.updateAllJobCosts(ctx)
		}
	}
}

// TrackJob starts tracking cost for a job
func (ct *CostTracker) TrackJob(jobID string, allocations []models.Allocation) {
	ct.mu.Lock()
	defer ct.mu.Unlock()

	ct.jobCosts[jobID] = &JobCost{
		JobID:       jobID,
		StartTime:   time.Now(),
		Allocations: allocations,
		LastUpdate:  time.Now(),
	}
}

// StopTracking stops tracking a job
func (ct *CostTracker) StopTracking(jobID string) {
	ct.mu.Lock()
	defer ct.mu.Unlock()

	delete(ct.jobCosts, jobID)
}

// updateAllJobCosts updates costs for all tracked jobs
func (ct *CostTracker) updateAllJobCosts(ctx context.Context) {
	ct.mu.RLock()
	jobIDs := make([]string, 0, len(ct.jobCosts))
	for jobID := range ct.jobCosts {
		jobIDs = append(jobIDs, jobID)
	}
	ct.mu.RUnlock()

	for _, jobID := range jobIDs {
		ct.updateJobCost(ctx, jobID)
	}
}

// updateJobCost updates cost for a single job
func (ct *CostTracker) updateJobCost(_ context.Context, jobID string) {
	ct.mu.Lock()
	jobCost, exists := ct.jobCosts[jobID]
	if !exists {
		ct.mu.Unlock()
		return
	}
	ct.mu.Unlock()

	// Get current job status
	job, err := ct.jobRepo.GetJob(jobID)
	if err != nil {
		log.Printf("Failed to fetch job %s for cost update: %v", jobID, err)
		return
	}

	// Only track costs for running jobs
	if job.Status != models.JobStatusRunning {
		return
	}

	// Calculate delta time since last update
	now := time.Now()
	deltaHours := now.Sub(jobCost.LastUpdate).Hours()

	// Calculate cost for delta time
	deltaCost := 0.0
	for _, alloc := range jobCost.Allocations {
		deltaCost += alloc.PricePerHour * float64(alloc.Count) * deltaHours
	}

	// Update running cost
	jobCost.RunningCost += deltaCost
	jobCost.LastUpdate = now

	// Update in database
	if err := ct.jobRepo.UpdateJobCost(jobID, jobCost.RunningCost); err != nil {
		log.Printf("Failed to update cost for job %s: %v", jobID, err)
	}
}

// GetRunningCost returns the current running cost for a job
func (ct *CostTracker) GetRunningCost(jobID string) float64 {
	ct.mu.RLock()
	defer ct.mu.RUnlock()

	jobCost, exists := ct.jobCosts[jobID]
	if !exists {
		return 0.0
	}

	return jobCost.RunningCost
}
