CREATE TABLE IF NOT EXISTS webhook_subscriptions (
    id uuid PRIMARY KEY,
    project_id uuid NOT NULL,
    name text NOT NULL,
    target_url text NOT NULL,
    enabled boolean NOT NULL DEFAULT true,
    event_types text[] NOT NULL,
    secret_ref text NULL,
    headers_jsonb jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS webhook_subscriptions_project_idx
    ON webhook_subscriptions (project_id);

CREATE INDEX IF NOT EXISTS webhook_subscriptions_project_enabled_idx
    ON webhook_subscriptions (project_id, enabled);
