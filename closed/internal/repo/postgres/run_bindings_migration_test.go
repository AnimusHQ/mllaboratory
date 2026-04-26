package postgres

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunBindingsMigrationImmutabilityTriggers(t *testing.T) {
	path := filepath.Join("..", "..", "..", "migrations", "000017_run_execution_bindings.up.sql")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	sql := string(raw)
	needles := []string{
		"CREATE OR REPLACE FUNCTION prevent_run_binding_update",
		"CREATE OR REPLACE FUNCTION prevent_run_binding_delete",
		"trg_run_code_refs_no_update",
		"trg_run_code_refs_no_delete",
		"trg_run_env_locks_no_update",
		"trg_run_env_locks_no_delete",
		"trg_run_policy_snapshots_no_update",
		"trg_run_policy_snapshots_no_delete",
	}
	for _, needle := range needles {
		if !strings.Contains(sql, needle) {
			t.Fatalf("expected immutability trigger element %q", needle)
		}
	}
}
