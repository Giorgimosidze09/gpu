package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// ProvisionGPUInstance provisions GPU instances on AWS
func (c *Client) ProvisionGPUInstance(
	ctx context.Context,
	instanceType string,
	region string,
	spot bool,
	count int,
) ([]string, error) { // Returns instance IDs
	// Get GPU-optimized AMI for this region and instance type
	amiID, err := c.GetGPUOptimizedAMI(ctx, region, instanceType)
	if err != nil {
		return nil, fmt.Errorf("failed to get GPU AMI: %w", err)
	}

	// Create EC2 instances
	input := &ec2.RunInstancesInput{
		ImageId:      aws.String(amiID),
		InstanceType: types.InstanceType(instanceType),
		MinCount:     aws.Int32(int32(count)),
		MaxCount:     aws.Int32(int32(count)),
		IamInstanceProfile: &types.IamInstanceProfileSpecification{
			Name: aws.String("gpu-instance-profile"), // TODO: Make configurable
		},
		UserData: aws.String(getUserDataScript()),
		TagSpecifications: []types.TagSpecification{
			{
				ResourceType: types.ResourceTypeInstance,
				Tags: []types.Tag{
					{
						Key:   aws.String("Name"),
						Value: aws.String(fmt.Sprintf("gpu-training-%s", instanceType)),
					},
					{
						Key:   aws.String("ManagedBy"),
						Value: aws.String("gpu-orchestrator"),
					},
				},
			},
		},
	}

	if spot {
		input.InstanceMarketOptions = &types.InstanceMarketOptionsRequest{
			MarketType: types.MarketTypeSpot,
			SpotOptions: &types.SpotMarketOptions{
				SpotInstanceType: types.SpotInstanceTypeOneTime,
				MaxPrice:         aws.String("0.50"), // TODO: Use dynamic max price
			},
		}
	}

	result, err := c.ec2Client.RunInstances(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to provision instances: %w", err)
	}

	instanceIDs := make([]string, len(result.Instances))
	for i, instance := range result.Instances {
		instanceIDs[i] = *instance.InstanceId
	}

	return instanceIDs, nil
}

// getUserDataScript returns the user data script for instance initialization
func getUserDataScript() string {
	return `#!/bin/bash
set -e

# Update system
apt-get update -y

# Install AWS CLI (if not present)
if ! command -v aws &> /dev/null; then
    apt-get install -y awscli
fi

# Install NVIDIA drivers (if not present)
if ! command -v nvidia-smi &> /dev/null; then
    # Drivers should be pre-installed in Deep Learning AMI
    # But we can verify and install if needed
    apt-get install -y nvidia-driver-470
fi

# Install PyTorch and dependencies
pip3 install torch torchvision torchaudio --index-url https://download.pytorch.org/whl/cu118

# Install other common ML libraries
pip3 install numpy pandas scikit-learn

# Create training directory
mkdir -p /opt/training
chmod 777 /opt/training

# Log completion
echo "Instance initialization complete" >> /var/log/user-data.log
`
}
