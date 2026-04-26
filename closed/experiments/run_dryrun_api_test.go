package main

import (
	"reflect"
	"testing"

	"github.com/animus-labs/animus-go/closed/internal/domain"
	"github.com/animus-labs/animus-go/closed/internal/execution/plan"
)

func TestExecutionPlanCodec(t *testing.T) {
	execPlan := domain.ExecutionPlan{
		RunID:     "run-1",
		ProjectID: "proj-1",
		Steps: []domain.ExecutionPlanStep{
			{
				Name: "step-a",
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
		},
		Edges: []domain.ExecutionPlanEdge{
			{From: "step-a", To: "step-b"},
		},
	}

	raw, err := plan.MarshalExecutionPlan(execPlan)
	if err != nil {
		t.Fatalf("marshal plan: %v", err)
	}
	decoded, err := plan.UnmarshalExecutionPlan(raw)
	if err != nil {
		t.Fatalf("decode plan: %v", err)
	}

	if decoded.RunID != execPlan.RunID || decoded.ProjectID != execPlan.ProjectID {
		t.Fatalf("unexpected ids: %+v", decoded)
	}
	if !reflect.DeepEqual(decoded.Steps, execPlan.Steps) {
		t.Fatalf("steps mismatch: %+v vs %+v", decoded.Steps, execPlan.Steps)
	}
	if !reflect.DeepEqual(decoded.Edges, execPlan.Edges) {
		t.Fatalf("edges mismatch: %+v vs %+v", decoded.Edges, execPlan.Edges)
	}
}
