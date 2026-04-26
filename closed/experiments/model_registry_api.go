package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/animus-labs/animus-go/closed/internal/domain"
	"github.com/animus-labs/animus-go/closed/internal/platform/auditlog"
	"github.com/animus-labs/animus-go/closed/internal/platform/auth"
	"github.com/animus-labs/animus-go/closed/internal/platform/lineageevent"
	"github.com/animus-labs/animus-go/closed/internal/repo"
	"github.com/animus-labs/animus-go/closed/internal/repo/postgres"
	"github.com/google/uuid"
)

const (
	auditModelCreated           = "model.created"
	auditModelVersionCreated    = "model.version.created"
	auditModelVersionValidated  = "model.version.validated"
	auditModelVersionApproved   = "model.version.approved"
	auditModelVersionDeprecated = "model.version.deprecated"
	auditModelExportRequested   = "model.export.requested"
)

type modelCreateRequest struct {
	IdempotencyKey string         `json:"idempotencyKey"`
	Name           string         `json:"name"`
	Metadata       map[string]any `json:"metadata,omitempty"`
}

type modelCreateResponse struct {
	Model   domain.Model `json:"model"`
	Created bool         `json:"created"`
}

type modelListResponse struct {
	Models []domain.Model `json:"models"`
}

type modelVersionCreateRequest struct {
	IdempotencyKey    string   `json:"idempotencyKey"`
	Version           string   `json:"version"`
	RunID             string   `json:"runId"`
	ArtifactIDs       []string `json:"artifactIds"`
	DatasetVersionIDs []string `json:"datasetVersionIds,omitempty"`
}

type modelVersionCreateResponse struct {
	ModelVersion domain.ModelVersion `json:"modelVersion"`
	Created      bool                `json:"created"`
}

type modelVersionListResponse struct {
	ModelVersions []domain.ModelVersion `json:"modelVersions"`
}

type modelVersionProvenanceResponse struct {
	ModelVersionID    string         `json:"modelVersionId"`
	ProjectID         string         `json:"projectId"`
	ModelID           string         `json:"modelId"`
	Version           string         `json:"version"`
	RunID             string         `json:"runId"`
	ArtifactIDs       []string       `json:"artifactIds"`
	DatasetVersionIDs []string       `json:"datasetVersionIds,omitempty"`
	EnvLockID         string         `json:"envLockId,omitempty"`
	CodeRef           domain.CodeRef `json:"codeRef,omitempty"`
	PolicySnapshotSHA string         `json:"policySnapshotSha256,omitempty"`
}

type modelVersionTransitionResponse struct {
	ModelVersion domain.ModelVersion `json:"modelVersion"`
}

type modelExportRequest struct {
	IdempotencyKey string `json:"idempotencyKey"`
	Target         string `json:"target,omitempty"`
}

type modelExportResponse struct {
	Export  domain.ModelExport `json:"export"`
	Created bool               `json:"created"`
}

type modelStore interface {
	CreateModel(ctx context.Context, model domain.Model, idempotencyKey string) (domain.Model, bool, error)
	GetModel(ctx context.Context, projectID, id string) (domain.Model, error)
	ListModels(ctx context.Context, filter repo.ModelFilter) ([]domain.Model, error)
	UpdateModelStatus(ctx context.Context, projectID, id string, status domain.ModelStatus) error
}

type modelVersionStore interface {
	Create(ctx context.Context, version domain.ModelVersion, idempotencyKey string) (domain.ModelVersion, bool, error)
	Get(ctx context.Context, projectID, versionID string) (domain.ModelVersion, error)
	List(ctx context.Context, filter repo.ModelVersionFilter) ([]domain.ModelVersion, error)
	UpdateStatus(ctx context.Context, projectID, versionID string, status domain.ModelStatus) error
}

type modelVersionTransitionStore interface {
	Insert(ctx context.Context, transition domain.ModelVersionTransition) error
}

type modelVersionProvenanceStore interface {
	InsertArtifacts(ctx context.Context, projectID, modelVersionID string, artifactIDs []string, createdAt time.Time) error
	InsertDatasets(ctx context.Context, projectID, modelVersionID string, datasetVersionIDs []string, createdAt time.Time) error
}

type modelExportStore interface {
	Create(ctx context.Context, export domain.ModelExport, idempotencyKey string) (domain.ModelExport, bool, error)
}

type runBindingsStore interface {
	GetCodeRef(ctx context.Context, projectID, runID string) (domain.CodeRef, error)
	GetEnvLock(ctx context.Context, projectID, runID string) (domain.EnvLock, error)
	PolicySnapshotSHA(ctx context.Context, projectID, runID string) (string, error)
}

type runSpecStore interface {
	GetRun(ctx context.Context, projectID, id string) (repo.RunRecord, error)
}

type modelAuditAppender interface {
	Append(ctx context.Context, event auditlog.Event) error
}

type modelLineageAppender interface {
	Append(ctx context.Context, event lineageevent.Event) error
}

func (api *experimentsAPI) modelStore() modelStore {
	if api == nil {
		return nil
	}
	if api.modelStoreOverride != nil {
		return api.modelStoreOverride
	}
	return postgres.NewModelStore(api.db)
}

func (api *experimentsAPI) modelVersionStore() modelVersionStore {
	if api == nil {
		return nil
	}
	if api.modelVersionStoreOverride != nil {
		return api.modelVersionStoreOverride
	}
	return postgres.NewModelVersionStore(api.db)
}

func (api *experimentsAPI) modelVersionTransitionStore() modelVersionTransitionStore {
	if api == nil {
		return nil
	}
	if api.modelVersionTransitionOverride != nil {
		return api.modelVersionTransitionOverride
	}
	return postgres.NewModelVersionTransitionStore(api.db)
}

func (api *experimentsAPI) modelVersionProvenanceStore() modelVersionProvenanceStore {
	if api == nil {
		return nil
	}
	if api.modelVersionProvenanceOverride != nil {
		return api.modelVersionProvenanceOverride
	}
	return postgres.NewModelVersionProvenanceStore(api.db)
}

func (api *experimentsAPI) modelExportStore() modelExportStore {
	if api == nil {
		return nil
	}
	if api.modelExportStoreOverride != nil {
		return api.modelExportStoreOverride
	}
	return postgres.NewModelExportStore(api.db)
}

func (api *experimentsAPI) modelRunBindingsStore() runBindingsStore {
	if api == nil {
		return nil
	}
	if api.modelRunBindingsOverride != nil {
		return api.modelRunBindingsOverride
	}
	return postgres.NewRunBindingsStore(api.db)
}

func (api *experimentsAPI) modelRunSpecStore() runSpecStore {
	if api == nil {
		return nil
	}
	if api.modelRunSpecOverride != nil {
		return api.modelRunSpecOverride
	}
	return postgres.NewRunSpecStore(api.db)
}

func (api *experimentsAPI) appendModelAudit(ctx context.Context, q auditlog.QueryRower, event auditlog.Event) error {
	if api == nil {
		return errors.New("api not initialized")
	}
	if api.modelAuditOverride != nil {
		return api.modelAuditOverride.Append(ctx, event)
	}
	if q == nil {
		return errors.New("audit appender required")
	}
	_, err := auditlog.Insert(ctx, q, event)
	return err
}

func (api *experimentsAPI) appendModelLineage(ctx context.Context, q lineageevent.QueryRower, event lineageevent.Event) error {
	if api == nil {
		return errors.New("api not initialized")
	}
	if api.modelLineageOverride != nil {
		return api.modelLineageOverride.Append(ctx, event)
	}
	if q == nil {
		return errors.New("lineage appender required")
	}
	_, err := lineageevent.Insert(ctx, q, event)
	return err
}

func (api *experimentsAPI) handleCreateModel(w http.ResponseWriter, r *http.Request) {
	identity, ok := auth.IdentityFromContext(r.Context())
	if !ok || strings.TrimSpace(identity.Subject) == "" {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	projectID := strings.TrimSpace(r.PathValue("project_id"))
	if projectID == "" {
		api.writeError(w, r, http.StatusBadRequest, "project_id_required")
		return
	}

	var req modelCreateRequest
	if err := decodeJSON(r, &req); err != nil {
		api.writeError(w, r, http.StatusBadRequest, "invalid_json")
		return
	}
	idempotencyKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if idempotencyKey == "" {
		idempotencyKey = strings.TrimSpace(req.IdempotencyKey)
	}
	if idempotencyKey == "" {
		api.writeError(w, r, http.StatusBadRequest, "idempotency_key_required")
		return
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		api.writeError(w, r, http.StatusBadRequest, "name_required")
		return
	}
	metadata := req.Metadata
	if metadata == nil {
		metadata = map[string]any{}
	}

	now := time.Now().UTC()
	model := domain.Model{
		ID:        uuid.NewString(),
		ProjectID: projectID,
		Name:      name,
		Version:   "v1",
		Status:    domain.ModelStatusDraft,
		Metadata:  metadata,
		CreatedAt: now,
		CreatedBy: strings.TrimSpace(identity.Subject),
	}
	integrity, err := modelIntegrity(model)
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "integrity_failed")
		return
	}
	model.IntegritySHA256 = integrity

	store := api.modelStore()
	if store == nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	if api.modelStoreOverride != nil || api.modelAuditOverride != nil {
		record, created, err := store.CreateModel(r.Context(), model, idempotencyKey)
		if err != nil {
			api.writeError(w, r, http.StatusInternalServerError, "internal_error")
			return
		}
		if !created && record.IntegritySHA256 != integrity {
			api.writeError(w, r, http.StatusConflict, "idempotency_conflict")
			return
		}
		if created {
			if err := api.appendModelAudit(r.Context(), nil, auditlog.Event{
				OccurredAt:   now,
				Actor:        identity.Subject,
				Action:       auditModelCreated,
				ResourceType: "model",
				ResourceID:   record.ID,
				RequestID:    r.Header.Get("X-Request-Id"),
				IP:           requestIP(r.RemoteAddr),
				UserAgent:    r.UserAgent(),
				Payload: map[string]any{
					"service":    "experiments",
					"project_id": projectID,
					"model_id":   record.ID,
					"name":       record.Name,
				},
			}); err != nil {
				api.writeError(w, r, http.StatusInternalServerError, "audit_failed")
				return
			}
		}
		api.writeJSON(w, http.StatusOK, modelCreateResponse{Model: record, Created: created})
		return
	}

	tx, err := api.db.BeginTx(r.Context(), nil)
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	defer func() { _ = tx.Rollback() }()

	txStore := postgres.NewModelStore(tx)
	record, created, err := txStore.CreateModel(r.Context(), model, idempotencyKey)
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	if !created && record.IntegritySHA256 != integrity {
		api.writeError(w, r, http.StatusConflict, "idempotency_conflict")
		return
	}
	if created {
		if err := api.appendModelAudit(r.Context(), tx, auditlog.Event{
			OccurredAt:   now,
			Actor:        identity.Subject,
			Action:       auditModelCreated,
			ResourceType: "model",
			ResourceID:   record.ID,
			RequestID:    r.Header.Get("X-Request-Id"),
			IP:           requestIP(r.RemoteAddr),
			UserAgent:    r.UserAgent(),
			Payload: map[string]any{
				"service":    "experiments",
				"project_id": projectID,
				"model_id":   record.ID,
				"name":       record.Name,
			},
		}); err != nil {
			api.writeError(w, r, http.StatusInternalServerError, "audit_failed")
			return
		}
	}
	if err := tx.Commit(); err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	api.writeJSON(w, http.StatusOK, modelCreateResponse{Model: record, Created: created})
}

func (api *experimentsAPI) handleListModels(w http.ResponseWriter, r *http.Request) {
	projectID := strings.TrimSpace(r.PathValue("project_id"))
	if projectID == "" {
		api.writeError(w, r, http.StatusBadRequest, "project_id_required")
		return
	}
	name := strings.TrimSpace(r.URL.Query().Get("name"))
	limit := clampInt(parseIntQuery(r, "limit", 100), 1, 500)

	store := api.modelStore()
	if store == nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	models, err := store.ListModels(r.Context(), repo.ModelFilter{ProjectID: projectID, Name: name, Limit: limit})
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	api.writeJSON(w, http.StatusOK, modelListResponse{Models: models})
}

func (api *experimentsAPI) handleGetModel(w http.ResponseWriter, r *http.Request) {
	projectID := strings.TrimSpace(r.PathValue("project_id"))
	modelID := strings.TrimSpace(r.PathValue("model_id"))
	if projectID == "" {
		api.writeError(w, r, http.StatusBadRequest, "project_id_required")
		return
	}
	if modelID == "" {
		api.writeError(w, r, http.StatusBadRequest, "model_id_required")
		return
	}
	store := api.modelStore()
	if store == nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	model, err := store.GetModel(r.Context(), projectID, modelID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			api.writeError(w, r, http.StatusNotFound, "not_found")
			return
		}
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	api.writeJSON(w, http.StatusOK, model)
}

func (api *experimentsAPI) handleListModelVersions(w http.ResponseWriter, r *http.Request) {
	projectID := strings.TrimSpace(r.PathValue("project_id"))
	modelID := strings.TrimSpace(r.PathValue("model_id"))
	if projectID == "" {
		api.writeError(w, r, http.StatusBadRequest, "project_id_required")
		return
	}
	if modelID == "" {
		api.writeError(w, r, http.StatusBadRequest, "model_id_required")
		return
	}
	limit := clampInt(parseIntQuery(r, "limit", 100), 1, 500)

	store := api.modelVersionStore()
	if store == nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	versions, err := store.List(r.Context(), repo.ModelVersionFilter{ProjectID: projectID, ModelID: modelID, Limit: limit})
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	api.writeJSON(w, http.StatusOK, modelVersionListResponse{ModelVersions: versions})
}

func (api *experimentsAPI) handleCreateModelVersion(w http.ResponseWriter, r *http.Request) {
	identity, ok := auth.IdentityFromContext(r.Context())
	if !ok || strings.TrimSpace(identity.Subject) == "" {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	projectID := strings.TrimSpace(r.PathValue("project_id"))
	modelID := strings.TrimSpace(r.PathValue("model_id"))
	if projectID == "" {
		api.writeError(w, r, http.StatusBadRequest, "project_id_required")
		return
	}
	if modelID == "" {
		api.writeError(w, r, http.StatusBadRequest, "model_id_required")
		return
	}
	var req modelVersionCreateRequest
	if err := decodeJSON(r, &req); err != nil {
		api.writeError(w, r, http.StatusBadRequest, "invalid_json")
		return
	}
	idempotencyKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if idempotencyKey == "" {
		idempotencyKey = strings.TrimSpace(req.IdempotencyKey)
	}
	if idempotencyKey == "" {
		api.writeError(w, r, http.StatusBadRequest, "idempotency_key_required")
		return
	}
	versionLabel := strings.TrimSpace(req.Version)
	if versionLabel == "" {
		api.writeError(w, r, http.StatusBadRequest, "version_required")
		return
	}
	runID := strings.TrimSpace(req.RunID)
	if runID == "" {
		api.writeError(w, r, http.StatusBadRequest, "run_id_required")
		return
	}
	artifactIDs := normalizeStringList(req.ArtifactIDs)
	if len(artifactIDs) == 0 {
		api.writeError(w, r, http.StatusBadRequest, "artifact_ids_required")
		return
	}

	modelStore := api.modelStore()
	if modelStore == nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	if _, err := modelStore.GetModel(r.Context(), projectID, modelID); err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			api.writeError(w, r, http.StatusNotFound, "not_found")
			return
		}
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	datasetIDs := normalizeStringList(req.DatasetVersionIDs)
	if len(datasetIDs) == 0 {
		resolved, err := api.resolveDatasetVersionsFromRun(r.Context(), projectID, runID)
		if err != nil {
			api.writeError(w, r, http.StatusInternalServerError, "run_spec_unavailable")
			return
		}
		datasetIDs = resolved
	}

	bindings := api.modelRunBindingsStore()
	if bindings == nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	codeRef, err := bindings.GetCodeRef(r.Context(), projectID, runID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			api.writeError(w, r, http.StatusNotFound, "not_found")
			return
		}
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	envLock, err := bindings.GetEnvLock(r.Context(), projectID, runID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			api.writeError(w, r, http.StatusNotFound, "not_found")
			return
		}
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	policySHA, err := bindings.PolicySnapshotSHA(r.Context(), projectID, runID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			api.writeError(w, r, http.StatusNotFound, "not_found")
			return
		}
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	now := time.Now().UTC()
	modelVersion := domain.ModelVersion{
		ID:                   uuid.NewString(),
		ProjectID:            projectID,
		ModelID:              modelID,
		Version:              versionLabel,
		Status:               domain.ModelStatusDraft,
		RunID:                runID,
		ArtifactIDs:          artifactIDs,
		DatasetVersionIDs:    datasetIDs,
		EnvLockID:            envLock.LockID,
		CodeRef:              codeRef,
		PolicySnapshotSHA256: policySHA,
		CreatedAt:            now,
		CreatedBy:            strings.TrimSpace(identity.Subject),
	}
	integrity, err := modelVersionIntegrity(modelVersion)
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "integrity_failed")
		return
	}
	modelVersion.IntegritySHA256 = integrity

	versionStore := api.modelVersionStore()
	transitionStore := api.modelVersionTransitionStore()
	provenanceStore := api.modelVersionProvenanceStore()
	if versionStore == nil || transitionStore == nil || provenanceStore == nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	if api.modelVersionStoreOverride != nil || api.modelVersionTransitionOverride != nil || api.modelAuditOverride != nil || api.modelLineageOverride != nil || api.modelVersionProvenanceOverride != nil {
		record, created, err := versionStore.Create(r.Context(), modelVersion, idempotencyKey)
		if err != nil {
			api.writeError(w, r, http.StatusInternalServerError, "internal_error")
			return
		}
		if !created && record.IntegritySHA256 != integrity {
			api.writeError(w, r, http.StatusConflict, "idempotency_conflict")
			return
		}
		if created {
			if err := provenanceStore.InsertArtifacts(r.Context(), projectID, record.ID, artifactIDs, record.CreatedAt); err != nil {
				api.writeError(w, r, http.StatusInternalServerError, "internal_error")
				return
			}
			if err := provenanceStore.InsertDatasets(r.Context(), projectID, record.ID, datasetIDs, record.CreatedAt); err != nil {
				api.writeError(w, r, http.StatusInternalServerError, "internal_error")
				return
			}
			_ = transitionStore.Insert(r.Context(), domain.ModelVersionTransition{
				ProjectID:      projectID,
				ModelVersionID: record.ID,
				FromStatus:     domain.ModelStatusDraft,
				ToStatus:       domain.ModelStatusDraft,
				Action:         "create",
				RequestID:      r.Header.Get("X-Request-Id"),
				OccurredAt:     now,
				Actor:          identity.Subject,
			})
			_ = api.appendModelLineage(r.Context(), nil, lineageevent.Event{
				OccurredAt:  now,
				Actor:       identity.Subject,
				RequestID:   r.Header.Get("X-Request-Id"),
				SubjectType: "experiment_run",
				SubjectID:   runID,
				Predicate:   "produced",
				ObjectType:  "model_version",
				ObjectID:    record.ID,
			})
			for _, artifactID := range artifactIDs {
				_ = api.appendModelLineage(r.Context(), nil, lineageevent.Event{
					OccurredAt:  now,
					Actor:       identity.Subject,
					RequestID:   r.Header.Get("X-Request-Id"),
					SubjectType: "model_version",
					SubjectID:   record.ID,
					Predicate:   "derived_from",
					ObjectType:  "artifact",
					ObjectID:    artifactID,
				})
			}
			for _, datasetID := range datasetIDs {
				_ = api.appendModelLineage(r.Context(), nil, lineageevent.Event{
					OccurredAt:  now,
					Actor:       identity.Subject,
					RequestID:   r.Header.Get("X-Request-Id"),
					SubjectType: "model_version",
					SubjectID:   record.ID,
					Predicate:   "trained_on",
					ObjectType:  "dataset_version",
					ObjectID:    datasetID,
				})
			}
			_ = api.appendModelAudit(r.Context(), nil, auditlog.Event{
				OccurredAt:   now,
				Actor:        identity.Subject,
				Action:       auditModelVersionCreated,
				ResourceType: "model_version",
				ResourceID:   record.ID,
				RequestID:    r.Header.Get("X-Request-Id"),
				IP:           requestIP(r.RemoteAddr),
				UserAgent:    r.UserAgent(),
				Payload: map[string]any{
					"service":       "experiments",
					"project_id":    projectID,
					"model_id":      modelID,
					"model_version": record.Version,
					"run_id":        runID,
				},
			})
		}
		api.writeJSON(w, http.StatusOK, modelVersionCreateResponse{ModelVersion: record, Created: created})
		return
	}

	tx, err := api.db.BeginTx(r.Context(), nil)
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	defer func() { _ = tx.Rollback() }()

	txVersionStore := postgres.NewModelVersionStore(tx)
	txTransitionStore := postgres.NewModelVersionTransitionStore(tx)
	txProvenanceStore := postgres.NewModelVersionProvenanceStore(tx)
	if txProvenanceStore == nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	record, created, err := txVersionStore.Create(r.Context(), modelVersion, idempotencyKey)
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	if !created && record.IntegritySHA256 != integrity {
		api.writeError(w, r, http.StatusConflict, "idempotency_conflict")
		return
	}
	if created {
		if err := txProvenanceStore.InsertArtifacts(r.Context(), projectID, record.ID, artifactIDs, record.CreatedAt); err != nil {
			api.writeError(w, r, http.StatusInternalServerError, "internal_error")
			return
		}
		if err := txProvenanceStore.InsertDatasets(r.Context(), projectID, record.ID, datasetIDs, record.CreatedAt); err != nil {
			api.writeError(w, r, http.StatusInternalServerError, "internal_error")
			return
		}
		_ = txTransitionStore.Insert(r.Context(), domain.ModelVersionTransition{
			ProjectID:      projectID,
			ModelVersionID: record.ID,
			FromStatus:     domain.ModelStatusDraft,
			ToStatus:       domain.ModelStatusDraft,
			Action:         "create",
			RequestID:      r.Header.Get("X-Request-Id"),
			OccurredAt:     now,
			Actor:          identity.Subject,
		})
		_ = api.appendModelLineage(r.Context(), tx, lineageevent.Event{
			OccurredAt:  now,
			Actor:       identity.Subject,
			RequestID:   r.Header.Get("X-Request-Id"),
			SubjectType: "experiment_run",
			SubjectID:   runID,
			Predicate:   "produced",
			ObjectType:  "model_version",
			ObjectID:    record.ID,
		})
		for _, artifactID := range artifactIDs {
			_ = api.appendModelLineage(r.Context(), tx, lineageevent.Event{
				OccurredAt:  now,
				Actor:       identity.Subject,
				RequestID:   r.Header.Get("X-Request-Id"),
				SubjectType: "model_version",
				SubjectID:   record.ID,
				Predicate:   "derived_from",
				ObjectType:  "artifact",
				ObjectID:    artifactID,
			})
		}
		for _, datasetID := range datasetIDs {
			_ = api.appendModelLineage(r.Context(), tx, lineageevent.Event{
				OccurredAt:  now,
				Actor:       identity.Subject,
				RequestID:   r.Header.Get("X-Request-Id"),
				SubjectType: "model_version",
				SubjectID:   record.ID,
				Predicate:   "trained_on",
				ObjectType:  "dataset_version",
				ObjectID:    datasetID,
			})
		}
		if err := api.appendModelAudit(r.Context(), tx, auditlog.Event{
			OccurredAt:   now,
			Actor:        identity.Subject,
			Action:       auditModelVersionCreated,
			ResourceType: "model_version",
			ResourceID:   record.ID,
			RequestID:    r.Header.Get("X-Request-Id"),
			IP:           requestIP(r.RemoteAddr),
			UserAgent:    r.UserAgent(),
			Payload: map[string]any{
				"service":       "experiments",
				"project_id":    projectID,
				"model_id":      modelID,
				"model_version": record.Version,
				"run_id":        runID,
			},
		}); err != nil {
			api.writeError(w, r, http.StatusInternalServerError, "audit_failed")
			return
		}
	}
	if err := tx.Commit(); err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	api.writeJSON(w, http.StatusOK, modelVersionCreateResponse{ModelVersion: record, Created: created})
}

func (api *experimentsAPI) handleGetModelVersion(w http.ResponseWriter, r *http.Request) {
	projectID := strings.TrimSpace(r.PathValue("project_id"))
	versionID := strings.TrimSpace(r.PathValue("model_version_id"))
	if projectID == "" {
		api.writeError(w, r, http.StatusBadRequest, "project_id_required")
		return
	}
	if versionID == "" {
		api.writeError(w, r, http.StatusBadRequest, "model_version_id_required")
		return
	}
	store := api.modelVersionStore()
	if store == nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	version, err := store.Get(r.Context(), projectID, versionID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			api.writeError(w, r, http.StatusNotFound, "not_found")
			return
		}
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	api.writeJSON(w, http.StatusOK, version)
}

func (api *experimentsAPI) handleGetModelVersionProvenance(w http.ResponseWriter, r *http.Request) {
	projectID := strings.TrimSpace(r.PathValue("project_id"))
	versionID := strings.TrimSpace(r.PathValue("model_version_id"))
	if projectID == "" {
		api.writeError(w, r, http.StatusBadRequest, "project_id_required")
		return
	}
	if versionID == "" {
		api.writeError(w, r, http.StatusBadRequest, "model_version_id_required")
		return
	}
	store := api.modelVersionStore()
	if store == nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	version, err := store.Get(r.Context(), projectID, versionID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			api.writeError(w, r, http.StatusNotFound, "not_found")
			return
		}
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	api.writeJSON(w, http.StatusOK, modelVersionProvenanceResponse{
		ModelVersionID:    version.ID,
		ProjectID:         version.ProjectID,
		ModelID:           version.ModelID,
		Version:           version.Version,
		RunID:             version.RunID,
		ArtifactIDs:       version.ArtifactIDs,
		DatasetVersionIDs: version.DatasetVersionIDs,
		EnvLockID:         version.EnvLockID,
		CodeRef:           version.CodeRef,
		PolicySnapshotSHA: version.PolicySnapshotSHA256,
	})
}

func (api *experimentsAPI) handleValidateModelVersion(w http.ResponseWriter, r *http.Request) {
	api.handleModelVersionTransition(w, r, domain.ModelStatusValidated, auditModelVersionValidated, "validate", false)
}

func (api *experimentsAPI) handleApproveModelVersion(w http.ResponseWriter, r *http.Request) {
	api.handleModelVersionTransition(w, r, domain.ModelStatusApproved, auditModelVersionApproved, "approve", true)
}

func (api *experimentsAPI) handleDeprecateModelVersion(w http.ResponseWriter, r *http.Request) {
	api.handleModelVersionTransition(w, r, domain.ModelStatusDeprecated, auditModelVersionDeprecated, "deprecate", true)
}

func (api *experimentsAPI) handleModelVersionTransition(w http.ResponseWriter, r *http.Request, target domain.ModelStatus, auditAction, action string, enqueueWebhook bool) {
	identity, ok := auth.IdentityFromContext(r.Context())
	if !ok || strings.TrimSpace(identity.Subject) == "" {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	projectID := strings.TrimSpace(r.PathValue("project_id"))
	versionID := strings.TrimSpace(r.PathValue("model_version_id"))
	if projectID == "" {
		api.writeError(w, r, http.StatusBadRequest, "project_id_required")
		return
	}
	if versionID == "" {
		api.writeError(w, r, http.StatusBadRequest, "model_version_id_required")
		return
	}
	store := api.modelVersionStore()
	transitionStore := api.modelVersionTransitionStore()
	if store == nil || transitionStore == nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	current, err := store.Get(r.Context(), projectID, versionID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			api.writeError(w, r, http.StatusNotFound, "not_found")
			return
		}
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	if err := domain.ValidateTransition(current.Status, target); err != nil {
		api.writeError(w, r, http.StatusConflict, "invalid_transition")
		return
	}

	now := time.Now().UTC()
	if api.modelVersionStoreOverride != nil || api.modelVersionTransitionOverride != nil || api.modelAuditOverride != nil {
		if err := store.UpdateStatus(r.Context(), projectID, versionID, target); err != nil {
			if errors.Is(err, repo.ErrNotFound) {
				api.writeError(w, r, http.StatusNotFound, "not_found")
				return
			}
			api.writeError(w, r, http.StatusInternalServerError, "internal_error")
			return
		}
		_ = transitionStore.Insert(r.Context(), domain.ModelVersionTransition{
			ProjectID:      projectID,
			ModelVersionID: versionID,
			FromStatus:     current.Status,
			ToStatus:       target,
			Action:         action,
			RequestID:      r.Header.Get("X-Request-Id"),
			OccurredAt:     now,
			Actor:          identity.Subject,
		})
		if err := api.appendModelAudit(r.Context(), nil, auditlog.Event{
			OccurredAt:   now,
			Actor:        identity.Subject,
			Action:       auditAction,
			ResourceType: "model_version",
			ResourceID:   versionID,
			RequestID:    r.Header.Get("X-Request-Id"),
			IP:           requestIP(r.RemoteAddr),
			UserAgent:    r.UserAgent(),
			Payload: map[string]any{
				"service":     "experiments",
				"project_id":  projectID,
				"model_id":    current.ModelID,
				"from_status": string(current.Status),
				"to_status":   string(target),
			},
		}); err != nil {
			api.writeError(w, r, http.StatusInternalServerError, "audit_failed")
			return
		}
		current.Status = target
		if enqueueWebhook {
			_ = api.enqueueWebhookModelApproved(r.Context(), identity.Subject, r.Header.Get("X-Request-Id"), projectID, current.ID, now)
		}
		api.writeJSON(w, http.StatusOK, modelVersionTransitionResponse{ModelVersion: current})
		return
	}

	tx, err := api.db.BeginTx(r.Context(), nil)
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	defer func() { _ = tx.Rollback() }()

	txStore := postgres.NewModelVersionStore(tx)
	txTransitionStore := postgres.NewModelVersionTransitionStore(tx)
	if err := txStore.UpdateStatus(r.Context(), projectID, versionID, target); err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			api.writeError(w, r, http.StatusNotFound, "not_found")
			return
		}
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	_ = txTransitionStore.Insert(r.Context(), domain.ModelVersionTransition{
		ProjectID:      projectID,
		ModelVersionID: versionID,
		FromStatus:     current.Status,
		ToStatus:       target,
		Action:         action,
		RequestID:      r.Header.Get("X-Request-Id"),
		OccurredAt:     now,
		Actor:          identity.Subject,
	})
	if err := api.appendModelAudit(r.Context(), tx, auditlog.Event{
		OccurredAt:   now,
		Actor:        identity.Subject,
		Action:       auditAction,
		ResourceType: "model_version",
		ResourceID:   versionID,
		RequestID:    r.Header.Get("X-Request-Id"),
		IP:           requestIP(r.RemoteAddr),
		UserAgent:    r.UserAgent(),
		Payload: map[string]any{
			"service":     "experiments",
			"project_id":  projectID,
			"model_id":    current.ModelID,
			"from_status": string(current.Status),
			"to_status":   string(target),
		},
	}); err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "audit_failed")
		return
	}
	if err := tx.Commit(); err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	current.Status = target
	if enqueueWebhook {
		_ = api.enqueueWebhookModelApproved(r.Context(), identity.Subject, r.Header.Get("X-Request-Id"), projectID, current.ID, now)
	}
	api.writeJSON(w, http.StatusOK, modelVersionTransitionResponse{ModelVersion: current})
}

func (api *experimentsAPI) handleExportModelVersion(w http.ResponseWriter, r *http.Request) {
	identity, ok := auth.IdentityFromContext(r.Context())
	if !ok || strings.TrimSpace(identity.Subject) == "" {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	projectID := strings.TrimSpace(r.PathValue("project_id"))
	versionID := strings.TrimSpace(r.PathValue("model_version_id"))
	if projectID == "" {
		api.writeError(w, r, http.StatusBadRequest, "project_id_required")
		return
	}
	if versionID == "" {
		api.writeError(w, r, http.StatusBadRequest, "model_version_id_required")
		return
	}
	var req modelExportRequest
	if err := decodeJSON(r, &req); err != nil && !errors.Is(err, io.EOF) {
		api.writeError(w, r, http.StatusBadRequest, "invalid_json")
		return
	}
	idempotencyKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if idempotencyKey == "" {
		idempotencyKey = strings.TrimSpace(req.IdempotencyKey)
	}
	if idempotencyKey == "" {
		api.writeError(w, r, http.StatusBadRequest, "idempotency_key_required")
		return
	}

	versionStore := api.modelVersionStore()
	exportStore := api.modelExportStore()
	if versionStore == nil || exportStore == nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	version, err := versionStore.Get(r.Context(), projectID, versionID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			api.writeError(w, r, http.StatusNotFound, "not_found")
			return
		}
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	if version.Status != domain.ModelStatusApproved {
		api.writeError(w, r, http.StatusConflict, "model_version_not_approved")
		return
	}
	policyDecision, err := api.checkModelExportPolicy(r.Context(), projectID, version.RunID)
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	if !policyDecision.Allowed {
		api.writeError(w, r, http.StatusForbidden, policyDecision.Code)
		return
	}

	now := time.Now().UTC()
	export := domain.ModelExport{
		ExportID:       uuid.NewString(),
		ProjectID:      projectID,
		ModelVersionID: versionID,
		Status:         "requested",
		Target:         strings.TrimSpace(req.Target),
		CreatedAt:      now,
		CreatedBy:      strings.TrimSpace(identity.Subject),
	}
	integrity, err := modelExportIntegrity(export)
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "integrity_failed")
		return
	}
	export.IntegritySHA256 = integrity

	if api.modelExportStoreOverride != nil || api.modelAuditOverride != nil {
		record, created, err := exportStore.Create(r.Context(), export, idempotencyKey)
		if err != nil {
			api.writeError(w, r, http.StatusInternalServerError, "internal_error")
			return
		}
		if !created && record.IntegritySHA256 != integrity {
			api.writeError(w, r, http.StatusConflict, "idempotency_conflict")
			return
		}
		if created {
			if err := api.appendModelAudit(r.Context(), nil, auditlog.Event{
				OccurredAt:   now,
				Actor:        identity.Subject,
				Action:       auditModelExportRequested,
				ResourceType: "model_export",
				ResourceID:   record.ExportID,
				RequestID:    r.Header.Get("X-Request-Id"),
				IP:           requestIP(r.RemoteAddr),
				UserAgent:    r.UserAgent(),
				Payload: map[string]any{
					"service":          "experiments",
					"project_id":       projectID,
					"model_version_id": versionID,
					"status":           record.Status,
				},
			}); err != nil {
				api.writeError(w, r, http.StatusInternalServerError, "audit_failed")
				return
			}
		}
		api.writeJSON(w, http.StatusOK, modelExportResponse{Export: record, Created: created})
		return
	}

	tx, err := api.db.BeginTx(r.Context(), nil)
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	defer func() { _ = tx.Rollback() }()

	txExportStore := postgres.NewModelExportStore(tx)
	record, created, err := txExportStore.Create(r.Context(), export, idempotencyKey)
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	if !created && record.IntegritySHA256 != integrity {
		api.writeError(w, r, http.StatusConflict, "idempotency_conflict")
		return
	}
	if created {
		if err := api.appendModelAudit(r.Context(), tx, auditlog.Event{
			OccurredAt:   now,
			Actor:        identity.Subject,
			Action:       auditModelExportRequested,
			ResourceType: "model_export",
			ResourceID:   record.ExportID,
			RequestID:    r.Header.Get("X-Request-Id"),
			IP:           requestIP(r.RemoteAddr),
			UserAgent:    r.UserAgent(),
			Payload: map[string]any{
				"service":          "experiments",
				"project_id":       projectID,
				"model_version_id": versionID,
				"status":           record.Status,
			},
		}); err != nil {
			api.writeError(w, r, http.StatusInternalServerError, "audit_failed")
			return
		}
	}
	if err := tx.Commit(); err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	api.writeJSON(w, http.StatusOK, modelExportResponse{Export: record, Created: created})
}

func (api *experimentsAPI) resolveDatasetVersionsFromRun(ctx context.Context, projectID, runID string) ([]string, error) {
	store := api.modelRunSpecStore()
	if store == nil {
		return nil, errors.New("run spec store unavailable")
	}
	record, err := store.GetRun(ctx, projectID, runID)
	if err != nil {
		return nil, err
	}
	if len(record.RunSpec) == 0 {
		return nil, errors.New("run spec not found")
	}
	var spec domain.RunSpec
	if err := json.Unmarshal(record.RunSpec, &spec); err != nil {
		return nil, err
	}
	if len(spec.DatasetBindings) == 0 {
		return nil, nil
	}
	values := make([]string, 0, len(spec.DatasetBindings))
	for _, id := range spec.DatasetBindings {
		values = append(values, id)
	}
	return normalizeStringList(values), nil
}

func modelIntegrity(model domain.Model) (string, error) {
	type integrityInput struct {
		ProjectID string         `json:"project_id"`
		Name      string         `json:"name"`
		Version   string         `json:"version"`
		Status    string         `json:"status"`
		Metadata  map[string]any `json:"metadata"`
		CreatedBy string         `json:"created_by"`
	}
	input := integrityInput{
		ProjectID: strings.TrimSpace(model.ProjectID),
		Name:      strings.TrimSpace(model.Name),
		Version:   strings.TrimSpace(model.Version),
		Status:    string(model.Status),
		Metadata:  model.Metadata,
		CreatedBy: strings.TrimSpace(model.CreatedBy),
	}
	return integritySHA256(input)
}

func modelVersionIntegrity(version domain.ModelVersion) (string, error) {
	type integrityInput struct {
		ProjectID         string         `json:"project_id"`
		ModelID           string         `json:"model_id"`
		Version           string         `json:"version"`
		Status            string         `json:"status"`
		RunID             string         `json:"run_id"`
		ArtifactIDs       []string       `json:"artifact_ids"`
		DatasetVersionIDs []string       `json:"dataset_version_ids"`
		EnvLockID         string         `json:"env_lock_id"`
		CodeRef           domain.CodeRef `json:"code_ref"`
		PolicySnapshotSHA string         `json:"policy_snapshot_sha256"`
		CreatedBy         string         `json:"created_by"`
	}
	input := integrityInput{
		ProjectID:         strings.TrimSpace(version.ProjectID),
		ModelID:           strings.TrimSpace(version.ModelID),
		Version:           strings.TrimSpace(version.Version),
		Status:            string(version.Status),
		RunID:             strings.TrimSpace(version.RunID),
		ArtifactIDs:       normalizeStringList(version.ArtifactIDs),
		DatasetVersionIDs: normalizeStringList(version.DatasetVersionIDs),
		EnvLockID:         strings.TrimSpace(version.EnvLockID),
		CodeRef:           version.CodeRef,
		PolicySnapshotSHA: strings.TrimSpace(version.PolicySnapshotSHA256),
		CreatedBy:         strings.TrimSpace(version.CreatedBy),
	}
	return integritySHA256(input)
}

func modelExportIntegrity(export domain.ModelExport) (string, error) {
	type integrityInput struct {
		ProjectID      string `json:"project_id"`
		ModelVersionID string `json:"model_version_id"`
		Status         string `json:"status"`
		Target         string `json:"target,omitempty"`
		CreatedBy      string `json:"created_by"`
	}
	input := integrityInput{
		ProjectID:      strings.TrimSpace(export.ProjectID),
		ModelVersionID: strings.TrimSpace(export.ModelVersionID),
		Status:         strings.TrimSpace(export.Status),
		Target:         strings.TrimSpace(export.Target),
		CreatedBy:      strings.TrimSpace(export.CreatedBy),
	}
	return integritySHA256(input)
}

func normalizeStringList(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	if len(out) == 0 {
		return nil
	}
	sort.Strings(out)
	return out
}
