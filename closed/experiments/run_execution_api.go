package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
)

type experimentRunExecution struct {
	ExecutionID       string          `json:"execution_id"`
	RunID             string          `json:"run_id"`
	Executor          string          `json:"executor"`
	ImageRef          string          `json:"image_ref"`
	ImageDigest       string          `json:"image_digest,omitempty"`
	Resources         json.RawMessage `json:"resources"`
	K8sNamespace      string          `json:"k8s_namespace,omitempty"`
	K8sJobName        string          `json:"k8s_job_name,omitempty"`
	DockerContainerID string          `json:"docker_container_id,omitempty"`
	DatapilotURL      string          `json:"datapilot_url"`
	CreatedAt         time.Time       `json:"created_at"`
	CreatedBy         string          `json:"created_by"`
}

type experimentRunBuildContext struct {
	RunID       string     `json:"run_id"`
	ImageRef    string     `json:"image_ref"`
	ImageDigest string     `json:"image_digest,omitempty"`
	Repo        string     `json:"repo,omitempty"`
	CommitSHA   string     `json:"commit_sha,omitempty"`
	PipelineID  string     `json:"pipeline_id,omitempty"`
	Provider    string     `json:"provider,omitempty"`
	ReceivedAt  *time.Time `json:"received_at,omitempty"`
}

func (api *experimentsAPI) handleGetExperimentRunExecution(w http.ResponseWriter, r *http.Request) {
	runID := strings.TrimSpace(r.PathValue("run_id"))
	if runID == "" {
		api.writeError(w, r, http.StatusBadRequest, "run_id_required")
		return
	}

	var (
		executionID       string
		executor          string
		imageRef          string
		imageDigest       sql.NullString
		resources         []byte
		k8sNamespace      sql.NullString
		k8sJobName        sql.NullString
		dockerContainerID sql.NullString
		datapilotURL      string
		createdAt         time.Time
		createdBy         string
	)
	err := api.db.QueryRowContext(
		r.Context(),
		`SELECT execution_id,
				executor,
				image_ref,
				image_digest,
				resources,
				k8s_namespace,
				k8s_job_name,
				docker_container_id,
				datapilot_url,
				created_at,
				created_by
		 FROM experiment_run_executions
		 WHERE run_id = $1`,
		runID,
	).Scan(&executionID, &executor, &imageRef, &imageDigest, &resources, &k8sNamespace, &k8sJobName, &dockerContainerID, &datapilotURL, &createdAt, &createdBy)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			api.writeError(w, r, http.StatusNotFound, "not_found")
			return
		}
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	api.writeJSON(w, http.StatusOK, experimentRunExecution{
		ExecutionID:       executionID,
		RunID:             runID,
		Executor:          executor,
		ImageRef:          imageRef,
		ImageDigest:       strings.TrimSpace(imageDigest.String),
		Resources:         normalizeJSON(resources),
		K8sNamespace:      k8sNamespace.String,
		K8sJobName:        k8sJobName.String,
		DockerContainerID: dockerContainerID.String,
		DatapilotURL:      datapilotURL,
		CreatedAt:         createdAt,
		CreatedBy:         createdBy,
	})
}

func (api *experimentsAPI) handleGetExperimentRunBuildContext(w http.ResponseWriter, r *http.Request) {
	runID := strings.TrimSpace(r.PathValue("run_id"))
	if runID == "" {
		api.writeError(w, r, http.StatusBadRequest, "run_id_required")
		return
	}

	var (
		imageRef    string
		imageDigest sql.NullString
		repo        sql.NullString
		commitSHA   sql.NullString
		pipelineID  sql.NullString
		provider    sql.NullString
		receivedAt  sql.NullTime
	)
	err := api.db.QueryRowContext(
		r.Context(),
		`SELECT e.image_ref,
				e.image_digest,
				m.repo,
				m.commit_sha,
				m.pipeline_id,
				m.provider,
				m.received_at
		 FROM experiment_run_executions e
		 LEFT JOIN model_images m ON m.image_digest = e.image_digest
		 WHERE e.run_id = $1`,
		runID,
	).Scan(&imageRef, &imageDigest, &repo, &commitSHA, &pipelineID, &provider, &receivedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			api.writeError(w, r, http.StatusNotFound, "not_found")
			return
		}
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	digest := strings.TrimSpace(imageDigest.String)
	if digest == "" {
		if parsed, ok := parseImageDigestFromRef(imageRef); ok {
			digest = parsed
		}
	}

	var receivedAtPtr *time.Time
	if receivedAt.Valid && !receivedAt.Time.IsZero() {
		t := receivedAt.Time.UTC()
		receivedAtPtr = &t
	}

	api.writeJSON(w, http.StatusOK, experimentRunBuildContext{
		RunID:       runID,
		ImageRef:    imageRef,
		ImageDigest: digest,
		Repo:        repo.String,
		CommitSHA:   commitSHA.String,
		PipelineID:  pipelineID.String,
		Provider:    provider.String,
		ReceivedAt:  receivedAtPtr,
	})
}
