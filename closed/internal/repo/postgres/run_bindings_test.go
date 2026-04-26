package postgres

import (
	"strings"
	"testing"
)

func TestRunBindingsInsertQueriesAreIdempotent(t *testing.T) {
	queries := []string{
		insertRunCodeRefQuery,
		insertRunEnvLockQuery,
		insertRunPolicySnapshotQuery,
	}
	for _, query := range queries {
		if !strings.Contains(query, "ON CONFLICT (run_id) DO NOTHING") {
			t.Fatalf("expected idempotent insert, got query: %s", query)
		}
	}
}

func TestRunBindingsQueriesAreProjectScoped(t *testing.T) {
	queries := []string{
		selectRunCodeRefQuery,
		selectRunEnvLockQuery,
		selectRunPolicySnapshotQuery,
		selectRunPolicySnapshotSHAQuery,
		selectEnvDefinitionExistsQuery,
	}
	for _, query := range queries {
		if !strings.Contains(query, "project_id = $1") {
			t.Fatalf("expected project scoping clause, got query: %s", query)
		}
	}
}
