DROP INDEX IF EXISTS idx_models_project_created_at;
DROP INDEX IF EXISTS idx_models_created_at;
DROP INDEX IF EXISTS idx_models_project_status;
DROP INDEX IF EXISTS idx_models_project_name_version_unique;
DROP TABLE IF EXISTS models;

ALTER TABLE experiment_run_artifacts DROP COLUMN IF EXISTS retention_until;
ALTER TABLE experiment_run_artifacts DROP COLUMN IF EXISTS retention_policy;

DROP INDEX IF EXISTS idx_experiment_run_artifacts_project_created_at;
DROP INDEX IF EXISTS idx_experiment_runs_project_started_at;
DROP INDEX IF EXISTS idx_experiments_project_created_at;
DROP INDEX IF EXISTS idx_dataset_versions_project_created_at;
DROP INDEX IF EXISTS idx_dataset_versions_project_dataset;
DROP INDEX IF EXISTS idx_datasets_project_created_at;

DROP INDEX IF EXISTS idx_experiment_run_artifacts_project_id;
DROP INDEX IF EXISTS idx_experiment_runs_project_id;
DROP INDEX IF EXISTS idx_experiments_project_id;
DROP INDEX IF EXISTS idx_dataset_versions_project_id;
DROP INDEX IF EXISTS idx_datasets_project_id;

ALTER TABLE experiment_run_artifacts DROP CONSTRAINT IF EXISTS fk_experiment_run_artifacts_project;
ALTER TABLE experiment_runs DROP CONSTRAINT IF EXISTS fk_experiment_runs_project;
ALTER TABLE experiments DROP CONSTRAINT IF EXISTS fk_experiments_project;
ALTER TABLE dataset_versions DROP CONSTRAINT IF EXISTS fk_dataset_versions_project;
ALTER TABLE datasets DROP CONSTRAINT IF EXISTS fk_datasets_project;
-- models table is dropped above; constraints removed implicitly

DROP TRIGGER IF EXISTS trg_audit_events_immutable ON audit_events;
DROP TRIGGER IF EXISTS trg_dataset_versions_immutable ON dataset_versions;
DROP FUNCTION IF EXISTS prevent_update_delete();

ALTER TABLE experiment_run_artifacts DROP COLUMN IF EXISTS project_id;
ALTER TABLE experiment_runs DROP COLUMN IF EXISTS project_id;
ALTER TABLE experiments DROP COLUMN IF EXISTS project_id;
ALTER TABLE dataset_versions DROP COLUMN IF EXISTS project_id;
ALTER TABLE datasets DROP COLUMN IF EXISTS project_id;

DROP INDEX IF EXISTS idx_projects_created_at;
DROP INDEX IF EXISTS idx_projects_name_unique;
DROP TABLE IF EXISTS projects;
