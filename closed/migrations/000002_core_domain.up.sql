CREATE TABLE IF NOT EXISTS datasets (
  dataset_id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  description TEXT,
  metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_by TEXT NOT NULL,
  integrity_sha256 TEXT NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_datasets_name_unique ON datasets (name);
CREATE INDEX IF NOT EXISTS idx_datasets_created_at ON datasets (created_at);

CREATE TABLE IF NOT EXISTS dataset_versions (
  version_id TEXT PRIMARY KEY,
  dataset_id TEXT NOT NULL REFERENCES datasets(dataset_id),
  ordinal BIGINT NOT NULL,
  content_sha256 TEXT NOT NULL,
  object_key TEXT NOT NULL,
  size_bytes BIGINT,
  metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_by TEXT NOT NULL,
  integrity_sha256 TEXT NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_dataset_versions_dataset_ordinal_unique ON dataset_versions (dataset_id, ordinal);
CREATE UNIQUE INDEX IF NOT EXISTS idx_dataset_versions_dataset_content_unique ON dataset_versions (dataset_id, content_sha256);
CREATE INDEX IF NOT EXISTS idx_dataset_versions_created_at ON dataset_versions (created_at);

CREATE TABLE IF NOT EXISTS experiments (
  experiment_id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  description TEXT,
  metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_by TEXT NOT NULL,
  integrity_sha256 TEXT NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_experiments_name_unique ON experiments (name);
CREATE INDEX IF NOT EXISTS idx_experiments_created_at ON experiments (created_at);

CREATE TABLE IF NOT EXISTS experiment_runs (
  run_id TEXT PRIMARY KEY,
  experiment_id TEXT NOT NULL REFERENCES experiments(experiment_id),
  dataset_version_id TEXT REFERENCES dataset_versions(version_id),
  status TEXT NOT NULL,
  started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  ended_at TIMESTAMPTZ,
  git_repo TEXT,
  git_commit TEXT,
  git_ref TEXT,
  params JSONB NOT NULL DEFAULT '{}'::jsonb,
  metrics JSONB NOT NULL DEFAULT '{}'::jsonb,
  artifacts_prefix TEXT,
  integrity_sha256 TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_experiment_runs_experiment_started ON experiment_runs (experiment_id, started_at DESC);
CREATE INDEX IF NOT EXISTS idx_experiment_runs_dataset_version ON experiment_runs (dataset_version_id);
CREATE INDEX IF NOT EXISTS idx_experiment_runs_git_commit ON experiment_runs (git_commit);

CREATE TABLE IF NOT EXISTS lineage_events (
  event_id BIGSERIAL PRIMARY KEY,
  occurred_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  actor TEXT NOT NULL,
  request_id TEXT,
  subject_type TEXT NOT NULL,
  subject_id TEXT NOT NULL,
  predicate TEXT NOT NULL,
  object_type TEXT NOT NULL,
  object_id TEXT NOT NULL,
  metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
  integrity_sha256 TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_lineage_events_occurred_at ON lineage_events (occurred_at DESC);
CREATE INDEX IF NOT EXISTS idx_lineage_events_subject ON lineage_events (subject_type, subject_id);
CREATE INDEX IF NOT EXISTS idx_lineage_events_object ON lineage_events (object_type, object_id);

