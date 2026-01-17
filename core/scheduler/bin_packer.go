package scheduler

import (
	"sort"

	"gpu-orchestrator/core/models"
)

// BinPacker efficiently packs multiple jobs onto the same instances
// Inspired by Cast AI's bin-packing approach
// Phase 2: Full implementation
type BinPacker struct {
	nodes []NodeCapacity
}

// NodeCapacity represents available capacity on a node
type NodeCapacity struct {
	NodeID        string
	TotalGPUs     int
	UsedGPUs      int
	AvailableGPUs int
	Provider      models.Provider
	Region        string
	InstanceType  string
}

// PackJobs packs multiple jobs onto available nodes
// Returns allocations that maximize GPU utilization
func (bp *BinPacker) PackJobs(jobs []*models.Job, nodes []NodeCapacity) []models.Allocation {
	var allocations []models.Allocation

	// Sort jobs by GPU requirements (largest first for better packing)
	sortedJobs := make([]*models.Job, len(jobs))
	copy(sortedJobs, jobs)
	sort.Slice(sortedJobs, func(i, j int) bool {
		return sortedJobs[i].Requirements.GPUs > sortedJobs[j].Requirements.GPUs
	})

	// Track node usage
	nodeUsage := make(map[string]int)
	for _, node := range nodes {
		nodeUsage[node.NodeID] = node.UsedGPUs
	}

	// Pack jobs greedily (best-fit decreasing algorithm)
	for _, job := range sortedJobs {
		gpusNeeded := job.Requirements.GPUs
		packed := false

		// Try to pack on existing nodes first (best-fit)
		bestNode := ""
		bestFit := -1
		
		for _, node := range nodes {
			used := nodeUsage[node.NodeID]
			available := node.AvailableGPUs - used

			if available >= gpusNeeded {
				// Find best fit (smallest available space that fits)
				if bestFit == -1 || available < bestFit {
					bestFit = available
					bestNode = node.NodeID
				}
			}
		}

		if bestNode != "" {
			// Pack job on best-fit node
			nodeUsage[bestNode] += gpusNeeded
			
			// Find node details
			var nodeDetails *NodeCapacity
			for i := range nodes {
				if nodes[i].NodeID == bestNode {
					nodeDetails = &nodes[i]
					break
				}
			}
			
			if nodeDetails != nil {
				// TODO: Phase 2 - Get actual prices and spot status from node/cluster
				allocations = append(allocations, models.Allocation{
					Provider:      nodeDetails.Provider,
					InstanceType:  nodeDetails.InstanceType,
					Region:        nodeDetails.Region,
					Count:         1, // Using existing node
					Spot:          false, // TODO: Get from node
					PricePerHour:  0.0,   // TODO: Get from node
					EstimatedCost: 0.0,   // TODO: Calculate
				})
				packed = true
			}
		}

		// If couldn't pack on existing node, will need new allocation
		// This is handled by the scheduler/optimizer
		if !packed {
			// Job will get its own allocation (not packed)
		}
	}

	return allocations
}

// CalculateUtilization calculates GPU utilization across nodes
func (bp *BinPacker) CalculateUtilization(allocations []models.Allocation, totalGPUs int) float64 {
	if totalGPUs == 0 {
		return 0.0
	}

	usedGPUs := 0
	for _, alloc := range allocations {
		// TODO: Get GPUs per instance from instance type
		usedGPUs += alloc.Count * 8 // Placeholder: assume 8 GPUs per instance
	}

	return float64(usedGPUs) / float64(totalGPUs)
}

// NewBinPacker creates a new bin packer
func NewBinPacker() *BinPacker {
	return &BinPacker{
		nodes: []NodeCapacity{},
	}
}
