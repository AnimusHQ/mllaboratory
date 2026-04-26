package postgres

import (
	"strings"
	"testing"
)

func TestModelVersionInsertQueryIsIdempotent(t *testing.T) {
	if !strings.Contains(insertModelVersionQuery, "ON CONFLICT (project_id, idempotency_key) DO NOTHING") {
		t.Fatalf("expected idempotent insert, got query: %s", insertModelVersionQuery)
	}
}

func TestModelVersionQueriesAreProjectScoped(t *testing.T) {
	queries := []string{selectModelVersionByIDQuery, selectModelVersionByIdempotencyQuery, updateModelVersionStatusQuery}
	for _, query := range queries {
		if !strings.Contains(query, "project_id") {
			t.Fatalf("expected project scoping in query: %s", query)
		}
	}
}

func TestModelVersionTransitionQueryHasProjectScope(t *testing.T) {
	if !strings.Contains(insertModelVersionTransitionQuery, "project_id") {
		t.Fatalf("expected project scoping in transition query: %s", insertModelVersionTransitionQuery)
	}
}

func TestModelExportInsertQueryIsIdempotent(t *testing.T) {
	if !strings.Contains(insertModelExportQuery, "ON CONFLICT (project_id, idempotency_key) DO NOTHING") {
		t.Fatalf("expected idempotent insert, got query: %s", insertModelExportQuery)
	}
}

func TestModelExportQueriesAreProjectScoped(t *testing.T) {
	if !strings.Contains(selectModelExportByIdempotencyQuery, "project_id") {
		t.Fatalf("expected project scoping in query: %s", selectModelExportByIdempotencyQuery)
	}
}
