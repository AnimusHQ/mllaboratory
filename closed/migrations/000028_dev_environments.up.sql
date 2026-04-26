CREATE TABLE IF NOT EXISTS dev_environments (
  dev_env_id TEXT PRIMARY KEY,
  project_id TEXT NOT NULL,
  template_ref TEXT NOT NULL,
  template_version INTEGER NOT NULL,
  template_integrity_sha256 TEXT NOT NULL,
  image_name TEXT NOT NULL,
  image_ref TEXT NOT NULL,
  ttl_seconds INTEGER NOT NULL,
  state TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_by TEXT NOT NULL,
  last_access_at TIMESTAMPTZ,
  expires_at TIMESTAMPTZ NOT NULL,
  policy_snapshot_sha256 TEXT NOT NULL,
  dp_job_name TEXT,
  dp_namespace TEXT,
  idempotency_key TEXT NOT NULL,
  integrity_sha256 TEXT NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_dev_envs_project_idem ON dev_environments (project_id, idempotency_key);
CREATE INDEX IF NOT EXISTS idx_dev_envs_project_created_at ON dev_environments (project_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_dev_envs_project_state_exp ON dev_environments (project_id, state, expires_at);

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_dev_envs_project') THEN
    ALTER TABLE dev_environments ADD CONSTRAINT fk_dev_envs_project FOREIGN KEY (project_id) REFERENCES projects(project_id);
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_dev_envs_template') THEN
    ALTER TABLE dev_environments ADD CONSTRAINT fk_dev_envs_template FOREIGN KEY (template_ref) REFERENCES environment_definitions(environment_definition_id);
  END IF;
END $$;

CREATE TABLE IF NOT EXISTS dev_env_policy_snapshots (
  snapshot_id TEXT PRIMARY KEY,
  dev_env_id TEXT NOT NULL,
  project_id TEXT NOT NULL,
  snapshot JSONB NOT NULL,
  snapshot_sha256 TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_by TEXT NOT NULL,
  integrity_sha256 TEXT NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_dev_env_policy_snapshots_env_id ON dev_env_policy_snapshots (dev_env_id);
CREATE INDEX IF NOT EXISTS idx_dev_env_policy_snapshots_project_created_at ON dev_env_policy_snapshots (project_id, created_at DESC);

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_dev_env_policy_project') THEN
    ALTER TABLE dev_env_policy_snapshots ADD CONSTRAINT fk_dev_env_policy_project FOREIGN KEY (project_id) REFERENCES projects(project_id);
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_dev_env_policy_env') THEN
    ALTER TABLE dev_env_policy_snapshots ADD CONSTRAINT fk_dev_env_policy_env FOREIGN KEY (dev_env_id) REFERENCES dev_environments(dev_env_id);
  END IF;
END $$;

CREATE TABLE IF NOT EXISTS dev_env_access_sessions (
  session_id TEXT PRIMARY KEY,
  dev_env_id TEXT NOT NULL,
  project_id TEXT NOT NULL,
  issued_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  expires_at TIMESTAMPTZ NOT NULL,
  issued_by TEXT NOT NULL,
  integrity_sha256 TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_dev_env_access_sessions_env_id ON dev_env_access_sessions (dev_env_id, issued_at DESC);
CREATE INDEX IF NOT EXISTS idx_dev_env_access_sessions_project_exp ON dev_env_access_sessions (project_id, expires_at);

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_dev_env_sessions_project') THEN
    ALTER TABLE dev_env_access_sessions ADD CONSTRAINT fk_dev_env_sessions_project FOREIGN KEY (project_id) REFERENCES projects(project_id);
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_dev_env_sessions_env') THEN
    ALTER TABLE dev_env_access_sessions ADD CONSTRAINT fk_dev_env_sessions_env FOREIGN KEY (dev_env_id) REFERENCES dev_environments(dev_env_id);
  END IF;
END $$;

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'trg_dev_env_policy_no_update') THEN
    CREATE TRIGGER trg_dev_env_policy_no_update
      BEFORE UPDATE ON dev_env_policy_snapshots
      FOR EACH ROW EXECUTE FUNCTION prevent_run_binding_update();
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'trg_dev_env_policy_no_delete') THEN
    CREATE TRIGGER trg_dev_env_policy_no_delete
      BEFORE DELETE ON dev_env_policy_snapshots
      FOR EACH ROW EXECUTE FUNCTION prevent_run_binding_delete();
  END IF;

  IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'trg_dev_env_sessions_no_update') THEN
    CREATE TRIGGER trg_dev_env_sessions_no_update
      BEFORE UPDATE ON dev_env_access_sessions
      FOR EACH ROW EXECUTE FUNCTION prevent_run_binding_update();
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'trg_dev_env_sessions_no_delete') THEN
    CREATE TRIGGER trg_dev_env_sessions_no_delete
      BEFORE DELETE ON dev_env_access_sessions
      FOR EACH ROW EXECUTE FUNCTION prevent_run_binding_delete();
  END IF;
END $$;
