package plan

import (
	"reflect"
	"testing"

	"github.com/animus-labs/animus-go/closed/internal/domain"
)

func TestBuildPlanDeterministicOrdering(t *testing.T) {
	spec := domain.PipelineSpec{
		APIVersion:  "animus/v1alpha1",
		Kind:        "Pipeline",
		SpecVersion: "1.0",
		Spec: domain.PipelineSpecBody{
			Steps: []domain.PipelineStep{
				step("step-b"),
				step("step-a"),
				step("step-c"),
			},
			Dependencies: []domain.PipelineDependency{
				{From: "step-a", To: "step-c"},
				{From: "step-b", To: "step-c"},
			},
		},
	}

	first, err := BuildPlan(spec, "run-1", "proj-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	second, err := BuildPlan(spec, "run-1", "proj-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	firstOrder := extractStepNames(first)
	secondOrder := extractStepNames(second)
	if !reflect.DeepEqual(firstOrder, secondOrder) {
		t.Fatalf("expected deterministic order, got %v vs %v", firstOrder, secondOrder)
	}
	if want := []string{"step-a", "step-b", "step-c"}; !reflect.DeepEqual(firstOrder, want) {
		t.Fatalf("expected order %v, got %v", want, firstOrder)
	}
}

func step(name string) domain.PipelineStep {
	return domain.PipelineStep{
		Name:    name,
		Image:   "ghcr.io/acme/train@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Command: []string{"echo"},
		Args:    []string{"ok"},
		Inputs: domain.PipelineStepInputs{
			Datasets:  []domain.PipelineDatasetInput{},
			Artifacts: []domain.PipelineArtifactInput{},
		},
		Outputs: domain.PipelineStepOutputs{
			Artifacts: []domain.PipelineArtifactOutput{},
		},
		Env: []domain.EnvVar{},
		Resources: domain.PipelineResources{
			CPU:    "1",
			Memory: "1Gi",
			GPU:    0,
		},
		RetryPolicy: domain.PipelineRetryPolicy{
			MaxAttempts: 1,
			Backoff: domain.PipelineBackoff{
				Type:           "fixed",
				InitialSeconds: 0,
				MaxSeconds:     0,
				Multiplier:     1,
			},
		},
	}
}

func extractStepNames(plan domain.ExecutionPlan) []string {
	out := make([]string, 0, len(plan.Steps))
	for _, step := range plan.Steps {
		out = append(out, step.Name)
	}
	return out
}
