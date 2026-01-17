package optimizer

import (
	"gpu-orchestrator/core/models"
)

// PerformanceMetricsStore provides performance benchmarks for different GPU/framework combinations
// Phase 1: Static benchmarks (MVP)
// Phase 2: Historical telemetry
// Phase 3: Per-customer profiles
type PerformanceMetricsStore struct {
	benchmarks map[string]models.PerformanceMetrics
}

// NewPerformanceMetricsStore creates a new performance metrics store
func NewPerformanceMetricsStore() *PerformanceMetricsStore {
	store := &PerformanceMetricsStore{
		benchmarks: make(map[string]models.PerformanceMetrics),
	}

	// Initialize with static benchmarks (MVP)
	store.initializeBenchmarks()

	return store
}

// initializeBenchmarks loads static benchmark data
func (pms *PerformanceMetricsStore) initializeBenchmarks() {
	// Key format: "framework:gpu_type:model_class"
	// Example: "pytorch:A100:resnet50"

	// PyTorch + A100 benchmarks
	pms.benchmarks["pytorch:A100:resnet50"] = models.PerformanceMetrics{
		StepsPerHour:      1200.0,
		StorageThroughput: 500.0, // MB/s
		NetworkBandwidth:  100.0, // Gbps
	}

	pms.benchmarks["pytorch:A100:bert"] = models.PerformanceMetrics{
		StepsPerHour:      800.0,
		StorageThroughput: 400.0,
		NetworkBandwidth:  100.0,
	}

	pms.benchmarks["pytorch:A100:llama"] = models.PerformanceMetrics{
		StepsPerHour:    200.0,
		TokensPerHour:   50000.0,
		StorageThroughput: 300.0,
		NetworkBandwidth: 100.0,
	}

	// PyTorch + V100 benchmarks
	pms.benchmarks["pytorch:V100:resnet50"] = models.PerformanceMetrics{
		StepsPerHour:      600.0,
		StorageThroughput: 300.0,
		NetworkBandwidth:  25.0,
	}

	// Horovod + A100 benchmarks
	pms.benchmarks["horovod:A100:resnet50"] = models.PerformanceMetrics{
		StepsPerHour:      1100.0,
		StorageThroughput: 450.0,
		NetworkBandwidth:  100.0,
	}
}

// GetPerformanceMetrics returns performance metrics for a framework+GPU combination
func (pms *PerformanceMetricsStore) GetPerformanceMetrics(framework, gpuType, modelClass string) models.PerformanceMetrics {
	key := framework + ":" + gpuType + ":" + modelClass
	if metrics, ok := pms.benchmarks[key]; ok {
		return metrics
	}
	// Return default/unknown metrics
	return models.PerformanceMetrics{
		StepsPerHour:      500.0, // Conservative default
		StorageThroughput: 200.0,
		NetworkBandwidth:  10.0,
	}
}

// GetPerformanceMetricsForAllocation returns performance metrics for an allocation
func (pms *PerformanceMetricsStore) GetPerformanceMetricsForAllocation(
	allocation []models.Allocation,
	framework string,
) models.PerformanceMetrics {
	// For Phase 1, use first instance's GPU type
	// Phase 2: Aggregate across all instances
	if len(allocation) == 0 {
		return models.PerformanceMetrics{}
	}

	// Extract GPU type from instance type (simplified for MVP)
	// TODO: Phase 2 - Map instance types to GPU types properly
	gpuType := "A100" // Default assumption
	modelClass := "resnet50" // Default assumption

	return pms.GetPerformanceMetrics(framework, gpuType, modelClass)
}

// GetBaselineCostPerStep returns baseline cost per step for comparison
func (pms *PerformanceMetricsStore) GetBaselineCostPerStep(framework, gpuType string) float64 {
	// Phase 1: Static baseline
	// Phase 2: Learn from historical data
	baselines := map[string]float64{
		"pytorch:A100": 0.001,
		"pytorch:V100": 0.002,
		"horovod:A100": 0.001,
	}
	key := framework + ":" + gpuType
	if baseline, ok := baselines[key]; ok {
		return baseline
	}
	return 0.002 // Conservative default
}

// GetBaselineStepsPerHour returns baseline steps per hour for comparison
func (pms *PerformanceMetricsStore) GetBaselineStepsPerHour(framework, gpuType string) float64 {
	// Phase 1: Static baseline
	// Phase 2: Learn from historical data
	baselines := map[string]float64{
		"pytorch:A100": 1000.0,
		"pytorch:V100": 500.0,
		"horovod:A100": 900.0,
	}
	key := framework + ":" + gpuType
	if baseline, ok := baselines[key]; ok {
		return baseline
	}
	return 500.0 // Conservative default
}
