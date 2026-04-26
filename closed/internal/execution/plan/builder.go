package plan

import (
	"fmt"
	"sort"
	"strings"

	"github.com/animus-labs/animus-go/closed/internal/domain"
	"github.com/animus-labs/animus-go/closed/internal/execution/specvalidator"
)

// BuildPlan generates a deterministic execution plan from a PipelineSpec.
func BuildPlan(spec domain.PipelineSpec, runID, projectID string) (domain.ExecutionPlan, error) {
	runID = strings.TrimSpace(runID)
	projectID = strings.TrimSpace(projectID)
	if runID == "" {
		return domain.ExecutionPlan{}, fmt.Errorf("run id is required")
	}
	if projectID == "" {
		return domain.ExecutionPlan{}, fmt.Errorf("project id is required")
	}

	if err := specvalidator.ValidatePipelineSpec(spec); err != nil {
		return domain.ExecutionPlan{}, err
	}

	ordered, err := topoSortSteps(spec)
	if err != nil {
		return domain.ExecutionPlan{}, err
	}

	steps := make([]domain.ExecutionPlanStep, 0, len(ordered))
	for _, step := range ordered {
		steps = append(steps, domain.ExecutionPlanStep{
			Name:         step.Name,
			RetryPolicy:  step.RetryPolicy,
			AttemptStart: 1,
		})
	}

	edges := make([]domain.ExecutionPlanEdge, 0, len(spec.Spec.Dependencies))
	for _, dep := range spec.Spec.Dependencies {
		edges = append(edges, domain.ExecutionPlanEdge{
			From: dep.From,
			To:   dep.To,
		})
	}

	return domain.ExecutionPlan{
		RunID:     runID,
		ProjectID: projectID,
		Steps:     steps,
		Edges:     edges,
	}, nil
}

func topoSortSteps(spec domain.PipelineSpec) ([]domain.PipelineStep, error) {
	stepMap := make(map[string]domain.PipelineStep, len(spec.Spec.Steps))
	for _, step := range spec.Spec.Steps {
		stepMap[step.Name] = step
	}

	inDegree := make(map[string]int, len(stepMap))
	adj := make(map[string][]string, len(stepMap))
	for name := range stepMap {
		inDegree[name] = 0
	}
	for _, dep := range spec.Spec.Dependencies {
		from := dep.From
		to := dep.To
		adj[from] = append(adj[from], to)
		inDegree[to]++
	}

	ready := make([]string, 0, len(stepMap))
	for name, degree := range inDegree {
		if degree == 0 {
			ready = append(ready, name)
		}
	}
	sort.Strings(ready)

	ordered := make([]domain.PipelineStep, 0, len(stepMap))
	for len(ready) > 0 {
		name := ready[0]
		ready = ready[1:]
		ordered = append(ordered, stepMap[name])
		for _, neighbor := range adj[name] {
			inDegree[neighbor]--
			if inDegree[neighbor] == 0 {
				ready = append(ready, neighbor)
				sort.Strings(ready)
			}
		}
	}

	if len(ordered) != len(stepMap) {
		return nil, fmt.Errorf("dependency graph contains a cycle")
	}
	return ordered, nil
}
