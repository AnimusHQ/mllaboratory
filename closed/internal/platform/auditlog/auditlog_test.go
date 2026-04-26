package auditlog

import (
	"net"
	"testing"
	"time"
)

func TestComputeIntegritySHA256_Deterministic(t *testing.T) {
	occurredAt := time.Unix(1700000000, 0).UTC()
	event := Event{
		OccurredAt:   occurredAt,
		Actor:        "alice",
		Action:       "auth.forbidden",
		ResourceType: "http",
		ResourceID:   "GET /api/dataset-registry/datasets",
		RequestID:    "req-123",
		IP:           net.ParseIP("192.0.2.1"),
		UserAgent:    "test-agent",
	}
	payloadJSON := []byte(`{"a":1,"b":"x"}`)

	a, err := ComputeIntegritySHA256(event, payloadJSON)
	if err != nil {
		t.Fatalf("ComputeIntegritySHA256() err=%v", err)
	}
	b, err := ComputeIntegritySHA256(event, payloadJSON)
	if err != nil {
		t.Fatalf("ComputeIntegritySHA256() err=%v", err)
	}
	if a != b {
		t.Fatalf("integrity mismatch: %q vs %q", a, b)
	}
}

func TestComputeIntegritySHA256_ChangesOnPayload(t *testing.T) {
	occurredAt := time.Unix(1700000000, 0).UTC()
	event := Event{
		OccurredAt:   occurredAt,
		Actor:        "alice",
		Action:       "auth.forbidden",
		ResourceType: "http",
		ResourceID:   "GET /api/dataset-registry/datasets",
	}

	a, err := ComputeIntegritySHA256(event, []byte(`{"a":1}`))
	if err != nil {
		t.Fatalf("ComputeIntegritySHA256() err=%v", err)
	}
	b, err := ComputeIntegritySHA256(event, []byte(`{"a":2}`))
	if err != nil {
		t.Fatalf("ComputeIntegritySHA256() err=%v", err)
	}
	if a == b {
		t.Fatalf("expected integrity to differ")
	}
}
