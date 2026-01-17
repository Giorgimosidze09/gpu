package optimizer

import (
	"fmt"
	"sort"
	"strings"

	"gpu-orchestrator/core/models"
)

// cheapestMultiProviderStrategy distributes tasks across providers for multi-task jobs
// Phase 2: Full implementation
func (ao *AllocationOptimizer) cheapestMultiProviderStrategy(
	candidates []models.GPUInstance,
	requirements models.JobRequirements,
	constraints models.JobConstraints,
) Strategy {
	// For multi-task, we can distribute across providers
	// This is the original cheapestStrategy (allows cross-provider)
	return ao.cheapestStrategy(candidates, requirements, constraints)
}

// geoDistributedTaskStrategy distributes tasks geographically for parallel execution
// Phase 2: Full implementation
func (ao *AllocationOptimizer) geoDistributedTaskStrategy(
	candidates []models.GPUInstance,
	requirements models.JobRequirements,
	constraints models.JobConstraints,
) Strategy {
	// Phase 2: Distribute tasks across regions for parallel execution
	// This is useful for HPO sweeps, batch inference, etc.

	// Group candidates by region
	regionGroups := make(map[string][]models.GPUInstance)
	for _, instance := range candidates {
		key := fmt.Sprintf("%s:%s", instance.Provider, instance.Region)
		regionGroups[key] = append(regionGroups[key], instance)
	}

	// For multi-task, we can distribute across regions
	// Calculate how many tasks we can run in parallel
	// For now, distribute evenly across available regions

	var allocations []models.Allocation
	gpusPerTask := 1 // Assume each task needs 1 GPU (can be configured)
	totalTasks := requirements.GPUs / gpusPerTask

	if totalTasks == 0 {
		totalTasks = 1
	}

	// Distribute tasks across regions (round-robin)
	regions := make([]string, 0, len(regionGroups))
	for key := range regionGroups {
		regions = append(regions, key)
	}

	if len(regions) == 0 {
		return Strategy{}
	}

	tasksPerRegion := totalTasks / len(regions)
	if tasksPerRegion == 0 {
		tasksPerRegion = 1
	}

	for i, regionKey := range regions {
		instances := regionGroups[regionKey]
		if len(instances) == 0 {
			continue
		}

		// Get cheapest instance in this region
		sort.Slice(instances, func(i, j int) bool {
			priceI := instances[i].PricePerHour
			if instances[i].SpotPrice > 0 && constraints.AllowSpot {
				priceI = instances[i].SpotPrice
			}
			priceJ := instances[j].PricePerHour
			if instances[j].SpotPrice > 0 && constraints.AllowSpot {
				priceJ = instances[j].SpotPrice
			}
			return priceI < priceJ
		})

		bestInstance := instances[0]
		// Parse region key (format: "provider:region")
		parts := strings.Split(regionKey, ":")
		var provider models.Provider
		var region string
		if len(parts) == 2 {
			provider = models.Provider(parts[0])
			region = parts[1]
		} else {
			provider = models.ProviderAWS
			region = "us-east-1"
		}

		// Calculate how many instances needed for tasks in this region
		remainingTasks := tasksPerRegion
		if i == len(regions)-1 {
			// Last region gets remaining tasks
			remainingTasks = totalTasks - (tasksPerRegion * (len(regions) - 1))
		}

		instancesNeeded := (remainingTasks*gpusPerTask + bestInstance.GPUsPerInstance - 1) / bestInstance.GPUsPerInstance

		useSpot := constraints.AllowSpot && bestInstance.SpotPrice > 0
		price := bestInstance.PricePerHour
		if useSpot {
			price = bestInstance.SpotPrice
		}

		allocations = append(allocations, models.Allocation{
			Provider:      provider,
			InstanceType:  bestInstance.InstanceType,
			Region:        region,
			Count:         instancesNeeded,
			Spot:          useSpot,
			PricePerHour:  price,
			EstimatedCost: price * float64(instancesNeeded) * requirements.EstimatedHours,
		})
	}

	return Strategy{Allocation: allocations}
}

// hybridTaskStrategy uses on-prem first, cloud as backup
// Phase 2: Full implementation
func (ao *AllocationOptimizer) hybridTaskStrategy(
	candidates []models.GPUInstance,
	requirements models.JobRequirements,
	constraints models.JobConstraints,
) Strategy {
	// Prefer on-premise instances first
	// Use cloud as backup if on-premise doesn't have capacity
	onPremCandidates := []models.GPUInstance{}
	cloudCandidates := []models.GPUInstance{}

	for _, instance := range candidates {
		if instance.Provider == models.ProviderOnPrem {
			onPremCandidates = append(onPremCandidates, instance)
		} else {
			cloudCandidates = append(cloudCandidates, instance)
		}
	}

	// Try on-premise first
	if len(onPremCandidates) > 0 {
		strategy := ao.cheapestStrategy(onPremCandidates, requirements, constraints)
		if len(strategy.Allocation) > 0 {
			return strategy
		}
	}

	// Fall back to cloud
	return ao.cheapestStrategy(cloudCandidates, requirements, constraints)
}
