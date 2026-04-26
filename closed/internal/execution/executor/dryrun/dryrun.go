package dryrun

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/animus-labs/animus-go/closed/internal/domain"
	executor "github.com/animus-labs/animus-go/closed/internal/execution/executor"
	"github.com/animus-labs/animus-go/closed/internal/repo"
)

const (
	StatusSucceeded = "Succeeded"
	StatusFailed    = "Failed"
	StatusRetried   = "Retried"
	StatusSkipped   = "Skipped"
)

type outcomeDecider func(specHash, runID, stepName string, attempt int) float64

type Executor struct {
	repo   repo.StepExecutionRepository
	now    func() time.Time
	decide outcomeDecider
}

func New(repo repo.StepExecutionRepository) *Executor {
	return &Executor{
		repo:   repo,
		now:    time.Now,
		decide: deterministicScore,
	}
}

func (e *Executor) DryRun(ctx context.Context, input executor.DryRunInput) (executor.DryRunResult, error) {
	projectID := strings.TrimSpace(input.ProjectID)
	runID := strings.TrimSpace(input.RunID)
	specHash := strings.TrimSpace(input.SpecHash)
	if projectID == "" {
		return executor.DryRunResult{}, fmt.Errorf("project id is required")
	}
	if runID == "" {
		return executor.DryRunResult{}, fmt.Errorf("run id is required")
	}
	if specHash == "" {
		return executor.DryRunResult{}, fmt.Errorf("spec hash is required")
	}
	if e == nil || e.repo == nil {
		return executor.DryRunResult{}, fmt.Errorf("step execution repository is required")
	}

	records, err := e.repo.ListByRun(ctx, projectID, runID)
	if err != nil {
		return executor.DryRunResult{}, err
	}

	state := buildStepState(records)
	if isComplete(input.Plan, state) {
		return buildResult(input.Plan, records, true), nil
	}

	baseTime := e.now().UTC()
	inserted := make([]repo.StepExecutionRecord, 0)
	finalStatuses := make(map[string]string, len(input.Plan.Steps))
	for name, st := range state {
		if st.TerminalStatus != "" {
			finalStatuses[name] = st.TerminalStatus
		}
	}

	deps := dependencies(input.Plan.Edges)
	seq := 0

	for _, step := range input.Plan.Steps {
		stepName := strings.TrimSpace(step.Name)
		if stepName == "" {
			continue
		}

		st := state[stepName]
		if st.TerminalStatus != "" {
			continue
		}

		if depsFailed(stepName, deps, finalStatuses) {
			attempt := max(1, st.Attempts+1)
			record := newAttemptRecord(projectID, runID, stepName, attempt, StatusSkipped, specHash, baseTime, seq, map[string]any{
				"dry_run": true,
				"reason":  "dependency_failed",
			}, "dependency_failed", "skipped due to failed dependency")
			seq++
			insertedRecord, created, err := e.repo.InsertAttempt(ctx, record)
			if err != nil {
				return executor.DryRunResult{}, err
			}
			if created {
				inserted = append(inserted, insertedRecord)
			}
			finalStatuses[stepName] = StatusSkipped
			continue
		}

		maxAttempts := step.RetryPolicy.MaxAttempts
		if maxAttempts < 1 {
			return executor.DryRunResult{}, fmt.Errorf("invalid maxAttempts for step %q", stepName)
		}
		startAttempt := st.Attempts + 1
		if startAttempt < 1 {
			startAttempt = 1
		}
		if startAttempt > maxAttempts {
			return executor.DryRunResult{}, fmt.Errorf("attempts exceed maxAttempts for step %q", stepName)
		}

		for attempt := startAttempt; attempt <= maxAttempts; attempt++ {
			score := e.decide(specHash, runID, stepName, attempt)
			success := score < 0.8
			status := StatusFailed
			errorCode := ""
			errorMessage := ""
			resultPayload := map[string]any{
				"dry_run": true,
				"attempt": attempt,
				"score":   score,
			}

			if success {
				status = StatusSucceeded
			} else if attempt < maxAttempts {
				status = StatusRetried
				errorCode = "dry_run_retry"
				errorMessage = "simulated failure; retrying"
				backoffSeconds := computeBackoffSeconds(step.RetryPolicy, attempt)
				resultPayload["backoff_seconds"] = backoffSeconds
			} else {
				status = StatusFailed
				errorCode = "dry_run_failed"
				errorMessage = "simulated failure; retries exhausted"
			}

			record := newAttemptRecord(projectID, runID, stepName, attempt, status, specHash, baseTime, seq, resultPayload, errorCode, errorMessage)
			seq++
			insertedRecord, created, err := e.repo.InsertAttempt(ctx, record)
			if err != nil {
				return executor.DryRunResult{}, err
			}
			if created {
				inserted = append(inserted, insertedRecord)
			}

			st.Attempts = attempt
			if status == StatusSucceeded {
				finalStatuses[stepName] = StatusSucceeded
				break
			}
			if status == StatusFailed {
				finalStatuses[stepName] = StatusFailed
				break
			}
		}
	}

	allRecords := append(records, inserted...)
	result := buildResult(input.Plan, allRecords, false)
	result.Attempts = buildAttemptResults(inserted)
	return result, nil
}

type stepState struct {
	Attempts        int
	TerminalStatus  string
	TerminalAttempt int
}

func buildStepState(records []repo.StepExecutionRecord) map[string]stepState {
	state := make(map[string]stepState)
	for _, record := range records {
		stepName := strings.TrimSpace(record.StepName)
		if stepName == "" {
			continue
		}
		st := state[stepName]
		if record.Attempt > st.Attempts {
			st.Attempts = record.Attempt
		}
		if isTerminalStatus(record.Status) && record.Attempt >= st.TerminalAttempt {
			st.TerminalStatus = record.Status
			st.TerminalAttempt = record.Attempt
		}
		state[stepName] = st
	}
	return state
}

func isComplete(plan domain.ExecutionPlan, state map[string]stepState) bool {
	for _, step := range plan.Steps {
		if step.Name == "" {
			continue
		}
		if state[step.Name].TerminalStatus == "" {
			return false
		}
	}
	return true
}

func buildResult(plan domain.ExecutionPlan, records []repo.StepExecutionRecord, existing bool) executor.DryRunResult {
	state := buildStepState(records)
	stepResults := make([]executor.DryRunStepResult, 0, len(plan.Steps))
	overallStatus := StatusSucceeded
	for _, step := range plan.Steps {
		if step.Name == "" {
			continue
		}
		st := state[step.Name]
		status := st.TerminalStatus
		if status == "" {
			status = StatusFailed
		}
		if status != StatusSucceeded {
			overallStatus = StatusFailed
		}
		stepResults = append(stepResults, executor.DryRunStepResult{
			Name:     step.Name,
			Attempts: max(1, st.Attempts),
			Status:   status,
		})
	}
	return executor.DryRunResult{
		Status:   overallStatus,
		Steps:    stepResults,
		Existing: existing,
	}
}

func buildAttemptResults(records []repo.StepExecutionRecord) []executor.DryRunAttempt {
	out := make([]executor.DryRunAttempt, 0, len(records))
	for _, record := range records {
		out = append(out, executor.DryRunAttempt{
			StepName: record.StepName,
			Attempt:  record.Attempt,
			Status:   record.Status,
		})
	}
	return out
}

func isTerminalStatus(status string) bool {
	switch status {
	case StatusSucceeded, StatusFailed, StatusSkipped:
		return true
	default:
		return false
	}
}

func dependencies(edges []domain.ExecutionPlanEdge) map[string][]string {
	out := make(map[string][]string)
	for _, edge := range edges {
		if strings.TrimSpace(edge.From) == "" || strings.TrimSpace(edge.To) == "" {
			continue
		}
		out[edge.To] = append(out[edge.To], edge.From)
	}
	return out
}

func depsFailed(stepName string, deps map[string][]string, statuses map[string]string) bool {
	for _, dep := range deps[stepName] {
		if statuses[dep] != StatusSucceeded {
			return true
		}
	}
	return false
}

func newAttemptRecord(projectID, runID, stepName string, attempt int, status, specHash string, base time.Time, seq int, payload map[string]any, errorCode, errorMessage string) repo.StepExecutionRecord {
	result, _ := json.Marshal(payload)
	startedAt := base.Add(time.Duration(seq) * time.Millisecond)
	finishedAt := startedAt
	return repo.StepExecutionRecord{
		ProjectID:    projectID,
		RunID:        runID,
		StepName:     stepName,
		Attempt:      attempt,
		Status:       status,
		StartedAt:    startedAt,
		FinishedAt:   &finishedAt,
		ErrorCode:    errorCode,
		ErrorMessage: errorMessage,
		Result:       result,
		SpecHash:     specHash,
	}
}

func deterministicScore(specHash, runID, stepName string, attempt int) float64 {
	seed := fmt.Sprintf("%s:%s:%s:%d", specHash, runID, stepName, attempt)
	sum := sha256.Sum256([]byte(seed))
	value := binary.BigEndian.Uint64(sum[:8])
	return float64(value) / float64(math.MaxUint64)
}

func computeBackoffSeconds(policy domain.PipelineRetryPolicy, attempt int) int {
	if attempt < 1 {
		return 0
	}
	initial := policy.Backoff.InitialSeconds
	if initial < 0 {
		initial = 0
	}
	max := policy.Backoff.MaxSeconds
	if max < 0 {
		max = 0
	}

	switch strings.ToLower(policy.Backoff.Type) {
	case "exponential":
		backoff := float64(initial) * math.Pow(policy.Backoff.Multiplier, float64(attempt-1))
		if backoff > float64(max) {
			return max
		}
		return int(backoff)
	default:
		if max > 0 && initial > max {
			return max
		}
		return initial
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
