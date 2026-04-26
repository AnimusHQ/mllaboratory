package main

import (
	"context"
	"errors"
	"strings"

	"github.com/animus-labs/animus-go/closed/internal/domain"
	"github.com/animus-labs/animus-go/closed/internal/execution/plan"
	"github.com/animus-labs/animus-go/closed/internal/execution/state"
	"github.com/animus-labs/animus-go/closed/internal/repo"
)

type derivedRunState struct {
	State       domain.RunState
	PlanExists  bool
	AttemptsMap map[string]int
}

func loadExecutionPlan(ctx context.Context, planStore repo.PlanRepository, projectID, runID string) (*domain.ExecutionPlan, bool, error) {
	record, err := planStore.GetPlan(ctx, projectID, runID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return nil, false, nil
		}
		return nil, false, err
	}
	execPlan, err := plan.UnmarshalExecutionPlan(record.Plan)
	if err != nil {
		return nil, false, err
	}
	return &execPlan, true, nil
}

func deriveRunStateFromRecords(planSpec *domain.ExecutionPlan, planExists bool, stepExecutions []repo.StepExecutionRecord) derivedRunState {
	expected := planStepNames(planSpec)
	outcomes, attempts := state.DeriveStepOutcomes(stepExecutions, expected)
	derived := state.DeriveRunState(planExists, outcomes, expected)
	if len(attempts) == 0 {
		attempts = nil
	}
	return derivedRunState{
		State:       derived,
		PlanExists:  planExists,
		AttemptsMap: attempts,
	}
}

func planStepNames(plan *domain.ExecutionPlan) []string {
	if plan == nil || len(plan.Steps) == 0 {
		return nil
	}
	names := make([]string, 0, len(plan.Steps))
	for _, step := range plan.Steps {
		name := strings.TrimSpace(step.Name)
		if name == "" {
			continue
		}
		names = append(names, name)
	}
	return names
}
