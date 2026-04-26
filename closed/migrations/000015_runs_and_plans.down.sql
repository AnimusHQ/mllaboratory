DROP TRIGGER IF EXISTS trg_execution_plans_no_delete ON execution_plans;
DROP TRIGGER IF EXISTS trg_execution_plans_immutable ON execution_plans;
DROP FUNCTION IF EXISTS prevent_execution_plan_delete();
DROP FUNCTION IF EXISTS prevent_execution_plan_update();

ALTER TABLE execution_plans DROP CONSTRAINT IF EXISTS fk_execution_plans_run;
ALTER TABLE execution_plans DROP CONSTRAINT IF EXISTS fk_execution_plans_project;

DROP INDEX IF EXISTS idx_execution_plans_project_id;
DROP INDEX IF EXISTS idx_execution_plans_project_created_at;
DROP INDEX IF EXISTS idx_execution_plans_run_id;

DROP TABLE IF EXISTS execution_plans;

DROP TRIGGER IF EXISTS trg_runs_no_delete ON runs;
DROP TRIGGER IF EXISTS trg_runs_immutable ON runs;
DROP FUNCTION IF EXISTS prevent_run_delete();
DROP FUNCTION IF EXISTS prevent_run_immutable_update();

ALTER TABLE runs DROP CONSTRAINT IF EXISTS fk_runs_project;

DROP INDEX IF EXISTS idx_runs_project_id;
DROP INDEX IF EXISTS idx_runs_project_created_at;
DROP INDEX IF EXISTS idx_runs_project_idempotency_key;

DROP TABLE IF EXISTS runs;
