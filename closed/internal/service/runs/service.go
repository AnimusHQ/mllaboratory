package runs

import (
	"context"
	"errors"
	"net"
	"strings"
	"time"

	"github.com/animus-labs/animus-go/closed/internal/domain"
	"github.com/animus-labs/animus-go/closed/internal/execution/plan"
	"github.com/animus-labs/animus-go/closed/internal/execution/state"
	"github.com/animus-labs/animus-go/closed/internal/platform/auditlog"
	"github.com/animus-labs/animus-go/closed/internal/repo"
)

type Service struct {
	runs  repo.RunRepository
	plans repo.PlanRepository
	steps repo.StepExecutionRepository
}

type AuditAppender interface {
	Append(ctx context.Context, event auditlog.Event) error
}

type AuditInfo struct {
	Actor     string
	RequestID string
	UserAgent string
	IP        net.IP
	Service   string
}

func New(runRepo repo.RunRepository, planRepo repo.PlanRepository, stepRepo repo.StepExecutionRepository) *Service {
	if runRepo == nil || planRepo == nil || stepRepo == nil {
		return nil
	}
	return &Service{
		runs:  runRepo,
		plans: planRepo,
		steps: stepRepo,
	}
}

type auditAppenderFunc func(ctx context.Context, event auditlog.Event) error

func (fn auditAppenderFunc) Append(ctx context.Context, event auditlog.Event) error {
	return fn(ctx, event)
}

// NewAuditAppender adapts an auditlog.QueryRower into an AuditAppender.
func NewAuditAppender(q auditlog.QueryRower) AuditAppender {
	if q == nil {
		return nil
	}
	return auditAppenderFunc(func(ctx context.Context, event auditlog.Event) error {
		_, err := auditlog.Insert(ctx, q, event)
		return err
	})
}

// Derive computes the derived run state without mutating persisted status.
func (s *Service) Derive(ctx context.Context, projectID, runID string) (repo.RunRecord, domain.RunState, error) {
	runRecord, err := s.runs.GetRun(ctx, projectID, runID)
	if err != nil {
		return repo.RunRecord{}, "", err
	}
	planSpec, err := s.loadPlan(ctx, projectID, runID)
	if err != nil {
		return repo.RunRecord{}, "", err
	}
	stepExecutions, err := s.steps.ListByRun(ctx, projectID, runID)
	if err != nil {
		return repo.RunRecord{}, "", err
	}
	expectedSteps := planStepNames(planSpec)
	outcomes, _ := state.DeriveStepOutcomes(stepExecutions, expectedSteps)
	derived := state.DeriveRunState(planSpec != nil, outcomes, expectedSteps)
	return runRecord, derived, nil
}

// deriveAndPersist computes the derived run state and persists it.
func (s *Service) deriveAndPersist(ctx context.Context, projectID, runID string) (repo.RunRecord, domain.RunState, domain.RunState, bool, error) {
	runRecord, err := s.runs.GetRun(ctx, projectID, runID)
	if err != nil {
		return repo.RunRecord{}, "", "", false, err
	}
	planSpec, err := s.loadPlan(ctx, projectID, runID)
	if err != nil {
		return repo.RunRecord{}, "", "", false, err
	}
	stepExecutions, err := s.steps.ListByRun(ctx, projectID, runID)
	if err != nil {
		return repo.RunRecord{}, "", "", false, err
	}
	expectedSteps := planStepNames(planSpec)
	outcomes, _ := state.DeriveStepOutcomes(stepExecutions, expectedSteps)
	derived := state.DeriveRunState(planSpec != nil, outcomes, expectedSteps)
	prev, applied, err := s.runs.UpdateDerivedStatus(ctx, projectID, runID, derived)
	if err != nil {
		return repo.RunRecord{}, "", "", false, err
	}
	return runRecord, prev, derived, applied, nil
}

// DeriveAndPersistWithAudit computes and persists derived state, then emits a run-level audit event.
func (s *Service) DeriveAndPersistWithAudit(ctx context.Context, appender AuditAppender, info AuditInfo, projectID, runID, specHash string) (repo.RunRecord, domain.RunState, domain.RunState, error) {
	runRecord, prev, derived, applied, err := s.deriveAndPersist(ctx, projectID, runID)
	if err != nil {
		return repo.RunRecord{}, "", "", err
	}
	if applied {
		if err := s.AppendRunTransitionAudit(ctx, appender, info, projectID, runID, specHash, prev, derived); err != nil {
			return repo.RunRecord{}, "", "", err
		}
	}
	return runRecord, prev, derived, nil
}

// markDryRunRunning transitions a run to dryrun_running after verifying a plan exists.
func (s *Service) markDryRunRunning(ctx context.Context, projectID, runID string) (domain.RunState, bool, error) {
	planSpec, err := s.loadPlan(ctx, projectID, runID)
	if err != nil {
		return "", false, err
	}
	if planSpec == nil {
		return "", false, errors.New("plan required for dry-run")
	}
	prev, applied, err := s.runs.UpdateDerivedStatus(ctx, projectID, runID, domain.RunStateDryRunRunning)
	if err != nil {
		return "", false, err
	}
	return prev, applied, nil
}

// MarkDryRunRunningWithAudit transitions to dryrun_running and emits a run-level audit event.
func (s *Service) MarkDryRunRunningWithAudit(ctx context.Context, appender AuditAppender, info AuditInfo, projectID, runID, specHash string) (domain.RunState, error) {
	prev, applied, err := s.markDryRunRunning(ctx, projectID, runID)
	if err != nil {
		return "", err
	}
	if applied {
		if err := s.AppendRunTransitionAudit(ctx, appender, info, projectID, runID, specHash, prev, domain.RunStateDryRunRunning); err != nil {
			return "", err
		}
	}
	return prev, nil
}

// AppendRunTransitionAudit emits a run-level audit event for a successful transition.
func (s *Service) AppendRunTransitionAudit(ctx context.Context, appender AuditAppender, info AuditInfo, projectID, runID, specHash string, from, to domain.RunState) error {
	if appender == nil {
		return errors.New("audit appender is required")
	}
	event, ok, err := BuildRunTransitionEvent(info, projectID, runID, specHash, from, to)
	if err != nil || !ok {
		return err
	}
	return appender.Append(ctx, *event)
}

// BuildRunTransitionEvent returns a run-level audit event for a transition.
// It returns ok=false when no event should be emitted.
func BuildRunTransitionEvent(info AuditInfo, projectID, runID, specHash string, from, to domain.RunState) (*auditlog.Event, bool, error) {
	if strings.TrimSpace(info.Actor) == "" {
		return nil, false, errors.New("audit actor is required")
	}
	if from == to {
		return nil, false, nil
	}
	action := transitionAction(to)
	if action == "" {
		return nil, false, nil
	}
	occurredAt := time.Now().UTC()
	idempotencyKey := transitionIdempotencyKey(projectID, runID, from, to)
	requestID := strings.TrimSpace(info.RequestID)
	if requestID == "" {
		requestID = idempotencyKey
	}
	event := auditlog.Event{
		OccurredAt:   occurredAt,
		Actor:        info.Actor,
		Action:       action,
		ResourceType: "run",
		ResourceID:   runID,
		RequestID:    requestID,
		IP:           info.IP,
		UserAgent:    info.UserAgent,
		Payload: map[string]any{
			"service":         strings.TrimSpace(info.Service),
			"project_id":      projectID,
			"run_id":          runID,
			"spec_hash":       specHash,
			"from":            string(from),
			"to":              string(to),
			"state":           string(to),
			"transition":      string(from) + "->" + string(to),
			"actor":           strings.TrimSpace(info.Actor),
			"occurred_at":     occurredAt,
			"request_id":      requestID,
			"idempotency_key": idempotencyKey,
		},
	}
	return &event, true, nil
}

func (s *Service) loadPlan(ctx context.Context, projectID, runID string) (*domain.ExecutionPlan, error) {
	planRecord, err := s.plans.GetPlan(ctx, projectID, runID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}
	parsed, err := plan.UnmarshalExecutionPlan(planRecord.Plan)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

func transitionAction(to domain.RunState) string {
	switch to {
	case domain.RunStatePlanned:
		return "run.planned"
	case domain.RunStateRunning:
		return "run.state_changed"
	case domain.RunStateSucceeded:
		return "run.state_changed"
	case domain.RunStateFailed:
		return "run.state_changed"
	case domain.RunStateCanceled:
		return "run.state_changed"
	case domain.RunStateDryRunRunning:
		return "dry_run.started"
	case domain.RunStateDryRunSucceeded:
		return "dry_run.completed"
	case domain.RunStateDryRunFailed:
		return "dry_run.failed"
	default:
		return ""
	}
}

func transitionIdempotencyKey(projectID, runID string, from, to domain.RunState) string {
	return strings.Join([]string{
		strings.TrimSpace(projectID),
		strings.TrimSpace(runID),
		string(from),
		string(to),
	}, ":")
}

func planStepNames(plan *domain.ExecutionPlan) []string {
	if plan == nil || len(plan.Steps) == 0 {
		return nil
	}
	names := make([]string, 0, len(plan.Steps))
	for _, step := range plan.Steps {
		name := strings.TrimSpace(step.Name)
		if name == "" {
			continue
		}
		names = append(names, name)
	}
	return names
}
