CREATE TABLE IF NOT EXISTS webhook_deliveries (
    id uuid PRIMARY KEY,
    project_id uuid NOT NULL,
    subscription_id uuid NOT NULL REFERENCES webhook_subscriptions(id) ON DELETE CASCADE,
    event_id text NOT NULL,
    event_type text NOT NULL,
    payload_jsonb jsonb NOT NULL DEFAULT '{}'::jsonb,
    status text NOT NULL,
    next_attempt_at timestamptz NOT NULL,
    attempt_count int NOT NULL DEFAULT 0,
    last_error text NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS webhook_deliveries_unique_event
    ON webhook_deliveries (subscription_id, event_id);

CREATE INDEX IF NOT EXISTS webhook_deliveries_due_idx
    ON webhook_deliveries (status, next_attempt_at, created_at, id);

CREATE TABLE IF NOT EXISTS webhook_delivery_attempts (
    id bigserial PRIMARY KEY,
    delivery_id uuid NOT NULL REFERENCES webhook_deliveries(id) ON DELETE CASCADE,
    attempted_at timestamptz NOT NULL,
    status_code int NULL,
    outcome text NOT NULL,
    error text NULL,
    latency_ms int NULL,
    request_id text NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS webhook_delivery_attempts_delivery_idx
    ON webhook_delivery_attempts (delivery_id);

CREATE TABLE IF NOT EXISTS webhook_delivery_replays (
    id bigserial PRIMARY KEY,
    delivery_id uuid NOT NULL REFERENCES webhook_deliveries(id) ON DELETE CASCADE,
    replay_token text NOT NULL,
    requested_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (delivery_id, replay_token)
);
