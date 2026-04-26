DROP TRIGGER IF EXISTS trg_artifacts_immutable ON artifacts;
DROP FUNCTION IF EXISTS prevent_artifact_scope_update();

ALTER TABLE artifacts DROP CONSTRAINT IF EXISTS fk_artifacts_project;

DROP INDEX IF EXISTS idx_artifacts_project_id;
DROP INDEX IF EXISTS idx_artifacts_project_created_at;

DROP TABLE IF EXISTS artifacts;
