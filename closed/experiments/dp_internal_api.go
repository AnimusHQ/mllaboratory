package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/animus-labs/animus-go/closed/internal/dataplane"
	"github.com/animus-labs/animus-go/closed/internal/domain"
	"github.com/animus-labs/animus-go/closed/internal/platform/auth"
	"github.com/animus-labs/animus-go/closed/internal/repo"
	"github.com/animus-labs/animus-go/closed/internal/repo/postgres"
	"github.com/animus-labs/animus-go/closed/internal/service/runs"
)

type dpEventIntegrityInput struct {
	EventID   string          `json:"event_id"`
	RunID     string          `json:"run_id"`
	ProjectID string          `json:"project_id"`
	EventType string          `json:"event_type"`
	EmittedAt time.Time       `json:"emitted_at"`
	Payload   json.RawMessage `json:"payload"`
}

func (api *experimentsAPI) handleDPHeartbeat(w http.ResponseWriter, r *http.Request) {
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

	var req dataplane.RunHeartbeat
	if err := decodeJSON(r, &req); err != nil {
		api.writeError(w, r, http.StatusBadRequest, "invalid_json")
		return
	}
	if strings.TrimSpace(req.RunID) == "" {
		api.writeError(w, r, http.StatusBadRequest, "run_id_required")
		return
	}
	if strings.TrimSpace(req.EventID) == "" {
		api.writeError(w, r, http.StatusBadRequest, "event_id_required")
		return
	}
	if strings.TrimSpace(req.ProjectID) == "" {
		api.writeError(w, r, http.StatusBadRequest, "project_id_required")
		return
	}
	if !strings.EqualFold(req.RunID, runID) {
		api.writeError(w, r, http.StatusBadRequest, "run_id_mismatch")
		return
	}
	if req.EmittedAt.IsZero() {
		api.writeError(w, r, http.StatusBadRequest, "emitted_at_required")
		return
	}

	projectID := strings.TrimSpace(req.ProjectID)
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

	payloadJSON, err := json.Marshal(req)
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	integrity, err := integritySHA256(dpEventIntegrityInput{
		EventID:   strings.TrimSpace(req.EventID),
		RunID:     runID,
		ProjectID: projectID,
		EventType: dataplane.EventTypeHeartbeat,
		EmittedAt: req.EmittedAt.UTC(),
		Payload:   payloadJSON,
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

	dpStore := postgres.NewDPEventStore(tx)
	runStoreTx := postgres.NewRunSpecStore(tx)
	if dpStore == nil || runStoreTx == nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	inserted, err := dpStore.InsertEvent(r.Context(), postgres.RunDPEventRecord{
		EventID:      req.EventID,
		RunID:        runID,
		ProjectID:    projectID,
		EventType:    dataplane.EventTypeHeartbeat,
		Payload:      payloadJSON,
		EmittedAt:    req.EmittedAt.UTC(),
		ReceivedAt:   time.Now().UTC(),
		IntegritySHA: integrity,
	})
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	duplicate := !inserted
	if duplicate {
		if err := ensureDPEventMatches(r.Context(), dpStore, req.EventID, runID, projectID, dataplane.EventTypeHeartbeat); err != nil {
			if errors.Is(err, errDPEventMismatch) {
				api.writeError(w, r, http.StatusConflict, "event_conflict")
				return
			}
			api.writeError(w, r, http.StatusInternalServerError, "internal_error")
			return
		}
	}

	var applied bool
	var stateErr error
	if inserted {
		applied, stateErr = updateRunStateWithAudit(r.Context(), tx, runStoreTx, runRecord.SpecHash, runs.AuditInfo{
			Actor:     identity.Subject,
			RequestID: r.Header.Get("X-Request-Id"),
			UserAgent: r.UserAgent(),
			IP:        requestIP(r.RemoteAddr),
			Service:   "experiments",
		}, projectID, runID, domain.RunStateRunning)
	}
	if stateErr != nil && !errors.Is(stateErr, repo.ErrInvalidTransition) {
		api.writeRepoError(w, r, stateErr)
		return
	}

	if applied {
		_ = updateDispatchStatus(r.Context(), dpStore, projectID, runID, dataplane.DispatchStatusRunning, "")
	}

	if err := tx.Commit(); err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	if errors.Is(stateErr, repo.ErrInvalidTransition) {
		api.writeError(w, r, http.StatusConflict, "invalid_transition")
		return
	}

	api.writeJSON(w, http.StatusOK, dataplane.RunHeartbeatResponse{
		Accepted:  !duplicate,
		Duplicate: duplicate,
	})
}

func (api *experimentsAPI) handleDPTerminal(w http.ResponseWriter, r *http.Request) {
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

	var req dataplane.RunTerminalState
	if err := decodeJSON(r, &req); err != nil {
		api.writeError(w, r, http.StatusBadRequest, "invalid_json")
		return
	}
	if strings.TrimSpace(req.RunID) == "" {
		api.writeError(w, r, http.StatusBadRequest, "run_id_required")
		return
	}
	if strings.TrimSpace(req.EventID) == "" {
		api.writeError(w, r, http.StatusBadRequest, "event_id_required")
		return
	}
	if strings.TrimSpace(req.ProjectID) == "" {
		api.writeError(w, r, http.StatusBadRequest, "project_id_required")
		return
	}
	if !strings.EqualFold(req.RunID, runID) {
		api.writeError(w, r, http.StatusBadRequest, "run_id_mismatch")
		return
	}
	if req.EmittedAt.IsZero() {
		api.writeError(w, r, http.StatusBadRequest, "emitted_at_required")
		return
	}

	projectID := strings.TrimSpace(req.ProjectID)
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

	nextState := domain.NormalizeRunState(req.State)
	if nextState == "" || !domain.IsTerminalRunState(nextState) {
		api.writeError(w, r, http.StatusBadRequest, "invalid_terminal_state")
		return
	}

	payloadJSON, err := json.Marshal(req)
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	integrity, err := integritySHA256(dpEventIntegrityInput{
		EventID:   strings.TrimSpace(req.EventID),
		RunID:     runID,
		ProjectID: projectID,
		EventType: dataplane.EventTypeTerminal,
		EmittedAt: req.EmittedAt.UTC(),
		Payload:   payloadJSON,
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

	dpStore := postgres.NewDPEventStore(tx)
	runStoreTx := postgres.NewRunSpecStore(tx)
	if dpStore == nil || runStoreTx == nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	inserted, err := dpStore.InsertEvent(r.Context(), postgres.RunDPEventRecord{
		EventID:      req.EventID,
		RunID:        runID,
		ProjectID:    projectID,
		EventType:    dataplane.EventTypeTerminal,
		Payload:      payloadJSON,
		EmittedAt:    req.EmittedAt.UTC(),
		ReceivedAt:   time.Now().UTC(),
		IntegritySHA: integrity,
	})
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	duplicate := !inserted
	if duplicate {
		if err := ensureDPEventMatches(r.Context(), dpStore, req.EventID, runID, projectID, dataplane.EventTypeTerminal); err != nil {
			if errors.Is(err, errDPEventMismatch) {
				api.writeError(w, r, http.StatusConflict, "event_conflict")
				return
			}
			api.writeError(w, r, http.StatusInternalServerError, "internal_error")
			return
		}
	}

	var applied bool
	var stateErr error
	if inserted {
		applied, stateErr = updateRunStateWithAudit(r.Context(), tx, runStoreTx, runRecord.SpecHash, runs.AuditInfo{
			Actor:     identity.Subject,
			RequestID: r.Header.Get("X-Request-Id"),
			UserAgent: r.UserAgent(),
			IP:        requestIP(r.RemoteAddr),
			Service:   "experiments",
		}, projectID, runID, nextState)
	}
	if stateErr != nil && !errors.Is(stateErr, repo.ErrInvalidTransition) {
		api.writeRepoError(w, r, stateErr)
		return
	}

	if applied {
		dispatchStatus := dispatchStatusFromRunState(nextState)
		lastError := strings.TrimSpace(req.Reason)
		if err := updateDispatchStatus(r.Context(), dpStore, projectID, runID, dispatchStatus, lastError); err != nil {
			api.writeError(w, r, http.StatusInternalServerError, "internal_error")
			return
		}
	}

	if err := tx.Commit(); err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	if errors.Is(stateErr, repo.ErrInvalidTransition) {
		api.writeError(w, r, http.StatusConflict, "invalid_transition")
		return
	}

	api.writeJSON(w, http.StatusOK, dataplane.RunTerminalResponse{
		Accepted:  !duplicate,
		Duplicate: duplicate,
	})
}

func (api *experimentsAPI) handleDPArtifactCommitted(w http.ResponseWriter, r *http.Request) {
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

	var req dataplane.ArtifactCommitted
	if err := decodeJSON(r, &req); err != nil {
		api.writeError(w, r, http.StatusBadRequest, "invalid_json")
		return
	}
	if strings.TrimSpace(req.RunID) == "" {
		api.writeError(w, r, http.StatusBadRequest, "run_id_required")
		return
	}
	if strings.TrimSpace(req.EventID) == "" {
		api.writeError(w, r, http.StatusBadRequest, "event_id_required")
		return
	}
	if strings.TrimSpace(req.ProjectID) == "" {
		api.writeError(w, r, http.StatusBadRequest, "project_id_required")
		return
	}
	if !strings.EqualFold(req.RunID, runID) {
		api.writeError(w, r, http.StatusBadRequest, "run_id_mismatch")
		return
	}
	if req.EmittedAt.IsZero() {
		api.writeError(w, r, http.StatusBadRequest, "emitted_at_required")
		return
	}

	projectID := strings.TrimSpace(req.ProjectID)
	runStore := postgres.NewRunSpecStore(api.db)
	if runStore == nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	if _, err := runStore.GetRun(r.Context(), projectID, runID); err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			api.writeError(w, r, http.StatusNotFound, "not_found")
			return
		}
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	payloadJSON, err := json.Marshal(req)
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	integrity, err := integritySHA256(dpEventIntegrityInput{
		EventID:   strings.TrimSpace(req.EventID),
		RunID:     runID,
		ProjectID: projectID,
		EventType: dataplane.EventTypeArtifactCommitted,
		EmittedAt: req.EmittedAt.UTC(),
		Payload:   payloadJSON,
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

	dpStore := postgres.NewDPEventStore(tx)
	if dpStore == nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	inserted, err := dpStore.InsertEvent(r.Context(), postgres.RunDPEventRecord{
		EventID:      req.EventID,
		RunID:        runID,
		ProjectID:    projectID,
		EventType:    dataplane.EventTypeArtifactCommitted,
		Payload:      payloadJSON,
		EmittedAt:    req.EmittedAt.UTC(),
		ReceivedAt:   time.Now().UTC(),
		IntegritySHA: integrity,
	})
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	duplicate := !inserted
	if duplicate {
		if err := ensureDPEventMatches(r.Context(), dpStore, req.EventID, runID, projectID, dataplane.EventTypeArtifactCommitted); err != nil {
			if errors.Is(err, errDPEventMismatch) {
				api.writeError(w, r, http.StatusConflict, "event_conflict")
				return
			}
			api.writeError(w, r, http.StatusInternalServerError, "internal_error")
			return
		}
	}

	if err := tx.Commit(); err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	api.writeJSON(w, http.StatusOK, dataplane.ArtifactCommittedResponse{
		Accepted:  !duplicate,
		Duplicate: duplicate,
	})
}

var errDPEventMismatch = errors.New("dp event mismatch")

type dpEventReader interface {
	GetEvent(ctx context.Context, eventID string) (postgres.RunDPEventRecord, error)
}

func ensureDPEventMatches(ctx context.Context, store dpEventReader, eventID, runID, projectID, eventType string) error {
	record, err := store.GetEvent(ctx, eventID)
	if err != nil {
		return err
	}
	if strings.TrimSpace(record.RunID) != strings.TrimSpace(runID) {
		return errDPEventMismatch
	}
	if strings.TrimSpace(record.ProjectID) != strings.TrimSpace(projectID) {
		return errDPEventMismatch
	}
	if strings.TrimSpace(record.EventType) != strings.TrimSpace(eventType) {
		return errDPEventMismatch
	}
	return nil
}

func updateRunStateWithAudit(ctx context.Context, tx *sql.Tx, runStore *postgres.RunSpecStore, specHash string, info runs.AuditInfo, projectID, runID string, next domain.RunState) (bool, error) {
	prev, applied, err := runStore.UpdateDerivedStatus(ctx, projectID, runID, next)
	if err != nil {
		return false, err
	}
	if !applied {
		return false, nil
	}
	appender := runs.NewAuditAppender(tx)
	if appender == nil {
		return false, errors.New("audit appender unavailable")
	}
	event, ok, err := runs.BuildRunTransitionEvent(info, projectID, runID, specHash, prev, next)
	if err != nil {
		return applied, err
	}
	if ok {
		if err := appender.Append(ctx, *event); err != nil {
			return applied, err
		}
	}
	return applied, nil
}

func updateDispatchStatus(ctx context.Context, store *postgres.DPEventStore, projectID, runID, status, lastError string) error {
	record, err := store.GetDispatchByRunID(ctx, projectID, runID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) || errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return err
	}
	if isTerminalDispatchStatus(record.Status) {
		return nil
	}
	return store.UpdateDispatchStatus(ctx, record.DispatchID, status, lastError, time.Now().UTC())
}

func isTerminalDispatchStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case dataplane.DispatchStatusSucceeded, dataplane.DispatchStatusFailed, dataplane.DispatchStatusCanceled:
		return true
	default:
		return false
	}
}

func dispatchStatusFromRunState(state domain.RunState) string {
	switch state {
	case domain.RunStateSucceeded:
		return dataplane.DispatchStatusSucceeded
	case domain.RunStateFailed:
		return dataplane.DispatchStatusFailed
	case domain.RunStateCanceled:
		return dataplane.DispatchStatusCanceled
	case domain.RunStateRunning:
		return dataplane.DispatchStatusRunning
	default:
		return dataplane.DispatchStatusError
	}
}

func (api *experimentsAPI) handleDPSecretAccessed(w http.ResponseWriter, r *http.Request) {
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

	var req dataplane.SecretAccessed
	if err := decodeJSON(r, &req); err != nil {
		api.writeError(w, r, http.StatusBadRequest, "invalid_json")
		return
	}
	if strings.TrimSpace(req.RunID) == "" {
		api.writeError(w, r, http.StatusBadRequest, "run_id_required")
		return
	}
	if strings.TrimSpace(req.EventID) == "" {
		api.writeError(w, r, http.StatusBadRequest, "event_id_required")
		return
	}
	if strings.TrimSpace(req.ProjectID) == "" {
		api.writeError(w, r, http.StatusBadRequest, "project_id_required")
		return
	}
	if !strings.EqualFold(req.RunID, runID) {
		api.writeError(w, r, http.StatusBadRequest, "run_id_mismatch")
		return
	}
	if req.EmittedAt.IsZero() {
		api.writeError(w, r, http.StatusBadRequest, "emitted_at_required")
		return
	}

	projectID := strings.TrimSpace(req.ProjectID)
	runStore := postgres.NewRunSpecStore(api.db)
	if runStore == nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	if _, err := runStore.GetRun(r.Context(), projectID, runID); err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			api.writeError(w, r, http.StatusNotFound, "not_found")
			return
		}
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	payloadJSON, err := json.Marshal(req)
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	integrity, err := integritySHA256(dpEventIntegrityInput{
		EventID:   strings.TrimSpace(req.EventID),
		RunID:     runID,
		ProjectID: projectID,
		EventType: dataplane.EventTypeSecretAccessed,
		EmittedAt: req.EmittedAt.UTC(),
		Payload:   payloadJSON,
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

	dpStore := postgres.NewDPEventStore(tx)
	if dpStore == nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	inserted, err := dpStore.InsertEvent(r.Context(), postgres.RunDPEventRecord{
		EventID:      req.EventID,
		RunID:        runID,
		ProjectID:    projectID,
		EventType:    dataplane.EventTypeSecretAccessed,
		Payload:      payloadJSON,
		EmittedAt:    req.EmittedAt.UTC(),
		ReceivedAt:   time.Now().UTC(),
		IntegritySHA: integrity,
	})
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	duplicate := !inserted
	if duplicate {
		if err := ensureDPEventMatches(r.Context(), dpStore, req.EventID, runID, projectID, dataplane.EventTypeSecretAccessed); err != nil {
			if errors.Is(err, errDPEventMismatch) {
				api.writeError(w, r, http.StatusConflict, "event_conflict")
				return
			}
			api.writeError(w, r, http.StatusInternalServerError, "internal_error")
			return
		}
	}

	if inserted {
		auditAppender := postgres.NewAuditAppender(tx, nil)
		if auditAppender != nil {
			payload := domain.Metadata{
				"project_id":     projectID,
				"run_id":         runID,
				"subject":        strings.TrimSpace(req.Subject),
				"class_ref":      strings.TrimSpace(req.ClassRef),
				"lease_id":       strings.TrimSpace(req.LeaseID),
				"correlation_id": strings.TrimSpace(req.CorrelationID),
			}
			_, _ = auditAppender.Append(r.Context(), domain.AuditEvent{
				OccurredAt:   time.Now().UTC(),
				Actor:        strings.TrimSpace(identity.Subject),
				Action:       "secret.accessed",
				ResourceType: "run",
				ResourceID:   runID,
				RequestID:    r.Header.Get("X-Request-Id"),
				IP:           requestIP(r.RemoteAddr),
				UserAgent:    r.UserAgent(),
				Payload:      payload,
			})
		}
	}

	if err := tx.Commit(); err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	api.writeJSON(w, http.StatusOK, dataplane.SecretAccessedResponse{
		Accepted:  !duplicate,
		Duplicate: duplicate,
	})
}
