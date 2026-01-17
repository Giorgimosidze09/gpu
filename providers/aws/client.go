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

// FetchOnDemandPricing fetches on-demand pricing from AWS Pricing API
func (c *Client) FetchOnDemandPricing(ctx context.Context) ([]models.GPUInstance, error) {
	// Phase 2: Real AWS Pricing API implementation
	// For now, use mock data but structure is ready for real API calls
	instances := c.getMockGPUInstances()

	// TODO: Phase 2 - Replace with real Pricing API calls:
	// 1. Use pricingClient.GetProducts() with filters:
	//    - ServiceCode: "AmazonEC2"
	//    - Filters: instanceType, location (region)
	// 2. Parse response to extract OnDemand pricing
	// 3. Map to GPUInstance struct

	return instances, nil
}

// FetchSpotPricing fetches spot pricing from EC2 Spot Price History API
func (c *Client) FetchSpotPricing(ctx context.Context) ([]models.GPUInstance, error) {
	// Phase 2: Real EC2 Spot Price History API implementation
	instances := c.getMockGPUInstances()

	// TODO: Phase 2 - Replace with real Spot Price History API:
	// 1. Use ec2Client.DescribeSpotPriceHistory() with:
	//    - InstanceTypes: all GPU instance types
	//    - ProductDescriptions: ["Linux/UNIX"]
	//    - MaxResults: 1000
	// 2. Group by instance type and availability zone
	// 3. Calculate average spot price and availability
	// 4. Map to GPUInstance with SpotPrice and Availability

	for i := range instances {
		// Calculate realistic spot pricing (60-90% discount)
		spotDiscount := 0.3 // 70% discount average
		instances[i].SpotPrice = instances[i].PricePerHour * spotDiscount
		instances[i].Availability = 0.8 // 80% availability estimate
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
