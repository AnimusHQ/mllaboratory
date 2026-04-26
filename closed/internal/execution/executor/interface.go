package executor

import (
	"context"

	"github.com/animus-labs/animus-go/closed/internal/domain"
)

// DryRunExecutor simulates execution without running user code.
type DryRunExecutor interface {
	DryRun(ctx context.Context, input DryRunInput) (DryRunResult, error)
}

type DryRunInput struct {
	ProjectID string
	RunID     string
	SpecHash  string
	Plan      domain.ExecutionPlan
}

type DryRunResult struct {
	Status   string
	Steps    []DryRunStepResult
	Attempts []DryRunAttempt
	Existing bool
}

type DryRunStepResult struct {
	Name     string
	Attempts int
	Status   string
}

type DryRunAttempt struct {
	StepName string
	Attempt  int
	Status   string
}
