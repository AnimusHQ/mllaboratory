package main

import (
	"context"
	"errors"
	"testing"

	"github.com/animus-labs/animus-go/closed/internal/dataplane"
	"github.com/animus-labs/animus-go/closed/internal/domain"
	"github.com/animus-labs/animus-go/closed/internal/repo/postgres"
)

type stubDPEventReader struct {
	record postgres.RunDPEventRecord
	err    error
}

func (s stubDPEventReader) GetEvent(_ context.Context, _ string) (postgres.RunDPEventRecord, error) {
	if s.err != nil {
		return postgres.RunDPEventRecord{}, s.err
	}
	return s.record, nil
}

func TestEnsureDPEventMatches(t *testing.T) {
	base := postgres.RunDPEventRecord{
		EventID:   "evt-1",
		RunID:     "run-1",
		ProjectID: "proj-1",
		EventType: dataplane.EventTypeHeartbeat,
	}

	tests := []struct {
		name      string
		record    postgres.RunDPEventRecord
		runID     string
		projectID string
		eventType string
		wantErr   error
	}{
		{"match", base, "run-1", "proj-1", dataplane.EventTypeHeartbeat, nil},
		{"run-mismatch", base, "run-2", "proj-1", dataplane.EventTypeHeartbeat, errDPEventMismatch},
		{"project-mismatch", base, "run-1", "proj-2", dataplane.EventTypeHeartbeat, errDPEventMismatch},
		{"type-mismatch", base, "run-1", "proj-1", dataplane.EventTypeTerminal, errDPEventMismatch},
	}

	for _, tc := range tests {
		err := ensureDPEventMatches(context.Background(), stubDPEventReader{record: tc.record}, tc.record.EventID, tc.runID, tc.projectID, tc.eventType)
		switch {
		case tc.wantErr == nil && err != nil:
			t.Fatalf("%s: unexpected error %v", tc.name, err)
		case tc.wantErr != nil && !errors.Is(err, tc.wantErr):
			t.Fatalf("%s: expected %v got %v", tc.name, tc.wantErr, err)
		}
	}
}

func TestDispatchStatusFromRunState(t *testing.T) {
	cases := []struct {
		state domain.RunState
		want  string
	}{
		{domain.RunStateSucceeded, dataplane.DispatchStatusSucceeded},
		{domain.RunStateFailed, dataplane.DispatchStatusFailed},
		{domain.RunStateCanceled, dataplane.DispatchStatusCanceled},
		{domain.RunStateRunning, dataplane.DispatchStatusRunning},
		{domain.RunStateCreated, dataplane.DispatchStatusError},
	}

	for _, tc := range cases {
		if got := dispatchStatusFromRunState(tc.state); got != tc.want {
			t.Fatalf("state %s: expected %s got %s", tc.state, tc.want, got)
		}
	}
}
