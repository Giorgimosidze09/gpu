package frameworks

import (
	"fmt"
	"strconv"
	"strings"

	"gpu-orchestrator/core/models"
)

// PyTorchSetup handles PyTorch DDP training setup
type PyTorchSetup struct{}

// DistributedConfig represents distributed training configuration
type DistributedConfig struct {
	Framework  string
	MasterAddr string
	MasterPort int
	WorldSize  int
	Nodes      []NodeConfig
}

// NodeConfig represents configuration for a single node
type NodeConfig struct {
	Rank        int
	Address     string
	GPUs        int
	Environment map[string]string
}

// SetupDistributedTraining sets up PyTorch DDP training within a single cluster
func (p *PyTorchSetup) SetupDistributedTraining(
	cluster *models.Cluster,
	job *models.Job,
) (*DistributedConfig, error) {
	// Validate cluster topology
	if err := p.validateClusterTopology(cluster); err != nil {
		return nil, fmt.Errorf("cluster topology validation failed: %w", err)
	}

	nodes := cluster.Nodes
	if len(nodes) == 0 {
		return nil, fmt.Errorf("cluster has no nodes")
	}

	// All nodes should be in same provider/region/VPC (validated above)
	config := &DistributedConfig{
		Framework:  "pytorch",
		MasterAddr: nodes[0].PrivateIP,
		MasterPort: 29500,
		WorldSize:  len(nodes),
		Nodes:      make([]NodeConfig, len(nodes)),
	}

	for i, node := range nodes {
		config.Nodes[i] = NodeConfig{
			Rank:        i,
			Address:     node.PrivateIP,
			GPUs:        node.GPUs,
			Environment: p.getEnvironment(job, i, len(nodes)),
		}
	}

	return config, nil
}

// validateClusterTopology ensures all nodes are in same provider+region+network
func (p *PyTorchSetup) validateClusterTopology(cluster *models.Cluster) error {
	nodes := cluster.Nodes
	if len(nodes) == 0 {
		return fmt.Errorf("empty cluster")
	}

	// Get first node's topology
	firstNode := nodes[0]
	expectedProvider := firstNode.Provider
	expectedRegion := firstNode.Region
	expectedVPC := firstNode.VPC

	// Check all other nodes match
	for i, node := range nodes {
		if node.Provider != expectedProvider {
			return fmt.Errorf("node %d has provider %s, expected %s", i, node.Provider, expectedProvider)
		}
		if node.Region != expectedRegion {
			return fmt.Errorf("node %d has region %s, expected %s", i, node.Region, expectedRegion)
		}
		if node.VPC != expectedVPC {
			return fmt.Errorf("node %d has VPC %s, expected %s", i, node.VPC, expectedVPC)
		}
	}

	return nil
}

// getEnvironment returns environment variables for a node
func (p *PyTorchSetup) getEnvironment(_ *models.Job, rank int, worldSize int) map[string]string {
	return map[string]string{
		"MASTER_ADDR":          "", // Will be set per node
		"MASTER_PORT":          "29500",
		"WORLD_SIZE":           strconv.Itoa(worldSize),
		"RANK":                 strconv.Itoa(rank),
		"NCCL_DEBUG":           "INFO",
		"NCCL_SOCKET_IFNAME":   "eth0",
		"CUDA_VISIBLE_DEVICES": "0,1,2,3,4,5,6,7", // TODO: Set based on actual GPUs
	}
}

// GenerateTrainingScript generates the training script wrapper
func (p *PyTorchSetup) GenerateTrainingScript(config *DistributedConfig, job *models.Job) string {
	// For single-node multi-GPU
	if config.WorldSize == 1 {
		return fmt.Sprintf(`#!/bin/bash
set -e

# Download training script from S3
aws s3 cp %s /tmp/train.py

# Set environment variables
export MASTER_ADDR=%s
export MASTER_PORT=%d
export WORLD_SIZE=%d
export RANK=0
export NCCL_DEBUG=INFO

# Launch training with torchrun (PyTorch 2.0+)
python -m torch.distributed.run \
    --nproc_per_node=%d \
    --nnodes=1 \
    --node_rank=0 \
    --master_addr=$MASTER_ADDR \
    --master_port=$MASTER_PORT \
    /tmp/train.py
`, job.EntrypointURI, config.MasterAddr, config.MasterPort, config.WorldSize, config.Nodes[0].GPUs)
	}

	// For multi-node
	var nodeScripts []string
	for i, node := range config.Nodes {
		script := fmt.Sprintf(`# Node %d (Rank %d)
export MASTER_ADDR=%s
export MASTER_PORT=%d
export WORLD_SIZE=%d
export RANK=%d
export NCCL_DEBUG=INFO

python -m torch.distributed.run \
    --nproc_per_node=%d \
    --nnodes=%d \
    --node_rank=%d \
    --master_addr=$MASTER_ADDR \
    --master_port=$MASTER_PORT \
    /tmp/train.py
`, i, node.Rank, config.MasterAddr, config.MasterPort, config.WorldSize, node.Rank, node.GPUs, config.WorldSize, node.Rank)
		nodeScripts = append(nodeScripts, script)
	}

	return fmt.Sprintf(`#!/bin/bash
set -e

# Download training script from S3
aws s3 cp %s /tmp/train.py

%s
`, job.EntrypointURI, strings.Join(nodeScripts, "\n\n"))
}
