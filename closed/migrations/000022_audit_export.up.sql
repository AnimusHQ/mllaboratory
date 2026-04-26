CREATE TABLE IF NOT EXISTS audit_export_sinks (
  sink_id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  destination TEXT NOT NULL,
  format TEXT NOT NULL,
  config JSONB NOT NULL DEFAULT '{}'::jsonb,
  enabled BOOLEAN NOT NULL DEFAULT true,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS audit_export_outbox (
  outbox_id BIGSERIAL PRIMARY KEY,
  event_id BIGINT NOT NULL,
  sink_id TEXT NOT NULL,
  status TEXT NOT NULL,
  attempt_count INT NOT NULL DEFAULT 0,
  last_attempt_at TIMESTAMPTZ,
  next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  last_error TEXT,
  delivered_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (event_id, sink_id)
);

CREATE INDEX IF NOT EXISTS idx_audit_export_outbox_due
  ON audit_export_outbox (status, next_attempt_at);
CREATE INDEX IF NOT EXISTS idx_audit_export_outbox_event
  ON audit_export_outbox (event_id);

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_audit_export_outbox_event') THEN
    ALTER TABLE audit_export_outbox
      ADD CONSTRAINT fk_audit_export_outbox_event
      FOREIGN KEY (event_id) REFERENCES audit_events(event_id);
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_audit_export_outbox_sink') THEN
    ALTER TABLE audit_export_outbox
      ADD CONSTRAINT fk_audit_export_outbox_sink
      FOREIGN KEY (sink_id) REFERENCES audit_export_sinks(sink_id);
  END IF;
END $$;

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

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'trg_audit_export_outbox_enqueue') THEN
    CREATE TRIGGER trg_audit_export_outbox_enqueue
      AFTER INSERT ON audit_events
      FOR EACH ROW EXECUTE FUNCTION enqueue_audit_export_outbox();
  END IF;
END $$;
