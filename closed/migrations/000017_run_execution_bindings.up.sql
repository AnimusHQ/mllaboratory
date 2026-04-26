CREATE TABLE IF NOT EXISTS environment_definitions (
  environment_definition_id TEXT PRIMARY KEY,
  project_id TEXT NOT NULL,
  name TEXT NOT NULL,
  version INTEGER NOT NULL DEFAULT 1,
  description TEXT,
  base_image_ref TEXT NOT NULL,
  metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_by TEXT NOT NULL,
  integrity_sha256 TEXT NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_env_definitions_project_name_version ON environment_definitions (project_id, name, version);
CREATE INDEX IF NOT EXISTS idx_env_definitions_project_created_at ON environment_definitions (project_id, created_at DESC);

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_env_definitions_project') THEN
    ALTER TABLE environment_definitions ADD CONSTRAINT fk_env_definitions_project FOREIGN KEY (project_id) REFERENCES projects(project_id);
  END IF;
END $$;

CREATE TABLE IF NOT EXISTS run_code_refs (
  run_id TEXT PRIMARY KEY,
  project_id TEXT NOT NULL,
  repo_url TEXT NOT NULL,
  commit_sha TEXT NOT NULL,
  path TEXT,
  scm_type TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_by TEXT NOT NULL,
  integrity_sha256 TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_run_code_refs_project_run ON run_code_refs (project_id, run_id);
CREATE INDEX IF NOT EXISTS idx_run_code_refs_project_commit ON run_code_refs (project_id, commit_sha);

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_run_code_refs_project') THEN
    ALTER TABLE run_code_refs ADD CONSTRAINT fk_run_code_refs_project FOREIGN KEY (project_id) REFERENCES projects(project_id);
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_run_code_refs_run') THEN
    ALTER TABLE run_code_refs ADD CONSTRAINT fk_run_code_refs_run FOREIGN KEY (run_id) REFERENCES runs(run_id);
  END IF;
END $$;

CREATE TABLE IF NOT EXISTS run_environment_locks (
  lock_id TEXT PRIMARY KEY,
  run_id TEXT NOT NULL,
  project_id TEXT NOT NULL,
  env_hash TEXT NOT NULL,
  env_template_id TEXT,
  image_digests JSONB NOT NULL DEFAULT '{}'::jsonb,
  dependency_checksums JSONB NOT NULL DEFAULT '{}'::jsonb,
  sbom_ref TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_by TEXT NOT NULL,
  integrity_sha256 TEXT NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_run_environment_locks_run_id ON run_environment_locks (run_id);
CREATE INDEX IF NOT EXISTS idx_run_environment_locks_project_created_at ON run_environment_locks (project_id, created_at DESC);

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_run_environment_locks_project') THEN
    ALTER TABLE run_environment_locks ADD CONSTRAINT fk_run_environment_locks_project FOREIGN KEY (project_id) REFERENCES projects(project_id);
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_run_environment_locks_run') THEN
    ALTER TABLE run_environment_locks ADD CONSTRAINT fk_run_environment_locks_run FOREIGN KEY (run_id) REFERENCES runs(run_id);
  END IF;
END $$;

CREATE TABLE IF NOT EXISTS run_policy_snapshots (
  snapshot_id TEXT PRIMARY KEY,
  run_id TEXT NOT NULL,
  project_id TEXT NOT NULL,
  snapshot JSONB NOT NULL,
  snapshot_sha256 TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_by TEXT NOT NULL,
  integrity_sha256 TEXT NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_run_policy_snapshots_run_id ON run_policy_snapshots (run_id);
CREATE INDEX IF NOT EXISTS idx_run_policy_snapshots_project_created_at ON run_policy_snapshots (project_id, created_at DESC);

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_run_policy_snapshots_project') THEN
    ALTER TABLE run_policy_snapshots ADD CONSTRAINT fk_run_policy_snapshots_project FOREIGN KEY (project_id) REFERENCES projects(project_id);
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_run_policy_snapshots_run') THEN
    ALTER TABLE run_policy_snapshots ADD CONSTRAINT fk_run_policy_snapshots_run FOREIGN KEY (run_id) REFERENCES runs(run_id);
  END IF;
END $$;

CREATE OR REPLACE FUNCTION prevent_run_binding_update() RETURNS trigger AS $$
BEGIN
  RAISE EXCEPTION 'run bindings are immutable';
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION prevent_run_binding_delete() RETURNS trigger AS $$
BEGIN
  RAISE EXCEPTION 'run bindings are immutable';
END;
$$ LANGUAGE plpgsql;

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'trg_env_definitions_no_update') THEN
    CREATE TRIGGER trg_env_definitions_no_update
      BEFORE UPDATE ON environment_definitions
      FOR EACH ROW EXECUTE FUNCTION prevent_run_binding_update();
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'trg_env_definitions_no_delete') THEN
    CREATE TRIGGER trg_env_definitions_no_delete
      BEFORE DELETE ON environment_definitions
      FOR EACH ROW EXECUTE FUNCTION prevent_run_binding_delete();
  END IF;

  IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'trg_run_code_refs_no_update') THEN
    CREATE TRIGGER trg_run_code_refs_no_update
      BEFORE UPDATE ON run_code_refs
      FOR EACH ROW EXECUTE FUNCTION prevent_run_binding_update();
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'trg_run_code_refs_no_delete') THEN
    CREATE TRIGGER trg_run_code_refs_no_delete
      BEFORE DELETE ON run_code_refs
      FOR EACH ROW EXECUTE FUNCTION prevent_run_binding_delete();
  END IF;

  IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'trg_run_env_locks_no_update') THEN
    CREATE TRIGGER trg_run_env_locks_no_update
      BEFORE UPDATE ON run_environment_locks
      FOR EACH ROW EXECUTE FUNCTION prevent_run_binding_update();
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'trg_run_env_locks_no_delete') THEN
    CREATE TRIGGER trg_run_env_locks_no_delete
      BEFORE DELETE ON run_environment_locks
      FOR EACH ROW EXECUTE FUNCTION prevent_run_binding_delete();
  END IF;

  IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'trg_run_policy_snapshots_no_update') THEN
    CREATE TRIGGER trg_run_policy_snapshots_no_update
      BEFORE UPDATE ON run_policy_snapshots
      FOR EACH ROW EXECUTE FUNCTION prevent_run_binding_update();
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'trg_run_policy_snapshots_no_delete') THEN
    CREATE TRIGGER trg_run_policy_snapshots_no_delete
      BEFORE DELETE ON run_policy_snapshots
      FOR EACH ROW EXECUTE FUNCTION prevent_run_binding_delete();
  END IF;
END $$;
