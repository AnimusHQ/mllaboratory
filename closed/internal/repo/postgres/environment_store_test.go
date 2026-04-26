package postgres

import (
	"strings"
	"testing"
)

func TestEnvironmentStoreInsertQueriesAreIdempotent(t *testing.T) {
	queries := []string{
		insertEnvironmentDefinitionQuery,
		insertEnvironmentLockQuery,
	}
	for _, query := range queries {
		if !strings.Contains(query, "ON CONFLICT (project_id, idempotency_key) DO NOTHING") {
			t.Fatalf("expected idempotent insert, got query: %s", query)
		}
	}
}

func TestEnvironmentStoreQueriesAreProjectScoped(t *testing.T) {
	queries := []string{
		selectEnvironmentDefinitionByIDQuery,
		selectEnvironmentDefinitionByIdempotencyQuery,
		selectEnvironmentDefinitionsListQuery,
		selectEnvironmentDefinitionMaxVersionQuery,
		selectEnvironmentLockByIDQuery,
		selectEnvironmentLockByIdempotencyQuery,
		selectEnvironmentLocksListQuery,
	}
	for _, query := range queries {
		if !strings.Contains(query, "project_id") {
			t.Fatalf("expected project scoping in query: %s", query)
		}
	}
}
