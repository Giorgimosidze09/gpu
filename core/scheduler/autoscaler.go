package scheduler

import (
	"context"
	"fmt"
	"log"
	"time"

	"gpu-orchestrator/core/resource_manager"
)

// AutoScaler automatically scales cluster pool based on demand
// Inspired by Cast AI's autoscaling approach
// Phase 2: Full implementation
type AutoScaler struct {
	clusterPool       *resource_manager.ClusterPool
	queue             *JobQueue
	scaleUpThreshold  int           // Number of pending jobs to trigger scale-up
	scaleDownIdleTime time.Duration // Idle time before scale-down
}

// NewAutoScaler creates a new autoscaler
func NewAutoScaler(
	clusterPool *resource_manager.ClusterPool,
	queue *JobQueue,
	scaleUpThreshold int,
	scaleDownIdleTime time.Duration,
) *AutoScaler {
	return &AutoScaler{
		clusterPool:       clusterPool,
		queue:             queue,
		scaleUpThreshold:  scaleUpThreshold,
		scaleDownIdleTime: scaleDownIdleTime,
	}
}

// Start starts the autoscaler background worker
func (as *AutoScaler) Start(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second) // Check every 30 seconds
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := as.CheckAndScale(ctx); err != nil {
				log.Printf("Autoscaler error: %v", err)
			}
		}
	}
}

// CheckAndScale checks queue depth and scales cluster pool accordingly
func (as *AutoScaler) CheckAndScale(ctx context.Context) error {
	queueDepth := as.queue.Len()

	// Scale up if queue depth exceeds threshold
	if queueDepth > as.scaleUpThreshold {
		demand := queueDepth - as.scaleUpThreshold
		log.Printf("Autoscaler: Queue depth %d exceeds threshold %d, scaling up by %d", queueDepth, as.scaleUpThreshold, demand)
		if err := as.clusterPool.ScaleUp(ctx, demand); err != nil {
			return fmt.Errorf("failed to scale up: %w", err)
		}
	}

	// Scale down idle clusters
	if err := as.clusterPool.ScaleDown(ctx, as.scaleDownIdleTime); err != nil {
		return fmt.Errorf("failed to scale down: %w", err)
	}

	return nil
}

// GetStatistics returns autoscaler statistics
func (as *AutoScaler) GetStatistics() map[string]interface{} {
	return map[string]interface{}{
		"queue_depth":                  as.queue.Len(),
		"scale_up_threshold":           as.scaleUpThreshold,
		"scale_down_idle_time_seconds": int(as.scaleDownIdleTime.Seconds()),
	}
}
