package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/animus-labs/animus-go/closed/internal/integrations/webhooks"
	"github.com/animus-labs/animus-go/closed/internal/platform/auditlog"
	"github.com/animus-labs/animus-go/closed/internal/platform/auth"
	"github.com/animus-labs/animus-go/closed/internal/platform/lineageevent"
	"github.com/animus-labs/animus-go/closed/internal/platform/objectstore"
	"github.com/animus-labs/animus-go/closed/internal/platform/redaction"
	"github.com/animus-labs/animus-go/closed/internal/repo"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/minio/minio-go/v7"
)

type experimentsAPI struct {
	logger   *slog.Logger
	db       *sql.DB
	store    *minio.Client
	storeCfg objectstore.Config

	ciWebhookSecret  string
	ciWebhookMaxSkew time.Duration

	runTokenSecret string
	runTokenTTL    time.Duration
	datapilotURL   string
	dataplaneURL   string

	evidenceSigningSecret string
	gitlabWebhookSecret   string

	trainingExecutor  trainingExecutor
	trainingNamespace string

	webhookConfig webhooks.Config
}

func newExperimentsAPI(
	logger *slog.Logger,
	db *sql.DB,
	store *minio.Client,
	storeCfg objectstore.Config,
	ciWebhookSecret string,
	runTokenSecret string,
	runTokenTTL time.Duration,
	datapilotURL string,
	dataplaneURL string,
	evidenceSigningSecret string,
	gitlabWebhookSecret string,
	trainingExecutor trainingExecutor,
	trainingNamespace string,
	webhookConfig webhooks.Config,
) *experimentsAPI {
	return &experimentsAPI{
		logger:                logger,
		db:                    db,
		store:                 store,
		storeCfg:              storeCfg,
		ciWebhookSecret:       strings.TrimSpace(ciWebhookSecret),
		ciWebhookMaxSkew:      5 * time.Minute,
		runTokenSecret:        strings.TrimSpace(runTokenSecret),
		runTokenTTL:           runTokenTTL,
		datapilotURL:          strings.TrimSpace(datapilotURL),
		dataplaneURL:          strings.TrimSpace(dataplaneURL),
		evidenceSigningSecret: strings.TrimSpace(evidenceSigningSecret),
		gitlabWebhookSecret:   strings.TrimSpace(gitlabWebhookSecret),
		trainingExecutor:      trainingExecutor,
		trainingNamespace:     strings.TrimSpace(trainingNamespace),
		webhookConfig:         webhookConfig,
	}
}

func (api *experimentsAPI) register(mux *http.ServeMux) {
	mux.HandleFunc("GET /experiments", api.handleListExperiments)
	mux.HandleFunc("POST /experiments", api.handleCreateExperiment)
	mux.HandleFunc("GET /experiments/{experiment_id}", api.handleGetExperiment)

	mux.HandleFunc("POST /projects/{project_id}/runs", api.handleCreateRun)
	mux.HandleFunc("GET /projects/{project_id}/runs/{run_id}", api.handleGetRun)
	mux.HandleFunc("GET /projects/{project_id}/runs/{run_id}/policy-snapshot", api.handleGetRunPolicySnapshot)
	mux.HandleFunc("GET /projects/{project_id}/runs/{run_id}/reproducibility-bundle", api.handleGetRunReproducibilityBundle)
	mux.HandleFunc("POST /projects/{project_id}/runs/{run_id}:dispatch", api.handleDispatchRun)
	mux.HandleFunc("POST /projects/{project_id}/runs/{run_id}:plan", api.handlePlanRun)
	mux.HandleFunc("GET /projects/{project_id}/runs/{run_id}:plan", api.handleGetRunPlan)
	mux.HandleFunc("POST /projects/{project_id}/runs/{run_id}:dry-run", api.handleDryRun)
	mux.HandleFunc("GET /projects/{project_id}/runs/{run_id}:dry-run", api.handleGetDryRun)
	mux.HandleFunc("POST /projects/{project_id}/role-bindings", api.handleUpsertRoleBinding)
	mux.HandleFunc("GET /projects/{project_id}/role-bindings", api.handleListRoleBindings)
	mux.HandleFunc("POST /projects/{project_id}/role-bindings/{binding_id}:delete", api.handleDeleteRoleBinding)

	mux.HandleFunc("POST /projects/{project_id}/environment-definitions", api.handleCreateEnvironmentDefinition)
	mux.HandleFunc("GET /projects/{project_id}/environment-definitions", api.handleListEnvironmentDefinitions)
	mux.HandleFunc("GET /projects/{project_id}/environment-definitions/{definition_id}", api.handleGetEnvironmentDefinition)
	mux.HandleFunc("POST /projects/{project_id}/environment-definitions/{definition_id}:update", api.handleUpdateEnvironmentDefinition)
	mux.HandleFunc("POST /projects/{project_id}/environment-definitions/{definition_id}:archive", api.handleArchiveEnvironmentDefinition)
	mux.HandleFunc("POST /projects/{project_id}/environment-locks", api.handleCreateEnvironmentLock)
	mux.HandleFunc("GET /projects/{project_id}/environment-locks", api.handleListEnvironmentLocks)
	mux.HandleFunc("GET /projects/{project_id}/environment-locks/{lock_id}", api.handleGetEnvironmentLock)
	mux.HandleFunc("POST /projects/{project_id}/webhooks/subscriptions", api.handleCreateWebhookSubscription)
	mux.HandleFunc("GET /projects/{project_id}/webhooks/subscriptions", api.handleListWebhookSubscriptions)
	mux.HandleFunc("PATCH /projects/{project_id}/webhooks/subscriptions/{subscription_id}", api.handleUpdateWebhookSubscription)
	mux.HandleFunc("PUT /projects/{project_id}/webhooks/subscriptions/{subscription_id}", api.handleUpdateWebhookSubscription)
	mux.HandleFunc("GET /projects/{project_id}/webhooks/deliveries", api.handleListWebhookDeliveries)
	mux.HandleFunc("GET /projects/{project_id}/webhooks/deliveries/{delivery_id}/attempts", api.handleListWebhookDeliveryAttempts)
	mux.HandleFunc("POST /projects/{project_id}/webhooks/deliveries/{delivery_id}:replay", api.handleReplayWebhookDelivery)

	mux.HandleFunc("GET /experiments/{experiment_id}/runs", api.handleListExperimentRuns)
	mux.HandleFunc("POST /experiments/{experiment_id}/runs", api.handleCreateExperimentRun)
	mux.HandleFunc("POST /experiments/runs:execute", api.handleExecuteExperimentRun)
	mux.HandleFunc("GET /experiment-runs", api.handleListAllExperimentRuns)
	mux.HandleFunc("GET /experiment-runs/{run_id}", api.handleGetExperimentRun)
	mux.HandleFunc("GET /experiment-runs/{run_id}/metrics", api.handleListExperimentRunMetrics)
	mux.HandleFunc("POST /experiment-runs/{run_id}/metrics", api.handleIngestExperimentRunMetrics)
	mux.HandleFunc("GET /experiment-runs/{run_id}/artifacts", api.handleListExperimentRunArtifacts)
	mux.HandleFunc("POST /experiment-runs/{run_id}/artifacts", api.handleCreateExperimentRunArtifact)
	mux.HandleFunc("GET /experiment-runs/{run_id}/artifacts/{artifact_id}", api.handleGetExperimentRunArtifact)
	mux.HandleFunc("GET /experiment-runs/{run_id}/artifacts/{artifact_id}/download", api.handleDownloadExperimentRunArtifact)
	mux.HandleFunc("GET /experiment-runs/{run_id}/evidence-bundles", api.handleListEvidenceBundles)
	mux.HandleFunc("POST /experiment-runs/{run_id}/evidence-bundles", api.handleCreateEvidenceBundle)
	mux.HandleFunc("GET /experiment-runs/{run_id}/evidence-bundles/{bundle_id}", api.handleGetEvidenceBundle)
	mux.HandleFunc("GET /experiment-runs/{run_id}/evidence-bundles/{bundle_id}/download", api.handleDownloadEvidenceBundle)
	mux.HandleFunc("GET /experiment-runs/{run_id}/evidence-bundles/{bundle_id}/report", api.handleDownloadEvidenceReport)
	mux.HandleFunc("GET /experiment-runs/{run_id}/stream", api.handleStreamExperimentRun)
	mux.HandleFunc("GET /experiment-runs/{run_id}/events", api.handleListExperimentRunEvents)
	mux.HandleFunc("POST /experiment-runs/{run_id}/events", api.handleCreateExperimentRunEvent)
	mux.HandleFunc("GET /experiment-runs/{run_id}/execution", api.handleGetExperimentRunExecution)
	mux.HandleFunc("GET /experiment-runs/{run_id}/build-context", api.handleGetExperimentRunBuildContext)
	mux.HandleFunc("GET /execution-ledger", api.handleListExecutionLedger)
	mux.HandleFunc("GET /execution-ledger/{run_id}", api.handleGetExecutionLedger)
	mux.HandleFunc("POST /gitlab/webhook", api.handleGitlabWebhook)
	mux.HandleFunc("POST /internal/cp/runs/{run_id}/heartbeat", api.handleDPHeartbeat)
	mux.HandleFunc("POST /internal/cp/runs/{run_id}/terminal", api.handleDPTerminal)
	mux.HandleFunc("POST /internal/cp/runs/{run_id}/artifact-committed", api.handleDPArtifactCommitted)
	mux.HandleFunc("POST /internal/cp/runs/{run_id}/secrets-accessed", api.handleDPSecretAccessed)

	mux.HandleFunc("GET /policies", api.handleListPolicies)
	mux.HandleFunc("POST /policies", api.handleCreatePolicy)
	mux.HandleFunc("GET /policies/{policy_id}", api.handleGetPolicy)
	mux.HandleFunc("GET /policies/{policy_id}/versions", api.handleListPolicyVersions)
	mux.HandleFunc("POST /policies/{policy_id}/versions", api.handleCreatePolicyVersion)
	mux.HandleFunc("GET /policy-decisions", api.handleListPolicyDecisions)
	mux.HandleFunc("GET /policy-decisions/{decision_id}", api.handleGetPolicyDecision)
	mux.HandleFunc("GET /policy-approvals", api.handleListPolicyApprovals)
	mux.HandleFunc("GET /policy-approvals/{approval_id}", api.handleGetPolicyApproval)
	mux.HandleFunc("POST /policy-approvals/{approval_id}/approve", api.handleApprovePolicyApproval)
	mux.HandleFunc("POST /policy-approvals/{approval_id}/deny", api.handleDenyPolicyApproval)

	mux.HandleFunc("POST /ci/webhook", api.handleCIWebhook)
	mux.HandleFunc("POST /ci/report", api.handleCIReport)
	mux.HandleFunc("GET /model-images", api.handleListModelImages)
	mux.HandleFunc("GET /model-images/{image_digest}", api.handleGetModelImage)
}

type experiment struct {
	ExperimentID string          `json:"experiment_id"`
	Name         string          `json:"name"`
	Description  string          `json:"description,omitempty"`
	Metadata     json.RawMessage `json:"metadata"`
	CreatedAt    time.Time       `json:"created_at"`
	CreatedBy    string          `json:"created_by"`
}

type createExperimentRequest struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

func (api *experimentsAPI) handleCreateExperiment(w http.ResponseWriter, r *http.Request) {
	identity, ok := auth.IdentityFromContext(r.Context())
	if !ok || strings.TrimSpace(identity.Subject) == "" {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	projectID, ok := auth.ProjectIDFromContext(r.Context())
	if !ok || strings.TrimSpace(projectID) == "" {
		api.writeError(w, r, http.StatusBadRequest, "project_id_required")
		return
	}

	var req createExperimentRequest
	if err := decodeJSON(r, &req); err != nil {
		api.writeError(w, r, http.StatusBadRequest, "invalid_json")
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		api.writeError(w, r, http.StatusBadRequest, "name_required")
		return
	}
	description := strings.TrimSpace(req.Description)

	metadataMap := req.Metadata
	if metadataMap == nil {
		metadataMap = map[string]any{}
	}
	metadataMap = redaction.RedactMetadata(metadataMap)
	metadataJSON, err := json.Marshal(metadataMap)
	if err != nil {
		api.writeError(w, r, http.StatusBadRequest, "invalid_metadata")
		return
	}

	now := time.Now().UTC()
	experimentID := uuid.NewString()

	type integrityInput struct {
		ExperimentID string          `json:"experiment_id"`
		ProjectID    string          `json:"project_id"`
		Name         string          `json:"name"`
		Description  string          `json:"description,omitempty"`
		Metadata     json.RawMessage `json:"metadata"`
		CreatedAt    time.Time       `json:"created_at"`
		CreatedBy    string          `json:"created_by"`
	}
	integrity, err := integritySHA256(integrityInput{
		ExperimentID: experimentID,
		ProjectID:    projectID,
		Name:         name,
		Description:  description,
		Metadata:     metadataJSON,
		CreatedAt:    now,
		CreatedBy:    identity.Subject,
	})
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	tx, err := api.db.BeginTx(r.Context(), nil)
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.ExecContext(
		r.Context(),
		`INSERT INTO experiments (
			experiment_id,
			project_id,
			name,
			description,
			metadata,
			created_at,
			created_by,
			integrity_sha256
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		experimentID,
		projectID,
		name,
		nullString(description),
		metadataJSON,
		now,
		identity.Subject,
		integrity,
	)
	if err != nil {
		if isUniqueViolation(err) {
			api.writeError(w, r, http.StatusConflict, "experiment_name_exists")
			return
		}
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	_, err = auditlog.Insert(r.Context(), tx, auditlog.Event{
		OccurredAt:   now,
		Actor:        identity.Subject,
		Action:       "experiment.create",
		ResourceType: "experiment",
		ResourceID:   experimentID,
		RequestID:    r.Header.Get("X-Request-Id"),
		IP:           requestIP(r.RemoteAddr),
		UserAgent:    r.UserAgent(),
		Payload: map[string]any{
			"service":        "experiments",
			"experiment_id":  experimentID,
			"name":           name,
			"description":    description,
			"metadata":       metadataMap,
			"created_by":     identity.Subject,
			"request_path":   r.URL.Path,
			"request_method": r.Method,
		},
	})
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "audit_failed")
		return
	}

	if err := tx.Commit(); err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	w.Header().Set("Location", "/experiments/"+experimentID)
	api.writeJSON(w, http.StatusCreated, experiment{
		ExperimentID: experimentID,
		Name:         name,
		Description:  description,
		Metadata:     metadataJSON,
		CreatedAt:    now,
		CreatedBy:    identity.Subject,
	})
}

func (api *experimentsAPI) handleListExperiments(w http.ResponseWriter, r *http.Request) {
	projectID, ok := auth.ProjectIDFromContext(r.Context())
	if !ok || strings.TrimSpace(projectID) == "" {
		api.writeError(w, r, http.StatusBadRequest, "project_id_required")
		return
	}
	limit := clampInt(parseIntQuery(r, "limit", 100), 1, 500)
	nameFilter := strings.TrimSpace(r.URL.Query().Get("name"))

	var (
		rows *sql.Rows
		err  error
	)
	if nameFilter != "" {
		rows, err = api.db.QueryContext(
			r.Context(),
			`SELECT experiment_id, name, description, metadata, created_at, created_by
			 FROM experiments
			 WHERE project_id = $1 AND name = $2
			 ORDER BY created_at DESC
			 LIMIT $3`,
			projectID,
			nameFilter,
			limit,
		)
	} else {
		rows, err = api.db.QueryContext(
			r.Context(),
			`SELECT experiment_id, name, description, metadata, created_at, created_by
			 FROM experiments
			 WHERE project_id = $1
			 ORDER BY created_at DESC
			 LIMIT $2`,
			projectID,
			limit,
		)
	}
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	defer rows.Close()

	out := make([]experiment, 0, limit)
	for rows.Next() {
		var (
			experimentID string
			name         string
			description  sql.NullString
			metadata     []byte
			createdAt    time.Time
			createdBy    string
		)
		if err := rows.Scan(&experimentID, &name, &description, &metadata, &createdAt, &createdBy); err != nil {
			api.writeError(w, r, http.StatusInternalServerError, "internal_error")
			return
		}
		out = append(out, experiment{
			ExperimentID: experimentID,
			Name:         name,
			Description:  description.String,
			Metadata:     normalizeJSON(metadata),
			CreatedAt:    createdAt,
			CreatedBy:    createdBy,
		})
	}
	if err := rows.Err(); err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	api.writeJSON(w, http.StatusOK, map[string]any{"experiments": out})
}

func (api *experimentsAPI) handleGetExperiment(w http.ResponseWriter, r *http.Request) {
	projectID, ok := auth.ProjectIDFromContext(r.Context())
	if !ok || strings.TrimSpace(projectID) == "" {
		api.writeError(w, r, http.StatusBadRequest, "project_id_required")
		return
	}

	experimentID := strings.TrimSpace(r.PathValue("experiment_id"))
	if experimentID == "" {
		api.writeError(w, r, http.StatusBadRequest, "experiment_id_required")
		return
	}

	var (
		name        string
		description sql.NullString
		metadata    []byte
		createdAt   time.Time
		createdBy   string
	)
	err := api.db.QueryRowContext(
		r.Context(),
		`SELECT name, description, metadata, created_at, created_by
		 FROM experiments
		 WHERE experiment_id = $1 AND project_id = $2`,
		experimentID,
		projectID,
	).Scan(&name, &description, &metadata, &createdAt, &createdBy)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			api.writeError(w, r, http.StatusNotFound, "not_found")
			return
		}
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	api.writeJSON(w, http.StatusOK, experiment{
		ExperimentID: experimentID,
		Name:         name,
		Description:  description.String,
		Metadata:     normalizeJSON(metadata),
		CreatedAt:    createdAt,
		CreatedBy:    createdBy,
	})
}

type experimentRun struct {
	RunID            string          `json:"run_id"`
	ExperimentID     string          `json:"experiment_id"`
	DatasetVersionID string          `json:"dataset_version_id,omitempty"`
	Status           string          `json:"status"`
	StartedAt        time.Time       `json:"started_at"`
	EndedAt          *time.Time      `json:"ended_at,omitempty"`
	GitRepo          string          `json:"git_repo,omitempty"`
	GitCommit        string          `json:"git_commit,omitempty"`
	GitRef           string          `json:"git_ref,omitempty"`
	Params           json.RawMessage `json:"params"`
	Metrics          json.RawMessage `json:"metrics"`
	ArtifactsPrefix  string          `json:"artifacts_prefix,omitempty"`
}

type createExperimentRunRequest struct {
	DatasetVersionID string         `json:"dataset_version_id,omitempty"`
	Status           string         `json:"status"`
	StartedAt        *time.Time     `json:"started_at,omitempty"`
	EndedAt          *time.Time     `json:"ended_at,omitempty"`
	GitRepo          string         `json:"git_repo,omitempty"`
	GitCommit        string         `json:"git_commit,omitempty"`
	GitRef           string         `json:"git_ref,omitempty"`
	Params           map[string]any `json:"params,omitempty"`
	Metrics          map[string]any `json:"metrics,omitempty"`
	ArtifactsPrefix  string         `json:"artifacts_prefix,omitempty"`
}

var allowedRunStatuses = map[string]struct{}{
	"pending":   {},
	"running":   {},
	"succeeded": {},
	"failed":    {},
	"canceled":  {},
}

func (api *experimentsAPI) handleCreateExperimentRun(w http.ResponseWriter, r *http.Request) {
	identity, ok := auth.IdentityFromContext(r.Context())
	if !ok || strings.TrimSpace(identity.Subject) == "" {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	experimentID := strings.TrimSpace(r.PathValue("experiment_id"))
	if experimentID == "" {
		api.writeError(w, r, http.StatusBadRequest, "experiment_id_required")
		return
	}

	projectID, err := projectIDForExperiment(r.Context(), api.db, experimentID)
	if err != nil || strings.TrimSpace(projectID) == "" {
		api.writeError(w, r, http.StatusNotFound, "not_found")
		return
	}

	exists, err := api.experimentExists(r.Context(), experimentID)
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	if !exists {
		api.writeError(w, r, http.StatusNotFound, "not_found")
		return
	}

	var req createExperimentRunRequest
	if err := decodeJSON(r, &req); err != nil {
		api.writeError(w, r, http.StatusBadRequest, "invalid_json")
		return
	}

	status := strings.ToLower(strings.TrimSpace(req.Status))
	if status == "" {
		api.writeError(w, r, http.StatusBadRequest, "status_required")
		return
	}
	if _, ok := allowedRunStatuses[status]; !ok {
		api.writeError(w, r, http.StatusBadRequest, "invalid_status")
		return
	}

	datasetVersionID := strings.TrimSpace(req.DatasetVersionID)
	var gate gateDecision
	if datasetVersionID != "" {
		var ok bool
		gate, ok = api.requireQualityGatePass(w, r, identity, datasetVersionID, experimentID)
		if !ok {
			return
		}
	}

	now := time.Now().UTC()
	startedAt := now
	if req.StartedAt != nil && !req.StartedAt.IsZero() {
		startedAt = req.StartedAt.UTC()
	}
	var endedAt *time.Time
	if req.EndedAt != nil && !req.EndedAt.IsZero() {
		t := req.EndedAt.UTC()
		endedAt = &t
	}
	if endedAt != nil && endedAt.Before(startedAt) {
		api.writeError(w, r, http.StatusBadRequest, "ended_before_started")
		return
	}

	paramsMap := req.Params
	if paramsMap == nil {
		paramsMap = map[string]any{}
	}
	paramsJSON, err := json.Marshal(paramsMap)
	if err != nil {
		api.writeError(w, r, http.StatusBadRequest, "invalid_params")
		return
	}
	metricsMap := req.Metrics
	if metricsMap == nil {
		metricsMap = map[string]any{}
	}
	metricsJSON, err := json.Marshal(metricsMap)
	if err != nil {
		api.writeError(w, r, http.StatusBadRequest, "invalid_metrics")
		return
	}

	gitRepo := strings.TrimSpace(req.GitRepo)
	gitCommit := strings.TrimSpace(req.GitCommit)
	gitRef := strings.TrimSpace(req.GitRef)
	artifactsPrefix := strings.TrimSpace(req.ArtifactsPrefix)

	runID := uuid.NewString()

	type integrityInput struct {
		RunID            string          `json:"run_id"`
		ExperimentID     string          `json:"experiment_id"`
		ProjectID        string          `json:"project_id"`
		DatasetVersionID string          `json:"dataset_version_id,omitempty"`
		Status           string          `json:"status"`
		StartedAt        time.Time       `json:"started_at"`
		EndedAt          *time.Time      `json:"ended_at,omitempty"`
		GitRepo          string          `json:"git_repo,omitempty"`
		GitCommit        string          `json:"git_commit,omitempty"`
		GitRef           string          `json:"git_ref,omitempty"`
		Params           json.RawMessage `json:"params"`
		Metrics          json.RawMessage `json:"metrics"`
		ArtifactsPrefix  string          `json:"artifacts_prefix,omitempty"`
	}
	integrity, err := integritySHA256(integrityInput{
		RunID:            runID,
		ExperimentID:     experimentID,
		ProjectID:        projectID,
		DatasetVersionID: datasetVersionID,
		Status:           status,
		StartedAt:        startedAt,
		EndedAt:          endedAt,
		GitRepo:          gitRepo,
		GitCommit:        gitCommit,
		GitRef:           gitRef,
		Params:           paramsJSON,
		Metrics:          metricsJSON,
		ArtifactsPrefix:  artifactsPrefix,
	})
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	tx, err := api.db.BeginTx(r.Context(), nil)
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.ExecContext(
		r.Context(),
		`INSERT INTO experiment_runs (
			run_id,
			experiment_id,
			project_id,
			dataset_version_id,
			status,
			started_at,
			ended_at,
			git_repo,
			git_commit,
			git_ref,
			params,
			metrics,
			artifacts_prefix,
			integrity_sha256
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`,
		runID,
		experimentID,
		projectID,
		nullString(datasetVersionID),
		status,
		startedAt,
		nullTimePtr(endedAt),
		nullString(gitRepo),
		nullString(gitCommit),
		nullString(gitRef),
		paramsJSON,
		metricsJSON,
		nullString(artifactsPrefix),
		integrity,
	)
	if err != nil {
		if isForeignKeyViolation(err) {
			api.writeError(w, r, http.StatusNotFound, "not_found")
			return
		}
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	_, err = lineageevent.Insert(r.Context(), tx, lineageevent.Event{
		OccurredAt:  now,
		Actor:       identity.Subject,
		RequestID:   r.Header.Get("X-Request-Id"),
		SubjectType: "experiment",
		SubjectID:   experimentID,
		Predicate:   "has_run",
		ObjectType:  "experiment_run",
		ObjectID:    runID,
		Metadata: map[string]any{
			"status":             status,
			"dataset_version_id": datasetVersionID,
			"git_commit":         gitCommit,
		},
	})
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "lineage_write_failed")
		return
	}

	if datasetVersionID != "" {
		_, err = lineageevent.Insert(r.Context(), tx, lineageevent.Event{
			OccurredAt:  now,
			Actor:       identity.Subject,
			RequestID:   r.Header.Get("X-Request-Id"),
			SubjectType: "dataset_version",
			SubjectID:   datasetVersionID,
			Predicate:   "used_by",
			ObjectType:  "experiment_run",
			ObjectID:    runID,
			Metadata: map[string]any{
				"dataset_id":    gate.DatasetID,
				"experiment_id": experimentID,
				"rule_id":       gate.RuleID,
				"evaluation_id": gate.EvaluationID,
				"status":        gate.Status,
			},
		})
		if err != nil {
			api.writeError(w, r, http.StatusInternalServerError, "lineage_write_failed")
			return
		}
	}

	if gitCommit != "" {
		_, err = lineageevent.Insert(r.Context(), tx, lineageevent.Event{
			OccurredAt:  now,
			Actor:       identity.Subject,
			RequestID:   r.Header.Get("X-Request-Id"),
			SubjectType: "experiment_run",
			SubjectID:   runID,
			Predicate:   "built_from",
			ObjectType:  "git_commit",
			ObjectID:    gitCommit,
			Metadata: map[string]any{
				"git_repo": gitRepo,
				"git_ref":  gitRef,
			},
		})
		if err != nil {
			api.writeError(w, r, http.StatusInternalServerError, "lineage_write_failed")
			return
		}
	}

	if datasetVersionID != "" {
		_, err = auditlog.Insert(r.Context(), tx, auditlog.Event{
			OccurredAt:   now,
			Actor:        identity.Subject,
			Action:       "quality_gate.allow",
			ResourceType: "dataset_version",
			ResourceID:   datasetVersionID,
			RequestID:    r.Header.Get("X-Request-Id"),
			IP:           requestIP(r.RemoteAddr),
			UserAgent:    r.UserAgent(),
			Payload: map[string]any{
				"service":            "experiments",
				"dataset_id":         gate.DatasetID,
				"dataset_version_id": datasetVersionID,
				"rule_id":            gate.RuleID,
				"evaluation_id":      gate.EvaluationID,
				"status":             gate.Status,
				"experiment_id":      experimentID,
				"run_id":             runID,
			},
		})
		if err != nil {
			api.writeError(w, r, http.StatusInternalServerError, "audit_failed")
			return
		}
	}

	_, err = auditlog.Insert(r.Context(), tx, auditlog.Event{
		OccurredAt:   now,
		Actor:        identity.Subject,
		Action:       "experiment_run.create",
		ResourceType: "experiment_run",
		ResourceID:   runID,
		RequestID:    r.Header.Get("X-Request-Id"),
		IP:           requestIP(r.RemoteAddr),
		UserAgent:    r.UserAgent(),
		Payload: map[string]any{
			"service":            "experiments",
			"run_id":             runID,
			"experiment_id":      experimentID,
			"dataset_version_id": datasetVersionID,
			"status":             status,
			"started_at":         startedAt.Format(time.RFC3339Nano),
			"ended_at":           formatTimePtr(endedAt),
			"git_repo":           gitRepo,
			"git_commit":         gitCommit,
			"git_ref":            gitRef,
			"params":             paramsMap,
			"metrics":            metricsMap,
			"artifacts_prefix":   artifactsPrefix,
		},
	})
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "audit_failed")
		return
	}

	if err := tx.Commit(); err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	w.Header().Set("Location", "/experiment-runs/"+runID)
	api.writeJSON(w, http.StatusCreated, experimentRun{
		RunID:            runID,
		ExperimentID:     experimentID,
		DatasetVersionID: datasetVersionID,
		Status:           status,
		StartedAt:        startedAt,
		EndedAt:          endedAt,
		GitRepo:          gitRepo,
		GitCommit:        gitCommit,
		GitRef:           gitRef,
		Params:           paramsJSON,
		Metrics:          metricsJSON,
		ArtifactsPrefix:  artifactsPrefix,
	})
}

func (api *experimentsAPI) handleListExperimentRuns(w http.ResponseWriter, r *http.Request) {
	experimentID := strings.TrimSpace(r.PathValue("experiment_id"))
	if experimentID == "" {
		api.writeError(w, r, http.StatusBadRequest, "experiment_id_required")
		return
	}
	limit := clampInt(parseIntQuery(r, "limit", 100), 1, 500)

	rows, err := api.db.QueryContext(
		r.Context(),
		`SELECT r.run_id,
				r.dataset_version_id,
				COALESCE(s.status, r.status) AS status,
				r.started_at,
				COALESCE(r.ended_at, CASE WHEN s.status IN ('succeeded','failed','canceled') THEN s.observed_at END) AS ended_at,
				r.git_repo,
				r.git_commit,
				r.git_ref,
				r.params,
				r.metrics,
				r.artifacts_prefix
		 FROM experiment_runs r
		 LEFT JOIN LATERAL (
			SELECT status, observed_at
			FROM experiment_run_state_events
			WHERE run_id = r.run_id
			ORDER BY observed_at DESC
			LIMIT 1
		 ) s ON true
		 WHERE r.experiment_id = $1
		 ORDER BY r.started_at DESC
		 LIMIT $2`,
		experimentID,
		limit,
	)
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	defer rows.Close()

	out := make([]experimentRun, 0, limit)
	for rows.Next() {
		var (
			runID            string
			datasetVersionID sql.NullString
			status           string
			startedAt        time.Time
			endedAt          sql.NullTime
			gitRepo          sql.NullString
			gitCommit        sql.NullString
			gitRef           sql.NullString
			params           []byte
			metrics          []byte
			artifactsPrefix  sql.NullString
		)
		if err := rows.Scan(&runID, &datasetVersionID, &status, &startedAt, &endedAt, &gitRepo, &gitCommit, &gitRef, &params, &metrics, &artifactsPrefix); err != nil {
			api.writeError(w, r, http.StatusInternalServerError, "internal_error")
			return
		}

		var endedAtPtr *time.Time
		if endedAt.Valid && !endedAt.Time.IsZero() {
			t := endedAt.Time.UTC()
			endedAtPtr = &t
		}

		out = append(out, experimentRun{
			RunID:            runID,
			ExperimentID:     experimentID,
			DatasetVersionID: strings.TrimSpace(datasetVersionID.String),
			Status:           status,
			StartedAt:        startedAt,
			EndedAt:          endedAtPtr,
			GitRepo:          strings.TrimSpace(gitRepo.String),
			GitCommit:        strings.TrimSpace(gitCommit.String),
			GitRef:           strings.TrimSpace(gitRef.String),
			Params:           normalizeJSON(params),
			Metrics:          normalizeJSON(metrics),
			ArtifactsPrefix:  strings.TrimSpace(artifactsPrefix.String),
		})
	}
	if err := rows.Err(); err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	api.writeJSON(w, http.StatusOK, map[string]any{"runs": out})
}

func (api *experimentsAPI) handleListAllExperimentRuns(w http.ResponseWriter, r *http.Request) {
	projectID, ok := auth.ProjectIDFromContext(r.Context())
	if !ok || strings.TrimSpace(projectID) == "" {
		api.writeError(w, r, http.StatusBadRequest, "project_id_required")
		return
	}
	limit := clampInt(parseIntQuery(r, "limit", 100), 1, 500)

	statusFilter := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("status")))
	if statusFilter != "" {
		if _, ok := allowedRunStatuses[statusFilter]; !ok {
			api.writeError(w, r, http.StatusBadRequest, "invalid_status")
			return
		}
	}

	active := false
	activeRaw := strings.TrimSpace(r.URL.Query().Get("active"))
	if activeRaw != "" {
		switch strings.ToLower(activeRaw) {
		case "1", "true", "yes", "on":
			active = true
		}
	}

	rows, err := api.db.QueryContext(
		r.Context(),
		`SELECT r.run_id,
				r.experiment_id,
				r.dataset_version_id,
				COALESCE(s.status, r.status) AS status,
				r.started_at,
				COALESCE(r.ended_at, CASE WHEN s.status IN ('succeeded','failed','canceled') THEN s.observed_at END) AS ended_at,
				r.git_repo,
				r.git_commit,
				r.git_ref,
				r.params,
				r.metrics,
				r.artifacts_prefix
		 FROM experiment_runs r
		 LEFT JOIN LATERAL (
			SELECT status, observed_at
			FROM experiment_run_state_events
			WHERE run_id = r.run_id
			ORDER BY observed_at DESC
			LIMIT 1
		 ) s ON true
		 WHERE r.project_id = $1
		   AND ($2::bool IS false OR COALESCE(s.status, r.status) IN ('pending','running'))
		   AND ($3 = '' OR COALESCE(s.status, r.status) = $3)
		 ORDER BY r.started_at DESC
		 LIMIT $4`,
		projectID,
		active,
		statusFilter,
		limit,
	)
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	defer rows.Close()

	out := make([]experimentRun, 0, limit)
	for rows.Next() {
		var (
			runID            string
			experimentID     string
			datasetVersionID sql.NullString
			status           string
			startedAt        time.Time
			endedAt          sql.NullTime
			gitRepo          sql.NullString
			gitCommit        sql.NullString
			gitRef           sql.NullString
			params           []byte
			metrics          []byte
			artifactsPrefix  sql.NullString
		)
		if err := rows.Scan(&runID, &experimentID, &datasetVersionID, &status, &startedAt, &endedAt, &gitRepo, &gitCommit, &gitRef, &params, &metrics, &artifactsPrefix); err != nil {
			api.writeError(w, r, http.StatusInternalServerError, "internal_error")
			return
		}

		var endedAtPtr *time.Time
		if endedAt.Valid && !endedAt.Time.IsZero() {
			t := endedAt.Time.UTC()
			endedAtPtr = &t
		}

		out = append(out, experimentRun{
			RunID:            runID,
			ExperimentID:     experimentID,
			DatasetVersionID: strings.TrimSpace(datasetVersionID.String),
			Status:           status,
			StartedAt:        startedAt,
			EndedAt:          endedAtPtr,
			GitRepo:          strings.TrimSpace(gitRepo.String),
			GitCommit:        strings.TrimSpace(gitCommit.String),
			GitRef:           strings.TrimSpace(gitRef.String),
			Params:           normalizeJSON(params),
			Metrics:          normalizeJSON(metrics),
			ArtifactsPrefix:  strings.TrimSpace(artifactsPrefix.String),
		})
	}
	if err := rows.Err(); err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	api.writeJSON(w, http.StatusOK, map[string]any{"runs": out})
}

func (api *experimentsAPI) handleGetExperimentRun(w http.ResponseWriter, r *http.Request) {
	runID := strings.TrimSpace(r.PathValue("run_id"))
	if runID == "" {
		api.writeError(w, r, http.StatusBadRequest, "run_id_required")
		return
	}

	var (
		experimentID     string
		datasetVersionID sql.NullString
		status           string
		startedAt        time.Time
		endedAt          sql.NullTime
		gitRepo          sql.NullString
		gitCommit        sql.NullString
		gitRef           sql.NullString
		params           []byte
		metrics          []byte
		artifactsPrefix  sql.NullString
	)
	err := api.db.QueryRowContext(
		r.Context(),
		`SELECT r.experiment_id,
				r.dataset_version_id,
				COALESCE(s.status, r.status) AS status,
				r.started_at,
				COALESCE(r.ended_at, CASE WHEN s.status IN ('succeeded','failed','canceled') THEN s.observed_at END) AS ended_at,
				r.git_repo,
				r.git_commit,
				r.git_ref,
				r.params,
				r.metrics,
				r.artifacts_prefix
		 FROM experiment_runs r
		 LEFT JOIN LATERAL (
			SELECT status, observed_at
			FROM experiment_run_state_events
			WHERE run_id = r.run_id
			ORDER BY observed_at DESC
			LIMIT 1
		 ) s ON true
		 WHERE r.run_id = $1`,
		runID,
	).Scan(&experimentID, &datasetVersionID, &status, &startedAt, &endedAt, &gitRepo, &gitCommit, &gitRef, &params, &metrics, &artifactsPrefix)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			api.writeError(w, r, http.StatusNotFound, "not_found")
			return
		}
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	var endedAtPtr *time.Time
	if endedAt.Valid && !endedAt.Time.IsZero() {
		t := endedAt.Time.UTC()
		endedAtPtr = &t
	}

	api.writeJSON(w, http.StatusOK, experimentRun{
		RunID:            runID,
		ExperimentID:     experimentID,
		DatasetVersionID: strings.TrimSpace(datasetVersionID.String),
		Status:           status,
		StartedAt:        startedAt,
		EndedAt:          endedAtPtr,
		GitRepo:          strings.TrimSpace(gitRepo.String),
		GitCommit:        strings.TrimSpace(gitCommit.String),
		GitRef:           strings.TrimSpace(gitRef.String),
		Params:           normalizeJSON(params),
		Metrics:          normalizeJSON(metrics),
		ArtifactsPrefix:  strings.TrimSpace(artifactsPrefix.String),
	})
}

type gateDecision struct {
	DatasetID     string
	ContentSHA256 string
	RuleID        string
	EvaluationID  string
	Status        string
}

func (api *experimentsAPI) requireQualityGatePass(w http.ResponseWriter, r *http.Request, identity auth.Identity, datasetVersionID string, experimentID string) (gateDecision, bool) {
	ctx := r.Context()

	var (
		datasetID     string
		qualityRuleID sql.NullString
		contentSHA256 string
	)
	err := api.db.QueryRowContext(
		ctx,
		`SELECT dataset_id, quality_rule_id, content_sha256
		 FROM dataset_versions
		 WHERE version_id = $1`,
		datasetVersionID,
	).Scan(&datasetID, &qualityRuleID, &contentSHA256)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			api.writeError(w, r, http.StatusNotFound, "not_found")
			return gateDecision{}, false
		}
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return gateDecision{}, false
	}

	ruleID := strings.TrimSpace(qualityRuleID.String)
	if ruleID == "" {
		now := time.Now().UTC()
		_, _ = auditlog.Insert(ctx, api.db, auditlog.Event{
			OccurredAt:   now,
			Actor:        identity.Subject,
			Action:       "quality_gate.block",
			ResourceType: "dataset_version",
			ResourceID:   datasetVersionID,
			RequestID:    r.Header.Get("X-Request-Id"),
			IP:           requestIP(r.RemoteAddr),
			UserAgent:    r.UserAgent(),
			Payload: map[string]any{
				"service":            "experiments",
				"dataset_id":         datasetID,
				"dataset_version_id": datasetVersionID,
				"experiment_id":      experimentID,
				"reason":             "no_rule",
			},
		})
		api.writeError(w, r, http.StatusConflict, "quality_rule_not_set")
		return gateDecision{}, false
	}

	var (
		evalID     string
		evalStatus string
	)
	err = api.db.QueryRowContext(
		ctx,
		`SELECT evaluation_id, status
		 FROM quality_evaluations
		 WHERE dataset_version_id = $1 AND rule_id = $2
		 ORDER BY evaluated_at DESC
		 LIMIT 1`,
		datasetVersionID,
		ruleID,
	).Scan(&evalID, &evalStatus)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			now := time.Now().UTC()
			_, _ = auditlog.Insert(ctx, api.db, auditlog.Event{
				OccurredAt:   now,
				Actor:        identity.Subject,
				Action:       "quality_gate.block",
				ResourceType: "dataset_version",
				ResourceID:   datasetVersionID,
				RequestID:    r.Header.Get("X-Request-Id"),
				IP:           requestIP(r.RemoteAddr),
				UserAgent:    r.UserAgent(),
				Payload: map[string]any{
					"service":            "experiments",
					"dataset_id":         datasetID,
					"dataset_version_id": datasetVersionID,
					"rule_id":            ruleID,
					"experiment_id":      experimentID,
					"reason":             "not_evaluated",
				},
			})
			api.writeError(w, r, http.StatusConflict, "quality_not_evaluated")
			return gateDecision{}, false
		}
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return gateDecision{}, false
	}

	if strings.ToLower(strings.TrimSpace(evalStatus)) != "pass" {
		now := time.Now().UTC()
		_, _ = auditlog.Insert(ctx, api.db, auditlog.Event{
			OccurredAt:   now,
			Actor:        identity.Subject,
			Action:       "quality_gate.block",
			ResourceType: "dataset_version",
			ResourceID:   datasetVersionID,
			RequestID:    r.Header.Get("X-Request-Id"),
			IP:           requestIP(r.RemoteAddr),
			UserAgent:    r.UserAgent(),
			Payload: map[string]any{
				"service":            "experiments",
				"dataset_id":         datasetID,
				"dataset_version_id": datasetVersionID,
				"rule_id":            ruleID,
				"evaluation_id":      evalID,
				"status":             evalStatus,
				"experiment_id":      experimentID,
				"reason":             "not_pass",
			},
		})
		api.writeError(w, r, http.StatusConflict, "quality_gate_failed")
		return gateDecision{}, false
	}

	return gateDecision{
		DatasetID:     datasetID,
		ContentSHA256: strings.TrimSpace(contentSHA256),
		RuleID:        ruleID,
		EvaluationID:  evalID,
		Status:        evalStatus,
	}, true
}

func (api *experimentsAPI) experimentExists(ctx context.Context, experimentID string) (bool, error) {
	var one int
	err := api.db.QueryRowContext(
		ctx,
		`SELECT 1 FROM experiments WHERE experiment_id = $1`,
		experimentID,
	).Scan(&one)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func formatTimePtr(t *time.Time) string {
	if t == nil || t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339Nano)
}

func nullTimePtr(value *time.Time) sql.NullTime {
	if value == nil || value.IsZero() {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: value.UTC(), Valid: true}
}

func decodeJSON(r *http.Request, dst any) error {
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return err
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return errors.New("multiple JSON values")
	}
	return nil
}

func (api *experimentsAPI) writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(true)
	_ = enc.Encode(body)
}

func (api *experimentsAPI) writeError(w http.ResponseWriter, r *http.Request, status int, code string) {
	api.writeJSON(w, status, map[string]any{
		"error":      code,
		"request_id": r.Header.Get("X-Request-Id"),
	})
}

func (api *experimentsAPI) writeRepoError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, repo.ErrInvalidTransition):
		api.writeError(w, r, http.StatusConflict, "invalid_transition")
		return
	case errors.Is(err, repo.ErrNotFound), errors.Is(err, sql.ErrNoRows):
		api.writeError(w, r, http.StatusNotFound, "not_found")
		return
	case isForeignKeyViolation(err):
		api.writeError(w, r, http.StatusNotFound, "not_found")
		return
	case isUniqueViolation(err):
		api.writeError(w, r, http.StatusConflict, "conflict")
		return
	default:
		if api.logger != nil {
			api.logger.Error("request failed", "err", err, "path", r.URL.Path, "method", r.Method)
		}
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
	}
}

func requestIP(remoteAddr string) net.IP {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return nil
	}
	return net.ParseIP(host)
}

func normalizeJSON(raw []byte) json.RawMessage {
	raw = bytesTrimSpace(raw)
	if len(raw) == 0 || string(raw) == "null" {
		return []byte("{}")
	}
	return redaction.RedactJSON(raw)
}

func bytesTrimSpace(in []byte) []byte {
	return []byte(strings.TrimSpace(string(in)))
}

func nullString(value string) sql.NullString {
	value = strings.TrimSpace(value)
	if value == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: value, Valid: true}
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}

func isForeignKeyViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23503"
	}
	return false
}

func parseIntQuery(r *http.Request, key string, def int) int {
	v := strings.TrimSpace(r.URL.Query().Get(key))
	if v == "" {
		return def
	}
	parsed, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return parsed
}

func clampInt(v int, min int, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
