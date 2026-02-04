CREATE TABLE IF NOT EXISTS audit_export_deliveries (
  delivery_id BIGSERIAL PRIMARY KEY,
  sink_id TEXT NOT NULL,
  event_id BIGINT NOT NULL,
  status TEXT NOT NULL,
  attempt_count INT NOT NULL DEFAULT 0,
  next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  last_error TEXT,
  dlq_reason TEXT,
  delivered_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (sink_id, event_id)
);

CREATE INDEX IF NOT EXISTS idx_audit_export_deliveries_due
  ON audit_export_deliveries (status, next_attempt_at);
CREATE INDEX IF NOT EXISTS idx_audit_export_deliveries_event
  ON audit_export_deliveries (event_id);
CREATE INDEX IF NOT EXISTS idx_audit_export_deliveries_sink
  ON audit_export_deliveries (sink_id);

CREATE TABLE IF NOT EXISTS audit_export_attempts (
  attempt_id BIGSERIAL PRIMARY KEY,
  delivery_id BIGINT NOT NULL,
  attempted_at TIMESTAMPTZ NOT NULL,
  outcome TEXT NOT NULL,
  status_code INT,
  error TEXT,
  latency_ms INT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_audit_export_attempts_delivery
  ON audit_export_attempts (delivery_id, attempted_at DESC, attempt_id DESC);

CREATE TABLE IF NOT EXISTS audit_export_replays (
  delivery_id BIGINT NOT NULL,
  replay_token TEXT NOT NULL,
  requested_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (delivery_id, replay_token)
);

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_audit_export_deliveries_event') THEN
    ALTER TABLE audit_export_deliveries
      ADD CONSTRAINT fk_audit_export_deliveries_event
      FOREIGN KEY (event_id) REFERENCES audit_events(event_id);
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_audit_export_deliveries_sink') THEN
    ALTER TABLE audit_export_deliveries
      ADD CONSTRAINT fk_audit_export_deliveries_sink
      FOREIGN KEY (sink_id) REFERENCES audit_export_sinks(sink_id);
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_audit_export_attempts_delivery') THEN
    ALTER TABLE audit_export_attempts
      ADD CONSTRAINT fk_audit_export_attempts_delivery
      FOREIGN KEY (delivery_id) REFERENCES audit_export_deliveries(delivery_id);
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
  INSERT INTO audit_export_deliveries (event_id, sink_id, status, next_attempt_at, created_at, updated_at)
    SELECT NEW.event_id, sink_id, 'pending', now(), now(), now()
    FROM audit_export_sinks
    WHERE enabled = true
    ON CONFLICT (sink_id, event_id) DO NOTHING;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;
