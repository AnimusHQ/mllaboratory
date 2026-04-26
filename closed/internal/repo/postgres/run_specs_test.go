package postgres

import (
	"strings"
	"testing"
)

func TestRunSpecInsertQueryIsIdempotent(t *testing.T) {
	if !strings.Contains(insertRunSpecQuery, "ON CONFLICT (project_id, idempotency_key) DO NOTHING") {
		t.Fatalf("expected idempotency conflict clause in insert query")
	}
	if !strings.Contains(selectRunByIDQuery, "project_id = $1") {
		t.Fatalf("expected project_id predicate in run lookup query")
	}
	if !strings.Contains(selectRunByIdempotencyQuery, "project_id = $1") {
		t.Fatalf("expected project_id predicate in idempotency lookup query")
	}
}
