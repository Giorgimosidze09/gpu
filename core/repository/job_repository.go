package repository

import (
	"database/sql"
	"fmt"
	"time"

	"gpu-orchestrator/core/models"

	"github.com/google/uuid"
)

// JobRepository handles database operations for jobs
type JobRepository struct {
	db *DB
}

// NewJobRepository creates a new job repository
func NewJobRepository(db *DB) *JobRepository {
	return &JobRepository{db: db}
}

// CreateJob creates a new job in the database
func (r *JobRepository) CreateJob(job *models.Job) error {
	query := `
		INSERT INTO jobs (
			id, user_id, name, team_id, project_id, job_type, framework, entrypoint_uri, dataset_uri,
			execution_mode, status, gpus, max_gpus_per_node, requires_multi_node,
			gpu_memory_gb, cpu_memory_gb, storage_gb, estimated_hours,
			locality, replication, budget_usd, deadline_at, allow_spot,
			min_reliability, performance_weight, spec_yaml, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16,
			$17, $18, $19, $20, $21, $22, $23, $24, $25, $26, $27, $28
		)
	`

	jobID := uuid.New()
	if job.ID != "" {
		var err error
		jobID, err = uuid.Parse(job.ID)
		if err != nil {
			return err
		}
	}

	var deadlineAt *time.Time
	if job.Constraints.Deadline != nil {
		deadlineAt = job.Constraints.Deadline
	}

	_, err := r.db.Exec(query,
		jobID,
		job.UserID,
		job.Name,
		job.TeamID,
		job.ProjectID,
		job.JobType,
		job.Framework,
		job.EntrypointURI,
		job.DatasetURI,
		job.Requirements.ExecutionMode,
		job.Status,
		job.Requirements.GPUs,
		job.Requirements.MaxGPUsPerNode,
		job.Requirements.RequiresMultiNode,
		job.Requirements.GPUMemory,
		job.Requirements.CPUMemory,
		job.Requirements.Storage,
		job.Requirements.EstimatedHours,
		job.Constraints.DataLocality,
		job.Constraints.ReplicationPolicy,
		job.Constraints.MaxBudget,
		deadlineAt,
		job.Constraints.AllowSpot,
		job.Constraints.MinReliability,
		job.Constraints.PerformanceWeight,
		job.SpecYAML,
		time.Now(),
		time.Now(),
	)

	if err != nil {
		return err
	}

	job.ID = jobID.String()
	job.CreatedAt = time.Now()

	// Create initial event
	return r.CreateJobEvent(job.ID, nil, job.Status, "job_created", nil)
}

// GetJob retrieves a job by ID
func (r *JobRepository) GetJob(id string) (*models.Job, error) {
	query := `
		SELECT id, user_id, name, team_id, project_id, job_type, framework, entrypoint_uri, dataset_uri,
			execution_mode, status, gpus, max_gpus_per_node, requires_multi_node,
			gpu_memory_gb, cpu_memory_gb, storage_gb, estimated_hours,
			locality, replication, budget_usd, deadline_at, allow_spot,
			min_reliability, performance_weight, selected_provider, selected_region,
			selected_backend, cluster_vpc, cluster_id, started_at, finished_at,
			cost_running_usd, cost_estimated_usd, spec_yaml, created_at, updated_at
		FROM jobs
		WHERE id = $1
	`

	var job models.Job
	var deadlineAt sql.NullTime
	var startedAt sql.NullTime
	var finishedAt sql.NullTime
	var selectedProvider sql.NullString
	var selectedRegion sql.NullString
	var selectedBackend sql.NullString
	var clusterID sql.NullString
	var costEstimatedUSD sql.NullFloat64

	var teamID sql.NullString
	var projectID sql.NullString

	err := r.db.QueryRow(query, id).Scan(
		&job.ID,
		&job.UserID,
		&job.Name,
		&teamID,
		&projectID,
		&job.JobType,
		&job.Framework,
		&job.EntrypointURI,
		&job.DatasetURI,
		&job.Requirements.ExecutionMode,
		&job.Status,
		&job.Requirements.GPUs,
		&job.Requirements.MaxGPUsPerNode,
		&job.Requirements.RequiresMultiNode,
		&job.Requirements.GPUMemory,
		&job.Requirements.CPUMemory,
		&job.Requirements.Storage,
		&job.Requirements.EstimatedHours,
		&job.Constraints.DataLocality,
		&job.Constraints.ReplicationPolicy,
		&job.Constraints.MaxBudget,
		&deadlineAt,
		&job.Constraints.AllowSpot,
		&job.Constraints.MinReliability,
		&job.Constraints.PerformanceWeight,
		&selectedProvider,
		&selectedRegion,
		&selectedBackend,
		&job.ClusterVPC,
		&clusterID,
		&startedAt,
		&finishedAt,
		&job.CostRunningUSD,
		&costEstimatedUSD,
		&job.SpecYAML,
		&job.CreatedAt,
		&job.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	if deadlineAt.Valid {
		job.Constraints.Deadline = &deadlineAt.Time
	}
	if startedAt.Valid {
		job.StartedAt = &startedAt.Time
	}
	if finishedAt.Valid {
		job.CompletedAt = &finishedAt.Time
	}
	if selectedProvider.Valid {
		provider := models.Provider(selectedProvider.String)
		job.SelectedProvider = &provider
	}
	if selectedRegion.Valid {
		job.SelectedRegion = selectedRegion.String
	}
	if selectedBackend.Valid {
		job.SelectedBackend = models.BackendType(selectedBackend.String)
	}
	if clusterID.Valid {
		job.ClusterID = &clusterID.String
	}
	if costEstimatedUSD.Valid {
		job.CostEstimatedUSD = &costEstimatedUSD.Float64
	}
	if teamID.Valid {
		job.TeamID = teamID.String
	}
	if projectID.Valid {
		job.ProjectID = projectID.String
	}

	return &job, nil
}

// UpdateJobStatus updates job status atomically with event logging
func (r *JobRepository) UpdateJobStatus(jobID string, fromStatus, toStatus models.JobStatus, reason string, meta map[string]interface{}) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Update job status
	updateQuery := `UPDATE jobs SET status = $1, updated_at = NOW() WHERE id = $2`
	_, err = tx.Exec(updateQuery, toStatus, jobID)
	if err != nil {
		return err
	}

	// Create event
	err = r.createJobEventTx(tx, jobID, &fromStatus, toStatus, reason, meta)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// CreateJobEvent creates a job event
func (r *JobRepository) CreateJobEvent(jobID string, fromStatus *models.JobStatus, toStatus models.JobStatus, reason string, meta map[string]interface{}) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	err = r.createJobEventTx(tx, jobID, fromStatus, toStatus, reason, meta)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (r *JobRepository) createJobEventTx(tx *sql.Tx, jobID string, fromStatus *models.JobStatus, toStatus models.JobStatus, reason string, meta map[string]interface{}) error {
	query := `
		INSERT INTO job_events (job_id, from_status, to_status, reason, meta_json)
		VALUES ($1, $2, $3, $4, $5)
	`

	var fromStatusStr *string
	if fromStatus != nil {
		s := string(*fromStatus)
		fromStatusStr = &s
	}

	// TODO: Serialize meta to JSON
	metaJSON := "{}"
	if meta != nil {
		// Use json.Marshal in real implementation
		metaJSON = "{}"
	}

	_, err := tx.Exec(query, jobID, fromStatusStr, toStatus, reason, metaJSON)
	return err
}

// ListJobs lists jobs with optional filters
func (r *JobRepository) ListJobs(userID string, status *models.JobStatus, limit int, cursor string) ([]*models.Job, string, error) {
	// TODO: Implement pagination with cursor
	query := `
		SELECT id, user_id, name, job_type, framework, status, created_at
		FROM jobs
		WHERE user_id = $1
	`
	args := []interface{}{userID}
	argIndex := 2

	if status != nil {
		query += fmt.Sprintf(" AND status = $%d", argIndex)
		args = append(args, *status)
		argIndex++
	}

	query += " ORDER BY created_at DESC LIMIT $%d"
	args = append(args, limit)
	query = fmt.Sprintf(query, argIndex)

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	var jobs []*models.Job
	for rows.Next() {
		var job models.Job
		err := rows.Scan(
			&job.ID,
			&job.UserID,
			&job.Name,
			&job.JobType,
			&job.Framework,
			&job.Status,
			&job.CreatedAt,
		)
		if err != nil {
			continue
		}
		jobs = append(jobs, &job)
	}

	// TODO: Calculate next cursor
	nextCursor := ""

	return jobs, nextCursor, nil
}

// UpdateJobCost updates the running cost for a job
func (r *JobRepository) UpdateJobCost(jobID string, cost float64) error {
	query := `UPDATE jobs SET cost_running_usd = $1, updated_at = NOW() WHERE id = $2`
	_, err := r.db.Exec(query, cost, jobID)
	return err
}
