CREATE TABLE IF NOT EXISTS artifacts (
  artifact_id TEXT PRIMARY KEY,
  project_id TEXT NOT NULL,
  kind TEXT NOT NULL,
  object_key TEXT NOT NULL,
  content_type TEXT,
  size_bytes BIGINT,
  sha256 TEXT NOT NULL,
  retention_until TIMESTAMPTZ,
  legal_hold BOOLEAN NOT NULL DEFAULT FALSE,
  metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_by TEXT NOT NULL,
  integrity_sha256 TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_artifacts_project_created_at ON artifacts (project_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_artifacts_project_id ON artifacts (project_id, artifact_id);

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_artifacts_project') THEN
    ALTER TABLE artifacts ADD CONSTRAINT fk_artifacts_project FOREIGN KEY (project_id) REFERENCES projects(project_id);
  END IF;
END $$;

CREATE OR REPLACE FUNCTION prevent_artifact_scope_update() RETURNS trigger AS $$
BEGIN
  IF NEW.project_id IS DISTINCT FROM OLD.project_id THEN
    RAISE EXCEPTION 'artifact project_id is immutable';
  END IF;
  IF NEW.object_key IS DISTINCT FROM OLD.object_key THEN
    RAISE EXCEPTION 'artifact object_key is immutable';
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'trg_artifacts_immutable') THEN
    CREATE TRIGGER trg_artifacts_immutable
      BEFORE UPDATE ON artifacts
      FOR EACH ROW EXECUTE FUNCTION prevent_artifact_scope_update();
  END IF;
END $$;
