package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/animus-labs/animus-go/closed/internal/dataplane"
	"github.com/animus-labs/animus-go/closed/internal/domain"
	"github.com/animus-labs/animus-go/closed/internal/platform/auditlog"
	"github.com/animus-labs/animus-go/closed/internal/platform/auth"
	"github.com/animus-labs/animus-go/closed/internal/platform/rbac"
	"github.com/animus-labs/animus-go/closed/internal/repo"
	"github.com/animus-labs/animus-go/closed/internal/repo/postgres"
	"github.com/google/uuid"
)

const (
	auditDevEnvCreated       = "devenv.created"
	auditDevEnvProvisioned   = "devenv.provisioned"
	auditDevEnvProvisionFail = "devenv.provision_failed"
	auditDevEnvRepoCloned    = "devenv.repo.cloned"
	auditDevEnvSessionOpened = "devenv.session.opened"
	auditDevEnvSessionAccess = "devenv.session.accessed"
	auditDevEnvExpired       = "devenv.expired"
	auditDevEnvDeleted       = "devenv.deleted"
)

type devEnvCreateRequest struct {
	IdempotencyKey string `json:"idempotencyKey"`
	TemplateRef    string `json:"templateRef"`
	RepoURL        string `json:"repoUrl,omitempty"`
	RefType        string `json:"refType,omitempty"`
	RefValue       string `json:"refValue,omitempty"`
	CommitPin      string `json:"commitPin,omitempty"`
	ImageName      string `json:"imageName,omitempty"`
	TTLSeconds     int64  `json:"ttlSeconds,omitempty"`
}

type devEnvAccessRequest struct {
	TTLSeconds int64 `json:"ttlSeconds,omitempty"`
}

type devEnvResponse struct {
	Environment domain.DevEnvironment `json:"environment"`
	Created     bool                  `json:"created"`
}

type devEnvListResponse struct {
	Environments []domain.DevEnvironment `json:"environments"`
}

type devEnvAccessResponse struct {
	DevEnvID  string    `json:"devEnvId"`
	ProjectID string    `json:"projectId"`
	SessionID string    `json:"sessionId"`
	ExpiresAt time.Time `json:"expiresAt"`
	ProxyPath string    `json:"proxyPath"`
	Status    string    `json:"status,omitempty"`
	Message   string    `json:"message,omitempty"`
}

type devEnvironmentStore interface {
	Create(ctx context.Context, env domain.DevEnvironment, idempotencyKey string) (postgres.DevEnvironmentRecord, bool, error)
	Get(ctx context.Context, projectID, devEnvID string) (postgres.DevEnvironmentRecord, error)
	List(ctx context.Context, projectID, state string, limit int) ([]postgres.DevEnvironmentRecord, error)
	UpdateState(ctx context.Context, projectID, devEnvID, state, dpJobName, dpNamespace string) (bool, error)
	UpdateLastAccess(ctx context.Context, projectID, devEnvID string, accessedAt time.Time, minInterval time.Duration) (bool, error)
	ListExpired(ctx context.Context, projectID string, now time.Time, limit int) ([]postgres.DevEnvironmentRecord, error)
}

type devEnvPolicyStore interface {
	Insert(ctx context.Context, devEnvID, projectID string, snapshot domain.PolicySnapshot, snapshotJSON []byte, createdAt time.Time, createdBy, integritySHA string) error
	Get(ctx context.Context, projectID, devEnvID string) (domain.PolicySnapshot, error)
	GetSHA(ctx context.Context, projectID, devEnvID string) (string, error)
}

type devEnvSessionStore interface {
	Insert(ctx context.Context, session domain.DevEnvAccessSession, integritySHA string) error
	GetBySessionID(ctx context.Context, projectID, sessionID string) (domain.DevEnvAccessSession, error)
}

type devEnvDataplaneClient interface {
	ProvisionDevEnv(ctx context.Context, req dataplane.DevEnvProvisionRequest, requestID string) (dataplane.DevEnvProvisionResponse, int, error)
	DeleteDevEnv(ctx context.Context, req dataplane.DevEnvDeleteRequest, requestID string) (dataplane.DevEnvDeleteResponse, int, error)
	AccessDevEnv(ctx context.Context, req dataplane.DevEnvAccessRequest, requestID string) (dataplane.DevEnvAccessResponse, int, error)
}

type devEnvPolicySnapshotBuilder func(ctx context.Context, projectID string, identity auth.Identity, envLock domain.EnvLock) (domain.PolicySnapshot, error)

type devEnvAuditAppender interface {
	Append(ctx context.Context, event auditlog.Event) error
}

type environmentStore interface {
	GetDefinition(ctx context.Context, projectID, definitionID string) (postgres.EnvironmentDefinitionRecord, error)
}

func (api *experimentsAPI) devEnvStore() devEnvironmentStore {
	if api == nil {
		return nil
	}
	if api.devEnvStoreOverride != nil {
		return api.devEnvStoreOverride
	}
	return postgres.NewDevEnvironmentStore(api.db)
}

func (api *experimentsAPI) devEnvPolicyStore() devEnvPolicyStore {
	if api == nil {
		return nil
	}
	if api.devEnvPolicyStoreOverride != nil {
		return api.devEnvPolicyStoreOverride
	}
	return postgres.NewDevEnvPolicyStore(api.db)
}

func (api *experimentsAPI) devEnvSessionStore() devEnvSessionStore {
	if api == nil {
		return nil
	}
	if api.devEnvSessionStoreOverride != nil {
		return api.devEnvSessionStoreOverride
	}
	return postgres.NewDevEnvSessionStore(api.db)
}

func (api *experimentsAPI) envDefinitionStore() environmentStore {
	if api == nil {
		return nil
	}
	if api.environmentStoreOverride != nil {
		return api.environmentStoreOverride
	}
	return postgres.NewEnvironmentStore(api.db)
}

func (api *experimentsAPI) devEnvDataplaneClient() (devEnvDataplaneClient, error) {
	if api == nil {
		return nil, errors.New("api not initialized")
	}
	if api.devEnvDPClientOverride != nil {
		return api.devEnvDPClientOverride, nil
	}
	if strings.TrimSpace(api.dataplaneURL) == "" {
		return nil, errors.New("dataplane url not configured")
	}
	return newDataplaneClient(api.dataplaneURL, api.runTokenSecret)
}

func (api *experimentsAPI) buildDevEnvPolicySnapshot(ctx context.Context, projectID string, identity auth.Identity, envLock domain.EnvLock) (domain.PolicySnapshot, error) {
	if api == nil {
		return domain.PolicySnapshot{}, errors.New("api not initialized")
	}
	if api.devEnvPolicySnapshotOverride != nil {
		return api.devEnvPolicySnapshotOverride(ctx, projectID, identity, envLock)
	}
	return api.buildPolicySnapshot(ctx, projectID, identity, envLock)
}

func (api *experimentsAPI) appendDevEnvAudit(ctx context.Context, event auditlog.Event) error {
	if api == nil {
		return errors.New("api not initialized")
	}
	if api.devEnvAuditOverride != nil {
		return api.devEnvAuditOverride.Append(ctx, event)
	}
	_, err := auditlog.Insert(ctx, api.db, event)
	return err
}

func (api *experimentsAPI) handleCreateDevEnvironment(w http.ResponseWriter, r *http.Request) {
	identity, ok := auth.IdentityFromContext(r.Context())
	if !ok || strings.TrimSpace(identity.Subject) == "" {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	if rbac.IsRunToken(identity) {
		api.writeError(w, r, http.StatusForbidden, "forbidden")
		return
	}

	projectID := strings.TrimSpace(r.PathValue("project_id"))
	if projectID == "" {
		api.writeError(w, r, http.StatusBadRequest, "project_id_required")
		return
	}

	var req devEnvCreateRequest
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

	templateRef := strings.TrimSpace(req.TemplateRef)
	if templateRef == "" {
		api.writeError(w, r, http.StatusBadRequest, "template_ref_required")
		return
	}
	repoURL := strings.TrimSpace(req.RepoURL)
	refType := strings.ToLower(strings.TrimSpace(req.RefType))
	refValue := strings.TrimSpace(req.RefValue)
	commitPin := strings.TrimSpace(req.CommitPin)
	if repoURL == "" {
		api.writeError(w, r, http.StatusBadRequest, "repo_url_required")
		return
	}
	repoInfo, err := parseRepoURL(repoURL, false)
	if err != nil {
		api.writeError(w, r, http.StatusBadRequest, "invalid_repo_url")
		return
	}
	if repoInfo.Scheme != "https" && repoInfo.Scheme != "ssh" {
		api.writeError(w, r, http.StatusBadRequest, "unsupported_repo_scheme")
		return
	}
	if err := validateRepoAllowlist(repoURL, api.devEnvRepoAllowlist); err != nil {
		if errors.Is(err, errRepoURLNotAllowed) {
			api.writeError(w, r, http.StatusForbidden, "repo_url_not_allowed")
			return
		}
		api.writeError(w, r, http.StatusBadRequest, "invalid_repo_url")
		return
	}
	if !validDevEnvRefType(refType) {
		api.writeError(w, r, http.StatusBadRequest, "invalid_ref_type")
		return
	}
	if refValue == "" {
		api.writeError(w, r, http.StatusBadRequest, "ref_value_required")
		return
	}
	if refType == domain.DevEnvRefTypeCommit && !validCommitSHA(refValue) {
		api.writeError(w, r, http.StatusBadRequest, "invalid_ref_value")
		return
	}
	if commitPin != "" && !validCommitSHA(commitPin) {
		api.writeError(w, r, http.StatusBadRequest, "invalid_commit_pin")
		return
	}

	defStore := api.envDefinitionStore()
	if defStore == nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	defRecord, err := defStore.GetDefinition(r.Context(), projectID, templateRef)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			api.writeError(w, r, http.StatusNotFound, "not_found")
			return
		}
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	imageName, imageRef, err := resolveDevEnvImage(defRecord.Definition, req.ImageName)
	if err != nil {
		api.writeError(w, r, http.StatusBadRequest, "invalid_image")
		return
	}

	ttlSeconds := req.TTLSeconds
	if ttlSeconds <= 0 && api.devEnvDefaultTTL > 0 {
		ttlSeconds = int64(api.devEnvDefaultTTL.Seconds())
	}
	if ttlSeconds <= 0 {
		api.writeError(w, r, http.StatusBadRequest, "ttl_seconds_required")
		return
	}

	now := time.Now().UTC()
	expiresAt := now.Add(time.Duration(ttlSeconds) * time.Second)

	envLock := domain.EnvLock{
		EnvironmentDefinitionID:      defRecord.Definition.ID,
		EnvironmentDefinitionVersion: defRecord.Definition.Version,
		ResourceDefaults:             defRecord.Definition.ResourceDefaults,
		ResourceLimits:               defRecord.Definition.ResourceLimits,
		AllowedAccelerators:          defRecord.Definition.AllowedAccelerators,
		NetworkClassRef:              defRecord.Definition.NetworkClassRef,
		SecretAccessClassRef:         defRecord.Definition.SecretAccessClassRef,
	}
	policySnapshot, err := api.buildDevEnvPolicySnapshot(r.Context(), projectID, identity, envLock)
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "policy_snapshot_failed")
		return
	}

	devEnv := domain.DevEnvironment{
		ID:                        uuid.NewString(),
		ProjectID:                 projectID,
		TemplateRef:               defRecord.Definition.ID,
		TemplateDefinitionID:      defRecord.Definition.ID,
		TemplateDefinitionVersion: defRecord.Definition.Version,
		TemplateIntegritySHA256:   defRecord.Definition.IntegritySHA256,
		RepoURL:                   repoURL,
		RefType:                   refType,
		RefValue:                  refValue,
		CommitPin:                 commitPin,
		ImageName:                 imageName,
		ImageRef:                  imageRef,
		TTLSeconds:                ttlSeconds,
		State:                     domain.DevEnvStateProvisioning,
		CreatedAt:                 now,
		CreatedBy:                 strings.TrimSpace(identity.Subject),
		ExpiresAt:                 expiresAt,
		PolicySnapshotSHA256:      policySnapshot.SnapshotSHA256,
	}
	integrity, err := devEnvIntegrity(devEnv)
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "integrity_failed")
		return
	}
	devEnv.IntegritySHA256 = integrity

	policyJSON, err := json.Marshal(policySnapshot)
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "policy_snapshot_failed")
		return
	}
	policyIntegrity, err := devEnvPolicyIntegrity(projectID, devEnv.ID, policySnapshot.SnapshotSHA256, now, identity.Subject)
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "integrity_failed")
		return
	}

	store := api.devEnvStore()
	if store == nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	policyStore := api.devEnvPolicyStore()
	if policyStore == nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	var record postgres.DevEnvironmentRecord
	var created bool
	if api.devEnvStoreOverride != nil || api.devEnvPolicyStoreOverride != nil {
		record, created, err = store.Create(r.Context(), devEnv, idempotencyKey)
		if err != nil {
			api.writeError(w, r, http.StatusInternalServerError, "internal_error")
			return
		}
		if !created && !devEnvIdempotencyMatch(record.Environment, devEnv) {
			api.writeError(w, r, http.StatusConflict, "idempotency_conflict")
			return
		}
		if created {
			if err := policyStore.Insert(r.Context(), devEnv.ID, projectID, policySnapshot, policyJSON, now, identity.Subject, policyIntegrity); err != nil {
				api.writeError(w, r, http.StatusInternalServerError, "internal_error")
				return
			}
			if err := api.appendDevEnvAudit(r.Context(), auditlog.Event{
				OccurredAt:   now,
				Actor:        identity.Subject,
				Action:       auditDevEnvCreated,
				ResourceType: "dev_environment",
				ResourceID:   devEnv.ID,
				RequestID:    r.Header.Get("X-Request-Id"),
				IP:           requestIP(r.RemoteAddr),
				UserAgent:    r.UserAgent(),
				Payload: map[string]any{
					"service":      "experiments",
					"project_id":   projectID,
					"template_ref": devEnv.TemplateRef,
					"image_name":   devEnv.ImageName,
					"repo_host":    repoInfo.Host,
					"repo_path":    repoInfo.Path,
					"ref_type":     devEnv.RefType,
					"ref_value":    devEnv.RefValue,
					"commit_pin":   devEnv.CommitPin,
					"ttl_seconds":  devEnv.TTLSeconds,
					"state":        devEnv.State,
				},
			}); err != nil {
				api.writeError(w, r, http.StatusInternalServerError, "audit_failed")
				return
			}
		}
	} else {
		tx, err := api.db.BeginTx(r.Context(), nil)
		if err != nil {
			api.writeError(w, r, http.StatusInternalServerError, "internal_error")
			return
		}
		defer func() { _ = tx.Rollback() }()

		txStore := postgres.NewDevEnvironmentStore(tx)
		if txStore == nil {
			api.writeError(w, r, http.StatusInternalServerError, "internal_error")
			return
		}
		txPolicyStore := postgres.NewDevEnvPolicyStore(tx)
		if txPolicyStore == nil {
			api.writeError(w, r, http.StatusInternalServerError, "internal_error")
			return
		}

		record, created, err = txStore.Create(r.Context(), devEnv, idempotencyKey)
		if err != nil {
			api.writeError(w, r, http.StatusInternalServerError, "internal_error")
			return
		}
		if !created && !devEnvIdempotencyMatch(record.Environment, devEnv) {
			api.writeError(w, r, http.StatusConflict, "idempotency_conflict")
			return
		}
		if created {
			if err := txPolicyStore.Insert(r.Context(), devEnv.ID, projectID, policySnapshot, policyJSON, now, identity.Subject, policyIntegrity); err != nil {
				api.writeError(w, r, http.StatusInternalServerError, "internal_error")
				return
			}
			if _, err := auditlog.Insert(r.Context(), tx, auditlog.Event{
				OccurredAt:   now,
				Actor:        identity.Subject,
				Action:       auditDevEnvCreated,
				ResourceType: "dev_environment",
				ResourceID:   devEnv.ID,
				RequestID:    r.Header.Get("X-Request-Id"),
				IP:           requestIP(r.RemoteAddr),
				UserAgent:    r.UserAgent(),
				Payload: map[string]any{
					"service":      "experiments",
					"project_id":   projectID,
					"template_ref": devEnv.TemplateRef,
					"image_name":   devEnv.ImageName,
					"repo_host":    repoInfo.Host,
					"repo_path":    repoInfo.Path,
					"ref_type":     devEnv.RefType,
					"ref_value":    devEnv.RefValue,
					"commit_pin":   devEnv.CommitPin,
					"ttl_seconds":  devEnv.TTLSeconds,
					"state":        devEnv.State,
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
	}
	client, err := api.devEnvDataplaneClient()
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "dataplane_unavailable")
		return
	}

	provisionResp, statusCode, err := client.ProvisionDevEnv(r.Context(), dataplane.DevEnvProvisionRequest{
		DevEnvID:             record.Environment.ID,
		ProjectID:            projectID,
		TemplateRef:          record.Environment.TemplateRef,
		TemplateVersion:      record.Environment.TemplateDefinitionVersion,
		TemplateIntegritySHA: record.Environment.TemplateIntegritySHA256,
		RepoURL:              record.Environment.RepoURL,
		RefType:              record.Environment.RefType,
		RefValue:             record.Environment.RefValue,
		CommitPin:            record.Environment.CommitPin,
		ImageName:            record.Environment.ImageName,
		ImageRef:             record.Environment.ImageRef,
		ResourceDefaults:     defRecord.Definition.ResourceDefaults,
		ResourceLimits:       defRecord.Definition.ResourceLimits,
		NetworkClassRef:      defRecord.Definition.NetworkClassRef,
		SecretAccessClassRef: defRecord.Definition.SecretAccessClassRef,
		TTLSeconds:           record.Environment.TTLSeconds,
		EmittedAt:            time.Now().UTC(),
		RequestedBy:          identity.Subject,
		CorrelationID:        r.Header.Get("X-Request-Id"),
	}, r.Header.Get("X-Request-Id"))
	if err != nil || !provisionResp.Accepted {
		_, _ = store.UpdateState(r.Context(), projectID, record.Environment.ID, domain.DevEnvStateFailed, provisionResp.JobName, provisionResp.Namespace)
		_ = api.appendDevEnvAudit(r.Context(), auditlog.Event{
			OccurredAt:   time.Now().UTC(),
			Actor:        identity.Subject,
			Action:       auditDevEnvProvisionFail,
			ResourceType: "dev_environment",
			ResourceID:   record.Environment.ID,
			RequestID:    r.Header.Get("X-Request-Id"),
			IP:           requestIP(r.RemoteAddr),
			UserAgent:    r.UserAgent(),
			Payload: map[string]any{
				"service":      "experiments",
				"project_id":   projectID,
				"template_ref": record.Environment.TemplateRef,
				"status_code":  statusCode,
			},
		})
		api.writeError(w, r, http.StatusBadGateway, "devenv_provision_failed")
		return
	}

	if _, err := store.UpdateState(r.Context(), projectID, record.Environment.ID, domain.DevEnvStateActive, provisionResp.JobName, provisionResp.Namespace); err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	if err := api.appendDevEnvAudit(r.Context(), auditlog.Event{
		OccurredAt:   time.Now().UTC(),
		Actor:        identity.Subject,
		Action:       auditDevEnvProvisioned,
		ResourceType: "dev_environment",
		ResourceID:   record.Environment.ID,
		RequestID:    r.Header.Get("X-Request-Id"),
		IP:           requestIP(r.RemoteAddr),
		UserAgent:    r.UserAgent(),
		Payload: map[string]any{
			"service":    "experiments",
			"project_id": projectID,
			"job_name":   provisionResp.JobName,
			"namespace":  provisionResp.Namespace,
		},
	}); err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "audit_failed")
		return
	}
	_ = api.appendDevEnvAudit(r.Context(), auditlog.Event{
		OccurredAt:   time.Now().UTC(),
		Actor:        identity.Subject,
		Action:       auditDevEnvRepoCloned,
		ResourceType: "dev_environment",
		ResourceID:   record.Environment.ID,
		RequestID:    r.Header.Get("X-Request-Id"),
		IP:           requestIP(r.RemoteAddr),
		UserAgent:    r.UserAgent(),
		Payload: map[string]any{
			"service":    "experiments",
			"project_id": projectID,
			"repo_host":  repoInfo.Host,
			"repo_path":  repoInfo.Path,
			"ref_type":   record.Environment.RefType,
			"ref_value":  record.Environment.RefValue,
			"commit_pin": record.Environment.CommitPin,
		},
	})

	record.Environment.State = domain.DevEnvStateActive
	record.Environment.DPJobName = provisionResp.JobName
	record.Environment.DPNamespace = provisionResp.Namespace

	api.writeJSON(w, http.StatusOK, devEnvResponse{
		Environment: record.Environment,
		Created:     created,
	})
}

func (api *experimentsAPI) handleListDevEnvironments(w http.ResponseWriter, r *http.Request) {
	projectID := strings.TrimSpace(r.PathValue("project_id"))
	if projectID == "" {
		api.writeError(w, r, http.StatusBadRequest, "project_id_required")
		return
	}
	state := strings.TrimSpace(r.URL.Query().Get("state"))
	limit := clampInt(parseIntQuery(r, "limit", 100), 1, 500)

	store := api.devEnvStore()
	if store == nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	records, err := store.List(r.Context(), projectID, state, limit)
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	out := make([]domain.DevEnvironment, 0, len(records))
	for _, record := range records {
		out = append(out, record.Environment)
	}
	api.writeJSON(w, http.StatusOK, devEnvListResponse{Environments: out})
}

func (api *experimentsAPI) handleGetDevEnvironment(w http.ResponseWriter, r *http.Request) {
	projectID := strings.TrimSpace(r.PathValue("project_id"))
	devEnvID := strings.TrimSpace(r.PathValue("dev_env_id"))
	if projectID == "" {
		api.writeError(w, r, http.StatusBadRequest, "project_id_required")
		return
	}
	if devEnvID == "" {
		api.writeError(w, r, http.StatusBadRequest, "dev_env_id_required")
		return
	}

	store := api.devEnvStore()
	if store == nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	record, err := store.Get(r.Context(), projectID, devEnvID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			api.writeError(w, r, http.StatusNotFound, "not_found")
			return
		}
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	api.writeJSON(w, http.StatusOK, record.Environment)
}

func (api *experimentsAPI) handleAccessDevEnvironment(w http.ResponseWriter, r *http.Request) {
	identity, ok := auth.IdentityFromContext(r.Context())
	if !ok || strings.TrimSpace(identity.Subject) == "" {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	if rbac.IsRunToken(identity) {
		api.writeError(w, r, http.StatusForbidden, "forbidden")
		return
	}

	projectID := strings.TrimSpace(r.PathValue("project_id"))
	devEnvID := strings.TrimSpace(r.PathValue("dev_env_id"))
	if projectID == "" {
		api.writeError(w, r, http.StatusBadRequest, "project_id_required")
		return
	}
	if devEnvID == "" {
		api.writeError(w, r, http.StatusBadRequest, "dev_env_id_required")
		return
	}

	var req devEnvAccessRequest
	if err := decodeJSON(r, &req); err != nil && !errors.Is(err, io.EOF) {
		api.writeError(w, r, http.StatusBadRequest, "invalid_json")
		return
	}

	store := api.devEnvStore()
	if store == nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	record, err := store.Get(r.Context(), projectID, devEnvID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			api.writeError(w, r, http.StatusNotFound, "not_found")
			return
		}
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	if record.Environment.State != domain.DevEnvStateActive {
		api.writeError(w, r, http.StatusConflict, "devenv_not_active")
		return
	}

	sessionTTL := req.TTLSeconds
	if sessionTTL <= 0 && api.devEnvAccessTTL > 0 {
		sessionTTL = int64(api.devEnvAccessTTL.Seconds())
	}
	if sessionTTL <= 0 {
		api.writeError(w, r, http.StatusBadRequest, "ttl_seconds_required")
		return
	}

	now := time.Now().UTC()
	expiresAt := now.Add(time.Duration(sessionTTL) * time.Second)
	session := domain.DevEnvAccessSession{
		SessionID: uuid.NewString(),
		ProjectID: projectID,
		DevEnvID:  devEnvID,
		IssuedAt:  now,
		ExpiresAt: expiresAt,
		IssuedBy:  strings.TrimSpace(identity.Subject),
	}
	sessionIntegrity, err := devEnvSessionIntegrity(projectID, session)
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "integrity_failed")
		return
	}

	sessionStore := api.devEnvSessionStore()
	if sessionStore == nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	if err := sessionStore.Insert(r.Context(), session, sessionIntegrity); err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	_, _ = store.UpdateLastAccess(r.Context(), projectID, devEnvID, now, 0)

	if err := api.appendDevEnvAudit(r.Context(), auditlog.Event{
		OccurredAt:   now,
		Actor:        identity.Subject,
		Action:       auditDevEnvSessionOpened,
		ResourceType: "dev_environment",
		ResourceID:   devEnvID,
		RequestID:    r.Header.Get("X-Request-Id"),
		IP:           requestIP(r.RemoteAddr),
		UserAgent:    r.UserAgent(),
		Payload: map[string]any{
			"service":    "experiments",
			"project_id": projectID,
			"session_id": session.SessionID,
			"expires_at": expiresAt.UTC().Format(time.RFC3339),
		},
	}); err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "audit_failed")
		return
	}

	client, err := api.devEnvDataplaneClient()
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "dataplane_unavailable")
		return
	}
	accessResp, _, err := client.AccessDevEnv(r.Context(), dataplane.DevEnvAccessRequest{
		DevEnvID:      devEnvID,
		ProjectID:     projectID,
		SessionID:     session.SessionID,
		EmittedAt:     now,
		CorrelationID: r.Header.Get("X-Request-Id"),
	}, r.Header.Get("X-Request-Id"))
	if err != nil {
		api.writeError(w, r, http.StatusBadGateway, "devenv_access_failed")
		return
	}

	api.writeJSON(w, http.StatusOK, devEnvAccessResponse{
		DevEnvID:  devEnvID,
		ProjectID: projectID,
		SessionID: session.SessionID,
		ExpiresAt: expiresAt,
		ProxyPath: "/devenv-sessions/" + session.SessionID + "/proxy/",
		Status:    boolToStatus(accessResp.Ready),
		Message:   accessResp.Message,
	})
}

func (api *experimentsAPI) handleStopDevEnvironment(w http.ResponseWriter, r *http.Request) {
	identity, ok := auth.IdentityFromContext(r.Context())
	if !ok || strings.TrimSpace(identity.Subject) == "" {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	if rbac.IsRunToken(identity) {
		api.writeError(w, r, http.StatusForbidden, "forbidden")
		return
	}

	projectID := strings.TrimSpace(r.PathValue("project_id"))
	devEnvID := strings.TrimSpace(r.PathValue("dev_env_id"))
	if projectID == "" {
		api.writeError(w, r, http.StatusBadRequest, "project_id_required")
		return
	}
	if devEnvID == "" {
		api.writeError(w, r, http.StatusBadRequest, "dev_env_id_required")
		return
	}

	store := api.devEnvStore()
	if store == nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	record, err := store.Get(r.Context(), projectID, devEnvID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			api.writeError(w, r, http.StatusNotFound, "not_found")
			return
		}
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	client, err := api.devEnvDataplaneClient()
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "dataplane_unavailable")
		return
	}
	_, _, err = client.DeleteDevEnv(r.Context(), dataplane.DevEnvDeleteRequest{
		DevEnvID:      devEnvID,
		ProjectID:     projectID,
		EmittedAt:     time.Now().UTC(),
		RequestedBy:   identity.Subject,
		CorrelationID: r.Header.Get("X-Request-Id"),
	}, r.Header.Get("X-Request-Id"))
	if err != nil {
		api.writeError(w, r, http.StatusBadGateway, "devenv_delete_failed")
		return
	}
	if _, err := store.UpdateState(r.Context(), projectID, devEnvID, domain.DevEnvStateDeleted, record.Environment.DPJobName, record.Environment.DPNamespace); err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	if err := api.appendDevEnvAudit(r.Context(), auditlog.Event{
		OccurredAt:   time.Now().UTC(),
		Actor:        identity.Subject,
		Action:       auditDevEnvDeleted,
		ResourceType: "dev_environment",
		ResourceID:   devEnvID,
		RequestID:    r.Header.Get("X-Request-Id"),
		IP:           requestIP(r.RemoteAddr),
		UserAgent:    r.UserAgent(),
		Payload: map[string]any{
			"service":    "experiments",
			"project_id": projectID,
		},
	}); err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "audit_failed")
		return
	}

	record.Environment.State = domain.DevEnvStateDeleted
	api.writeJSON(w, http.StatusOK, devEnvResponse{Environment: record.Environment, Created: false})
}

func resolveDevEnvImage(def domain.EnvironmentDefinition, requested string) (string, string, error) {
	if len(def.BaseImages) == 0 {
		return "", "", errors.New("no base images")
	}
	requested = strings.TrimSpace(requested)
	if requested == "" && len(def.BaseImages) == 1 {
		return def.BaseImages[0].Name, def.BaseImages[0].Ref, nil
	}
	for _, image := range def.BaseImages {
		if strings.EqualFold(strings.TrimSpace(image.Name), requested) {
			return image.Name, image.Ref, nil
		}
	}
	return "", "", errors.New("image not found")
}

func devEnvIntegrity(env domain.DevEnvironment) (string, error) {
	type integrityInput struct {
		DevEnvID                string    `json:"dev_env_id"`
		ProjectID               string    `json:"project_id"`
		TemplateRef             string    `json:"template_ref"`
		TemplateVersion         int       `json:"template_version"`
		TemplateIntegritySHA256 string    `json:"template_integrity_sha256"`
		RepoURL                 string    `json:"repo_url"`
		RefType                 string    `json:"ref_type"`
		RefValue                string    `json:"ref_value"`
		CommitPin               string    `json:"commit_pin"`
		ImageName               string    `json:"image_name"`
		ImageRef                string    `json:"image_ref"`
		TTLSeconds              int64     `json:"ttl_seconds"`
		CreatedAt               time.Time `json:"created_at"`
		CreatedBy               string    `json:"created_by"`
		ExpiresAt               time.Time `json:"expires_at"`
		PolicySnapshotSHA256    string    `json:"policy_snapshot_sha256"`
	}

	input := integrityInput{
		DevEnvID:                strings.TrimSpace(env.ID),
		ProjectID:               strings.TrimSpace(env.ProjectID),
		TemplateRef:             strings.TrimSpace(env.TemplateRef),
		TemplateVersion:         env.TemplateDefinitionVersion,
		TemplateIntegritySHA256: strings.TrimSpace(env.TemplateIntegritySHA256),
		RepoURL:                 strings.TrimSpace(env.RepoURL),
		RefType:                 strings.TrimSpace(env.RefType),
		RefValue:                strings.TrimSpace(env.RefValue),
		CommitPin:               strings.TrimSpace(env.CommitPin),
		ImageName:               strings.TrimSpace(env.ImageName),
		ImageRef:                strings.TrimSpace(env.ImageRef),
		TTLSeconds:              env.TTLSeconds,
		CreatedAt:               env.CreatedAt.UTC(),
		CreatedBy:               strings.TrimSpace(env.CreatedBy),
		ExpiresAt:               env.ExpiresAt.UTC(),
		PolicySnapshotSHA256:    strings.TrimSpace(env.PolicySnapshotSHA256),
	}
	return integritySHA256(input)
}

func devEnvPolicyIntegrity(projectID, devEnvID, snapshotSHA string, createdAt time.Time, createdBy string) (string, error) {
	type integrityInput struct {
		ProjectID   string    `json:"project_id"`
		DevEnvID    string    `json:"dev_env_id"`
		SnapshotSHA string    `json:"snapshot_sha256"`
		CreatedAt   time.Time `json:"created_at"`
		CreatedBy   string    `json:"created_by"`
	}
	input := integrityInput{
		ProjectID:   strings.TrimSpace(projectID),
		DevEnvID:    strings.TrimSpace(devEnvID),
		SnapshotSHA: strings.TrimSpace(snapshotSHA),
		CreatedAt:   createdAt.UTC(),
		CreatedBy:   strings.TrimSpace(createdBy),
	}
	return integritySHA256(input)
}

func devEnvSessionIntegrity(projectID string, session domain.DevEnvAccessSession) (string, error) {
	type integrityInput struct {
		ProjectID string    `json:"project_id"`
		DevEnvID  string    `json:"dev_env_id"`
		SessionID string    `json:"session_id"`
		IssuedAt  time.Time `json:"issued_at"`
		ExpiresAt time.Time `json:"expires_at"`
		IssuedBy  string    `json:"issued_by"`
	}
	input := integrityInput{
		ProjectID: strings.TrimSpace(projectID),
		DevEnvID:  strings.TrimSpace(session.DevEnvID),
		SessionID: strings.TrimSpace(session.SessionID),
		IssuedAt:  session.IssuedAt.UTC(),
		ExpiresAt: session.ExpiresAt.UTC(),
		IssuedBy:  strings.TrimSpace(session.IssuedBy),
	}
	return integritySHA256(input)
}

func devEnvIdempotencyMatch(existing, requested domain.DevEnvironment) bool {
	if strings.TrimSpace(existing.ProjectID) != strings.TrimSpace(requested.ProjectID) {
		return false
	}
	if strings.TrimSpace(existing.TemplateRef) != strings.TrimSpace(requested.TemplateRef) {
		return false
	}
	if existing.TemplateDefinitionVersion != requested.TemplateDefinitionVersion {
		return false
	}
	if strings.TrimSpace(existing.TemplateIntegritySHA256) != strings.TrimSpace(requested.TemplateIntegritySHA256) {
		return false
	}
	if strings.TrimSpace(existing.RepoURL) != strings.TrimSpace(requested.RepoURL) {
		return false
	}
	if strings.TrimSpace(existing.RefType) != strings.TrimSpace(requested.RefType) {
		return false
	}
	if strings.TrimSpace(existing.RefValue) != strings.TrimSpace(requested.RefValue) {
		return false
	}
	if strings.TrimSpace(existing.CommitPin) != strings.TrimSpace(requested.CommitPin) {
		return false
	}
	if strings.TrimSpace(existing.ImageName) != strings.TrimSpace(requested.ImageName) {
		return false
	}
	if strings.TrimSpace(existing.ImageRef) != strings.TrimSpace(requested.ImageRef) {
		return false
	}
	if existing.TTLSeconds != requested.TTLSeconds {
		return false
	}
	if strings.TrimSpace(existing.PolicySnapshotSHA256) != strings.TrimSpace(requested.PolicySnapshotSHA256) {
		return false
	}
	return true
}

func boolToStatus(ready bool) string {
	if ready {
		return "ready"
	}
	return "not_ready"
}
