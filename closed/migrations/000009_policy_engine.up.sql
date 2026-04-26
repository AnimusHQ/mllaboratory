CREATE TABLE IF NOT EXISTS policies (
  policy_id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  description TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_by TEXT NOT NULL,
  integrity_sha256 TEXT NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_policies_name_unique ON policies (name);
CREATE INDEX IF NOT EXISTS idx_policies_created_at ON policies (created_at DESC);

CREATE TABLE IF NOT EXISTS policy_versions (
  policy_version_id TEXT PRIMARY KEY,
  policy_id TEXT NOT NULL REFERENCES policies(policy_id),
  version INTEGER NOT NULL,
  status TEXT NOT NULL,
  spec_yaml TEXT NOT NULL,
  spec_json JSONB NOT NULL DEFAULT '{}'::jsonb,
  spec_sha256 TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_by TEXT NOT NULL,
  integrity_sha256 TEXT NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_policy_versions_policy_version_unique ON policy_versions (policy_id, version);
CREATE INDEX IF NOT EXISTS idx_policy_versions_policy_created_at ON policy_versions (policy_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_policy_versions_status ON policy_versions (status);

CREATE TABLE IF NOT EXISTS policy_decisions (
  decision_id TEXT PRIMARY KEY,
  run_id TEXT REFERENCES experiment_runs(run_id),
  policy_id TEXT NOT NULL REFERENCES policies(policy_id),
  policy_version_id TEXT NOT NULL REFERENCES policy_versions(policy_version_id),
  policy_sha256 TEXT NOT NULL,
  context JSONB NOT NULL DEFAULT '{}'::jsonb,
  context_sha256 TEXT NOT NULL,
  decision TEXT NOT NULL,
  rule_id TEXT,
  reason TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_by TEXT NOT NULL,
  integrity_sha256 TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_policy_decisions_run_id ON policy_decisions (run_id);
CREATE INDEX IF NOT EXISTS idx_policy_decisions_created_at ON policy_decisions (created_at DESC);
CREATE INDEX IF NOT EXISTS idx_policy_decisions_policy ON policy_decisions (policy_id, created_at DESC);

CREATE TABLE IF NOT EXISTS policy_approvals (
  approval_id TEXT PRIMARY KEY,
  decision_id TEXT NOT NULL REFERENCES policy_decisions(decision_id),
  run_id TEXT REFERENCES experiment_runs(run_id),
  status TEXT NOT NULL,
  requested_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  requested_by TEXT NOT NULL,
  decided_at TIMESTAMPTZ,
  decided_by TEXT,
  reason TEXT,
  integrity_sha256 TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_policy_approvals_status ON policy_approvals (status, requested_at DESC);
CREATE INDEX IF NOT EXISTS idx_policy_approvals_run_id ON policy_approvals (run_id);
