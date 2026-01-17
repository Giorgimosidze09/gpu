package models

import "time"

// Job represents a training job submitted to the platform
type Job struct {
	ID               string
	UserID           string
	Name             string
	TeamID           string // For cost attribution (like Run:AI/Cast AI)
	ProjectID        string // For cost attribution (like Run:AI/Cast AI)
	JobType          JobType
	Framework        string // "pytorch_ddp", "horovod", "tensorflow_multiworker"
	EntrypointURI    string // S3/MinIO path or git repo (s3:// or minio:// for MVP)
	DatasetURI       string // Dataset location
	Requirements     JobRequirements
	Constraints      JobConstraints
	Status           JobStatus
	SelectedProvider *Provider
	SelectedRegion   string
	SelectedBackend  BackendType
	ClusterVPC       string
	ClusterID        *string
	CreatedAt        time.Time
	StartedAt        *time.Time
	CompletedAt      *time.Time
	UpdatedAt        time.Time
	CostRunningUSD   float64
	CostEstimatedUSD *float64
	SpecYAML         string // Original spec for replay/debug
}

// JobType represents the type of job
type JobType string

const (
	JobTypeTraining  JobType = "training"
	JobTypeHPO       JobType = "hpo"
	JobTypeInference JobType = "inference"
	JobTypeEval      JobType = "eval"
)

// JobRequirements specifies the resource requirements for a job
type JobRequirements struct {
	GPUs              int
	GPUFraction       float64 // 0.0 - 1.0 (for fractional GPUs, like Run:AI) - MVP: always 1.0
	UseMIG            bool    // Enable MIG partitioning (like Run:AI/Cast AI) - MVP: false
	MIGProfile        string  // e.g., "1g.10gb" (for MIG-capable GPUs like A100)
	MaxGPUsPerNode    int     // Max GPUs per instance (for multi-node training)
	RequiresMultiNode bool    // Whether job requires multiple nodes
	GPUMemory         int     // GB per GPU
	CPUMemory         int     // GB per instance
	Storage           int     // GB
	EstimatedHours    float64
	Framework         string
	ExecutionMode     ExecutionMode // ModeSingleCluster or ModeMultiTask
	DatasetLocation   string        // URI (s3://, gs://, az://, minio://)
}

// JobConstraints specifies constraints for job execution
type JobConstraints struct {
	MaxBudget         float64 // USD
	Deadline          *time.Time
	PreferredRegions  []string
	AllowSpot         bool
	MinReliability    float64           // 0.0 - 1.0
	DataLocality      DataLocality      // prefer | required | ignore
	PerformanceWeight float64           // 0.0 (cost only) to 1.0 (performance only)
	ReplicationPolicy ReplicationPolicy // none | pre-stage | on-demand-cache
}

// JobStatus represents the current status of a job
type JobStatus string

const (
	JobStatusPending       JobStatus = "pending"
	JobStatusScheduled     JobStatus = "scheduled"
	JobStatusProvisioning  JobStatus = "provisioning"
	JobStatusRunning       JobStatus = "running"
	JobStatusCheckpointing JobStatus = "checkpointing"
	JobStatusCompleted     JobStatus = "completed"
	JobStatusFailed        JobStatus = "failed"
	JobStatusCancelled     JobStatus = "cancelled"
)

// ExecutionMode determines how the job is executed
type ExecutionMode string

const (
	ModeSingleCluster ExecutionMode = "single_cluster"
	ModeMultiTask     ExecutionMode = "multi_task"
)

// DataLocality specifies data locality requirements
type DataLocality string

const (
	DataLocalityPrefer   DataLocality = "prefer"
	DataLocalityRequired DataLocality = "required"
	DataLocalityIgnore   DataLocality = "ignore"
)

// ReplicationPolicy specifies how datasets should be replicated
type ReplicationPolicy string

const (
	ReplicationNone          ReplicationPolicy = "none"
	ReplicationPreStage      ReplicationPolicy = "pre-stage"
	ReplicationOnDemandCache ReplicationPolicy = "on-demand-cache"
)
