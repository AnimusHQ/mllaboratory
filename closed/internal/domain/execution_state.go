package domain

import "strings"

// RunState represents the derived execution state of a run.
type RunState string

const (
	RunStateCreated         RunState = "created"
	RunStatePlanned         RunState = "planned"
	RunStateRunning         RunState = "running"
	RunStateSucceeded       RunState = "succeeded"
	RunStateFailed          RunState = "failed"
	RunStateCanceled        RunState = "canceled"
	RunStateDryRunRunning   RunState = "dryrun_running"
	RunStateDryRunSucceeded RunState = "dryrun_succeeded"
	RunStateDryRunFailed    RunState = "dryrun_failed"
)

// StepOutcome represents a terminal step outcome.
type StepOutcome string

const (
	StepOutcomeSucceeded StepOutcome = "succeeded"
	StepOutcomeFailed    StepOutcome = "failed"
	StepOutcomeSkipped   StepOutcome = "skipped"
)

// StepState is kept for backward compatibility with earlier call sites.
type StepState = StepOutcome

const (
	StepStateSucceeded StepState = StepOutcomeSucceeded
	StepStateFailed    StepState = StepOutcomeFailed
	StepStateSkipped   StepState = StepOutcomeSkipped
)

// NormalizeRunState maps free-form status values to canonical run states.
func NormalizeRunState(value string) RunState {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case string(RunStateCreated), "pending":
		return RunStateCreated
	case string(RunStatePlanned):
		return RunStatePlanned
	case string(RunStateRunning):
		return RunStateRunning
	case string(RunStateSucceeded):
		return RunStateSucceeded
	case string(RunStateFailed):
		return RunStateFailed
	case string(RunStateCanceled), "cancelled":
		return RunStateCanceled
	case string(RunStateDryRunRunning):
		return RunStateDryRunRunning
	case string(RunStateDryRunSucceeded):
		return RunStateDryRunSucceeded
	case string(RunStateDryRunFailed):
		return RunStateDryRunFailed
	default:
		return ""
	}
}

// CanTransitionRunState enforces a forward-only state progression across execution states.
func CanTransitionRunState(current, next RunState) bool {
	if current == "" || next == "" {
		return false
	}
	if current == next {
		return true
	}
	for _, allowed := range allowedRunTransitions[current] {
		if allowed == next {
			return true
		}
	}
	return false
}

// IsTerminalRunState returns true when the state cannot progress further.
func IsTerminalRunState(state RunState) bool {
	switch state {
	case RunStateSucceeded, RunStateFailed, RunStateCanceled, RunStateDryRunSucceeded, RunStateDryRunFailed:
		return true
	default:
		return false
	}
}

func runStateOrder(state RunState) int {
	switch state {
	case RunStateCreated:
		return 1
	case RunStatePlanned:
		return 2
	case RunStateRunning:
		return 3
	case RunStateSucceeded, RunStateFailed, RunStateCanceled:
		return 4
	case RunStateDryRunRunning:
		return 5
	case RunStateDryRunSucceeded, RunStateDryRunFailed:
		return 6
	default:
		return 0
	}
}

var allowedRunTransitions = map[RunState][]RunState{
	RunStateCreated: {
		RunStatePlanned,
		RunStateRunning,
		RunStateDryRunRunning,
		RunStateCanceled,
	},
	RunStatePlanned: {
		RunStateRunning,
		RunStateDryRunRunning,
		RunStateDryRunSucceeded,
		RunStateDryRunFailed,
		RunStateCanceled,
	},
	RunStateDryRunRunning: {
		RunStateDryRunSucceeded,
		RunStateDryRunFailed,
		RunStateRunning,
		RunStateCanceled,
	},
	RunStateDryRunSucceeded: {
		RunStateRunning,
		RunStateCanceled,
	},
	RunStateDryRunFailed: {
		RunStateRunning,
		RunStateCanceled,
	},
	RunStateRunning: {
		RunStateSucceeded,
		RunStateFailed,
		RunStateCanceled,
	},
	RunStateSucceeded: {},
	RunStateFailed:    {},
	RunStateCanceled:  {},
}
