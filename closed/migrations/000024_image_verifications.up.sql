CREATE TABLE IF NOT EXISTS image_verifications (
  id BIGSERIAL PRIMARY KEY,
  project_id TEXT NOT NULL,
  image_digest_ref TEXT NOT NULL,
  policy_mode TEXT NOT NULL,
  provider TEXT NOT NULL,
  status TEXT NOT NULL,
  signed BOOLEAN NOT NULL DEFAULT false,
  verified BOOLEAN NOT NULL DEFAULT false,
  failure_reason TEXT,
  details_jsonb JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  verified_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_image_verifications_unique
  ON image_verifications (project_id, image_digest_ref, policy_mode, provider);

CREATE INDEX IF NOT EXISTS idx_image_verifications_lookup
  ON image_verifications (project_id, image_digest_ref, verified_at DESC);

CREATE TABLE IF NOT EXISTS registry_policies (
  project_id TEXT PRIMARY KEY,
  mode TEXT NOT NULL,
  provider TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
