package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/animus-labs/animus-go/closed/internal/domain"
	"github.com/animus-labs/animus-go/closed/internal/repo"
)

type DatasetStore struct {
	db DB
}

func NewDatasetStore(db DB) *DatasetStore {
	if db == nil {
		return nil
	}
	return &DatasetStore{db: db}
}

func (s *DatasetStore) CreateDataset(ctx context.Context, dataset domain.Dataset) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("dataset store not initialized")
	}
	if err := dataset.Validate(); err != nil {
		return err
	}
	if err := requireIntegrity(dataset.IntegritySHA256); err != nil {
		return err
	}
	metadataJSON, err := encodeMetadata(dataset.Metadata)
	if err != nil {
		return fmt.Errorf("encode metadata: %w", err)
	}
	createdAt := normalizeTime(dataset.CreatedAt)
	_, err = s.db.ExecContext(
		ctx,
		`INSERT INTO datasets (
			dataset_id,
			project_id,
			name,
			description,
			metadata,
			created_at,
			created_by,
			integrity_sha256
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		strings.TrimSpace(dataset.ID),
		strings.TrimSpace(dataset.ProjectID),
		strings.TrimSpace(dataset.Name),
		strings.TrimSpace(dataset.Description),
		metadataJSON,
		createdAt,
		strings.TrimSpace(dataset.CreatedBy),
		strings.TrimSpace(dataset.IntegritySHA256),
	)
	if err != nil {
		return fmt.Errorf("insert dataset: %w", err)
	}
	return nil
}

func (s *DatasetStore) GetDataset(ctx context.Context, projectID, id string) (domain.Dataset, error) {
	if s == nil || s.db == nil {
		return domain.Dataset{}, fmt.Errorf("dataset store not initialized")
	}
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return domain.Dataset{}, fmt.Errorf("project id is required")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return domain.Dataset{}, fmt.Errorf("dataset id is required")
	}
	var dataset domain.Dataset
	var metadataJSON []byte
	row := s.db.QueryRowContext(
		ctx,
		`SELECT dataset_id, project_id, name, description, metadata, created_at, created_by, integrity_sha256
		 FROM datasets
		 WHERE project_id = $1 AND dataset_id = $2`,
		projectID,
		id,
	)
	if err := row.Scan(&dataset.ID, &dataset.ProjectID, &dataset.Name, &dataset.Description, &metadataJSON, &dataset.CreatedAt, &dataset.CreatedBy, &dataset.IntegritySHA256); err != nil {
		return domain.Dataset{}, handleNotFound(err)
	}
	meta, err := decodeMetadata(metadataJSON)
	if err != nil {
		return domain.Dataset{}, fmt.Errorf("decode metadata: %w", err)
	}
	dataset.Metadata = meta
	return dataset, nil
}

func (s *DatasetStore) ListDatasets(ctx context.Context, filter repo.DatasetFilter) ([]domain.Dataset, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("dataset store not initialized")
	}
	if strings.TrimSpace(filter.ProjectID) == "" {
		return nil, fmt.Errorf("project id is required")
	}
	clauses := make([]string, 0, 2)
	args := make([]any, 0, 2)

	if strings.TrimSpace(filter.ProjectID) != "" {
		args = append(args, strings.TrimSpace(filter.ProjectID))
		clauses = append(clauses, fmt.Sprintf("project_id = $%d", len(args)))
	}
	if strings.TrimSpace(filter.Name) != "" {
		args = append(args, strings.TrimSpace(filter.Name))
		clauses = append(clauses, fmt.Sprintf("name = $%d", len(args)))
	}

	query := `SELECT dataset_id, project_id, name, description, metadata, created_at, created_by, integrity_sha256 FROM datasets`
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
		return nil, fmt.Errorf("list datasets: %w", err)
	}
	defer rows.Close()

	datasets := make([]domain.Dataset, 0)
	for rows.Next() {
		var dataset domain.Dataset
		var metadataJSON []byte
		if err := rows.Scan(&dataset.ID, &dataset.ProjectID, &dataset.Name, &dataset.Description, &metadataJSON, &dataset.CreatedAt, &dataset.CreatedBy, &dataset.IntegritySHA256); err != nil {
			return nil, fmt.Errorf("scan dataset: %w", err)
		}
		meta, err := decodeMetadata(metadataJSON)
		if err != nil {
			return nil, fmt.Errorf("decode metadata: %w", err)
		}
		dataset.Metadata = meta
		datasets = append(datasets, dataset)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list datasets: %w", err)
	}
	return datasets, nil
}

func (s *DatasetStore) CreateDatasetVersion(ctx context.Context, version domain.DatasetVersion) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("dataset store not initialized")
	}
	if err := version.Validate(); err != nil {
		return err
	}
	if err := requireIntegrity(version.IntegritySHA256); err != nil {
		return err
	}
	metadataJSON, err := encodeMetadata(version.Metadata)
	if err != nil {
		return fmt.Errorf("encode metadata: %w", err)
	}
	createdAt := normalizeTime(version.CreatedAt)
	_, err = s.db.ExecContext(
		ctx,
		`INSERT INTO dataset_versions (
			version_id,
			dataset_id,
			project_id,
			quality_rule_id,
			ordinal,
			content_sha256,
			object_key,
			size_bytes,
			metadata,
			created_at,
			created_by,
			integrity_sha256
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
		strings.TrimSpace(version.ID),
		strings.TrimSpace(version.DatasetID),
		strings.TrimSpace(version.ProjectID),
		strings.TrimSpace(version.QualityRuleID),
		version.Ordinal,
		strings.TrimSpace(version.ContentSHA256),
		strings.TrimSpace(version.ObjectKey),
		version.SizeBytes,
		metadataJSON,
		createdAt,
		strings.TrimSpace(version.CreatedBy),
		strings.TrimSpace(version.IntegritySHA256),
	)
	if err != nil {
		return fmt.Errorf("insert dataset version: %w", err)
	}
	return nil
}

func (s *DatasetStore) GetDatasetVersion(ctx context.Context, projectID, id string) (domain.DatasetVersion, error) {
	if s == nil || s.db == nil {
		return domain.DatasetVersion{}, fmt.Errorf("dataset store not initialized")
	}
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return domain.DatasetVersion{}, fmt.Errorf("project id is required")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return domain.DatasetVersion{}, fmt.Errorf("version id is required")
	}
	var version domain.DatasetVersion
	var metadataJSON []byte
	row := s.db.QueryRowContext(
		ctx,
		`SELECT version_id, dataset_id, project_id, quality_rule_id, ordinal, content_sha256, object_key, size_bytes, metadata, created_at, created_by, integrity_sha256
		 FROM dataset_versions
		 WHERE project_id = $1 AND version_id = $2`,
		projectID,
		id,
	)
	if err := row.Scan(&version.ID, &version.DatasetID, &version.ProjectID, &version.QualityRuleID, &version.Ordinal, &version.ContentSHA256, &version.ObjectKey, &version.SizeBytes, &metadataJSON, &version.CreatedAt, &version.CreatedBy, &version.IntegritySHA256); err != nil {
		return domain.DatasetVersion{}, handleNotFound(err)
	}
	meta, err := decodeMetadata(metadataJSON)
	if err != nil {
		return domain.DatasetVersion{}, fmt.Errorf("decode metadata: %w", err)
	}
	version.Metadata = meta
	return version, nil
}

func (s *DatasetStore) ListDatasetVersions(ctx context.Context, filter repo.DatasetVersionFilter) ([]domain.DatasetVersion, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("dataset store not initialized")
	}
	if strings.TrimSpace(filter.ProjectID) == "" {
		return nil, fmt.Errorf("project id is required")
	}
	clauses := make([]string, 0, 2)
	args := make([]any, 0, 2)

	if strings.TrimSpace(filter.ProjectID) != "" {
		args = append(args, strings.TrimSpace(filter.ProjectID))
		clauses = append(clauses, fmt.Sprintf("project_id = $%d", len(args)))
	}
	if strings.TrimSpace(filter.DatasetID) != "" {
		args = append(args, strings.TrimSpace(filter.DatasetID))
		clauses = append(clauses, fmt.Sprintf("dataset_id = $%d", len(args)))
	}

	query := `SELECT version_id, dataset_id, project_id, quality_rule_id, ordinal, content_sha256, object_key, size_bytes, metadata, created_at, created_by, integrity_sha256 FROM dataset_versions`
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
		return nil, fmt.Errorf("list dataset versions: %w", err)
	}
	defer rows.Close()

	versions := make([]domain.DatasetVersion, 0)
	for rows.Next() {
		var version domain.DatasetVersion
		var metadataJSON []byte
		if err := rows.Scan(&version.ID, &version.DatasetID, &version.ProjectID, &version.QualityRuleID, &version.Ordinal, &version.ContentSHA256, &version.ObjectKey, &version.SizeBytes, &metadataJSON, &version.CreatedAt, &version.CreatedBy, &version.IntegritySHA256); err != nil {
			return nil, fmt.Errorf("scan dataset version: %w", err)
		}
		meta, err := decodeMetadata(metadataJSON)
		if err != nil {
			return nil, fmt.Errorf("decode metadata: %w", err)
		}
		version.Metadata = meta
		versions = append(versions, version)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list dataset versions: %w", err)
	}
	return versions, nil
}

func (s *DatasetStore) NextDatasetVersionOrdinal(ctx context.Context, projectID, datasetID string) (int64, error) {
	if s == nil || s.db == nil {
		return 0, fmt.Errorf("dataset store not initialized")
	}
	projectID = strings.TrimSpace(projectID)
	datasetID = strings.TrimSpace(datasetID)
	if projectID == "" {
		return 0, fmt.Errorf("project id is required")
	}
	if datasetID == "" {
		return 0, fmt.Errorf("dataset id is required")
	}
	var ordinal int64
	err := s.db.QueryRowContext(
		ctx,
		`SELECT COALESCE(MAX(ordinal), 0) + 1 FROM dataset_versions WHERE project_id = $1 AND dataset_id = $2`,
		projectID,
		datasetID,
	).Scan(&ordinal)
	if err != nil {
		return 0, fmt.Errorf("next dataset version ordinal: %w", err)
	}
	return ordinal, nil
}
