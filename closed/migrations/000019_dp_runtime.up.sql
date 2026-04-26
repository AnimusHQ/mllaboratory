CREATE TABLE IF NOT EXISTS run_dp_events (
  event_id TEXT PRIMARY KEY,
  run_id TEXT NOT NULL,
  project_id TEXT NOT NULL,
  event_type TEXT NOT NULL,
  payload JSONB NOT NULL DEFAULT '{}'::jsonb,
  emitted_at TIMESTAMPTZ NOT NULL,
  received_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  integrity_sha256 TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_run_dp_events_project_emitted_at
  ON run_dp_events (project_id, emitted_at DESC);
CREATE INDEX IF NOT EXISTS idx_run_dp_events_run_type_emitted_at
  ON run_dp_events (run_id, event_type, emitted_at DESC);

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_run_dp_events_project') THEN
    ALTER TABLE run_dp_events
      ADD CONSTRAINT fk_run_dp_events_project
      FOREIGN KEY (project_id) REFERENCES projects(project_id);
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_run_dp_events_run') THEN
    ALTER TABLE run_dp_events
      ADD CONSTRAINT fk_run_dp_events_run
      FOREIGN KEY (run_id) REFERENCES runs(run_id);
  END IF;
END $$;

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'trg_run_dp_events_no_update') THEN
    CREATE TRIGGER trg_run_dp_events_no_update
      BEFORE UPDATE ON run_dp_events
      FOR EACH ROW EXECUTE FUNCTION prevent_run_binding_update();
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'trg_run_dp_events_no_delete') THEN
    CREATE TRIGGER trg_run_dp_events_no_delete
      BEFORE DELETE ON run_dp_events
      FOR EACH ROW EXECUTE FUNCTION prevent_run_binding_delete();
  END IF;
END $$;

CREATE TABLE IF NOT EXISTS run_dispatches (
  dispatch_id TEXT PRIMARY KEY,
  run_id TEXT NOT NULL,
  project_id TEXT NOT NULL,
  idempotency_key TEXT NOT NULL,
  dp_base_url TEXT NOT NULL,
  status TEXT NOT NULL,
  last_error TEXT,
  spec_hash TEXT NOT NULL,
  requested_at TIMESTAMPTZ NOT NULL,
  requested_by TEXT NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  integrity_sha256 TEXT NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_run_dispatches_project_idempotency
  ON run_dispatches (project_id, idempotency_key);
CREATE UNIQUE INDEX IF NOT EXISTS idx_run_dispatches_run_id
  ON run_dispatches (run_id);
CREATE INDEX IF NOT EXISTS idx_run_dispatches_project_requested_at
  ON run_dispatches (project_id, requested_at DESC);

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_run_dispatches_project') THEN
    ALTER TABLE run_dispatches
      ADD CONSTRAINT fk_run_dispatches_project
      FOREIGN KEY (project_id) REFERENCES projects(project_id);
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_run_dispatches_run') THEN
    ALTER TABLE run_dispatches
      ADD CONSTRAINT fk_run_dispatches_run
      FOREIGN KEY (run_id) REFERENCES runs(run_id);
  END IF;
END $$;

CREATE OR REPLACE FUNCTION prevent_run_dispatch_update() RETURNS trigger AS $$
BEGIN
  IF NEW.run_id IS DISTINCT FROM OLD.run_id THEN
    RAISE EXCEPTION 'run dispatch run_id is immutable';
  END IF;
  IF NEW.project_id IS DISTINCT FROM OLD.project_id THEN
    RAISE EXCEPTION 'run dispatch project_id is immutable';
  END IF;
  IF NEW.idempotency_key IS DISTINCT FROM OLD.idempotency_key THEN
    RAISE EXCEPTION 'run dispatch idempotency_key is immutable';
  END IF;
  IF NEW.dp_base_url IS DISTINCT FROM OLD.dp_base_url THEN
    RAISE EXCEPTION 'run dispatch dp_base_url is immutable';
  END IF;
  IF NEW.spec_hash IS DISTINCT FROM OLD.spec_hash THEN
    RAISE EXCEPTION 'run dispatch spec_hash is immutable';
  END IF;
  IF NEW.requested_at IS DISTINCT FROM OLD.requested_at THEN
    RAISE EXCEPTION 'run dispatch requested_at is immutable';
  END IF;
  IF NEW.requested_by IS DISTINCT FROM OLD.requested_by THEN
    RAISE EXCEPTION 'run dispatch requested_by is immutable';
  END IF;
  IF NEW.integrity_sha256 IS DISTINCT FROM OLD.integrity_sha256 THEN
    RAISE EXCEPTION 'run dispatch integrity_sha256 is immutable';
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'trg_run_dispatches_guard') THEN
    CREATE TRIGGER trg_run_dispatches_guard
      BEFORE UPDATE ON run_dispatches
      FOR EACH ROW EXECUTE FUNCTION prevent_run_dispatch_update();
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'trg_run_dispatches_no_delete') THEN
    CREATE TRIGGER trg_run_dispatches_no_delete
      BEFORE DELETE ON run_dispatches
      FOR EACH ROW EXECUTE FUNCTION prevent_run_binding_delete();
  END IF;
END $$;
