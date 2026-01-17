package optimizer

import (
	"gpu-orchestrator/core/models"
)

// CostCalculator calculates costs for allocations
type CostCalculator struct {
	pricingFetcher *PricingFetcher
}

// NewCostCalculator creates a new cost calculator
func NewCostCalculator(pf *PricingFetcher) *CostCalculator {
	return &CostCalculator{
		pricingFetcher: pf,
	}
}

// CalculateCost calculates total cost for an allocation
func (cc *CostCalculator) CalculateCost(allocation []models.Allocation, estimatedHours float64) (float64, error) {
	totalCost := 0.0

	for _, alloc := range allocation {
		// PricePerHour is explicitly stored per instance
		cost := alloc.PricePerHour * float64(alloc.Count) * estimatedHours
		totalCost += cost
	}

	return totalCost, nil
}

// CalculateCostWithReliability calculates cost with spot instance interruption probability
func (cc *CostCalculator) CalculateCostWithReliability(
	allocation []models.Allocation,
	estimatedHours float64,
	spotInterruptionRate float64, // e.g., 0.1 = 10% chance of interruption
) (float64, float64) {
	baseCost, _ := cc.CalculateCost(allocation, estimatedHours)

	// Calculate expected cost including restarts
	spotInstances := 0
	for _, alloc := range allocation {
		if alloc.Spot {
			spotInstances += alloc.Count
		}
	}

	// Expected interruptions = hours × interruption_rate
	expectedInterruptions := estimatedHours * spotInterruptionRate

	// Each interruption adds ~10 minutes overhead (restart time)
	overheadHours := expectedInterruptions * (10.0 / 60.0)

	// Recalculate cost with overhead
	totalCost := baseCost * (1 + overheadHours/estimatedHours)
	reliability := 1.0 - (expectedInterruptions / estimatedHours)
	if reliability < 0 {
		reliability = 0
	}

	return totalCost, reliability
}

// CalculateCostPerStep calculates cost per training step
func (cc *CostCalculator) CalculateCostPerStep(
	allocation []models.Allocation,
	metrics models.PerformanceMetrics,
) (float64, error) {
	if metrics.StepsPerHour == 0 {
		return 0, nil
	}

	// Calculate hourly cost
	hourlyCost := 0.0
	for _, alloc := range allocation {
		hourlyCost += alloc.PricePerHour * float64(alloc.Count)
	}

	// Cost per step = hourly cost / steps per hour
	return hourlyCost / metrics.StepsPerHour, nil
}

// CalculateDataTransferCost calculates data transfer cost between regions
func (cc *CostCalculator) CalculateDataTransferCost(
	dataSizeGB float64,
	sourceProvider models.Provider,
	sourceRegion string,
	targetProvider models.Provider,
	targetRegion string,
) float64 {
	// If same provider and region, no transfer cost
	if sourceProvider == targetProvider && sourceRegion == targetRegion {
		return 0.0
	}

	// TODO: Implement provider-specific egress pricing
	// AWS: ~$0.09/GB for first 10TB
	// GCP: ~$0.12/GB for first 10TB
	// Azure: ~$0.087/GB for first 10TB

	// Simplified: Use average egress cost
	egressCostPerGB := 0.10 // $0.10 per GB
	return dataSizeGB * egressCostPerGB
}
