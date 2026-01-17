package frameworks

import (
	"fmt"
	"strconv"

	"gpu-orchestrator/core/models"
)

// HorovodSetup handles Horovod distributed training setup
// Phase 4: Full Horovod support
type HorovodSetup struct{}

// SetupDistributedTraining sets up Horovod distributed training
func (h *HorovodSetup) SetupDistributedTraining(
	cluster *models.Cluster,
	job *models.Job,
) (*DistributedConfig, error) {
	// Phase 4: Horovod distributed training setup
	// Horovod is framework-agnostic and works with PyTorch, TensorFlow, etc.

	// Validate cluster topology (same as PyTorch)
	if err := validateClusterTopology(cluster); err != nil {
		return nil, fmt.Errorf("cluster topology validation failed: %w", err)
	}

	if len(cluster.Nodes) == 0 {
		return nil, fmt.Errorf("cluster has no nodes")
	}

	// Horovod uses MPI for communication
	// Master node (rank 0) coordinates training
	config := &DistributedConfig{
		Framework:  "horovod",
		MasterAddr: cluster.Nodes[0].PrivateIP,
		MasterPort: 29500,
		WorldSize:  len(cluster.Nodes),
		Nodes:      make([]NodeConfig, len(cluster.Nodes)),
	}

	// Calculate total GPUs across all nodes
	totalGPUs := 0
	for _, node := range cluster.Nodes {
		totalGPUs += node.GPUs
	}

	// Setup each node
	for i, node := range cluster.Nodes {
		config.Nodes[i] = NodeConfig{
			Rank:        i,
			Address:     node.PrivateIP,
			GPUs:        node.GPUs,
			Environment: h.getEnvironment(job, i, len(cluster.Nodes), totalGPUs),
		}
	}

	return config, nil
}

// getEnvironment returns environment variables for Horovod
func (h *HorovodSetup) getEnvironment(
	job *models.Job,
	rank int,
	worldSize int,
	totalGPUs int,
) map[string]string {
	return map[string]string{
		"HOROVOD_RANK":              strconv.Itoa(rank),
		"HOROVOD_SIZE":              strconv.Itoa(worldSize),
		"HOROVOD_LOCAL_RANK":        "0", // Per-node local rank
		"HOROVOD_LOCAL_SIZE":        strconv.Itoa(worldSize),
		"HOROVOD_CROSS_RANK":        strconv.Itoa(rank),
		"HOROVOD_CROSS_SIZE":        strconv.Itoa(worldSize),
		"HOROVOD_HOSTNAME":          fmt.Sprintf("node-%d", rank),
		"HOROVOD_GPU_ALLREDUCE":     "nccl",
		"HOROVOD_GPU_BROADCAST":     "nccl",
		"HOROVOD_NCCL_HOME":         "/usr/local/nccl",
		"HOROVOD_NCCL_INCLUDE":      "/usr/local/nccl/include",
		"HOROVOD_NCCL_LIB":          "/usr/local/nccl/lib",
		"HOROVOD_NCCL_LINK":         "SHARED",
		"HOROVOD_WITH_PYTORCH":      "1",
		"HOROVOD_WITH_TENSORFLOW":   "1",
		"HOROVOD_WITHOUT_MXNET":     "1",
		"HOROVOD_WITHOUT_GLOO":      "1",
		"HOROVOD_CPU_OPERATIONS":    "gloo",
		"HOROVOD_NUM_GPUS":          strconv.Itoa(totalGPUs),
	}
}

// GenerateTrainingScript generates Horovod training script
func (h *HorovodSetup) GenerateTrainingScript(
	config *DistributedConfig,
	job *models.Job,
) string {
	// Phase 4: Generate Horovod run command
	// Horovod uses `horovodrun` command for distributed training

	script := `#!/bin/bash
# Auto-generated Horovod training script

# Set environment variables
`
	
	// Add environment variables
	for key, value := range config.Nodes[0].Environment {
		script += fmt.Sprintf("export %s=%s\n", key, value)
	}

	script += fmt.Sprintf(`
# Horovod hostfile (for multi-node)
HOSTFILE=/tmp/horovod_hostfile
cat > $HOSTFILE <<EOF
`)
	
	// Generate hostfile entries
	for _, node := range config.Nodes {
		script += fmt.Sprintf("%s slots=%d\n", node.Address, node.GPUs)
	}
	
	script += `EOF

# Calculate total processes
TOTAL_PROCESSES=0
for node in ` + config.Nodes[0].Address
	for i := 1; i < len(config.Nodes); i++ {
		script += " " + config.Nodes[i].Address
	}
	script += `; do
    TOTAL_PROCESSES=$((TOTAL_PROCESSES + ` + strconv.Itoa(config.Nodes[0].GPUs) + `))
done

# Run Horovod training
horovodrun \
    -np $TOTAL_PROCESSES \
    -H ` + config.MasterAddr + `:$TOTAL_PROCESSES \
    --hostfile $HOSTFILE \
    python ` + job.EntrypointURI + `
`

	return script
}

// GenerateElasticTrainingScript generates script for Horovod Elastic training
// Phase 4: Horovod Elastic allows dynamic scaling
func (h *HorovodSetup) GenerateElasticTrainingScript(
	config *DistributedConfig,
	job *models.Job,
	minWorkers int,
	maxWorkers int,
) string {
	// Phase 4: Horovod Elastic training script
	// Elastic training allows adding/removing workers dynamically
	
	script := `#!/bin/bash
# Auto-generated Horovod Elastic training script

# Horovod Elastic configuration
export HOROVOD_ELASTIC_MIN_WORKERS=` + strconv.Itoa(minWorkers) + `
export HOROVOD_ELASTIC_MAX_WORKERS=` + strconv.Itoa(maxWorkers) + `
export HOROVOD_ELASTIC_DISCOVERY_SCRIPT=/tmp/discovery.sh

# Discovery script for elastic training
cat > $HOROVOD_ELASTIC_DISCOVERY_SCRIPT <<'EOF'
#!/bin/bash
# Discovery script returns available hosts
`
	
	// Add hosts to discovery script
	for _, node := range config.Nodes {
		script += fmt.Sprintf("echo \"%s:%d\"\n", node.Address, node.GPUs)
	}
	
	script += `EOF
chmod +x $HOROVOD_ELASTIC_DISCOVERY_SCRIPT

# Run Horovod Elastic training
horovodrun \
    --elastic \
    --min-np ` + strconv.Itoa(minWorkers) + ` \
    --max-np ` + strconv.Itoa(maxWorkers) + ` \
    --discovery-script $HOROVOD_ELASTIC_DISCOVERY_SCRIPT \
    python ` + job.EntrypointURI + `
`

	return script
}
