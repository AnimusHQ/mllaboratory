CREATE TABLE IF NOT EXISTS step_executions (
  step_execution_id TEXT PRIMARY KEY,
  project_id TEXT NOT NULL,
  run_id TEXT NOT NULL,
  step_name TEXT NOT NULL,
  attempt INT NOT NULL CHECK (attempt >= 1),
  status TEXT NOT NULL,
  started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  finished_at TIMESTAMPTZ,
  error_code TEXT,
  error_message TEXT,
  result JSONB,
  spec_hash TEXT NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_step_executions_unique_attempt ON step_executions (project_id, run_id, step_name, attempt);
CREATE INDEX IF NOT EXISTS idx_step_executions_run ON step_executions (project_id, run_id);
CREATE INDEX IF NOT EXISTS idx_step_executions_step ON step_executions (project_id, run_id, step_name);

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_step_executions_project') THEN
    ALTER TABLE step_executions ADD CONSTRAINT fk_step_executions_project FOREIGN KEY (project_id) REFERENCES projects(project_id);
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_step_executions_run') THEN
    ALTER TABLE step_executions ADD CONSTRAINT fk_step_executions_run FOREIGN KEY (run_id) REFERENCES runs(run_id);
  END IF;
END $$;

CREATE OR REPLACE FUNCTION prevent_step_execution_update() RETURNS trigger AS $$
BEGIN
  RAISE EXCEPTION 'step executions are append-only';
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION prevent_step_execution_delete() RETURNS trigger AS $$
BEGIN
  RAISE EXCEPTION 'step executions are append-only';
END;
$$ LANGUAGE plpgsql;

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'trg_step_executions_no_update') THEN
    CREATE TRIGGER trg_step_executions_no_update
      BEFORE UPDATE ON step_executions
      FOR EACH ROW EXECUTE FUNCTION prevent_step_execution_update();
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'trg_step_executions_no_delete') THEN
    CREATE TRIGGER trg_step_executions_no_delete
      BEFORE DELETE ON step_executions
      FOR EACH ROW EXECUTE FUNCTION prevent_step_execution_delete();
  END IF;
END $$;
