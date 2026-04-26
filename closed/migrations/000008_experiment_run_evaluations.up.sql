CREATE TABLE IF NOT EXISTS experiment_run_evaluations (
  evaluation_id TEXT PRIMARY KEY,
  run_id TEXT NOT NULL REFERENCES experiment_runs(run_id),
  executor TEXT NOT NULL,
  image_ref TEXT NOT NULL,
  image_digest TEXT,
  resources JSONB NOT NULL DEFAULT '{}'::jsonb,
  k8s_namespace TEXT,
  k8s_job_name TEXT,
  docker_container_id TEXT,
  datapilot_url TEXT NOT NULL,
  preview_samples INTEGER NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_by TEXT NOT NULL,
  integrity_sha256 TEXT NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_experiment_run_evaluations_run_unique ON experiment_run_evaluations (run_id);
CREATE INDEX IF NOT EXISTS idx_experiment_run_evaluations_created_at ON experiment_run_evaluations (created_at DESC);
CREATE INDEX IF NOT EXISTS idx_experiment_run_evaluations_image_digest ON experiment_run_evaluations (image_digest);

CREATE TABLE IF NOT EXISTS experiment_run_evaluation_state_events (
  state_id TEXT PRIMARY KEY,
  evaluation_id TEXT NOT NULL REFERENCES experiment_run_evaluations(evaluation_id),
  status TEXT NOT NULL,
  observed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  details JSONB NOT NULL DEFAULT '{}'::jsonb,
  integrity_sha256 TEXT NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_experiment_run_evaluation_state_events_eval_status_unique ON experiment_run_evaluation_state_events (evaluation_id, status);
CREATE INDEX IF NOT EXISTS idx_experiment_run_evaluation_state_events_eval_observed ON experiment_run_evaluation_state_events (evaluation_id, observed_at DESC);
CREATE INDEX IF NOT EXISTS idx_experiment_run_evaluation_state_events_observed ON experiment_run_evaluation_state_events (observed_at DESC);

