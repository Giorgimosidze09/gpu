package gcp

import (
	"context"

	"gpu-orchestrator/core/models"
	// Phase 2: Uncomment when GCP credentials are configured
	// "google.golang.org/api/compute/v1"
	// "google.golang.org/api/option"
)

// Client is the GCP provider client
type Client struct {
	// computeService *compute.Service // Phase 2: Uncomment when GCP client is initialized
	projectID string
	regions   []string
}

// NewClient creates a new GCP client
func NewClient(ctx context.Context, projectID string, regions []string) (*Client, error) {
	// Phase 2: Initialize GCP Compute Service
	// TODO: Uncomment when GCP credentials are configured:
	// computeService, err := compute.NewService(ctx, option.WithScopes(compute.CloudPlatformScope))
	// if err != nil {
	// 	return nil, err
	// }

	return &Client{
		projectID: projectID,
		regions:   regions,
	}, nil
}

// GetGPUInstances returns available GPU instances (Phase 2: from GCP API)
func (c *Client) GetGPUInstances(ctx context.Context) ([]models.GPUInstance, error) {
	// Phase 2: Query GCP Compute Engine API for GPU instances
	// For now, return mock data
	return c.getMockGPUInstances(), nil
}

// FetchOnDemandPricing fetches on-demand pricing from GCP
func (c *Client) FetchOnDemandPricing(ctx context.Context) ([]models.GPUInstance, error) {
	// Phase 2: Query GCP Pricing API
	// TODO: Use computeService.MachineTypes.List() and Pricing API
	// For now, return mock data
	return c.getMockGPUInstances(), nil
}

// FetchPreemptiblePricing fetches preemptible pricing from GCP
func (c *Client) FetchPreemptiblePricing(ctx context.Context) ([]models.GPUInstance, error) {
	// Phase 2: GCP preemptible instances have fixed discount (~60-70%)
	instances := c.getMockGPUInstances()
	for i := range instances {
		instances[i].SpotPrice = instances[i].PricePerHour * 0.35 // 65% discount
		instances[i].Availability = 0.9                           // 90% availability (better than AWS spot)
	}
	return instances, nil
}

// getMockGPUInstances returns mock GPU instances for MVP
func (c *Client) getMockGPUInstances() []models.GPUInstance {
	gpuInstances := []struct {
		InstanceType     string
		GPUType          string
		GPUs             int
		Memory           int
		PricePerHour     float64
		InterconnectTier models.InterconnectTier
	}{
		{"a2-highgpu-1g", "A100", 1, 40, 3.67, models.InterconnectStandard},
		{"a2-highgpu-2g", "A100", 2, 80, 7.34, models.InterconnectStandard},
		{"a2-highgpu-4g", "A100", 4, 160, 14.68, models.InterconnectStandard},
		{"a2-highgpu-8g", "A100", 8, 320, 29.36, models.InterconnectHigh},
		{"n1-standard-4-k80", "K80", 4, 12, 1.50, models.InterconnectStandard},
	}

	var instances []models.GPUInstance
	for _, region := range c.regions {
		for _, gpu := range gpuInstances {
			instances = append(instances, models.GPUInstance{
				Provider:         models.ProviderGCP,
				InstanceType:     gpu.InstanceType,
				Region:           region,
				GPUType:          gpu.GPUType,
				GPUsPerInstance:  gpu.GPUs,
				MemoryPerGPU:     gpu.Memory,
				PricePerHour:     gpu.PricePerHour,
				InterconnectTier: gpu.InterconnectTier,
			})
		}
	}

	return instances
}
