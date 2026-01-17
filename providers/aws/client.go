package aws

import (
	"context"

	"gpu-orchestrator/core/models"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/pricing"
)

// Client is the AWS provider client
type Client struct {
	ec2Client     *ec2.Client
	pricingClient *pricing.Client
	regions       []string
}

// NewClient creates a new AWS client
func NewClient(ctx context.Context, regions []string) (*Client, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, err
	}

	return &Client{
		ec2Client:     ec2.NewFromConfig(cfg),
		pricingClient: pricing.NewFromConfig(cfg),
		regions:       regions,
	}, nil
}

// FetchOnDemandPricing fetches on-demand pricing from AWS
func (c *Client) FetchOnDemandPricing(ctx context.Context) ([]models.GPUInstance, error) {
	// TODO: Implement AWS Pricing API calls
	// For MVP, return mock data
	return c.getMockGPUInstances(), nil
}

// FetchSpotPricing fetches spot pricing from AWS
func (c *Client) FetchSpotPricing(ctx context.Context) ([]models.GPUInstance, error) {
	// TODO: Implement EC2 Spot Price History API
	// For MVP, return mock data with spot prices
	instances := c.getMockGPUInstances()
	for i := range instances {
		instances[i].SpotPrice = instances[i].PricePerHour * 0.3 // 70% discount
		instances[i].Availability = 0.8                          // 80% availability
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
		{"p3.2xlarge", "V100", 1, 16, 3.06, models.InterconnectStandard},
		{"p3.8xlarge", "V100", 4, 64, 12.24, models.InterconnectStandard},
		{"p3.16xlarge", "V100", 8, 128, 24.48, models.InterconnectStandard},
		{"p4d.24xlarge", "A100", 8, 320, 32.77, models.InterconnectHigh},
		{"g4dn.xlarge", "T4", 1, 16, 0.526, models.InterconnectStandard},
	}

	var instances []models.GPUInstance
	for _, region := range c.regions {
		for _, gpu := range gpuInstances {
			instances = append(instances, models.GPUInstance{
				Provider:         models.ProviderAWS,
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
