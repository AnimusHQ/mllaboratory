package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/animus-labs/animus-go/closed/internal/domain"
	"github.com/animus-labs/animus-go/closed/internal/repo"
)

type ModelStore struct {
	db DB
}

const (
	insertModelQuery = `INSERT INTO models (
			model_id,
			project_id,
			name,
			version,
			status,
			artifact_id,
			metadata,
			created_at,
			created_by,
			idempotency_key,
			integrity_sha256
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		ON CONFLICT (project_id, idempotency_key) DO NOTHING
		RETURNING model_id, project_id, name, version, status, artifact_id, metadata, created_at, created_by, integrity_sha256`
	selectModelByIDQuery = `SELECT model_id, project_id, name, version, status, artifact_id, metadata, created_at, created_by, integrity_sha256
		 FROM models
		 WHERE project_id = $1 AND model_id = $2`
	selectModelByIdempotencyQuery = `SELECT model_id, project_id, name, version, status, artifact_id, metadata, created_at, created_by, integrity_sha256
		 FROM models
		 WHERE project_id = $1 AND idempotency_key = $2`
)

func NewModelStore(db DB) *ModelStore {
	if db == nil {
		return nil
	}
	return &ModelStore{db: db}
}

func (s *ModelStore) CreateModel(ctx context.Context, model domain.Model, idempotencyKey string) (domain.Model, bool, error) {
	if s == nil || s.db == nil {
		return domain.Model{}, false, fmt.Errorf("model store not initialized")
	}
	if err := model.Validate(); err != nil {
		return domain.Model{}, false, err
	}
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if idempotencyKey == "" {
		return domain.Model{}, false, fmt.Errorf("idempotency key is required")
	}
	if err := requireIntegrity(model.IntegritySHA256); err != nil {
		return domain.Model{}, false, err
	}
	metadataJSON, err := encodeMetadata(model.Metadata)
	if err != nil {
		return domain.Model{}, false, fmt.Errorf("encode metadata: %w", err)
	}
	createdAt := normalizeTime(model.CreatedAt)

	var out domain.Model
	var artifactID sql.NullString
	row := s.db.QueryRowContext(
		ctx,
		insertModelQuery,
		strings.TrimSpace(model.ID),
		strings.TrimSpace(model.ProjectID),
		strings.TrimSpace(model.Name),
		strings.TrimSpace(model.Version),
		string(model.Status),
		nullIfEmpty(model.ArtifactID),
		metadataJSON,
		createdAt,
		strings.TrimSpace(model.CreatedBy),
		idempotencyKey,
		strings.TrimSpace(model.IntegritySHA256),
	)
	if err := row.Scan(&out.ID, &out.ProjectID, &out.Name, &out.Version, &out.Status, &artifactID, &metadataJSON, &out.CreatedAt, &out.CreatedBy, &out.IntegritySHA256); err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return domain.Model{}, false, fmt.Errorf("insert model: %w", err)
		}
		existing, err := s.GetModelByIdempotencyKey(ctx, model.ProjectID, idempotencyKey)
		if err != nil {
			return domain.Model{}, false, err
		}
		return existing, false, nil
	}
	if artifactID.Valid {
		out.ArtifactID = artifactID.String
	}
	meta, err := decodeMetadata(metadataJSON)
	if err != nil {
		return domain.Model{}, false, fmt.Errorf("decode metadata: %w", err)
	}
	out.Metadata = meta
	return out, true, nil
}

func (s *ModelStore) GetModelByIdempotencyKey(ctx context.Context, projectID, idempotencyKey string) (domain.Model, error) {
	if s == nil || s.db == nil {
		return domain.Model{}, fmt.Errorf("model store not initialized")
	}
	projectID = strings.TrimSpace(projectID)
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if projectID == "" {
		return domain.Model{}, fmt.Errorf("project id is required")
	}
	if idempotencyKey == "" {
		return domain.Model{}, fmt.Errorf("idempotency key is required")
	}
	var model domain.Model
	var metadataJSON []byte
	var artifactID sql.NullString
	row := s.db.QueryRowContext(ctx, selectModelByIdempotencyQuery, projectID, idempotencyKey)
	if err := row.Scan(&model.ID, &model.ProjectID, &model.Name, &model.Version, &model.Status, &artifactID, &metadataJSON, &model.CreatedAt, &model.CreatedBy, &model.IntegritySHA256); err != nil {
		return domain.Model{}, handleNotFound(err)
	}
	if artifactID.Valid {
		model.ArtifactID = artifactID.String
	}
	meta, err := decodeMetadata(metadataJSON)
	if err != nil {
		return domain.Model{}, fmt.Errorf("decode metadata: %w", err)
	}
	model.Metadata = meta
	return model, nil
}

func (s *ModelStore) GetModel(ctx context.Context, projectID, id string) (domain.Model, error) {
	if s == nil || s.db == nil {
		return domain.Model{}, fmt.Errorf("model store not initialized")
	}
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return domain.Model{}, fmt.Errorf("project id is required")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return domain.Model{}, fmt.Errorf("model id is required")
	}
	var model domain.Model
	var metadataJSON []byte
	var artifactID sql.NullString
	row := s.db.QueryRowContext(ctx, selectModelByIDQuery, projectID, id)
	if err := row.Scan(&model.ID, &model.ProjectID, &model.Name, &model.Version, &model.Status, &artifactID, &metadataJSON, &model.CreatedAt, &model.CreatedBy, &model.IntegritySHA256); err != nil {
		return domain.Model{}, handleNotFound(err)
	}
	if artifactID.Valid {
		model.ArtifactID = artifactID.String
	}
	meta, err := decodeMetadata(metadataJSON)
	if err != nil {
		return domain.Model{}, fmt.Errorf("decode metadata: %w", err)
	}
	model.Metadata = meta
	return model, nil
}

func (s *ModelStore) ListModels(ctx context.Context, filter repo.ModelFilter) ([]domain.Model, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("model store not initialized")
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
	if strings.TrimSpace(filter.Name) != "" {
		args = append(args, strings.TrimSpace(filter.Name))
		clauses = append(clauses, fmt.Sprintf("name = $%d", len(args)))
	}
	if strings.TrimSpace(string(filter.Status)) != "" {
		args = append(args, string(filter.Status))
		clauses = append(clauses, fmt.Sprintf("status = $%d", len(args)))
	}

	query := `SELECT model_id, project_id, name, version, status, artifact_id, metadata, created_at, created_by, integrity_sha256 FROM models`
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
		return nil, fmt.Errorf("list models: %w", err)
	}
	defer rows.Close()

	models := make([]domain.Model, 0)
	for rows.Next() {
		var model domain.Model
		var metadataJSON []byte
		var artifactID sql.NullString
		if err := rows.Scan(&model.ID, &model.ProjectID, &model.Name, &model.Version, &model.Status, &artifactID, &metadataJSON, &model.CreatedAt, &model.CreatedBy, &model.IntegritySHA256); err != nil {
			return nil, fmt.Errorf("scan model: %w", err)
		}
		if artifactID.Valid {
			model.ArtifactID = artifactID.String
		}
		meta, err := decodeMetadata(metadataJSON)
		if err != nil {
			return nil, fmt.Errorf("decode metadata: %w", err)
		}
		model.Metadata = meta
		models = append(models, model)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list models: %w", err)
	}
	return models, nil
}

func (s *ModelStore) UpdateModelStatus(ctx context.Context, projectID, id string, status domain.ModelStatus) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("model store not initialized")
	}
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return fmt.Errorf("project id is required")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("model id is required")
	}
	if !status.Valid() {
		return fmt.Errorf("invalid model status")
	}
	res, err := s.db.ExecContext(ctx, `UPDATE models SET status = $1 WHERE project_id = $2 AND model_id = $3`, string(status), projectID, id)
	if err != nil {
		return fmt.Errorf("update model status: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("update model status: %w", err)
	}
	if rows == 0 {
		return repo.ErrNotFound
	}
	return nil
}
