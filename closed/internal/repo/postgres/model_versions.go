package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/animus-labs/animus-go/closed/internal/domain"
	"github.com/animus-labs/animus-go/closed/internal/repo"
)

type ModelVersionStore struct {
	db DB
}

type ModelVersionTransitionStore struct {
	db DB
}

type ModelExportStore struct {
	db DB
}

const (
	insertModelVersionQuery = `INSERT INTO model_versions (
			model_version_id,
			project_id,
			model_id,
			version,
			status,
			run_id,
			artifact_ids,
			dataset_version_ids,
			env_lock_id,
			code_ref,
			policy_snapshot_sha256,
			created_at,
			created_by,
			idempotency_key,
			integrity_sha256
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
		ON CONFLICT (project_id, idempotency_key) DO NOTHING
		RETURNING model_version_id, project_id, model_id, version, status, run_id, artifact_ids, dataset_version_ids,
			env_lock_id, code_ref, policy_snapshot_sha256, created_at, created_by, integrity_sha256`
	selectModelVersionByIDQuery = `SELECT model_version_id, project_id, model_id, version, status, run_id, artifact_ids, dataset_version_ids,
			env_lock_id, code_ref, policy_snapshot_sha256, created_at, created_by, integrity_sha256
		 FROM model_versions
		 WHERE project_id = $1 AND model_version_id = $2`
	selectModelVersionByIdempotencyQuery = `SELECT model_version_id, project_id, model_id, version, status, run_id, artifact_ids, dataset_version_ids,
			env_lock_id, code_ref, policy_snapshot_sha256, created_at, created_by, integrity_sha256
		 FROM model_versions
		 WHERE project_id = $1 AND idempotency_key = $2`
	selectModelVersionListQuery = `SELECT model_version_id, project_id, model_id, version, status, run_id, artifact_ids, dataset_version_ids,
			env_lock_id, code_ref, policy_snapshot_sha256, created_at, created_by, integrity_sha256
		 FROM model_versions`
	updateModelVersionStatusQuery = `UPDATE model_versions SET status = $1 WHERE project_id = $2 AND model_version_id = $3`

	insertModelVersionTransitionQuery = `INSERT INTO model_version_transitions (
			project_id,
			model_version_id,
			from_status,
			to_status,
			action,
			request_id,
			occurred_at,
			actor
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`

	insertModelExportQuery = `INSERT INTO model_exports (
			export_id,
			project_id,
			model_version_id,
			status,
			target,
			created_at,
			created_by,
			idempotency_key,
			integrity_sha256
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		ON CONFLICT (project_id, idempotency_key) DO NOTHING
		RETURNING export_id, project_id, model_version_id, status, target, created_at, created_by, integrity_sha256`
	selectModelExportByIdempotencyQuery = `SELECT export_id, project_id, model_version_id, status, target, created_at, created_by, integrity_sha256
		 FROM model_exports
		 WHERE project_id = $1 AND idempotency_key = $2`
)

func NewModelVersionStore(db DB) *ModelVersionStore {
	if db == nil {
		return nil
	}
	return &ModelVersionStore{db: db}
}

func NewModelVersionTransitionStore(db DB) *ModelVersionTransitionStore {
	if db == nil {
		return nil
	}
	return &ModelVersionTransitionStore{db: db}
}

func NewModelExportStore(db DB) *ModelExportStore {
	if db == nil {
		return nil
	}
	return &ModelExportStore{db: db}
}

func (s *ModelVersionStore) Create(ctx context.Context, version domain.ModelVersion, idempotencyKey string) (domain.ModelVersion, bool, error) {
	if s == nil || s.db == nil {
		return domain.ModelVersion{}, false, fmt.Errorf("model version store not initialized")
	}
	if err := version.Validate(); err != nil {
		return domain.ModelVersion{}, false, err
	}
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if idempotencyKey == "" {
		return domain.ModelVersion{}, false, fmt.Errorf("idempotency key is required")
	}
	if err := requireIntegrity(version.IntegritySHA256); err != nil {
		return domain.ModelVersion{}, false, err
	}
	artifactJSON, err := json.Marshal(version.ArtifactIDs)
	if err != nil {
		return domain.ModelVersion{}, false, fmt.Errorf("marshal artifact ids: %w", err)
	}
	datasetJSON, err := json.Marshal(version.DatasetVersionIDs)
	if err != nil {
		return domain.ModelVersion{}, false, fmt.Errorf("marshal dataset version ids: %w", err)
	}
	codeRefJSON, err := json.Marshal(version.CodeRef)
	if err != nil {
		return domain.ModelVersion{}, false, fmt.Errorf("marshal code ref: %w", err)
	}
	createdAt := normalizeTime(version.CreatedAt)

	out, err := s.scanModelVersionRow(
		s.db.QueryRowContext(
			ctx,
			insertModelVersionQuery,
			strings.TrimSpace(version.ID),
			strings.TrimSpace(version.ProjectID),
			strings.TrimSpace(version.ModelID),
			strings.TrimSpace(version.Version),
			string(version.Status),
			strings.TrimSpace(version.RunID),
			artifactJSON,
			datasetJSON,
			nullIfEmpty(version.EnvLockID),
			codeRefJSON,
			nullIfEmpty(version.PolicySnapshotSHA256),
			createdAt,
			strings.TrimSpace(version.CreatedBy),
			idempotencyKey,
			strings.TrimSpace(version.IntegritySHA256),
		),
	)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return domain.ModelVersion{}, false, fmt.Errorf("insert model version: %w", err)
		}
		existing, err := s.GetByIdempotencyKey(ctx, version.ProjectID, idempotencyKey)
		if err != nil {
			return domain.ModelVersion{}, false, err
		}
		return existing, false, nil
	}
	return out, true, nil
}

func (s *ModelVersionStore) GetByIdempotencyKey(ctx context.Context, projectID, idempotencyKey string) (domain.ModelVersion, error) {
	if s == nil || s.db == nil {
		return domain.ModelVersion{}, fmt.Errorf("model version store not initialized")
	}
	projectID = strings.TrimSpace(projectID)
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if projectID == "" {
		return domain.ModelVersion{}, fmt.Errorf("project id is required")
	}
	if idempotencyKey == "" {
		return domain.ModelVersion{}, fmt.Errorf("idempotency key is required")
	}
	return s.scanModelVersionRow(s.db.QueryRowContext(ctx, selectModelVersionByIdempotencyQuery, projectID, idempotencyKey))
}

func (s *ModelVersionStore) Get(ctx context.Context, projectID, versionID string) (domain.ModelVersion, error) {
	if s == nil || s.db == nil {
		return domain.ModelVersion{}, fmt.Errorf("model version store not initialized")
	}
	projectID = strings.TrimSpace(projectID)
	versionID = strings.TrimSpace(versionID)
	if projectID == "" {
		return domain.ModelVersion{}, fmt.Errorf("project id is required")
	}
	if versionID == "" {
		return domain.ModelVersion{}, fmt.Errorf("model version id is required")
	}
	return s.scanModelVersionRow(s.db.QueryRowContext(ctx, selectModelVersionByIDQuery, projectID, versionID))
}

func (s *ModelVersionStore) List(ctx context.Context, filter repo.ModelVersionFilter) ([]domain.ModelVersion, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("model version store not initialized")
	}
	if strings.TrimSpace(filter.ProjectID) == "" {
		return nil, fmt.Errorf("project id is required")
	}
	clauses := make([]string, 0, 3)
	args := make([]any, 0, 3)

	args = append(args, strings.TrimSpace(filter.ProjectID))
	clauses = append(clauses, fmt.Sprintf("project_id = $%d", len(args)))

	if strings.TrimSpace(filter.ModelID) != "" {
		args = append(args, strings.TrimSpace(filter.ModelID))
		clauses = append(clauses, fmt.Sprintf("model_id = $%d", len(args)))
	}
	if strings.TrimSpace(string(filter.Status)) != "" {
		args = append(args, string(filter.Status))
		clauses = append(clauses, fmt.Sprintf("status = $%d", len(args)))
	}

	query := selectModelVersionListQuery
	if len(clauses) > 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}
	query += " ORDER BY created_at DESC"
	if filter.Limit > 0 {
		args = append(args, filter.Limit)
		query += fmt.Sprintf(" LIMIT $%d", len(args))
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list model versions: %w", err)
	}
	defer rows.Close()

	out := make([]domain.ModelVersion, 0)
	for rows.Next() {
		version, err := s.scanModelVersionRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, version)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list model versions: %w", err)
	}
	return out, nil
}

func (s *ModelVersionStore) UpdateStatus(ctx context.Context, projectID, versionID string, status domain.ModelStatus) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("model version store not initialized")
	}
	projectID = strings.TrimSpace(projectID)
	versionID = strings.TrimSpace(versionID)
	if projectID == "" {
		return fmt.Errorf("project id is required")
	}
	if versionID == "" {
		return fmt.Errorf("model version id is required")
	}
	if !status.Valid() {
		return fmt.Errorf("invalid status")
	}
	res, err := s.db.ExecContext(ctx, updateModelVersionStatusQuery, string(status), projectID, versionID)
	if err != nil {
		return fmt.Errorf("update model version status: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("update model version status: %w", err)
	}
	if rows == 0 {
		return repo.ErrNotFound
	}
	return nil
}

func (s *ModelVersionStore) scanModelVersionRow(row *sql.Row) (domain.ModelVersion, error) {
	var version domain.ModelVersion
	var artifactJSON []byte
	var datasetJSON []byte
	var codeRefJSON []byte
	var envLockID sql.NullString
	var policySHA sql.NullString
	if err := row.Scan(&version.ID, &version.ProjectID, &version.ModelID, &version.Version, &version.Status, &version.RunID, &artifactJSON, &datasetJSON, &envLockID, &codeRefJSON, &policySHA, &version.CreatedAt, &version.CreatedBy, &version.IntegritySHA256); err != nil {
		return domain.ModelVersion{}, handleNotFound(err)
	}
	if envLockID.Valid {
		version.EnvLockID = envLockID.String
	}
	if policySHA.Valid {
		version.PolicySnapshotSHA256 = policySHA.String
	}
	if err := json.Unmarshal(artifactJSON, &version.ArtifactIDs); err != nil {
		return domain.ModelVersion{}, fmt.Errorf("decode artifact ids: %w", err)
	}
	if err := json.Unmarshal(datasetJSON, &version.DatasetVersionIDs); err != nil {
		return domain.ModelVersion{}, fmt.Errorf("decode dataset version ids: %w", err)
	}
	if len(codeRefJSON) > 0 {
		if err := json.Unmarshal(codeRefJSON, &version.CodeRef); err != nil {
			return domain.ModelVersion{}, fmt.Errorf("decode code ref: %w", err)
		}
	}
	return version, nil
}

func (s *ModelVersionStore) scanModelVersionRows(rows *sql.Rows) (domain.ModelVersion, error) {
	var version domain.ModelVersion
	var artifactJSON []byte
	var datasetJSON []byte
	var codeRefJSON []byte
	var envLockID sql.NullString
	var policySHA sql.NullString
	if err := rows.Scan(&version.ID, &version.ProjectID, &version.ModelID, &version.Version, &version.Status, &version.RunID, &artifactJSON, &datasetJSON, &envLockID, &codeRefJSON, &policySHA, &version.CreatedAt, &version.CreatedBy, &version.IntegritySHA256); err != nil {
		return domain.ModelVersion{}, fmt.Errorf("scan model version: %w", err)
	}
	if envLockID.Valid {
		version.EnvLockID = envLockID.String
	}
	if policySHA.Valid {
		version.PolicySnapshotSHA256 = policySHA.String
	}
	if err := json.Unmarshal(artifactJSON, &version.ArtifactIDs); err != nil {
		return domain.ModelVersion{}, fmt.Errorf("decode artifact ids: %w", err)
	}
	if err := json.Unmarshal(datasetJSON, &version.DatasetVersionIDs); err != nil {
		return domain.ModelVersion{}, fmt.Errorf("decode dataset version ids: %w", err)
	}
	if len(codeRefJSON) > 0 {
		if err := json.Unmarshal(codeRefJSON, &version.CodeRef); err != nil {
			return domain.ModelVersion{}, fmt.Errorf("decode code ref: %w", err)
		}
	}
	return version, nil
}

func (s *ModelVersionTransitionStore) Insert(ctx context.Context, transition domain.ModelVersionTransition) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("model version transition store not initialized")
	}
	if strings.TrimSpace(transition.ProjectID) == "" {
		return fmt.Errorf("project id is required")
	}
	if strings.TrimSpace(transition.ModelVersionID) == "" {
		return fmt.Errorf("model version id is required")
	}
	if !transition.FromStatus.Valid() || !transition.ToStatus.Valid() {
		return fmt.Errorf("invalid status")
	}
	if strings.TrimSpace(transition.Action) == "" {
		return fmt.Errorf("action is required")
	}
	if strings.TrimSpace(transition.Actor) == "" {
		return fmt.Errorf("actor is required")
	}
	occurredAt := transition.OccurredAt
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}
	_, err := s.db.ExecContext(
		ctx,
		insertModelVersionTransitionQuery,
		strings.TrimSpace(transition.ProjectID),
		strings.TrimSpace(transition.ModelVersionID),
		string(transition.FromStatus),
		string(transition.ToStatus),
		strings.TrimSpace(transition.Action),
		nullIfEmpty(transition.RequestID),
		occurredAt,
		strings.TrimSpace(transition.Actor),
	)
	return err
}

func (s *ModelExportStore) Create(ctx context.Context, export domain.ModelExport, idempotencyKey string) (domain.ModelExport, bool, error) {
	if s == nil || s.db == nil {
		return domain.ModelExport{}, false, fmt.Errorf("model export store not initialized")
	}
	if err := export.Validate(); err != nil {
		return domain.ModelExport{}, false, err
	}
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if idempotencyKey == "" {
		return domain.ModelExport{}, false, fmt.Errorf("idempotency key is required")
	}
	if err := requireIntegrity(export.IntegritySHA256); err != nil {
		return domain.ModelExport{}, false, err
	}
	createdAt := normalizeTime(export.CreatedAt)

	var out domain.ModelExport
	row := s.db.QueryRowContext(
		ctx,
		insertModelExportQuery,
		strings.TrimSpace(export.ExportID),
		strings.TrimSpace(export.ProjectID),
		strings.TrimSpace(export.ModelVersionID),
		strings.TrimSpace(export.Status),
		nullIfEmpty(export.Target),
		createdAt,
		strings.TrimSpace(export.CreatedBy),
		idempotencyKey,
		strings.TrimSpace(export.IntegritySHA256),
	)
	if err := row.Scan(&out.ExportID, &out.ProjectID, &out.ModelVersionID, &out.Status, &out.Target, &out.CreatedAt, &out.CreatedBy, &out.IntegritySHA256); err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return domain.ModelExport{}, false, fmt.Errorf("insert model export: %w", err)
		}
		existing, err := s.GetByIdempotencyKey(ctx, export.ProjectID, idempotencyKey)
		if err != nil {
			return domain.ModelExport{}, false, err
		}
		return existing, false, nil
	}
	return out, true, nil
}

func (s *ModelExportStore) GetByIdempotencyKey(ctx context.Context, projectID, idempotencyKey string) (domain.ModelExport, error) {
	if s == nil || s.db == nil {
		return domain.ModelExport{}, fmt.Errorf("model export store not initialized")
	}
	projectID = strings.TrimSpace(projectID)
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if projectID == "" {
		return domain.ModelExport{}, fmt.Errorf("project id is required")
	}
	if idempotencyKey == "" {
		return domain.ModelExport{}, fmt.Errorf("idempotency key is required")
	}
	var out domain.ModelExport
	row := s.db.QueryRowContext(ctx, selectModelExportByIdempotencyQuery, projectID, idempotencyKey)
	if err := row.Scan(&out.ExportID, &out.ProjectID, &out.ModelVersionID, &out.Status, &out.Target, &out.CreatedAt, &out.CreatedBy, &out.IntegritySHA256); err != nil {
		return domain.ModelExport{}, handleNotFound(err)
	}
	return out, nil
}
