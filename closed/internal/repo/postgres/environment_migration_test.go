package postgres

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnvironmentLocksMigrationHasImmutabilityTriggers(t *testing.T) {
	path := filepath.Join("..", "..", "..", "migrations", "000018_environment_definitions_locks.up.sql")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	sql := string(raw)
	needles := []string{
		"CREATE TABLE IF NOT EXISTS environment_locks",
		"trg_environment_locks_no_update",
		"trg_environment_locks_no_delete",
	}
	for _, needle := range needles {
		if !strings.Contains(sql, needle) {
			t.Fatalf("expected migration to include %q", needle)
		}
	}
}
