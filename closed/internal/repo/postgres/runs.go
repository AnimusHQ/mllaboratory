package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/animus-labs/animus-go/closed/internal/domain"
	"github.com/animus-labs/animus-go/closed/internal/repo"
)

type RunStore struct {
	db DB
}

func NewRunStore(db DB) *RunStore {
	if db == nil {
		return nil
	}
	return &RunStore{db: db}
}

func (s *RunStore) CreateRun(ctx context.Context, run domain.Run) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("run store not initialized")
	}
	if err := run.Validate(); err != nil {
		return err
	}
	if err := requireIntegrity(run.IntegritySHA256); err != nil {
		return err
	}
	paramsJSON, err := encodeMetadata(run.Params)
	if err != nil {
		return fmt.Errorf("encode params: %w", err)
	}
	metricsJSON, err := encodeMetadata(run.Metrics)
	if err != nil {
		return fmt.Errorf("encode metrics: %w", err)
	}
	startedAt := normalizeTime(run.StartedAt)
	var endedAt sql.NullTime
	if run.EndedAt != nil {
		endedAt = sql.NullTime{Time: run.EndedAt.UTC(), Valid: true}
	}
	_, err = s.db.ExecContext(
		ctx,
		`INSERT INTO experiment_runs (
			run_id,
			experiment_id,
			project_id,
			dataset_version_id,
			status,
			started_at,
			ended_at,
			git_repo,
			git_commit,
			git_ref,
			params,
			metrics,
			artifacts_prefix,
			integrity_sha256
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`,
		strings.TrimSpace(run.ID),
		strings.TrimSpace(run.ExperimentID),
		strings.TrimSpace(run.ProjectID),
		nullIfEmpty(run.DatasetVersionID),
		strings.TrimSpace(run.Status),
		startedAt,
		endedAt,
		nullIfEmpty(run.GitRepo),
		nullIfEmpty(run.GitCommit),
		nullIfEmpty(run.GitRef),
		paramsJSON,
		metricsJSON,
		nullIfEmpty(run.ArtifactsPrefix),
		strings.TrimSpace(run.IntegritySHA256),
	)
	if err != nil {
		return fmt.Errorf("insert run: %w", err)
	}
	return nil
}

func (s *RunStore) GetRun(ctx context.Context, projectID, id string) (domain.Run, error) {
	if s == nil || s.db == nil {
		return domain.Run{}, fmt.Errorf("run store not initialized")
	}
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return domain.Run{}, fmt.Errorf("project id is required")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return domain.Run{}, fmt.Errorf("run id is required")
	}
	var run domain.Run
	var paramsJSON []byte
	var metricsJSON []byte
	var datasetVersionID sql.NullString
	var gitRepo sql.NullString
	var gitCommit sql.NullString
	var gitRef sql.NullString
	var artifactsPrefix sql.NullString
	var endedAt sql.NullTime
	row := s.db.QueryRowContext(
		ctx,
		`SELECT run_id, experiment_id, project_id, dataset_version_id, status, started_at, ended_at,
			git_repo, git_commit, git_ref, params, metrics, artifacts_prefix, integrity_sha256
		 FROM experiment_runs
		 WHERE project_id = $1 AND run_id = $2`,
		projectID,
		id,
	)
	if err := row.Scan(&run.ID, &run.ExperimentID, &run.ProjectID, &datasetVersionID, &run.Status, &run.StartedAt, &endedAt,
		&gitRepo, &gitCommit, &gitRef, &paramsJSON, &metricsJSON, &artifactsPrefix, &run.IntegritySHA256); err != nil {
		return domain.Run{}, handleNotFound(err)
	}
	if datasetVersionID.Valid {
		run.DatasetVersionID = datasetVersionID.String
	}
	if gitRepo.Valid {
		run.GitRepo = gitRepo.String
	}
	if gitCommit.Valid {
		run.GitCommit = gitCommit.String
	}
	if gitRef.Valid {
		run.GitRef = gitRef.String
	}
	if artifactsPrefix.Valid {
		run.ArtifactsPrefix = artifactsPrefix.String
	}
	if endedAt.Valid {
		ended := endedAt.Time.UTC()
		run.EndedAt = &ended
	}
	params, err := decodeMetadata(paramsJSON)
	if err != nil {
		return domain.Run{}, fmt.Errorf("decode params: %w", err)
	}
	metrics, err := decodeMetadata(metricsJSON)
	if err != nil {
		return domain.Run{}, fmt.Errorf("decode metrics: %w", err)
	}
	run.Params = params
	run.Metrics = metrics
	return run, nil
}

func (s *RunStore) ListRuns(ctx context.Context, filter repo.RunFilter) ([]domain.Run, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("run store not initialized")
	}
	if strings.TrimSpace(filter.ProjectID) == "" {
		return nil, fmt.Errorf("project id is required")
	}
	clauses := make([]string, 0, 3)
	args := make([]any, 0, 3)

	if strings.TrimSpace(filter.ProjectID) != "" {
		args = append(args, strings.TrimSpace(filter.ProjectID))
		clauses = append(clauses, fmt.Sprintf("project_id = $%d", len(args)))
	}
	if strings.TrimSpace(filter.ExperimentID) != "" {
		args = append(args, strings.TrimSpace(filter.ExperimentID))
		clauses = append(clauses, fmt.Sprintf("experiment_id = $%d", len(args)))
	}
	if strings.TrimSpace(filter.Status) != "" {
		args = append(args, strings.TrimSpace(filter.Status))
		clauses = append(clauses, fmt.Sprintf("status = $%d", len(args)))
	}

	query := `SELECT run_id, experiment_id, project_id, dataset_version_id, status, started_at, ended_at,
		git_repo, git_commit, git_ref, params, metrics, artifacts_prefix, integrity_sha256
		FROM experiment_runs`
	if len(clauses) > 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}
	query += " ORDER BY started_at DESC"
	if filter.Limit > 0 {
		args = append(args, filter.Limit)
		query += fmt.Sprintf(" LIMIT $%d", len(args))
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list runs: %w", err)
	}
	defer rows.Close()

	runs := make([]domain.Run, 0)
	for rows.Next() {
		var run domain.Run
		var paramsJSON []byte
		var metricsJSON []byte
		var datasetVersionID sql.NullString
		var gitRepo sql.NullString
		var gitCommit sql.NullString
		var gitRef sql.NullString
		var artifactsPrefix sql.NullString
		var endedAt sql.NullTime
		if err := rows.Scan(&run.ID, &run.ExperimentID, &run.ProjectID, &datasetVersionID, &run.Status, &run.StartedAt, &endedAt,
			&gitRepo, &gitCommit, &gitRef, &paramsJSON, &metricsJSON, &artifactsPrefix, &run.IntegritySHA256); err != nil {
			return nil, fmt.Errorf("scan run: %w", err)
		}
		if datasetVersionID.Valid {
			run.DatasetVersionID = datasetVersionID.String
		}
		if gitRepo.Valid {
			run.GitRepo = gitRepo.String
		}
		if gitCommit.Valid {
			run.GitCommit = gitCommit.String
		}
		if gitRef.Valid {
			run.GitRef = gitRef.String
		}
		if artifactsPrefix.Valid {
			run.ArtifactsPrefix = artifactsPrefix.String
		}
		if endedAt.Valid {
			ended := endedAt.Time.UTC()
			run.EndedAt = &ended
		}
		params, err := decodeMetadata(paramsJSON)
		if err != nil {
			return nil, fmt.Errorf("decode params: %w", err)
		}
		metrics, err := decodeMetadata(metricsJSON)
		if err != nil {
			return nil, fmt.Errorf("decode metrics: %w", err)
		}
		run.Params = params
		run.Metrics = metrics
		runs = append(runs, run)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list runs: %w", err)
	}
	return runs, nil
}

func (s *RunStore) UpdateRunStatus(ctx context.Context, projectID, id string, status string, endedAt *time.Time) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("run store not initialized")
	}
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return fmt.Errorf("project id is required")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("run id is required")
	}
	status = strings.TrimSpace(status)
	if status == "" {
		return fmt.Errorf("status is required")
	}
	var ended sql.NullTime
	if endedAt != nil {
		ended = sql.NullTime{Time: endedAt.UTC(), Valid: true}
	}
	res, err := s.db.ExecContext(
		ctx,
		`UPDATE experiment_runs SET status = $1, ended_at = $2 WHERE project_id = $3 AND run_id = $4`,
		status,
		ended,
		projectID,
		id,
	)
	if err != nil {
		return fmt.Errorf("update run status: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("update run status: %w", err)
	}
	if rows == 0 {
		return repo.ErrNotFound
	}
	return nil
}

func nullIfEmpty(value string) sql.NullString {
	value = strings.TrimSpace(value)
	if value == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: value, Valid: true}
}
