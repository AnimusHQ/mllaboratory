CREATE TABLE IF NOT EXISTS quality_rules (
  rule_id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  description TEXT,
  spec JSONB NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_by TEXT NOT NULL,
  integrity_sha256 TEXT NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_quality_rules_name_unique ON quality_rules (name);
CREATE INDEX IF NOT EXISTS idx_quality_rules_created_at ON quality_rules (created_at DESC);

ALTER TABLE dataset_versions
  ADD COLUMN IF NOT EXISTS quality_rule_id TEXT REFERENCES quality_rules(rule_id);

CREATE INDEX IF NOT EXISTS idx_dataset_versions_quality_rule_id ON dataset_versions (quality_rule_id);

CREATE TABLE IF NOT EXISTS quality_evaluations (
  evaluation_id TEXT PRIMARY KEY,
  dataset_version_id TEXT NOT NULL REFERENCES dataset_versions(version_id),
  rule_id TEXT NOT NULL REFERENCES quality_rules(rule_id),
  status TEXT NOT NULL CHECK (status IN ('pass','fail','error')),
  evaluated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  evaluated_by TEXT NOT NULL,
  summary JSONB NOT NULL,
  report_object_key TEXT NOT NULL,
  report_sha256 TEXT NOT NULL,
  report_size_bytes BIGINT NOT NULL,
  integrity_sha256 TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_quality_evaluations_dataset_version ON quality_evaluations (dataset_version_id, evaluated_at DESC);
CREATE INDEX IF NOT EXISTS idx_quality_evaluations_rule ON quality_evaluations (rule_id, evaluated_at DESC);
CREATE INDEX IF NOT EXISTS idx_quality_evaluations_status ON quality_evaluations (status);

