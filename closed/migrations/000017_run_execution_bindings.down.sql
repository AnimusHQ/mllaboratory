DROP TRIGGER IF EXISTS trg_run_policy_snapshots_no_delete ON run_policy_snapshots;
DROP TRIGGER IF EXISTS trg_run_policy_snapshots_no_update ON run_policy_snapshots;
DROP TRIGGER IF EXISTS trg_run_env_locks_no_delete ON run_environment_locks;
DROP TRIGGER IF EXISTS trg_run_env_locks_no_update ON run_environment_locks;
DROP TRIGGER IF EXISTS trg_run_code_refs_no_delete ON run_code_refs;
DROP TRIGGER IF EXISTS trg_run_code_refs_no_update ON run_code_refs;
DROP TRIGGER IF EXISTS trg_env_definitions_no_delete ON environment_definitions;
DROP TRIGGER IF EXISTS trg_env_definitions_no_update ON environment_definitions;

DROP FUNCTION IF EXISTS prevent_run_binding_delete();
DROP FUNCTION IF EXISTS prevent_run_binding_update();

DROP TABLE IF EXISTS run_policy_snapshots;
DROP TABLE IF EXISTS run_environment_locks;
DROP TABLE IF EXISTS run_code_refs;
DROP TABLE IF EXISTS environment_definitions;
