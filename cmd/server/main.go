package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"gpu-orchestrator/api/rest/routes"
	"gpu-orchestrator/config"
	"gpu-orchestrator/core/executor"
	"gpu-orchestrator/core/monitoring"
	"gpu-orchestrator/core/optimizer"
	"gpu-orchestrator/core/repository"
	"gpu-orchestrator/core/resource_manager"
	"gpu-orchestrator/core/scheduler"
	"gpu-orchestrator/providers/aws"
	"gpu-orchestrator/providers/azure"
	"gpu-orchestrator/providers/gcp"

	"github.com/gorilla/mux"
)

func main() {
	cfg := config.Load()

	// Initialize database
	db, err := repository.NewDB(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	log.Println("Database connected successfully")

	// Initialize providers
	ctx := context.Background()
	awsClient, _ := aws.NewClient(ctx, []string{"us-east-1", "us-west-2"})
	gcpClient, _ := gcp.NewClient(ctx, "project-id", []string{"us-central1"})
	azureClient, _ := azure.NewClient(ctx, "subscription-id", []string{"eastus"})

	// Initialize pricing fetcher
	pricingFetcher := optimizer.NewPricingFetcher(awsClient, gcpClient, azureClient, db.DB)
	go pricingFetcher.StartRefreshWorker(ctx)

	// Initialize optimizer
	costCalculator := optimizer.NewCostCalculator(pricingFetcher)
	allocationOptimizer := optimizer.NewAllocationOptimizer(costCalculator, pricingFetcher)

	// Initialize repositories
	jobRepo := repository.NewJobRepository(db)
	allocationRepo := repository.NewAllocationRepository(db)

	// Initialize resource manager
	provisioner := resource_manager.NewProvisioner(awsClient, gcpClient, azureClient)

	// Initialize training executor
	trainingExecutor := executor.NewTrainingExecutor(jobRepo)

	// Initialize cost tracker
	costTracker := monitoring.NewCostTracker(jobRepo)
	go costTracker.Start(ctx)

	// Initialize scheduler
	scheduler := scheduler.NewScheduler(jobRepo, allocationRepo, allocationOptimizer, provisioner, trainingExecutor)
	go scheduler.Start(ctx)
	defer scheduler.Stop()

	// Setup routes with database and scheduler
	r := mux.NewRouter()
	routes.SetupRoutes(r, db, scheduler)

	// Health check endpoint
	r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}).Methods("GET")

	// Start server
	server := &http.Server{
		Addr:    ":" + cfg.ServerPort,
		Handler: r,
	}

	// Graceful shutdown
	go func() {
		log.Printf("Starting server on port %s", cfg.ServerPort)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")
	if err := server.Shutdown(context.Background()); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}
	log.Println("Server exited")
}
