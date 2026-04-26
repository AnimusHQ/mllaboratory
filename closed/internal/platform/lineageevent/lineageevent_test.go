package lineageevent

import (
	"testing"
	"time"
)

func TestComputeIntegritySHA256_Deterministic(t *testing.T) {
	occurredAt := time.Unix(1700000000, 0).UTC()
	event := Event{
		OccurredAt:  occurredAt,
		Actor:       "alice",
		RequestID:   "req-123",
		SubjectType: "dataset_version",
		SubjectID:   "dv-1",
		Predicate:   "used_by",
		ObjectType:  "experiment_run",
		ObjectID:    "run-1",
	}
	metadataJSON := []byte(`{"a":1,"b":"x"}`)

	a, err := ComputeIntegritySHA256(event, metadataJSON)
	if err != nil {
		t.Fatalf("ComputeIntegritySHA256() err=%v", err)
	}
	b, err := ComputeIntegritySHA256(event, metadataJSON)
	if err != nil {
		t.Fatalf("ComputeIntegritySHA256() err=%v", err)
	}
	if a != b {
		t.Fatalf("integrity mismatch: %q vs %q", a, b)
	}
}

func TestComputeIntegritySHA256_ChangesOnMetadata(t *testing.T) {
	occurredAt := time.Unix(1700000000, 0).UTC()
	event := Event{
		OccurredAt:  occurredAt,
		Actor:       "alice",
		SubjectType: "dataset_version",
		SubjectID:   "dv-1",
		Predicate:   "used_by",
		ObjectType:  "experiment_run",
		ObjectID:    "run-1",
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
