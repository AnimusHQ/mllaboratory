package postgres

import (
	"strings"
	"testing"
)

func TestDevEnvironmentInsertQueryIsIdempotent(t *testing.T) {
	if !strings.Contains(insertDevEnvironmentQuery, "ON CONFLICT (project_id, idempotency_key) DO NOTHING") {
		t.Fatalf("expected idempotent insert, got query: %s", insertDevEnvironmentQuery)
	}
}

func TestDevEnvironmentQueriesAreProjectScoped(t *testing.T) {
	queries := []string{
		selectDevEnvironmentByIDQuery,
		selectDevEnvironmentByIdempotencyQuery,
		selectDevEnvironmentListQuery,
		selectDevEnvironmentExpiredQuery,
	}
	for _, query := range queries {
		if !strings.Contains(query, "project_id") {
			t.Fatalf("expected project scoping in query: %s", query)
		}
	}
}

func TestDevEnvPolicyQueriesAreProjectScoped(t *testing.T) {
	queries := []string{
		selectDevEnvPolicySnapshotQuery,
		selectDevEnvPolicySnapshotSHAQuery,
	}
	for _, query := range queries {
		if !strings.Contains(query, "project_id") {
			t.Fatalf("expected project scoping in query: %s", query)
		}
	}
}

func TestDevEnvSessionQueriesAreProjectScoped(t *testing.T) {
	if !strings.Contains(selectDevEnvSessionByIDQuery, "project_id") {
		t.Fatalf("expected project scoping in query: %s", selectDevEnvSessionByIDQuery)
	}
}
