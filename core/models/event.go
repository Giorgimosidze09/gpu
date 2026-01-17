package models

import "time"

// JobEvent represents a state transition event for a job
type JobEvent struct {
	ID         int64
	JobID      string
	At         time.Time
	FromStatus *JobStatus
	ToStatus   JobStatus
	Reason     string
	MetaJSON   map[string]interface{} // Additional metadata
}

// ArtifactType represents the type of job artifact
type ArtifactType string

const (
	ArtifactTypeCheckpoint ArtifactType = "checkpoint"
	ArtifactTypeLog        ArtifactType = "log"
	ArtifactTypeOutput     ArtifactType = "output"
	ArtifactTypeMetrics    ArtifactType = "metrics"
)

// JobArtifact represents a job artifact (checkpoint, log, output, etc.)
type JobArtifact struct {
	ID        int64
	JobID     string
	Type      ArtifactType
	URI       string
	CreatedAt time.Time
	MetaJSON  map[string]interface{}
}
