DROP TABLE IF EXISTS quality_evaluations;
ALTER TABLE dataset_versions DROP COLUMN IF EXISTS quality_rule_id;
DROP TABLE IF EXISTS quality_rules;

