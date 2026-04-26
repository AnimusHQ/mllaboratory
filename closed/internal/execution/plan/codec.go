package plan

import (
	"encoding/json"

	"github.com/animus-labs/animus-go/closed/internal/domain"
)

// MarshalExecutionPlan serializes an execution plan with stable field names.
func MarshalExecutionPlan(plan domain.ExecutionPlan) ([]byte, error) {
	payload := executionPlanPayload{
		RunID:     plan.RunID,
		ProjectID: plan.ProjectID,
		Steps:     make([]executionPlanStepPayload, 0, len(plan.Steps)),
		Edges:     make([]executionPlanEdgePayload, 0, len(plan.Edges)),
	}
	for _, step := range plan.Steps {
		payload.Steps = append(payload.Steps, executionPlanStepPayload{
			Name:         step.Name,
			RetryPolicy:  retryPolicyPayloadFromDomain(step.RetryPolicy),
			AttemptStart: step.AttemptStart,
		})
	}
	for _, edge := range plan.Edges {
		payload.Edges = append(payload.Edges, executionPlanEdgePayload{
			From: edge.From,
			To:   edge.To,
		})
	}
	return json.Marshal(payload)
}

// UnmarshalExecutionPlan parses a persisted plan JSON into a domain ExecutionPlan.
func UnmarshalExecutionPlan(raw []byte) (domain.ExecutionPlan, error) {
	var payload executionPlanPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return domain.ExecutionPlan{}, err
	}
	steps := make([]domain.ExecutionPlanStep, 0, len(payload.Steps))
	for _, step := range payload.Steps {
		steps = append(steps, domain.ExecutionPlanStep{
			Name: step.Name,
			RetryPolicy: domain.PipelineRetryPolicy{
				MaxAttempts: step.RetryPolicy.MaxAttempts,
				Backoff: domain.PipelineBackoff{
					Type:           step.RetryPolicy.Backoff.Type,
					InitialSeconds: step.RetryPolicy.Backoff.InitialSeconds,
					MaxSeconds:     step.RetryPolicy.Backoff.MaxSeconds,
					Multiplier:     step.RetryPolicy.Backoff.Multiplier,
				},
			},
			AttemptStart: step.AttemptStart,
		})
	}
	edges := make([]domain.ExecutionPlanEdge, 0, len(payload.Edges))
	for _, edge := range payload.Edges {
		edges = append(edges, domain.ExecutionPlanEdge{
			From: edge.From,
			To:   edge.To,
		})
	}
	return domain.ExecutionPlan{
		RunID:     payload.RunID,
		ProjectID: payload.ProjectID,
		Steps:     steps,
		Edges:     edges,
	}, nil
}

type executionPlanPayload struct {
	RunID     string                     `json:"runId"`
	ProjectID string                     `json:"projectId"`
	Steps     []executionPlanStepPayload `json:"steps"`
	Edges     []executionPlanEdgePayload `json:"edges"`
}

type executionPlanStepPayload struct {
	Name         string             `json:"name"`
	RetryPolicy  retryPolicyPayload `json:"retryPolicy"`
	AttemptStart int                `json:"attemptStart"`
}

type executionPlanEdgePayload struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type retryPolicyPayload struct {
	MaxAttempts int            `json:"maxAttempts"`
	Backoff     backoffPayload `json:"backoff"`
}

type backoffPayload struct {
	Type           string  `json:"type"`
	InitialSeconds int     `json:"initialSeconds"`
	MaxSeconds     int     `json:"maxSeconds"`
	Multiplier     float64 `json:"multiplier"`
}

func retryPolicyPayloadFromDomain(policy domain.PipelineRetryPolicy) retryPolicyPayload {
	return retryPolicyPayload{
		MaxAttempts: policy.MaxAttempts,
		Backoff: backoffPayload{
			Type:           policy.Backoff.Type,
			InitialSeconds: policy.Backoff.InitialSeconds,
			MaxSeconds:     policy.Backoff.MaxSeconds,
			Multiplier:     policy.Backoff.Multiplier,
		},
	}
}
