package postgres

import (
	"strings"
	"testing"
)

func TestStepExecutionQueriesProjectScoped(t *testing.T) {
	if !strings.Contains(insertStepExecutionQuery, "ON CONFLICT (project_id, run_id, step_name, attempt) DO NOTHING") {
		t.Fatalf("expected idempotency conflict clause in insert query")
	}
	if !strings.Contains(selectStepExecutionQuery, "project_id = $1") {
		t.Fatalf("expected project_id predicate in select query")
	}
	if !strings.Contains(listStepExecutionsByRunQuery, "project_id = $1") {
		t.Fatalf("expected project_id predicate in list query")
	}
	if !strings.Contains(listStepExecutionsByRunQuery, "ORDER BY") {
		t.Fatalf("expected ORDER BY in list query")
	}
}
