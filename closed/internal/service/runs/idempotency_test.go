package runs

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/animus-labs/animus-go/closed/internal/domain"
	"github.com/animus-labs/animus-go/closed/internal/repo"
	"github.com/animus-labs/animus-go/closed/internal/testsupport"
)

type memoryRunRepo struct {
	mu       sync.Mutex
	seq      int
	byID     map[string]repo.RunRecord
	byKey    map[string]string
	injector *testsupport.ErrorInjector
}

func newMemoryRunRepo(injector *testsupport.ErrorInjector) *memoryRunRepo {
	return &memoryRunRepo{
		byID:     map[string]repo.RunRecord{},
		byKey:    map[string]string{},
		injector: injector,
	}
}

func (m *memoryRunRepo) CreateRun(ctx context.Context, projectID, idempotencyKey string, pipelineSpecJSON, runSpecJSON []byte, specHash string) (repo.RunRecord, bool, error) {
	if err := m.injector.Check("create_run"); err != nil {
		return repo.RunRecord{}, false, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	key := projectID + ":" + idempotencyKey
	if runID, ok := m.byKey[key]; ok {
		return m.byID[runID], false, nil
	}
	m.seq++
	runID := fmt.Sprintf("run-%d", m.seq)
	record := repo.RunRecord{
		ID:             runID,
		ProjectID:      projectID,
		IdempotencyKey: idempotencyKey,
		Status:         string(domain.RunStateCreated),
		PipelineSpec:   pipelineSpecJSON,
		RunSpec:        runSpecJSON,
		SpecHash:       specHash,
	}
	m.byID[runID] = record
	m.byKey[key] = runID
	return record, true, nil
}

func (m *memoryRunRepo) GetRun(ctx context.Context, projectID, id string) (repo.RunRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	record, ok := m.byID[id]
	if !ok || record.ProjectID != projectID {
		return repo.RunRecord{}, repo.ErrNotFound
	}
	return record, nil
}

func (m *memoryRunRepo) UpdateDerivedStatus(ctx context.Context, projectID, runID string, status domain.RunState) (domain.RunState, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	record, ok := m.byID[runID]
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
	m.byID[runID] = record
	return current, true, nil
}

func TestCreateRunIdempotencyKeyReturnsSameRun(t *testing.T) {
	repo := newMemoryRunRepo(nil)
	pipelineSpec := []byte(`{"kind":"Pipeline"}`)
	runSpec := []byte(`{"envLock":{"lockId":"lock-1"}}`)
	first, created, err := repo.CreateRun(context.Background(), "proj-1", "idem-1", pipelineSpec, runSpec, "spec-hash")
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	if !created {
		t.Fatalf("expected created on first insert")
	}
	second, created, err := repo.CreateRun(context.Background(), "proj-1", "idem-1", pipelineSpec, runSpec, "spec-hash")
	if err != nil {
		t.Fatalf("create run retry: %v", err)
	}
	if created {
		t.Fatalf("expected idempotent insert to return created=false")
	}
	if first.ID != second.ID {
		t.Fatalf("expected same run_id, got %s vs %s", first.ID, second.ID)
	}
	if len(repo.byID) != 1 {
		t.Fatalf("expected 1 run record, got %d", len(repo.byID))
	}
}

func TestCreateRunRetryAfterTransientFailure(t *testing.T) {
	injector := testsupport.NewErrorInjector(context.DeadlineExceeded)
	injector.FailOn("create_run", 1)
	repo := newMemoryRunRepo(injector)
	pipelineSpec := []byte(`{"kind":"Pipeline"}`)
	runSpec := []byte(`{"envLock":{"lockId":"lock-1"}}`)
	if _, _, err := repo.CreateRun(context.Background(), "proj-1", "idem-1", pipelineSpec, runSpec, "spec-hash"); err == nil {
		t.Fatalf("expected transient error on first create")
	}
	if len(repo.byID) != 0 {
		t.Fatalf("expected no records after failed create")
	}
	record, created, err := repo.CreateRun(context.Background(), "proj-1", "idem-1", pipelineSpec, runSpec, "spec-hash")
	if err != nil {
		t.Fatalf("retry create: %v", err)
	}
	if !created {
		t.Fatalf("expected created on retry")
	}
	if record.ID == "" {
		t.Fatalf("expected run_id on retry")
	}
	if len(repo.byID) != 1 {
		t.Fatalf("expected 1 run record after retry, got %d", len(repo.byID))
	}
}

func TestRunStateDuplicateTerminalIsNoOp(t *testing.T) {
	runRepo := newFakeRunRepo("run-1", "proj-1", string(domain.RunStateFailed))
	prev, applied, err := runRepo.UpdateDerivedStatus(context.Background(), "proj-1", "run-1", domain.RunStateFailed)
	if err != nil {
		t.Fatalf("duplicate terminal: %v", err)
	}
	if applied {
		t.Fatalf("expected no-op on duplicate terminal transition")
	}
	if prev != domain.RunStateFailed {
		t.Fatalf("expected prev failed, got %s", prev)
	}
	if got := runRepo.records["run-1"].Status; got != string(domain.RunStateFailed) {
		t.Fatalf("expected failed status preserved, got %s", got)
	}
}

func TestRunStateRejectsOutOfOrderTransition(t *testing.T) {
	runRepo := newFakeRunRepo("run-1", "proj-1", string(domain.RunStateRunning))
	if _, applied, err := runRepo.UpdateDerivedStatus(context.Background(), "proj-1", "run-1", domain.RunStateFailed); err != nil || !applied {
		t.Fatalf("expected running->failed transition to apply, err=%v applied=%v", err, applied)
	}
	_, applied, err := runRepo.UpdateDerivedStatus(context.Background(), "proj-1", "run-1", domain.RunStateRunning)
	if err == nil || !errors.Is(err, repo.ErrInvalidTransition) {
		t.Fatalf("expected invalid transition error, got %v", err)
	}
	if applied {
		t.Fatalf("expected no apply on out-of-order transition")
	}
	if got := runRepo.records["run-1"].Status; got != string(domain.RunStateFailed) {
		t.Fatalf("expected failed status preserved, got %s", got)
	}
}
