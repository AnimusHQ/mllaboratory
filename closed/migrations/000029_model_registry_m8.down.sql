DROP TABLE IF EXISTS model_exports;
DROP TABLE IF EXISTS model_version_transitions;
DROP TABLE IF EXISTS model_versions;

DROP INDEX IF EXISTS idx_models_project_idempotency_key;
ALTER TABLE models DROP COLUMN IF EXISTS idempotency_key;
