package main

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/animus-labs/animus-go/closed/internal/platform/auditlog"
	"github.com/animus-labs/animus-go/closed/internal/platform/auth"
	"github.com/animus-labs/animus-go/closed/internal/repo"
	"github.com/animus-labs/animus-go/closed/internal/repo/postgres"
)

func (api *experimentsAPI) handleGetRunPolicySnapshot(w http.ResponseWriter, r *http.Request) {
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

	store := postgres.NewRunBindingsStore(api.db)
	if store == nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	snapshot, err := store.GetPolicySnapshot(r.Context(), projectID, runID)
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
		Action:       "policy.snapshot.read",
		ResourceType: "policy_snapshot",
		ResourceID:   snapshot.SnapshotSHA256,
		RequestID:    r.Header.Get("X-Request-Id"),
		IP:           requestIP(r.RemoteAddr),
		UserAgent:    r.UserAgent(),
		Payload: map[string]any{
			"service":         "experiments",
			"project_id":      projectID,
			"run_id":          runID,
			"snapshot_sha256": snapshot.SnapshotSHA256,
		},
	}); err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "audit_failed")
		return
	}

	api.writeJSON(w, http.StatusOK, snapshot)
}
