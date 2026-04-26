package state

import (
	"strings"

	"github.com/animus-labs/animus-go/closed/internal/domain"
	"github.com/animus-labs/animus-go/closed/internal/repo"
)

// DeriveRunState computes the run state from plan presence, step outcomes, and expected steps.
func DeriveRunState(planExists bool, stepOutcomes map[string]domain.StepOutcome, expectedSteps []string) domain.RunState {
	if !planExists {
		// TODO: treat step executions without a plan as inconsistent; derived state stays Created.
		return domain.RunStateCreated
	}
	if len(expectedSteps) == 0 {
		return domain.RunStatePlanned
	}
	if len(stepOutcomes) == 0 {
		return domain.RunStatePlanned
	}

	incomplete := false
	for _, step := range expectedSteps {
		stepName := strings.TrimSpace(step)
		if stepName == "" {
			continue
		}
		outcome, ok := stepOutcomes[stepName]
		if !ok || outcome == "" {
			incomplete = true
			continue
		}
		if outcome == domain.StepOutcomeFailed {
			return domain.RunStateDryRunFailed
		}
	}

	if incomplete {
		return domain.RunStateDryRunRunning
	}
	return domain.RunStateDryRunSucceeded
}

// DeriveStepOutcomes returns terminal outcomes and attempt counts per step.
func DeriveStepOutcomes(executions []repo.StepExecutionRecord, expectedSteps []string) (map[string]domain.StepOutcome, map[string]int) {
	outcomes := make(map[string]domain.StepOutcome)
	attempts := make(map[string]int)
	expected := map[string]struct{}{}
	if len(expectedSteps) > 0 {
		for _, step := range expectedSteps {
			name := strings.TrimSpace(step)
			if name == "" {
				continue
			}
			expected[name] = struct{}{}
		}
	}

	type agg struct {
		maxAttempt int
		outcome    domain.StepOutcome
	}
	state := make(map[string]*agg)
	for _, record := range executions {
		step := strings.TrimSpace(record.StepName)
		if step == "" {
			continue
		}
		if len(expected) > 0 {
			if _, ok := expected[step]; !ok {
				continue
			}
		}
		aggState := state[step]
		if aggState == nil {
			aggState = &agg{}
			state[step] = aggState
		}
		if record.Attempt > aggState.maxAttempt {
			aggState.maxAttempt = record.Attempt
			aggState.outcome = ""
		}
		if record.Attempt == aggState.maxAttempt {
			if outcome, ok := mapOutcome(record.Status); ok {
				aggState.outcome = outcome
			}
		}
	}

	for step, aggState := range state {
		if aggState.maxAttempt > 0 {
			attempts[step] = aggState.maxAttempt
		}
		if aggState.outcome != "" {
			outcomes[step] = aggState.outcome
		}
	}
	return outcomes, attempts
}

// DeriveStepOutcome returns attempts count and terminal outcome for a single step.
func DeriveStepOutcome(executions []repo.StepExecutionRecord) (int, domain.StepOutcome) {
	if len(executions) == 0 {
		return 0, ""
	}
	maxAttempt := 0
	var outcome domain.StepOutcome
	for _, record := range executions {
		if record.Attempt > maxAttempt {
			maxAttempt = record.Attempt
			outcome = ""
		}
		if record.Attempt == maxAttempt {
			if mapped, ok := mapOutcome(record.Status); ok {
				outcome = mapped
			}
		}
	}
	if maxAttempt == 0 {
		return 0, ""
	}
	return maxAttempt, outcome
}

func mapOutcome(status string) (domain.StepOutcome, bool) {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "succeeded":
		return domain.StepOutcomeSucceeded, true
	case "failed":
		return domain.StepOutcomeFailed, true
	case "skipped":
		return domain.StepOutcomeSkipped, true
	default:
		return "", false
	}
}
