CREATE TABLE IF NOT EXISTS auth_sessions (
  session_id TEXT PRIMARY KEY,
  subject TEXT NOT NULL,
  email TEXT,
  roles JSONB NOT NULL DEFAULT '[]'::jsonb,
  issuer TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  expires_at TIMESTAMPTZ NOT NULL,
  last_seen_at TIMESTAMPTZ,
  revoked_at TIMESTAMPTZ,
  revoked_by TEXT,
  revoke_reason TEXT,
  id_token_sha256 TEXT,
  user_agent TEXT,
  ip TEXT,
  metadata JSONB NOT NULL DEFAULT '{}'::jsonb
);

CREATE INDEX IF NOT EXISTS idx_auth_sessions_subject_active
  ON auth_sessions (subject, revoked_at, expires_at);
CREATE INDEX IF NOT EXISTS idx_auth_sessions_expires_at
  ON auth_sessions (expires_at);

CREATE TABLE IF NOT EXISTS project_role_bindings (
  binding_id TEXT PRIMARY KEY,
  project_id TEXT NOT NULL,
  subject_type TEXT NOT NULL,
  subject TEXT NOT NULL,
  role TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_by TEXT NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_by TEXT NOT NULL,
  integrity_sha256 TEXT NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_project_role_bindings_unique
  ON project_role_bindings (project_id, subject_type, subject);
CREATE INDEX IF NOT EXISTS idx_project_role_bindings_project_role
  ON project_role_bindings (project_id, role);

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_project_role_bindings_project') THEN
    ALTER TABLE project_role_bindings
      ADD CONSTRAINT fk_project_role_bindings_project
      FOREIGN KEY (project_id) REFERENCES projects(project_id);
  END IF;
END $$;
