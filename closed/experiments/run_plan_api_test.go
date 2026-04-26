package main

import (
	"reflect"
	"testing"

	"github.com/animus-labs/animus-go/closed/internal/domain"
	"github.com/animus-labs/animus-go/closed/internal/execution/plan"
)

func TestMarshalExecutionPlan(t *testing.T) {
	execPlan := domain.ExecutionPlan{
		RunID:     "run-1",
		ProjectID: "proj-1",
		Steps: []domain.ExecutionPlanStep{
			{
				Name: "step-b",
				RetryPolicy: domain.PipelineRetryPolicy{
					MaxAttempts: 2,
					Backoff: domain.PipelineBackoff{
						Type:           "fixed",
						InitialSeconds: 1,
						MaxSeconds:     2,
						Multiplier:     1,
					},
				},
				AttemptStart: 1,
			},
			{
				Name: "step-a",
				RetryPolicy: domain.PipelineRetryPolicy{
					MaxAttempts: 1,
					Backoff: domain.PipelineBackoff{
						Type:           "fixed",
						InitialSeconds: 0,
						MaxSeconds:     0,
						Multiplier:     1,
					},
				},
				AttemptStart: 1,
			},
		},
		Edges: []domain.ExecutionPlanEdge{
			{From: "step-b", To: "step-a"},
		},
	}

	raw, err := plan.MarshalExecutionPlan(execPlan)
	if err != nil {
		t.Fatalf("marshal plan: %v", err)
	}

	decoded, err := plan.UnmarshalExecutionPlan(raw)
	if err != nil {
		t.Fatalf("unmarshal plan: %v", err)
	}

	if decoded.RunID != execPlan.RunID || decoded.ProjectID != execPlan.ProjectID {
		t.Fatalf("unexpected ids: %+v", decoded)
	}
	if !reflect.DeepEqual(decoded.Steps, execPlan.Steps) {
		t.Fatalf("unexpected steps: %+v", decoded.Steps)
	}
	if !reflect.DeepEqual(decoded.Edges, execPlan.Edges) {
		t.Fatalf("unexpected edges: %+v", decoded.Edges)
	}
}
