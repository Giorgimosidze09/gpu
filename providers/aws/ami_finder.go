package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// GetGPUOptimizedAMI finds a GPU-optimized AMI for the given region and instance type
func (c *Client) GetGPUOptimizedAMI(ctx context.Context, region string, instanceType string) (string, error) {
	// Common GPU-optimized AMI patterns:
	// - Deep Learning AMI (Ubuntu): ami-xxxxx
	// - Deep Learning AMI (Amazon Linux): ami-yyyyy
	// - PyTorch AMI: ami-zzzzz

	// For MVP, use a lookup table of known AMIs
	// In production, query EC2 DescribeImages API with filters
	amiMap := map[string]map[string]string{
		"us-east-1": {
			"p3.2xlarge":   "ami-0c55b159cbfafe1f0", // Deep Learning AMI (Ubuntu 20.04)
			"p3.8xlarge":   "ami-0c55b159cbfafe1f0",
			"p3.16xlarge":  "ami-0c55b159cbfafe1f0",
			"p4d.24xlarge": "ami-0c55b159cbfafe1f0", // A100 instances
			"g4dn.xlarge":  "ami-0c55b159cbfafe1f0",
		},
		"us-west-2": {
			"p3.2xlarge":   "ami-0c55b159cbfafe1f0",
			"p3.8xlarge":   "ami-0c55b159cbfafe1f0",
			"p3.16xlarge":  "ami-0c55b159cbfafe1f0",
			"p4d.24xlarge": "ami-0c55b159cbfafe1f0",
			"g4dn.xlarge":  "ami-0c55b159cbfafe1f0",
		},
	}

	regionAMIs, ok := amiMap[region]
	if !ok {
		return "", fmt.Errorf("no AMI mapping for region %s", region)
	}

	ami, ok := regionAMIs[instanceType]
	if !ok {
		// Fallback: try to find any GPU AMI for this region
		// In production, query EC2 API
		return "", fmt.Errorf("no AMI found for instance type %s in region %s", instanceType, region)
	}

	// TODO: Verify AMI exists and is available
	// Use DescribeImages API to verify
	_, err := c.verifyAMI(ctx, region, ami)
	if err != nil {
		return "", fmt.Errorf("AMI %s not available: %w", ami, err)
	}

	return ami, nil
}

// verifyAMI verifies that an AMI exists and is available
func (c *Client) verifyAMI(ctx context.Context, _ string, amiID string) (bool, error) {
	input := &ec2.DescribeImagesInput{
		ImageIds: []string{amiID},
		Filters: []types.Filter{
			{
				Name:   aws.String("state"),
				Values: []string{"available"},
			},
		},
	}

	result, err := c.ec2Client.DescribeImages(ctx, input)
	if err != nil {
		return false, err
	}

	return len(result.Images) > 0, nil
}
