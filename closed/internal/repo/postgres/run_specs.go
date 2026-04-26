package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/animus-labs/animus-go/closed/internal/domain"
	"github.com/animus-labs/animus-go/closed/internal/repo"
)

type RunSpecStore struct {
	db DB
}

const (
	insertRunSpecQuery = `INSERT INTO runs (
		run_id,
		project_id,
		idempotency_key,
		status,
		pipeline_spec,
		run_spec,
		spec_hash
	) VALUES ($1,$2,$3,$4,$5,$6,$7)
	ON CONFLICT (project_id, idempotency_key) DO NOTHING
	RETURNING run_id, project_id, idempotency_key, status, pipeline_spec, run_spec, spec_hash, created_at`

	selectRunByIDQuery = `SELECT run_id, project_id, idempotency_key, status, pipeline_spec, run_spec, spec_hash, created_at
	 FROM runs
	 WHERE project_id = $1 AND run_id = $2`

	selectRunByIdempotencyQuery = `SELECT run_id, project_id, idempotency_key, status, pipeline_spec, run_spec, spec_hash, created_at
	 FROM runs
	 WHERE project_id = $1 AND idempotency_key = $2`

	selectRunStatusQuery = `SELECT status
	 FROM runs
	 WHERE project_id = $1 AND run_id = $2`

	selectRunStatusForUpdateQuery = `SELECT status
	 FROM runs
	 WHERE project_id = $1 AND run_id = $2
	 FOR UPDATE`

	updateRunStatusQuery = `UPDATE runs SET status = $1 WHERE project_id = $2 AND run_id = $3`
)

func NewRunSpecStore(db DB) *RunSpecStore {
	if db == nil {
		return nil
	}
	return &RunSpecStore{db: db}
}

func (s *RunSpecStore) CreateRun(ctx context.Context, projectID, idempotencyKey string, pipelineSpecJSON, runSpecJSON []byte, specHash string) (repo.RunRecord, bool, error) {
	if s == nil || s.db == nil {
		return repo.RunRecord{}, false, fmt.Errorf("run spec store not initialized")
	}
	projectID = strings.TrimSpace(projectID)
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	specHash = strings.TrimSpace(specHash)
	if projectID == "" {
		return repo.RunRecord{}, false, fmt.Errorf("project id is required")
	}
	if idempotencyKey == "" {
		return repo.RunRecord{}, false, fmt.Errorf("idempotency key is required")
	}
	if len(pipelineSpecJSON) == 0 {
		return repo.RunRecord{}, false, fmt.Errorf("pipeline spec is required")
	}
	if len(runSpecJSON) == 0 {
		return repo.RunRecord{}, false, fmt.Errorf("run spec is required")
	}
	if specHash == "" {
		return repo.RunRecord{}, false, fmt.Errorf("spec hash is required")
	}

	runID := uuid.NewString()
	status := string(domain.RunStateCreated)

	var record repo.RunRecord
	err := s.db.QueryRowContext(
		ctx,
		insertRunSpecQuery,
		runID,
		projectID,
		idempotencyKey,
		status,
		pipelineSpecJSON,
		runSpecJSON,
		specHash,
	).Scan(&record.ID, &record.ProjectID, &record.IdempotencyKey, &record.Status, &record.PipelineSpec, &record.RunSpec, &record.SpecHash, &record.CreatedAt)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return repo.RunRecord{}, false, fmt.Errorf("insert run: %w", err)
		}
		existing, err := s.GetRunByIdempotencyKey(ctx, projectID, idempotencyKey)
		if err != nil {
			return repo.RunRecord{}, false, err
		}
		return existing, false, nil
	}
	return record, true, nil
}

func (s *RunSpecStore) GetRun(ctx context.Context, projectID, id string) (repo.RunRecord, error) {
	if s == nil || s.db == nil {
		return repo.RunRecord{}, fmt.Errorf("run spec store not initialized")
	}
	projectID = strings.TrimSpace(projectID)
	id = strings.TrimSpace(id)
	if projectID == "" {
		return repo.RunRecord{}, fmt.Errorf("project id is required")
	}
	if id == "" {
		return repo.RunRecord{}, fmt.Errorf("run id is required")
	}
	var record repo.RunRecord
	row := s.db.QueryRowContext(
		ctx,
		selectRunByIDQuery,
		projectID,
		id,
	)
	if err := row.Scan(&record.ID, &record.ProjectID, &record.IdempotencyKey, &record.Status, &record.PipelineSpec, &record.RunSpec, &record.SpecHash, &record.CreatedAt); err != nil {
		return repo.RunRecord{}, handleNotFound(err)
	}
	return record, nil
}

func (s *RunSpecStore) UpdateDerivedStatus(ctx context.Context, projectID, runID string, status domain.RunState) (domain.RunState, bool, error) {
	if s == nil || s.db == nil {
		return "", false, fmt.Errorf("run spec store not initialized")
	}
	projectID = strings.TrimSpace(projectID)
	runID = strings.TrimSpace(runID)
	if projectID == "" {
		return "", false, fmt.Errorf("project id is required")
	}
	if runID == "" {
		return "", false, fmt.Errorf("run id is required")
	}
	next := domain.NormalizeRunState(string(status))
	if next == "" {
		return "", false, fmt.Errorf("status is required")
	}

	var currentRaw string
	row := s.db.QueryRowContext(ctx, selectRunStatusForUpdateQuery, projectID, runID)
	if err := row.Scan(&currentRaw); err != nil {
		return "", false, handleNotFound(err)
	}
	current := domain.NormalizeRunState(currentRaw)
	if current == "" {
		current = domain.RunStateCreated
	}
	if current == next {
		return current, false, nil
	}
	if !domain.CanTransitionRunState(current, next) {
		return current, false, repo.ErrInvalidTransition
	}
	res, err := s.db.ExecContext(ctx, updateRunStatusQuery, string(next), projectID, runID)
	if err != nil {
		return current, false, fmt.Errorf("update run status: %w", err)
	}
	if rows, err := res.RowsAffected(); err == nil && rows == 0 {
		return current, false, repo.ErrNotFound
	}
	return current, true, nil
}

func (s *RunSpecStore) GetRunByIdempotencyKey(ctx context.Context, projectID, idempotencyKey string) (repo.RunRecord, error) {
	if s == nil || s.db == nil {
		return repo.RunRecord{}, fmt.Errorf("run spec store not initialized")
	}
	projectID = strings.TrimSpace(projectID)
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if projectID == "" {
		return repo.RunRecord{}, fmt.Errorf("project id is required")
	}
	if idempotencyKey == "" {
		return repo.RunRecord{}, fmt.Errorf("idempotency key is required")
	}
	var record repo.RunRecord
	row := s.db.QueryRowContext(
		ctx,
		selectRunByIdempotencyQuery,
		projectID,
		idempotencyKey,
	)
	if err := row.Scan(&record.ID, &record.ProjectID, &record.IdempotencyKey, &record.Status, &record.PipelineSpec, &record.RunSpec, &record.SpecHash, &record.CreatedAt); err != nil {
		return repo.RunRecord{}, handleNotFound(err)
	}
	return record, nil
}
