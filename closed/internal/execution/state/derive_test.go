package state

import (
	"reflect"
	"testing"

	"github.com/animus-labs/animus-go/closed/internal/domain"
	"github.com/animus-labs/animus-go/closed/internal/repo"
)

func TestDeriveRunState(t *testing.T) {
	expectedSteps := []string{"a", "b"}

	tests := []struct {
		name         string
		planExists   bool
		expected     []string
		executions   []repo.StepExecutionRecord
		wantRunState domain.RunState
	}{
		{
			name:         "no plan",
			planExists:   false,
			expected:     expectedSteps,
			executions:   []repo.StepExecutionRecord{stepRecord("a", 1, "Succeeded")},
			wantRunState: domain.RunStateCreated,
		},
		{
			name:         "plan no executions",
			planExists:   true,
			expected:     expectedSteps,
			wantRunState: domain.RunStatePlanned,
		},
		{
			name:       "partial execution",
			planExists: true,
			expected:   expectedSteps,
			executions: []repo.StepExecutionRecord{
				stepRecord("a", 1, "Succeeded"),
			},
			wantRunState: domain.RunStateDryRunRunning,
		},
		{
			name:       "all succeeded",
			planExists: true,
			expected:   expectedSteps,
			executions: []repo.StepExecutionRecord{
				stepRecord("a", 1, "Succeeded"),
				stepRecord("b", 1, "Succeeded"),
			},
			wantRunState: domain.RunStateDryRunSucceeded,
		},
		{
			name:       "failed step",
			planExists: true,
			expected:   expectedSteps,
			executions: []repo.StepExecutionRecord{
				stepRecord("a", 1, "Failed"),
			},
			wantRunState: domain.RunStateDryRunFailed,
		},
		{
			name:       "skipped after failure",
			planExists: true,
			expected:   expectedSteps,
			executions: []repo.StepExecutionRecord{
				stepRecord("a", 1, "Failed"),
				stepRecord("b", 1, "Skipped"),
			},
			wantRunState: domain.RunStateDryRunFailed,
		},
	}

	for _, tc := range tests {
		outcomes, _ := DeriveStepOutcomes(tc.executions, tc.expected)
		if got := DeriveRunState(tc.planExists, outcomes, tc.expected); got != tc.wantRunState {
			t.Fatalf("%s: expected %s got %s", tc.name, tc.wantRunState, got)
		}
	}
}

func TestDeriveStepOutcomesAttempts(t *testing.T) {
	execs := []repo.StepExecutionRecord{
		stepRecord("a", 1, "Retried"),
		stepRecord("a", 2, "Failed"),
	}
	outcomes, attempts := DeriveStepOutcomes(execs, []string{"a"})
	if attempts["a"] != 2 {
		t.Fatalf("expected attempts 2, got %d", attempts["a"])
	}
	if outcomes["a"] != domain.StepOutcomeFailed {
		t.Fatalf("expected failed outcome, got %s", outcomes["a"])
	}
}

func TestDeriveRunStateOrderIndependent(t *testing.T) {
	expectedSteps := []string{"a", "b"}
	execs := []repo.StepExecutionRecord{
		stepRecord("a", 1, "Retried"),
		stepRecord("b", 1, "Succeeded"),
		stepRecord("a", 2, "Succeeded"),
	}
	outcomesA, attemptsA := DeriveStepOutcomes(execs, expectedSteps)
	stateA := DeriveRunState(true, outcomesA, expectedSteps)

	reversed := []repo.StepExecutionRecord{
		execs[2],
		execs[1],
		execs[0],
	}
	outcomesB, attemptsB := DeriveStepOutcomes(reversed, expectedSteps)
	stateB := DeriveRunState(true, outcomesB, expectedSteps)

	if stateA != stateB {
		t.Fatalf("expected same state, got %s vs %s", stateA, stateB)
	}
	if !reflect.DeepEqual(outcomesA, outcomesB) {
		t.Fatalf("expected same outcomes, got %+v vs %+v", outcomesA, outcomesB)
	}
	if !reflect.DeepEqual(attemptsA, attemptsB) {
		t.Fatalf("expected same attempts, got %+v vs %+v", attemptsA, attemptsB)
	}
}

func stepRecord(step string, attempt int, status string) repo.StepExecutionRecord {
	return repo.StepExecutionRecord{
		ProjectID: "proj-1",
		RunID:     "run-1",
		StepName:  step,
		Attempt:   attempt,
		Status:    status,
	}
}
