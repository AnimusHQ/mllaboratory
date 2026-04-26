package postgres

import (
	"strings"
	"testing"
)

func TestDPEventStoreInsertIsIdempotent(t *testing.T) {
	if !strings.Contains(insertRunDPEventQuery, "ON CONFLICT (event_id) DO NOTHING") {
		t.Fatalf("expected idempotent insert, got: %s", insertRunDPEventQuery)
	}
}

func TestRunDispatchInsertIsIdempotent(t *testing.T) {
	if !strings.Contains(insertRunDispatchQuery, "ON CONFLICT (project_id, idempotency_key) DO NOTHING") {
		t.Fatalf("expected idempotent insert, got: %s", insertRunDispatchQuery)
	}
}

func TestDPEventStoreQueriesAreProjectScoped(t *testing.T) {
	queries := []string{
		selectRunDPLatestByTypeQuery,
		selectRunDispatchByIdempotencyQuery,
		selectRunDispatchByRunIDQuery,
	}
	for _, query := range queries {
		if !strings.Contains(query, "project_id") {
			t.Fatalf("expected project scoping in query: %s", query)
		}
	}
}
