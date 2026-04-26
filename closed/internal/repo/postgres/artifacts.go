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

type ArtifactStore struct {
	db DB
}

func NewArtifactStore(db DB) *ArtifactStore {
	if db == nil {
		return nil
	}
	return &ArtifactStore{db: db}
}

func (s *ArtifactStore) CreateArtifact(ctx context.Context, projectID string, artifact domain.Artifact) (domain.Artifact, error) {
	if s == nil || s.db == nil {
		return domain.Artifact{}, fmt.Errorf("artifact store not initialized")
	}
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return domain.Artifact{}, fmt.Errorf("project id is required")
	}
	if strings.TrimSpace(artifact.ProjectID) == "" {
		artifact.ProjectID = projectID
	}
	if strings.TrimSpace(artifact.ProjectID) != projectID {
		return domain.Artifact{}, fmt.Errorf("project id mismatch")
	}
	if err := artifact.Validate(); err != nil {
		return domain.Artifact{}, err
	}
	if err := requireIntegrity(artifact.IntegritySHA256); err != nil {
		return domain.Artifact{}, err
	}
	metadataJSON, err := encodeMetadata(artifact.Metadata)
	if err != nil {
		return domain.Artifact{}, fmt.Errorf("encode metadata: %w", err)
	}
	createdAt := normalizeTime(artifact.CreatedAt)

	var retentionUntil sql.NullTime
	if artifact.RetentionUntil != nil {
		retentionUntil = sql.NullTime{Time: artifact.RetentionUntil.UTC(), Valid: true}
	}
	_, err = s.db.ExecContext(
		ctx,
		`INSERT INTO artifacts (
			artifact_id,
			project_id,
			kind,
			object_key,
			content_type,
			size_bytes,
			sha256,
			metadata,
			retention_until,
			legal_hold,
			created_at,
			created_by,
			integrity_sha256
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
		strings.TrimSpace(artifact.ID),
		projectID,
		strings.TrimSpace(artifact.Kind),
		strings.TrimSpace(artifact.ObjectKey),
		nullIfEmpty(artifact.ContentType),
		artifact.SizeBytes,
		strings.TrimSpace(artifact.SHA256),
		metadataJSON,
		retentionUntil,
		artifact.LegalHold,
		createdAt,
		strings.TrimSpace(artifact.CreatedBy),
		strings.TrimSpace(artifact.IntegritySHA256),
	)
	if err != nil {
		return domain.Artifact{}, fmt.Errorf("insert artifact: %w", err)
	}
	artifact.ProjectID = projectID
	artifact.CreatedAt = createdAt
	return artifact, nil
}

func (s *ArtifactStore) GetArtifact(ctx context.Context, projectID, id string) (domain.Artifact, error) {
	if s == nil || s.db == nil {
		return domain.Artifact{}, fmt.Errorf("artifact store not initialized")
	}
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return domain.Artifact{}, fmt.Errorf("project id is required")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return domain.Artifact{}, fmt.Errorf("artifact id is required")
	}
	var artifact domain.Artifact
	var metadataJSON []byte
	var retentionUntil sql.NullTime
	var contentType sql.NullString
	row := s.db.QueryRowContext(
		ctx,
		`SELECT artifact_id, project_id, kind, object_key, content_type, size_bytes, sha256, metadata, retention_until, legal_hold, created_at, created_by, integrity_sha256
		 FROM artifacts
		 WHERE project_id = $1 AND artifact_id = $2`,
		projectID,
		id,
	)
	if err := row.Scan(&artifact.ID, &artifact.ProjectID, &artifact.Kind, &artifact.ObjectKey, &contentType, &artifact.SizeBytes, &artifact.SHA256, &metadataJSON, &retentionUntil, &artifact.LegalHold, &artifact.CreatedAt, &artifact.CreatedBy, &artifact.IntegritySHA256); err != nil {
		return domain.Artifact{}, handleNotFound(err)
	}
	if retentionUntil.Valid {
		retention := retentionUntil.Time.UTC()
		artifact.RetentionUntil = &retention
	}
	artifact.ContentType = strings.TrimSpace(contentType.String)
	meta, err := decodeMetadata(metadataJSON)
	if err != nil {
		return domain.Artifact{}, fmt.Errorf("decode metadata: %w", err)
	}
	artifact.Metadata = meta
	return artifact, nil
}

func (s *ArtifactStore) ListArtifacts(ctx context.Context, filter repo.ArtifactFilter) ([]domain.Artifact, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("artifact store not initialized")
	}
	query, args, err := buildArtifactListQuery(filter)
	if err != nil {
		return nil, err
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list artifacts: %w", err)
	}
	defer rows.Close()

	artifacts := make([]domain.Artifact, 0)
	for rows.Next() {
		var artifact domain.Artifact
		var metadataJSON []byte
		var retentionUntil sql.NullTime
		var contentType sql.NullString
		if err := rows.Scan(&artifact.ID, &artifact.ProjectID, &artifact.Kind, &artifact.ObjectKey, &contentType, &artifact.SizeBytes, &artifact.SHA256, &metadataJSON, &retentionUntil, &artifact.LegalHold, &artifact.CreatedAt, &artifact.CreatedBy, &artifact.IntegritySHA256); err != nil {
			return nil, fmt.Errorf("scan artifact: %w", err)
		}
		if retentionUntil.Valid {
			retention := retentionUntil.Time.UTC()
			artifact.RetentionUntil = &retention
		}
		artifact.ContentType = strings.TrimSpace(contentType.String)
		meta, err := decodeMetadata(metadataJSON)
		if err != nil {
			return nil, fmt.Errorf("decode metadata: %w", err)
		}
		artifact.Metadata = meta
		artifacts = append(artifacts, artifact)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list artifacts: %w", err)
	}
	return artifacts, nil
}

func (s *ArtifactStore) UpdateRetention(ctx context.Context, projectID, id string, retentionUntil *time.Time, legalHold bool) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("artifact store not initialized")
	}
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return fmt.Errorf("project id is required")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("artifact id is required")
	}

	var retention sql.NullTime
	if retentionUntil != nil {
		retention = sql.NullTime{Time: retentionUntil.UTC(), Valid: true}
	}
	res, err := s.db.ExecContext(
		ctx,
		`UPDATE artifacts
		 SET retention_until = $1,
		     legal_hold = $2
		 WHERE project_id = $3 AND artifact_id = $4`,
		retention,
		legalHold,
		projectID,
		id,
	)
	if err != nil {
		return fmt.Errorf("update artifact retention: %w", err)
	}
	rows, err := res.RowsAffected()
	if err == nil && rows == 0 {
		return repo.ErrNotFound
	}
	return nil
}

func buildArtifactListQuery(filter repo.ArtifactFilter) (string, []any, error) {
	if strings.TrimSpace(filter.ProjectID) == "" {
		return "", nil, fmt.Errorf("project id is required")
	}
	clauses := make([]string, 0, 2)
	args := make([]any, 0, 2)

	args = append(args, strings.TrimSpace(filter.ProjectID))
	clauses = append(clauses, fmt.Sprintf("project_id = $%d", len(args)))

	if strings.TrimSpace(filter.Kind) != "" {
		args = append(args, strings.TrimSpace(filter.Kind))
		clauses = append(clauses, fmt.Sprintf("kind = $%d", len(args)))
	}

	query := `SELECT artifact_id, project_id, kind, object_key, content_type, size_bytes, sha256, metadata, retention_until, legal_hold, created_at, created_by, integrity_sha256 FROM artifacts`
	if len(clauses) > 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}
	query += " ORDER BY created_at DESC"
	if filter.Limit > 0 {
		args = append(args, filter.Limit)
		query += fmt.Sprintf(" LIMIT $%d", len(args))
	}
	return query, args, nil
}
