package optimizer

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"gpu-orchestrator/core/models"
)

// AllocationOptimizer optimizes compute allocation for jobs
type AllocationOptimizer struct {
	costCalculator     *CostCalculator
	pricingFetcher     *PricingFetcher
	performanceMetrics *PerformanceMetricsStore
}

// NewAllocationOptimizer creates a new allocation optimizer
func NewAllocationOptimizer(cc *CostCalculator, pf *PricingFetcher) *AllocationOptimizer {
	return &AllocationOptimizer{
		costCalculator:     cc,
		pricingFetcher:     pf,
		performanceMetrics: NewPerformanceMetricsStore(),
	}
}

// Strategy represents an allocation strategy with scoring
type Strategy struct {
	Allocation    []models.Allocation
	TotalCost     float64
	Reliability   float64
	EstimatedTime time.Duration
	Score         float64
}

// Optimize optimizes allocation based on job requirements and constraints
func (ao *AllocationOptimizer) Optimize(
	ctx context.Context,
	requirements models.JobRequirements,
	constraints models.JobConstraints,
) ([]models.Allocation, error) {
	// Step 1: Get all available GPU instances
	allInstances, err := ao.pricingFetcher.FetchAllPricing(ctx)
	if err != nil {
		return nil, err
	}

	// Step 2: Filter instances that meet requirements
	candidates := ao.filterCandidates(allInstances, requirements)

	// Step 3: Generate allocation strategies
	strategies := ao.generateStrategies(candidates, requirements, constraints)

	// Step 4: Score each strategy
	scoredStrategies := ao.scoreStrategies(strategies, requirements, constraints)

	// Step 5: Return best strategy
	if len(scoredStrategies) == 0 {
		return nil, fmt.Errorf("no suitable allocation found")
	}

	return scoredStrategies[0].Allocation, nil
}

func (ao *AllocationOptimizer) filterCandidates(
	allInstances map[models.Provider][]models.GPUInstance,
	requirements models.JobRequirements,
) []models.GPUInstance {
	var candidates []models.GPUInstance

	for _, instances := range allInstances {
		for _, instance := range instances {
			// Check if instance meets requirements
			if instance.GPUsPerInstance > 0 &&
				instance.MemoryPerGPU >= requirements.GPUMemory {
				candidates = append(candidates, instance)
			}
		}
	}

	return candidates
}

func (ao *AllocationOptimizer) generateStrategies(
	candidates []models.GPUInstance,
	requirements models.JobRequirements,
	constraints models.JobConstraints,
) []Strategy {
	var strategies []Strategy

	// Split strategy generation by execution mode
	switch requirements.ExecutionMode {
	case models.ModeSingleCluster:
		// Single-cluster strategies: ALL nodes must be same provider+region
		// Strategy 1: Cheapest single region (prefer spot)
		strategies = append(strategies, ao.cheapestSingleRegionStrategy(candidates, requirements, constraints))

		// Strategy 2: Most reliable single region (avoid spot, prefer on-prem)
		strategies = append(strategies, ao.reliableSingleRegionStrategy(candidates, requirements, constraints))

		// Strategy 3: Data locality (prefer region where dataset exists)
		if constraints.DataLocality == models.DataLocalityRequired || constraints.DataLocality == models.DataLocalityPrefer {
			strategies = append(strategies, ao.dataLocalityStrategy(candidates, requirements, constraints))
		}

	case models.ModeMultiTask:
		// Multi-task strategies: Can distribute across providers/regions
		// Strategy 1: Cheapest overall (distribute tasks)
		strategies = append(strategies, ao.cheapestMultiProviderStrategy(candidates, requirements, constraints))

		// Strategy 2: Geographic distribution (for parallel tasks)
		strategies = append(strategies, ao.geoDistributedTaskStrategy(candidates, requirements, constraints))

		// Strategy 3: On-prem first, cloud backup
		strategies = append(strategies, ao.hybridTaskStrategy(candidates, requirements, constraints))
	}

	return strategies
}

// cheapestSingleRegionStrategy finds cheapest strategy within ONE provider+region
func (ao *AllocationOptimizer) cheapestSingleRegionStrategy(
	candidates []models.GPUInstance,
	requirements models.JobRequirements,
	constraints models.JobConstraints,
) Strategy {
	// Group by provider+region
	regionGroups := make(map[string][]models.GPUInstance)
	for _, instance := range candidates {
		key := fmt.Sprintf("%s:%s", instance.Provider, instance.Region)
		regionGroups[key] = append(regionGroups[key], instance)
	}

	// Find cheapest provider+region combination
	var bestStrategy Strategy
	bestCost := 999999.0

	for regionKey, instances := range regionGroups {
		// Check if we can allocate all GPUs in this region
		regionStrategy := ao.cheapestStrategy(instances, requirements, constraints)
		if regionStrategy.TotalCost < bestCost && len(regionStrategy.Allocation) > 0 {
			bestCost = regionStrategy.TotalCost
			bestStrategy = regionStrategy
			// Verify all allocations are in same provider+region
			provider, region := parseRegionKey(regionKey)
			for _, alloc := range regionStrategy.Allocation {
				if alloc.Provider != provider || alloc.Region != region {
					// Skip if allocation spans regions
					bestCost = 999999.0
					break
				}
			}
		}
	}

	return bestStrategy
}

func (ao *AllocationOptimizer) cheapestStrategy(
	candidates []models.GPUInstance,
	requirements models.JobRequirements,
	constraints models.JobConstraints,
) Strategy {
	// Validation: For single-cluster training, check multi-node constraints
	if requirements.ExecutionMode == models.ModeSingleCluster && requirements.RequiresMultiNode {
		// Reject instances without fast interconnect (e.g., single-node only)
		candidates = ao.filterMultiNodeCompatible(candidates, requirements)
	}

	// Sort by price per GPU (prefer spot instances)
	sorted := make([]models.GPUInstance, len(candidates))
	copy(sorted, candidates)

	sort.Slice(sorted, func(i, j int) bool {
		iPricePerGPU := sorted[i].PricePerHour / float64(sorted[i].GPUsPerInstance)
		if sorted[i].SpotPrice > 0 && constraints.AllowSpot {
			iPricePerGPU = sorted[i].SpotPrice / float64(sorted[i].GPUsPerInstance)
		}
		jPricePerGPU := sorted[j].PricePerHour / float64(sorted[j].GPUsPerInstance)
		if sorted[j].SpotPrice > 0 && constraints.AllowSpot {
			jPricePerGPU = sorted[j].SpotPrice / float64(sorted[j].GPUsPerInstance)
		}
		return iPricePerGPU < jPricePerGPU
	})

	// Allocate greedily
	var allocation []models.Allocation
	remaining := requirements.GPUs

	for _, instance := range sorted {
		if remaining <= 0 {
			break
		}

		// Check per-instance constraints
		if requirements.MaxGPUsPerNode > 0 && instance.GPUsPerInstance > requirements.MaxGPUsPerNode {
			continue // Instance has too many GPUs per node
		}

		instancesNeeded := (remaining + instance.GPUsPerInstance - 1) / instance.GPUsPerInstance
		if instancesNeeded > 0 {
			// For multi-node training, check max nodes per cluster/AZ constraints
			if requirements.RequiresMultiNode {
				maxNodes := ao.getMaxNodesForProvider(instance.Provider, instance.Region)
				if instancesNeeded > maxNodes {
					// Can't allocate all in one region - skip this instance type
					continue
				}
			}

			useSpot := constraints.AllowSpot && instance.SpotPrice > 0
			price := instance.PricePerHour
			if useSpot {
				price = instance.SpotPrice
			}

			allocation = append(allocation, models.Allocation{
				Provider:      instance.Provider,
				InstanceType:  instance.InstanceType,
				Region:        instance.Region,
				Count:         instancesNeeded,
				Spot:          useSpot,
				PricePerHour:  price, // Store explicitly per instance
				EstimatedCost: price * float64(instancesNeeded) * requirements.EstimatedHours,
			})

			remaining -= instancesNeeded * instance.GPUsPerInstance
		}
	}

	// Check if allocation is complete
	if remaining > 0 {
		// Could not allocate all GPUs - return empty strategy (will be filtered by scoring)
		return Strategy{Allocation: []models.Allocation{}}
	}

	return Strategy{Allocation: allocation}
}

// filterMultiNodeCompatible filters instances compatible with multi-node training
func (ao *AllocationOptimizer) filterMultiNodeCompatible(
	candidates []models.GPUInstance,
	requirements models.JobRequirements,
) []models.GPUInstance {
	var filtered []models.GPUInstance

	for _, instance := range candidates {
		// MVP-safe rule: Multi-node training only supported for high-tier interconnect
		hasFastInterconnect := instance.InterconnectTier == models.InterconnectHigh

		// Check max nodes per AZ/cluster for this instance type
		maxNodes := ao.getMaxNodesForProvider(instance.Provider, instance.Region)
		minNodesNeeded := (requirements.GPUs + instance.GPUsPerInstance - 1) / instance.GPUsPerInstance

		if hasFastInterconnect && minNodesNeeded <= maxNodes {
			filtered = append(filtered, instance)
		}
	}

	return filtered
}

// parseRegionKey parses a region key (format: "provider:region") into provider and region
func parseRegionKey(key string) (models.Provider, string) {
	parts := strings.Split(key, ":")
	if len(parts) != 2 {
		return models.ProviderAWS, "us-east-1" // Default
	}
	return models.Provider(parts[0]), parts[1]
}

// getMaxNodesForProvider returns max nodes per cluster/AZ for provider+region
func (ao *AllocationOptimizer) getMaxNodesForProvider(provider models.Provider, region string) int {
	// Provider-specific limits (region can be used for region-specific quotas in future)
	_ = region // Reserved for future region-specific quota checks
	switch provider {
	case models.ProviderAWS:
		return 16 // Conservative limit per AZ
	case models.ProviderGCP:
		return 32 // Per region
	case models.ProviderAzure:
		return 16 // Per availability set
	case models.ProviderOnPrem:
		return 100 // K8s cluster can be large
	default:
		return 8 // Conservative default
	}
}

func (ao *AllocationOptimizer) reliableSingleRegionStrategy(
	candidates []models.GPUInstance,
	requirements models.JobRequirements,
	constraints models.JobConstraints,
) Strategy {
	// Phase 2: Prefer on-demand and on-premise for reliability
	// Filter out spot instances and prefer on-premise

	// Filter candidates: prefer on-demand and on-premise
	reliableCandidates := []models.GPUInstance{}
	for _, instance := range candidates {
		// Prefer on-premise
		if instance.Provider == models.ProviderOnPrem {
			reliableCandidates = append(reliableCandidates, instance)
			continue
		}
		// Prefer on-demand (no spot)
		// Note: We can't filter by spot here since GPUInstance doesn't have spot flag
		// But we can prefer instances with higher availability
		if instance.Availability >= 0.95 { // High availability = likely on-demand
			reliableCandidates = append(reliableCandidates, instance)
		}
	}

	// If no reliable candidates, fall back to all candidates
	if len(reliableCandidates) == 0 {
		reliableCandidates = candidates
	}

	// Use cheapest strategy but with reliable candidates
	// Group by provider+region (single-cluster requirement)
	regionGroups := make(map[string][]models.GPUInstance)
	for _, instance := range reliableCandidates {
		key := fmt.Sprintf("%s:%s", instance.Provider, instance.Region)
		regionGroups[key] = append(regionGroups[key], instance)
	}

	// Find cheapest reliable provider+region combination
	var bestStrategy Strategy
	bestCost := 999999.0

	for regionKey, instances := range regionGroups {
		regionStrategy := ao.cheapestStrategy(instances, requirements, constraints)
		if regionStrategy.TotalCost < bestCost && len(regionStrategy.Allocation) > 0 {
			bestCost = regionStrategy.TotalCost
			bestStrategy = regionStrategy

			// Verify all allocations are in same provider+region
			provider, region := parseRegionKey(regionKey)
			for _, alloc := range regionStrategy.Allocation {
				if alloc.Provider != provider || alloc.Region != region {
					bestCost = 999999.0
					break
				}
			}
		}
	}

	return bestStrategy
}

func (ao *AllocationOptimizer) dataLocalityStrategy(
	candidates []models.GPUInstance,
	requirements models.JobRequirements,
	constraints models.JobConstraints,
) Strategy {
	// Phase 2: Prefer region where dataset exists
	// Parse dataset URI to extract provider/region

	datasetProvider, datasetRegion := parseDatasetLocation(requirements.DatasetLocation)

	// Filter candidates to prefer dataset region
	preferredCandidates := []models.GPUInstance{}
	otherCandidates := []models.GPUInstance{}

	for _, instance := range candidates {
		// Exact match: same provider and region
		if instance.Provider == datasetProvider && instance.Region == datasetRegion {
			preferredCandidates = append(preferredCandidates, instance)
		} else if instance.Provider == datasetProvider {
			// Same provider, different region (still better than different provider)
			otherCandidates = append(otherCandidates, instance)
		} else {
			// Different provider (least preferred)
			otherCandidates = append(otherCandidates, instance)
		}
	}

	// Try preferred candidates first
	if len(preferredCandidates) > 0 {
		// Group by provider+region (single-cluster requirement)
		regionGroups := make(map[string][]models.GPUInstance)
		for _, instance := range preferredCandidates {
			key := fmt.Sprintf("%s:%s", instance.Provider, instance.Region)
			regionGroups[key] = append(regionGroups[key], instance)
		}

		// Find cheapest in preferred region
		var bestStrategy Strategy
		bestCost := 999999.0

		for _, instances := range regionGroups {
			regionStrategy := ao.cheapestStrategy(instances, requirements, constraints)
			if regionStrategy.TotalCost < bestCost && len(regionStrategy.Allocation) > 0 {
				bestCost = regionStrategy.TotalCost
				bestStrategy = regionStrategy
			}
		}

		if len(bestStrategy.Allocation) > 0 {
			return bestStrategy
		}
	}

	// Fall back to other candidates if preferred region doesn't work
	if len(otherCandidates) > 0 {
		return ao.cheapestSingleRegionStrategy(otherCandidates, requirements, constraints)
	}

	// Last resort: use all candidates
	return ao.cheapestSingleRegionStrategy(candidates, requirements, constraints)
}

// parseDatasetLocation extracts provider and region from dataset URI
// Supports: s3://bucket/path, gs://bucket/path, az://container/path, minio://endpoint/bucket/path
func parseDatasetLocation(uri string) (models.Provider, string) {
	// Phase 2: Parse URI to extract provider and region
	// For now, use simple parsing

	if len(uri) < 5 {
		return models.ProviderAWS, "us-east-1" // Default
	}

	scheme := uri[:5] // s3://, gs://, az://, minio://

	switch {
	case scheme == "s3://":
		// AWS S3 - default to us-east-1 (can be enhanced to detect bucket region)
		return models.ProviderAWS, "us-east-1"
	case scheme == "gs://":
		// GCP GCS - default to us-central1
		return models.ProviderGCP, "us-central1"
	case scheme == "az://":
		// Azure Blob - default to eastus
		return models.ProviderAzure, "eastus"
	case scheme == "minio":
		// MinIO (on-premise) - no specific region
		return models.ProviderOnPrem, ""
	default:
		// Default to AWS
		return models.ProviderAWS, "us-east-1"
	}
}

func (ao *AllocationOptimizer) scoreStrategies(
	strategies []Strategy,
	requirements models.JobRequirements,
	constraints models.JobConstraints,
) []Strategy {
	for i := range strategies {
		strategy := &strategies[i]

		// Calculate cost metrics
		totalCost, _ := ao.costCalculator.CalculateCost(
			strategy.Allocation,
			requirements.EstimatedHours,
		)
		strategy.TotalCost = totalCost

		// Calculate data transfer cost
		dataTransferCost := 0.0
		if requirements.DatasetLocation != "" {
			// Estimate transfer cost if dataset not in same region
			for _, alloc := range strategy.Allocation {
				transferCost := ao.costCalculator.CalculateDataTransferCost(
					100.0, // Estimate dataset size (should be from job config)
					parseProviderFromLocation(requirements.DatasetLocation),
					parseRegionFromLocation(requirements.DatasetLocation),
					alloc.Provider,
					alloc.Region,
				)
				dataTransferCost += transferCost
			}
		}

		// Calculate reliability
		spotCount := 0
		for _, alloc := range strategy.Allocation {
			if alloc.Spot {
				spotCount += alloc.Count
			}
		}
		// Simplified 10% interruption rate for spot instances
		strategy.Reliability = 1.0 - (float64(spotCount) / float64(len(strategy.Allocation)) * 0.1)

		// Calculate score (lower is better)
		costWeight := 1.0 - constraints.PerformanceWeight
		normalizedCost := (totalCost + dataTransferCost) / constraints.MaxBudget

		reliabilityPenalty := (1.0 - strategy.Reliability) * 0.2

		strategy.Score = costWeight*normalizedCost + reliabilityPenalty

		// Filter out strategies that don't meet constraints
		if (totalCost + dataTransferCost) > constraints.MaxBudget {
			strategy.Score = 999999 // Very bad score
		}
		if strategy.Reliability < constraints.MinReliability {
			strategy.Score = 999999
		}
	}

	// Sort by score (best first)
	sort.Slice(strategies, func(i, j int) bool {
		return strategies[i].Score < strategies[j].Score
	})

	return strategies
}

// Helper functions
// parseRegionKey is defined above (line 266)

func parseProviderFromLocation(location string) models.Provider {
	// Parse URI scheme: s3:// -> aws, gs:// -> gcp, az:// -> azure, minio:// -> onprem
	// TODO: Implement
	_ = location // Reserved for future parsing implementation
	return models.ProviderAWS
}

func parseRegionFromLocation(location string) string {
	// Parse region from URI
	// TODO: Implement
	_ = location // Reserved for future parsing implementation
	return "us-east-1"
}
