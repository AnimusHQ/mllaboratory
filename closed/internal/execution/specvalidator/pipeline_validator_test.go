package specvalidator

import (
	"testing"

	"github.com/animus-labs/animus-go/closed/internal/domain"
)

const testDigest = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

func TestValidatePipelineSpec(t *testing.T) {
	tests := []struct {
		name    string
		spec    domain.PipelineSpec
		wantErr bool
	}{
		{
			name:    "ok minimal pipeline",
			spec:    minimalPipelineSpec(),
			wantErr: false,
		},
		{
			name:    "duplicate step name",
			spec:    withSecondStep(minimalPipelineSpec(), "step-a"),
			wantErr: true,
		},
		{
			name: "unknown dependency node",
			spec: withDependencies(minimalPipelineSpec(), []domain.PipelineDependency{
				{From: "step-a", To: "missing"},
			}),
			wantErr: true,
		},
		{
			name: "cycle detected",
			spec: withDependencies(withSecondStep(minimalPipelineSpec(), "step-b"), []domain.PipelineDependency{
				{From: "step-a", To: "step-b"},
				{From: "step-b", To: "step-a"},
			}),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		if err := ValidatePipelineSpec(tt.spec); (err != nil) != tt.wantErr {
			t.Fatalf("%s: expected err=%v, got %v", tt.name, tt.wantErr, err)
		}
	}
}

func minimalPipelineSpec() domain.PipelineSpec {
	return domain.PipelineSpec{
		APIVersion:  "animus/v1alpha1",
		Kind:        "Pipeline",
		SpecVersion: "1.0",
		Spec: domain.PipelineSpecBody{
			Steps: []domain.PipelineStep{
				{
					Name:    "step-a",
					Image:   "ghcr.io/acme/train@" + testDigest,
					Command: []string{"echo"},
					Args:    []string{"hello"},
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
				},
			},
			Dependencies: []domain.PipelineDependency{},
		},
	}
}

func withSecondStep(spec domain.PipelineSpec, name string) domain.PipelineSpec {
	steps := append([]domain.PipelineStep{}, spec.Spec.Steps...)
	steps = append(steps, domain.PipelineStep{
		Name:    name,
		Image:   "ghcr.io/acme/train@" + testDigest,
		Command: []string{"echo"},
		Args:    []string{"second"},
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
	})
	spec.Spec.Steps = steps
	return spec
}

func withDependencies(spec domain.PipelineSpec, deps []domain.PipelineDependency) domain.PipelineSpec {
	spec.Spec.Dependencies = deps
	return spec
}
