package main

import (
	"encoding/json"
	"testing"
	"time"
)

func TestGetRunResponseIncludesDerivedState(t *testing.T) {
	resp := getRunResponse{
		RunID:      "run-1",
		Status:     "created",
		State:      "planned",
		SpecHash:   "hash",
		CreatedAt:  time.Unix(0, 0).UTC(),
		PlanExists: true,
		AttemptsByStep: map[string]int{
			"step-a": 2,
		},
	}

	body := decodeJSONMap(t, resp)

	assertJSONField(t, body, "state", "planned")
	assertJSONField(t, body, "planExists", true)
	assertJSONMapValue(t, body, "attemptsByStep", "step-a", float64(2))
}

func TestDryRunExecutionsResponseIncludesDerivedState(t *testing.T) {
	resp := dryRunExecutionsResponse{
		RunID:      "run-1",
		State:      "dryrun_succeeded",
		PlanExists: true,
		AttemptsByStep: map[string]int{
			"step-a": 1,
		},
		Executions: []stepExecutionResponse{
			{
				StepName:  "step-a",
				Attempt:   1,
				Status:    "succeeded",
				StartedAt: time.Unix(0, 0).UTC(),
			},
		},
	}

	body := decodeJSONMap(t, resp)

	assertJSONField(t, body, "state", "dryrun_succeeded")
	assertJSONField(t, body, "planExists", true)
	assertJSONMapValue(t, body, "attemptsByStep", "step-a", float64(1))
}

func TestDryRunResponseIncludesAttemptsByStep(t *testing.T) {
	resp := dryRunResponse{
		RunID:    "run-1",
		Status:   "succeeded",
		State:    "dryrun_succeeded",
		Existing: true,
		AttemptsByStep: map[string]int{
			"step-a": 3,
		},
	}

	body := decodeJSONMap(t, resp)

	assertJSONField(t, body, "state", "dryrun_succeeded")
	assertJSONMapValue(t, body, "attemptsByStep", "step-a", float64(3))
}

func decodeJSONMap(t *testing.T, v any) map[string]any {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	return body
}

func assertJSONField(t *testing.T, body map[string]any, field string, want any) {
	t.Helper()
	got, ok := body[field]
	if !ok {
		t.Fatalf("expected field %q", field)
	}
	if got != want {
		t.Fatalf("expected %q to be %v, got %v", field, want, got)
	}
}

func assertJSONMapValue(t *testing.T, body map[string]any, field, key string, want any) {
	t.Helper()
	raw, ok := body[field]
	if !ok {
		t.Fatalf("expected field %q", field)
	}
	m, ok := raw.(map[string]any)
	if !ok {
		t.Fatalf("expected %q to be map, got %T", field, raw)
	}
	got, ok := m[key]
	if !ok {
		t.Fatalf("expected %q to include key %q", field, key)
	}
	if got != want {
		t.Fatalf("expected %q[%q] to be %v, got %v", field, key, want, got)
	}
}
