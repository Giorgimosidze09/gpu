package frameworks

import (
	"fmt"
	"strconv"

	"gpu-orchestrator/core/models"
)

// TensorFlowSetup handles TensorFlow MultiWorkerMirroredStrategy setup
// Phase 4: TensorFlow distributed training support
type TensorFlowSetup struct{}

// SetupDistributedTraining sets up TensorFlow MultiWorkerMirroredStrategy
func (t *TensorFlowSetup) SetupDistributedTraining(
	cluster *models.Cluster,
	job *models.Job,
) (*DistributedConfig, error) {
	// Phase 4: TensorFlow MultiWorkerMirroredStrategy setup
	// TensorFlow uses TF_CONFIG environment variable for multi-worker setup

	// Validate cluster topology
	if err := validateClusterTopology(cluster); err != nil {
		return nil, fmt.Errorf("cluster topology validation failed: %w", err)
	}

	if len(cluster.Nodes) == 0 {
		return nil, fmt.Errorf("cluster has no nodes")
	}

	// Calculate total workers
	totalWorkers := 0
	for _, node := range cluster.Nodes {
		totalWorkers += node.GPUs // Each GPU is a worker
	}

	config := &DistributedConfig{
		Framework:  "tensorflow",
		MasterAddr: cluster.Nodes[0].PrivateIP,
		MasterPort: 2222, // TensorFlow default port
		WorldSize:  len(cluster.Nodes),
		Nodes:      make([]NodeConfig, len(cluster.Nodes)),
	}

	// Setup each node
	workerIndex := 0
	for i, node := range cluster.Nodes {
		config.Nodes[i] = NodeConfig{
			Rank:        i,
			Address:     node.PrivateIP,
			GPUs:        node.GPUs,
			Environment: t.getEnvironment(job, i, len(cluster.Nodes), workerIndex, totalWorkers),
		}
		workerIndex += node.GPUs
	}

	return config, nil
}

// getEnvironment returns environment variables for TensorFlow
func (t *TensorFlowSetup) getEnvironment(
	job *models.Job,
	taskIndex int,
	numTasks int,
	workerIndex int,
	totalWorkers int,
) map[string]string {
	// Phase 4: Generate TF_CONFIG JSON
	// TensorFlow uses TF_CONFIG environment variable with cluster and task info

	// Build cluster spec
	clusterSpec := `{"worker": [`
	for i := 0; i < numTasks; i++ {
		if i > 0 {
			clusterSpec += ","
		}
		clusterSpec += fmt.Sprintf(`"%s:2222"`, fmt.Sprintf("node-%d", i))
	}
	clusterSpec += `]}`

	// Build task spec
	taskSpec := fmt.Sprintf(`{"type": "worker", "index": %d}`, taskIndex)

	// TF_CONFIG JSON
	tfConfig := fmt.Sprintf(`{
  "cluster": %s,
  "task": %s,
  "environment": "cloud"
}`, clusterSpec, taskSpec)

	return map[string]string{
		"TF_CONFIG":                    tfConfig,
		"TF_CPP_MIN_LOG_LEVEL":         "0",
		"TF_FORCE_GPU_ALLOW_GROWTH":    "true",
		"TF_GPU_THREAD_MODE":           "gpu_private",
		"TF_GPU_THREAD_COUNT":          "2",
		"TF_NUM_INTEROP_THREADS":       strconv.Itoa(totalWorkers),
		"TF_NUM_INTRAOP_THREADS":       strconv.Itoa(totalWorkers),
		"TF_DISTRIBUTE_STRATEGY":       "MultiWorkerMirroredStrategy",
		"TF_USE_LEGACY_KERAS":          "0",
		"TF_ENABLE_ONEDNN_OPTS":        "1",
	}
}

// GenerateTrainingScript generates TensorFlow training script
func (t *TensorFlowSetup) GenerateTrainingScript(
	config *DistributedConfig,
	job *models.Job,
) string {
	// Phase 4: Generate TensorFlow training script
	// TensorFlow uses TF_CONFIG for multi-worker setup

	script := `#!/bin/bash
# Auto-generated TensorFlow MultiWorker training script

# Set environment variables
`
	
	// Add environment variables (TF_CONFIG is set per node)
	for key, value := range config.Nodes[0].Environment {
		if key != "TF_CONFIG" {
			script += fmt.Sprintf("export %s=%s\n", key, value)
		}
	}

	script += fmt.Sprintf(`
# TF_CONFIG is set per node (different for each worker)
# This script runs on each node with its own TF_CONFIG

# Run TensorFlow training
python %s
`, job.EntrypointURI)

	return script
}

// GenerateTFConfig generates TF_CONFIG JSON for a specific node
func (t *TensorFlowSetup) GenerateTFConfig(
	cluster *models.Cluster,
	taskIndex int,
) string {
	// Phase 4: Generate TF_CONFIG for specific task
	clusterSpec := `{"worker": [`
	for i := 0; i < len(cluster.Nodes); i++ {
		if i > 0 {
			clusterSpec += ","
		}
		clusterSpec += fmt.Sprintf(`"%s:2222"`, cluster.Nodes[i].PrivateIP)
	}
	clusterSpec += `]}`

	taskSpec := fmt.Sprintf(`{"type": "worker", "index": %d}`, taskIndex)

	return fmt.Sprintf(`{
  "cluster": %s,
  "task": %s,
  "environment": "cloud"
}`, clusterSpec, taskSpec)
}
