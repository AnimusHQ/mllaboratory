package postgres

import (
	"strings"
	"testing"
)

func TestImageVerificationUpsertIsIdempotent(t *testing.T) {
	if !strings.Contains(upsertImageVerificationQuery, "ON CONFLICT") {
		t.Fatalf("expected ON CONFLICT in upsert query: %s", upsertImageVerificationQuery)
	}
}

func TestImageVerificationQueriesAreProjectScoped(t *testing.T) {
	queries := []string{
		selectLatestImageVerificationQuery,
		selectImageVerificationsListQuery,
	}
	for _, query := range queries {
		if !strings.Contains(query, "project_id") {
			t.Fatalf("expected project scoping in query: %s", query)
		}
	}
}
