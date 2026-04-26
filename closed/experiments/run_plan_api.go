package main

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/animus-labs/animus-go/closed/internal/execution/plan"
	"github.com/animus-labs/animus-go/closed/internal/platform/auditlog"
	"github.com/animus-labs/animus-go/closed/internal/platform/auth"
	"github.com/animus-labs/animus-go/closed/internal/repo"
	"github.com/animus-labs/animus-go/closed/internal/repo/postgres"
	"github.com/animus-labs/animus-go/closed/internal/service/runs"
)

type planSummaryResponse struct {
	RunID     string   `json:"runId"`
	StepCount int      `json:"stepCount"`
	StepNames []string `json:"stepNames"`
	State     string   `json:"state"`
}

func (api *experimentsAPI) handlePlanRun(w http.ResponseWriter, r *http.Request) {
	identity, ok := auth.IdentityFromContext(r.Context())
	if !ok || strings.TrimSpace(identity.Subject) == "" {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

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
	if runStore == nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	runRecord, err := runStore.GetRun(r.Context(), projectID, runID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			api.writeError(w, r, http.StatusNotFound, "not_found")
			return
		}
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	pipelineSpec, err := decodePipelineSpec(runRecord.PipelineSpec)
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "invalid_pipeline_spec")
		return
	}

	execPlan, err := plan.BuildPlan(pipelineSpec, runID, projectID)
	if err != nil {
		api.writeError(w, r, http.StatusBadRequest, "invalid_pipeline_spec")
		return
	}

	planJSON, err := plan.MarshalExecutionPlan(execPlan)
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

	runStoreTx := postgres.NewRunSpecStore(tx)
	planStore := postgres.NewPlanStore(tx)
	stepStore := postgres.NewStepExecutionStore(tx)
	if runStoreTx == nil || planStore == nil || stepStore == nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	stateSvc := runs.New(runStoreTx, planStore, stepStore)
	if stateSvc == nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	planRecord, err := planStore.UpsertPlan(r.Context(), projectID, runID, planJSON)
	if err != nil {
		api.writeError(w, r, http.StatusConflict, "plan_conflict")
		return
	}

	_, err = auditlog.Insert(r.Context(), tx, auditlog.Event{
		OccurredAt:   time.Now().UTC(),
		Actor:        identity.Subject,
		Action:       "execution.planned",
		ResourceType: "execution_plan",
		ResourceID:   planRecord.ID,
		RequestID:    r.Header.Get("X-Request-Id"),
		IP:           requestIP(r.RemoteAddr),
		UserAgent:    r.UserAgent(),
		Payload: map[string]any{
			"service":    "experiments",
			"project_id": projectID,
			"run_id":     runID,
			"spec_hash":  runRecord.SpecHash,
		},
	})
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "audit_failed")
		return
	}

	auditInfo := runs.AuditInfo{
		Actor:     identity.Subject,
		RequestID: r.Header.Get("X-Request-Id"),
		UserAgent: r.UserAgent(),
		IP:        requestIP(r.RemoteAddr),
		Service:   "experiments",
	}
	auditAppender := runs.NewAuditAppender(tx)
	if auditAppender == nil {
		api.writeError(w, r, http.StatusInternalServerError, "audit_failed")
		return
	}
	_, _, derived, err := stateSvc.DeriveAndPersistWithAudit(r.Context(), auditAppender, auditInfo, projectID, runID, runRecord.SpecHash)
	if err != nil {
		api.writeRepoError(w, r, err)
		return
	}

	if err := tx.Commit(); err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	stepNames := make([]string, 0, len(execPlan.Steps))
	for _, step := range execPlan.Steps {
		stepNames = append(stepNames, step.Name)
	}
	api.writeJSON(w, http.StatusOK, planSummaryResponse{
		RunID:     runID,
		StepCount: len(execPlan.Steps),
		StepNames: stepNames,
		State:     string(derived),
	})
}

func (api *experimentsAPI) handleGetRunPlan(w http.ResponseWriter, r *http.Request) {
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

	planStore := postgres.NewPlanStore(api.db)
	if planStore == nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	record, err := planStore.GetPlan(r.Context(), projectID, runID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			api.writeError(w, r, http.StatusNotFound, "not_found")
			return
		}
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(record.Plan)
}
