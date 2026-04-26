package main

import (
	"context"
	"testing"
	"time"

	"github.com/animus-labs/animus-go/closed/internal/dataplane"
	"github.com/animus-labs/animus-go/closed/internal/domain"
	"github.com/animus-labs/animus-go/closed/internal/repo/postgres"
)

func TestExpireDevEnvironmentsDeletesExpired(t *testing.T) {
	now := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	store := newStubDevEnvStore()
	record := postgres.DevEnvironmentRecord{
		Environment: domain.DevEnvironment{
			ID:          "dev-1",
			ProjectID:   "proj-1",
			State:       domain.DevEnvStateActive,
			ExpiresAt:   now.Add(-time.Minute),
			DPJobName:   "job-1",
			DPNamespace: "ns",
		},
		IdempotencyKey: "idem-1",
	}
	store.records["dev-1"] = record
	store.listExpired = []postgres.DevEnvironmentRecord{record}

	audit := &captureAudit{}
	client := &stubDevEnvDPClient{deleteResp: dataplane.DevEnvDeleteResponse{Deleted: true}}

	err := expireDevEnvironments(context.Background(), store, client, audit, "proj-1", now, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(store.updateStates) < 2 {
		t.Fatalf("expected state updates, got %d", len(store.updateStates))
	}
	if store.updateStates[0] != domain.DevEnvStateExpired {
		t.Fatalf("expected first state expired, got %s", store.updateStates[0])
	}
	if store.updateStates[1] != domain.DevEnvStateDeleted {
		t.Fatalf("expected second state deleted, got %s", store.updateStates[1])
	}
	if audit.count(auditDevEnvExpired) != 1 {
		t.Fatalf("expected expired audit event")
	}
	if audit.count(auditDevEnvDeleted) != 1 {
		t.Fatalf("expected deleted audit event")
	}
	if client.deleteCalls != 1 {
		t.Fatalf("expected delete call, got %d", client.deleteCalls)
	}
}
