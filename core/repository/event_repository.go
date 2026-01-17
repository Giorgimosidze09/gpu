package repository

import (
	"database/sql"
	"encoding/json"

	"gpu-orchestrator/core/models"
)

// EventRepository handles database operations for job events
type EventRepository struct {
	db *DB
}

// NewEventRepository creates a new event repository
func NewEventRepository(db *DB) *EventRepository {
	return &EventRepository{db: db}
}

// GetJobEvents retrieves events for a job
func (r *EventRepository) GetJobEvents(jobID string, limit int) ([]models.JobEvent, error) {
	query := `
		SELECT id, job_id, at, from_status, to_status, reason, meta_json
		FROM job_events
		WHERE job_id = $1
		ORDER BY at DESC
		LIMIT $2
	`

	rows, err := r.db.Query(query, jobID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []models.JobEvent
	for rows.Next() {
		var event models.JobEvent
		var fromStatus sql.NullString
		var metaJSON string

		err := rows.Scan(
			&event.ID,
			&event.JobID,
			&event.At,
			&fromStatus,
			&event.ToStatus,
			&event.Reason,
			&metaJSON,
		)
		if err != nil {
			continue
		}

		if fromStatus.Valid {
			status := models.JobStatus(fromStatus.String)
			event.FromStatus = &status
		}

		// Parse meta JSON
		if metaJSON != "" {
			json.Unmarshal([]byte(metaJSON), &event.MetaJSON)
		}

		events = append(events, event)
	}

	return events, nil
}
