DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'trg_environment_locks_no_delete') THEN
    DROP TRIGGER trg_environment_locks_no_delete ON environment_locks;
  END IF;
  IF EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'trg_environment_locks_no_update') THEN
    DROP TRIGGER trg_environment_locks_no_update ON environment_locks;
  END IF;
END $$;

DROP TABLE IF EXISTS environment_locks;

DROP INDEX IF EXISTS idx_environment_locks_project_created_at;
DROP INDEX IF EXISTS idx_environment_locks_project_definition;
DROP INDEX IF EXISTS idx_environment_locks_project_idempotency;

ALTER TABLE environment_definitions
  DROP COLUMN IF EXISTS base_images,
  DROP COLUMN IF EXISTS resource_defaults,
  DROP COLUMN IF EXISTS resource_limits,
  DROP COLUMN IF EXISTS allowed_accelerators,
  DROP COLUMN IF EXISTS network_class_ref,
  DROP COLUMN IF EXISTS secret_access_class_ref,
  DROP COLUMN IF EXISTS status,
  DROP COLUMN IF EXISTS supersedes_definition_id,
  DROP COLUMN IF EXISTS idempotency_key;

DROP INDEX IF EXISTS idx_env_definitions_project_idempotency;

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_env_definitions_supersedes') THEN
    ALTER TABLE environment_definitions DROP CONSTRAINT fk_env_definitions_supersedes;
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

DROP INDEX IF EXISTS idx_run_environment_locks_lock_id;

ALTER TABLE run_environment_locks
  ADD COLUMN IF NOT EXISTS env_template_id TEXT,
  ADD COLUMN IF NOT EXISTS image_digests JSONB NOT NULL DEFAULT '{}'::jsonb,
  ADD COLUMN IF NOT EXISTS dependency_checksums JSONB NOT NULL DEFAULT '{}'::jsonb,
  ADD COLUMN IF NOT EXISTS sbom_ref TEXT;

ALTER TABLE run_environment_locks
  ALTER COLUMN lock_id DROP NOT NULL;

ALTER TABLE run_environment_locks
  ADD CONSTRAINT run_environment_locks_pkey PRIMARY KEY (lock_id);

CREATE UNIQUE INDEX IF NOT EXISTS idx_run_environment_locks_run_id
  ON run_environment_locks (run_id);
