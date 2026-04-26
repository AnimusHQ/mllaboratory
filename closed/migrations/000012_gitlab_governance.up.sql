CREATE TABLE IF NOT EXISTS gitlab_governance_events (
  governance_id TEXT PRIMARY KEY,
  repo TEXT NOT NULL,
  commit_sha TEXT NOT NULL,
  ref TEXT,
  ref_protected BOOLEAN,
  project_id BIGINT,
  project_path TEXT,
  pipeline_id BIGINT,
  pipeline_url TEXT,
  pipeline_status TEXT,
  pipeline_source TEXT,
  merge_request_id BIGINT,
  merge_request_iid BIGINT,
  merge_request_url TEXT,
  merge_request_source_branch TEXT,
  merge_request_target_branch TEXT,
  approvals_required INTEGER,
  approvals_received INTEGER,
  approvals_approved BOOLEAN,
  event_type TEXT NOT NULL,
  payload JSONB NOT NULL,
  payload_sha256 TEXT NOT NULL,
  received_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  received_by TEXT NOT NULL,
  signature TEXT NOT NULL,
  integrity_sha256 TEXT NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_gitlab_governance_payload_unique ON gitlab_governance_events (payload_sha256);
CREATE INDEX IF NOT EXISTS idx_gitlab_governance_repo_commit ON gitlab_governance_events (repo, commit_sha, ref, received_at DESC);
CREATE INDEX IF NOT EXISTS idx_gitlab_governance_pipeline ON gitlab_governance_events (pipeline_id);
