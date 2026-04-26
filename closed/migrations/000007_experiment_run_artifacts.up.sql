CREATE TABLE IF NOT EXISTS experiment_run_artifacts (
  artifact_id TEXT PRIMARY KEY,
  run_id TEXT NOT NULL REFERENCES experiment_runs(run_id),
  kind TEXT NOT NULL,
  name TEXT,
  filename TEXT,
  content_type TEXT,
  object_key TEXT NOT NULL,
  sha256 TEXT NOT NULL,
  size_bytes BIGINT NOT NULL,
  metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL,
  created_by TEXT NOT NULL,
  integrity_sha256 TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_experiment_run_artifacts_run_id_created_at ON experiment_run_artifacts (run_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_experiment_run_artifacts_kind ON experiment_run_artifacts (kind);
