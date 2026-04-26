ALTER TABLE environment_definitions
  ADD COLUMN IF NOT EXISTS base_images JSONB NOT NULL DEFAULT '[]'::jsonb,
  ADD COLUMN IF NOT EXISTS resource_defaults JSONB NOT NULL DEFAULT '{}'::jsonb,
  ADD COLUMN IF NOT EXISTS resource_limits JSONB NOT NULL DEFAULT '{}'::jsonb,
  ADD COLUMN IF NOT EXISTS allowed_accelerators JSONB NOT NULL DEFAULT '[]'::jsonb,
  ADD COLUMN IF NOT EXISTS network_class_ref TEXT,
  ADD COLUMN IF NOT EXISTS secret_access_class_ref TEXT,
  ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'active',
  ADD COLUMN IF NOT EXISTS supersedes_definition_id TEXT,
  ADD COLUMN IF NOT EXISTS idempotency_key TEXT;

UPDATE environment_definitions
SET base_images = jsonb_build_array(jsonb_build_object('name', 'base', 'ref', base_image_ref))
WHERE base_images = '[]'::jsonb
  AND COALESCE(base_image_ref, '') <> '';

CREATE UNIQUE INDEX IF NOT EXISTS idx_env_definitions_project_idempotency
  ON environment_definitions (project_id, idempotency_key)
  WHERE idempotency_key IS NOT NULL;

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_env_definitions_supersedes') THEN
    ALTER TABLE environment_definitions
      ADD CONSTRAINT fk_env_definitions_supersedes
      FOREIGN KEY (supersedes_definition_id)
      REFERENCES environment_definitions(environment_definition_id);
  END IF;
END $$;

CREATE TABLE IF NOT EXISTS environment_locks (
  lock_id TEXT PRIMARY KEY,
  project_id TEXT NOT NULL,
  environment_definition_id TEXT NOT NULL,
  environment_definition_version INTEGER NOT NULL,
  images JSONB NOT NULL DEFAULT '[]'::jsonb,
  resource_defaults JSONB NOT NULL DEFAULT '{}'::jsonb,
  resource_limits JSONB NOT NULL DEFAULT '{}'::jsonb,
  allowed_accelerators JSONB NOT NULL DEFAULT '[]'::jsonb,
  network_class_ref TEXT,
  secret_access_class_ref TEXT,
  dependency_checksums JSONB NOT NULL DEFAULT '{}'::jsonb,
  sbom_ref TEXT,
  env_hash TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_by TEXT NOT NULL,
  integrity_sha256 TEXT NOT NULL,
  idempotency_key TEXT
);

CREATE INDEX IF NOT EXISTS idx_environment_locks_project_created_at
  ON environment_locks (project_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_environment_locks_project_definition
  ON environment_locks (project_id, environment_definition_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_environment_locks_project_idempotency
  ON environment_locks (project_id, idempotency_key)
  WHERE idempotency_key IS NOT NULL;

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_environment_locks_project') THEN
    ALTER TABLE environment_locks
      ADD CONSTRAINT fk_environment_locks_project
      FOREIGN KEY (project_id) REFERENCES projects(project_id);
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_environment_locks_definition') THEN
    ALTER TABLE environment_locks
      ADD CONSTRAINT fk_environment_locks_definition
      FOREIGN KEY (environment_definition_id) REFERENCES environment_definitions(environment_definition_id);
  END IF;
END $$;

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'trg_environment_locks_no_update') THEN
    CREATE TRIGGER trg_environment_locks_no_update
      BEFORE UPDATE ON environment_locks
      FOR EACH ROW EXECUTE FUNCTION prevent_run_binding_update();
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'trg_environment_locks_no_delete') THEN
    CREATE TRIGGER trg_environment_locks_no_delete
      BEFORE DELETE ON environment_locks
      FOR EACH ROW EXECUTE FUNCTION prevent_run_binding_delete();
  END IF;
END $$;

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'run_environment_locks_pkey') THEN
    ALTER TABLE run_environment_locks DROP CONSTRAINT run_environment_locks_pkey;
  END IF;
  IF EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_run_environment_locks_lock') THEN
    ALTER TABLE run_environment_locks DROP CONSTRAINT fk_run_environment_locks_lock;
  END IF;
END $$;

ALTER TABLE run_environment_locks
  DROP COLUMN IF EXISTS env_template_id,
  DROP COLUMN IF EXISTS image_digests,
  DROP COLUMN IF EXISTS dependency_checksums,
  DROP COLUMN IF EXISTS sbom_ref;

ALTER TABLE run_environment_locks
  ADD COLUMN IF NOT EXISTS env_hash TEXT,
  ALTER COLUMN lock_id SET NOT NULL;

ALTER TABLE run_environment_locks
  ADD CONSTRAINT run_environment_locks_pkey PRIMARY KEY (run_id);

CREATE INDEX IF NOT EXISTS idx_run_environment_locks_lock_id
  ON run_environment_locks (lock_id);

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_run_environment_locks_lock') THEN
    ALTER TABLE run_environment_locks
      ADD CONSTRAINT fk_run_environment_locks_lock
      FOREIGN KEY (lock_id) REFERENCES environment_locks(lock_id);
  END IF;
END $$;
