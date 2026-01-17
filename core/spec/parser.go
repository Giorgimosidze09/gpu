package spec

import (
	"fmt"
	"time"

	"gpu-orchestrator/core/models"

	"gopkg.in/yaml.v3"
)

// JobSpec represents the YAML job specification
type JobSpec struct {
	Job JobSpecJob `yaml:"job"`
}

// JobSpecJob represents the job section of the spec
type JobSpecJob struct {
	Type        string             `yaml:"type"`
	Framework   string             `yaml:"framework"`
	Entrypoint  string             `yaml:"entrypoint"`
	Resources   JobSpecResources   `yaml:"resources"`
	Data        JobSpecData        `yaml:"data"`
	Constraints JobSpecConstraints `yaml:"constraints"`
	Execution   JobSpecExecution   `yaml:"execution"`
}

// JobSpecResources represents resource requirements
type JobSpecResources struct {
	GPUs              int      `yaml:"gpus"`
	GPUFraction       *float64 `yaml:"gpu_fraction,omitempty"` // Phase 3: Fractional GPU (0.0-1.0)
	UseMIG            *bool    `yaml:"use_mig,omitempty"`     // Phase 3: Enable MIG
	MIGProfile        *string  `yaml:"mig_profile,omitempty"` // Phase 3: MIG profile (e.g., "1g.10gb")
	MaxGPUsPerNode    int      `yaml:"max_gpus_per_node"`
	RequiresMultiNode bool     `yaml:"requires_multi_node"`
	GPUMemory         string   `yaml:"gpu_memory"` // e.g., "80GB"
	CPUMemory         string   `yaml:"cpu_memory"` // e.g., "512GB"
}

// JobSpecData represents data configuration
type JobSpecData struct {
	Dataset           string `yaml:"dataset"`
	Locality          string `yaml:"locality"`
	ReplicationPolicy string `yaml:"replication_policy"`
}

// JobSpecConstraints represents job constraints
type JobSpecConstraints struct {
	Budget            float64 `yaml:"budget"`
	Deadline          string  `yaml:"deadline"` // ISO 8601
	AllowSpot         bool    `yaml:"allow_spot"`
	MinReliability    float64 `yaml:"min_reliability"`
	PerformanceWeight float64 `yaml:"performance_weight"`
}

// JobSpecExecution represents execution configuration
type JobSpecExecution struct {
	Mode   string `yaml:"mode"`              // single_cluster | multi_task
	Backend string `yaml:"backend,omitempty"` // Phase 3: k8s | vm | slurm | ray (default: vm)
}

// ParseJobSpec parses a YAML job specification into a Job model
func ParseJobSpec(specYAML string) (*models.Job, error) {
	var spec JobSpec
	if err := yaml.Unmarshal([]byte(specYAML), &spec); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	job := &models.Job{
		JobType:       models.JobType(spec.Job.Type),
		Framework:     spec.Job.Framework,
		EntrypointURI: spec.Job.Entrypoint,
		DatasetURI:    spec.Job.Data.Dataset,
		Status:        models.JobStatusPending,
		SpecYAML:      specYAML,
	}

	// Parse resources
	// Phase 3: Support GPU sharing (fractional GPUs, MIG)
	gpuFraction := 1.0
	if spec.Job.Resources.GPUFraction != nil {
		gpuFraction = *spec.Job.Resources.GPUFraction
	}
	
	useMIG := false
	migProfile := ""
	if spec.Job.Resources.UseMIG != nil {
		useMIG = *spec.Job.Resources.UseMIG
	}
	if spec.Job.Resources.MIGProfile != nil {
		migProfile = *spec.Job.Resources.MIGProfile
	}
	
	job.Requirements = models.JobRequirements{
		GPUs:              spec.Job.Resources.GPUs,
		GPUFraction:       gpuFraction, // Phase 3: Support fractional GPUs
		UseMIG:            useMIG,     // Phase 3: Support MIG
		MIGProfile:        migProfile,  // Phase 3: MIG profile
		MaxGPUsPerNode:    spec.Job.Resources.MaxGPUsPerNode,
		RequiresMultiNode: spec.Job.Resources.RequiresMultiNode,
		GPUMemory:         parseMemoryGB(spec.Job.Resources.GPUMemory),
		CPUMemory:         parseMemoryGB(spec.Job.Resources.CPUMemory),
		Storage:           0,   // TODO: Parse from spec
		EstimatedHours:    1.0, // TODO: Parse from spec
		Framework:         spec.Job.Framework,
		DatasetLocation:   spec.Job.Data.Dataset,
	}

	// Determine execution mode
	if spec.Job.Execution.Mode != "" {
		job.Requirements.ExecutionMode = models.ExecutionMode(spec.Job.Execution.Mode)
	} else {
		// Auto-detect based on framework
		job.Requirements.ExecutionMode = detectExecutionMode(spec.Job.Framework, spec.Job.Type)
	}
	
	// Phase 3: Parse backend type
	if spec.Job.Execution.Backend != "" {
		job.SelectedBackend = models.BackendType(spec.Job.Execution.Backend)
	} else {
		job.SelectedBackend = models.BackendVM // Default to VM
	}

	// Parse constraints
	job.Constraints = models.JobConstraints{
		MaxBudget:         spec.Job.Constraints.Budget,
		AllowSpot:         spec.Job.Constraints.AllowSpot,
		MinReliability:    spec.Job.Constraints.MinReliability,
		PerformanceWeight: spec.Job.Constraints.PerformanceWeight,
		DataLocality:      models.DataLocality(spec.Job.Data.Locality),
		ReplicationPolicy: models.ReplicationPolicy(spec.Job.Data.ReplicationPolicy),
	}

	// Parse deadline
	if spec.Job.Constraints.Deadline != "" {
		deadline, err := time.Parse(time.RFC3339, spec.Job.Constraints.Deadline)
		if err != nil {
			return nil, fmt.Errorf("invalid deadline format: %w", err)
		}
		job.Constraints.Deadline = &deadline
	}

	// Set defaults
	if job.Constraints.MinReliability == 0 {
		job.Constraints.MinReliability = 0.9
	}
	if job.Constraints.DataLocality == "" {
		job.Constraints.DataLocality = models.DataLocalityPrefer
	}
	if job.Constraints.ReplicationPolicy == "" {
		job.Constraints.ReplicationPolicy = models.ReplicationNone
	}

	return job, nil
}

// parseMemoryGB parses memory string (e.g., "80GB") to GB integer
func parseMemoryGB(memoryStr string) int {
	// Simple parser - assumes format like "80GB" or "512GB"
	// TODO: Handle more formats
	var gb int
	fmt.Sscanf(memoryStr, "%dGB", &gb)
	return gb
}

// detectExecutionMode auto-detects execution mode based on framework and job type
func detectExecutionMode(framework, jobType string) models.ExecutionMode {
	// Multi-task for HPO, inference, eval
	if jobType == "hpo" || jobType == "inference" || jobType == "eval" {
		return models.ModeMultiTask
	}

	// Single-cluster for synchronous training frameworks
	if framework == "pytorch_ddp" || framework == "horovod" || framework == "tensorflow_multiworker" {
		return models.ModeSingleCluster
	}

	// Default to single-cluster for safety
	return models.ModeSingleCluster
}
