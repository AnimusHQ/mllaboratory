package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/animus-labs/animus-go/closed/internal/domain"
	"github.com/animus-labs/animus-go/closed/internal/execution/specvalidator"
	"github.com/animus-labs/animus-go/closed/internal/platform/auditlog"
	"github.com/animus-labs/animus-go/closed/internal/platform/auth"
	"github.com/animus-labs/animus-go/closed/internal/repo"
	"github.com/animus-labs/animus-go/closed/internal/repo/postgres"
)

const runSpecVersion = "1.0"

var (
	errPipelineSpecRequired  = errors.New("pipeline spec required")
	errInvalidPipelineSpec   = errors.New("invalid pipeline spec")
	errDatasetBindingsNeeded = errors.New("dataset bindings required")
	errInvalidRunSpec        = errors.New("invalid run spec")
	errDatasetVersionMissing = errors.New("dataset version not found")
	errEnvLockMissing        = errors.New("environment lock not found")
)

type createRunRequest struct {
	IdempotencyKey  string            `json:"idempotencyKey"`
	PipelineSpec    json.RawMessage   `json:"pipelineSpec"`
	DatasetBindings map[string]string `json:"datasetBindings"`
	CodeRef         runSpecCodeRef    `json:"codeRef"`
	EnvLock         runSpecEnvLockRef `json:"envLock"`
	Parameters      map[string]any    `json:"parameters"`
}

type runSpecCodeRef struct {
	RepoURL   string `json:"repoUrl"`
	CommitSHA string `json:"commitSha"`
	Path      string `json:"path,omitempty"`
	SCMType   string `json:"scmType,omitempty"`
}

type runSpecEnvLockRef struct {
	LockID string `json:"lockId"`
}

type runSpecEnvLockPayload struct {
	LockID                       string                      `json:"lockId"`
	EnvironmentDefinitionID      string                      `json:"environmentDefinitionId"`
	EnvironmentDefinitionVersion int                         `json:"environmentDefinitionVersion"`
	Images                       []domain.EnvironmentImage   `json:"images"`
	ResourceDefaults             domain.EnvironmentResources `json:"resourceDefaults,omitempty"`
	ResourceLimits               domain.EnvironmentResources `json:"resourceLimits,omitempty"`
	AllowedAccelerators          []string                    `json:"allowedAccelerators,omitempty"`
	NetworkClassRef              string                      `json:"networkClassRef,omitempty"`
	SecretAccessClassRef         string                      `json:"secretAccessClassRef,omitempty"`
	DependencyChecksums          map[string]string           `json:"dependencyChecksums,omitempty"`
	SBOMRef                      string                      `json:"sbomRef,omitempty"`
	EnvHash                      string                      `json:"envHash"`
}

type createRunResponse struct {
	RunID    string `json:"runId"`
	Status   string `json:"status"`
	Created  bool   `json:"created"`
	SpecHash string `json:"specHash"`
}

type getRunResponse struct {
	RunID          string          `json:"runId"`
	Status         string          `json:"status"`
	State          string          `json:"state"`
	SpecHash       string          `json:"specHash"`
	CreatedAt      time.Time       `json:"createdAt"`
	PlanExists     bool            `json:"planExists"`
	AttemptsByStep map[string]int  `json:"attemptsByStep,omitempty"`
	RunSpec        json.RawMessage `json:"runSpec,omitempty"`
}

func (api *experimentsAPI) handleCreateRun(w http.ResponseWriter, r *http.Request) {
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

	var req createRunRequest
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

	lockID := strings.TrimSpace(req.EnvLock.LockID)
	if lockID == "" {
		api.writeError(w, r, http.StatusBadRequest, "environment_lock_required")
		return
	}
	envStore := postgres.NewEnvironmentStore(api.db)
	if envStore == nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	lockRecord, err := envStore.GetLock(r.Context(), projectID, lockID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			api.writeError(w, r, http.StatusNotFound, "environment_lock_not_found")
			return
		}
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	policySnapshot, err := api.buildPolicySnapshot(r.Context(), projectID, identity, lockRecord.Lock)
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "policy_snapshot_failed")
		return
	}

	_, runSpec, err := buildRunSpec(projectID, identity.Subject, req, lockRecord.Lock, policySnapshot)
	if err != nil {
		switch err {
		case errPipelineSpecRequired:
			api.writeError(w, r, http.StatusBadRequest, "pipeline_spec_required")
		case errInvalidPipelineSpec:
			api.writeError(w, r, http.StatusBadRequest, "invalid_pipeline_spec")
		case errDatasetBindingsNeeded:
			api.writeError(w, r, http.StatusBadRequest, "dataset_bindings_required")
		case errInvalidRunSpec:
			api.writeError(w, r, http.StatusBadRequest, "invalid_run_spec")
		case errDatasetVersionMissing:
			api.writeError(w, r, http.StatusNotFound, "dataset_version_not_found")
		case errEnvLockMissing:
			api.writeError(w, r, http.StatusNotFound, "environment_lock_not_found")
		default:
			api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		}
		return
	}

	if err := api.ensureDatasetBindingsExist(r.Context(), projectID, runSpec.DatasetBindings); err != nil {
		if errors.Is(err, errDatasetVersionMissing) {
			api.writeError(w, r, http.StatusNotFound, "dataset_version_not_found")
			return
		}
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	specHash, err := hashRunSpec(runSpec)
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	pipelineSpecJSON := bytes.TrimSpace(req.PipelineSpec)
	runSpecJSON, err := marshalRunSpec(runSpec, pipelineSpecJSON)
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

	runStore := postgres.NewRunSpecStore(tx)
	bindingsStore := postgres.NewRunBindingsStore(tx)
	if runStore == nil || bindingsStore == nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	record, created, err := runStore.CreateRun(r.Context(), projectID, idempotencyKey, pipelineSpecJSON, runSpecJSON, specHash)
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	if !created && record.SpecHash != specHash {
		api.writeError(w, r, http.StatusConflict, "idempotency_conflict")
		return
	}

	if err := api.persistRunBindings(r.Context(), bindingsStore, record.ID, projectID, runSpec, identity.Subject); err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "run_binding_failed")
		return
	}

	if created {
		now := time.Now().UTC()
		_, err = auditlog.Insert(r.Context(), tx, auditlog.Event{
			OccurredAt:   now,
			Actor:        identity.Subject,
			Action:       "run.created",
			ResourceType: "run",
			ResourceID:   record.ID,
			RequestID:    r.Header.Get("X-Request-Id"),
			IP:           requestIP(r.RemoteAddr),
			UserAgent:    r.UserAgent(),
			Payload: map[string]any{
				"service":         "experiments",
				"project_id":      projectID,
				"run_id":          record.ID,
				"spec_hash":       specHash,
				"idempotency_key": idempotencyKey,
			},
		})
		if err != nil {
			api.writeError(w, r, http.StatusInternalServerError, "audit_failed")
			return
		}

		_, err = auditlog.Insert(r.Context(), tx, auditlog.Event{
			OccurredAt:   now,
			Actor:        identity.Subject,
			Action:       "run.validated",
			ResourceType: "run",
			ResourceID:   record.ID,
			RequestID:    r.Header.Get("X-Request-Id"),
			IP:           requestIP(r.RemoteAddr),
			UserAgent:    r.UserAgent(),
			Payload: map[string]any{
				"service":    "experiments",
				"project_id": projectID,
				"run_id":     record.ID,
				"spec_hash":  specHash,
			},
		})
		if err != nil {
			api.writeError(w, r, http.StatusInternalServerError, "audit_failed")
			return
		}

		_, err = auditlog.Insert(r.Context(), tx, auditlog.Event{
			OccurredAt:   now,
			Actor:        identity.Subject,
			Action:       "policy.snapshot.materialized",
			ResourceType: "policy_snapshot",
			ResourceID:   runSpec.PolicySnapshot.SnapshotSHA256,
			RequestID:    r.Header.Get("X-Request-Id"),
			IP:           requestIP(r.RemoteAddr),
			UserAgent:    r.UserAgent(),
			Payload: map[string]any{
				"service":                "experiments",
				"project_id":             projectID,
				"run_id":                 record.ID,
				"policy_snapshot_sha256": runSpec.PolicySnapshot.SnapshotSHA256,
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

	api.writeJSON(w, http.StatusOK, createRunResponse{
		RunID:    record.ID,
		Status:   record.Status,
		Created:  created,
		SpecHash: record.SpecHash,
	})
}

func (api *experimentsAPI) handleGetRun(w http.ResponseWriter, r *http.Request) {
	projectID := strings.TrimSpace(r.PathValue("project_id"))
	runID := strings.TrimSpace(r.PathValue("run_id"))
	if projectID == "" {
		api.writeError(w, r, http.StatusBadRequest, "project_id_required")
		return
	}
	if runID == "" {
		api.writeError(w, r, http.StatusBadRequest, "run_id_required")
		return
	}

	runStore := postgres.NewRunSpecStore(api.db)
	planStore := postgres.NewPlanStore(api.db)
	stepStore := postgres.NewStepExecutionStore(api.db)
	if runStore == nil || planStore == nil || stepStore == nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	record, err := runStore.GetRun(r.Context(), projectID, runID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			api.writeError(w, r, http.StatusNotFound, "not_found")
			return
		}
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	planSpec, planExists, err := loadExecutionPlan(r.Context(), planStore, projectID, runID)
	if err != nil {
		api.writeRepoError(w, r, err)
		return
	}
	stepExecutions, err := stepStore.ListByRun(r.Context(), projectID, runID)
	if err != nil {
		api.writeRepoError(w, r, err)
		return
	}
	derivedState := deriveRunStateFromRecords(planSpec, planExists, stepExecutions)

	api.writeJSON(w, http.StatusOK, getRunResponse{
		RunID:          record.ID,
		Status:         record.Status,
		State:          string(derivedState.State),
		SpecHash:       record.SpecHash,
		CreatedAt:      record.CreatedAt,
		PlanExists:     derivedState.PlanExists,
		AttemptsByStep: derivedState.AttemptsMap,
		RunSpec:        json.RawMessage(record.RunSpec),
	})
}

func decodePipelineSpec(raw json.RawMessage) (domain.PipelineSpec, error) {
	var spec domain.PipelineSpec
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&spec); err != nil {
		return domain.PipelineSpec{}, err
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return domain.PipelineSpec{}, errors.New("multiple JSON values")
	}
	return spec, nil
}

func buildRunSpec(projectID, actor string, req createRunRequest, envLock domain.EnvLock, snapshot domain.PolicySnapshot) (domain.PipelineSpec, domain.RunSpec, error) {
	if len(req.PipelineSpec) == 0 {
		return domain.PipelineSpec{}, domain.RunSpec{}, errPipelineSpecRequired
	}
	pipelineSpec, err := decodePipelineSpec(req.PipelineSpec)
	if err != nil {
		return domain.PipelineSpec{}, domain.RunSpec{}, errInvalidPipelineSpec
	}
	if err := specvalidator.ValidatePipelineSpec(pipelineSpec); err != nil {
		return domain.PipelineSpec{}, domain.RunSpec{}, errInvalidPipelineSpec
	}
	if req.DatasetBindings == nil {
		return domain.PipelineSpec{}, domain.RunSpec{}, errDatasetBindingsNeeded
	}
	if strings.TrimSpace(envLock.LockID) == "" {
		return domain.PipelineSpec{}, domain.RunSpec{}, errEnvLockMissing
	}

	runSpec := domain.RunSpec{
		RunSpecVersion:  runSpecVersion,
		ProjectID:       projectID,
		PipelineSpec:    pipelineSpec,
		DatasetBindings: req.DatasetBindings,
		CodeRef: domain.CodeRef{
			RepoURL:   strings.TrimSpace(req.CodeRef.RepoURL),
			CommitSHA: strings.TrimSpace(req.CodeRef.CommitSHA),
			Path:      strings.TrimSpace(req.CodeRef.Path),
			SCMType:   strings.TrimSpace(req.CodeRef.SCMType),
		},
		EnvLock:        envLock,
		Parameters:     req.Parameters,
		PolicySnapshot: snapshot,
		CreatedAt:      time.Now().UTC(),
		CreatedBy:      strings.TrimSpace(actor),
	}
	if err := specvalidator.ValidateRunSpec(runSpec); err != nil {
		return domain.PipelineSpec{}, domain.RunSpec{}, errInvalidRunSpec
	}
	return pipelineSpec, runSpec, nil
}

func marshalRunSpec(spec domain.RunSpec, pipelineSpecJSON []byte) ([]byte, error) {
	payload := runSpecPayload{
		RunSpecVersion:  spec.RunSpecVersion,
		ProjectID:       spec.ProjectID,
		PipelineSpec:    pipelineSpecJSON,
		DatasetBindings: spec.DatasetBindings,
		CodeRef: runSpecCodeRef{
			RepoURL:   spec.CodeRef.RepoURL,
			CommitSHA: spec.CodeRef.CommitSHA,
			Path:      spec.CodeRef.Path,
			SCMType:   spec.CodeRef.SCMType,
		},
		EnvLock: runSpecEnvLockPayload{
			LockID:                       spec.EnvLock.LockID,
			EnvironmentDefinitionID:      spec.EnvLock.EnvironmentDefinitionID,
			EnvironmentDefinitionVersion: spec.EnvLock.EnvironmentDefinitionVersion,
			Images:                       spec.EnvLock.Images,
			ResourceDefaults:             spec.EnvLock.ResourceDefaults,
			ResourceLimits:               spec.EnvLock.ResourceLimits,
			AllowedAccelerators:          spec.EnvLock.AllowedAccelerators,
			NetworkClassRef:              spec.EnvLock.NetworkClassRef,
			SecretAccessClassRef:         spec.EnvLock.SecretAccessClassRef,
			DependencyChecksums:          spec.EnvLock.DependencyChecksums,
			SBOMRef:                      spec.EnvLock.SBOMRef,
			EnvHash:                      spec.EnvLock.EnvHash,
		},
		Parameters:     spec.Parameters,
		PolicySnapshot: spec.PolicySnapshot,
		CreatedAt:      spec.CreatedAt,
		CreatedBy:      strings.TrimSpace(spec.CreatedBy),
	}
	return json.Marshal(payload)
}

type runSpecPayload struct {
	RunSpecVersion  string                `json:"runSpecVersion"`
	ProjectID       string                `json:"projectId"`
	PipelineSpec    json.RawMessage       `json:"pipelineSpec"`
	DatasetBindings map[string]string     `json:"datasetBindings"`
	CodeRef         runSpecCodeRef        `json:"codeRef"`
	EnvLock         runSpecEnvLockPayload `json:"envLock"`
	Parameters      map[string]any        `json:"parameters"`
	PolicySnapshot  domain.PolicySnapshot `json:"policySnapshot"`
	CreatedAt       time.Time             `json:"createdAt"`
	CreatedBy       string                `json:"createdBy,omitempty"`
}

func hashRunSpec(spec domain.RunSpec) (string, error) {
	canonical := canonicalRunSpec{
		RunSpecVersion:  spec.RunSpecVersion,
		ProjectID:       spec.ProjectID,
		PipelineSpec:    canonicalPipelineSpecFromDomain(spec.PipelineSpec),
		DatasetBindings: sortedBindings(spec.DatasetBindings),
		CodeRef: runSpecCodeRef{
			RepoURL:   spec.CodeRef.RepoURL,
			CommitSHA: spec.CodeRef.CommitSHA,
			Path:      spec.CodeRef.Path,
			SCMType:   spec.CodeRef.SCMType,
		},
		EnvLock: canonicalEnvLock{
			LockID:                       spec.EnvLock.LockID,
			EnvironmentDefinitionID:      spec.EnvLock.EnvironmentDefinitionID,
			EnvironmentDefinitionVersion: spec.EnvLock.EnvironmentDefinitionVersion,
			Images:                       canonicalImages(spec.EnvLock.Images),
			ResourceDefaults:             spec.EnvLock.ResourceDefaults,
			ResourceLimits:               spec.EnvLock.ResourceLimits,
			AllowedAccelerators:          canonicalAccelerators(spec.EnvLock.AllowedAccelerators),
			NetworkClassRef:              spec.EnvLock.NetworkClassRef,
			SecretAccessClassRef:         spec.EnvLock.SecretAccessClassRef,
			DependencyChecksums:          sortedChecksums(spec.EnvLock.DependencyChecksums),
			SBOMRef:                      spec.EnvLock.SBOMRef,
			EnvHash:                      spec.EnvLock.EnvHash,
		},
		Parameters:        canonicalParameters(spec.Parameters),
		PolicySnapshotSHA: spec.PolicySnapshot.SnapshotSHA256,
	}
	blob, err := json.Marshal(canonical)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(blob)
	return hex.EncodeToString(sum[:]), nil
}

type canonicalRunSpec struct {
	RunSpecVersion    string                `json:"runSpecVersion"`
	ProjectID         string                `json:"projectId"`
	PipelineSpec      canonicalPipelineSpec `json:"pipelineSpec"`
	DatasetBindings   []bindingPair         `json:"datasetBindings"`
	CodeRef           runSpecCodeRef        `json:"codeRef"`
	EnvLock           canonicalEnvLock      `json:"envLock"`
	Parameters        json.RawMessage       `json:"parameters"`
	PolicySnapshotSHA string                `json:"policySnapshotSha256"`
}

type canonicalEnvLock struct {
	LockID                       string                      `json:"lockId"`
	EnvironmentDefinitionID      string                      `json:"environmentDefinitionId"`
	EnvironmentDefinitionVersion int                         `json:"environmentDefinitionVersion"`
	Images                       []canonicalImage            `json:"images"`
	ResourceDefaults             domain.EnvironmentResources `json:"resourceDefaults,omitempty"`
	ResourceLimits               domain.EnvironmentResources `json:"resourceLimits,omitempty"`
	AllowedAccelerators          []string                    `json:"allowedAccelerators,omitempty"`
	NetworkClassRef              string                      `json:"networkClassRef,omitempty"`
	SecretAccessClassRef         string                      `json:"secretAccessClassRef,omitempty"`
	DependencyChecksums          []checksumPair              `json:"dependencyChecksums,omitempty"`
	SBOMRef                      string                      `json:"sbomRef,omitempty"`
	EnvHash                      string                      `json:"envHash"`
}

type bindingPair struct {
	DatasetRef       string `json:"datasetRef"`
	DatasetVersionID string `json:"datasetVersionId"`
}

type canonicalImage struct {
	Name   string `json:"name"`
	Ref    string `json:"ref"`
	Digest string `json:"digest"`
}

type checksumPair struct {
	Key      string `json:"key"`
	Checksum string `json:"checksum"`
}

type canonicalPipelineSpec struct {
	APIVersion  string                     `json:"apiVersion"`
	Kind        string                     `json:"kind"`
	SpecVersion string                     `json:"specVersion"`
	Metadata    *canonicalPipelineMetadata `json:"metadata,omitempty"`
	Spec        canonicalPipelineBody      `json:"spec"`
}

type canonicalPipelineMetadata struct {
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	Labels      []labelPair `json:"labels,omitempty"`
}

type labelPair struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type canonicalPipelineBody struct {
	Steps        []canonicalPipelineStep       `json:"steps"`
	Dependencies []canonicalPipelineDependency `json:"dependencies"`
}

type canonicalPipelineDependency struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type canonicalPipelineStep struct {
	Name        string                   `json:"name"`
	Image       string                   `json:"image"`
	Command     []string                 `json:"command"`
	Args        []string                 `json:"args"`
	Inputs      canonicalPipelineInputs  `json:"inputs"`
	Outputs     canonicalPipelineOutputs `json:"outputs"`
	Env         []canonicalEnvVar        `json:"env"`
	Resources   canonicalResources       `json:"resources"`
	RetryPolicy canonicalRetryPolicy     `json:"retryPolicy"`
}

type canonicalPipelineInputs struct {
	Datasets  []canonicalDatasetInput  `json:"datasets"`
	Artifacts []canonicalArtifactInput `json:"artifacts"`
}

type canonicalPipelineOutputs struct {
	Artifacts []canonicalArtifactOutput `json:"artifacts"`
}

type canonicalDatasetInput struct {
	Name       string `json:"name"`
	DatasetRef string `json:"datasetRef"`
}

type canonicalArtifactInput struct {
	Name     string `json:"name"`
	FromStep string `json:"fromStep"`
	Artifact string `json:"artifact"`
}

type canonicalArtifactOutput struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	MediaType   string `json:"mediaType,omitempty"`
	Description string `json:"description,omitempty"`
}

type canonicalEnvVar struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type canonicalResources struct {
	CPU    string `json:"cpu"`
	Memory string `json:"memory"`
	GPU    int    `json:"gpu"`
}

type canonicalRetryPolicy struct {
	MaxAttempts int              `json:"maxAttempts"`
	Backoff     canonicalBackoff `json:"backoff"`
}

type canonicalBackoff struct {
	Type           string  `json:"type"`
	InitialSeconds int     `json:"initialSeconds"`
	MaxSeconds     int     `json:"maxSeconds"`
	Multiplier     float64 `json:"multiplier"`
}

func canonicalPipelineSpecFromDomain(spec domain.PipelineSpec) canonicalPipelineSpec {
	var metadata *canonicalPipelineMetadata
	if spec.Metadata != nil {
		metadata = &canonicalPipelineMetadata{
			Name:        spec.Metadata.Name,
			Description: spec.Metadata.Description,
			Labels:      sortedLabels(spec.Metadata.Labels),
		}
	}

	steps := make([]canonicalPipelineStep, 0, len(spec.Spec.Steps))
	for _, step := range spec.Spec.Steps {
		steps = append(steps, canonicalPipelineStep{
			Name:    step.Name,
			Image:   step.Image,
			Command: step.Command,
			Args:    step.Args,
			Inputs: canonicalPipelineInputs{
				Datasets:  canonicalDatasetInputs(step.Inputs.Datasets),
				Artifacts: canonicalArtifactInputs(step.Inputs.Artifacts),
			},
			Outputs: canonicalPipelineOutputs{
				Artifacts: canonicalArtifactOutputs(step.Outputs.Artifacts),
			},
			Env:         canonicalEnvVars(step.Env),
			Resources:   canonicalResourcesFromDomain(step.Resources),
			RetryPolicy: canonicalRetryPolicyFromDomain(step.RetryPolicy),
		})
	}

	deps := make([]canonicalPipelineDependency, 0, len(spec.Spec.Dependencies))
	for _, dep := range spec.Spec.Dependencies {
		deps = append(deps, canonicalPipelineDependency{From: dep.From, To: dep.To})
	}

	return canonicalPipelineSpec{
		APIVersion:  spec.APIVersion,
		Kind:        spec.Kind,
		SpecVersion: spec.SpecVersion,
		Metadata:    metadata,
		Spec: canonicalPipelineBody{
			Steps:        steps,
			Dependencies: deps,
		},
	}
}

func canonicalDatasetInputs(inputs []domain.PipelineDatasetInput) []canonicalDatasetInput {
	out := make([]canonicalDatasetInput, 0, len(inputs))
	for _, input := range inputs {
		out = append(out, canonicalDatasetInput{
			Name:       input.Name,
			DatasetRef: input.DatasetRef,
		})
	}
	return out
}

func canonicalArtifactInputs(inputs []domain.PipelineArtifactInput) []canonicalArtifactInput {
	out := make([]canonicalArtifactInput, 0, len(inputs))
	for _, input := range inputs {
		out = append(out, canonicalArtifactInput{
			Name:     input.Name,
			FromStep: input.FromStep,
			Artifact: input.Artifact,
		})
	}
	return out
}

func canonicalArtifactOutputs(outputs []domain.PipelineArtifactOutput) []canonicalArtifactOutput {
	out := make([]canonicalArtifactOutput, 0, len(outputs))
	for _, output := range outputs {
		out = append(out, canonicalArtifactOutput{
			Name:        output.Name,
			Type:        output.Type,
			MediaType:   output.MediaType,
			Description: output.Description,
		})
	}
	return out
}

func canonicalEnvVars(vars []domain.EnvVar) []canonicalEnvVar {
	out := make([]canonicalEnvVar, 0, len(vars))
	for _, env := range vars {
		out = append(out, canonicalEnvVar{
			Name:  env.Name,
			Value: env.Value,
		})
	}
	return out
}

func canonicalResourcesFromDomain(res domain.PipelineResources) canonicalResources {
	return canonicalResources{
		CPU:    res.CPU,
		Memory: res.Memory,
		GPU:    res.GPU,
	}
}

func canonicalRetryPolicyFromDomain(policy domain.PipelineRetryPolicy) canonicalRetryPolicy {
	return canonicalRetryPolicy{
		MaxAttempts: policy.MaxAttempts,
		Backoff: canonicalBackoff{
			Type:           policy.Backoff.Type,
			InitialSeconds: policy.Backoff.InitialSeconds,
			MaxSeconds:     policy.Backoff.MaxSeconds,
			Multiplier:     policy.Backoff.Multiplier,
		},
	}
}

func sortedBindings(bindings map[string]string) []bindingPair {
	if len(bindings) == 0 {
		return []bindingPair{}
	}
	keys := make([]string, 0, len(bindings))
	for key := range bindings {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]bindingPair, 0, len(keys))
	for _, key := range keys {
		out = append(out, bindingPair{
			DatasetRef:       key,
			DatasetVersionID: bindings[key],
		})
	}
	return out
}

func canonicalImages(images []domain.EnvironmentImage) []canonicalImage {
	if len(images) == 0 {
		return []canonicalImage{}
	}
	out := make([]canonicalImage, 0, len(images))
	for _, image := range images {
		out = append(out, canonicalImage{
			Name:   image.Name,
			Ref:    image.Ref,
			Digest: image.Digest,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Name == out[j].Name {
			return out[i].Ref < out[j].Ref
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func canonicalAccelerators(accels []string) []string {
	if len(accels) == 0 {
		return []string{}
	}
	out := append([]string{}, accels...)
	sort.Strings(out)
	return out
}

func sortedChecksums(checksums map[string]string) []checksumPair {
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

func canonicalParameters(params map[string]any) json.RawMessage {
	if params == nil {
		return json.RawMessage(`{}`)
	}
	blob, err := json.Marshal(params)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return json.RawMessage(blob)
}

func sortedLabels(labels map[string]string) []labelPair {
	if len(labels) == 0 {
		return []labelPair{}
	}
	keys := make([]string, 0, len(labels))
	for key := range labels {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]labelPair, 0, len(keys))
	for _, key := range keys {
		out = append(out, labelPair{
			Key:   key,
			Value: labels[key],
		})
	}
	return out
}
