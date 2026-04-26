package domain

// ExecutionPlan is a deterministic plan derived from a PipelineSpec.
type ExecutionPlan struct {
	RunID     string
	ProjectID string
	Steps     []ExecutionPlanStep
	Edges     []ExecutionPlanEdge
}

type ExecutionPlanStep struct {
	Name         string
	RetryPolicy  PipelineRetryPolicy
	AttemptStart int
}

type ExecutionPlanEdge struct {
	From string
	To   string
}
