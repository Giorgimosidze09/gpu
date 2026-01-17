-- Migration: Add team/project attribution for cost tracking
-- This enables cost attribution per team/workload (like Run:AI/Cast AI)

-- Add team and project columns to jobs table
ALTER TABLE jobs 
  ADD COLUMN IF NOT EXISTS team_id text,
  ADD COLUMN IF NOT EXISTS project_id text;

-- Create indexes for cost attribution queries
CREATE INDEX IF NOT EXISTS idx_jobs_team ON jobs (team_id);
CREATE INDEX IF NOT EXISTS idx_jobs_project ON jobs (project_id);
CREATE INDEX IF NOT EXISTS idx_jobs_team_project ON jobs (team_id, project_id);

-- Add comment
COMMENT ON COLUMN jobs.team_id IS 'Team identifier for cost attribution';
COMMENT ON COLUMN jobs.project_id IS 'Project identifier for cost attribution';
