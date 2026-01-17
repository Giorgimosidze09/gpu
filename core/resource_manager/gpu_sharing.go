package resource_manager

import (
	"context"
	"fmt"
	"log"

	"gpu-orchestrator/core/models"
)

// GPUSharingManager manages GPU sharing features (MIG, fractional GPUs, time-slicing)
// Phase 3: Like Run:AI/Cast AI GPU sharing capabilities
type GPUSharingManager struct {
	// Tracks GPU allocations and sharing
	gpuAllocations map[string]*GPUAllocation
}

// GPUAllocation represents a GPU allocation with sharing info
type GPUAllocation struct {
	GPUID        string
	NodeID       string
	Provider     models.Provider
	GPUType      string
	TotalMemory  int // GB
	UsedMemory   int // GB
	Allocations  []JobGPUAllocation
	MIGEnabled   bool
	MIGProfile   string
	TimeSlicing  bool
}

// JobGPUAllocation represents a job's allocation on a shared GPU
type JobGPUAllocation struct {
	JobID       string
	GPUFraction float64 // 0.0 - 1.0
	MemoryGB    int
	MIGInstance string // MIG instance ID if using MIG
}

// NewGPUSharingManager creates a new GPU sharing manager
func NewGPUSharingManager() *GPUSharingManager {
	return &GPUSharingManager{
		gpuAllocations: make(map[string]*GPUAllocation),
	}
}

// AllocateGPU allocates GPU resources for a job with sharing support
// Phase 3: Supports fractional GPUs, MIG, and time-slicing
func (gsm *GPUSharingManager) AllocateGPU(
	ctx context.Context,
	job *models.Job,
	node *models.Node,
) (*GPUAllocation, error) {
	// Phase 3: GPU sharing logic
	
	// Check if job requires MIG
	if job.Requirements.UseMIG {
		return gsm.allocateMIG(ctx, job, node)
	}
	
	// Check if job requires fractional GPU
	if job.Requirements.GPUFraction < 1.0 {
		return gsm.allocateFractionalGPU(ctx, job, node)
	}
	
	// Full GPU allocation (no sharing)
	return gsm.allocateFullGPU(ctx, job, node)
}

// allocateMIG allocates MIG (Multi-Instance GPU) partition
// Phase 3: MIG support for A100 and other MIG-capable GPUs
func (gsm *GPUSharingManager) allocateMIG(
	ctx context.Context,
	job *models.Job,
	node *models.Node,
) (*GPUAllocation, error) {
	// Phase 3: MIG allocation
	// MIG allows partitioning a GPU into multiple isolated instances
	// Example: A100 80GB can be partitioned into 7x 1g.10gb instances
	
	log.Printf("Allocating MIG instance for job %s", job.ID)
	
	// Validate MIG profile
	migProfile := job.Requirements.MIGProfile
	if migProfile == "" {
		return nil, fmt.Errorf("MIG profile required when UseMIG is true")
	}
	
	// Parse MIG profile (e.g., "1g.10gb")
	// Format: {count}g.{memory}gb
	// Example: "1g.10gb" = 1 GPU instance with 10GB memory
	
	// TODO: Query node for available MIG instances
	// This requires:
	// 1. Check if GPU supports MIG (A100, A30, etc.)
	// 2. Check if MIG is enabled on the GPU
	// 3. List available MIG instances
	// 4. Allocate matching MIG instance
	
	// For now, create placeholder allocation
	allocation := &GPUAllocation{
		GPUID:       fmt.Sprintf("gpu-%s-0", node.ID),
		NodeID:      node.ID,
		Provider:    node.Provider,
		GPUType:     "A100", // Assume A100 for MIG
		MIGEnabled:  true,
		MIGProfile:  migProfile,
		TimeSlicing: false,
		Allocations: []JobGPUAllocation{
			{
				JobID:       job.ID,
				GPUFraction: 1.0, // MIG instance is full allocation
				MemoryGB:   10,  // From MIG profile
				MIGInstance: "MIG-GPU-0/1/0", // MIG instance ID
			},
		},
	}
	
	return allocation, nil
}

// allocateFractionalGPU allocates fractional GPU (time-slicing)
// Phase 3: Multiple jobs can share one GPU using time-slicing
func (gsm *GPUSharingManager) allocateFractionalGPU(
	ctx context.Context,
	job *models.Job,
	node *models.Node,
) (*GPUAllocation, error) {
	// Phase 3: Fractional GPU allocation
	// This allows multiple jobs to share one physical GPU
	// Uses time-slicing or memory partitioning
	
	log.Printf("Allocating fractional GPU (%.2f) for job %s", job.Requirements.GPUFraction, job.ID)
	
	// Find available GPU on node
	gpuID := fmt.Sprintf("gpu-%s-0", node.ID)
	
	// Check if GPU already has allocations
	existingAlloc, exists := gsm.gpuAllocations[gpuID]
	if !exists {
		// Create new GPU allocation
		existingAlloc = &GPUAllocation{
			GPUID:       gpuID,
			NodeID:      node.ID,
			Provider:    node.Provider,
			GPUType:     "T4", // Assume T4 for fractional (common for sharing)
			TotalMemory: 16,   // GB
			MIGEnabled:  false,
			TimeSlicing: true,
			Allocations: []JobGPUAllocation{},
		}
		gsm.gpuAllocations[gpuID] = existingAlloc
	}
	
	// Check if there's enough capacity
	usedFraction := 0.0
	usedMemory := 0
	for _, alloc := range existingAlloc.Allocations {
		usedFraction += alloc.GPUFraction
		usedMemory += alloc.MemoryGB
	}
	
	requiredFraction := job.Requirements.GPUFraction
	requiredMemory := job.Requirements.GPUMemory
	
	if usedFraction+requiredFraction > 1.0 {
		return nil, fmt.Errorf("insufficient GPU capacity: %.2f used, %.2f required", usedFraction, requiredFraction)
	}
	
	if usedMemory+requiredMemory > existingAlloc.TotalMemory {
		return nil, fmt.Errorf("insufficient GPU memory: %dGB used, %dGB required", usedMemory, requiredMemory)
	}
	
	// Allocate fractional GPU
	jobAlloc := JobGPUAllocation{
		JobID:       job.ID,
		GPUFraction: requiredFraction,
		MemoryGB:    requiredMemory,
	}
	
	existingAlloc.Allocations = append(existingAlloc.Allocations, jobAlloc)
	existingAlloc.UsedMemory += requiredMemory
	
	return existingAlloc, nil
}

// allocateFullGPU allocates full GPU (no sharing)
func (gsm *GPUSharingManager) allocateFullGPU(
	ctx context.Context,
	job *models.Job,
	node *models.Node,
) (*GPUAllocation, error) {
	// Full GPU allocation (no sharing)
	log.Printf("Allocating full GPU for job %s", job.ID)
	
	gpuID := fmt.Sprintf("gpu-%s-0", node.ID)
	
	allocation := &GPUAllocation{
		GPUID:       gpuID,
		NodeID:      node.ID,
		Provider:    node.Provider,
		GPUType:     "V100", // Assume V100
		TotalMemory: job.Requirements.GPUMemory,
		UsedMemory:  job.Requirements.GPUMemory,
		MIGEnabled:  false,
		TimeSlicing: false,
		Allocations: []JobGPUAllocation{
			{
				JobID:       job.ID,
				GPUFraction: 1.0,
				MemoryGB:    job.Requirements.GPUMemory,
			},
		},
	}
	
	gsm.gpuAllocations[gpuID] = allocation
	
	return allocation, nil
}

// ReleaseGPU releases GPU allocation for a job
func (gsm *GPUSharingManager) ReleaseGPU(ctx context.Context, jobID string) error {
	// Phase 3: Release GPU allocation
	log.Printf("Releasing GPU allocation for job %s", jobID)
	
	// Find and remove job allocation
	for gpuID, alloc := range gsm.gpuAllocations {
		for i, jobAlloc := range alloc.Allocations {
			if jobAlloc.JobID == jobID {
				// Remove job allocation
				alloc.Allocations = append(alloc.Allocations[:i], alloc.Allocations[i+1:]...)
				alloc.UsedMemory -= jobAlloc.MemoryGB
				
				// If no more allocations, remove GPU allocation
				if len(alloc.Allocations) == 0 {
					delete(gsm.gpuAllocations, gpuID)
				}
				
				return nil
			}
		}
	}
	
	return fmt.Errorf("GPU allocation not found for job %s", jobID)
}

// GetGPUUtilization returns GPU utilization metrics
func (gsm *GPUSharingManager) GetGPUUtilization(gpuID string) (float64, error) {
	// Phase 3: Calculate GPU utilization
	alloc, exists := gsm.gpuAllocations[gpuID]
	if !exists {
		return 0.0, fmt.Errorf("GPU allocation not found: %s", gpuID)
	}
	
	// Utilization = sum of all fractional allocations
	utilization := 0.0
	for _, jobAlloc := range alloc.Allocations {
		utilization += jobAlloc.GPUFraction
	}
	
	return utilization, nil
}

// CheckMIGSupport checks if GPU supports MIG
func (gsm *GPUSharingManager) CheckMIGSupport(gpuType string) bool {
	// Phase 3: Check if GPU type supports MIG
	// MIG-capable GPUs: A100, A30, A10
	migCapableGPUs := map[string]bool{
		"A100": true,
		"A30":  true,
		"A10":  false, // A10 doesn't support MIG
	}
	
	return migCapableGPUs[gpuType]
}

// GetMIGProfiles returns available MIG profiles for a GPU type
func (gsm *GPUSharingManager) GetMIGProfiles(gpuType string) []string {
	// Phase 3: Return available MIG profiles
	// Example: A100 80GB supports:
	// - 1g.10gb (7 instances)
	// - 2g.20gb (3 instances)
	// - 3g.40gb (2 instances)
	// - 7g.80gb (1 instance)
	
	profiles := map[string][]string{
		"A100": {"1g.10gb", "2g.20gb", "3g.40gb", "7g.80gb"},
		"A30":  {"1g.6gb", "2g.12gb", "3g.24gb", "4g.48gb"},
	}
	
	return profiles[gpuType]
}
