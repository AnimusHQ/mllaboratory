DROP TRIGGER IF EXISTS trg_step_executions_no_delete ON step_executions;
DROP TRIGGER IF EXISTS trg_step_executions_no_update ON step_executions;
DROP FUNCTION IF EXISTS prevent_step_execution_delete();
DROP FUNCTION IF EXISTS prevent_step_execution_update();

ALTER TABLE step_executions DROP CONSTRAINT IF EXISTS fk_step_executions_run;
ALTER TABLE step_executions DROP CONSTRAINT IF EXISTS fk_step_executions_project;

DROP INDEX IF EXISTS idx_step_executions_step;
DROP INDEX IF EXISTS idx_step_executions_run;
DROP INDEX IF EXISTS idx_step_executions_unique_attempt;

DROP TABLE IF EXISTS step_executions;
