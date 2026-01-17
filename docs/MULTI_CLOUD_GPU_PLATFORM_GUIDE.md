# Multi-Cloud GPU Orchestration Platform - Implementation Guide

> **Important Realities**: This guide is based on practical constraints of distributed training. Synchronous training (PyTorch DDP, Horovod) requires low-latency, high-bandwidth networking and should run within a single cluster. Multi-cloud distribution is best for independent tasks (HPO, inference, preprocessing), not gradient synchronization.

## Table of Contents
1. [System Architecture](#system-architecture)
2. [Execution Modes](#execution-modes)
3. [Phase 1: Foundation](#phase-1-foundation)
4. [Phase 2: Core Components](#phase-2-core-components)
5. [Phase 3: Optimization Engine](#phase-3-optimization-engine)
6. [Phase 4: Training Orchestration](#phase-4-training-orchestration)
7. [Phase 5: Monitoring & Cost Tracking](#phase-5-monitoring--cost-tracking)
8. [Technology Stack](#technology-stack)
9. [Implementation Roadmap](#implementation-roadmap)
10. [MVP Plan](#mvp-plan)

---

## System Architecture

### High-Level Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    User Interface / API                      │
│              (REST API, Web UI, CLI, SDK)                    │
└─────────────────────────────────────────────────────────────┘
                            ↓
┌─────────────────────────────────────────────────────────────┐
│                    Job Scheduler                             │
│  - Job Queue Management                                      │
│  - Priority Scheduling                                       │
│  - Execution Mode Selection (Single-Cluster vs Multi-Task)  │
└─────────────────────────────────────────────────────────────┘
                            ↓
┌─────────────────────────────────────────────────────────────┐
│              Cost & Performance Optimization Engine          │
│  - Real-time Pricing Fetcher (from Provider APIs)           │
│  - Performance Metrics ($/step, $/token)                     │
│  - Cluster Selection Optimizer                              │
│  - Data Locality Analyzer                                   │
└─────────────────────────────────────────────────────────────┘
                            ↓
┌─────────────────────────────────────────────────────────────┐
│              Compute Backend Abstraction                     │
│  - Cluster Provisioner (K8s, Slurm, Ray, Managed)          │
│  - Workload Submitter (Container + Command)                 │
│  - Artifact Manager (Checkpoints, Results)                  │
│  - Monitor (Logs, Metrics, Status)                           │
└─────────────────────────────────────────────────────────────┘
                            ↓
        ┌──────────┬──────────┬──────────┬──────────┐
        │   AWS    │   GCP    │  Azure   │ On-Prem  │
        │ Cluster  │ Cluster  │ Cluster  │ Cluster  │
        │ (K8s/EC2)│ (GKE)    │ (AKS)    │ (K8s)    │
        └──────────┴──────────┴──────────┴──────────┘
```

### Key Design Principles

1. **Single-Cluster Training**: Synchronous training (DDP, Horovod) runs entirely within one cluster
2. **Multi-Cloud Task Distribution**: Independent tasks (HPO, inference, eval) can run across clouds
3. **Data Locality**: Prefer compute where data already exists
4. **Checkpoint & Resume**: Mandatory for spot/preemptible, enables relaunch on cheaper targets
5. **Performance-Aware Cost**: Optimize for $/step or $/token, not just $/hour

---

## Execution Modes

### Mode 1: Single-Cluster Training (Default)

**Use Case**: Synchronous distributed training (PyTorch DDP, Horovod, TensorFlow MultiWorker)

**How It Works**:
- Select ONE optimal cluster (AWS/GCP/Azure/on-prem)
- Provision entire cluster in that provider/region
- Run training with full intra-cluster networking
- Optimize: provider selection, spot instances, GPU type, region

**Why Not Multi-Cloud?**:
- Cross-cloud latency (50-200ms) kills all-reduce performance
- Internet/VPN bandwidth insufficient for gradient synchronization
- Egress costs can erase GPU savings
- Model/pipeline parallelism needs high-throughput links

**Example**:
```
Job: Train ResNet-50 on ImageNet (8x A100, 20 hours)
→ Optimizer selects: AWS us-east-1 (cheapest A100 spot) [MVP example]
→ Provisions: 1x p4d.24xlarge instance (8 A100s)
→ Runs: PyTorch DDP within single instance
→ Cost: $72 (vs $80 on other regions)
→ Phase 2: Can also select GCP us-central1 if cheaper
```

### Mode 2: Multi-Cloud Task Distribution

**Use Case**: Embarrassingly parallel workloads

**How It Works**:
- Split job into independent tasks
- Distribute tasks across providers concurrently
- Aggregate results centrally
- Optimize: task allocation, cost per task, parallelization

**Suitable Workloads**:
- **HPO Sweeps**: 200 independent hyperparameter trials
- **Batch Inference**: Process 10K images across clouds
- **Evaluation Runs**: Test model on multiple datasets
- **Data Preprocessing**: Parallel data transformation

**Example**:
```
Job: HPO sweep (200 trials, 1 GPU each, 2 hours/trial)
→ Distributes:
  - 80 trials → AWS spot (cheapest)
  - 70 trials → GCP preemptible
  - 50 trials → On-prem (free)
→ Runs: All trials concurrently
→ Aggregates: Best hyperparameters from all trials
→ Cost: $45 (vs $120 if all on AWS on-demand)
```

### Mode Selection Logic

```go
func SelectExecutionMode(job Job) ExecutionMode {
    // Check if job is embarrassingly parallel
    if job.IsHPO || job.IsBatchInference || job.IsEvaluation {
        return ModeMultiTask
    }
    
    // Check if job requires synchronous training
    if job.Framework == "pytorch_ddp" || 
       job.Framework == "horovod" || 
       job.Framework == "tensorflow_multiworker" {
        return ModeSingleCluster
    }
    
    // Default to single-cluster for safety
    return ModeSingleCluster
}
```

---

## Job Specification (Product Surface)

**This is your product API surface.** Everything in the system maps to this spec.

```yaml
job:
  type: training  # training | hpo | inference | eval
  framework: pytorch  # pytorch_ddp | horovod | tensorflow_multiworker
  entrypoint: s3://my-bucket/train.py  # Script location
  resources:
    gpus: 8
    max_gpus_per_node: 4  # For multi-node training
    requires_multi_node: true  # Whether job needs multiple nodes
    gpu_memory: 80GB  # Per GPU
    cpu_memory: 512GB  # Per instance
  data:
    dataset: s3://datasets/imagenet  # Accepted URIs: s3://, gs://, az://, minio://
    locality: required  # prefer | required | ignore
    replication_policy: pre-stage  # none | pre-stage | on-demand-cache
  constraints:
    budget: 100  # USD
    deadline: 2024-01-15T10:00:00Z  # ISO 8601
    allow_spot: true
    min_reliability: 0.9  # 0.0 - 1.0
    performance_weight: 0.3  # 0.0 (cost only) to 1.0 (performance only)
  execution:
    mode: single_cluster  # single_cluster | multi_task
    # mode is auto-detected if not specified:
    # - single_cluster: For pytorch_ddp, horovod, tensorflow_multiworker
    # - multi_task: For hpo, batch_inference, evaluation
```

### Dataset Handling Contract

**Accepted URI Schemes:**
- `s3://bucket/path` - AWS S3
- `gs://bucket/path` - Google Cloud Storage
- `az://container/path` - Azure Blob Storage
- `minio://endpoint/bucket/path` - On-premise MinIO

**Data Locality Rules:**
- **ModeSingleCluster**: All training jobs read from object storage in **same provider/region** as compute.
- **Cross-cloud reads are disallowed** for ModeSingleCluster (would incur massive egress costs).
- If dataset is in different region, optimizer must:
  1. Replicate dataset (if `replication_policy` allows), OR
  2. Choose compute in same region as dataset

**Replication Policy:**
- `none`: Don't replicate - use source (incur transfer cost)
- `pre-stage`: Replicate to target region before job starts (one-time cost)
- `on-demand-cache`: Replicate on first access, cache for future jobs

**Egress Costs:**
- Customer pays egress cost (included in job budget)
- Optimizer accounts for egress cost in allocation strategy
- Data transfer cost is included in `CalculateDataTransferCost()`

**Dataset Caching:**
- Datasets are **not automatically cached per cluster** - each job reads from source
- Future enhancement: Cache frequently-used datasets per cluster/region

---

## Target Abstraction

**Allocation and scheduling work with `Target`, not raw `(provider, region)` tuples.**

```go
// core/models/target.go
package models

type Target struct {
    Provider Provider  // aws | gcp | azure | onprem
    Region   string    // us-east-1 | us-central1 | etc.
    Backend  BackendType  // k8s | slurm | ray | vm
}

type BackendType string

const (
    BackendKubernetes BackendType = "k8s"      // Kubernetes cluster
    BackendSlurm      BackendType = "slurm"    // Slurm cluster
    BackendRay        BackendType = "ray"      // Ray cluster
    BackendVM         BackendType = "vm"       // Raw VMs (MVP only)
)

// Example usage:
target := Target{
    Provider: ProviderAWS,
    Region:   "us-east-1",
    Backend:  BackendKubernetes,  // Or BackendVM for MVP
}
```

**Why this matters:**
- Prevents codebase from becoming `if provider == aws && backend == k8s && ...`
- Allows switching backends without changing allocation logic
- Makes it easy to add new backends (e.g., Ray, Slurm) without touching provider code

**MVP Note**: MVP uses `BackendVM` (raw VMs). Kubernetes/Slurm/Ray support comes later.

---

## Phase 1: Foundation

### 1.1 Project Structure

```
gpu-orchestrator/
├── api/
│   ├── rest/
│   │   ├── routes/
│   │   │   ├── jobs.go
│   │   │   ├── providers.go
│   │   │   └── monitoring.go
│   │   └── handlers/
│   └── grpc/  # For internal services
├── core/
│   ├── scheduler/
│   │   ├── queue.go
│   │   ├── allocator.go
│   │   └── priority.go
│   ├── optimizer/
│   │   ├── cost_calculator.go
│   │   ├── pricing_fetcher.go
│   │   └── allocation_optimizer.go
│   └── resource_manager/
│       ├── provisioner.go
│       ├── network.go
│       └── storage.go
├── providers/
│   ├── aws/
│   │   ├── client.go
│   │   ├── gpu_provisioner.go
│   │   └── pricing.go
│   ├── gcp/
│   │   ├── client.go
│   │   ├── gpu_provisioner.go
│   │   └── pricing.go
│   ├── azure/
│   │   ├── client.go
│   │   ├── gpu_provisioner.go
│   │   └── pricing.go
│   └── onprem/
│       ├── kubernetes.go
│       └── gpu_discovery.go
├── training/
│   ├── frameworks/
│   │   ├── pytorch.go
│   │   ├── tensorflow.go
│   │   └── horovod.go
│   ├── distributed/
│   │   ├── setup.go
│   │   └── coordinator.go
│   └── checkpointing.go
├── monitoring/
│   ├── metrics.go
│   ├── cost_tracker.go
│   └── performance.go
├── storage/
│   ├── data_manager.go
│   └── checkpoint_manager.go
└── config/
    └── config.go
```

### 1.2 Core Data Models

```go
// core/models/job.go
package models

type Job struct {
    ID              string
    UserID          string
    Name            string
    Framework       string  // "pytorch", "tensorflow", "horovod"
    TrainingScript  string  // S3/MinIO path or git repo (s3:// or minio:// for MVP)
    Dataset         string  // Dataset location
    Requirements    JobRequirements
    Constraints     JobConstraints
    Status          JobStatus
    CreatedAt       time.Time
    StartedAt       *time.Time
    CompletedAt     *time.Time
}

type JobRequirements struct {
    GPUs            int
    MaxGPUsPerNode  int     // Max GPUs per instance (for multi-node training)
    RequiresMultiNode bool  // Whether job requires multiple nodes
    GPUMemory       int     // GB per GPU
    CPUMemory       int     // GB per instance
    Storage         int     // GB
    EstimatedHours  float64
    Framework       string
    ExecutionMode   ExecutionMode // ModeSingleCluster or ModeMultiTask
    DatasetLocation string  // URI (s3://, gs://, az://, minio://)
}

type JobConstraints struct {
    MaxBudget       float64 // USD
    Deadline        time.Time
    PreferredRegions []string
    AllowSpot      bool
    MinReliability float64 // 0.0 - 1.0
    DataLocality   bool    // Prefer compute where data exists
    PerformanceWeight float64 // 0.0 (cost only) to 1.0 (performance only)
}

type JobStatus string
const (
    JobStatusPending      JobStatus = "pending"
    JobStatusScheduled    JobStatus = "scheduled"
    JobStatusProvisioning JobStatus = "provisioning"
    JobStatusRunning      JobStatus = "running"
    JobStatusCheckpointing JobStatus = "checkpointing"
    JobStatusCompleted    JobStatus = "completed"
    JobStatusFailed       JobStatus = "failed"
    JobStatusCancelled    JobStatus = "cancelled"
)

// Job Lifecycle State Machine
// PENDING → SCHEDULED → PROVISIONING → RUNNING → CHECKPOINTING → COMPLETED
//                                                      ↓
//                                                   FAILED
//                                                      ↓
//                                                  CANCELLED
//
// Valid transitions:
// - PENDING → SCHEDULED (optimizer selects allocation)
// - SCHEDULED → PROVISIONING (instances being created)
// - PROVISIONING → RUNNING (training started)
// - RUNNING → CHECKPOINTING (periodic checkpoint or preemption)
// - CHECKPOINTING → RUNNING (checkpoint complete, continue)
// - RUNNING → COMPLETED (training finished successfully)
// - RUNNING → FAILED (error occurred)
// - RUNNING → CANCELLED (user or system cancelled)
// - CHECKPOINTING → FAILED (checkpoint failed)
//
// Why this matters:
// - Retries become clean (failed jobs can be retried from last checkpoint)
// - Checkpoint/resume logic becomes deterministic (only transition from CHECKPOINTING → RUNNING)
// - UI + API become easier (clear state representation)

// core/models/provider.go
package models

type Provider string
const (
    ProviderAWS    Provider = "aws"
    ProviderGCP    Provider = "gcp"
    ProviderAzure  Provider = "azure"
    ProviderOnPrem Provider = "onprem"
)

type GPUInstance struct {
    Provider        Provider
    InstanceType    string  // "p3.2xlarge", "a2-highgpu-1g"
    Region          string
    GPUType         string  // "A100", "V100", "T4"
    GPUsPerInstance int
    MemoryPerGPU    int     // GB
    PricePerHour    float64
    SpotPrice       float64 // If available
    Availability    float64 // 0.0 - 1.0
    InterconnectTier string // "standard" | "high" (for multi-node training)
    LastUpdated     time.Time // When pricing was fetched
}

// Cluster: logical grouping of nodes that share provider/region/network domain
// BackendVM cluster = "a managed group of instances in same VPC/subnet/AZ group"
// All nodes in a cluster can communicate with low latency (required for DDP/Horovod)
type Cluster struct {
    ID              string
    Provider        Provider
    Region          string
    VPC             string  // Network domain
    Backend         BackendType
    Nodes           []Node  // All nodes in this cluster
}

type Node struct {
    ID              string
    InstanceID      string  // Provider-specific instance ID
    Provider        Provider
    Region          string
    VPC             string
    PrivateIP       string  // For DDP communication
    GPUs            int
}

// Performance metrics (for $/step optimization)
type PerformanceMetrics struct {
    StepsPerHour    float64 // Training steps per hour
    TokensPerHour   float64 // For LLM training
    StorageThroughput float64 // MB/s
    NetworkBandwidth float64 // Gbps (for multi-node)
    EffectiveCostPerStep float64 // PricePerHour / StepsPerHour
}

type Allocation struct {
    Provider        Provider
    InstanceType    string
    Region          string
    Count           int
    Spot            bool
    PricePerHour    float64  // Price per hour per instance (explicit for cost tracking)
    EstimatedCost   float64  // Total estimated cost (PricePerHour * Count * Hours)
    EstimatedTime   time.Duration
}
```

---

## Phase 1.3: Database Schema (PostgreSQL)

**Design Goals:**
- Append-only event log (`job_events`) so state transitions are auditable and reliable
- `jobs` holds the current state + user-facing fields
- `allocations` records the chosen compute plan and prices used at decision time
- `job_artifacts` tracks checkpoints/logs/output URIs
- `gpu_pricing` stores on-demand + spot estimates and interconnect tier

```sql
-- Enable UUID generation
CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- ---------- ENUMS ----------
DO $$ BEGIN
  CREATE TYPE job_status AS ENUM (
    'pending',
    'scheduled',
    'provisioning',
    'running',
    'checkpointing',
    'completed',
    'failed',
    'cancelled'
  );
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
  CREATE TYPE provider AS ENUM ('aws','gcp','azure','onprem');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
  CREATE TYPE backend_type AS ENUM ('vm','k8s','slurm','ray');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
  CREATE TYPE execution_mode AS ENUM ('single_cluster','multi_task');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
  CREATE TYPE job_type AS ENUM ('training','hpo','inference','eval');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
  CREATE TYPE interconnect_tier AS ENUM ('standard','high');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
  CREATE TYPE data_locality AS ENUM ('prefer','required','ignore');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
  CREATE TYPE replication_policy AS ENUM ('none','pre-stage','on-demand-cache');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

-- ---------- JOBS ----------
CREATE TABLE IF NOT EXISTS jobs (
  id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id           text NOT NULL,
  name              text NOT NULL,

  job_type          job_type NOT NULL,
  framework         text NOT NULL,               -- e.g. pytorch_ddp | horovod
  entrypoint_uri    text NOT NULL,               -- s3://... or minio://...
  dataset_uri       text NOT NULL,               -- s3://... or minio://...

  execution_mode    execution_mode NOT NULL,     -- single_cluster for MVP
  status            job_status NOT NULL DEFAULT 'pending',

  -- Resources
  gpus              int NOT NULL CHECK (gpus > 0),
  max_gpus_per_node int NOT NULL CHECK (max_gpus_per_node > 0),
  requires_multi_node boolean NOT NULL DEFAULT false,
  gpu_memory_gb     int NOT NULL CHECK (gpu_memory_gb > 0),
  cpu_memory_gb     int NOT NULL CHECK (cpu_memory_gb >= 0),
  storage_gb        int NOT NULL CHECK (storage_gb >= 0),
  estimated_hours   numeric(10,2) NOT NULL CHECK (estimated_hours > 0),

  -- Data rules
  locality          data_locality NOT NULL DEFAULT 'prefer',
  replication       replication_policy NOT NULL DEFAULT 'none',

  -- Constraints
  budget_usd        numeric(12,2) NOT NULL CHECK (budget_usd >= 0),
  deadline_at       timestamptz NULL,
  allow_spot        boolean NOT NULL DEFAULT false,
  min_reliability   numeric(4,3) NOT NULL DEFAULT 0.900 CHECK (min_reliability >= 0 AND min_reliability <= 1),
  performance_weight numeric(4,3) NOT NULL DEFAULT 0.000 CHECK (performance_weight >= 0 AND performance_weight <= 1),

  -- Scheduling outputs (filled after optimize)
  selected_provider provider NULL,
  selected_region   text NULL,
  selected_backend  backend_type NULL DEFAULT 'vm',
  cluster_vpc       text NULL,                   -- network domain for BackendVM cluster
  cluster_id        uuid NULL,                   -- optional if you later store clusters separately

  -- Runtime tracking
  started_at        timestamptz NULL,
  finished_at       timestamptz NULL,
  last_heartbeat_at timestamptz NULL,

  -- Cost tracking (MVP)
  cost_running_usd  numeric(12,4) NOT NULL DEFAULT 0,
  cost_estimated_usd numeric(12,4) NULL,

  -- Spec storage
  spec_yaml         text NOT NULL,               -- store original spec for replay/debug
  created_at        timestamptz NOT NULL DEFAULT now(),
  updated_at        timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_jobs_user_created ON jobs (user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_jobs_status ON jobs (status);
CREATE INDEX IF NOT EXISTS idx_jobs_deadline ON jobs (deadline_at);

-- ---------- JOB EVENTS (append-only state machine) ----------
CREATE TABLE IF NOT EXISTS job_events (
  id          bigserial PRIMARY KEY,
  job_id      uuid NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
  at          timestamptz NOT NULL DEFAULT now(),
  from_status job_status NULL,
  to_status   job_status NOT NULL,
  reason      text NULL,
  meta_json   jsonb NOT NULL DEFAULT '{}'::jsonb
);

CREATE INDEX IF NOT EXISTS idx_job_events_job_at ON job_events (job_id, at DESC);

-- ---------- ALLOCATIONS (what optimizer decided) ----------
CREATE TABLE IF NOT EXISTS allocations (
  id            bigserial PRIMARY KEY,
  job_id        uuid NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,

  provider      provider NOT NULL,
  region        text NOT NULL,
  backend       backend_type NOT NULL DEFAULT 'vm',

  instance_type text NOT NULL,
  count         int NOT NULL CHECK (count > 0),
  spot          boolean NOT NULL DEFAULT false,

  price_per_hour numeric(12,6) NOT NULL CHECK (price_per_hour >= 0),
  estimated_hours numeric(10,2) NOT NULL CHECK (estimated_hours > 0),
  estimated_cost_usd numeric(12,4) NOT NULL CHECK (estimated_cost_usd >= 0),

  created_at    timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_allocations_job ON allocations (job_id);

-- ---------- ARTIFACTS (checkpoints, logs, outputs) ----------
DO $$ BEGIN
  CREATE TYPE artifact_type AS ENUM ('checkpoint','log','output','metrics');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

CREATE TABLE IF NOT EXISTS job_artifacts (
  id          bigserial PRIMARY KEY,
  job_id      uuid NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
  type        artifact_type NOT NULL,
  uri         text NOT NULL,
  created_at  timestamptz NOT NULL DEFAULT now(),
  meta_json   jsonb NOT NULL DEFAULT '{}'::jsonb
);

CREATE INDEX IF NOT EXISTS idx_artifacts_job_type ON job_artifacts (job_id, type);

-- ---------- GPU PRICING CACHE ----------
CREATE TABLE IF NOT EXISTS gpu_pricing (
  id                bigserial PRIMARY KEY,
  provider          provider NOT NULL,
  region            text NOT NULL,
  instance_type     text NOT NULL,
  gpu_type          text NOT NULL,               -- A100/V100/T4
  gpus_per_instance int NOT NULL CHECK (gpus_per_instance > 0),
  memory_per_gpu_gb int NOT NULL CHECK (memory_per_gpu_gb > 0),
  interconnect      interconnect_tier NOT NULL DEFAULT 'standard',

  on_demand_price_per_hour numeric(12,6) NOT NULL CHECK (on_demand_price_per_hour >= 0),
  spot_price_per_hour      numeric(12,6) NULL CHECK (spot_price_per_hour >= 0),
  spot_availability        numeric(4,3) NULL CHECK (spot_availability >= 0 AND spot_availability <= 1),
  interruption_rate        numeric(6,5) NULL CHECK (interruption_rate >= 0 AND interruption_rate <= 1),

  last_updated       timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_gpu_pricing_key
ON gpu_pricing (provider, region, instance_type);

CREATE INDEX IF NOT EXISTS idx_gpu_pricing_updated
ON gpu_pricing (last_updated DESC);
```

**Why this schema fits the design:**
- Lifecycle state machine maps directly to `jobs.status` + `job_events`
- Optimizer outputs are stored immutably in `allocations` with `PricePerHour` exactly as modeled
- Pricing fetcher writes into `gpu_pricing`
- Dataset/checkpoint URIs are stored without enforcing provider early (enforced in optimizer/validator)
- `spec_yaml` is source of truth - parse it → validate → store it → derive fields for indexing

---

## Phase 1.4: REST API (MVP)

**Principles:**
- Job submission uses YAML spec verbatim
- Status is `jobs.status` plus last events
- Cancel triggers state transition + cleanup (even if provisioning)

### Endpoints

#### 1. Submit Job

**POST** `/v1/jobs`

**Request (JSON):**
```json
{
  "name": "resnet50-imagenet",
  "spec_yaml": "job:\n  type: training\n  framework: pytorch_ddp\n  entrypoint: s3://...\n  ..."
}
```

**Response:**
```json
{
  "id": "b6b0d3d6-6f01-4f21-9e3b-1b3d3a1d9a5c",
  "status": "pending",
  "created_at": "2026-01-17T10:00:00Z"
}
```

**Admission Control Failures (422):**
```json
{
  "error": "infeasible_job",
  "message": "dataset locality required but dataset region cannot be satisfied within budget",
  "details": { "field": "data.locality" }
}
```

#### 2. Get Job

**GET** `/v1/jobs/{id}`

**Response:**
```json
{
  "id": "…",
  "name": "…",
  "status": "running",
  "job_type": "training",
  "framework": "pytorch_ddp",
  "execution_mode": "single_cluster",
  "selected": {
    "provider": "aws",
    "region": "us-east-1",
    "backend": "vm",
    "instance_type": "p4d.24xlarge",
    "spot": true,
    "count": 1
  },
  "cost": {
    "running_usd": 12.3456,
    "estimated_usd": 72.0000
  },
  "timestamps": {
    "created_at": "…",
    "started_at": "…",
    "finished_at": null
  }
}
```

#### 3. List Jobs

**GET** `/v1/jobs?status=running&limit=50`

**Response:**
```json
{
  "items": [ { "id": "…", "name": "…", "status": "…" } ],
  "next_cursor": null
}
```

#### 4. Cancel Job

**POST** `/v1/jobs/{id}/cancel`

**Response:**
```json
{ "id": "…", "status": "cancelled" }
```

#### 5. Get Job Events (debug + UI)

**GET** `/v1/jobs/{id}/events`

**Response:**
```json
{
  "items": [
    { "at": "…", "from": "pending", "to": "scheduled", "reason": "optimizer_selected_allocation" },
    { "at": "…", "from": "scheduled", "to": "provisioning" }
  ]
}
```

#### 6. Artifacts (checkpoints/logs)

**GET** `/v1/jobs/{id}/artifacts`

**Response:**
```json
{
  "items": [
    { "type": "checkpoint", "uri": "s3://checkpoints/job-…/step-1000.pt", "created_at": "…" },
    { "type": "log", "uri": "s3://logs/job-…/stdout.log", "created_at": "…" }
  ]
}
```

### Implementation Notes

#### A) Keep `spec_yaml` as Source of Truth

Parse it → validate → store it → derive fields for indexing (gpus, budget, etc.).

When you later change the schema, jobs are still reproducible because the spec is intact.

#### B) State Transitions Must Be Atomic

Every status change should:
1. Update `jobs.status`
2. Insert into `job_events`

In one DB transaction. That's your reliability backbone.

---

## Phase 2: Core Components

### 2.1 Pricing Fetcher

```go
// core/optimizer/pricing_fetcher.go
package optimizer

import (
    "context"
    "sync"
    "time"
    "gpu-orchestrator/providers/aws"
    "gpu-orchestrator/providers/gcp"
    "gpu-orchestrator/providers/azure"
)

type PricingFetcher struct {
    awsClient   *aws.Client
    gcpClient   *gcp.Client
    azureClient *azure.Client
    cache       map[string]CachedPrice
    cacheTTL    time.Duration
    mu          sync.RWMutex
}

type CachedPrice struct {
    Price     float64
    Timestamp time.Time
}

func NewPricingFetcher(
    awsClient *aws.Client,
    gcpClient *gcp.Client,
    azureClient *azure.Client,
    db *sql.DB,
) *PricingFetcher {
    return &PricingFetcher{
        awsClient:   awsClient,
        gcpClient:   gcpClient,
        azureClient: azureClient,
        db:          db,
        cacheTTL:    15 * time.Minute, // Refresh every 15 minutes
    }
}

// Background worker to refresh pricing from provider APIs
func (pf *PricingFetcher) StartRefreshWorker(ctx context.Context) {
    ticker := time.NewTicker(pf.cacheTTL)
    defer ticker.Stop()
    
    // Initial refresh
    pf.refreshAllPricing(ctx)
    
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            pf.refreshAllPricing(ctx)
        }
    }
}

func (pf *PricingFetcher) refreshAllPricing(ctx context.Context) {
    // Fetch on-demand pricing from provider APIs (stable)
    awsPricing, err := pf.awsClient.FetchOnDemandPricing(ctx)
    if err == nil {
        pf.storePricing(awsPricing)
    }
    
    // Fetch spot pricing (probabilistic - uses EC2 Spot Price History)
    awsSpotPricing, err := pf.awsClient.FetchSpotPricing(ctx)
    if err == nil {
        pf.storeSpotPricing(awsSpotPricing) // Store with availability estimates
    }
    
    // Fetch GCP on-demand + preemptible (similar approach)
    gcpPricing, err := pf.gcpClient.FetchOnDemandPricing(ctx)
    if err == nil {
        pf.storePricing(gcpPricing)
    }
    
    gcpPreemptiblePricing, err := pf.gcpClient.FetchPreemptiblePricing(ctx)
    if err == nil {
        pf.storePreemptiblePricing(gcpPreemptiblePricing)
    }
    
    // Fetch Azure on-demand + spot (similar approach)
    azurePricing, err := pf.azureClient.FetchOnDemandPricing(ctx)
    if err == nil {
        pf.storePricing(azurePricing)
    }
    
    azureSpotPricing, err := pf.azureClient.FetchSpotPricing(ctx)
    if err == nil {
        pf.storeSpotPricing(azureSpotPricing)
    }
}

// Note: Spot pricing is PROBABILISTIC, not guaranteed
// - AWS Spot: Price varies by AZ, availability changes
// - GCP Preemptible: Fixed discount (~60-70%), but can be terminated
// - Azure Spot: Similar to AWS, varies by region/AZ
// Store with: price, availability_estimate, interruption_rate

// Fetch real-time pricing from all providers
func (pf *PricingFetcher) FetchAllPricing(ctx context.Context) (map[Provider][]GPUInstance, error) {
    var wg sync.WaitGroup
    results := make(map[Provider][]GPUInstance)
    errChan := make(chan error, 3)
    
    // Fetch AWS pricing
    wg.Add(1)
    go func() {
        defer wg.Done()
        instances, err := pf.awsClient.GetGPUInstances(ctx)
        if err != nil {
            errChan <- err
            return
        }
        results[ProviderAWS] = instances
    }()
    
    // Fetch GCP pricing
    wg.Add(1)
    go func() {
        defer wg.Done()
        instances, err := pf.gcpClient.GetGPUInstances(ctx)
        if err != nil {
            errChan <- err
            return
        }
        results[ProviderGCP] = instances
    }()
    
    // Fetch Azure pricing
    wg.Add(1)
    go func() {
        defer wg.Done()
        instances, err := pf.azureClient.GetGPUInstances(ctx)
        if err != nil {
            errChan <- err
            return
        }
        results[ProviderAzure] = instances
    }()
    
    wg.Wait()
    close(errChan)
    
    // Check for errors
    if len(errChan) > 0 {
        return nil, <-errChan
    }
    
    return results, nil
}

// Get price from database (refreshed by background worker)
func (pf *PricingFetcher) GetPrice(provider Provider, instanceType string, region string, spot bool) (float64, error) {
    // Query database for latest pricing
    var price float64
    var lastUpdated time.Time
    
    query := `
        SELECT on_demand_price, spot_price, last_updated 
        FROM gpu_pricing 
        WHERE provider = $1 AND instance_type = $2 AND region = $3
        ORDER BY last_updated DESC 
        LIMIT 1
    `
    
    err := pf.db.QueryRow(query, provider, instanceType, region).Scan(
        &price, &spotPrice, &lastUpdated,
    )
    if err == sql.ErrNoRows {
        // No pricing data, fetch fresh
        return pf.fetchFreshPrice(context.Background(), provider, instanceType, region, spot)
    }
    if err != nil {
        return 0, err
    }
    
    // Use spot price if requested and available
    if spot && spotPrice > 0 {
        price = spotPrice
    }
    
    // If pricing is stale (> 1 hour), fetch fresh in background
    if time.Since(lastUpdated) > 1*time.Hour {
        go pf.refreshPricingForInstance(context.Background(), provider, instanceType, region)
    }
    
    return price, nil
}

// Get all instances from database
func (pf *PricingFetcher) GetAllInstances(ctx context.Context) (map[Provider][]GPUInstance, error) {
    query := `
        SELECT provider, instance_type, region, gpu_type, gpus_per_instance,
               memory_per_gpu, on_demand_price, spot_price, spot_availability, last_updated
        FROM gpu_pricing
        WHERE last_updated > NOW() - INTERVAL '1 hour'
    `
    
    rows, err := pf.db.QueryContext(ctx, query)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    
    results := make(map[Provider][]GPUInstance)
    
    for rows.Next() {
        var instance GPUInstance
        var spotPrice sql.NullFloat64
        var spotAvailability sql.NullFloat64
        
        err := rows.Scan(
            &instance.Provider,
            &instance.InstanceType,
            &instance.Region,
            &instance.GPUType,
            &instance.GPUsPerInstance,
            &instance.MemoryPerGPU,
            &instance.PricePerHour,
            &spotPrice,
            &spotAvailability,
            &instance.LastUpdated,
        )
        if err != nil {
            continue
        }
        
        if spotPrice.Valid {
            instance.SpotPrice = spotPrice.Float64
        }
        if spotAvailability.Valid {
            instance.Availability = spotAvailability.Float64
        }
        
        results[instance.Provider] = append(results[instance.Provider], instance)
    }
    
    return results, nil
}
```

### 2.2 Cost Calculator

```go
// core/optimizer/cost_calculator.go
package optimizer

import (
    "gpu-orchestrator/core/models"
    "time"
)

type CostCalculator struct {
    pricingFetcher *PricingFetcher
}

func NewCostCalculator(pf *PricingFetcher) *CostCalculator {
    return &CostCalculator{
        pricingFetcher: pf,
    }
}

// Calculate total cost for an allocation
func (cc *CostCalculator) CalculateCost(allocation []Allocation, estimatedHours float64) (float64, error) {
    totalCost := 0.0
    
    for _, alloc := range allocation {
        price, err := cc.pricingFetcher.GetPrice(
            alloc.Provider,
            alloc.InstanceType,
            alloc.Region,
            alloc.Spot,
        )
        if err != nil {
            return 0, err
        }
        
        // Cost = price per hour × number of instances × hours
        cost := price * float64(alloc.Count) * estimatedHours
        totalCost += cost
    }
    
    return totalCost, nil
}

// Calculate cost with spot instance interruption probability
func (cc *CostCalculator) CalculateCostWithReliability(
    allocation []Allocation,
    estimatedHours float64,
    spotInterruptionRate float64, // e.g., 0.1 = 10% chance of interruption
) (float64, float64) {
    baseCost, _ := cc.CalculateCost(allocation, estimatedHours)
    
    // Calculate expected cost including restarts
    // If spot instance is interrupted, we need to restart (adds overhead)
    spotInstances := 0
    for _, alloc := range allocation {
        if alloc.Spot {
            spotInstances += alloc.Count
        }
    }
    
    // Expected interruptions = hours × interruption_rate
    expectedInterruptions := estimatedHours * spotInterruptionRate
    
    // Each interruption adds ~10 minutes overhead (restart time)
    overheadHours := expectedInterruptions * (10.0 / 60.0)
    
    // Recalculate cost with overhead
    totalCost := baseCost * (1 + overheadHours/estimatedHours)
    reliability := 1.0 - (expectedInterruptions / estimatedHours)
    
    return totalCost, reliability
}
```

### 2.3 Allocation Optimizer

```go
// core/optimizer/allocation_optimizer.go
package optimizer

import (
    "context"
    "gpu-orchestrator/core/models"
    "sort"
)

type AllocationOptimizer struct {
    costCalculator *CostCalculator
    pricingFetcher *PricingFetcher
}

func NewAllocationOptimizer(cc *CostCalculator, pf *PricingFetcher) *AllocationOptimizer {
    return &AllocationOptimizer{
        costCalculator: cc,
        pricingFetcher: pf,
    }
}

// Optimize allocation based on job requirements and constraints
func (ao *AllocationOptimizer) Optimize(
    ctx context.Context,
    requirements JobRequirements,
    constraints JobConstraints,
) ([]Allocation, error) {
    // Step 1: Get all available GPU instances
    allInstances, err := ao.pricingFetcher.FetchAllPricing(ctx)
    if err != nil {
        return nil, err
    }
    
    // Step 2: Filter instances that meet requirements
    candidates := ao.filterCandidates(allInstances, requirements)
    
    // Step 3: Generate allocation strategies
    strategies := ao.generateStrategies(candidates, requirements, constraints)
    
    // Step 4: Score each strategy
    scoredStrategies := ao.scoreStrategies(strategies, requirements, constraints)
    
    // Step 5: Return best strategy
    if len(scoredStrategies) == 0 {
        return nil, fmt.Errorf("no suitable allocation found")
    }
    
    return scoredStrategies[0].Allocation, nil
}

func (ao *AllocationOptimizer) filterCandidates(
    allInstances map[Provider][]GPUInstance,
    requirements JobRequirements,
) []GPUInstance {
    var candidates []GPUInstance
    
    for _, instances := range allInstances {
        for _, instance := range instances {
            // Check if instance meets requirements
            if instance.GPUsPerInstance > 0 &&
               instance.MemoryPerGPU >= requirements.GPUMemory {
                candidates = append(candidates, instance)
            }
        }
    }
    
    return candidates
}

type Strategy struct {
    Allocation     []Allocation
    TotalCost      float64
    Reliability    float64
    EstimatedTime  time.Duration
    Score          float64
}

func (ao *AllocationOptimizer) generateStrategies(
    candidates []GPUInstance,
    requirements JobRequirements,
    constraints JobConstraints,
) []Strategy {
    var strategies []Strategy
    
    gpusNeeded := requirements.GPUs
    
    // Split strategy generation by execution mode
    switch requirements.ExecutionMode {
    case ModeSingleCluster:
        // Single-cluster strategies: ALL nodes must be same provider+region
        // Strategy 1: Cheapest single region (prefer spot)
        strategies = append(strategies, ao.cheapestSingleRegionStrategy(candidates, requirements, constraints))
        
        // Strategy 2: Most reliable single region (avoid spot, prefer on-prem)
        strategies = append(strategies, ao.reliableSingleRegionStrategy(candidates, requirements, constraints))
        
        // Strategy 3: Data locality (prefer region where dataset exists)
        if constraints.DataLocality {
            strategies = append(strategies, ao.dataLocalityStrategy(candidates, requirements, constraints))
        }
        
    case ModeMultiTask:
        // Multi-task strategies: Can distribute across providers/regions
        // Strategy 1: Cheapest overall (distribute tasks)
        strategies = append(strategies, ao.cheapestMultiProviderStrategy(candidates, requirements, constraints))
        
        // Strategy 2: Geographic distribution (for parallel tasks)
        strategies = append(strategies, ao.geoDistributedTaskStrategy(candidates, requirements, constraints))
        
        // Strategy 3: On-prem first, cloud backup
        strategies = append(strategies, ao.hybridTaskStrategy(candidates, requirements, constraints))
    }
    
    return strategies
}

// Single-cluster: Cheapest strategy within ONE provider+region
func (ao *AllocationOptimizer) cheapestSingleRegionStrategy(
    candidates []GPUInstance,
    requirements JobRequirements,
    constraints JobConstraints,
) Strategy {
    // Group by provider+region
    regionGroups := make(map[string][]GPUInstance)
    for _, instance := range candidates {
        key := fmt.Sprintf("%s:%s", instance.Provider, instance.Region)
        regionGroups[key] = append(regionGroups[key], instance)
    }
    
    // Find cheapest provider+region combination
    var bestStrategy Strategy
    bestCost := 999999.0
    
    for regionKey, instances := range regionGroups {
        // Check if we can allocate all GPUs in this region
        regionStrategy := ao.cheapestStrategy(instances, requirements, constraints)
        if regionStrategy.TotalCost < bestCost && len(regionStrategy.Allocation) > 0 {
            bestCost = regionStrategy.TotalCost
            bestStrategy = regionStrategy
            // Verify all allocations are in same provider+region
            provider, region := parseRegionKey(regionKey)
            for _, alloc := range regionStrategy.Allocation {
                if alloc.Provider != provider || alloc.Region != region {
                    // Skip if allocation spans regions
                    bestCost = 999999.0
                    break
                }
            }
        }
    }
    
    return bestStrategy
}

// Multi-task: Distribute tasks across providers
func (ao *AllocationOptimizer) cheapestMultiProviderStrategy(
    candidates []GPUInstance,
    requirements JobRequirements,
    constraints JobConstraints,
) Strategy {
    // For multi-task, we can distribute across providers
    // This is the original cheapestStrategy (allows cross-provider)
    return ao.cheapestStrategy(candidates, requirements, constraints)
}

func (ao *AllocationOptimizer) cheapestStrategy(
    candidates []GPUInstance,
    requirements JobRequirements,
    constraints JobConstraints,
) Strategy {
    // Validation: For single-cluster training, check multi-node constraints
    if requirements.ExecutionMode == ModeSingleCluster && requirements.RequiresMultiNode {
        // Reject instances without fast interconnect (e.g., single-node only)
        candidates = ao.filterMultiNodeCompatible(candidates, requirements)
    }
    
    // Sort by price per GPU (prefer spot instances)
    sorted := make([]GPUInstance, len(candidates))
    copy(sorted, candidates)
    
    sort.Slice(sorted, func(i, j int) bool {
        iPricePerGPU := sorted[i].PricePerHour / float64(sorted[i].GPUsPerInstance)
        if sorted[i].SpotPrice > 0 && constraints.AllowSpot {
            iPricePerGPU = sorted[i].SpotPrice / float64(sorted[i].GPUsPerInstance)
        }
        jPricePerGPU := sorted[j].PricePerHour / float64(sorted[j].GPUsPerInstance)
        if sorted[j].SpotPrice > 0 && constraints.AllowSpot {
            jPricePerGPU = sorted[j].SpotPrice / float64(sorted[j].GPUsPerInstance)
        }
        return iPricePerGPU < jPricePerGPU
    })
    
    // Allocate greedily
    var allocation []Allocation
    remaining := requirements.GPUs
    
    for _, instance := range sorted {
        if remaining <= 0 {
            break
        }
        
        // Check per-instance constraints
        if requirements.MaxGPUsPerNode > 0 && instance.GPUsPerInstance > requirements.MaxGPUsPerNode {
            continue // Instance has too many GPUs per node
        }
        
        instancesNeeded := (remaining + instance.GPUsPerInstance - 1) / instance.GPUsPerInstance
        if instancesNeeded > 0 {
            // For multi-node training, check max nodes per cluster/AZ constraints
            if requirements.RequiresMultiNode {
                maxNodes := ao.getMaxNodesForProvider(instance.Provider, instance.Region)
                if instancesNeeded > maxNodes {
                    // Can't allocate all in one region - skip this instance type
                    continue
                }
            }
            
            useSpot := constraints.AllowSpot && instance.SpotPrice > 0
            price := instance.PricePerHour
            if useSpot {
                price = instance.SpotPrice
            }
            
            allocation = append(allocation, Allocation{
                Provider:      instance.Provider,
                InstanceType:  instance.InstanceType,
                Region:        instance.Region,
                Count:         instancesNeeded,
                Spot:          useSpot,
                PricePerHour:  price,  // Store explicitly per instance
                EstimatedCost: price * float64(instancesNeeded) * requirements.EstimatedHours,
            })
            
            remaining -= instancesNeeded * instance.GPUsPerInstance
        }
    }
    
    // Check if allocation is complete
    if remaining > 0 {
        // Could not allocate all GPUs - return empty strategy (will be filtered by scoring)
        return Strategy{Allocation: []Allocation{}}
    }
    
    return Strategy{Allocation: allocation}
}

// Filter instances compatible with multi-node training
func (ao *AllocationOptimizer) filterMultiNodeCompatible(
    candidates []GPUInstance,
    requirements JobRequirements,
) []GPUInstance {
    var filtered []GPUInstance
    
    for _, instance := range candidates {
        // MVP-safe rule: Multi-node training only supported for high-tier interconnect
        // Whitelisted instance types: p4d.24xlarge (EFA-enabled), etc.
        // Single-node multi-GPU can use any instance type
        hasFastInterconnect := instance.InterconnectTier == "high"
        
        // For MVP, reject multi-node training on standard-tier instances
        // This prevents users from assuming multi-node will work on any instance type
        
        // Check max nodes per AZ/cluster for this instance type
        maxNodes := ao.getMaxNodesForProvider(instance.Provider, instance.Region)
        minNodesNeeded := (requirements.GPUs + instance.GPUsPerInstance - 1) / instance.GPUsPerInstance
        
        if hasFastInterconnect && minNodesNeeded <= maxNodes {
            filtered = append(filtered, instance)
        }
    }
    
    return filtered
}

// Get max nodes per cluster/AZ for provider+region
func (ao *AllocationOptimizer) getMaxNodesForProvider(provider Provider, region string) int {
    // Provider-specific limits
    // AWS: ~16 nodes per AZ for EFA-enabled instances
    // GCP: ~32 nodes per region for high-bandwidth VPC
    // Azure: ~16 nodes per availability set
    // On-prem: depends on K8s cluster size
    
    switch provider {
    case ProviderAWS:
        return 16 // Conservative limit per AZ
    case ProviderGCP:
        return 32 // Per region
    case ProviderAzure:
        return 16 // Per availability set
    case ProviderOnPrem:
        return 100 // K8s cluster can be large
    default:
        return 8 // Conservative default
    }
}

func (ao *AllocationOptimizer) scoreStrategies(
    strategies []Strategy,
    requirements JobRequirements,
    constraints JobConstraints,
) []Strategy {
    for i := range strategies {
        strategy := &strategies[i]
        
        // Calculate cost metrics
        totalCost, _ := ao.costCalculator.CalculateCost(
            strategy.Allocation,
            requirements.EstimatedHours,
        )
        strategy.TotalCost = totalCost
        
        // Calculate performance metrics (if available)
        // Performance metrics come from:
        // Phase 1: Static benchmark table (ResNet, BERT, LLaMA on common GPU types)
        // Phase 2: Historical job telemetry (learn from past runs)
        // Phase 3: Per-customer performance profiles (custom benchmarks)
        performanceMetrics := ao.getPerformanceMetrics(strategy.Allocation, requirements.Framework)
        
        // Calculate effective cost per step (if performance data available)
        var costPerStep float64
        if performanceMetrics.StepsPerHour > 0 {
            costPerStep, _ = ao.costCalculator.CalculateCostPerStep(
                strategy.Allocation,
                performanceMetrics,
            )
        }
        
        // Calculate data transfer cost
        dataTransferCost := 0.0
        if requirements.DatasetLocation != "" {
            // Estimate transfer cost if dataset not in same region
            for _, alloc := range strategy.Allocation {
                transferCost := ao.costCalculator.CalculateDataTransferCost(
                    100.0, // Estimate dataset size (should be from job config)
                    parseProviderFromLocation(requirements.DatasetLocation),
                    parseRegionFromLocation(requirements.DatasetLocation),
                    alloc.Provider,
                    alloc.Region,
                )
                dataTransferCost += transferCost
            }
        }
        
        // Calculate reliability
        spotCount := 0
        for _, alloc := range strategy.Allocation {
            if alloc.Spot {
                spotCount += alloc.Count
            }
        }
        // TODO: Replace with per-provider interruption probability × runtime model
        // Current implementation: Simplified 10% interruption rate for spot instances
        // This ignores duration and provider-specific interruption rates (acceptable for MVP)
        strategy.Reliability = 1.0 - (float64(spotCount) / float64(len(strategy.Allocation)) * 0.1)
        
        // Calculate score (lower is better)
        // Weighted combination of cost and performance
        costWeight := 1.0 - constraints.PerformanceWeight
        perfWeight := constraints.PerformanceWeight
        
        normalizedCost := (totalCost + dataTransferCost) / constraints.MaxBudget
        
        // If performance metrics available, use cost per step
        // Otherwise, use hourly cost
        var costMetric float64
        if costPerStep > 0 {
            // Normalize cost per step (compare to baseline)
            // Baselines are (framework, model_class, GPU_type) tuples
            // Unknown combinations fall back to conservative defaults
            // Example: baselineCostPerStep = getBaselineCostPerStep("pytorch", "A100")
            baselineCostPerStep := 0.001 // Placeholder - implement per-framework+GPU lookup
            costMetric = costPerStep / baselineCostPerStep
        } else {
            costMetric = normalizedCost
        }
        
        // Performance score (higher steps/hour is better)
        // Baselines are (framework, model_class, GPU_type) tuples
        // Unknown combinations fall back to conservative defaults
        // Example: baselineStepsPerHour = getBaselineStepsPerHour("pytorch", "A100")
        var perfScore float64
        if performanceMetrics.StepsPerHour > 0 {
            baselineStepsPerHour := 1000.0 // Placeholder - implement per-framework+GPU lookup
            perfScore = 1.0 - (performanceMetrics.StepsPerHour / baselineStepsPerHour)
            if perfScore < 0 {
                perfScore = 0 // Better than baseline
            }
        } else {
            perfScore = 0.5 // Unknown performance
        }
        
        reliabilityPenalty := (1.0 - strategy.Reliability) * 0.2
        
        strategy.Score = costWeight*costMetric +
                        perfWeight*perfScore +
                        reliabilityPenalty
        
        // Filter out strategies that don't meet constraints
        if (totalCost + dataTransferCost) > constraints.MaxBudget {
            strategy.Score = 999999 // Very bad score
        }
        if strategy.Reliability < constraints.MinReliability {
            strategy.Score = 999999
        }
    }
    
    // Sort by score (best first)
    sort.Slice(strategies, func(i, j int) bool {
        return strategies[i].Score < strategies[j].Score
    })
    
    return strategies
}
```

---

## Phase 3: Provider Implementations

### 3.1 AWS Provider

```go
// providers/aws/client.go
package aws

import (
    "context"
    "github.com/aws/aws-sdk-go-v2/aws"
    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/aws/aws-sdk-go-v2/service/ec2"
    "github.com/aws/aws-sdk-go-v2/service/pricing"
)

type Client struct {
    ec2Client    *ec2.Client
    pricingClient *pricing.Client
    regions      []string
}

func NewClient(ctx context.Context, regions []string) (*Client, error) {
    cfg, err := config.LoadDefaultConfig(ctx)
    if err != nil {
        return nil, err
    }
    
    return &Client{
        ec2Client:     ec2.NewFromConfig(cfg),
        pricingClient: pricing.NewFromConfig(cfg),
        regions:       regions,
    }, nil
}

// providers/aws/gpu_provisioner.go
func (c *Client) ProvisionGPUInstance(
    ctx context.Context,
    instanceType string,
    region string,
    spot bool,
    count int,
) ([]string, error) { // Returns instance IDs
    // Create EC2 instances
    input := &ec2.RunInstancesInput{
        ImageId:      aws.String("ami-xxxxx"), // GPU-optimized AMI
        InstanceType: ec2.InstanceType(instanceType),
        MinCount:     aws.Int32(int32(count)),
        MaxCount:     aws.Int32(int32(count)),
        IamInstanceProfile: &ec2.IamInstanceProfileSpecification{
            Name: aws.String("gpu-instance-profile"),
        },
        UserData: aws.String(getUserDataScript()), // Install training framework
    }
    
    if spot {
        input.InstanceMarketOptions = &ec2.InstanceMarketOptionsRequest{
            MarketType: ec2.MarketTypeSpot,
            SpotOptions: &ec2.SpotMarketOptions{
                SpotInstanceType: ec2.SpotInstanceTypeOneTime,
                MaxPrice:         aws.String("0.50"), // Max spot price
            },
        }
    }
    
    result, err := c.ec2Client.RunInstances(ctx, input)
    if err != nil {
        return nil, err
    }
    
    instanceIDs := make([]string, len(result.Instances))
    for i, instance := range result.Instances {
        instanceIDs[i] = *instance.InstanceId
    }
    
    return instanceIDs, nil
}

// providers/aws/pricing.go
func (c *Client) GetGPUInstances(ctx context.Context) ([]GPUInstance, error) {
    // AWS GPU instance types
    gpuInstances := []struct {
        InstanceType string
        GPUType      string
        GPUs         int
        Memory       int
    }{
        {"p3.2xlarge", "V100", 1, 16},
        {"p3.8xlarge", "V100", 4, 64},
        {"p3.16xlarge", "V100", 8, 128},
        {"p4d.24xlarge", "A100", 8, 320},
        {"g4dn.xlarge", "T4", 1, 16},
    }
    
    var instances []GPUInstance
    
    for _, region := range c.regions {
        for _, gpu := range gpuInstances {
            // Fetch on-demand price
            onDemandPrice, err := c.getInstancePrice(ctx, gpu.InstanceType, region, false)
            if err != nil {
                continue
            }
            
            // Fetch spot price
            spotPrice, _ := c.getInstancePrice(ctx, gpu.InstanceType, region, true)
            
            instances = append(instances, GPUInstance{
                Provider:        ProviderAWS,
                InstanceType:    gpu.InstanceType,
                Region:          region,
                GPUType:         gpu.GPUType,
                GPUsPerInstance: gpu.GPUs,
                MemoryPerGPU:    gpu.Memory,
                PricePerHour:    onDemandPrice,
                SpotPrice:       spotPrice,
                Availability:    1.0, // Would fetch from AWS API
            })
        }
    }
    
    return instances, nil
}

func (c *Client) GetInstancePrice(
    ctx context.Context,
    instanceType string,
    region string,
    spot bool,
) (float64, error) {
    // AWS Pricing API: On-demand pricing is stable and queryable
    // Spot pricing: Use EC2 Spot Price History API (per-AZ, varies frequently)
    // Spot pricing is PROBABILISTIC - not guaranteed availability
    // Store: price, availability_estimate, interruption_rate
    
    // This is simplified - actual implementation would query Pricing API
    // or use AWS Cost Explorer API for on-demand
    
    // For spot: Use DescribeSpotPriceHistory API
    // For now, return mock data (replace with actual API calls)
    prices := map[string]float64{
        "p3.2xlarge":   3.06,
        "p3.8xlarge":   12.24,
        "p3.16xlarge":  24.48,
        "p4d.24xlarge": 32.77,
        "g4dn.xlarge":  0.526,
    }
    
    basePrice, ok := prices[instanceType]
    if !ok {
        return 0, fmt.Errorf("unknown instance type: %s", instanceType)
    }
    
    if spot {
        // Spot instances are typically 60-90% cheaper
        return basePrice * 0.3, nil
    }
    
    return basePrice, nil
}
```

### 3.2 GCP Provider

```go
// providers/gcp/client.go
package gcp

import (
    "context"
    "google.golang.org/api/compute/v1"
    "google.golang.org/api/option"
)

type Client struct {
    computeService *compute.Service
    projectID      string
    regions        []string
}

func NewClient(ctx context.Context, projectID string, regions []string) (*Client, error) {
    computeService, err := compute.NewService(ctx, option.WithScopes(compute.CloudPlatformScope))
    if err != nil {
        return nil, err
    }
    
    return &Client{
        computeService: computeService,
        projectID:      projectID,
        regions:        regions,
    }, nil
}

// providers/gcp/gpu_provisioner.go
func (c *Client) ProvisionGPUInstance(
    ctx context.Context,
    instanceType string,
    zone string,
    preemptible bool,
    count int,
) ([]string, error) {
    // GCP instance types
    // a2-highgpu-1g = 1x A100 GPU
    // a2-highgpu-2g = 2x A100 GPUs
    // a2-highgpu-4g = 4x A100 GPUs
    // a2-highgpu-8g = 8x A100 GPUs
    
    instanceIDs := make([]string, count)
    
    for i := 0; i < count; i++ {
        instance := &compute.Instance{
            Name:        fmt.Sprintf("gpu-instance-%d-%d", time.Now().Unix(), i),
            MachineType: fmt.Sprintf("zones/%s/machineTypes/%s", zone, instanceType),
            Scheduling: &compute.Scheduling{
                Preemptible: preemptible,
            },
            Disks: []*compute.AttachedDisk{
                {
                    Boot:       true,
                    AutoDelete: true,
                    InitializeParams: &compute.AttachedDiskInitializeParams{
                        SourceImage: "projects/deeplearning-platform-release/global/images/family/tf2-2-8-cu113",
                    },
                },
            },
            NetworkInterfaces: []*compute.NetworkInterface{
                {
                    Network: "global/networks/default",
                    AccessConfigs: []*compute.AccessConfig{
                        {
                            Type: "ONE_TO_ONE_NAT",
                            Name: "External NAT",
                        },
                    },
                },
            },
            ServiceAccounts: []*compute.ServiceAccount{
                {
                    Email:  "default",
                    Scopes: []string{"https://www.googleapis.com/auth/cloud-platform"},
                },
            },
            Metadata: &compute.Metadata{
                Items: []*compute.MetadataItems{
                    {
                        Key:   "startup-script",
                        Value: aws.String(getUserDataScript()),
                    },
                },
            },
        }
        
        op, err := c.computeService.Instances.Insert(c.projectID, zone, instance).Do()
        if err != nil {
            return nil, err
        }
        
        // Wait for instance to be created
        instanceID, err := c.waitForInstance(ctx, op.Name, zone)
        if err != nil {
            return nil, err
        }
        
        instanceIDs[i] = instanceID
    }
    
    return instanceIDs, nil
}
```

---

## Phase 4: Training Setup (Single Cluster Only)

### 4.1 Single-Cluster Training Setup (DDP/Horovod)

**Critical Constraint**: Synchronous training (DDP, Horovod, TensorFlow MultiWorker) **MUST** run within a single provider, single region, single network domain.

**Validation Rule**: All instances must share `(provider, region, network_domain)` before DDP setup is allowed.

```go
// training/frameworks/pytorch.go
package frameworks

import (
    "fmt"
    "strconv"
    "gpu-orchestrator/core/models"
)

type PyTorchSetup struct{}

// Setup distributed training within a SINGLE cluster
// Enforces same provider+region+network constraint
func (p *PyTorchSetup) SetupDistributedTraining(
    cluster Cluster, // Single cluster (all nodes MUST be same provider/region/VPC)
    job Job,
) (*DistributedConfig, error) {
    // VALIDATION: Ensure all nodes are in same provider+region+network
    if err := p.validateClusterTopology(cluster); err != nil {
        return nil, fmt.Errorf("cluster topology validation failed: %w", err)
    }
    
    // PyTorch Distributed Data Parallel (DDP) setup
    // All nodes must be in same cluster for low-latency all-reduce
    
    nodes := cluster.GetNodes()
    
    if len(nodes) == 0 {
        return nil, fmt.Errorf("cluster has no nodes")
    }
    
    // All nodes should be in same provider/region/VPC (validated above)
    config := &DistributedConfig{
        Framework: "pytorch",
        MasterAddr: nodes[0].PrivateIP,
        MasterPort: 29500,
        WorldSize: len(nodes),
        Nodes: make([]NodeConfig, len(nodes)),
    }
    
    for i, node := range nodes {
        config.Nodes[i] = NodeConfig{
            Rank:        i,
            Address:     node.PrivateIP,
            GPUs:        node.GPUs,
            Environment: p.getEnvironment(job, i, len(nodes)),
        }
    }
    
    return config, nil
}

// Validate that all nodes are in same provider+region+network
func (p *PyTorchSetup) validateClusterTopology(cluster Cluster) error {
    nodes := cluster.GetNodes()
    if len(nodes) == 0 {
        return fmt.Errorf("empty cluster")
    }
    
    // Get first node's topology
    firstNode := nodes[0]
    expectedProvider := firstNode.Provider
    expectedRegion := firstNode.Region
    expectedVPC := firstNode.VPC // or NetworkDomain
    
    // Check all other nodes match
    for i, node := range nodes {
        if node.Provider != expectedProvider {
            return fmt.Errorf("node %d has provider %s, expected %s", i, node.Provider, expectedProvider)
        }
        if node.Region != expectedRegion {
            return fmt.Errorf("node %d has region %s, expected %s", i, node.Region, expectedRegion)
        }
        if node.VPC != expectedVPC {
            return fmt.Errorf("node %d has VPC %s, expected %s", i, node.VPC, expectedVPC)
        }
    }
    
    return nil
}

// Mode selection with validation
func SelectExecutionMode(job Job) (ExecutionMode, error) {
    // Check if job is embarrassingly parallel
    if job.IsHPO || job.IsBatchInference || job.IsEvaluation {
        return ModeMultiTask, nil
    }
    
    // Check if job requires synchronous training
    if job.Framework == "pytorch_ddp" || 
       job.Framework == "horovod" || 
       job.Framework == "tensorflow_multiworker" {
        // For synchronous training, MUST use single cluster
        return ModeSingleCluster, nil
    }
    
    // Default to single-cluster for safety
    return ModeSingleCluster, nil
}

func (p *PyTorchSetup) getEnvironment(job Job) map[string]string {
    return map[string]string{
        "MASTER_ADDR":     "", // Will be set per node
        "MASTER_PORT":     "29500",
        "WORLD_SIZE":      "", // Will be set per node
        "RANK":            "", // Will be set per node
        "NCCL_DEBUG":      "INFO",
        "NCCL_SOCKET_IFNAME": "eth0",
        // For multi-cloud, may need VPN interface
    }
}

// Generate training script wrapper
func (p *PyTorchSetup) GenerateTrainingScript(config *DistributedConfig, job Job) string {
    return `#!/bin/bash
# Auto-generated PyTorch DDP training script

export MASTER_ADDR=` + config.MasterAddr + `
export MASTER_PORT=` + config.MasterPort + `
export WORLD_SIZE=` + strconv.Itoa(config.WorldSize) + `
export RANK=$1  # Passed as argument

# Launch training
python -m torch.distributed.launch \
    --nproc_per_node=` + strconv.Itoa(config.Nodes[0].GPUs) + ` \
    --nnodes=` + strconv.Itoa(config.WorldSize) + ` \
    --node_rank=$RANK \
    --master_addr=$MASTER_ADDR \
    --master_port=$MASTER_PORT \
    ` + job.TrainingScript + `
`
}
```

### 4.2 Network Setup

**Critical**: Network setup depends on execution mode:

- **ModeSingleCluster**: Use native VPC/CNI (NO VPN). All nodes are in same provider/region/VPC.
- **ModeMultiTask**: VPN mesh is optional for control plane only (not for task execution).

**VPN/NetworkManager is NOT used for single-cluster training** - it would degrade performance.

```go
// core/resource_manager/network.go
package resource_manager

import (
    "context"
    "gpu-orchestrator/core/models"
)

type NetworkManager struct {
    // VPN mesh for control plane (multi-task distribution)
    // NOT for single-cluster training
    vpnService VPNService
}

// Setup network based on execution mode
func (nm *NetworkManager) SetupNetwork(
    ctx context.Context,
    job *Job,
    instances []Instance,
) error {
    // Check execution mode
    if job.Requirements.ExecutionMode == ModeSingleCluster {
        // Single-cluster: NO VPN - use native VPC/CNI
        // All instances are already in same VPC
        // Return early - no network setup needed
        return nil
    }
    
    // Multi-task: VPN mesh is optional for control plane
    // Tasks themselves don't communicate - only scheduler needs control plane access
    // VPN is only for:
    // - Scheduler → worker communication
    // - Log aggregation
    // - Checkpoint upload/download
    return nm.setupControlPlaneVPN(ctx, instances)
}

// Setup VPN mesh for control plane (multi-task only)
func (nm *NetworkManager) setupControlPlaneVPN(
    ctx context.Context,
    instances []Instance,
) error {
    // Use Tailscale or WireGuard for control plane VPN
    // Only scheduler/control plane nodes join VPN
    // Task workers use native provider networking
    
    for _, instance := range instances {
        // Only setup VPN on control plane nodes
        if instance.IsControlPlane {
            err := nm.setupTailscaleNode(ctx, instance)
            if err != nil {
                return err
            }
        }
    }
    
    return nil
}

func (nm *NetworkManager) setupTailscaleNode(ctx context.Context, instance Instance) error {
    // Install Tailscale on instance
    installScript := `
        curl -fsSL https://tailscale.com/install.sh | sh
        tailscale up --authkey=$TAILSCALE_AUTH_KEY
    `
    
    // Execute on instance via SSH
    return nm.executeOnInstance(ctx, instance, installScript)
}
```
<｜tool▁calls▁begin｜><｜tool▁call▁begin｜>
read_file

---

## Phase 5: Job Execution & Monitoring

### 5.1 Job Scheduler

**Admission Control**: Jobs are rejected early if infeasible (budget too low, no capacity, dataset locality required but unavailable). This prevents users from waiting forever.

```go
// core/scheduler/queue.go
package scheduler

import (
    "container/heap"
    "sync"
    "gpu-orchestrator/core/models"
)

type JobQueue struct {
    jobs  []*Job
    mu    sync.Mutex
}

func (jq *JobQueue) Push(job *Job) {
    jq.mu.Lock()
    defer jq.mu.Unlock()
    
    heap.Push(jq, job)
}

func (jq *JobQueue) Pop() *Job {
    jq.mu.Lock()
    defer jq.mu.Unlock()
    
    if jq.Len() == 0 {
        return nil
    }
    
    return heap.Pop(jq).(*Job)
}

// Implement heap.Interface
func (jq *JobQueue) Len() int { return len(jq.jobs) }
func (jq *JobQueue) Less(i, j int) bool {
    // Priority: deadline first, then budget
    // Note: Scheduler fairness (per-user quotas, starvation prevention, fairness windows)
    // is out of scope for MVP. Future enhancement.
    if jq.jobs[i].Constraints.Deadline.Before(jq.jobs[j].Constraints.Deadline) {
        return true
    }
    return jq.jobs[i].Constraints.MaxBudget < jq.jobs[j].Constraints.MaxBudget
}
func (jq *JobQueue) Swap(i, j int) {
    jq.jobs[i], jq.jobs[j] = jq.jobs[j], jq.jobs[i]
}
```

### 5.2 Monitoring & Auto-Scaling

```go
// monitoring/cost_tracker.go
package monitoring

import (
    "context"
    "time"
    "gpu-orchestrator/core/models"
)

type CostTracker struct {
    jobCosts map[string]*JobCost
    mu       sync.RWMutex
}

type JobCost struct {
    JobID         string
    StartTime     time.Time
    RunningCost   float64
    Allocations   []Allocation
    LastUpdate    time.Time
}

func (ct *CostTracker) TrackJob(ctx context.Context, job *Job, allocations []Allocation) {
    ct.mu.Lock()
    ct.jobCosts[job.ID] = &JobCost{
        JobID:       job.ID,
        StartTime:   time.Now(),
        Allocations: allocations,
        LastUpdate:  time.Now(),
    }
    ct.mu.Unlock()
    
    // Start background goroutine to update costs
    go ct.updateCosts(ctx, job.ID)
}

func (ct *CostTracker) updateCosts(ctx context.Context, jobID string) {
    ticker := time.NewTicker(1 * time.Minute)
    defer ticker.Stop()
    
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            ct.mu.Lock()
            jobCost, exists := ct.jobCosts[jobID]
            if !exists {
                ct.mu.Unlock()
                return
            }
            
            // Calculate delta time since last update
            now := time.Now()
            deltaHours := now.Sub(jobCost.LastUpdate).Hours()
            
            // Add cost for delta time only (not total elapsed)
            for _, alloc := range jobCost.Allocations {
                // PricePerHour is explicitly stored per instance
                deltaCost := alloc.PricePerHour * float64(alloc.Count) * deltaHours
                jobCost.RunningCost += deltaCost
            }
            
            jobCost.LastUpdate = now
            ct.mu.Unlock()
        }
    }
}

// monitoring/auto_scaler.go
type AutoScaler struct {
    costTracker     *CostTracker
    resourceManager *ResourceManager
    checkpointMgr   *CheckpointManager
}

func (as *AutoScaler) CheckAndScale(ctx context.Context, job *Job) error {
    // Check execution mode
    if job.Requirements.ExecutionMode == ModeMultiTask {
        // Multi-task: Can scale freely (add more tasks)
        return as.scaleMultiTask(ctx, job)
    }
    
    // Single-cluster: Limited scaling options
    // Most synchronous training (DDP, Horovod) cannot scale mid-run
    // Options:
    // 1. Elastic training (Horovod Elastic) - framework must support
    // 2. Checkpoint + restart with new world size
    
    // Check if framework supports elastic scaling
    if job.Framework == "horovod_elastic" {
        progress := as.getJobProgress(ctx, job.ID)
        currentCost := as.costTracker.GetRunningCost(job.ID)
        
        if progress.IsSlow() && currentCost < job.Constraints.MaxBudget*0.8 {
            // Horovod Elastic can add/remove workers dynamically
            return as.scaleUpElastic(ctx, job, 2)
        }
    }
    
    // For other frameworks (DDP, TensorFlow), scaling requires checkpoint + restart
    // This is handled by considerRelaunch (checkpoint + restart with new allocation)
    
    // Check if cheaper option available (checkpoint + relaunch)
    if as.cheaperOptionAvailable(ctx, job) {
        return as.considerRelaunch(ctx, job)
    }
    
    return nil
}

// Scale multi-task jobs (easy - just add more tasks)
func (as *AutoScaler) scaleMultiTask(ctx context.Context, job *Job) error {
    // Multi-task jobs can scale freely - add more tasks to different providers
    // This doesn't affect running tasks, just adds more parallel tasks
    // Implementation depends on task distributor
    return nil // Placeholder
}

// Scale elastic training (only for Horovod Elastic)
func (as *AutoScaler) scaleUpElastic(ctx context.Context, job *Job, additionalGPUs int) error {
    // Horovod Elastic can add/remove workers dynamically
    // Add new nodes to cluster and they'll join training
    return as.resourceManager.AddNodesToCluster(ctx, job.ClusterID, additionalGPUs)
}
```

---

## Technology Stack Recommendations

### Core Platform
- **Language**: Go (concurrent, good for orchestration) or Python (easier ML integration)
- **API**: REST (Go) or FastAPI (Python)
- **Database**: PostgreSQL (jobs, metadata) + Redis (queue, caching)
- **Message Queue**: RabbitMQ or NATS (job queue)

### Cloud SDKs
- **AWS**: AWS SDK for Go/Python
- **GCP**: Google Cloud SDK
- **Azure**: Azure SDK for Go/Python

### Training Frameworks
- **PyTorch**: Native DDP support
- **TensorFlow**: MultiWorkerMirroredStrategy
- **Horovod**: Framework-agnostic distributed training

### Networking
- **VPN**: Tailscale (easiest) or WireGuard
- **Cloud Interconnect**: For lower latency (AWS Direct Connect, GCP Interconnect)

### Monitoring
- **Metrics**: Prometheus + Grafana
- **Logging**: ELK Stack or Cloud Logging
- **Cost Tracking**: Custom + Cloud Cost APIs

### Infrastructure as Code
- **Terraform**: For provisioning infrastructure
- **Kubernetes**: For on-premise GPU management

---

## Implementation Roadmap

### Week 1-2: Foundation
- [ ] Set up project structure
- [ ] Implement basic data models
- [ ] Set up database schema
- [ ] Create REST API skeleton

### Week 3-4: Provider Integration
- [ ] Implement AWS provider (pricing + provisioning)
- [ ] Implement GCP provider
- [ ] Implement Azure provider
- [ ] Test provider integrations

### Week 5-6: Optimization Engine
- [ ] Implement pricing fetcher
- [ ] Implement cost calculator
- [ ] Implement allocation optimizer
- [ ] Test optimization algorithms

### Week 7-8: Resource Management
- [ ] Implement GPU provisioner
- [ ] Implement network manager (VPN setup)
- [ ] Implement storage manager
- [ ] Test multi-cloud provisioning

### Week 9-10: Training Orchestration
- [ ] Implement PyTorch DDP setup
- [ ] Implement TensorFlow distributed setup
- [ ] Implement Horovod setup
- [ ] Test distributed training

### Week 11-12: Monitoring & Auto-Scaling
- [ ] Implement cost tracking
- [ ] Implement job monitoring
- [ ] Implement auto-scaling
- [ ] Implement cost alerts

### Week 13-14: Testing & Optimization
- [ ] End-to-end testing
- [ ] Performance optimization
- [ ] Cost optimization tuning
- [ ] Documentation

---

## Summary: Realistic Implementation Approach

### ✅ What Works (Keep These)

1. **Control Plane + Provider Adapters**: GPU abstraction layer is correct architecture
2. **Cost Optimization**: Budget, deadlines, spot tolerance - all good
3. **On-Prem First, Cloud Burst**: Very practical and valuable
4. **Checkpointing + Restart**: Mandatory for spot/preemptible instances
5. **Single-Cluster Training**: Optimize provider/region, run training in one cluster
6. **Multi-Cloud Task Distribution**: HPO sweeps, batch inference, evaluation runs

### ❌ What Doesn't Work (Remove/Change)

1. **Cross-Cloud Synchronous Training**: Latency kills all-reduce - DON'T DO THIS
2. **Model Parallelism Across Clouds**: Needs high-throughput links - KEEP IN ONE CLUSTER
3. **Live Migration Mid-Job**: Not possible - USE CHECKPOINT + RELAUNCH instead
4. **Hardcoded Pricing**: Use provider APIs + database storage

### 🔄 What to Change

1. **Execution Modes**: 
   - Mode 1: Single-cluster training (default for DDP/Horovod)
   - Mode 2: Multi-cloud task distribution (for HPO, inference, eval)

2. **Optimization Metric**: 
   - Use $/step or $/token, not just $/hour
   - Include performance metrics (storage throughput, network bandwidth)

3. **Data Locality**: 
   - Prefer compute where dataset exists
   - Account for data transfer costs in optimization

4. **Pricing Storage**: 
   - Store in database, refresh from provider APIs
   - Track historical pricing for trends

5. **Migration Strategy**: 
   - Checkpoint → Stop → Resume (not live migration)
   - Only if savings > switching cost

### 🎯 MVP Scope Lock (Non-Negotiable)

**MVP = Production-Ready Minimum Viable Product**

**✅ IN SCOPE (Must Have):**

1. **Single-cluster training only**
   - PyTorch DDP, Horovod (no TensorFlow MultiWorker initially)
   - AWS + on-premise only (no GCP/Azure initially)
   - Native VPC/CNI networking (NO VPN for training)

2. **Basic cost optimization**
   - On-demand + spot pricing
   - Budget + deadline constraints
   - Spot + checkpoint + resume

3. **Checkpoint + resume**
   - S3/MinIO checkpoint storage (s3:// or minio:// URIs)
   - Resume on different provider/region if cheaper
   - Note: GCS (gs://) support is Phase 2 (not MVP)

4. **Cost reporting**
   - Real-time cost tracking ($/hour, $/step if available)
   - Job cost alerts
   - Budget enforcement

5. **Backend: Raw VMs only** (no Kubernetes/Slurm/Ray)
   - Provision EC2 instances directly
   - On-premise: Basic GPU discovery (no K8s initially)

**❌ OUT OF SCOPE (Future):**

1. **Multi-cloud task distribution** (HPO, inference, eval) - Week 13+
2. **Azure / GCP** - Week 9+
3. **Kubernetes / Slurm / Ray backends** - Week 15+
4. **Live elasticity** (except Horovod Elastic) - Week 11+
5. **Fancy geo strategies** - Week 14+
6. **TensorFlow MultiWorker** - Week 12+
7. **Performance metrics from telemetry** - Phase 2 (use static benchmarks for MVP)

**MVP Timeline: 4-6 weeks**

1. **Week 1-2**: Foundation + AWS provider + basic optimizer
2. **Week 3**: On-premise support
3. **Week 4**: Checkpoint + resume
4. **Week 5**: Cost tracking + alerts
5. **Week 6**: Polish, testing, documentation

### 📊 Example Customer Jobs

**Job 1: Fine-tune LLM (8x A100, 6 hours) - MVP Example**
- Execution Mode: Single-cluster
- Optimizer selects: AWS us-east-1 (cheapest A100 spot)
- Runs: PyTorch DDP within single cluster
- Checkpoints: Every 1000 steps (to S3)
- Cost: ~$72

**Job 2: HPO Sweep (200 trials, 1 GPU each) - Phase 2 Example**
- Execution Mode: Multi-task distribution
- Distributes: 80 → AWS spot, 70 → GCP preemptible, 50 → on-prem
- Runs: All trials concurrently
- Aggregates: Best hyperparameters
- Cost: ~$45 (vs $120 if all on-demand)
- Note: Multi-cloud task distribution is Phase 2 (not MVP)

**Job 3: Batch Inference (10K images) - Phase 2 Example**
- Execution Mode: Multi-task distribution
- Distributes: Images across AWS, GCP, Azure
- Runs: Parallel inference
- Cost: Optimized per-image processing cost
- Note: Azure/GCP support is Phase 2 (not MVP)

---

## Next Steps

1. **Define First Customer Job**: What's the actual use case? (Fine-tune LLM? Train ResNet? HPO?)
2. **Build MVP**: Single-cluster training, AWS + on-prem, basic optimization
3. **Test with Real Job**: Validate end-to-end workflow
4. **Iterate**: Add features based on customer feedback

**Ready to start?** I can help you:
- Create detailed MVP implementation plan
- Start implementing specific components
- Design the job specification format
- Set up the database schema
