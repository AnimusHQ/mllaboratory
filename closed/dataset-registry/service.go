package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/animus-labs/animus-go/closed/internal/domain"
	"github.com/animus-labs/animus-go/closed/internal/repo"
	"github.com/google/uuid"
)

type auditContext struct {
	Actor     string
	RequestID string
	IP        net.IP
	UserAgent string
	Path      string
	Service   string
}

type datasetService struct {
	projects repo.ProjectRepository
	datasets repo.DatasetRepository
	audit    repo.AuditEventAppender
	now      func() time.Time
}

func newDatasetService(projects repo.ProjectRepository, datasets repo.DatasetRepository, audit repo.AuditEventAppender) *datasetService {
	return &datasetService{
		projects: projects,
		datasets: datasets,
		audit:    audit,
		now:      time.Now,
	}
}

func (s *datasetService) CreateProject(ctx context.Context, projectID string, name string, description string, metadata map[string]any, auditCtx auditContext) (domain.Project, error) {
	if s == nil || s.projects == nil {
		return domain.Project{}, fmt.Errorf("project service not initialized")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return domain.Project{}, fmt.Errorf("project name is required")
	}
	if strings.TrimSpace(projectID) == "" {
		projectID = uuid.NewString()
	}
	if metadata == nil {
		metadata = map[string]any{}
	}
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return domain.Project{}, fmt.Errorf("invalid metadata: %w", err)
	}

	now := s.now().UTC()
	type integrityInput struct {
		ProjectID   string          `json:"project_id"`
		Name        string          `json:"name"`
		Description string          `json:"description,omitempty"`
		Metadata    json.RawMessage `json:"metadata"`
		CreatedAt   time.Time       `json:"created_at"`
		CreatedBy   string          `json:"created_by"`
	}
	integrity, err := integritySHA256(integrityInput{
		ProjectID:   projectID,
		Name:        name,
		Description: description,
		Metadata:    metadataJSON,
		CreatedAt:   now,
		CreatedBy:   auditCtx.Actor,
	})
	if err != nil {
		return domain.Project{}, fmt.Errorf("integrity: %w", err)
	}

	project := domain.Project{
		ID:              projectID,
		Name:            name,
		Description:     strings.TrimSpace(description),
		Metadata:        domain.Metadata(metadata),
		CreatedAt:       now,
		CreatedBy:       auditCtx.Actor,
		IntegritySHA256: integrity,
	}
	if err := s.projects.Create(ctx, project); err != nil {
		return domain.Project{}, err
	}

	if s.audit != nil {
		_, _ = s.audit.Append(ctx, domain.AuditEvent{
			OccurredAt:   now,
			Actor:        auditCtx.Actor,
			Action:       "project.create",
			ResourceType: "project",
			ResourceID:   projectID,
			RequestID:    auditCtx.RequestID,
			IP:           auditCtx.IP,
			UserAgent:    auditCtx.UserAgent,
			Payload: map[string]any{
				"service":      auditCtx.Service,
				"project_id":   projectID,
				"name":         name,
				"description":  description,
				"metadata":     metadata,
				"request_path": auditCtx.Path,
			},
		})
	}
	return project, nil
}

func (s *datasetService) GetProject(ctx context.Context, projectID string) (domain.Project, error) {
	if s == nil || s.projects == nil {
		return domain.Project{}, fmt.Errorf("project service not initialized")
	}
	return s.projects.Get(ctx, projectID)
}

func (s *datasetService) ListProjects(ctx context.Context, createdBy string, limit int) ([]domain.Project, error) {
	if s == nil || s.projects == nil {
		return nil, fmt.Errorf("project service not initialized")
	}
	createdBy = strings.TrimSpace(createdBy)
	if createdBy == "" {
		return nil, fmt.Errorf("created_by is required")
	}
	if limit <= 0 {
		limit = 100
	}
	return s.projects.List(ctx, repo.ProjectFilter{CreatedBy: createdBy, Limit: limit})
}

func (s *datasetService) CreateDataset(ctx context.Context, projectID string, name string, description string, metadata map[string]any, auditCtx auditContext) (domain.Dataset, error) {
	if s == nil || s.datasets == nil {
		return domain.Dataset{}, fmt.Errorf("dataset service not initialized")
	}
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return domain.Dataset{}, fmt.Errorf("project id is required")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return domain.Dataset{}, fmt.Errorf("dataset name is required")
	}
	if metadata == nil {
		metadata = map[string]any{}
	}
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return domain.Dataset{}, fmt.Errorf("invalid metadata: %w", err)
	}

	now := s.now().UTC()
	datasetID := uuid.NewString()

	type integrityInput struct {
		DatasetID   string          `json:"dataset_id"`
		ProjectID   string          `json:"project_id"`
		Name        string          `json:"name"`
		Description string          `json:"description,omitempty"`
		Metadata    json.RawMessage `json:"metadata"`
		CreatedAt   time.Time       `json:"created_at"`
		CreatedBy   string          `json:"created_by"`
	}
	integrity, err := integritySHA256(integrityInput{
		DatasetID:   datasetID,
		ProjectID:   projectID,
		Name:        name,
		Description: description,
		Metadata:    metadataJSON,
		CreatedAt:   now,
		CreatedBy:   auditCtx.Actor,
	})
	if err != nil {
		return domain.Dataset{}, fmt.Errorf("integrity: %w", err)
	}

	dataset := domain.Dataset{
		ID:              datasetID,
		ProjectID:       projectID,
		Name:            name,
		Description:     strings.TrimSpace(description),
		Metadata:        domain.Metadata(metadata),
		CreatedAt:       now,
		CreatedBy:       auditCtx.Actor,
		IntegritySHA256: integrity,
	}
	if err := s.datasets.CreateDataset(ctx, dataset); err != nil {
		return domain.Dataset{}, err
	}
	if s.audit != nil {
		_, _ = s.audit.Append(ctx, domain.AuditEvent{
			OccurredAt:   now,
			Actor:        auditCtx.Actor,
			Action:       "dataset.create",
			ResourceType: "dataset",
			ResourceID:   datasetID,
			RequestID:    auditCtx.RequestID,
			IP:           auditCtx.IP,
			UserAgent:    auditCtx.UserAgent,
			Payload: map[string]any{
				"service":      auditCtx.Service,
				"project_id":   projectID,
				"dataset_id":   datasetID,
				"name":         name,
				"description":  description,
				"metadata":     metadata,
				"request_path": auditCtx.Path,
			},
		})
	}
	return dataset, nil
}

func (s *datasetService) GetDataset(ctx context.Context, projectID, datasetID string) (domain.Dataset, error) {
	if s == nil || s.datasets == nil {
		return domain.Dataset{}, fmt.Errorf("dataset service not initialized")
	}
	return s.datasets.GetDataset(ctx, projectID, datasetID)
}

func (s *datasetService) ListDatasets(ctx context.Context, projectID string, limit int) ([]domain.Dataset, error) {
	if s == nil || s.datasets == nil {
		return nil, fmt.Errorf("dataset service not initialized")
	}
	return s.datasets.ListDatasets(ctx, repo.DatasetFilter{ProjectID: projectID, Limit: limit})
}

func (s *datasetService) NextDatasetVersionOrdinal(ctx context.Context, projectID, datasetID string) (int64, error) {
	if s == nil || s.datasets == nil {
		return 0, fmt.Errorf("dataset service not initialized")
	}
	return s.datasets.NextDatasetVersionOrdinal(ctx, projectID, datasetID)
}

func (s *datasetService) CreateDatasetVersion(ctx context.Context, version domain.DatasetVersion, metadata map[string]any, auditCtx auditContext) (domain.DatasetVersion, error) {
	if s == nil || s.datasets == nil {
		return domain.DatasetVersion{}, fmt.Errorf("dataset service not initialized")
	}
	if metadata == nil {
		metadata = map[string]any{}
	}
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return domain.DatasetVersion{}, fmt.Errorf("invalid metadata: %w", err)
	}

	now := s.now().UTC()
	type integrityInput struct {
		VersionID     string          `json:"version_id"`
		ProjectID     string          `json:"project_id"`
		DatasetID     string          `json:"dataset_id"`
		QualityRuleID string          `json:"quality_rule_id,omitempty"`
		Ordinal       int64           `json:"ordinal"`
		ContentSHA256 string          `json:"content_sha256"`
		ObjectKey     string          `json:"object_key"`
		SizeBytes     int64           `json:"size_bytes,omitempty"`
		Metadata      json.RawMessage `json:"metadata"`
		CreatedAt     time.Time       `json:"created_at"`
		CreatedBy     string          `json:"created_by"`
	}
	integrity, err := integritySHA256(integrityInput{
		VersionID:     version.ID,
		ProjectID:     version.ProjectID,
		DatasetID:     version.DatasetID,
		QualityRuleID: version.QualityRuleID,
		Ordinal:       version.Ordinal,
		ContentSHA256: version.ContentSHA256,
		ObjectKey:     version.ObjectKey,
		SizeBytes:     version.SizeBytes,
		Metadata:      metadataJSON,
		CreatedAt:     now,
		CreatedBy:     auditCtx.Actor,
	})
	if err != nil {
		return domain.DatasetVersion{}, fmt.Errorf("integrity: %w", err)
	}

	version.Metadata = domain.Metadata(metadata)
	version.CreatedAt = now
	version.CreatedBy = auditCtx.Actor
	version.IntegritySHA256 = integrity

	if err := s.datasets.CreateDatasetVersion(ctx, version); err != nil {
		return domain.DatasetVersion{}, err
	}
	if s.audit != nil {
		_, _ = s.audit.Append(ctx, domain.AuditEvent{
			OccurredAt:   now,
			Actor:        auditCtx.Actor,
			Action:       "dataset_version.create",
			ResourceType: "dataset_version",
			ResourceID:   version.ID,
			RequestID:    auditCtx.RequestID,
			IP:           auditCtx.IP,
			UserAgent:    auditCtx.UserAgent,
			Payload: map[string]any{
				"service":            auditCtx.Service,
				"project_id":         version.ProjectID,
				"dataset_id":         version.DatasetID,
				"dataset_version_id": version.ID,
				"quality_rule_id":    version.QualityRuleID,
				"ordinal":            version.Ordinal,
				"content_sha256":     version.ContentSHA256,
				"object_key":         version.ObjectKey,
				"size_bytes":         version.SizeBytes,
				"metadata":           metadata,
				"request_path":       auditCtx.Path,
			},
		})
	}
	return version, nil
}

func (s *datasetService) GetDatasetVersion(ctx context.Context, projectID, versionID string) (domain.DatasetVersion, error) {
	if s == nil || s.datasets == nil {
		return domain.DatasetVersion{}, fmt.Errorf("dataset service not initialized")
	}
	return s.datasets.GetDatasetVersion(ctx, projectID, versionID)
}

func (s *datasetService) ListDatasetVersions(ctx context.Context, projectID, datasetID string, limit int) ([]domain.DatasetVersion, error) {
	if s == nil || s.datasets == nil {
		return nil, fmt.Errorf("dataset service not initialized")
	}
	return s.datasets.ListDatasetVersions(ctx, repo.DatasetVersionFilter{ProjectID: projectID, DatasetID: datasetID, Limit: limit})
}
