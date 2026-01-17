package repository

import (
	"time"

	"gpu-orchestrator/core/models"
)

// AllocationRepository handles database operations for allocations
type AllocationRepository struct {
	db *DB
}

// NewAllocationRepository creates a new allocation repository
func NewAllocationRepository(db *DB) *AllocationRepository {
	return &AllocationRepository{db: db}
}

// CreateAllocation creates an allocation record
func (r *AllocationRepository) CreateAllocation(jobID string, allocation models.Allocation) error {
	query := `
		INSERT INTO allocations (
			job_id, provider, region, backend, instance_type, count, spot,
			price_per_hour, estimated_hours, estimated_cost_usd
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10
		)
	`

	_, err := r.db.Exec(query,
		jobID,
		allocation.Provider,
		allocation.Region,
		models.BackendVM, // MVP uses VM backend
		allocation.InstanceType,
		allocation.Count,
		allocation.Spot,
		allocation.PricePerHour,
		allocation.EstimatedTime.Hours(),
		allocation.EstimatedCost,
	)

	return err
}

// GetAllocationsByJobID retrieves all allocations for a job
func (r *AllocationRepository) GetAllocationsByJobID(jobID string) ([]models.Allocation, error) {
	query := `
		SELECT provider, region, instance_type, count, spot,
			price_per_hour, estimated_hours, estimated_cost_usd
		FROM allocations
		WHERE job_id = $1
		ORDER BY created_at
	`

	rows, err := r.db.Query(query, jobID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var allocations []models.Allocation
	for rows.Next() {
		var alloc models.Allocation
		var estimatedHours float64

		err := rows.Scan(
			&alloc.Provider,
			&alloc.Region,
			&alloc.InstanceType,
			&alloc.Count,
			&alloc.Spot,
			&alloc.PricePerHour,
			&estimatedHours,
			&alloc.EstimatedCost,
		)
		if err != nil {
			continue
		}

		alloc.EstimatedTime = time.Duration(estimatedHours * float64(time.Hour))
		allocations = append(allocations, alloc)
	}

	return allocations, nil
}
