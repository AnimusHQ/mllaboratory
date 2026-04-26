package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/animus-labs/animus-go/closed/internal/repo"
)

type StepExecutionStore struct {
	db DB
}

const (
	insertStepExecutionQuery = `INSERT INTO step_executions (
		step_execution_id,
		project_id,
		run_id,
		step_name,
		attempt,
		status,
		started_at,
		finished_at,
		error_code,
		error_message,
		result,
		spec_hash
	) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
	ON CONFLICT (project_id, run_id, step_name, attempt) DO NOTHING
	RETURNING step_execution_id, project_id, run_id, step_name, attempt, status, started_at, finished_at, error_code, error_message, result, spec_hash`

	selectStepExecutionQuery = `SELECT step_execution_id, project_id, run_id, step_name, attempt, status, started_at, finished_at, error_code, error_message, result, spec_hash
	 FROM step_executions
	 WHERE project_id = $1 AND run_id = $2 AND step_name = $3 AND attempt = $4`

	listStepExecutionsByRunQuery = `SELECT step_execution_id, project_id, run_id, step_name, attempt, status, started_at, finished_at, error_code, error_message, result, spec_hash
	 FROM step_executions
	 WHERE project_id = $1 AND run_id = $2
	 ORDER BY started_at ASC, step_name ASC, attempt ASC`
)

func NewStepExecutionStore(db DB) *StepExecutionStore {
	if db == nil {
		return nil
	}
	return &StepExecutionStore{db: db}
}

func (s *StepExecutionStore) InsertAttempt(ctx context.Context, record repo.StepExecutionRecord) (repo.StepExecutionRecord, bool, error) {
	if s == nil || s.db == nil {
		return repo.StepExecutionRecord{}, false, fmt.Errorf("step execution store not initialized")
	}
	projectID := strings.TrimSpace(record.ProjectID)
	runID := strings.TrimSpace(record.RunID)
	stepName := strings.TrimSpace(record.StepName)
	status := strings.TrimSpace(record.Status)
	specHash := strings.TrimSpace(record.SpecHash)

	if projectID == "" {
		return repo.StepExecutionRecord{}, false, fmt.Errorf("project id is required")
	}
	if runID == "" {
		return repo.StepExecutionRecord{}, false, fmt.Errorf("run id is required")
	}
	if stepName == "" {
		return repo.StepExecutionRecord{}, false, fmt.Errorf("step name is required")
	}
	if record.Attempt < 1 {
		return repo.StepExecutionRecord{}, false, fmt.Errorf("attempt must be >= 1")
	}
	if status == "" {
		return repo.StepExecutionRecord{}, false, fmt.Errorf("status is required")
	}
	if specHash == "" {
		return repo.StepExecutionRecord{}, false, fmt.Errorf("spec hash is required")
	}

	startedAt := record.StartedAt
	if startedAt.IsZero() {
		startedAt = time.Now().UTC()
	}

	var finishedAt sql.NullTime
	if record.FinishedAt != nil && !record.FinishedAt.IsZero() {
		finishedAt = sql.NullTime{Time: record.FinishedAt.UTC(), Valid: true}
	}

	id := record.ID
	if strings.TrimSpace(id) == "" {
		id = uuid.NewString()
	}

	var inserted repo.StepExecutionRecord
	err := s.db.QueryRowContext(
		ctx,
		insertStepExecutionQuery,
		id,
		projectID,
		runID,
		stepName,
		record.Attempt,
		status,
		startedAt,
		finishedAt,
		nullIfEmpty(record.ErrorCode),
		nullIfEmpty(record.ErrorMessage),
		record.Result,
		specHash,
	).Scan(
		&inserted.ID,
		&inserted.ProjectID,
		&inserted.RunID,
		&inserted.StepName,
		&inserted.Attempt,
		&inserted.Status,
		&inserted.StartedAt,
		&finishedAt,
		&inserted.ErrorCode,
		&inserted.ErrorMessage,
		&inserted.Result,
		&inserted.SpecHash,
	)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return repo.StepExecutionRecord{}, false, fmt.Errorf("insert step execution: %w", err)
		}
		existing, err := s.getAttempt(ctx, projectID, runID, stepName, record.Attempt)
		if err != nil {
			return repo.StepExecutionRecord{}, false, err
		}
		return existing, false, nil
	}

	if finishedAt.Valid {
		t := finishedAt.Time.UTC()
		inserted.FinishedAt = &t
	}
	return inserted, true, nil
}

func (s *StepExecutionStore) ListByRun(ctx context.Context, projectID, runID string) ([]repo.StepExecutionRecord, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("step execution store not initialized")
	}
	projectID = strings.TrimSpace(projectID)
	runID = strings.TrimSpace(runID)
	if projectID == "" {
		return nil, fmt.Errorf("project id is required")
	}
	if runID == "" {
		return nil, fmt.Errorf("run id is required")
	}

	rows, err := s.db.QueryContext(ctx, listStepExecutionsByRunQuery, projectID, runID)
	if err != nil {
		return nil, fmt.Errorf("list step executions: %w", err)
	}
	defer rows.Close()

	records := make([]repo.StepExecutionRecord, 0)
	for rows.Next() {
		record, err := scanStepExecution(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list step executions: %w", err)
	}
	return records, nil
}

func (s *StepExecutionStore) getAttempt(ctx context.Context, projectID, runID, stepName string, attempt int) (repo.StepExecutionRecord, error) {
	row := s.db.QueryRowContext(ctx, selectStepExecutionQuery, projectID, runID, stepName, attempt)
	record, err := scanStepExecution(row)
	if err != nil {
		return repo.StepExecutionRecord{}, err
	}
	return record, nil
}

type stepExecutionScanner interface {
	Scan(dest ...any) error
}

func scanStepExecution(scanner stepExecutionScanner) (repo.StepExecutionRecord, error) {
	var record repo.StepExecutionRecord
	var finishedAt sql.NullTime
	var errorCode sql.NullString
	var errorMessage sql.NullString
	if err := scanner.Scan(
		&record.ID,
		&record.ProjectID,
		&record.RunID,
		&record.StepName,
		&record.Attempt,
		&record.Status,
		&record.StartedAt,
		&finishedAt,
		&errorCode,
		&errorMessage,
		&record.Result,
		&record.SpecHash,
	); err != nil {
		return repo.StepExecutionRecord{}, handleNotFound(err)
	}
	if finishedAt.Valid {
		t := finishedAt.Time.UTC()
		record.FinishedAt = &t
	}
	record.ErrorCode = strings.TrimSpace(errorCode.String)
	record.ErrorMessage = strings.TrimSpace(errorMessage.String)
	return record, nil
}
