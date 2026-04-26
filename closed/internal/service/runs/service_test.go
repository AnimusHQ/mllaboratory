package runs

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/animus-labs/animus-go/closed/internal/domain"
	"github.com/animus-labs/animus-go/closed/internal/execution/plan"
	"github.com/animus-labs/animus-go/closed/internal/platform/auditlog"
	"github.com/animus-labs/animus-go/closed/internal/repo"
)

func TestDeriveAndPersistWithAudit(t *testing.T) {
	runID := "run-1"
	projectID := "proj-1"
	runRepo := newFakeRunRepo(runID, projectID, string(domain.RunStateCreated))
	planRepo := &fakePlanRepo{plans: map[string]repo.PlanRecord{}}
	stepRepo := &fakeStepRepo{}

	execPlan := domain.ExecutionPlan{
		RunID:     runID,
		ProjectID: projectID,
		Steps: []domain.ExecutionPlanStep{
			{Name: "a"},
			{Name: "b"},
		},
	}
	planJSON, err := plan.MarshalExecutionPlan(execPlan)
	if err != nil {
		t.Fatalf("marshal plan: %v", err)
	}
	planRepo.plans[runID] = repo.PlanRecord{RunID: runID, ProjectID: projectID, Plan: planJSON}

	service := New(runRepo, planRepo, stepRepo)
	if service == nil {
		t.Fatalf("expected service")
	}

	appender := &fakeAuditAppender{}
	info := AuditInfo{Actor: "tester", Service: "tests", RequestID: "req-1"}

	_, prev, derived, err := service.DeriveAndPersistWithAudit(context.Background(), appender, info, projectID, runID, "spec-hash")
	if err != nil {
		t.Fatalf("derive: %v", err)
	}
	if prev != domain.RunStateCreated || derived != domain.RunStatePlanned {
		t.Fatalf("unexpected state transition: %s -> %s", prev, derived)
	}
	if runRepo.records[runID].Status != string(domain.RunStatePlanned) {
		t.Fatalf("expected persisted planned status")
	}

	before := len(appender.events)
	_, prev, derived, err = service.DeriveAndPersistWithAudit(context.Background(), appender, info, projectID, runID, "spec-hash")
	if err != nil {
		t.Fatalf("derive repeat: %v", err)
	}
	if prev != domain.RunStatePlanned || derived != domain.RunStatePlanned {
		t.Fatalf("expected planned repeat, got %s -> %s", prev, derived)
	}
	if len(appender.events) != before {
		t.Fatalf("expected no new audit events on repeat derive")
	}

	stepRepo.executions = []repo.StepExecutionRecord{
		{ProjectID: projectID, RunID: runID, StepName: "a", Attempt: 1, Status: "Succeeded"},
		{ProjectID: projectID, RunID: runID, StepName: "b", Attempt: 1, Status: "Succeeded"},
	}
	_, prev, derived, err = service.DeriveAndPersistWithAudit(context.Background(), appender, info, projectID, runID, "spec-hash")
	if err != nil {
		t.Fatalf("derive: %v", err)
	}
	if prev != domain.RunStatePlanned || derived != domain.RunStateDryRunSucceeded {
		t.Fatalf("unexpected state transition: %s -> %s", prev, derived)
	}
	if runRepo.records[runID].Status != string(domain.RunStateDryRunSucceeded) {
		t.Fatalf("expected persisted dryrun_succeeded status")
	}

	before = len(appender.events)
	_, prev, derived, err = service.DeriveAndPersistWithAudit(context.Background(), appender, info, projectID, runID, "spec-hash")
	if err != nil {
		t.Fatalf("derive repeat completed: %v", err)
	}
	if prev != domain.RunStateDryRunSucceeded || derived != domain.RunStateDryRunSucceeded {
		t.Fatalf("expected completed repeat, got %s -> %s", prev, derived)
	}
	if len(appender.events) != before {
		t.Fatalf("expected no new audit events on repeat completed")
	}
}

func TestMarkDryRunRunningRequiresPlan(t *testing.T) {
	runRepo := newFakeRunRepo("run-1", "proj-1", string(domain.RunStateCreated))
	service := New(runRepo, &fakePlanRepo{plans: map[string]repo.PlanRecord{}}, &fakeStepRepo{})
	appender := &fakeAuditAppender{}
	info := AuditInfo{Actor: "tester", Service: "tests", RequestID: "req-1"}
	if _, err := service.MarkDryRunRunningWithAudit(context.Background(), appender, info, "proj-1", "run-1", "spec-hash"); err == nil {
		t.Fatalf("expected error when plan missing")
	}
	if len(appender.events) != 0 {
		t.Fatalf("expected no audit events on rejected transition")
	}
}

func TestStateMachineSuccessWithAudit(t *testing.T) {
	ctx := context.Background()
	runID := "run-1"
	projectID := "proj-1"
	specHash := "spec-hash"
	runRepo := newFakeRunRepo(runID, projectID, string(domain.RunStateCreated))
	planRepo := &fakePlanRepo{plans: map[string]repo.PlanRecord{}}
	stepRepo := &fakeStepRepo{}

	execPlan := domain.ExecutionPlan{
		RunID:     runID,
		ProjectID: projectID,
		Steps: []domain.ExecutionPlanStep{
			{Name: "a"},
			{Name: "b"},
		},
	}
	planJSON, err := plan.MarshalExecutionPlan(execPlan)
	if err != nil {
		t.Fatalf("marshal plan: %v", err)
	}
	planRepo.plans[runID] = repo.PlanRecord{RunID: runID, ProjectID: projectID, Plan: planJSON}

	service := New(runRepo, planRepo, stepRepo)
	if service == nil {
		t.Fatalf("expected service")
	}

	appender := &fakeAuditAppender{}
	info := AuditInfo{Actor: "tester", Service: "tests", RequestID: "req-1"}

	_, _, derived, err := service.DeriveAndPersistWithAudit(ctx, appender, info, projectID, runID, specHash)
	if err != nil {
		t.Fatalf("derive planned: %v", err)
	}
	if derived != domain.RunStatePlanned {
		t.Fatalf("expected planned, got %s", derived)
	}
	assertRunStatePersisted(t, runRepo, runID, derived)

	if _, err := service.MarkDryRunRunningWithAudit(ctx, appender, info, projectID, runID, specHash); err != nil {
		t.Fatalf("mark running: %v", err)
	}
	assertRunStatePersisted(t, runRepo, runID, domain.RunStateDryRunRunning)

	before := len(appender.events)
	if _, err := service.MarkDryRunRunningWithAudit(ctx, appender, info, projectID, runID, specHash); err != nil {
		t.Fatalf("mark running repeat: %v", err)
	}
	if len(appender.events) != before {
		t.Fatalf("expected no new audit events on repeat running transition")
	}

	stepRepo.executions = []repo.StepExecutionRecord{
		{ProjectID: projectID, RunID: runID, StepName: "a", Attempt: 1, Status: "Succeeded"},
		{ProjectID: projectID, RunID: runID, StepName: "b", Attempt: 1, Status: "Succeeded"},
	}
	_, _, derived, err = service.DeriveAndPersistWithAudit(ctx, appender, info, projectID, runID, specHash)
	if err != nil {
		t.Fatalf("derive completed: %v", err)
	}
	if derived != domain.RunStateDryRunSucceeded {
		t.Fatalf("expected dryrun_succeeded, got %s", derived)
	}
	assertRunStatePersisted(t, runRepo, runID, derived)

	if len(appender.events) != 3 {
		t.Fatalf("expected 3 audit events, got %d", len(appender.events))
	}
	assertRunAuditEvent(t, appender.events[0], "run.planned", projectID, runID, specHash, string(domain.RunStateCreated), string(domain.RunStatePlanned))
	assertRunAuditEvent(t, appender.events[1], "dry_run.started", projectID, runID, specHash, string(domain.RunStatePlanned), string(domain.RunStateDryRunRunning))
	assertRunAuditEvent(t, appender.events[2], "dry_run.completed", projectID, runID, specHash, string(domain.RunStateDryRunRunning), string(domain.RunStateDryRunSucceeded))
}

func TestStateMachineFailureWithAudit(t *testing.T) {
	ctx := context.Background()
	runID := "run-fail"
	projectID := "proj-1"
	specHash := "spec-hash"
	runRepo := newFakeRunRepo(runID, projectID, string(domain.RunStateCreated))
	planRepo := &fakePlanRepo{plans: map[string]repo.PlanRecord{}}
	stepRepo := &fakeStepRepo{}

	execPlan := domain.ExecutionPlan{
		RunID:     runID,
		ProjectID: projectID,
		Steps: []domain.ExecutionPlanStep{
			{Name: "a"},
		},
	}
	planJSON, err := plan.MarshalExecutionPlan(execPlan)
	if err != nil {
		t.Fatalf("marshal plan: %v", err)
	}
	planRepo.plans[runID] = repo.PlanRecord{RunID: runID, ProjectID: projectID, Plan: planJSON}

	service := New(runRepo, planRepo, stepRepo)
	appender := &fakeAuditAppender{}
	info := AuditInfo{Actor: "tester", Service: "tests", RequestID: "req-2"}

	if _, _, _, err := service.DeriveAndPersistWithAudit(ctx, appender, info, projectID, runID, specHash); err != nil {
		t.Fatalf("derive planned: %v", err)
	}
	if _, err := service.MarkDryRunRunningWithAudit(ctx, appender, info, projectID, runID, specHash); err != nil {
		t.Fatalf("mark running: %v", err)
	}

	stepRepo.executions = []repo.StepExecutionRecord{
		{ProjectID: projectID, RunID: runID, StepName: "a", Attempt: 1, Status: "Failed"},
	}
	_, _, derived, err := service.DeriveAndPersistWithAudit(ctx, appender, info, projectID, runID, specHash)
	if err != nil {
		t.Fatalf("derive failed: %v", err)
	}
	if derived != domain.RunStateDryRunFailed {
		t.Fatalf("expected dryrun_failed, got %s", derived)
	}
	assertRunStatePersisted(t, runRepo, runID, derived)

	if len(appender.events) != 3 {
		t.Fatalf("expected 3 audit events, got %d", len(appender.events))
	}
	assertRunAuditEvent(t, appender.events[2], "dry_run.failed", projectID, runID, specHash, string(domain.RunStateDryRunRunning), string(domain.RunStateDryRunFailed))
}

func assertRunStatePersisted(t *testing.T, repo *fakeRunRepo, runID string, expected domain.RunState) {
	t.Helper()
	record, ok := repo.records[runID]
	if !ok {
		t.Fatalf("missing run record %s", runID)
	}
	if record.Status != string(expected) {
		t.Fatalf("expected persisted %s, got %s", expected, record.Status)
	}
}

func assertRunAuditEvent(t *testing.T, event auditlog.Event, action, projectID, runID, specHash, from, to string) {
	t.Helper()
	if event.Action != action {
		t.Fatalf("expected action %s, got %s", action, event.Action)
	}
	if event.ResourceType != "run" {
		t.Fatalf("expected resource type run, got %s", event.ResourceType)
	}
	if event.ResourceID != runID {
		t.Fatalf("expected resource id %s, got %s", runID, event.ResourceID)
	}
	payload, ok := event.Payload.(map[string]any)
	if !ok {
		t.Fatalf("expected payload map")
	}
	if payload["project_id"] != projectID {
		t.Fatalf("expected project_id %s, got %v", projectID, payload["project_id"])
	}
	if payload["run_id"] != runID {
		t.Fatalf("expected run_id %s, got %v", runID, payload["run_id"])
	}
	if payload["spec_hash"] != specHash {
		t.Fatalf("expected spec_hash %s, got %v", specHash, payload["spec_hash"])
	}
	if payload["from"] != from {
		t.Fatalf("expected from %s, got %v", from, payload["from"])
	}
	if payload["to"] != to {
		t.Fatalf("expected to %s, got %v", to, payload["to"])
	}
	if payload["actor"] != event.Actor {
		t.Fatalf("expected actor %s, got %v", event.Actor, payload["actor"])
	}
	if payload["request_id"] == "" {
		t.Fatalf("expected request_id in payload")
	}
	expectedKey := fmt.Sprintf("%s:%s:%s:%s", projectID, runID, from, to)
	if payload["idempotency_key"] != expectedKey {
		t.Fatalf("expected idempotency_key %s, got %v", expectedKey, payload["idempotency_key"])
	}
	if occurredAt, ok := payload["occurred_at"].(time.Time); !ok || occurredAt.IsZero() {
		t.Fatalf("expected occurred_at timestamp in payload")
	}
}

type fakeAuditAppender struct {
	events []auditlog.Event
}

func (f *fakeAuditAppender) Append(ctx context.Context, event auditlog.Event) error {
	f.events = append(f.events, event)
	return nil
}

type fakeRunRepo struct {
	records map[string]repo.RunRecord
}

func newFakeRunRepo(runID, projectID, status string) *fakeRunRepo {
	return &fakeRunRepo{
		records: map[string]repo.RunRecord{
			runID: {
				ID:        runID,
				ProjectID: projectID,
				Status:    status,
			},
		},
	}
}

func (f *fakeRunRepo) CreateRun(ctx context.Context, projectID, idempotencyKey string, pipelineSpecJSON, runSpecJSON []byte, specHash string) (repo.RunRecord, bool, error) {
	return repo.RunRecord{}, false, nil
}

func (f *fakeRunRepo) GetRun(ctx context.Context, projectID, id string) (repo.RunRecord, error) {
	record, ok := f.records[id]
	if !ok || record.ProjectID != projectID {
		return repo.RunRecord{}, repo.ErrNotFound
	}
	return record, nil
}

func (f *fakeRunRepo) UpdateDerivedStatus(ctx context.Context, projectID, runID string, status domain.RunState) (domain.RunState, bool, error) {
	record, ok := f.records[runID]
	if !ok || record.ProjectID != projectID {
		return "", false, repo.ErrNotFound
	}
	current := domain.NormalizeRunState(record.Status)
	if current == "" {
		current = domain.RunStateCreated
	}
	if current == status {
		return current, false, nil
	}
	if !domain.CanTransitionRunState(current, status) {
		return current, false, repo.ErrInvalidTransition
	}
	record.Status = string(status)
	f.records[runID] = record
	return current, true, nil
}

type fakePlanRepo struct {
	plans map[string]repo.PlanRecord
}

func (f *fakePlanRepo) UpsertPlan(ctx context.Context, projectID, runID string, planJSON []byte) (repo.PlanRecord, error) {
	return repo.PlanRecord{}, nil
}

func (f *fakePlanRepo) GetPlan(ctx context.Context, projectID, runID string) (repo.PlanRecord, error) {
	record, ok := f.plans[runID]
	if !ok || record.ProjectID != projectID {
		return repo.PlanRecord{}, repo.ErrNotFound
	}
	return record, nil
}

type fakeStepRepo struct {
	executions []repo.StepExecutionRecord
}

func (f *fakeStepRepo) InsertAttempt(ctx context.Context, record repo.StepExecutionRecord) (repo.StepExecutionRecord, bool, error) {
	return repo.StepExecutionRecord{}, false, nil
}

func (f *fakeStepRepo) ListByRun(ctx context.Context, projectID, runID string) ([]repo.StepExecutionRecord, error) {
	out := make([]repo.StepExecutionRecord, 0)
	for _, record := range f.executions {
		if record.ProjectID == projectID && record.RunID == runID {
			out = append(out, record)
		}
	}
	return out, nil
}
