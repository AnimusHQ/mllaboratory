CREATE TABLE IF NOT EXISTS model_images (
  image_digest TEXT PRIMARY KEY,
  repo TEXT NOT NULL,
  commit_sha TEXT NOT NULL,
  pipeline_id TEXT NOT NULL,
  provider TEXT,
  received_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  received_by TEXT NOT NULL,
  signature_ts BIGINT NOT NULL,
  signature TEXT NOT NULL,
  payload JSONB NOT NULL,
  payload_sha256 TEXT NOT NULL,
  integrity_sha256 TEXT NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_model_images_payload_unique ON model_images (payload_sha256);
CREATE INDEX IF NOT EXISTS idx_model_images_received_at ON model_images (received_at DESC);
CREATE INDEX IF NOT EXISTS idx_model_images_commit_sha ON model_images (commit_sha);
CREATE INDEX IF NOT EXISTS idx_model_images_repo ON model_images (repo);

ALTER TABLE experiment_run_executions
  ADD COLUMN IF NOT EXISTS image_digest TEXT;

CREATE INDEX IF NOT EXISTS idx_experiment_run_executions_image_digest ON experiment_run_executions (image_digest);
