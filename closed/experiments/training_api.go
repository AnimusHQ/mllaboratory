package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/animus-labs/animus-go/closed/internal/platform/auditlog"
	"github.com/animus-labs/animus-go/closed/internal/platform/auth"
	"github.com/animus-labs/animus-go/closed/internal/platform/lineageevent"
	"github.com/animus-labs/animus-go/closed/internal/platform/redaction"
	"github.com/animus-labs/animus-go/closed/internal/platform/policy"
	"github.com/animus-labs/animus-go/closed/internal/runtimeexec"
	"github.com/google/uuid"
)

type trainingExecutor = runtimeexec.Executor
type trainingJobSpec = runtimeexec.JobSpec
type trainingExecution = runtimeexec.Execution
type trainingObservation = runtimeexec.Observation

type executeExperimentRunRequest struct {
	ExperimentID     string         `json:"experiment_id"`
	DatasetVersionID string         `json:"dataset_version_id"`
	ImageRef         string         `json:"image_ref"`
	GitRepo          string         `json:"git_repo,omitempty"`
	GitCommit        string         `json:"git_commit,omitempty"`
	GitRef           string         `json:"git_ref,omitempty"`
	Params           map[string]any `json:"params,omitempty"`
	Resources        map[string]any `json:"resources,omitempty"`
}

func (api *experimentsAPI) handleExecuteExperimentRun(w http.ResponseWriter, r *http.Request) {
	identity, ok := auth.IdentityFromContext(r.Context())
	if !ok || strings.TrimSpace(identity.Subject) == "" {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	if api.trainingExecutor == nil {
		api.writeError(w, r, http.StatusNotImplemented, "training_executor_disabled")
		return
	}

	var req executeExperimentRunRequest
	if err := decodeJSON(r, &req); err != nil {
		api.writeError(w, r, http.StatusBadRequest, "invalid_json")
		return
	}

	experimentID := strings.TrimSpace(req.ExperimentID)
	if experimentID == "" {
		api.writeError(w, r, http.StatusBadRequest, "experiment_id_required")
		return
	}
	datasetVersionID := strings.TrimSpace(req.DatasetVersionID)
	if datasetVersionID == "" {
		api.writeError(w, r, http.StatusBadRequest, "dataset_version_id_required")
		return
	}
	imageRef := strings.TrimSpace(req.ImageRef)
	if imageRef == "" {
		api.writeError(w, r, http.StatusBadRequest, "image_ref_required")
		return
	}

	kind := strings.TrimSpace(api.trainingExecutor.Kind())
	if kind == "" {
		api.writeError(w, r, http.StatusInternalServerError, "invalid_training_executor")
		return
	}

	imageExecutionRef, imageDigest, err := resolveImageForExecutor(r.Context(), kind, api.trainingExecutor, imageRef)
	if err != nil {
		switch {
		case errors.Is(err, runtimeexec.ErrImageRefDigestRequired):
			api.writeError(w, r, http.StatusBadRequest, "image_ref_digest_required")
		case errors.Is(err, runtimeexec.ErrImageRefNotFound):
			api.writeError(w, r, http.StatusBadRequest, "image_ref_not_found")
		default:
			api.writeError(w, r, http.StatusBadGateway, "image_ref_resolution_failed")
		}
		return
	}

	gitRepo := strings.TrimSpace(req.GitRepo)
	gitCommit := strings.TrimSpace(req.GitCommit)
	gitRef := strings.TrimSpace(req.GitRef)
	if api.runTokenTTL <= 0 || strings.TrimSpace(api.runTokenSecret) == "" {
		api.writeError(w, r, http.StatusInternalServerError, "training_token_not_configured")
		return
	}
	if strings.TrimSpace(api.datapilotURL) == "" {
		api.writeError(w, r, http.StatusInternalServerError, "datapilot_url_not_configured")
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

	gate, ok := api.requireQualityGatePass(w, r, identity, datasetVersionID, experimentID)
	if !ok {
		return
	}

	var (
		modelRepo   string
		modelCommit string
	)
	err = api.db.QueryRowContext(
		r.Context(),
		`SELECT repo, commit_sha
		 FROM model_images
		 WHERE image_digest = $1`,
		imageDigest,
	).Scan(&modelRepo, &modelCommit)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			api.writeError(w, r, http.StatusInternalServerError, "internal_error")
			return
		}
	} else {
		if gitRepo != "" && gitRepo != modelRepo {
			api.writeError(w, r, http.StatusConflict, "git_repo_conflict")
			return
		}
		if gitCommit != "" && gitCommit != modelCommit {
			api.writeError(w, r, http.StatusConflict, "git_commit_conflict")
			return
		}
		if gitRepo == "" {
			gitRepo = modelRepo
		}
		if gitCommit == "" {
			gitCommit = modelCommit
		}
	}
	if gitCommit == "" {
		api.writeError(w, r, http.StatusBadRequest, "git_commit_required")
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

	resources := req.Resources
	if resources == nil {
		resources = map[string]any{}
	}
	resourcesJSON, err := json.Marshal(resources)
	if err != nil {
		api.writeError(w, r, http.StatusBadRequest, "invalid_resources")
		return
	}

	now := time.Now().UTC()
	runID := uuid.NewString()

	policyResult, err := api.evaluateExecutionPolicy(r.Context(), executionPolicyInput{
		RunID:             runID,
		Actor:             policy.ActorContext{Subject: identity.Subject, Email: identity.Email, Roles: identity.Roles},
		ExperimentID:      experimentID,
		DatasetID:         gate.DatasetID,
		DatasetVersionID:  datasetVersionID,
		DatasetSHA256:     gate.ContentSHA256,
		GitRepo:           gitRepo,
		GitCommit:         gitCommit,
		GitRef:            gitRef,
		ImageRef:          imageRef,
		ImageDigest:       imageDigest,
		ImageExecutionRef: imageExecutionRef,
		Resources:         resources,
	})
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "policy_evaluation_failed")
		return
	}
	policyDecision := aggregatePolicyDecision(policyResult.Evaluations)
	if policyDecision == policy.EffectDeny {
		tx, err := api.db.BeginTx(r.Context(), nil)
		if err != nil {
			api.writeError(w, r, http.StatusInternalServerError, "internal_error")
			return
		}
		defer func() { _ = tx.Rollback() }()

		inserted, err := api.insertPolicyDecisions(r.Context(), tx, "", identity, policyResult.ContextJSON, policyResult.ContextSHA256, policyResult.Evaluations)
		if err != nil {
			api.writeError(w, r, http.StatusInternalServerError, "internal_error")
			return
		}
		for _, decision := range inserted {
			_, err = auditlog.Insert(r.Context(), tx, auditlog.Event{
				OccurredAt:   now,
				Actor:        identity.Subject,
				Action:       "policy.decision",
				ResourceType: "policy_decision",
				ResourceID:   decision.DecisionID,
				RequestID:    r.Header.Get("X-Request-Id"),
				IP:           requestIP(r.RemoteAddr),
				UserAgent:    r.UserAgent(),
				Payload: map[string]any{
					"service":           "experiments",
					"policy_id":         decision.PolicyID,
					"policy_version_id": decision.PolicyVersionID,
					"decision":          decision.Effect,
					"rule_id":           decision.RuleID,
					"context_sha256":    policyResult.ContextSHA256,
				},
			})
			if err != nil {
				api.writeError(w, r, http.StatusInternalServerError, "audit_failed")
				return
			}
		}
		if err := tx.Commit(); err != nil {
			api.writeError(w, r, http.StatusInternalServerError, "internal_error")
			return
		}
		api.writeError(w, r, http.StatusForbidden, "policy_denied")
		return
	}

	k8sNamespace := ""
	k8sJobName := ""
	dockerName := ""
	switch kind {
	case "kubernetes_job":
		k8sNamespace = api.trainingNamespace
		if k8sNamespace == "" {
			api.writeError(w, r, http.StatusInternalServerError, "training_namespace_not_configured")
			return
		}
		k8sJobName = "animus-run-" + runID
	case "docker":
		dockerName = "animus-run-" + runID
	default:
		api.writeError(w, r, http.StatusInternalServerError, "invalid_training_executor")
		return
	}

	artifactsPrefix := strings.TrimSpace(req.ExperimentID)
	if artifactsPrefix == "" {
		artifactsPrefix = experimentID
	}
	artifactsPrefix = "experiments/" + artifactsPrefix + "/runs/" + runID

	metricsJSON := []byte("{}")
	status := "pending"

	type runIntegrityInput struct {
		RunID            string          `json:"run_id"`
		ExperimentID     string          `json:"experiment_id"`
		DatasetVersionID string          `json:"dataset_version_id,omitempty"`
		Status           string          `json:"status"`
		StartedAt        time.Time       `json:"started_at"`
		GitRepo          string          `json:"git_repo,omitempty"`
		GitCommit        string          `json:"git_commit,omitempty"`
		GitRef           string          `json:"git_ref,omitempty"`
		Params           json.RawMessage `json:"params"`
		Metrics          json.RawMessage `json:"metrics"`
		ArtifactsPrefix  string          `json:"artifacts_prefix,omitempty"`
	}
	runIntegrity, err := integritySHA256(runIntegrityInput{
		RunID:            runID,
		ExperimentID:     experimentID,
		DatasetVersionID: datasetVersionID,
		Status:           status,
		StartedAt:        now,
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
			dataset_version_id,
			status,
			started_at,
			git_repo,
			git_commit,
			git_ref,
			params,
			metrics,
			artifacts_prefix,
			integrity_sha256
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
		runID,
		experimentID,
		nullString(datasetVersionID),
		status,
		now,
		nullString(gitRepo),
		nullString(gitCommit),
		nullString(gitRef),
		paramsJSON,
		metricsJSON,
		nullString(artifactsPrefix),
		runIntegrity,
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
		},
	})
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "lineage_write_failed")
		return
	}

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
			"started_at":         now.Format(time.RFC3339Nano),
			"git_repo":           gitRepo,
			"git_commit":         gitCommit,
			"git_ref":            gitRef,
			"params":             paramsMap,
			"artifacts_prefix":   artifactsPrefix,
		},
	})
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "audit_failed")
		return
	}

	insertedDecisions, err := api.insertPolicyDecisions(r.Context(), tx, runID, identity, policyResult.ContextJSON, policyResult.ContextSHA256, policyResult.Evaluations)
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	for _, decision := range insertedDecisions {
		_, err = auditlog.Insert(r.Context(), tx, auditlog.Event{
			OccurredAt:   now,
			Actor:        identity.Subject,
			Action:       "policy.decision",
			ResourceType: "policy_decision",
			ResourceID:   decision.DecisionID,
			RequestID:    r.Header.Get("X-Request-Id"),
			IP:           requestIP(r.RemoteAddr),
			UserAgent:    r.UserAgent(),
			Payload: map[string]any{
				"service":           "experiments",
				"run_id":            runID,
				"policy_id":         decision.PolicyID,
				"policy_version_id": decision.PolicyVersionID,
				"decision":          decision.Effect,
				"rule_id":           decision.RuleID,
				"context_sha256":    policyResult.ContextSHA256,
			},
		})
		if err != nil {
			api.writeError(w, r, http.StatusInternalServerError, "audit_failed")
			return
		}
	}

	pendingDetails := map[string]any{
		"executor":           kind,
		"image_ref":          imageRef,
		"image_digest":       imageDigest,
		"k8s_namespace":      k8sNamespace,
		"k8s_job_name":       k8sJobName,
		"docker_container":   dockerName,
		"dataset_version_id": datasetVersionID,
	}
	pendingMessage := "run pending"

	if policyDecision == policy.EffectRequireApproval {
		approvalIDs, err := api.insertPolicyApprovals(r.Context(), tx, runID, identity, insertedDecisions)
		if err != nil {
			api.writeError(w, r, http.StatusInternalServerError, "internal_error")
			return
		}
		if len(approvalIDs) > 0 {
			pendingDetails["approval_required"] = true
			pendingDetails["approval_ids"] = approvalIDs
			pendingMessage = "run pending approval"
		}

		_, err = api.insertRunStateEvent(r.Context(), tx, runID, "pending", now, pendingDetails)
		if err != nil {
			api.writeError(w, r, http.StatusInternalServerError, "internal_error")
			return
		}
		if err := api.insertRunEvent(r.Context(), tx, runID, identity.Subject, "info", pendingMessage, pendingDetails); err != nil {
			api.writeError(w, r, http.StatusInternalServerError, "internal_error")
			return
		}

		for _, approvalID := range approvalIDs {
			_, err = auditlog.Insert(r.Context(), tx, auditlog.Event{
				OccurredAt:   now,
				Actor:        identity.Subject,
				Action:       "policy.approval.requested",
				ResourceType: "policy_approval",
				ResourceID:   approvalID,
				RequestID:    r.Header.Get("X-Request-Id"),
				IP:           requestIP(r.RemoteAddr),
				UserAgent:    r.UserAgent(),
				Payload: map[string]any{
					"service":       "experiments",
					"run_id":        runID,
					"experiment_id": experimentID,
					"approval_id":   approvalID,
				},
			})
			if err != nil {
				api.writeError(w, r, http.StatusInternalServerError, "audit_failed")
				return
			}
		}

		if err := tx.Commit(); err != nil {
			api.writeError(w, r, http.StatusInternalServerError, "internal_error")
			return
		}

		w.Header().Set("Location", "/experiment-runs/"+runID)
		api.writeJSON(w, http.StatusAccepted, map[string]any{
			"run_id":            runID,
			"status":            status,
			"approval_required": true,
			"approval_ids":      approvalIDs,
		})
		return
	}

	runToken, err := auth.GenerateRunToken(api.runTokenSecret, auth.RunTokenClaims{
		RunID:            runID,
		DatasetVersionID: datasetVersionID,
		ExpiresAtUnix:    now.Add(api.runTokenTTL).Unix(),
	}, now)
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	executionID := uuid.NewString()
	type executionIntegrityInput struct {
		ExecutionID      string          `json:"execution_id"`
		RunID            string          `json:"run_id"`
		Executor         string          `json:"executor"`
		ImageRef         string          `json:"image_ref"`
		ImageDigest      string          `json:"image_digest"`
		Resources        json.RawMessage `json:"resources"`
		K8sNamespace     string          `json:"k8s_namespace,omitempty"`
		K8sJobName       string          `json:"k8s_job_name,omitempty"`
		DockerContainer  string          `json:"docker_container_id,omitempty"`
		DatapilotURL     string          `json:"datapilot_url"`
		CreatedAt        time.Time       `json:"created_at"`
		CreatedBy        string          `json:"created_by"`
		RunTokenSHA256   string          `json:"run_token_sha256"`
		DatasetVersionID string          `json:"dataset_version_id"`
	}
	runTokenSum := sha256Hex(runToken)
	executionIntegrity, err := integritySHA256(executionIntegrityInput{
		ExecutionID:      executionID,
		RunID:            runID,
		Executor:         kind,
		ImageRef:         imageRef,
		ImageDigest:      imageDigest,
		Resources:        resourcesJSON,
		K8sNamespace:     k8sNamespace,
		K8sJobName:       k8sJobName,
		DockerContainer:  dockerName,
		DatapilotURL:     api.datapilotURL,
		CreatedAt:        now,
		CreatedBy:        identity.Subject,
		RunTokenSHA256:   runTokenSum,
		DatasetVersionID: datasetVersionID,
	})
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	_, err = tx.ExecContext(
		r.Context(),
		`INSERT INTO experiment_run_executions (
			execution_id,
			run_id,
			executor,
			image_ref,
			image_digest,
			resources,
			k8s_namespace,
			k8s_job_name,
			docker_container_id,
			datapilot_url,
			created_at,
			created_by,
			integrity_sha256
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
		ON CONFLICT (run_id) DO NOTHING`,
		executionID,
		runID,
		kind,
		imageRef,
		imageDigest,
		resourcesJSON,
		nullString(k8sNamespace),
		nullString(k8sJobName),
		nullString(dockerName),
		api.datapilotURL,
		now,
		identity.Subject,
		executionIntegrity,
	)
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	if _, _, err := api.insertExecutionLedgerEntry(r.Context(), tx, runID, executionID); err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "execution_ledger_failed")
		return
	}

	_, err = api.insertRunStateEvent(r.Context(), tx, runID, "pending", now, pendingDetails)
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	if err := api.insertRunEvent(r.Context(), tx, runID, identity.Subject, "info", pendingMessage, pendingDetails); err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	_, err = auditlog.Insert(r.Context(), tx, auditlog.Event{
		OccurredAt:   now,
		Actor:        identity.Subject,
		Action:       "experiment_run.execute",
		ResourceType: "experiment_run_execution",
		ResourceID:   executionID,
		RequestID:    r.Header.Get("X-Request-Id"),
		IP:           requestIP(r.RemoteAddr),
		UserAgent:    r.UserAgent(),
		Payload: map[string]any{
			"service":            "experiments",
			"execution_id":       executionID,
			"run_id":             runID,
			"experiment_id":      experimentID,
			"dataset_version_id": datasetVersionID,
			"executor":           kind,
			"image_ref":          imageRef,
			"image_digest":       imageDigest,
			"k8s_namespace":      k8sNamespace,
			"k8s_job_name":       k8sJobName,
			"docker_container":   dockerName,
			"resources":          resources,
			"datapilot_url":      api.datapilotURL,
			"run_token_sha256":   runTokenSum,
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

	spec := trainingJobSpec{
		RunID:            runID,
		DatasetVersionID: datasetVersionID,
		ImageRef:         imageExecutionRef,
		DatapilotURL:     api.datapilotURL,
		Token:            runToken,
		Resources:        resources,
		K8sNamespace:     k8sNamespace,
		K8sJobName:       k8sJobName,
		DockerName:       dockerName,
		JobKind:          "training",
	}
	if err := api.trainingExecutor.Submit(r.Context(), spec); err != nil {
		_ = api.failRunExecution(r.Context(), identity, r, runID, executionID, kind, imageRef, err)
		api.writeError(w, r, http.StatusBadGateway, "training_submit_failed")
		return
	}

	currentStatus := "running"
	if err := api.markRunRunning(r.Context(), identity, r, runID, executionID, kind, imageRef); err != nil {
		currentStatus = "pending"
		if api.logger != nil {
			api.logger.Error("mark run running failed", "run_id", runID, "error", err)
		}
	}

	w.Header().Set("Location", "/experiment-runs/"+runID)
	api.writeJSON(w, http.StatusCreated, map[string]any{
		"run_id":        runID,
		"execution_id":  executionID,
		"status":        currentStatus,
		"datapilot_url": api.datapilotURL,
	})
}

func (api *experimentsAPI) failRunExecution(ctx context.Context, identity auth.Identity, r *http.Request, runID string, executionID string, executor string, imageRef string, submitErr error) error {
	now := time.Now().UTC()

	tx, err := api.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	inserted, err := api.insertRunStateEvent(ctx, tx, runID, "failed", now, map[string]any{
		"reason":       "submit_failed",
		"executor":     executor,
		"image_ref":    imageRef,
		"execution_id": executionID,
		"error":        submitErr.Error(),
	})
	if err != nil {
		return err
	}
	if !inserted {
		return nil
	}
	if err := api.insertRunEvent(ctx, tx, runID, identity.Subject, "error", "run failed (submit_failed)", map[string]any{
		"executor":     executor,
		"image_ref":    imageRef,
		"execution_id": executionID,
	}); err != nil {
		return err
	}

	_, err = auditlog.Insert(ctx, tx, auditlog.Event{
		OccurredAt:   now,
		Actor:        identity.Subject,
		Action:       "experiment_run.execute_failed",
		ResourceType: "experiment_run_execution",
		ResourceID:   executionID,
		RequestID:    r.Header.Get("X-Request-Id"),
		IP:           requestIP(r.RemoteAddr),
		UserAgent:    r.UserAgent(),
		Payload: map[string]any{
			"service":      "experiments",
			"execution_id": executionID,
			"run_id":       runID,
			"executor":     executor,
			"image_ref":    imageRef,
			"error":        submitErr.Error(),
		},
	})
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (api *experimentsAPI) markRunRunning(ctx context.Context, identity auth.Identity, r *http.Request, runID string, executionID string, executor string, imageRef string) error {
	now := time.Now().UTC()

	tx, err := api.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	inserted, err := api.insertRunStateEvent(ctx, tx, runID, "running", now, map[string]any{
		"executor":     executor,
		"image_ref":    imageRef,
		"execution_id": executionID,
	})
	if err != nil {
		return err
	}
	if !inserted {
		return nil
	}
	if err := api.insertRunEvent(ctx, tx, runID, identity.Subject, "info", "run running", map[string]any{
		"executor":     executor,
		"image_ref":    imageRef,
		"execution_id": executionID,
	}); err != nil {
		return err
	}

	_, err = auditlog.Insert(ctx, tx, auditlog.Event{
		OccurredAt:   now,
		Actor:        identity.Subject,
		Action:       "experiment_run.running",
		ResourceType: "experiment_run",
		ResourceID:   runID,
		RequestID:    r.Header.Get("X-Request-Id"),
		IP:           requestIP(r.RemoteAddr),
		UserAgent:    r.UserAgent(),
		Payload: map[string]any{
			"service":      "experiments",
			"execution_id": executionID,
			"executor":     executor,
			"image_ref":    imageRef,
		},
	})
	if err != nil {
		return err
	}

	return tx.Commit()
}

type ingestExperimentRunMetricsRequest struct {
	Step    int64          `json:"step"`
	Metrics map[string]any `json:"metrics"`
	Meta    map[string]any `json:"metadata,omitempty"`
}

func (api *experimentsAPI) handleIngestExperimentRunMetrics(w http.ResponseWriter, r *http.Request) {
	identity, ok := auth.IdentityFromContext(r.Context())
	if !ok || strings.TrimSpace(identity.Subject) == "" {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	runID := strings.TrimSpace(r.PathValue("run_id"))
	if runID == "" {
		api.writeError(w, r, http.StatusBadRequest, "run_id_required")
		return
	}

	var req ingestExperimentRunMetricsRequest
	if err := decodeJSON(r, &req); err != nil {
		api.writeError(w, r, http.StatusBadRequest, "invalid_json")
		return
	}
	if req.Step < 0 {
		api.writeError(w, r, http.StatusBadRequest, "invalid_step")
		return
	}
	if len(req.Metrics) == 0 {
		api.writeError(w, r, http.StatusBadRequest, "metrics_required")
		return
	}

	metrics := make(map[string]float64, len(req.Metrics))
	for k, v := range req.Metrics {
		name := strings.TrimSpace(k)
		if name == "" {
			api.writeError(w, r, http.StatusBadRequest, "invalid_metric_name")
			return
		}
		switch n := v.(type) {
		case float64:
			metrics[name] = n
		case int:
			metrics[name] = float64(n)
		case int64:
			metrics[name] = float64(n)
		default:
			api.writeError(w, r, http.StatusBadRequest, "invalid_metric_value")
			return
		}
	}

	metadataMap := req.Meta
	if metadataMap == nil {
		metadataMap = map[string]any{}
	}
	metadataJSON, err := json.Marshal(metadataMap)
	if err != nil {
		api.writeError(w, r, http.StatusBadRequest, "invalid_metadata")
		return
	}

	tx, err := api.db.BeginTx(r.Context(), nil)
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	defer func() { _ = tx.Rollback() }()

	var one int
	if err := tx.QueryRowContext(r.Context(), `SELECT 1 FROM experiment_runs WHERE run_id = $1`, runID).Scan(&one); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			api.writeError(w, r, http.StatusNotFound, "not_found")
			return
		}
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	now := time.Now().UTC()
	inserted := 0
	names := make([]string, 0, len(metrics))
	for name := range metrics {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		value := metrics[name]
		sampleID := uuid.NewString()

		type integrityInput struct {
			SampleID    string          `json:"sample_id"`
			RunID       string          `json:"run_id"`
			RecordedAt  time.Time       `json:"recorded_at"`
			RecordedBy  string          `json:"recorded_by"`
			Step        int64           `json:"step"`
			Name        string          `json:"name"`
			Value       float64         `json:"value"`
			Metadata    json.RawMessage `json:"metadata"`
			RequestID   string          `json:"request_id,omitempty"`
			UserAgent   string          `json:"user_agent,omitempty"`
			RemoteAddr  string          `json:"remote_addr,omitempty"`
			DatapilotID string          `json:"datapilot_id,omitempty"`
		}
		integrity, err := integritySHA256(integrityInput{
			SampleID:   sampleID,
			RunID:      runID,
			RecordedAt: now,
			RecordedBy: identity.Subject,
			Step:       req.Step,
			Name:       name,
			Value:      value,
			Metadata:   metadataJSON,
			RequestID:  r.Header.Get("X-Request-Id"),
			UserAgent:  r.UserAgent(),
			RemoteAddr: r.RemoteAddr,
		})
		if err != nil {
			api.writeError(w, r, http.StatusInternalServerError, "internal_error")
			return
		}

		res, err := tx.ExecContext(
			r.Context(),
			`INSERT INTO experiment_run_metric_samples (
				sample_id,
				run_id,
				recorded_at,
				recorded_by,
				step,
				name,
				value,
				metadata,
				integrity_sha256
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
			ON CONFLICT (run_id, name, step) DO NOTHING`,
			sampleID,
			runID,
			now,
			identity.Subject,
			req.Step,
			name,
			value,
			metadataJSON,
			integrity,
		)
		if err != nil {
			api.writeError(w, r, http.StatusInternalServerError, "internal_error")
			return
		}
		affected, _ := res.RowsAffected()
		if affected > 0 {
			inserted++
		}
	}

	if err := tx.Commit(); err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	status := http.StatusCreated
	if inserted == 0 {
		status = http.StatusOK
	}
	api.writeJSON(w, status, map[string]any{
		"run_id":     runID,
		"step":       req.Step,
		"inserted":   inserted,
		"received":   len(metrics),
		"request_id": r.Header.Get("X-Request-Id"),
	})
}

type experimentRunMetricSample struct {
	SampleID   string          `json:"sample_id"`
	RunID      string          `json:"run_id"`
	RecordedAt time.Time       `json:"recorded_at"`
	RecordedBy string          `json:"recorded_by"`
	Step       int64           `json:"step"`
	Name       string          `json:"name"`
	Value      float64         `json:"value"`
	Metadata   json.RawMessage `json:"metadata"`
}

func (api *experimentsAPI) handleListExperimentRunMetrics(w http.ResponseWriter, r *http.Request) {
	runID := strings.TrimSpace(r.PathValue("run_id"))
	if runID == "" {
		api.writeError(w, r, http.StatusBadRequest, "run_id_required")
		return
	}

	var one int
	if err := api.db.QueryRowContext(r.Context(), `SELECT 1 FROM experiment_runs WHERE run_id = $1`, runID).Scan(&one); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			api.writeError(w, r, http.StatusNotFound, "not_found")
			return
		}
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	limit := clampInt(parseIntQuery(r, "limit", 200), 1, 1000)
	nameFilter := strings.TrimSpace(r.URL.Query().Get("name"))

	var (
		rows *sql.Rows
		err  error
	)
	if nameFilter != "" {
		rows, err = api.db.QueryContext(
			r.Context(),
			`SELECT sample_id, recorded_at, recorded_by, step, name, value, metadata
			 FROM experiment_run_metric_samples
			 WHERE run_id = $1 AND name = $2
			 ORDER BY step DESC
			 LIMIT $3`,
			runID,
			nameFilter,
			limit,
		)
	} else {
		rows, err = api.db.QueryContext(
			r.Context(),
			`SELECT DISTINCT ON (name) sample_id, recorded_at, recorded_by, step, name, value, metadata
			 FROM experiment_run_metric_samples
			 WHERE run_id = $1
			 ORDER BY name, step DESC
			 LIMIT $2`,
			runID,
			limit,
		)
	}
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	defer rows.Close()

	out := make([]experimentRunMetricSample, 0, limit)
	for rows.Next() {
		var (
			sample   experimentRunMetricSample
			metadata []byte
		)
		if err := rows.Scan(&sample.SampleID, &sample.RecordedAt, &sample.RecordedBy, &sample.Step, &sample.Name, &sample.Value, &metadata); err != nil {
			api.writeError(w, r, http.StatusInternalServerError, "internal_error")
			return
		}
		sample.RunID = runID
		sample.Metadata = normalizeJSON(metadata)
		out = append(out, sample)
	}
	if err := rows.Err(); err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	if nameFilter != "" {
		for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
			out[i], out[j] = out[j], out[i]
		}
	}

	api.writeJSON(w, http.StatusOK, map[string]any{
		"run_id":  runID,
		"name":    nameFilter,
		"samples": out,
	})
}

type createExperimentRunEventRequest struct {
	OccurredAt *time.Time     `json:"occurred_at,omitempty"`
	Level      string         `json:"level,omitempty"`
	Message    string         `json:"message"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

type experimentRunEvent struct {
	EventID    int64           `json:"event_id"`
	RunID      string          `json:"run_id"`
	OccurredAt time.Time       `json:"occurred_at"`
	Actor      string          `json:"actor"`
	Level      string          `json:"level"`
	Message    string          `json:"message"`
	Metadata   json.RawMessage `json:"metadata"`
}

func (api *experimentsAPI) handleCreateExperimentRunEvent(w http.ResponseWriter, r *http.Request) {
	identity, ok := auth.IdentityFromContext(r.Context())
	if !ok || strings.TrimSpace(identity.Subject) == "" {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	runID := strings.TrimSpace(r.PathValue("run_id"))
	if runID == "" {
		api.writeError(w, r, http.StatusBadRequest, "run_id_required")
		return
	}

	var req createExperimentRunEventRequest
	if err := decodeJSON(r, &req); err != nil {
		api.writeError(w, r, http.StatusBadRequest, "invalid_json")
		return
	}

	message := redaction.RedactString(strings.TrimSpace(req.Message))
	if message == "" {
		api.writeError(w, r, http.StatusBadRequest, "message_required")
		return
	}
	level := strings.ToLower(strings.TrimSpace(req.Level))
	if level == "" {
		level = "info"
	}
	switch level {
	case "debug", "info", "warn", "error":
	default:
		api.writeError(w, r, http.StatusBadRequest, "invalid_level")
		return
	}

	occurredAt := time.Now().UTC()
	if req.OccurredAt != nil && !req.OccurredAt.IsZero() {
		occurredAt = req.OccurredAt.UTC()
	}

	metaMap := req.Metadata
	if metaMap == nil {
		metaMap = map[string]any{}
	}
	metaMap = redaction.RedactMetadata(metaMap)
	metaJSON, err := json.Marshal(metaMap)
	if err != nil {
		api.writeError(w, r, http.StatusBadRequest, "invalid_metadata")
		return
	}

	type integrityInput struct {
		RunID       string          `json:"run_id"`
		OccurredAt  time.Time       `json:"occurred_at"`
		Actor       string          `json:"actor"`
		Level       string          `json:"level"`
		Message     string          `json:"message"`
		Metadata    json.RawMessage `json:"metadata"`
		RequestID   string          `json:"request_id,omitempty"`
		UserAgent   string          `json:"user_agent,omitempty"`
		RemoteAddr  string          `json:"remote_addr,omitempty"`
		DatapilotID string          `json:"datapilot_id,omitempty"`
	}
	integrity, err := integritySHA256(integrityInput{
		RunID:      runID,
		OccurredAt: occurredAt,
		Actor:      identity.Subject,
		Level:      level,
		Message:    message,
		Metadata:   metaJSON,
		RequestID:  r.Header.Get("X-Request-Id"),
		UserAgent:  r.UserAgent(),
		RemoteAddr: r.RemoteAddr,
	})
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	var eventID int64
	err = api.db.QueryRowContext(
		r.Context(),
		`INSERT INTO experiment_run_events (
			run_id,
			occurred_at,
			actor,
			level,
			message,
			metadata,
			integrity_sha256
		) VALUES ($1,$2,$3,$4,$5,$6,$7)
		RETURNING event_id`,
		runID,
		occurredAt,
		identity.Subject,
		level,
		message,
		metaJSON,
		integrity,
	).Scan(&eventID)
	if err != nil {
		if isForeignKeyViolation(err) {
			api.writeError(w, r, http.StatusNotFound, "not_found")
			return
		}
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	w.Header().Set("Location", "/experiment-runs/"+runID+"/events/"+strconv.FormatInt(eventID, 10))
	api.writeJSON(w, http.StatusCreated, experimentRunEvent{
		EventID:    eventID,
		RunID:      runID,
		OccurredAt: occurredAt,
		Actor:      identity.Subject,
		Level:      level,
		Message:    message,
		Metadata:   metaJSON,
	})
}

func (api *experimentsAPI) handleListExperimentRunEvents(w http.ResponseWriter, r *http.Request) {
	runID := strings.TrimSpace(r.PathValue("run_id"))
	if runID == "" {
		api.writeError(w, r, http.StatusBadRequest, "run_id_required")
		return
	}

	var one int
	if err := api.db.QueryRowContext(r.Context(), `SELECT 1 FROM experiment_runs WHERE run_id = $1`, runID).Scan(&one); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			api.writeError(w, r, http.StatusNotFound, "not_found")
			return
		}
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	limit := clampInt(parseIntQuery(r, "limit", 200), 1, 1000)
	beforeRaw := strings.TrimSpace(r.URL.Query().Get("before_event_id"))
	var beforeID int64
	if beforeRaw != "" {
		parsed, err := strconv.ParseInt(beforeRaw, 10, 64)
		if err != nil || parsed < 0 {
			api.writeError(w, r, http.StatusBadRequest, "invalid_before_event_id")
			return
		}
		beforeID = parsed
	}

	args := []any{runID}
	query := `SELECT event_id, occurred_at, actor, level, message, metadata
		FROM experiment_run_events
		WHERE run_id = $1`
	if beforeID > 0 {
		args = append(args, beforeID)
		query += " AND event_id < $" + strconv.Itoa(len(args))
	}
	args = append(args, limit)
	query += " ORDER BY event_id DESC LIMIT $" + strconv.Itoa(len(args))

	rows, err := api.db.QueryContext(r.Context(), query, args...)
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	defer rows.Close()

	out := make([]experimentRunEvent, 0, limit)
	for rows.Next() {
		var (
			ev          experimentRunEvent
			metadataRaw []byte
		)
		if err := rows.Scan(&ev.EventID, &ev.OccurredAt, &ev.Actor, &ev.Level, &ev.Message, &metadataRaw); err != nil {
			api.writeError(w, r, http.StatusInternalServerError, "internal_error")
			return
		}
		ev.RunID = runID
		ev.Metadata = normalizeJSON(metadataRaw)
		out = append(out, ev)
	}
	if err := rows.Err(); err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	resp := map[string]any{
		"run_id": runID,
		"events": out,
	}
	if len(out) > 0 {
		resp["next_before_event_id"] = out[len(out)-1].EventID
	}
	api.writeJSON(w, http.StatusOK, resp)
}

func (api *experimentsAPI) insertRunStateEvent(ctx context.Context, tx *sql.Tx, runID string, status string, observedAt time.Time, details map[string]any) (bool, error) {
	if tx == nil {
		return false, errors.New("tx is required")
	}
	runID = strings.TrimSpace(runID)
	status = strings.ToLower(strings.TrimSpace(status))
	if runID == "" || status == "" {
		return false, errors.New("run_id and status are required")
	}
	if observedAt.IsZero() {
		observedAt = time.Now().UTC()
	}
	if details == nil {
		details = map[string]any{}
	}
	detailsJSON, err := json.Marshal(details)
	if err != nil {
		return false, err
	}

	stateID := uuid.NewString()
	type integrityInput struct {
		StateID     string          `json:"state_id"`
		RunID       string          `json:"run_id"`
		Status      string          `json:"status"`
		ObservedAt  time.Time       `json:"observed_at"`
		Details     json.RawMessage `json:"details"`
		DatapilotID string          `json:"datapilot_id,omitempty"`
	}
	integrity, err := integritySHA256(integrityInput{
		StateID:    stateID,
		RunID:      runID,
		Status:     status,
		ObservedAt: observedAt,
		Details:    detailsJSON,
	})
	if err != nil {
		return false, err
	}

	res, err := tx.ExecContext(ctx,
		`INSERT INTO experiment_run_state_events (state_id, run_id, status, observed_at, details, integrity_sha256)
		 VALUES ($1,$2,$3,$4,$5,$6)
		 ON CONFLICT (run_id, status) DO NOTHING`,
		stateID,
		runID,
		status,
		observedAt,
		detailsJSON,
		integrity,
	)
	if err != nil {
		return false, err
	}
	affected, _ := res.RowsAffected()
	return affected > 0, nil
}

func (api *experimentsAPI) insertRunEvent(ctx context.Context, tx *sql.Tx, runID string, actor string, level string, message string, metadata map[string]any) error {
	if tx == nil {
		return errors.New("tx is required")
	}
	runID = strings.TrimSpace(runID)
	actor = strings.TrimSpace(actor)
	level = strings.ToLower(strings.TrimSpace(level))
	message = strings.TrimSpace(message)
	if runID == "" || actor == "" || level == "" || message == "" {
		return errors.New("run_id, actor, level, and message are required")
	}
	if metadata == nil {
		metadata = map[string]any{}
	}
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return err
	}

	type integrityInput struct {
		RunID      string          `json:"run_id"`
		OccurredAt time.Time       `json:"occurred_at"`
		Actor      string          `json:"actor"`
		Level      string          `json:"level"`
		Message    string          `json:"message"`
		Metadata   json.RawMessage `json:"metadata"`
	}

	occurredAt := time.Now().UTC()
	integrity, err := integritySHA256(integrityInput{
		RunID:      runID,
		OccurredAt: occurredAt,
		Actor:      actor,
		Level:      level,
		Message:    message,
		Metadata:   metadataJSON,
	})
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx,
		`INSERT INTO experiment_run_events (
			run_id,
			occurred_at,
			actor,
			level,
			message,
			metadata,
			integrity_sha256
		) VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		runID,
		occurredAt,
		actor,
		level,
		message,
		metadataJSON,
		integrity,
	)
	return err
}
