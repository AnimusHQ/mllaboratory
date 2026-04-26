package postgres

import (
	"strings"
	"testing"
)

func TestModelInsertQueryIsIdempotent(t *testing.T) {
	if !strings.Contains(insertModelQuery, "ON CONFLICT (project_id, idempotency_key) DO NOTHING") {
		t.Fatalf("expected idempotent insert, got query: %s", insertModelQuery)
	}
}

func TestModelQueriesAreProjectScoped(t *testing.T) {
	queries := []string{selectModelByIDQuery, selectModelByIdempotencyQuery}
	for _, query := range queries {
		if !strings.Contains(query, "project_id") {
			t.Fatalf("expected project scoping in query: %s", query)
		}
	}
}
