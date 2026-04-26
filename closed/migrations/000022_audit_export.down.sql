DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'trg_audit_export_outbox_enqueue') THEN
    DROP TRIGGER trg_audit_export_outbox_enqueue ON audit_events;
  END IF;
END $$;

DROP FUNCTION IF EXISTS enqueue_audit_export_outbox();

DROP TABLE IF EXISTS audit_export_outbox;
DROP TABLE IF EXISTS audit_export_sinks;
