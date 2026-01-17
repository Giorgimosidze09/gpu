package routes

import (
	"gpu-orchestrator/api/rest/handlers"
	"gpu-orchestrator/core/repository"
	"gpu-orchestrator/core/scheduler"

	"github.com/gorilla/mux"
)

// SetupRoutes configures all API routes
func SetupRoutes(r *mux.Router, db *repository.DB, sched *scheduler.Scheduler) {
	jobRepo := repository.NewJobRepository(db)
	allocationRepo := repository.NewAllocationRepository(db)
	eventRepo := repository.NewEventRepository(db)
	artifactRepo := repository.NewArtifactRepository(db)
	jobHandler := handlers.NewJobHandler(jobRepo, allocationRepo, eventRepo, artifactRepo, sched)

	api := r.PathPrefix("/v1").Subrouter()

	// Job endpoints
	api.HandleFunc("/jobs", jobHandler.SubmitJob).Methods("POST")
	api.HandleFunc("/jobs/{id}", jobHandler.GetJob).Methods("GET")
	api.HandleFunc("/jobs", jobHandler.ListJobs).Methods("GET")
	api.HandleFunc("/jobs/{id}/cancel", jobHandler.CancelJob).Methods("POST")
	api.HandleFunc("/jobs/{id}/events", jobHandler.GetJobEvents).Methods("GET")
	api.HandleFunc("/jobs/{id}/artifacts", jobHandler.GetJobArtifacts).Methods("GET")
}
