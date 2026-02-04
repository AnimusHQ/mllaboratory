CREATE OR REPLACE FUNCTION enqueue_audit_export_outbox() RETURNS trigger AS $$
BEGIN
  IF NEW.action LIKE 'audit.export.%' THEN
    RETURN NEW;
  END IF;
  INSERT INTO audit_export_outbox (event_id, sink_id, status, next_attempt_at, created_at, updated_at)
    SELECT NEW.event_id, sink_id, 'pending', now(), now(), now()
    FROM audit_export_sinks
    WHERE enabled = true
    ON CONFLICT (event_id, sink_id) DO NOTHING;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TABLE IF EXISTS audit_export_replays;
DROP TABLE IF EXISTS audit_export_attempts;
DROP TABLE IF EXISTS audit_export_deliveries;
