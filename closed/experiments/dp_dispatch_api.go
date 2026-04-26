package main

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/animus-labs/animus-go/closed/internal/dataplane"
	"github.com/animus-labs/animus-go/closed/internal/domain"
	"github.com/animus-labs/animus-go/closed/internal/platform/auditlog"
	"github.com/animus-labs/animus-go/closed/internal/platform/auth"
	"github.com/animus-labs/animus-go/closed/internal/repo"
	"github.com/animus-labs/animus-go/closed/internal/repo/postgres"
	"github.com/google/uuid"
)

type runDispatchRequest struct {
	IdempotencyKey string `json:"idempotencyKey,omitempty"`
}

type runDispatchResponse struct {
	RunID      string `json:"runId"`
	ProjectID  string `json:"projectId"`
	DispatchID string `json:"dispatchId"`
	Status     string `json:"status"`
	DPBaseURL  string `json:"dpBaseUrl"`
	Created    bool   `json:"created"`
}

func (api *experimentsAPI) handleDispatchRun(w http.ResponseWriter, r *http.Request) {
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
	if strings.TrimSpace(api.dataplaneURL) == "" {
		api.writeError(w, r, http.StatusInternalServerError, "dataplane_url_not_configured")
		return
	}
	if strings.TrimSpace(api.runTokenSecret) == "" {
		api.writeError(w, r, http.StatusInternalServerError, "internal_auth_not_configured")
		return
	}

	var req runDispatchRequest
	if err := decodeJSON(r, &req); err != nil {
		api.writeError(w, r, http.StatusBadRequest, "invalid_json")
		return
	}

	idempotencyKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if idempotencyKey == "" {
		idempotencyKey = strings.TrimSpace(req.IdempotencyKey)
	}
	if idempotencyKey == "" {
		idempotencyKey = runID
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
	currentState := domain.NormalizeRunState(runRecord.Status)
	if domain.IsTerminalRunState(currentState) {
		api.writeError(w, r, http.StatusConflict, "run_terminal")
		return
	}

	dpStore := postgres.NewDPEventStore(api.db)
	if dpStore == nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	if existing, err := dpStore.GetDispatchByRunID(r.Context(), projectID, runID); err == nil {
		if shouldRetryDispatch(existing.Status) {
			status, err := api.dispatchToDataplane(r, runRecord, existing.DispatchID, identity)
			if err != nil {
				api.writeError(w, r, http.StatusInternalServerError, "internal_error")
				return
			}
			api.writeJSON(w, http.StatusOK, runDispatchResponse{
				RunID:      runID,
				ProjectID:  projectID,
				DispatchID: existing.DispatchID,
				Status:     status,
				DPBaseURL:  existing.DPBaseURL,
				Created:    false,
			})
			return
		}
		api.writeJSON(w, http.StatusOK, runDispatchResponse{
			RunID:      runID,
			ProjectID:  projectID,
			DispatchID: existing.DispatchID,
			Status:     existing.Status,
			DPBaseURL:  existing.DPBaseURL,
			Created:    false,
		})
		return
	}

	dispatchID := uuid.NewString()
	now := time.Now().UTC()
	integrity, err := integritySHA256(struct {
		DispatchID  string    `json:"dispatch_id"`
		RunID       string    `json:"run_id"`
		ProjectID   string    `json:"project_id"`
		DPBaseURL   string    `json:"dp_base_url"`
		SpecHash    string    `json:"spec_hash"`
		Requested   time.Time `json:"requested_at"`
		RequestedBy string    `json:"requested_by"`
	}{
		DispatchID:  dispatchID,
		RunID:       runID,
		ProjectID:   projectID,
		DPBaseURL:   api.dataplaneURL,
		SpecHash:    runRecord.SpecHash,
		Requested:   now,
		RequestedBy: identity.Subject,
	})
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	record, created, err := dpStore.CreateDispatch(r.Context(), postgres.RunDispatchRecord{
		DispatchID:     dispatchID,
		RunID:          runID,
		ProjectID:      projectID,
		IdempotencyKey: idempotencyKey,
		DPBaseURL:      api.dataplaneURL,
		Status:         dataplane.DispatchStatusRequested,
		SpecHash:       runRecord.SpecHash,
		RequestedAt:    now,
		RequestedBy:    identity.Subject,
		UpdatedAt:      now,
		IntegritySHA:   integrity,
	})
	if err != nil {
		api.writeRepoError(w, r, err)
		return
	}
	if !created {
		api.writeJSON(w, http.StatusOK, runDispatchResponse{
			RunID:      runID,
			ProjectID:  projectID,
			DispatchID: record.DispatchID,
			Status:     record.Status,
			DPBaseURL:  record.DPBaseURL,
			Created:    false,
		})
		return
	}

	status, err := api.dispatchToDataplane(r, runRecord, record.DispatchID, identity)
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	api.writeJSON(w, http.StatusOK, runDispatchResponse{
		RunID:      runID,
		ProjectID:  projectID,
		DispatchID: record.DispatchID,
		Status:     status,
		DPBaseURL:  api.dataplaneURL,
		Created:    true,
	})
}

func (api *experimentsAPI) dispatchToDataplane(r *http.Request, runRecord repo.RunRecord, dispatchID string, identity auth.Identity) (string, error) {
	client, err := newDataplaneClient(api.dataplaneURL, api.runTokenSecret)
	if err != nil {
		return dataplane.DispatchStatusError, err
	}

	status := dataplane.DispatchStatusRequested
	lastError := ""
	resp, statusCode, err := client.ExecuteRun(r.Context(), dataplane.RunExecutionRequest{
		RunID:         runRecord.ID,
		ProjectID:     runRecord.ProjectID,
		DispatchID:    dispatchID,
		EmittedAt:     time.Now().UTC(),
		RequestedBy:   identity.Subject,
		CorrelationID: r.Header.Get("X-Request-Id"),
	}, r.Header.Get("X-Request-Id"))
	if err == nil {
		if resp.Accepted {
			status = dataplane.DispatchStatusAccepted
		} else {
			status = dataplane.DispatchStatusRejected
		}
	} else if statusCode >= 400 && statusCode < 500 {
		status = dataplane.DispatchStatusRejected
		lastError = err.Error()
	} else {
		status = dataplane.DispatchStatusError
		lastError = err.Error()
	}

	tx, err := api.db.BeginTx(r.Context(), nil)
	if err != nil {
		return status, err
	}
	defer func() { _ = tx.Rollback() }()
	dpStore := postgres.NewDPEventStore(tx)
	if dpStore == nil {
		return status, errors.New("dp store unavailable")
	}
	if err := dpStore.UpdateDispatchStatus(r.Context(), dispatchID, status, lastError, time.Now().UTC()); err != nil {
		return status, err
	}

	if _, err := auditlog.Insert(r.Context(), tx, auditlog.Event{
		OccurredAt:   time.Now().UTC(),
		Actor:        identity.Subject,
		Action:       "run.dispatched",
		ResourceType: "run",
		ResourceID:   runRecord.ID,
		RequestID:    r.Header.Get("X-Request-Id"),
		IP:           requestIP(r.RemoteAddr),
		UserAgent:    r.UserAgent(),
		Payload: map[string]any{
			"service":       "experiments",
			"project_id":    runRecord.ProjectID,
			"run_id":        runRecord.ID,
			"spec_hash":     runRecord.SpecHash,
			"dispatch_id":   dispatchID,
			"status":        status,
			"dp_base_url":   api.dataplaneURL,
			"requested_by":  identity.Subject,
			"response_code": statusCode,
		},
	}); err != nil {
		return status, err
	}

	if err := tx.Commit(); err != nil {
		return status, err
	}

	return status, nil
}

func shouldRetryDispatch(status string) bool {
	switch strings.TrimSpace(status) {
	case dataplane.DispatchStatusRequested, dataplane.DispatchStatusRejected, dataplane.DispatchStatusError:
		return true
	default:
		return false
	}
}
