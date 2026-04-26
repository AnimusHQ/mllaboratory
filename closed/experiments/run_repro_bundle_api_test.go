package main

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/animus-labs/animus-go/closed/internal/platform/auth"
)

func TestReproBundleAuditEvent(t *testing.T) {
	now := time.Date(2026, 2, 2, 10, 0, 0, 0, time.UTC)
	identity := auth.Identity{Subject: "user-1"}
	req := httptest.NewRequest(http.MethodGet, "/projects/proj-1/runs/run-1/reproducibility-bundle", nil)
	req.RemoteAddr = "203.0.113.10:1234"
	req.Header.Set("X-Request-Id", "req-123")
	req.Header.Set("User-Agent", "test-agent")

	event := reproBundleAuditEvent(now, identity, req, "proj-1", "run-1", "spec-hash", "policy-sha")

	if !event.OccurredAt.Equal(now) {
		t.Fatalf("unexpected occurredAt: %s", event.OccurredAt)
	}
	if event.Actor != "user-1" {
		t.Fatalf("unexpected actor: %s", event.Actor)
	}
	if event.Action != "run.repro_bundle.exported" {
		t.Fatalf("unexpected action: %s", event.Action)
	}
	if event.ResourceType != "run" || event.ResourceID != "run-1" {
		t.Fatalf("unexpected resource: %s/%s", event.ResourceType, event.ResourceID)
	}
	if event.RequestID != "req-123" {
		t.Fatalf("unexpected request id: %s", event.RequestID)
	}
	if event.UserAgent != "test-agent" {
		t.Fatalf("unexpected user agent: %s", event.UserAgent)
	}
	if event.IP == nil || !event.IP.Equal(net.ParseIP("203.0.113.10")) {
		t.Fatalf("unexpected ip: %v", event.IP)
	}
	payload, ok := event.Payload.(map[string]any)
	if !ok {
		t.Fatalf("unexpected payload type: %T", event.Payload)
	}
	if payload["service"] != "experiments" ||
		payload["project_id"] != "proj-1" ||
		payload["run_id"] != "run-1" ||
		payload["spec_hash"] != "spec-hash" ||
		payload["policy_snapshot_sha256"] != "policy-sha" {
		t.Fatalf("unexpected payload: %+v", payload)
	}
}
