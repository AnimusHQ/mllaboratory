CREATE TABLE IF NOT EXISTS experiment_run_executions (
  execution_id TEXT PRIMARY KEY,
  run_id TEXT NOT NULL REFERENCES experiment_runs(run_id),
  executor TEXT NOT NULL,
  image_ref TEXT NOT NULL,
  resources JSONB NOT NULL DEFAULT '{}'::jsonb,
  k8s_namespace TEXT,
  k8s_job_name TEXT,
  docker_container_id TEXT,
  datapilot_url TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_by TEXT NOT NULL,
  integrity_sha256 TEXT NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_experiment_run_executions_run_unique ON experiment_run_executions (run_id);
CREATE INDEX IF NOT EXISTS idx_experiment_run_executions_created_at ON experiment_run_executions (created_at DESC);

CREATE TABLE IF NOT EXISTS experiment_run_state_events (
  state_id TEXT PRIMARY KEY,
  run_id TEXT NOT NULL REFERENCES experiment_runs(run_id),
  status TEXT NOT NULL,
  observed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  details JSONB NOT NULL DEFAULT '{}'::jsonb,
  integrity_sha256 TEXT NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_experiment_run_state_events_run_status_unique ON experiment_run_state_events (run_id, status);
CREATE INDEX IF NOT EXISTS idx_experiment_run_state_events_run_observed ON experiment_run_state_events (run_id, observed_at DESC);
CREATE INDEX IF NOT EXISTS idx_experiment_run_state_events_observed ON experiment_run_state_events (observed_at DESC);

CREATE TABLE IF NOT EXISTS experiment_run_metric_samples (
  sample_id TEXT PRIMARY KEY,
  run_id TEXT NOT NULL REFERENCES experiment_runs(run_id),
  recorded_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  recorded_by TEXT NOT NULL,
  step BIGINT NOT NULL,
  name TEXT NOT NULL,
  value DOUBLE PRECISION NOT NULL,
  metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
  integrity_sha256 TEXT NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_experiment_run_metric_samples_run_name_step_unique ON experiment_run_metric_samples (run_id, name, step);
CREATE INDEX IF NOT EXISTS idx_experiment_run_metric_samples_run_name_step_desc ON experiment_run_metric_samples (run_id, name, step DESC);
CREATE INDEX IF NOT EXISTS idx_experiment_run_metric_samples_recorded_at ON experiment_run_metric_samples (recorded_at DESC);

CREATE TABLE IF NOT EXISTS experiment_run_events (
  event_id BIGSERIAL PRIMARY KEY,
  run_id TEXT NOT NULL REFERENCES experiment_runs(run_id),
  occurred_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  actor TEXT NOT NULL,
  level TEXT NOT NULL,
  message TEXT NOT NULL,
  metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
  integrity_sha256 TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_experiment_run_events_run_occurred ON experiment_run_events (run_id, occurred_at DESC);
CREATE INDEX IF NOT EXISTS idx_experiment_run_events_occurred ON experiment_run_events (occurred_at DESC);
