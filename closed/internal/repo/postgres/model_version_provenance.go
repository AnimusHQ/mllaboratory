package postgres

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type ModelVersionProvenanceStore struct {
	db DB
}

const (
	insertModelVersionArtifactQuery = `INSERT INTO model_version_artifacts (
			project_id,
			model_version_id,
			artifact_id,
			created_at
		) VALUES ($1,$2,$3,$4)
		ON CONFLICT (project_id, model_version_id, artifact_id) DO NOTHING`
	insertModelVersionDatasetQuery = `INSERT INTO model_version_datasets (
			project_id,
			model_version_id,
			dataset_version_id,
			created_at
		) VALUES ($1,$2,$3,$4)
		ON CONFLICT (project_id, model_version_id, dataset_version_id) DO NOTHING`
)

func NewModelVersionProvenanceStore(db DB) *ModelVersionProvenanceStore {
	if db == nil {
		return nil
	}
	return &ModelVersionProvenanceStore{db: db}
}

func (s *ModelVersionProvenanceStore) InsertArtifacts(ctx context.Context, projectID, modelVersionID string, artifactIDs []string, createdAt time.Time) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("model version provenance store not initialized")
	}
	projectID = strings.TrimSpace(projectID)
	modelVersionID = strings.TrimSpace(modelVersionID)
	if projectID == "" {
		return fmt.Errorf("project id is required")
	}
	if modelVersionID == "" {
		return fmt.Errorf("model version id is required")
	}
	if len(artifactIDs) == 0 {
		return nil
	}
	createdAt = normalizeTime(createdAt)
	for _, artifactID := range artifactIDs {
		artifactID = strings.TrimSpace(artifactID)
		if artifactID == "" {
			return fmt.Errorf("artifact id is required")
		}
		if _, err := s.db.ExecContext(ctx, insertModelVersionArtifactQuery, projectID, modelVersionID, artifactID, createdAt); err != nil {
			return fmt.Errorf("insert model version artifact: %w", err)
		}
	}
	return nil
}

func (s *ModelVersionProvenanceStore) InsertDatasets(ctx context.Context, projectID, modelVersionID string, datasetVersionIDs []string, createdAt time.Time) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("model version provenance store not initialized")
	}
	projectID = strings.TrimSpace(projectID)
	modelVersionID = strings.TrimSpace(modelVersionID)
	if projectID == "" {
		return fmt.Errorf("project id is required")
	}
	if modelVersionID == "" {
		return fmt.Errorf("model version id is required")
	}
	if len(datasetVersionIDs) == 0 {
		return nil
	}
	createdAt = normalizeTime(createdAt)
	for _, datasetID := range datasetVersionIDs {
		datasetID = strings.TrimSpace(datasetID)
		if datasetID == "" {
			return fmt.Errorf("dataset version id is required")
		}
		if _, err := s.db.ExecContext(ctx, insertModelVersionDatasetQuery, projectID, modelVersionID, datasetID, createdAt); err != nil {
			return fmt.Errorf("insert model version dataset: %w", err)
		}
	}
	return nil
}
