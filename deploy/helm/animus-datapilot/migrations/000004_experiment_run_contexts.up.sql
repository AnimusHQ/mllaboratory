CREATE TABLE IF NOT EXISTS experiment_run_contexts (
  context_id TEXT PRIMARY KEY,
  run_id TEXT NOT NULL REFERENCES experiment_runs(run_id),
  received_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  received_by TEXT NOT NULL,
  provider TEXT,
  signature_ts BIGINT NOT NULL,
  signature TEXT NOT NULL,
  payload JSONB NOT NULL,
  payload_sha256 TEXT NOT NULL,
  integrity_sha256 TEXT NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_experiment_run_contexts_run_payload_unique ON experiment_run_contexts (run_id, payload_sha256);
CREATE INDEX IF NOT EXISTS idx_experiment_run_contexts_run_received ON experiment_run_contexts (run_id, received_at DESC);
CREATE INDEX IF NOT EXISTS idx_experiment_run_contexts_received_at ON experiment_run_contexts (received_at DESC);

