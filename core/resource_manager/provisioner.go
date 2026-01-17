package resource_manager

import (
	"context"
	"fmt"
	"log"
	"time"

	"gpu-orchestrator/core/models"
	"gpu-orchestrator/providers/aws"
	"gpu-orchestrator/providers/azure"
	"gpu-orchestrator/providers/gcp"
)

// Provisioner manages compute resource provisioning across providers
type Provisioner struct {
	awsClient   *aws.Client
	gcpClient   *gcp.Client
	azureClient *azure.Client
}

// NewProvisioner creates a new provisioner
func NewProvisioner(
	awsClient *aws.Client,
	gcpClient *gcp.Client,
	azureClient *azure.Client,
) *Provisioner {
	return &Provisioner{
		awsClient:   awsClient,
		gcpClient:   gcpClient,
		azureClient: azureClient,
	}
}

// ProvisionCluster provisions a cluster for a job
func (p *Provisioner) ProvisionCluster(
	ctx context.Context,
	job *models.Job,
	allocations []models.Allocation,
) (*models.Cluster, error) {
	if len(allocations) == 0 {
		return nil, fmt.Errorf("no allocations provided")
	}

	// For single-cluster mode, all allocations must be same provider+region
	// Validate this
	firstAlloc := allocations[0]
	for _, alloc := range allocations {
		if alloc.Provider != firstAlloc.Provider || alloc.Region != firstAlloc.Region {
			return nil, fmt.Errorf("single-cluster mode requires all allocations in same provider+region")
		}
	}

	// Provision instances based on provider
	var nodes []models.Node
	var instanceIDs []string
	var err error

	switch firstAlloc.Provider {
	case models.ProviderAWS:
		instanceIDs, err = p.provisionAWS(ctx, allocations)
	case models.ProviderGCP:
		instanceIDs, err = p.provisionGCP(ctx, allocations)
	case models.ProviderAzure:
		instanceIDs, err = p.provisionAzure(ctx, allocations)
	case models.ProviderOnPrem:
		return nil, fmt.Errorf("on-premise provisioning not yet implemented")
	default:
		return nil, fmt.Errorf("unsupported provider: %s", firstAlloc.Provider)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to provision instances: %w", err)
	}

	// Wait for instances to be ready
	log.Printf("Waiting for %d instances to be ready...", len(instanceIDs))
	time.Sleep(30 * time.Second) // TODO: Implement proper instance readiness check

	// Build cluster and nodes
	cluster := &models.Cluster{
		ID:       fmt.Sprintf("cluster-%s", job.ID),
		Provider: firstAlloc.Provider,
		Region:   firstAlloc.Region,
		VPC:      "default", // TODO: Get actual VPC
		Backend:  models.BackendVM,
		Nodes:    nodes,
	}

	// Create nodes from instance IDs
	for i, instanceID := range instanceIDs {
		alloc := allocations[0] // For now, assume single allocation type
		node := models.Node{
			ID:         fmt.Sprintf("node-%s-%d", job.ID, i),
			InstanceID: instanceID,
			Provider:   firstAlloc.Provider,
			Region:     firstAlloc.Region,
			VPC:        cluster.VPC,
			PrivateIP:  fmt.Sprintf("10.0.1.%d", i+10), // TODO: Get actual private IP
			GPUs:       alloc.Count * 8,                // TODO: Get actual GPU count from instance type
		}
		cluster.Nodes = append(cluster.Nodes, node)
	}

	return cluster, nil
}

// provisionAWS provisions AWS EC2 instances
func (p *Provisioner) provisionAWS(ctx context.Context, allocations []models.Allocation) ([]string, error) {
	if p.awsClient == nil {
		return nil, fmt.Errorf("AWS client not initialized")
	}

	var allInstanceIDs []string

	for _, alloc := range allocations {
		instanceIDs, err := p.awsClient.ProvisionGPUInstance(
			ctx,
			alloc.InstanceType,
			alloc.Region,
			alloc.Spot,
			alloc.Count,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to provision AWS instances: %w", err)
		}
		allInstanceIDs = append(allInstanceIDs, instanceIDs...)
	}

	return allInstanceIDs, nil
}

// provisionGCP provisions GCP instances
func (p *Provisioner) provisionGCP(_ context.Context, _ []models.Allocation) ([]string, error) {
	if p.gcpClient == nil {
		return nil, fmt.Errorf("GCP client not initialized")
	}

	// TODO: Implement GCP provisioning
	return nil, fmt.Errorf("GCP provisioning not yet implemented")
}

// provisionAzure provisions Azure instances
func (p *Provisioner) provisionAzure(_ context.Context, _ []models.Allocation) ([]string, error) {
	if p.azureClient == nil {
		return nil, fmt.Errorf("Azure client not initialized")
	}

	// TODO: Implement Azure provisioning
	return nil, fmt.Errorf("Azure provisioning not yet implemented")
}

// TerminateCluster terminates all instances in a cluster
func (p *Provisioner) TerminateCluster(ctx context.Context, cluster *models.Cluster) error {
	// TODO: Implement termination logic
	return nil
}
