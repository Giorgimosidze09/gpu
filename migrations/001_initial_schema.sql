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
