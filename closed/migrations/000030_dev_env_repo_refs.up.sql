ALTER TABLE dev_environments
  ADD COLUMN IF NOT EXISTS repo_url TEXT,
  ADD COLUMN IF NOT EXISTS ref_type TEXT,
  ADD COLUMN IF NOT EXISTS ref_value TEXT,
  ADD COLUMN IF NOT EXISTS commit_pin TEXT;

UPDATE dev_environments SET repo_url = '' WHERE repo_url IS NULL;
UPDATE dev_environments SET ref_type = '' WHERE ref_type IS NULL;
UPDATE dev_environments SET ref_value = '' WHERE ref_value IS NULL;

ALTER TABLE dev_environments
  ALTER COLUMN repo_url SET DEFAULT '',
  ALTER COLUMN ref_type SET DEFAULT '',
  ALTER COLUMN ref_value SET DEFAULT '';

ALTER TABLE dev_environments
  ALTER COLUMN repo_url SET NOT NULL,
  ALTER COLUMN ref_type SET NOT NULL,
  ALTER COLUMN ref_value SET NOT NULL;
