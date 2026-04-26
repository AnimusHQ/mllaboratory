CREATE TABLE IF NOT EXISTS execution_ledger_entries (
  ledger_id TEXT PRIMARY KEY,
  run_id TEXT NOT NULL REFERENCES experiment_runs(run_id),
  execution_id TEXT NOT NULL REFERENCES experiment_run_executions(execution_id),
  entry JSONB NOT NULL,
  entry_sha256 TEXT NOT NULL,
  execution_hash TEXT NOT NULL,
  replay_bundle JSONB NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_by TEXT NOT NULL,
  integrity_sha256 TEXT NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_execution_ledger_entries_run_unique ON execution_ledger_entries (run_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_execution_ledger_entries_execution_unique ON execution_ledger_entries (execution_id);
CREATE INDEX IF NOT EXISTS idx_execution_ledger_entries_created_at ON execution_ledger_entries (created_at DESC);
CREATE INDEX IF NOT EXISTS idx_execution_ledger_entries_hash ON execution_ledger_entries (execution_hash);
