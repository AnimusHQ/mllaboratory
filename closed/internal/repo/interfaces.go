package repo

import (
	"context"
	"errors"
	"time"

	"github.com/animus-labs/animus-go/closed/internal/domain"
)

var ErrInvalidTransition = errors.New("invalid run state transition")

type ProjectFilter struct {
	Name      string
	CreatedBy string
	Limit     int
}

type DatasetFilter struct {
	ProjectID string
	Name      string
	Limit     int
}

type DatasetVersionFilter struct {
	ProjectID string
	DatasetID string
	Limit     int
}

type RunFilter struct {
	ProjectID    string
	ExperimentID string
	Status       string
	Limit        int
}

type ArtifactFilter struct {
	ProjectID string
	Kind      string
	Limit     int
}

type ModelFilter struct {
	ProjectID string
	Name      string
	Status    domain.ModelStatus
	Limit     int
}

type ModelVersionFilter struct {
	ProjectID string
	ModelID   string
	Status    domain.ModelStatus
	Limit     int
}

type RunRecord struct {
	ID             string
	ProjectID      string
	IdempotencyKey string
	Status         string
	PipelineSpec   []byte
	RunSpec        []byte
	SpecHash       string
	CreatedAt      time.Time
}

type RoleBindingSubject struct {
	Type  string
	Value string
}

type RoleBindingRecord struct {
	BindingID    string
	ProjectID    string
	SubjectType  string
	Subject      string
	Role         string
	CreatedAt    time.Time
	CreatedBy    string
	UpdatedAt    time.Time
	UpdatedBy    string
	IntegritySHA string
}

type SessionRecord struct {
	SessionID     string
	Subject       string
	Email         string
	Roles         []string
	Issuer        string
	CreatedAt     time.Time
	ExpiresAt     time.Time
	LastSeenAt    *time.Time
	RevokedAt     *time.Time
	RevokedBy     string
	RevokeReason  string
	IDTokenSHA256 string
	UserAgent     string
	IP            string
	Metadata      domain.Metadata
}

type PlanRecord struct {
	ID        string
	RunID     string
	ProjectID string
	Plan      []byte
	CreatedAt time.Time
}

type StepExecutionRecord struct {
	ID           string
	ProjectID    string
	RunID        string
	StepName     string
	Attempt      int
	Status       string
	StartedAt    time.Time
	FinishedAt   *time.Time
	ErrorCode    string
	ErrorMessage string
	Result       []byte
	SpecHash     string
}

// ProjectRepository manages Projects.
type ProjectRepository interface {
	Create(ctx context.Context, project domain.Project) error
	Get(ctx context.Context, id string) (domain.Project, error)
	List(ctx context.Context, filter ProjectFilter) ([]domain.Project, error)
}

// DatasetRepository manages datasets and immutable versions.
type DatasetRepository interface {
	CreateDataset(ctx context.Context, dataset domain.Dataset) error
	GetDataset(ctx context.Context, projectID, id string) (domain.Dataset, error)
	ListDatasets(ctx context.Context, filter DatasetFilter) ([]domain.Dataset, error)

	CreateDatasetVersion(ctx context.Context, version domain.DatasetVersion) error
	GetDatasetVersion(ctx context.Context, projectID, id string) (domain.DatasetVersion, error)
	ListDatasetVersions(ctx context.Context, filter DatasetVersionFilter) ([]domain.DatasetVersion, error)
	NextDatasetVersionOrdinal(ctx context.Context, projectID, datasetID string) (int64, error)
}

// RunRepository manages run state with immutable identity.
type RunRepository interface {
	CreateRun(ctx context.Context, projectID, idempotencyKey string, pipelineSpecJSON, runSpecJSON []byte, specHash string) (RunRecord, bool, error)
	GetRun(ctx context.Context, projectID, id string) (RunRecord, error)
	UpdateDerivedStatus(ctx context.Context, projectID, runID string, status domain.RunState) (domain.RunState, bool, error)
}

// ArtifactRepository manages project-scoped artifacts.
type ArtifactRepository interface {
	CreateArtifact(ctx context.Context, projectID string, artifact domain.Artifact) (domain.Artifact, error)
	GetArtifact(ctx context.Context, projectID, id string) (domain.Artifact, error)
	ListArtifacts(ctx context.Context, filter ArtifactFilter) ([]domain.Artifact, error)
	UpdateRetention(ctx context.Context, projectID, id string, retentionUntil *time.Time, legalHold bool) error
}

// ModelRepository manages model registry entries.
type ModelRepository interface {
	CreateModel(ctx context.Context, model domain.Model, idempotencyKey string) (domain.Model, bool, error)
	GetModel(ctx context.Context, projectID, id string) (domain.Model, error)
	ListModels(ctx context.Context, filter ModelFilter) ([]domain.Model, error)
	UpdateModelStatus(ctx context.Context, projectID, id string, status domain.ModelStatus) error
}

type ModelVersionRepository interface {
	Create(ctx context.Context, version domain.ModelVersion, idempotencyKey string) (domain.ModelVersion, bool, error)
	Get(ctx context.Context, projectID, versionID string) (domain.ModelVersion, error)
	List(ctx context.Context, filter ModelVersionFilter) ([]domain.ModelVersion, error)
	UpdateStatus(ctx context.Context, projectID, versionID string, status domain.ModelStatus) error
}

type ModelExportRepository interface {
	Create(ctx context.Context, export domain.ModelExport, idempotencyKey string) (domain.ModelExport, bool, error)
}

// PlanRepository stores deterministic execution plans for runs.
type PlanRepository interface {
	UpsertPlan(ctx context.Context, projectID, runID string, planJSON []byte) (PlanRecord, error)
	GetPlan(ctx context.Context, projectID, runID string) (PlanRecord, error)
}

// StepExecutionRepository stores append-only step attempt records.
type StepExecutionRepository interface {
	InsertAttempt(ctx context.Context, record StepExecutionRecord) (StepExecutionRecord, bool, error)
	ListByRun(ctx context.Context, projectID, runID string) ([]StepExecutionRecord, error)
}

type RoleBindingRepository interface {
	Upsert(ctx context.Context, record RoleBindingRecord) (RoleBindingRecord, bool, error)
	ListByProject(ctx context.Context, projectID string) ([]RoleBindingRecord, error)
	ListBySubjects(ctx context.Context, projectID string, subjects []RoleBindingSubject) ([]RoleBindingRecord, error)
	Delete(ctx context.Context, projectID, bindingID string) error
}

type SessionRepository interface {
	Create(ctx context.Context, record SessionRecord) error
	Get(ctx context.Context, sessionID string) (SessionRecord, error)
	ListActiveBySubject(ctx context.Context, subject string, limit int) ([]SessionRecord, error)
	UpdateLastSeen(ctx context.Context, sessionID string, at time.Time) error
	Revoke(ctx context.Context, sessionID, revokedBy, reason string, at time.Time) (bool, error)
	RevokeBySubject(ctx context.Context, subject, revokedBy, reason string, at time.Time) (int, error)
}

// AuditEventAppender ensures append-only audit writes.
type AuditEventAppender interface {
	Append(ctx context.Context, event domain.AuditEvent) (int64, error)
}
