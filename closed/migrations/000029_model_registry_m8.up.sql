ALTER TABLE models
  ADD COLUMN IF NOT EXISTS idempotency_key TEXT;

CREATE UNIQUE INDEX IF NOT EXISTS idx_models_project_idempotency_key ON models (project_id, idempotency_key);

CREATE TABLE IF NOT EXISTS model_versions (
  model_version_id TEXT PRIMARY KEY,
  project_id TEXT NOT NULL,
  model_id TEXT NOT NULL,
  version TEXT NOT NULL,
  status TEXT NOT NULL,
  run_id TEXT NOT NULL,
  artifact_ids JSONB NOT NULL DEFAULT '[]'::jsonb,
  dataset_version_ids JSONB NOT NULL DEFAULT '[]'::jsonb,
  env_lock_id TEXT,
  code_ref JSONB NOT NULL DEFAULT '{}'::jsonb,
  policy_snapshot_sha256 TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_by TEXT NOT NULL,
  idempotency_key TEXT NOT NULL,
  integrity_sha256 TEXT NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_model_versions_project_idempotency_key ON model_versions (project_id, idempotency_key);
CREATE UNIQUE INDEX IF NOT EXISTS idx_model_versions_project_model_version_unique ON model_versions (project_id, model_id, version);
CREATE INDEX IF NOT EXISTS idx_model_versions_project_status ON model_versions (project_id, status);
CREATE INDEX IF NOT EXISTS idx_model_versions_project_created_at ON model_versions (project_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_model_versions_model_id ON model_versions (model_id);

CREATE TABLE IF NOT EXISTS model_version_transitions (
  transition_id BIGSERIAL PRIMARY KEY,
  project_id TEXT NOT NULL,
  model_version_id TEXT NOT NULL,
  from_status TEXT NOT NULL,
  to_status TEXT NOT NULL,
  action TEXT NOT NULL,
  request_id TEXT,
  occurred_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  actor TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_model_version_transitions_version_id ON model_version_transitions (model_version_id, occurred_at DESC);

CREATE TABLE IF NOT EXISTS model_exports (
  export_id TEXT PRIMARY KEY,
  project_id TEXT NOT NULL,
  model_version_id TEXT NOT NULL,
  status TEXT NOT NULL,
  target TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_by TEXT NOT NULL,
  idempotency_key TEXT NOT NULL,
  integrity_sha256 TEXT NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_model_exports_project_idempotency_key ON model_exports (project_id, idempotency_key);
CREATE INDEX IF NOT EXISTS idx_model_exports_project_version ON model_exports (project_id, model_version_id);
CREATE INDEX IF NOT EXISTS idx_model_exports_project_created_at ON model_exports (project_id, created_at DESC);

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_model_versions_project') THEN
    ALTER TABLE model_versions ADD CONSTRAINT fk_model_versions_project FOREIGN KEY (project_id) REFERENCES projects(project_id);
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_model_versions_model') THEN
    ALTER TABLE model_versions ADD CONSTRAINT fk_model_versions_model FOREIGN KEY (model_id) REFERENCES models(model_id);
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_model_version_transitions_project') THEN
    ALTER TABLE model_version_transitions ADD CONSTRAINT fk_model_version_transitions_project FOREIGN KEY (project_id) REFERENCES projects(project_id);
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_model_version_transitions_version') THEN
    ALTER TABLE model_version_transitions ADD CONSTRAINT fk_model_version_transitions_version FOREIGN KEY (model_version_id) REFERENCES model_versions(model_version_id);
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_model_exports_project') THEN
    ALTER TABLE model_exports ADD CONSTRAINT fk_model_exports_project FOREIGN KEY (project_id) REFERENCES projects(project_id);
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_model_exports_version') THEN
    ALTER TABLE model_exports ADD CONSTRAINT fk_model_exports_version FOREIGN KEY (model_version_id) REFERENCES model_versions(model_version_id);
  END IF;
END $$;
