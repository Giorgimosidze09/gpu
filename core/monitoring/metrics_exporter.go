package monitoring

import (
	"context"
	"fmt"

	"gpu-orchestrator/core/models"
	"gpu-orchestrator/core/repository"
)

// MetricsExporter exports metrics for Prometheus/Grafana
// Enables cost visibility dashboards (like Run:AI/Cast AI)
type MetricsExporter struct {
	jobRepo     *repository.JobRepository
	costTracker *CostTracker
}

// NewMetricsExporter creates a new metrics exporter
func NewMetricsExporter(jobRepo *repository.JobRepository, costTracker *CostTracker) *MetricsExporter {
	return &MetricsExporter{
		jobRepo:     jobRepo,
		costTracker: costTracker,
	}
}

// GetPrometheusMetrics returns metrics in Prometheus format
func (me *MetricsExporter) GetPrometheusMetrics() string {
	// Get all running jobs
	status := models.JobStatusRunning
	jobs, _, err := me.jobRepo.ListJobs("", &status, 1000, "")
	if err != nil {
		return ""
	}

	var metrics string

	// Job count metrics
	metrics += "# HELP gpu_jobs_total Total number of jobs\n"
	metrics += "# TYPE gpu_jobs_total counter\n"
	metrics += fmt.Sprintf("gpu_jobs_total %d\n", len(jobs))

	// Cost metrics
	totalCost := 0.0
	for _, job := range jobs {
		cost := me.costTracker.GetRunningCost(job.ID)
		totalCost += cost

		// Per-job cost
		metrics += fmt.Sprintf("gpu_job_cost_usd{job_id=\"%s\",user_id=\"%s\",team_id=\"%s\",project_id=\"%s\"} %.4f\n",
			job.ID, job.UserID, job.TeamID, job.ProjectID, cost)
	}

	// Total cost
	metrics += "# HELP gpu_total_cost_usd Total cost of all running jobs\n"
	metrics += "# TYPE gpu_total_cost_usd gauge\n"
	metrics += fmt.Sprintf("gpu_total_cost_usd %.4f\n", totalCost)

	// Team cost breakdown
	teamCosts := make(map[string]float64)
	for _, job := range jobs {
		if job.TeamID != "" {
			teamCosts[job.TeamID] += me.costTracker.GetRunningCost(job.ID)
		}
	}

	metrics += "# HELP gpu_team_cost_usd Cost per team\n"
	metrics += "# TYPE gpu_team_cost_usd gauge\n"
	for teamID, cost := range teamCosts {
		metrics += fmt.Sprintf("gpu_team_cost_usd{team_id=\"%s\"} %.4f\n", teamID, cost)
	}

	// Project cost breakdown
	projectCosts := make(map[string]float64)
	for _, job := range jobs {
		if job.ProjectID != "" {
			projectCosts[job.ProjectID] += me.costTracker.GetRunningCost(job.ID)
		}
	}

	metrics += "# HELP gpu_project_cost_usd Cost per project\n"
	metrics += "# TYPE gpu_project_cost_usd gauge\n"
	for projectID, cost := range projectCosts {
		metrics += fmt.Sprintf("gpu_project_cost_usd{project_id=\"%s\"} %.4f\n", projectID, cost)
	}

	return metrics
}

// GetCostByTeam returns cost breakdown by team
func (me *MetricsExporter) GetCostByTeam(ctx context.Context) (map[string]float64, error) {
	// Get all jobs (running and completed)
	jobs, _, err := me.jobRepo.ListJobs("", nil, 10000, "")
	if err != nil {
		return nil, err
	}

	teamCosts := make(map[string]float64)

	for _, job := range jobs {
		if job.TeamID == "" {
			continue
		}

		var cost float64
		if job.Status == models.JobStatusRunning {
			cost = me.costTracker.GetRunningCost(job.ID)
		} else if job.CostRunningUSD > 0 {
			cost = job.CostRunningUSD
		} else if job.CostEstimatedUSD != nil {
			cost = *job.CostEstimatedUSD
		}

		teamCosts[job.TeamID] += cost
	}

	return teamCosts, nil
}

// GetCostByProject returns cost breakdown by project
func (me *MetricsExporter) GetCostByProject(ctx context.Context) (map[string]float64, error) {
	jobs, _, err := me.jobRepo.ListJobs("", nil, 10000, "")
	if err != nil {
		return nil, err
	}

	projectCosts := make(map[string]float64)

	for _, job := range jobs {
		if job.ProjectID == "" {
			continue
		}

		var cost float64
		if job.Status == models.JobStatusRunning {
			cost = me.costTracker.GetRunningCost(job.ID)
		} else if job.CostRunningUSD > 0 {
			cost = job.CostRunningUSD
		} else if job.CostEstimatedUSD != nil {
			cost = *job.CostEstimatedUSD
		}

		projectCosts[job.ProjectID] += cost
	}

	return projectCosts, nil
}
