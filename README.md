# Multi-Cloud GPU Orchestration Platform

A production-ready platform for orchestrating GPU workloads across multiple cloud providers (AWS, GCP, Azure) and on-premise infrastructure.

## MVP Scope

- **Single-cluster training** (PyTorch DDP, Horovod)
- **AWS + on-premise** support
- **Basic cost optimization** (spot instances, budget constraints)
- **Checkpoint + resume** functionality
- **Cost tracking** and reporting
- **Backend: Raw VMs** (no Kubernetes/Slurm/Ray initially)

## Project Structure

```
gpu-orchestrator/
├── api/          # REST API handlers
├── core/         # Core business logic (scheduler, optimizer, models)
├── providers/    # Cloud provider integrations
├── training/     # Training framework setup
├── monitoring/   # Cost tracking and metrics
├── storage/      # Data and checkpoint management
└── config/       # Configuration
```

## Getting Started

1. Set up PostgreSQL database
2. Run migrations: `go run cmd/migrate/main.go`
3. Configure providers (AWS credentials, etc.)
4. Start API server: `go run cmd/server/main.go`

## Documentation

See `MULTI_CLOUD_GPU_PLATFORM_GUIDE.md` for detailed implementation guide.
