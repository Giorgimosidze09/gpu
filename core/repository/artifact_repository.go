package repository

import (
	"encoding/json"
	"fmt"

	"gpu-orchestrator/core/models"
)

// ArtifactRepository handles database operations for job artifacts
type ArtifactRepository struct {
	db *DB
}

// NewArtifactRepository creates a new artifact repository
func NewArtifactRepository(db *DB) *ArtifactRepository {
	return &ArtifactRepository{db: db}
}

// GetJobArtifacts retrieves artifacts for a job
func (r *ArtifactRepository) GetJobArtifacts(jobID string, artifactType *models.ArtifactType) ([]models.JobArtifact, error) {
	query := `
		SELECT id, job_id, type, uri, created_at, meta_json
		FROM job_artifacts
		WHERE job_id = $1
	`
	args := []interface{}{jobID}
	argIndex := 2

	if artifactType != nil {
		query += fmt.Sprintf(" AND type = $%d", argIndex)
		args = append(args, *artifactType)
		argIndex++
	}

	query += " ORDER BY created_at DESC"

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var artifacts []models.JobArtifact
	for rows.Next() {
		var artifact models.JobArtifact
		var metaJSON string

		err := rows.Scan(
			&artifact.ID,
			&artifact.JobID,
			&artifact.Type,
			&artifact.URI,
			&artifact.CreatedAt,
			&metaJSON,
		)
		if err != nil {
			continue
		}

		// Parse meta JSON
		if metaJSON != "" {
			json.Unmarshal([]byte(metaJSON), &artifact.MetaJSON)
		}

		artifacts = append(artifacts, artifact)
	}

	return artifacts, nil
}

// CreateArtifact creates a new artifact record
func (r *ArtifactRepository) CreateArtifact(jobID string, artifactType models.ArtifactType, uri string, meta map[string]interface{}) error {
	metaJSON := "{}"
	if meta != nil {
		metaBytes, err := json.Marshal(meta)
		if err == nil {
			metaJSON = string(metaBytes)
		}
	}

	query := `
		INSERT INTO job_artifacts (job_id, type, uri, meta_json, created_at)
		VALUES ($1, $2, $3, $4, NOW())
	`

	_, err := r.db.Exec(query, jobID, artifactType, uri, metaJSON)
	return err
}
