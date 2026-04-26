package main

import (
	"testing"

	"github.com/animus-labs/animus-go/closed/internal/domain"
)

func TestMapStatusToRunStateUnknown(t *testing.T) {
	if got := mapStatusToRunState("unknown-state"); got != "" {
		t.Fatalf("expected unknown status to map to empty run state, got %s", got)
	}
}

func TestMapStatusToRunStateAliases(t *testing.T) {
	tests := []struct {
		input string
		want  domain.RunState
	}{
		{"running", domain.RunStateRunning},
		{"pending", domain.RunStateRunning},
		{"succeeded", domain.RunStateSucceeded},
		{"failed", domain.RunStateFailed},
	}
	for _, tc := range tests {
		if got := mapStatusToRunState(tc.input); got != tc.want {
			t.Fatalf("status %q: expected %s got %s", tc.input, tc.want, got)
		}
	}
}
