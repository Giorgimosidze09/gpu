package storage

import (
	"context"
	"fmt"
	"time"

	"gpu-orchestrator/core/models"
	"gpu-orchestrator/core/repository"
)

// CheckpointManager manages checkpoint storage and retrieval
type CheckpointManager struct {
	artifactRepo *repository.ArtifactRepository
}

// NewCheckpointManager creates a new checkpoint manager
func NewCheckpointManager(artifactRepo *repository.ArtifactRepository) *CheckpointManager {
	return &CheckpointManager{
		artifactRepo: artifactRepo,
	}
}

// SaveCheckpoint saves a checkpoint URI to the database
func (cm *CheckpointManager) SaveCheckpoint(
	ctx context.Context,
	jobID string,
	checkpointURI string,
	step int,
	metadata map[string]interface{},
) error {
	// Store checkpoint artifact
	meta := map[string]interface{}{
		"step": step,
		"uri":  checkpointURI,
	}
	// Merge with provided metadata
	for k, v := range metadata {
		meta[k] = v
	}

	return cm.artifactRepo.CreateArtifact(
		jobID,
		models.ArtifactTypeCheckpoint,
		checkpointURI,
		meta,
	)
}

// GetLatestCheckpoint retrieves the latest checkpoint for a job
func (cm *CheckpointManager) GetLatestCheckpoint(ctx context.Context, jobID string) (string, error) {
	artifacts, err := cm.artifactRepo.GetJobArtifacts(jobID, nil)
	if err != nil {
		return "", err
	}

	var latestCheckpoint string
	var latestStep int = -1
	latestTime := time.Time{}

	for _, artifact := range artifacts {
		if artifact.Type != models.ArtifactTypeCheckpoint {
			continue
		}

		// Extract step from metadata
		step, ok := artifact.MetaJSON["step"].(float64)
		if !ok {
			// Fallback to time-based selection
			if artifact.CreatedAt.After(latestTime) {
				latestTime = artifact.CreatedAt
				latestCheckpoint = artifact.URI
			}
			continue
		}

		if int(step) > latestStep {
			latestStep = int(step)
			latestCheckpoint = artifact.URI
		}
	}

	if latestCheckpoint == "" {
		return "", fmt.Errorf("no checkpoint found for job %s", jobID)
	}

	return latestCheckpoint, nil
}

// ListCheckpoints lists all checkpoints for a job
func (cm *CheckpointManager) ListCheckpoints(ctx context.Context, jobID string) ([]models.JobArtifact, error) {
	checkpointType := models.ArtifactTypeCheckpoint
	return cm.artifactRepo.GetJobArtifacts(jobID, &checkpointType)
}
