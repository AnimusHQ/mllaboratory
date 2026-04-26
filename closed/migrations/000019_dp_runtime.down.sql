DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'trg_run_dispatches_no_delete') THEN
    DROP TRIGGER trg_run_dispatches_no_delete ON run_dispatches;
  END IF;
  IF EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'trg_run_dispatches_guard') THEN
    DROP TRIGGER trg_run_dispatches_guard ON run_dispatches;
  END IF;
END $$;

DROP FUNCTION IF EXISTS prevent_run_dispatch_update();

DROP TABLE IF EXISTS run_dispatches;

DROP INDEX IF EXISTS idx_run_dispatches_project_idempotency;
DROP INDEX IF EXISTS idx_run_dispatches_run_id;
DROP INDEX IF EXISTS idx_run_dispatches_project_requested_at;

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'trg_run_dp_events_no_delete') THEN
    DROP TRIGGER trg_run_dp_events_no_delete ON run_dp_events;
  END IF;
  IF EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'trg_run_dp_events_no_update') THEN
    DROP TRIGGER trg_run_dp_events_no_update ON run_dp_events;
  END IF;
END $$;

DROP TABLE IF EXISTS run_dp_events;

DROP INDEX IF EXISTS idx_run_dp_events_project_emitted_at;
DROP INDEX IF EXISTS idx_run_dp_events_run_type_emitted_at;
