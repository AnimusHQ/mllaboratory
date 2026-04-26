package domain

import "testing"

func TestCanTransitionRunState(t *testing.T) {
	cases := []struct {
		name    string
		from    RunState
		to      RunState
		allowed bool
	}{
		{"created->planned", RunStateCreated, RunStatePlanned, true},
		{"planned->running", RunStatePlanned, RunStateRunning, true},
		{"running->succeeded", RunStateRunning, RunStateSucceeded, true},
		{"succeeded->failed", RunStateSucceeded, RunStateFailed, false},
		{"succeeded->running", RunStateSucceeded, RunStateRunning, false},
		{"canceled->running", RunStateCanceled, RunStateRunning, false},
		{"dryrun_succeeded->running", RunStateDryRunSucceeded, RunStateRunning, true},
		{"failed->running", RunStateFailed, RunStateRunning, false},
	}
	for _, tc := range cases {
		if got := CanTransitionRunState(tc.from, tc.to); got != tc.allowed {
			t.Fatalf("%s: expected %v got %v", tc.name, tc.allowed, got)
		}
	}
}

func TestIsTerminalRunState(t *testing.T) {
	cases := []struct {
		state    RunState
		terminal bool
	}{
		{RunStateSucceeded, true},
		{RunStateFailed, true},
		{RunStateCanceled, true},
		{RunStateDryRunSucceeded, true},
		{RunStateRunning, false},
		{RunStatePlanned, false},
	}
	for _, tc := range cases {
		if got := IsTerminalRunState(tc.state); got != tc.terminal {
			t.Fatalf("state %s: expected %v got %v", tc.state, tc.terminal, got)
		}
	}
}
