package domain

import (
	"errors"
	"fmt"
	"strings"
)

// PipelineSpec is a reusable execution template for deterministic planning.
type PipelineSpec struct {
	APIVersion  string
	Kind        string
	SpecVersion string
	Metadata    *PipelineMetadata
	Spec        PipelineSpecBody
}

type PipelineMetadata struct {
	Name        string
	Description string
	Labels      map[string]string
}

type PipelineSpecBody struct {
	Steps        []PipelineStep
	Dependencies []PipelineDependency
}

type PipelineDependency struct {
	From string
	To   string
}

type PipelineStep struct {
	Name        string
	Image       string
	Command     []string
	Args        []string
	Inputs      PipelineStepInputs
	Outputs     PipelineStepOutputs
	Env         []EnvVar
	Resources   PipelineResources
	RetryPolicy PipelineRetryPolicy
}

type PipelineStepInputs struct {
	Datasets  []PipelineDatasetInput
	Artifacts []PipelineArtifactInput
}

type PipelineStepOutputs struct {
	Artifacts []PipelineArtifactOutput
}

type PipelineDatasetInput struct {
	Name       string
	DatasetRef string
}

type PipelineArtifactInput struct {
	Name     string
	FromStep string
	Artifact string
}

type PipelineArtifactOutput struct {
	Name        string
	Type        string
	MediaType   string
	Description string
}

type EnvVar struct {
	Name  string
	Value string
}

type PipelineResources struct {
	CPU    string
	Memory string
	GPU    int
}

type PipelineRetryPolicy struct {
	MaxAttempts int
	Backoff     PipelineBackoff
}

type PipelineBackoff struct {
	Type           string
	InitialSeconds int
	MaxSeconds     int
	Multiplier     float64
}

// StepNameSet returns the set of step names declared in the spec.
func (p PipelineSpec) StepNameSet() map[string]struct{} {
	names := make(map[string]struct{}, len(p.Spec.Steps))
	for _, step := range p.Spec.Steps {
		if strings.TrimSpace(step.Name) == "" {
			continue
		}
		names[step.Name] = struct{}{}
	}
	return names
}

// DependencyEdges returns the dependency edges as declared.
func (p PipelineSpec) DependencyEdges() []PipelineDependency {
	return p.Spec.Dependencies
}

// ValidateBasicShape performs lightweight structural checks without DAG traversal.
func (p PipelineSpec) ValidateBasicShape() error {
	if strings.TrimSpace(p.APIVersion) == "" {
		return errors.New("apiVersion is required")
	}
	if strings.TrimSpace(p.Kind) == "" {
		return errors.New("kind is required")
	}
	if strings.TrimSpace(p.SpecVersion) == "" {
		return errors.New("specVersion is required")
	}
	if p.Spec.Steps == nil {
		return errors.New("steps are required")
	}
	if len(p.Spec.Steps) == 0 {
		return errors.New("steps must contain at least one step")
	}
	if p.Spec.Dependencies == nil {
		return errors.New("dependencies are required")
	}
	for i, step := range p.Spec.Steps {
		if strings.TrimSpace(step.Name) == "" {
			return fmt.Errorf("step[%d] name is required", i)
		}
		if strings.TrimSpace(step.Image) == "" {
			return fmt.Errorf("step[%d] image is required", i)
		}
		if step.Command == nil {
			return fmt.Errorf("step[%d] command is required", i)
		}
		if step.Args == nil {
			return fmt.Errorf("step[%d] args is required", i)
		}
		if step.Inputs.Datasets == nil {
			return fmt.Errorf("step[%d] inputs.datasets is required", i)
		}
		if step.Inputs.Artifacts == nil {
			return fmt.Errorf("step[%d] inputs.artifacts is required", i)
		}
		if step.Outputs.Artifacts == nil {
			return fmt.Errorf("step[%d] outputs.artifacts is required", i)
		}
		if step.Env == nil {
			return fmt.Errorf("step[%d] env is required", i)
		}
	}
	return nil
}
