ALTER TABLE dev_environments
  DROP COLUMN IF EXISTS commit_pin,
  DROP COLUMN IF EXISTS ref_value,
  DROP COLUMN IF EXISTS ref_type,
  DROP COLUMN IF EXISTS repo_url;
