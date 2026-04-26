package postgres

import (
	"strings"
	"testing"
)

func TestRegistryPolicyUpsertUsesOnConflict(t *testing.T) {
	if !strings.Contains(upsertRegistryPolicyQuery, "ON CONFLICT") {
		t.Fatalf("expected ON CONFLICT in upsert query: %s", upsertRegistryPolicyQuery)
	}
}

func TestRegistryPolicyQueriesAreProjectScoped(t *testing.T) {
	if !strings.Contains(selectRegistryPolicyQuery, "project_id") {
		t.Fatalf("expected project scoping in query: %s", selectRegistryPolicyQuery)
	}
}
