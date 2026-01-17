package scheduler

import (
	"context"
	"log"
	"time"

	"gpu-orchestrator/core/executor"
	"gpu-orchestrator/core/models"
	"gpu-orchestrator/core/optimizer"
	"gpu-orchestrator/core/repository"
	"gpu-orchestrator/core/resource_manager"
)

// Scheduler manages job scheduling and execution
type Scheduler struct {
	jobRepo        *repository.JobRepository
	allocationRepo *repository.AllocationRepository
	queue          *JobQueue
	optimizer      *optimizer.AllocationOptimizer
	provisioner    *resource_manager.Provisioner
	executor       *executor.TrainingExecutor
	stopChan       chan struct{}
}

// NewScheduler creates a new scheduler
func NewScheduler(
	jobRepo *repository.JobRepository,
	allocationRepo *repository.AllocationRepository,
	optimizer *optimizer.AllocationOptimizer,
	provisioner *resource_manager.Provisioner,
	executor *executor.TrainingExecutor,
) *Scheduler {
	return &Scheduler{
		jobRepo:        jobRepo,
		allocationRepo: allocationRepo,
		queue:          NewJobQueue(),
		optimizer:      optimizer,
		provisioner:    provisioner,
		executor:       executor,
		stopChan:       make(chan struct{}),
	}
}

// Start starts the scheduler worker
func (s *Scheduler) Start(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second) // Check queue every 5 seconds
	defer ticker.Stop()

	// Load pending jobs from database
	s.loadPendingJobs(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stopChan:
			return
		case <-ticker.C:
			s.processQueue(ctx)
		}
	}
}

// Stop stops the scheduler
func (s *Scheduler) Stop() {
	close(s.stopChan)
}

// Enqueue adds a job to the queue
func (s *Scheduler) Enqueue(job *models.Job) {
	s.queue.Enqueue(job)
}

// loadPendingJobs loads pending jobs from database
func (s *Scheduler) loadPendingJobs(_ context.Context) {
	status := models.JobStatusPending
	jobs, _, err := s.jobRepo.ListJobs("", &status, 100, "")
	if err != nil {
		log.Printf("Failed to load pending jobs: %v", err)
		return
	}

	for _, job := range jobs {
		s.queue.Enqueue(job)
	}
}

// processQueue processes jobs from the queue
func (s *Scheduler) processQueue(ctx context.Context) {
	for {
		job := s.queue.PopJob()
		if job == nil {
			return
		}

		// Re-fetch job to get latest state
		freshJob, err := s.jobRepo.GetJob(job.ID)
		if err != nil {
			log.Printf("Failed to fetch job %s: %v", job.ID, err)
			continue
		}

		// Skip if job is no longer pending
		if freshJob.Status != models.JobStatusPending {
			continue
		}

		// Process job
		if err := s.processJob(ctx, freshJob); err != nil {
			log.Printf("Failed to process job %s: %v", freshJob.ID, err)
			// Update job status to failed
			s.jobRepo.UpdateJobStatus(freshJob.ID, freshJob.Status, models.JobStatusFailed, "scheduler_error", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}
}

// processJob processes a single job
func (s *Scheduler) processJob(ctx context.Context, job *models.Job) error {
	log.Printf("Processing job %s", job.ID)

	// Step 1: Run optimizer to select allocation
	allocations, err := s.optimizer.Optimize(ctx, job.Requirements, job.Constraints)
	if err != nil {
		return err
	}

	if len(allocations) == 0 {
		return err
	}

	// Step 2: Update job status to scheduled
	if err := s.jobRepo.UpdateJobStatus(job.ID, models.JobStatusPending, models.JobStatusScheduled, "optimizer_selected_allocation", nil); err != nil {
		return err
	}

	// Step 3: Store allocations
	for _, alloc := range allocations {
		if err := s.allocationRepo.CreateAllocation(job.ID, alloc); err != nil {
			return err
		}
	}

	// Step 4: Update job with selected provider/region in database
	// This is done via allocations table, but we could also update jobs table
	// For now, allocations table is sufficient

	// Step 5: Trigger provisioning (async)
	go s.provisionAndExecuteJob(ctx, job, allocations)

	return nil
}

// provisionAndExecuteJob provisions compute resources and executes training
func (s *Scheduler) provisionAndExecuteJob(ctx context.Context, job *models.Job, allocations []models.Allocation) {
	log.Printf("Provisioning resources for job %s", job.ID)

	// Update status to provisioning
	if err := s.jobRepo.UpdateJobStatus(job.ID, models.JobStatusScheduled, models.JobStatusProvisioning, "starting_provisioning", nil); err != nil {
		log.Printf("Failed to update job status: %v", err)
		return
	}

	// Provision cluster
	cluster, err := s.provisioner.ProvisionCluster(ctx, job, allocations)
	if err != nil {
		log.Printf("Failed to provision cluster: %v", err)
		s.jobRepo.UpdateJobStatus(job.ID, models.JobStatusProvisioning, models.JobStatusFailed, "provisioning_failed", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}

	log.Printf("Cluster %s provisioned with %d nodes", cluster.ID, len(cluster.Nodes))

	// Update status to running
	if err := s.jobRepo.UpdateJobStatus(job.ID, models.JobStatusProvisioning, models.JobStatusRunning, "provisioning_complete", nil); err != nil {
		log.Printf("Failed to update job status: %v", err)
		return
	}

	// Execute training
	if err := s.executor.ExecuteJob(ctx, job, cluster); err != nil {
		log.Printf("Failed to execute training: %v", err)
		s.jobRepo.UpdateJobStatus(job.ID, models.JobStatusRunning, models.JobStatusFailed, "execution_failed", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}

	log.Printf("Job %s is now running", job.ID)
}
