package main

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/animus-labs/animus-go/closed/internal/dataplane"
	"github.com/animus-labs/animus-go/closed/internal/domain"
	"github.com/animus-labs/animus-go/closed/internal/platform/auditlog"
	"github.com/animus-labs/animus-go/closed/internal/repo"
	"github.com/animus-labs/animus-go/closed/internal/repo/postgres"
	"github.com/animus-labs/animus-go/closed/internal/service/runs"
	"github.com/google/uuid"
)

type dpReconciler struct {
	logger     *slog.Logger
	db         *sql.DB
	dpBaseURL  string
	authSecret string
	interval   time.Duration
	staleAfter time.Duration
	batchLimit int
}

func startDPReconciler(ctx context.Context, logger *slog.Logger, db *sql.DB, dpBaseURL, authSecret string, interval, staleAfter time.Duration) {
	dpBaseURL = strings.TrimSpace(dpBaseURL)
	authSecret = strings.TrimSpace(authSecret)
	if dpBaseURL == "" || authSecret == "" || db == nil {
		if logger != nil {
			logger.Warn("dp reconciler disabled", "dp_base_url", dpBaseURL != "", "auth", authSecret != "")
		}
		return
	}
	if interval <= 0 {
		interval = 30 * time.Second
	}
	if staleAfter <= 0 {
		staleAfter = 2 * time.Minute
	}
	reconciler := &dpReconciler{
		logger:     logger,
		db:         db,
		dpBaseURL:  dpBaseURL,
		authSecret: authSecret,
		interval:   interval,
		staleAfter: staleAfter,
		batchLimit: 200,
	}

	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				reconciler.reconcileOnce(ctx)
			}
		}
	}()
}

func (r *dpReconciler) reconcileOnce(ctx context.Context) {
	dpStore := postgres.NewDPEventStore(r.db)
	if dpStore == nil {
		return
	}
	statuses := []string{dataplane.DispatchStatusRequested, dataplane.DispatchStatusAccepted, dataplane.DispatchStatusRunning}
	dispatches, err := dpStore.ListDispatchesByStatus(ctx, statuses, r.batchLimit)
	if err != nil {
		if r.logger != nil {
			r.logger.Warn("dp reconcile list failed", "error", err)
		}
		return
	}

	for _, dispatch := range dispatches {
		if ctx.Err() != nil {
			return
		}
		if !r.isHeartbeatStale(ctx, dpStore, dispatch.ProjectID, dispatch.RunID) {
			continue
		}
		if err := r.reconcileDispatch(ctx, dispatch); err != nil && r.logger != nil {
			r.logger.Warn("dp reconcile failed", "run_id", dispatch.RunID, "error", err)
		}
	}
}

func (r *dpReconciler) isHeartbeatStale(ctx context.Context, store *postgres.DPEventStore, projectID, runID string) bool {
	latest, err := store.LatestEventByType(ctx, projectID, runID, dataplane.EventTypeHeartbeat)
	if err != nil {
		return true
	}
	age := time.Since(latest.EmittedAt)
	return age >= r.staleAfter
}

func (r *dpReconciler) reconcileDispatch(ctx context.Context, dispatch postgres.RunDispatchRecord) error {
	dpURL := strings.TrimSpace(dispatch.DPBaseURL)
	if dpURL == "" {
		dpURL = r.dpBaseURL
	}
	client, err := newDataplaneClient(dpURL, r.authSecret)
	if err != nil {
		return err
	}
	status, code, err := client.GetRunStatus(ctx, dispatch.ProjectID, dispatch.RunID, "")
	if err != nil {
		if code == http.StatusNotFound {
			return r.applyReconciledState(ctx, dispatch, domain.RunStateFailed, "dp_not_found")
		}
		return err
	}

	nextState := mapStatusToRunState(status.State)
	if nextState == "" {
		return nil
	}
	return r.applyReconciledState(ctx, dispatch, nextState, status.Reason)
}

func (r *dpReconciler) applyReconciledState(ctx context.Context, dispatch postgres.RunDispatchRecord, nextState domain.RunState, reason string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	runStore := postgres.NewRunSpecStore(tx)
	dpStore := postgres.NewDPEventStore(tx)
	if runStore == nil || dpStore == nil {
		return errors.New("stores unavailable")
	}
	current, err := runStore.GetRun(ctx, dispatch.ProjectID, dispatch.RunID)
	if err != nil {
		return err
	}
	prev, applied, err := runStore.UpdateDerivedStatus(ctx, dispatch.ProjectID, dispatch.RunID, nextState)
	if err != nil && !errors.Is(err, repo.ErrInvalidTransition) {
		return err
	}
	if errors.Is(err, repo.ErrInvalidTransition) {
		return nil
	}

	if applied {
		appender := runs.NewAuditAppender(tx)
		if appender == nil {
			return errors.New("audit appender unavailable")
		}
		auditInfo := runs.AuditInfo{
			Actor:     "system:reconciler",
			RequestID: uuid.NewString(),
			Service:   "experiments",
		}
		if event, ok, err := runs.BuildRunTransitionEvent(auditInfo, dispatch.ProjectID, dispatch.RunID, current.SpecHash, prev, nextState); err == nil && ok {
			if err := appender.Append(ctx, *event); err != nil {
				return err
			}
		}
		recEvent := auditlog.Event{
			OccurredAt:   time.Now().UTC(),
			Actor:        "system:reconciler",
			Action:       "run.reconciled",
			ResourceType: "run",
			ResourceID:   dispatch.RunID,
			RequestID:    uuid.NewString(),
			Payload: map[string]any{
				"service":       "experiments",
				"project_id":    dispatch.ProjectID,
				"run_id":        dispatch.RunID,
				"spec_hash":     current.SpecHash,
				"dispatch_id":   dispatch.DispatchID,
				"from":          string(prev),
				"to":            string(nextState),
				"reason":        strings.TrimSpace(reason),
				"dp_base_url":   dispatch.DPBaseURL,
				"status":        dispatch.Status,
				"reconciled_at": time.Now().UTC(),
			},
		}
		if _, err := auditlog.Insert(ctx, tx, recEvent); err != nil {
			return err
		}
	}
	dispatchStatus := dispatchStatusFromRunState(nextState)
	_ = updateDispatchStatus(ctx, dpStore, dispatch.ProjectID, dispatch.RunID, dispatchStatus, reason)

	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

func mapStatusToRunState(state string) domain.RunState {
	switch strings.ToLower(strings.TrimSpace(state)) {
	case "running", "pending":
		return domain.RunStateRunning
	case "succeeded":
		return domain.RunStateSucceeded
	case "failed":
		return domain.RunStateFailed
	default:
		return ""
	}
}
