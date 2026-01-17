package resource_manager

import (
	"context"
	"fmt"
	"sync"
	"time"

	"gpu-orchestrator/core/models"
)

// ClusterPool manages a pool of GPU clusters for reuse across jobs
// This improves utilization and reduces provisioning overhead (inspired by Cast AI)
// Phase 2: Full implementation
type ClusterPool struct {
	clusters map[string]*ClusterInfo
	mu       sync.RWMutex
	minSize  int
	maxSize  int
}

// ClusterInfo tracks cluster state and utilization
type ClusterInfo struct {
	Cluster       *models.Cluster
	CreatedAt     time.Time
	LastUsedAt    time.Time
	ActiveJobs    int
	TotalGPUs     int
	AvailableGPUs int
}

// NewClusterPool creates a new cluster pool
func NewClusterPool(minSize, maxSize int) *ClusterPool {
	return &ClusterPool{
		clusters: make(map[string]*ClusterInfo),
		minSize:  minSize,
		maxSize:  maxSize,
	}
}

// GetBestCluster returns the best cluster for the given requirements
func (cp *ClusterPool) GetBestCluster(requirements models.JobRequirements) *models.Cluster {
	cp.mu.RLock()
	defer cp.mu.RUnlock()

	var bestCluster *models.Cluster
	bestScore := 0.0

	for _, info := range cp.clusters {
		// Skip if cluster doesn't have enough GPUs
		if info.AvailableGPUs < requirements.GPUs {
			continue
		}

		// Score based on available GPUs and last used time
		// Prefer clusters with more available GPUs and recent usage
		utilization := float64(info.AvailableGPUs) / float64(info.TotalGPUs)
		ageScore := 1.0 / (1.0 + time.Since(info.LastUsedAt).Hours())
		score := utilization * ageScore

		if score > bestScore {
			bestScore = score
			bestCluster = info.Cluster
		}
	}

	return bestCluster
}

// ScaleUp scales up the cluster pool by adding new clusters
func (cp *ClusterPool) ScaleUp(ctx context.Context, demand int) error {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	// Check if we're at max size
	if len(cp.clusters) >= cp.maxSize {
		return fmt.Errorf("cluster pool at max size %d", cp.maxSize)
	}

	// Calculate how many clusters to add
	clustersToAdd := demand / 8 // Assume 8 GPUs per cluster (placeholder)
	if clustersToAdd == 0 {
		clustersToAdd = 1
	}

	// Don't exceed max size
	if len(cp.clusters)+clustersToAdd > cp.maxSize {
		clustersToAdd = cp.maxSize - len(cp.clusters)
	}

	// Phase 2: Actually provision clusters using provisioner
	// For now, create placeholder clusters (will be replaced with real provisioning)
	for i := 0; i < clustersToAdd; i++ {
		clusterID := fmt.Sprintf("cluster-%d-%d", time.Now().Unix(), i)

		// TODO: Phase 2 - Use provisioner to create real clusters
		// This would call:
		// - provisioner.ProvisionGPUInstances(ctx, allocations)
		// - Create cluster with real instance IDs
		// - Track actual GPU counts

		cp.clusters[clusterID] = &ClusterInfo{
			Cluster: &models.Cluster{
				ID:       clusterID,
				Provider: models.ProviderAWS, // Placeholder - should come from provisioner
				Region:   "us-east-1",        // Placeholder - should come from provisioner
				Backend:  models.BackendVM,
				Nodes:    []models.Node{}, // Will be populated by provisioner
			},
			CreatedAt:     time.Now(),
			LastUsedAt:    time.Now(),
			TotalGPUs:     8, // Placeholder - should come from actual instances
			AvailableGPUs: 8, // Placeholder - should come from actual instances
		}
	}

	return nil
}

// ScaleDown scales down idle clusters
func (cp *ClusterPool) ScaleDown(ctx context.Context, idleTime time.Duration) error {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	// Don't scale below min size
	if len(cp.clusters) <= cp.minSize {
		return nil
	}

	now := time.Now()
	var toRemove []string

	for id, info := range cp.clusters {
		// Check if cluster is idle (no active jobs and not used recently)
		if info.ActiveJobs == 0 && now.Sub(info.LastUsedAt) > idleTime {
			toRemove = append(toRemove, id)
		}
	}

	// Remove idle clusters (but keep at least minSize)
	removeCount := len(toRemove)
	if len(cp.clusters)-removeCount < cp.minSize {
		removeCount = len(cp.clusters) - cp.minSize
	}

	for i := 0; i < removeCount; i++ {
		// Phase 2: Actually terminate cluster instances
		// TODO: Use provisioner to terminate instances
		// This would call:
		// - provisioner.TerminateInstances(ctx, cluster.Node.InstanceIDs)
		// - Wait for termination
		// - Then delete from pool

		delete(cp.clusters, toRemove[i])
	}

	return nil
}

// ReserveGPUs reserves GPUs on a cluster
func (cp *ClusterPool) ReserveGPUs(clusterID string, gpus int) error {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	info, ok := cp.clusters[clusterID]
	if !ok {
		return fmt.Errorf("cluster %s not found", clusterID)
	}

	if info.AvailableGPUs < gpus {
		return fmt.Errorf("not enough GPUs available: need %d, have %d", gpus, info.AvailableGPUs)
	}

	info.AvailableGPUs -= gpus
	info.ActiveJobs++
	info.LastUsedAt = time.Now()

	return nil
}

// ReleaseGPUs releases GPUs from a cluster
func (cp *ClusterPool) ReleaseGPUs(clusterID string, gpus int) error {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	info, ok := cp.clusters[clusterID]
	if !ok {
		return fmt.Errorf("cluster %s not found", clusterID)
	}

	info.AvailableGPUs += gpus
	info.ActiveJobs--
	if info.ActiveJobs < 0 {
		info.ActiveJobs = 0
	}

	return nil
}

// GetStatistics returns cluster pool statistics
func (cp *ClusterPool) GetStatistics() map[string]interface{} {
	cp.mu.RLock()
	defer cp.mu.RUnlock()

	totalGPUs := 0
	availableGPUs := 0
	activeJobs := 0

	for _, info := range cp.clusters {
		totalGPUs += info.TotalGPUs
		availableGPUs += info.AvailableGPUs
		activeJobs += info.ActiveJobs
	}

	return map[string]interface{}{
		"total_clusters": len(cp.clusters),
		"min_size":       cp.minSize,
		"max_size":       cp.maxSize,
		"total_gpus":     totalGPUs,
		"available_gpus": availableGPUs,
		"active_jobs":    activeJobs,
		"utilization":    float64(totalGPUs-availableGPUs) / float64(totalGPUs),
	}
}
