CREATE TABLE IF NOT EXISTS projects (
  project_id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  description TEXT,
  metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_by TEXT NOT NULL,
  integrity_sha256 TEXT NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_projects_name_unique ON projects (name);
CREATE INDEX IF NOT EXISTS idx_projects_created_at ON projects (created_at);

ALTER TABLE datasets
  ADD COLUMN IF NOT EXISTS project_id TEXT;

ALTER TABLE dataset_versions
  ADD COLUMN IF NOT EXISTS project_id TEXT;

ALTER TABLE experiments
  ADD COLUMN IF NOT EXISTS project_id TEXT;

ALTER TABLE experiment_runs
  ADD COLUMN IF NOT EXISTS project_id TEXT;

ALTER TABLE experiment_run_artifacts
  ADD COLUMN IF NOT EXISTS project_id TEXT;

ALTER TABLE experiment_run_artifacts
  ADD COLUMN IF NOT EXISTS retention_until TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS retention_policy TEXT;

CREATE INDEX IF NOT EXISTS idx_datasets_project_id ON datasets (project_id);
CREATE INDEX IF NOT EXISTS idx_dataset_versions_project_id ON dataset_versions (project_id);
CREATE INDEX IF NOT EXISTS idx_experiments_project_id ON experiments (project_id);
CREATE INDEX IF NOT EXISTS idx_experiment_runs_project_id ON experiment_runs (project_id);
CREATE INDEX IF NOT EXISTS idx_experiment_run_artifacts_project_id ON experiment_run_artifacts (project_id);
CREATE INDEX IF NOT EXISTS idx_datasets_project_created_at ON datasets (project_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_dataset_versions_project_dataset ON dataset_versions (project_id, dataset_id);
CREATE INDEX IF NOT EXISTS idx_dataset_versions_project_created_at ON dataset_versions (project_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_experiments_project_created_at ON experiments (project_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_experiment_runs_project_started_at ON experiment_runs (project_id, started_at DESC);
CREATE INDEX IF NOT EXISTS idx_experiment_run_artifacts_project_created_at ON experiment_run_artifacts (project_id, created_at DESC);

CREATE TABLE IF NOT EXISTS models (
  model_id TEXT PRIMARY KEY,
  project_id TEXT NOT NULL,
  name TEXT NOT NULL,
  version TEXT NOT NULL,
  status TEXT NOT NULL,
  artifact_id TEXT,
  metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_by TEXT NOT NULL,
  integrity_sha256 TEXT NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_models_project_name_version_unique ON models (project_id, name, version);
CREATE INDEX IF NOT EXISTS idx_models_project_status ON models (project_id, status);
CREATE INDEX IF NOT EXISTS idx_models_created_at ON models (created_at);
CREATE INDEX IF NOT EXISTS idx_models_project_created_at ON models (project_id, created_at DESC);

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_datasets_project') THEN
    ALTER TABLE datasets ADD CONSTRAINT fk_datasets_project FOREIGN KEY (project_id) REFERENCES projects(project_id);
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_dataset_versions_project') THEN
    ALTER TABLE dataset_versions ADD CONSTRAINT fk_dataset_versions_project FOREIGN KEY (project_id) REFERENCES projects(project_id);
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_experiments_project') THEN
    ALTER TABLE experiments ADD CONSTRAINT fk_experiments_project FOREIGN KEY (project_id) REFERENCES projects(project_id);
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_experiment_runs_project') THEN
    ALTER TABLE experiment_runs ADD CONSTRAINT fk_experiment_runs_project FOREIGN KEY (project_id) REFERENCES projects(project_id);
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_experiment_run_artifacts_project') THEN
    ALTER TABLE experiment_run_artifacts ADD CONSTRAINT fk_experiment_run_artifacts_project FOREIGN KEY (project_id) REFERENCES projects(project_id);
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_models_project') THEN
    ALTER TABLE models ADD CONSTRAINT fk_models_project FOREIGN KEY (project_id) REFERENCES projects(project_id);
  END IF;
END $$;

CREATE OR REPLACE FUNCTION prevent_update_delete() RETURNS trigger AS $$
BEGIN
  RAISE EXCEPTION 'immutable table';
END;
$$ LANGUAGE plpgsql;

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'trg_dataset_versions_immutable') THEN
    CREATE TRIGGER trg_dataset_versions_immutable
      BEFORE UPDATE OR DELETE ON dataset_versions
      FOR EACH ROW EXECUTE FUNCTION prevent_update_delete();
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'trg_audit_events_immutable') THEN
    CREATE TRIGGER trg_audit_events_immutable
      BEFORE UPDATE OR DELETE ON audit_events
      FOR EACH ROW EXECUTE FUNCTION prevent_update_delete();
  END IF;
END $$;
