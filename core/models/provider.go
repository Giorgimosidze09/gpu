package models

import "time"

// Provider represents a cloud provider
type Provider string

const (
	ProviderAWS    Provider = "aws"
	ProviderGCP    Provider = "gcp"
	ProviderAzure  Provider = "azure"
	ProviderOnPrem Provider = "onprem"
)

// GPUInstance represents a GPU instance type available from a provider
type GPUInstance struct {
	Provider         Provider
	InstanceType     string // "p3.2xlarge", "a2-highgpu-1g"
	Region           string
	GPUType          string // "A100", "V100", "T4"
	GPUsPerInstance  int
	MemoryPerGPU     int // GB
	PricePerHour     float64
	SpotPrice        float64          // If available
	Availability     float64          // 0.0 - 1.0
	InterconnectTier InterconnectTier // "standard" | "high" (for multi-node training)
	LastUpdated      time.Time        // When pricing was fetched
}

// InterconnectTier specifies the network interconnect tier
type InterconnectTier string

const (
	InterconnectStandard InterconnectTier = "standard"
	InterconnectHigh     InterconnectTier = "high"
)

// Cluster represents a logical grouping of nodes that share provider/region/network domain
// For BackendVM, cluster = "a managed group of instances in same VPC/subnet/AZ group"
// All nodes in a cluster can communicate with low latency (required for DDP/Horovod)
type Cluster struct {
	ID       string
	Provider Provider
	Region   string
	VPC      string // Network domain
	Backend  BackendType
	Nodes    []Node // All nodes in this cluster
}

// Node represents a compute node in a cluster
type Node struct {
	ID         string
	InstanceID string // Provider-specific instance ID
	Provider   Provider
	Region     string
	VPC        string
	PrivateIP  string // For DDP communication
	GPUs       int
}

// BackendType represents the compute backend
type BackendType string

const (
	BackendKubernetes BackendType = "k8s"   // Kubernetes cluster
	BackendSlurm      BackendType = "slurm" // Slurm cluster
	BackendRay        BackendType = "ray"   // Ray cluster
	BackendVM         BackendType = "vm"    // Raw VMs (MVP only)
)

// Target represents a compute target (provider + region + backend)
type Target struct {
	Provider Provider
	Region   string
	Backend  BackendType
}

// PerformanceMetrics tracks performance metrics for $/step optimization
type PerformanceMetrics struct {
	StepsPerHour         float64 // Training steps per hour
	TokensPerHour        float64 // For LLM training
	StorageThroughput    float64 // MB/s
	NetworkBandwidth     float64 // Gbps (for multi-node)
	EffectiveCostPerStep float64 // PricePerHour / StepsPerHour
}

// Allocation represents a compute allocation decision
type Allocation struct {
	Provider      Provider
	InstanceType  string
	Region        string
	Count         int
	Spot          bool
	PricePerHour  float64 // Price per hour per instance (explicit for cost tracking)
	EstimatedCost float64 // Total estimated cost (PricePerHour * Count * Hours)
	EstimatedTime time.Duration
}
