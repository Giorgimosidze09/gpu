package executor

import (
	"context"
	"fmt"
	"log"
	"time"

	"gpu-orchestrator/core/models"
	"gpu-orchestrator/core/repository"
	"gpu-orchestrator/training/frameworks"
)

// TrainingExecutor executes training jobs on provisioned instances
type TrainingExecutor struct {
	jobRepo      *repository.JobRepository
	pyTorchSetup *frameworks.PyTorchSetup
}

// NewTrainingExecutor creates a new training executor
func NewTrainingExecutor(jobRepo *repository.JobRepository) *TrainingExecutor {
	return &TrainingExecutor{
		jobRepo:      jobRepo,
		pyTorchSetup: &frameworks.PyTorchSetup{},
	}
}

// ExecuteJob executes a training job on a cluster
func (e *TrainingExecutor) ExecuteJob(
	ctx context.Context,
	job *models.Job,
	cluster *models.Cluster,
) error {
	log.Printf("Executing training job %s on cluster %s", job.ID, cluster.ID)

	// Setup distributed training based on framework
	var config *frameworks.DistributedConfig
	var trainingScript string
	var err error

	switch job.Framework {
	case "pytorch_ddp":
		config, err = e.pyTorchSetup.SetupDistributedTraining(cluster, job)
		if err != nil {
			return fmt.Errorf("failed to setup PyTorch DDP: %w", err)
		}
		trainingScript = e.pyTorchSetup.GenerateTrainingScript(config, job)
	case "horovod", "horovod_elastic":
		// Phase 4: Horovod support
		horovodSetup := &frameworks.HorovodSetup{}
		config, err = horovodSetup.SetupDistributedTraining(cluster, job)
		if err != nil {
			return fmt.Errorf("failed to setup Horovod: %w", err)
		}
		trainingScript = horovodSetup.GenerateTrainingScript(config, job)
	case "tensorflow_multiworker":
		// Phase 4: TensorFlow MultiWorker support
		tfSetup := &frameworks.TensorFlowSetup{}
		config, err = tfSetup.SetupDistributedTraining(cluster, job)
		if err != nil {
			return fmt.Errorf("failed to setup TensorFlow: %w", err)
		}
		trainingScript = tfSetup.GenerateTrainingScript(config, job)
	default:
		return fmt.Errorf("unsupported framework: %s", job.Framework)
	}

	// Execute on each node
	// TODO: Implement SSH execution
	// For now, log the script
	log.Printf("Training script for job %s:\n%s", job.ID, trainingScript)

	// Simulate execution
	go e.simulateExecution(ctx, job, cluster)

	return nil
}

// simulateExecution simulates training execution (for MVP testing)
func (e *TrainingExecutor) simulateExecution(ctx context.Context, job *models.Job, cluster *models.Cluster) {
	// Simulate training time
	estimatedDuration := time.Duration(job.Requirements.EstimatedHours * float64(time.Hour))

	log.Printf("Simulating training execution for job %s (estimated: %v)", job.ID, estimatedDuration)

	// For testing, use shorter duration
	testDuration := 30 * time.Second
	if estimatedDuration < testDuration {
		testDuration = estimatedDuration
	}

	time.Sleep(testDuration)

	// Update job status to completed
	if err := e.jobRepo.UpdateJobStatus(
		job.ID,
		models.JobStatusRunning,
		models.JobStatusCompleted,
		"training_completed",
		nil,
	); err != nil {
		log.Printf("Failed to update job status: %v", err)
	}

	log.Printf("Job %s completed", job.ID)
}

// ExecuteOnNode executes a command on a specific node via SSH
// Phase 4: Real SSH execution implementation
func (e *TrainingExecutor) ExecuteOnNode(ctx context.Context, node *models.Node, command string) error {
	// Phase 4: Use SSH client for execution
	// TODO: Get SSH key and user from config
	// For now, return error indicating config needed
	return fmt.Errorf("SSH execution requires SSH key configuration - Phase 4")
}
