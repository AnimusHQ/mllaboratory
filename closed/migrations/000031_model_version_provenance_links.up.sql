CREATE TABLE IF NOT EXISTS model_version_artifacts (
  project_id TEXT NOT NULL,
  model_version_id TEXT NOT NULL,
  artifact_id TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (project_id, model_version_id, artifact_id)
);

CREATE INDEX IF NOT EXISTS idx_model_version_artifacts_artifact_id
  ON model_version_artifacts (artifact_id);
CREATE INDEX IF NOT EXISTS idx_model_version_artifacts_version_id
  ON model_version_artifacts (model_version_id);

CREATE TABLE IF NOT EXISTS model_version_datasets (
  project_id TEXT NOT NULL,
  model_version_id TEXT NOT NULL,
  dataset_version_id TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (project_id, model_version_id, dataset_version_id)
);

CREATE INDEX IF NOT EXISTS idx_model_version_datasets_dataset_id
  ON model_version_datasets (dataset_version_id);
CREATE INDEX IF NOT EXISTS idx_model_version_datasets_version_id
  ON model_version_datasets (model_version_id);

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_model_version_artifacts_project') THEN
    ALTER TABLE model_version_artifacts
      ADD CONSTRAINT fk_model_version_artifacts_project
      FOREIGN KEY (project_id) REFERENCES projects(project_id);
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_model_version_artifacts_version') THEN
    ALTER TABLE model_version_artifacts
      ADD CONSTRAINT fk_model_version_artifacts_version
      FOREIGN KEY (model_version_id) REFERENCES model_versions(model_version_id);
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_model_version_datasets_project') THEN
    ALTER TABLE model_version_datasets
      ADD CONSTRAINT fk_model_version_datasets_project
      FOREIGN KEY (project_id) REFERENCES projects(project_id);
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_model_version_datasets_version') THEN
    ALTER TABLE model_version_datasets
      ADD CONSTRAINT fk_model_version_datasets_version
      FOREIGN KEY (model_version_id) REFERENCES model_versions(model_version_id);
  END IF;
END $$;
