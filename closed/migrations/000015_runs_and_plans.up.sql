CREATE TABLE IF NOT EXISTS runs (
  run_id TEXT PRIMARY KEY,
  project_id TEXT NOT NULL,
  idempotency_key TEXT NOT NULL,
  status TEXT NOT NULL,
  pipeline_spec JSONB NOT NULL,
  run_spec JSONB NOT NULL,
  spec_hash TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_runs_project_idempotency_key ON runs (project_id, idempotency_key);
CREATE INDEX IF NOT EXISTS idx_runs_project_created_at ON runs (project_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_runs_project_id ON runs (project_id, run_id);

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_runs_project') THEN
    ALTER TABLE runs ADD CONSTRAINT fk_runs_project FOREIGN KEY (project_id) REFERENCES projects(project_id);
  END IF;
END $$;

CREATE OR REPLACE FUNCTION prevent_run_immutable_update() RETURNS trigger AS $$
BEGIN
  IF NEW.project_id IS DISTINCT FROM OLD.project_id THEN
    RAISE EXCEPTION 'run project_id is immutable';
  END IF;
  IF NEW.idempotency_key IS DISTINCT FROM OLD.idempotency_key THEN
    RAISE EXCEPTION 'run idempotency_key is immutable';
  END IF;
  IF NEW.pipeline_spec IS DISTINCT FROM OLD.pipeline_spec THEN
    RAISE EXCEPTION 'run pipeline_spec is immutable';
  END IF;
  IF NEW.run_spec IS DISTINCT FROM OLD.run_spec THEN
    RAISE EXCEPTION 'run run_spec is immutable';
  END IF;
  IF NEW.spec_hash IS DISTINCT FROM OLD.spec_hash THEN
    RAISE EXCEPTION 'run spec_hash is immutable';
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION prevent_run_delete() RETURNS trigger AS $$
BEGIN
  RAISE EXCEPTION 'runs are immutable';
END;
$$ LANGUAGE plpgsql;

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'trg_runs_immutable') THEN
    CREATE TRIGGER trg_runs_immutable
      BEFORE UPDATE ON runs
      FOR EACH ROW EXECUTE FUNCTION prevent_run_immutable_update();
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'trg_runs_no_delete') THEN
    CREATE TRIGGER trg_runs_no_delete
      BEFORE DELETE ON runs
      FOR EACH ROW EXECUTE FUNCTION prevent_run_delete();
  END IF;
END $$;

CREATE TABLE IF NOT EXISTS execution_plans (
  plan_id TEXT PRIMARY KEY,
  run_id TEXT NOT NULL,
  project_id TEXT NOT NULL,
  plan JSONB NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_execution_plans_run_id ON execution_plans (run_id);
CREATE INDEX IF NOT EXISTS idx_execution_plans_project_created_at ON execution_plans (project_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_execution_plans_project_id ON execution_plans (project_id, run_id);

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_execution_plans_project') THEN
    ALTER TABLE execution_plans ADD CONSTRAINT fk_execution_plans_project FOREIGN KEY (project_id) REFERENCES projects(project_id);
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_execution_plans_run') THEN
    ALTER TABLE execution_plans ADD CONSTRAINT fk_execution_plans_run FOREIGN KEY (run_id) REFERENCES runs(run_id);
  END IF;
END $$;

CREATE OR REPLACE FUNCTION prevent_execution_plan_update() RETURNS trigger AS $$
BEGIN
  IF NEW.project_id IS DISTINCT FROM OLD.project_id THEN
    RAISE EXCEPTION 'execution plan project_id is immutable';
  END IF;
  IF NEW.run_id IS DISTINCT FROM OLD.run_id THEN
    RAISE EXCEPTION 'execution plan run_id is immutable';
  END IF;
  IF NEW.plan IS DISTINCT FROM OLD.plan THEN
    RAISE EXCEPTION 'execution plan is immutable';
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION prevent_execution_plan_delete() RETURNS trigger AS $$
BEGIN
  RAISE EXCEPTION 'execution plans are immutable';
END;
$$ LANGUAGE plpgsql;

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'trg_execution_plans_immutable') THEN
    CREATE TRIGGER trg_execution_plans_immutable
      BEFORE UPDATE ON execution_plans
      FOR EACH ROW EXECUTE FUNCTION prevent_execution_plan_update();
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'trg_execution_plans_no_delete') THEN
    CREATE TRIGGER trg_execution_plans_no_delete
      BEFORE DELETE ON execution_plans
      FOR EACH ROW EXECUTE FUNCTION prevent_execution_plan_delete();
  END IF;
END $$;
