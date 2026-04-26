package main

import (
	"errors"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/animus-labs/animus-go/closed/internal/domain"
	"github.com/animus-labs/animus-go/closed/internal/platform/auditlog"
	"github.com/animus-labs/animus-go/closed/internal/platform/auth"
	"github.com/animus-labs/animus-go/closed/internal/repo"
	"github.com/animus-labs/animus-go/closed/internal/repo/postgres"
	"github.com/google/uuid"
)

const (
	environmentStatusActive   = "active"
	environmentStatusArchived = "archived"
)

type environmentDefinitionRequest struct {
	IdempotencyKey       string                        `json:"idempotencyKey"`
	Name                 string                        `json:"name"`
	Description          string                        `json:"description,omitempty"`
	BaseImages           []domain.EnvironmentBaseImage `json:"baseImages"`
	ResourceDefaults     domain.EnvironmentResources   `json:"resourceDefaults,omitempty"`
	ResourceLimits       domain.EnvironmentResources   `json:"resourceLimits,omitempty"`
	AllowedAccelerators  []string                      `json:"allowedAccelerators,omitempty"`
	NetworkClassRef      string                        `json:"networkClassRef,omitempty"`
	SecretAccessClassRef string                        `json:"secretAccessClassRef,omitempty"`
	Metadata             map[string]string             `json:"metadata,omitempty"`
}

type environmentDefinitionResponse struct {
	Definition domain.EnvironmentDefinition `json:"definition"`
	Created    bool                         `json:"created"`
}

type environmentDefinitionListResponse struct {
	Definitions []domain.EnvironmentDefinition `json:"definitions"`
}

type environmentLockRequest struct {
	IdempotencyKey          string            `json:"idempotencyKey"`
	EnvironmentDefinitionID string            `json:"environmentDefinitionId"`
	ImageDigests            map[string]string `json:"imageDigests"`
	DependencyChecksums     map[string]string `json:"dependencyChecksums,omitempty"`
	SBOMRef                 string            `json:"sbomRef,omitempty"`
}

type environmentLockResponse struct {
	Lock    domain.EnvLock `json:"lock"`
	Created bool           `json:"created"`
}

type environmentLockListResponse struct {
	Locks []domain.EnvLock `json:"locks"`
}

func (api *experimentsAPI) handleCreateEnvironmentDefinition(w http.ResponseWriter, r *http.Request) {
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
	var req environmentDefinitionRequest
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
	normalized, err := normalizeEnvironmentDefinitionRequest(req, false)
	if err != nil {
		api.writeError(w, r, http.StatusBadRequest, "invalid_environment_definition")
		return
	}
	metadata := mapStringToMetadata(normalized.Metadata)

	store := postgres.NewEnvironmentStore(api.db)
	if store == nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	version, err := store.NextDefinitionVersion(r.Context(), projectID, normalized.Name)
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	now := time.Now().UTC()
	definition := domain.EnvironmentDefinition{
		ID:                   uuid.NewString(),
		ProjectID:            projectID,
		Name:                 normalized.Name,
		Version:              version,
		Description:          normalized.Description,
		BaseImages:           normalized.BaseImages,
		ResourceDefaults:     normalized.ResourceDefaults,
		ResourceLimits:       normalized.ResourceLimits,
		AllowedAccelerators:  normalized.AllowedAccelerators,
		NetworkClassRef:      normalized.NetworkClassRef,
		SecretAccessClassRef: normalized.SecretAccessClassRef,
		Status:               environmentStatusActive,
		Metadata:             metadata,
		CreatedAt:            now,
		CreatedBy:            strings.TrimSpace(identity.Subject),
	}
	integrity, err := environmentDefinitionIntegrity(definition)
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "integrity_failed")
		return
	}
	definition.IntegritySHA256 = integrity

	tx, err := api.db.BeginTx(r.Context(), nil)
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	defer func() { _ = tx.Rollback() }()

	txStore := postgres.NewEnvironmentStore(tx)
	record, created, err := txStore.CreateDefinition(r.Context(), definition, idempotencyKey)
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	if !created && record.Definition.IntegritySHA256 != integrity {
		api.writeError(w, r, http.StatusConflict, "idempotency_conflict")
		return
	}
	if created {
		if _, err := auditlog.Insert(r.Context(), tx, auditlog.Event{
			OccurredAt:   now,
			Actor:        identity.Subject,
			Action:       "environment.defined",
			ResourceType: "environment_definition",
			ResourceID:   record.Definition.ID,
			RequestID:    r.Header.Get("X-Request-Id"),
			IP:           requestIP(r.RemoteAddr),
			UserAgent:    r.UserAgent(),
			Payload: map[string]any{
				"service":       "experiments",
				"project_id":    projectID,
				"definition_id": record.Definition.ID,
				"name":          record.Definition.Name,
				"version":       record.Definition.Version,
				"status":        record.Definition.Status,
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

	api.writeJSON(w, http.StatusOK, environmentDefinitionResponse{
		Definition: record.Definition,
		Created:    created,
	})
}

func (api *experimentsAPI) handleListEnvironmentDefinitions(w http.ResponseWriter, r *http.Request) {
	projectID := strings.TrimSpace(r.PathValue("project_id"))
	if projectID == "" {
		api.writeError(w, r, http.StatusBadRequest, "project_id_required")
		return
	}
	name := strings.TrimSpace(r.URL.Query().Get("name"))
	status := strings.TrimSpace(r.URL.Query().Get("status"))
	limit := clampInt(parseIntQuery(r, "limit", 100), 1, 500)

	store := postgres.NewEnvironmentStore(api.db)
	if store == nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	records, err := store.ListDefinitions(r.Context(), projectID, name, status, limit)
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	out := make([]domain.EnvironmentDefinition, 0, len(records))
	for _, record := range records {
		out = append(out, record.Definition)
	}
	api.writeJSON(w, http.StatusOK, environmentDefinitionListResponse{Definitions: out})
}

func (api *experimentsAPI) handleGetEnvironmentDefinition(w http.ResponseWriter, r *http.Request) {
	projectID := strings.TrimSpace(r.PathValue("project_id"))
	definitionID := strings.TrimSpace(r.PathValue("definition_id"))
	if projectID == "" {
		api.writeError(w, r, http.StatusBadRequest, "project_id_required")
		return
	}
	if definitionID == "" {
		api.writeError(w, r, http.StatusBadRequest, "definition_id_required")
		return
	}
	store := postgres.NewEnvironmentStore(api.db)
	if store == nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	record, err := store.GetDefinition(r.Context(), projectID, definitionID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			api.writeError(w, r, http.StatusNotFound, "not_found")
			return
		}
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	api.writeJSON(w, http.StatusOK, record.Definition)
}

func (api *experimentsAPI) handleUpdateEnvironmentDefinition(w http.ResponseWriter, r *http.Request) {
	api.handleEnvironmentDefinitionVersionedChange(w, r, environmentStatusActive, "environment.updated")
}

func (api *experimentsAPI) handleArchiveEnvironmentDefinition(w http.ResponseWriter, r *http.Request) {
	api.handleEnvironmentDefinitionVersionedChange(w, r, environmentStatusArchived, "environment.archived")
}

func (api *experimentsAPI) handleEnvironmentDefinitionVersionedChange(w http.ResponseWriter, r *http.Request, status string, action string) {
	identity, ok := auth.IdentityFromContext(r.Context())
	if !ok || strings.TrimSpace(identity.Subject) == "" {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	projectID := strings.TrimSpace(r.PathValue("project_id"))
	definitionID := strings.TrimSpace(r.PathValue("definition_id"))
	if projectID == "" {
		api.writeError(w, r, http.StatusBadRequest, "project_id_required")
		return
	}
	if definitionID == "" {
		api.writeError(w, r, http.StatusBadRequest, "definition_id_required")
		return
	}

	var req environmentDefinitionRequest
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

	store := postgres.NewEnvironmentStore(api.db)
	if store == nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	existing, err := store.GetDefinition(r.Context(), projectID, definitionID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			api.writeError(w, r, http.StatusNotFound, "not_found")
			return
		}
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	normalized, err := normalizeEnvironmentDefinitionRequest(req, true)
	if err != nil {
		api.writeError(w, r, http.StatusBadRequest, "invalid_environment_definition")
		return
	}
	name := normalized.Name
	if name == "" {
		name = existing.Definition.Name
	}
	if strings.TrimSpace(name) != strings.TrimSpace(existing.Definition.Name) {
		api.writeError(w, r, http.StatusBadRequest, "definition_name_mismatch")
		return
	}
	baseImages := normalized.BaseImages
	if len(baseImages) == 0 {
		baseImages = existing.Definition.BaseImages
	}
	resourceDefaults := normalized.ResourceDefaults
	if resourceDefaults == (domain.EnvironmentResources{}) {
		resourceDefaults = existing.Definition.ResourceDefaults
	}
	resourceLimits := normalized.ResourceLimits
	if resourceLimits == (domain.EnvironmentResources{}) {
		resourceLimits = existing.Definition.ResourceLimits
	}
	allowedAccelerators := normalized.AllowedAccelerators
	if len(allowedAccelerators) == 0 {
		allowedAccelerators = existing.Definition.AllowedAccelerators
	}
	networkClassRef := normalized.NetworkClassRef
	if networkClassRef == "" {
		networkClassRef = existing.Definition.NetworkClassRef
	}
	secretAccessClassRef := normalized.SecretAccessClassRef
	if secretAccessClassRef == "" {
		secretAccessClassRef = existing.Definition.SecretAccessClassRef
	}
	description := normalized.Description
	if description == "" {
		description = existing.Definition.Description
	}
	metadata := normalized.Metadata
	if len(metadata) == 0 {
		existingMeta := make(map[string]string, 0)
		for key, value := range existing.Definition.Metadata {
			if v, ok := value.(string); ok {
				existingMeta[key] = v
			}
		}
		metadata = existingMeta
	}
	metadataPayload := mapStringToMetadata(metadata)

	version, err := store.NextDefinitionVersion(r.Context(), projectID, name)
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	now := time.Now().UTC()
	definition := domain.EnvironmentDefinition{
		ID:                     uuid.NewString(),
		ProjectID:              projectID,
		Name:                   name,
		Version:                version,
		Description:            description,
		BaseImages:             baseImages,
		ResourceDefaults:       resourceDefaults,
		ResourceLimits:         resourceLimits,
		AllowedAccelerators:    allowedAccelerators,
		NetworkClassRef:        networkClassRef,
		SecretAccessClassRef:   secretAccessClassRef,
		Status:                 status,
		SupersedesDefinitionID: existing.Definition.ID,
		Metadata:               metadataPayload,
		CreatedAt:              now,
		CreatedBy:              strings.TrimSpace(identity.Subject),
	}
	integrity, err := environmentDefinitionIntegrity(definition)
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "integrity_failed")
		return
	}
	definition.IntegritySHA256 = integrity

	tx, err := api.db.BeginTx(r.Context(), nil)
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	defer func() { _ = tx.Rollback() }()

	txStore := postgres.NewEnvironmentStore(tx)
	record, created, err := txStore.CreateDefinition(r.Context(), definition, idempotencyKey)
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	if !created && record.Definition.IntegritySHA256 != integrity {
		api.writeError(w, r, http.StatusConflict, "idempotency_conflict")
		return
	}
	if created {
		if _, err := auditlog.Insert(r.Context(), tx, auditlog.Event{
			OccurredAt:   now,
			Actor:        identity.Subject,
			Action:       action,
			ResourceType: "environment_definition",
			ResourceID:   record.Definition.ID,
			RequestID:    r.Header.Get("X-Request-Id"),
			IP:           requestIP(r.RemoteAddr),
			UserAgent:    r.UserAgent(),
			Payload: map[string]any{
				"service":                  "experiments",
				"project_id":               projectID,
				"definition_id":            record.Definition.ID,
				"supersedes_definition_id": existing.Definition.ID,
				"name":                     record.Definition.Name,
				"version":                  record.Definition.Version,
				"status":                   record.Definition.Status,
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

	api.writeJSON(w, http.StatusOK, environmentDefinitionResponse{
		Definition: record.Definition,
		Created:    created,
	})
}

func (api *experimentsAPI) handleCreateEnvironmentLock(w http.ResponseWriter, r *http.Request) {
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
	var req environmentLockRequest
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
	definitionID := strings.TrimSpace(req.EnvironmentDefinitionID)
	if definitionID == "" {
		api.writeError(w, r, http.StatusBadRequest, "environment_definition_required")
		return
	}
	store := postgres.NewEnvironmentStore(api.db)
	if store == nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	definitionRecord, err := store.GetDefinition(r.Context(), projectID, definitionID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			api.writeError(w, r, http.StatusNotFound, "environment_definition_not_found")
			return
		}
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	if strings.EqualFold(definitionRecord.Definition.Status, environmentStatusArchived) {
		api.writeError(w, r, http.StatusConflict, "environment_definition_archived")
		return
	}

	now := time.Now().UTC()
	lockID := uuid.NewString()
	lock, err := buildEnvironmentLock(definitionRecord.Definition, req, lockID, now)
	if err != nil {
		api.writeError(w, r, http.StatusBadRequest, "invalid_environment_lock")
		return
	}
	allowed, reason, err := api.verifyRegistryImages(r.Context(), identity, projectID, lock.LockID, lock.Images, r.Header.Get("X-Request-Id"))
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	if !allowed {
		code := "registry_verification_failed"
		if reason == registryBlockReasonUnsigned {
			code = "registry_signature_required"
		}
		api.writeError(w, r, http.StatusUnprocessableEntity, code)
		return
	}
	integrity, err := environmentLockIntegrity(lock, projectID, identity.Subject, now)
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "integrity_failed")
		return
	}

	tx, err := api.db.BeginTx(r.Context(), nil)
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	defer func() { _ = tx.Rollback() }()

	txStore := postgres.NewEnvironmentStore(tx)
	record, created, err := txStore.CreateLock(r.Context(), lock, projectID, identity.Subject, idempotencyKey, now, integrity)
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	if !created && record.Lock.EnvHash != lock.EnvHash {
		api.writeError(w, r, http.StatusConflict, "idempotency_conflict")
		return
	}
	if created {
		if _, err := auditlog.Insert(r.Context(), tx, auditlog.Event{
			OccurredAt:   now,
			Actor:        identity.Subject,
			Action:       "environment.locked",
			ResourceType: "environment_lock",
			ResourceID:   record.Lock.LockID,
			RequestID:    r.Header.Get("X-Request-Id"),
			IP:           requestIP(r.RemoteAddr),
			UserAgent:    r.UserAgent(),
			Payload: map[string]any{
				"service":                   "experiments",
				"project_id":                projectID,
				"environment_definition_id": record.Lock.EnvironmentDefinitionID,
				"environment_lock_id":       record.Lock.LockID,
				"env_hash":                  record.Lock.EnvHash,
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

	api.writeJSON(w, http.StatusOK, environmentLockResponse{
		Lock:    record.Lock,
		Created: created,
	})
}

func (api *experimentsAPI) handleListEnvironmentLocks(w http.ResponseWriter, r *http.Request) {
	projectID := strings.TrimSpace(r.PathValue("project_id"))
	if projectID == "" {
		api.writeError(w, r, http.StatusBadRequest, "project_id_required")
		return
	}
	definitionID := strings.TrimSpace(r.URL.Query().Get("environment_definition_id"))
	limit := clampInt(parseIntQuery(r, "limit", 100), 1, 500)
	store := postgres.NewEnvironmentStore(api.db)
	if store == nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	records, err := store.ListLocks(r.Context(), projectID, definitionID, limit)
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	out := make([]domain.EnvLock, 0, len(records))
	for _, record := range records {
		out = append(out, record.Lock)
	}
	api.writeJSON(w, http.StatusOK, environmentLockListResponse{Locks: out})
}

func (api *experimentsAPI) handleGetEnvironmentLock(w http.ResponseWriter, r *http.Request) {
	identity, ok := auth.IdentityFromContext(r.Context())
	if !ok || strings.TrimSpace(identity.Subject) == "" {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	projectID := strings.TrimSpace(r.PathValue("project_id"))
	lockID := strings.TrimSpace(r.PathValue("lock_id"))
	if projectID == "" {
		api.writeError(w, r, http.StatusBadRequest, "project_id_required")
		return
	}
	if lockID == "" {
		api.writeError(w, r, http.StatusBadRequest, "lock_id_required")
		return
	}
	store := postgres.NewEnvironmentStore(api.db)
	if store == nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	record, err := store.GetLock(r.Context(), projectID, lockID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			api.writeError(w, r, http.StatusNotFound, "not_found")
			return
		}
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	if _, err := auditlog.Insert(r.Context(), api.db, auditlog.Event{
		OccurredAt:   time.Now().UTC(),
		Actor:        identity.Subject,
		Action:       "environment.lock.read",
		ResourceType: "environment_lock",
		ResourceID:   lockID,
		RequestID:    r.Header.Get("X-Request-Id"),
		IP:           requestIP(r.RemoteAddr),
		UserAgent:    r.UserAgent(),
		Payload: map[string]any{
			"service":    "experiments",
			"project_id": projectID,
			"lock_id":    lockID,
			"env_hash":   record.Lock.EnvHash,
		},
	}); err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "audit_failed")
		return
	}
	api.writeJSON(w, http.StatusOK, record.Lock)
}

func normalizeEnvironmentDefinitionRequest(req environmentDefinitionRequest, allowPartial bool) (environmentDefinitionRequest, error) {
	req.Name = strings.TrimSpace(req.Name)
	req.Description = strings.TrimSpace(req.Description)
	req.NetworkClassRef = strings.TrimSpace(req.NetworkClassRef)
	req.SecretAccessClassRef = strings.TrimSpace(req.SecretAccessClassRef)

	if req.Name == "" && !allowPartial {
		return req, errors.New("name is required")
	}
	if len(req.BaseImages) == 0 && !allowPartial {
		return req, errors.New("baseImages is required")
	}
	seen := make(map[string]struct{}, len(req.BaseImages))
	for i, image := range req.BaseImages {
		name := strings.TrimSpace(image.Name)
		ref := strings.TrimSpace(image.Ref)
		if name == "" || ref == "" {
			return req, errors.New("baseImages entries require name and ref")
		}
		if strings.Contains(ref, "@sha256:") {
			return req, errors.New("base image ref must not include digest")
		}
		if _, ok := seen[name]; ok {
			return req, errors.New("base image names must be unique")
		}
		seen[name] = struct{}{}
		req.BaseImages[i].Name = name
		req.BaseImages[i].Ref = ref
	}

	if req.ResourceDefaults.GPU < 0 || req.ResourceLimits.GPU < 0 {
		return req, errors.New("gpu must be >= 0")
	}

	accels := normalizeAccelerators(req.AllowedAccelerators)
	if len(accels) == 0 && !allowPartial {
		accels = []string{"cpu"}
	}
	req.AllowedAccelerators = accels

	metadata := make(map[string]string, len(req.Metadata))
	for key, value := range req.Metadata {
		k := strings.TrimSpace(key)
		v := strings.TrimSpace(value)
		if k == "" {
			return req, errors.New("metadata keys must be non-empty")
		}
		metadata[k] = v
	}
	req.Metadata = metadata
	return req, nil
}

func normalizeAccelerators(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		v := strings.ToLower(strings.TrimSpace(value))
		if v == "" {
			continue
		}
		if v != "cpu" && v != "gpu" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}

func buildEnvironmentLock(def domain.EnvironmentDefinition, req environmentLockRequest, lockID string, createdAt time.Time) (domain.EnvLock, error) {
	if strings.TrimSpace(lockID) == "" {
		return domain.EnvLock{}, errors.New("lock id is required")
	}
	if len(def.BaseImages) == 0 {
		return domain.EnvLock{}, errors.New("definition base images are required")
	}
	digests := make(map[string]string, len(req.ImageDigests))
	for key, value := range req.ImageDigests {
		k := strings.TrimSpace(key)
		v := strings.TrimSpace(value)
		if k == "" || v == "" {
			return domain.EnvLock{}, errors.New("image digests must be non-empty")
		}
		digests[k] = v
	}
	resolved := make([]domain.EnvironmentImage, 0, len(def.BaseImages))
	for _, image := range def.BaseImages {
		digest, ok := digests[image.Name]
		if !ok {
			return domain.EnvLock{}, errors.New("missing image digest for base image")
		}
		if !strings.HasPrefix(digest, "sha256:") {
			return domain.EnvLock{}, errors.New("image digest must be sha256")
		}
		resolved = append(resolved, domain.EnvironmentImage{
			Name:   image.Name,
			Ref:    image.Ref,
			Digest: digest,
		})
	}
	for key := range digests {
		if !baseImageExists(def.BaseImages, key) {
			return domain.EnvLock{}, errors.New("image digest provided for unknown base image")
		}
	}

	lock := domain.EnvLock{
		LockID:                       lockID,
		EnvironmentDefinitionID:      def.ID,
		EnvironmentDefinitionVersion: def.Version,
		Images:                       resolved,
		ResourceDefaults:             def.ResourceDefaults,
		ResourceLimits:               def.ResourceLimits,
		AllowedAccelerators:          def.AllowedAccelerators,
		NetworkClassRef:              def.NetworkClassRef,
		SecretAccessClassRef:         def.SecretAccessClassRef,
		DependencyChecksums:          req.DependencyChecksums,
		SBOMRef:                      strings.TrimSpace(req.SBOMRef),
		CreatedAt:                    createdAt,
	}
	envHash, err := hashEnvironmentLock(lock)
	if err != nil {
		return domain.EnvLock{}, err
	}
	lock.EnvHash = envHash
	return lock, nil
}

func baseImageExists(images []domain.EnvironmentBaseImage, name string) bool {
	for _, image := range images {
		if image.Name == name {
			return true
		}
	}
	return false
}

type environmentLockHashInput struct {
	EnvironmentDefinitionID      string                      `json:"environmentDefinitionId"`
	EnvironmentDefinitionVersion int                         `json:"environmentDefinitionVersion"`
	Images                       []domain.EnvironmentImage   `json:"images"`
	ResourceDefaults             domain.EnvironmentResources `json:"resourceDefaults,omitempty"`
	ResourceLimits               domain.EnvironmentResources `json:"resourceLimits,omitempty"`
	AllowedAccelerators          []string                    `json:"allowedAccelerators,omitempty"`
	NetworkClassRef              string                      `json:"networkClassRef,omitempty"`
	SecretAccessClassRef         string                      `json:"secretAccessClassRef,omitempty"`
	DependencyChecksums          []checksumPair              `json:"dependencyChecksums,omitempty"`
	SBOMRef                      string                      `json:"sbomRef,omitempty"`
}

func hashEnvironmentLock(lock domain.EnvLock) (string, error) {
	images := append([]domain.EnvironmentImage{}, lock.Images...)
	sort.Slice(images, func(i, j int) bool {
		if images[i].Name == images[j].Name {
			return images[i].Ref < images[j].Ref
		}
		return images[i].Name < images[j].Name
	})
	checksums := sortedChecksumPairs(lock.DependencyChecksums)
	payload := environmentLockHashInput{
		EnvironmentDefinitionID:      strings.TrimSpace(lock.EnvironmentDefinitionID),
		EnvironmentDefinitionVersion: lock.EnvironmentDefinitionVersion,
		Images:                       images,
		ResourceDefaults:             lock.ResourceDefaults,
		ResourceLimits:               lock.ResourceLimits,
		AllowedAccelerators:          normalizeAccelerators(lock.AllowedAccelerators),
		NetworkClassRef:              strings.TrimSpace(lock.NetworkClassRef),
		SecretAccessClassRef:         strings.TrimSpace(lock.SecretAccessClassRef),
		DependencyChecksums:          checksums,
		SBOMRef:                      strings.TrimSpace(lock.SBOMRef),
	}
	return integritySHA256(payload)
}

func environmentLockIntegrity(lock domain.EnvLock, projectID, createdBy string, createdAt time.Time) (string, error) {
	payload := struct {
		LockID            string    `json:"lock_id"`
		ProjectID         string    `json:"project_id"`
		EnvHash           string    `json:"env_hash"`
		CreatedAt         time.Time `json:"created_at"`
		CreatedBy         string    `json:"created_by"`
		DefinitionID      string    `json:"definition_id"`
		DefinitionVersion int       `json:"definition_version"`
	}{
		LockID:            strings.TrimSpace(lock.LockID),
		ProjectID:         strings.TrimSpace(projectID),
		EnvHash:           strings.TrimSpace(lock.EnvHash),
		CreatedAt:         createdAt.UTC(),
		CreatedBy:         strings.TrimSpace(createdBy),
		DefinitionID:      strings.TrimSpace(lock.EnvironmentDefinitionID),
		DefinitionVersion: lock.EnvironmentDefinitionVersion,
	}
	return integritySHA256(payload)
}

func environmentDefinitionIntegrity(def domain.EnvironmentDefinition) (string, error) {
	metadataPairs := make([]checksumPair, 0, len(def.Metadata))
	for key, value := range def.Metadata {
		metadataPairs = append(metadataPairs, checksumPair{
			Key:      key,
			Checksum: stringifyMetadataValue(value),
		})
	}
	sort.Slice(metadataPairs, func(i, j int) bool {
		return metadataPairs[i].Key < metadataPairs[j].Key
	})
	baseImages := append([]domain.EnvironmentBaseImage{}, def.BaseImages...)
	sort.Slice(baseImages, func(i, j int) bool {
		return baseImages[i].Name < baseImages[j].Name
	})
	payload := struct {
		DefinitionID         string                        `json:"definition_id"`
		ProjectID            string                        `json:"project_id"`
		Name                 string                        `json:"name"`
		Version              int                           `json:"version"`
		Description          string                        `json:"description,omitempty"`
		BaseImages           []domain.EnvironmentBaseImage `json:"base_images"`
		ResourceDefaults     domain.EnvironmentResources   `json:"resource_defaults,omitempty"`
		ResourceLimits       domain.EnvironmentResources   `json:"resource_limits,omitempty"`
		AllowedAccelerators  []string                      `json:"allowed_accelerators,omitempty"`
		NetworkClassRef      string                        `json:"network_class_ref,omitempty"`
		SecretAccessClassRef string                        `json:"secret_access_class_ref,omitempty"`
		Status               string                        `json:"status"`
		SupersedesID         string                        `json:"supersedes_definition_id,omitempty"`
		Metadata             []checksumPair                `json:"metadata,omitempty"`
		CreatedAt            time.Time                     `json:"created_at"`
		CreatedBy            string                        `json:"created_by"`
	}{
		DefinitionID:         strings.TrimSpace(def.ID),
		ProjectID:            strings.TrimSpace(def.ProjectID),
		Name:                 strings.TrimSpace(def.Name),
		Version:              def.Version,
		Description:          strings.TrimSpace(def.Description),
		BaseImages:           baseImages,
		ResourceDefaults:     def.ResourceDefaults,
		ResourceLimits:       def.ResourceLimits,
		AllowedAccelerators:  normalizeAccelerators(def.AllowedAccelerators),
		NetworkClassRef:      strings.TrimSpace(def.NetworkClassRef),
		SecretAccessClassRef: strings.TrimSpace(def.SecretAccessClassRef),
		Status:               strings.TrimSpace(def.Status),
		SupersedesID:         strings.TrimSpace(def.SupersedesDefinitionID),
		Metadata:             metadataPairs,
		CreatedAt:            def.CreatedAt.UTC(),
		CreatedBy:            strings.TrimSpace(def.CreatedBy),
	}
	return integritySHA256(payload)
}

func stringifyMetadataValue(value any) string {
	if value == nil {
		return ""
	}
	switch v := value.(type) {
	case string:
		return v
	default:
		return ""
	}
}

func mapStringToMetadata(input map[string]string) domain.Metadata {
	if len(input) == 0 {
		return domain.Metadata{}
	}
	out := make(domain.Metadata, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

func sortedChecksumPairs(checksums map[string]string) []checksumPair {
	if len(checksums) == 0 {
		return []checksumPair{}
	}
	keys := make([]string, 0, len(checksums))
	for key := range checksums {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]checksumPair, 0, len(keys))
	for _, key := range keys {
		out = append(out, checksumPair{
			Key:      key,
			Checksum: checksums[key],
		})
	}
	return out
}
