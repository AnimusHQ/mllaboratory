package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/animus-labs/animus-go/closed/internal/platform/auditlog"
	"github.com/animus-labs/animus-go/closed/internal/platform/auth"
	"github.com/animus-labs/animus-go/closed/internal/repo"
	"github.com/animus-labs/animus-go/closed/internal/repo/postgres"
)

const reproBundleSchemaV1 = "animus.repro_bundle.v1"

type reproBundleResponse struct {
	Schema            string          `json:"schema"`
	RunID             string          `json:"runId"`
	ProjectID         string          `json:"projectId"`
	SpecHash          string          `json:"specHash"`
	RunSpec           json.RawMessage `json:"runSpec"`
	PolicySnapshotSHA string          `json:"policySnapshotSha256"`
	GeneratedAt       time.Time       `json:"generatedAt"`
	GeneratedBy       string          `json:"generatedBy,omitempty"`
}

func (api *experimentsAPI) handleGetRunReproducibilityBundle(w http.ResponseWriter, r *http.Request) {
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
	bindingsStore := postgres.NewRunBindingsStore(api.db)
	if runStore == nil || bindingsStore == nil {
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

	policySHA, err := bindingsStore.PolicySnapshotSHA(r.Context(), projectID, runID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			api.writeError(w, r, http.StatusNotFound, "policy_snapshot_not_found")
			return
		}
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	now := time.Now().UTC()
	if _, err := auditlog.Insert(
		r.Context(),
		api.db,
		reproBundleAuditEvent(now, identity, r, projectID, runID, record.SpecHash, policySHA),
	); err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "audit_failed")
		return
	}

	api.writeJSON(w, http.StatusOK, reproBundleResponse{
		Schema:            reproBundleSchemaV1,
		RunID:             runID,
		ProjectID:         projectID,
		SpecHash:          record.SpecHash,
		RunSpec:           json.RawMessage(record.RunSpec),
		PolicySnapshotSHA: policySHA,
		GeneratedAt:       now,
		GeneratedBy:       identity.Subject,
	})
}

func reproBundleAuditEvent(now time.Time, identity auth.Identity, r *http.Request, projectID, runID, specHash, policySHA string) auditlog.Event {
	return auditlog.Event{
		OccurredAt:   now,
		Actor:        identity.Subject,
		Action:       "run.repro_bundle.exported",
		ResourceType: "run",
		ResourceID:   runID,
		RequestID:    r.Header.Get("X-Request-Id"),
		IP:           requestIP(r.RemoteAddr),
		UserAgent:    r.UserAgent(),
		Payload: map[string]any{
			"service":                "experiments",
			"project_id":             projectID,
			"run_id":                 runID,
			"spec_hash":              specHash,
			"policy_snapshot_sha256": policySHA,
		},
	}
}
