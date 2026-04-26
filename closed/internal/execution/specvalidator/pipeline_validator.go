package specvalidator

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/animus-labs/animus-go/closed/internal/domain"
)

var digestImageRef = regexp.MustCompile(`^.+@sha256:[a-f0-9]{64}$`)

// ValidatePipelineSpec performs strict validation of a PipelineSpec.
func ValidatePipelineSpec(spec domain.PipelineSpec) error {
	issues := &ValidationError{}

	if err := spec.ValidateBasicShape(); err != nil {
		issues.Add(err.Error())
	}

	if spec.Spec.Steps == nil || len(spec.Spec.Steps) == 0 {
		return issues.OrNil()
	}

	stepNames := make(map[string]struct{}, len(spec.Spec.Steps))
	for i, step := range spec.Spec.Steps {
		name := strings.TrimSpace(step.Name)
		if name == "" {
			issues.Add(fmt.Sprintf("step[%d] name is required", i))
			continue
		}
		if _, exists := stepNames[name]; exists {
			issues.Add(fmt.Sprintf("duplicate step name %q", name))
		}
		stepNames[name] = struct{}{}

		if strings.TrimSpace(step.Image) == "" {
			issues.Add(fmt.Sprintf("step[%s] image is required", name))
		} else if !digestImageRef.MatchString(step.Image) {
			issues.Add(fmt.Sprintf("step[%s] image must be digest-pinned", name))
		}

		if step.Command == nil {
			issues.Add(fmt.Sprintf("step[%s] command is required", name))
		}
		if step.Args == nil {
			issues.Add(fmt.Sprintf("step[%s] args is required", name))
		}
		if step.Inputs.Datasets == nil {
			issues.Add(fmt.Sprintf("step[%s] inputs.datasets is required", name))
		}
		if step.Inputs.Artifacts == nil {
			issues.Add(fmt.Sprintf("step[%s] inputs.artifacts is required", name))
		}
		if step.Outputs.Artifacts == nil {
			issues.Add(fmt.Sprintf("step[%s] outputs.artifacts is required", name))
		}
		if step.Env == nil {
			issues.Add(fmt.Sprintf("step[%s] env is required", name))
		}
	}

	if spec.Spec.Dependencies == nil {
		return issues.OrNil()
	}

	adj := make(map[string][]string, len(stepNames))
	for _, dep := range spec.Spec.Dependencies {
		from := strings.TrimSpace(dep.From)
		to := strings.TrimSpace(dep.To)
		if from == "" || to == "" {
			issues.Add("dependency edges must specify from and to")
			continue
		}
		if from == to {
			issues.Add(fmt.Sprintf("dependency %q has self-edge", from))
			continue
		}
		if _, ok := stepNames[from]; !ok {
			issues.Add(fmt.Sprintf("dependency from %q not found", from))
			continue
		}
		if _, ok := stepNames[to]; !ok {
			issues.Add(fmt.Sprintf("dependency to %q not found", to))
			continue
		}
		adj[from] = append(adj[from], to)
	}

	if hasCycle(adj, stepNames) {
		issues.Add("dependency graph contains a cycle")
	}

	return issues.OrNil()
}

func hasCycle(adj map[string][]string, nodes map[string]struct{}) bool {
	const (
		unvisited = 0
		visiting  = 1
		done      = 2
	)
	state := make(map[string]int, len(nodes))
	var visit func(string) bool
	visit = func(node string) bool {
		switch state[node] {
		case visiting:
			return true
		case done:
			return false
		}
		state[node] = visiting
		for _, next := range adj[node] {
			if visit(next) {
				return true
			}
		}
		state[node] = done
		return false
	}

	for node := range nodes {
		if state[node] == unvisited {
			if visit(node) {
				return true
			}
		}
	}
	return false
}
