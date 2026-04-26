CREATE TABLE IF NOT EXISTS experiment_run_evidence_bundles (
  bundle_id TEXT PRIMARY KEY,
  run_id TEXT NOT NULL REFERENCES experiment_runs(run_id),
  bundle_object_key TEXT NOT NULL,
  report_object_key TEXT NOT NULL,
  bundle_sha256 TEXT NOT NULL,
  bundle_size_bytes BIGINT NOT NULL,
  report_sha256 TEXT NOT NULL,
  report_size_bytes BIGINT NOT NULL,
  signature TEXT NOT NULL,
  signature_alg TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_by TEXT NOT NULL,
  integrity_sha256 TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_evidence_bundles_run_id ON experiment_run_evidence_bundles (run_id, created_at DESC);
