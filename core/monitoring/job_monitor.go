package monitoring

import (
	"context"
	"log"
	"time"

	"gpu-orchestrator/core/models"
	"gpu-orchestrator/core/repository"
)

// JobMonitor monitors job execution and health
// Phase 4: Enhanced job monitoring
type JobMonitor struct {
	jobRepo     *repository.JobRepository
	costTracker *CostTracker
}

// NewJobMonitor creates a new job monitor
func NewJobMonitor(
	jobRepo *repository.JobRepository,
	costTracker *CostTracker,
) *JobMonitor {
	return &JobMonitor{
		jobRepo:     jobRepo,
		costTracker: costTracker,
	}
}

// Start starts the job monitoring loop
func (jm *JobMonitor) Start(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second) // Check every 30 seconds
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			jm.monitorRunningJobs(ctx)
		}
	}
}

// monitorRunningJobs monitors all running jobs
func (jm *JobMonitor) monitorRunningJobs(ctx context.Context) {
	// Phase 4: Monitor running jobs for health, progress, and cost
	status := models.JobStatusRunning
	jobs, _, err := jm.jobRepo.ListJobs("", &status, 100, "")
	if err != nil {
		log.Printf("Failed to fetch running jobs: %v", err)
		return
	}

	for _, job := range jobs {
		jm.checkJobHealth(ctx, job)
		jm.checkJobProgress(ctx, job)
		jm.checkJobCost(ctx, job)
	}
}

// checkJobHealth checks if job is healthy
func (jm *JobMonitor) checkJobHealth(ctx context.Context, job *models.Job) {
	// Phase 4: Check job health
	// - Check if nodes are responsive
	// - Check if training process is running
	// - Check for errors in logs

	// TODO: Implement actual health checks
	// - SSH to nodes and check process status
	// - Check for error patterns in logs
	// - Check node availability

	// For now, just log
	log.Printf("Checking health for job %s", job.ID)
}

// checkJobProgress checks job training progress
func (jm *JobMonitor) checkJobProgress(ctx context.Context, job *models.Job) {
	// Phase 4: Check training progress
	// - Parse training logs for step/epoch progress
	// - Estimate completion time
	// - Detect if training is stuck

	// TODO: Implement progress tracking
	// - Parse logs for step numbers
	// - Calculate steps per hour
	// - Estimate remaining time
	// - Detect if progress stalled

	log.Printf("Checking progress for job %s", job.ID)
}

// checkJobCost checks if job is approaching budget limits
func (jm *JobMonitor) checkJobCost(ctx context.Context, job *models.Job) {
	// Phase 4: Check cost against budget
	currentCost := jm.costTracker.GetRunningCost(job.ID)

	if job.Constraints.MaxBudget > 0 {
		budgetUsage := currentCost / job.Constraints.MaxBudget

		if budgetUsage >= 0.9 {
			// 90% of budget used - send alert
			log.Printf("WARNING: Job %s has used %.1f%% of budget (%.2f / %.2f USD)",
				job.ID, budgetUsage*100, currentCost, job.Constraints.MaxBudget)
		}

		if budgetUsage >= 1.0 {
			// Budget exceeded - should cancel job
			log.Printf("ERROR: Job %s exceeded budget (%.2f / %.2f USD) - should cancel",
				job.ID, currentCost, job.Constraints.MaxBudget)
		}
	}
}

// GetJobMetrics returns metrics for a job
func (jm *JobMonitor) GetJobMetrics(jobID string) (*JobMetrics, error) {
	// Phase 4: Get comprehensive job metrics
	job, err := jm.jobRepo.GetJob(jobID)
	if err != nil {
		return nil, err
	}

	metrics := &JobMetrics{
		JobID:        jobID,
		Status:       job.Status,
		RunningCost:  jm.costTracker.GetRunningCost(jobID),
		EstimatedCost: 0.0,
		StartTime:    job.StartedAt,
		ElapsedTime:  time.Since(*job.StartedAt),
	}

	if job.CostEstimatedUSD != nil {
		metrics.EstimatedCost = *job.CostEstimatedUSD
	}

	return metrics, nil
}

// JobMetrics represents job monitoring metrics
type JobMetrics struct {
	JobID         string
	Status        models.JobStatus
	RunningCost   float64
	EstimatedCost float64
	StartTime     *time.Time
	ElapsedTime   time.Duration
	// TODO: Add more metrics:
	// - Steps completed
	// - Steps per hour
	// - GPU utilization
	// - Network bandwidth
	// - Storage throughput
}
