ALTER TABLE experiment_run_executions
  DROP COLUMN IF EXISTS image_digest;

DROP TABLE IF EXISTS model_images;
