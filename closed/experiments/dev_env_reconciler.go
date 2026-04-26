package main

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/animus-labs/animus-go/closed/internal/dataplane"
	"github.com/animus-labs/animus-go/closed/internal/domain"
	"github.com/animus-labs/animus-go/closed/internal/platform/auditlog"
	"github.com/animus-labs/animus-go/closed/internal/repo"
	"github.com/animus-labs/animus-go/closed/internal/repo/postgres"
)

type devEnvReconciler struct {
	logger     *slog.Logger
	db         *sql.DB
	dpBaseURL  string
	authSecret string
	interval   time.Duration
	batchLimit int
}

func startDevEnvReconciler(ctx context.Context, logger *slog.Logger, db *sql.DB, dpBaseURL, authSecret string, interval time.Duration) {
	dpBaseURL = strings.TrimSpace(dpBaseURL)
	authSecret = strings.TrimSpace(authSecret)
	if dpBaseURL == "" || authSecret == "" || db == nil {
		if logger != nil {
			logger.Warn("devenv reconciler disabled", "dp_base_url", dpBaseURL != "", "auth", authSecret != "")
		}
		return
	}
	if interval <= 0 {
		interval = 30 * time.Second
	}
	reconciler := &devEnvReconciler{
		logger:     logger,
		db:         db,
		dpBaseURL:  dpBaseURL,
		authSecret: authSecret,
		interval:   interval,
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

func (r *devEnvReconciler) reconcileOnce(ctx context.Context) {
	projectStore := postgres.NewProjectStore(r.db)
	devEnvStore := postgres.NewDevEnvironmentStore(r.db)
	if projectStore == nil || devEnvStore == nil {
		return
	}
	client, err := newDataplaneClient(r.dpBaseURL, r.authSecret)
	if err != nil {
		return
	}
	audit := auditlogAppender{db: r.db}

	projects, err := projectStore.List(ctx, repo.ProjectFilter{Limit: r.batchLimit})
	if err != nil {
		if r.logger != nil {
			r.logger.Warn("devenv reconcile projects failed", "error", err)
		}
		return
	}

	now := time.Now().UTC()
	for _, project := range projects {
		if ctx.Err() != nil {
			return
		}
		if err := expireDevEnvironments(ctx, devEnvStore, client, audit, project.ID, now, r.batchLimit); err != nil && r.logger != nil {
			r.logger.Warn("devenv reconcile failed", "project_id", project.ID, "error", err)
		}
	}
}

type auditlogAppender struct {
	db auditlog.QueryRower
}

func (a auditlogAppender) Append(ctx context.Context, event auditlog.Event) error {
	_, err := auditlog.Insert(ctx, a.db, event)
	return err
}

func expireDevEnvironments(ctx context.Context, store devEnvironmentStore, client devEnvDataplaneClient, audit devEnvAuditAppender, projectID string, now time.Time, limit int) error {
	if store == nil {
		return errors.New("store required")
	}
	if strings.TrimSpace(projectID) == "" {
		return errors.New("project id required")
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if limit <= 0 {
		limit = 200
	}

	records, err := store.ListExpired(ctx, projectID, now, limit)
	if err != nil {
		return err
	}
	for _, record := range records {
		env := record.Environment
		if env.ID == "" {
			continue
		}

		_, _ = store.UpdateState(ctx, projectID, env.ID, domain.DevEnvStateExpired, env.DPJobName, env.DPNamespace)
		if audit != nil {
			_ = audit.Append(ctx, auditlog.Event{
				OccurredAt:   now,
				Actor:        "system:devenv-reconciler",
				Action:       auditDevEnvExpired,
				ResourceType: "dev_environment",
				ResourceID:   env.ID,
				Payload: map[string]any{
					"service":    "experiments",
					"project_id": projectID,
					"expires_at": env.ExpiresAt.UTC().Format(time.RFC3339),
				},
			})
		}

		if client != nil {
			_, _, err = client.DeleteDevEnv(ctx, dataplane.DevEnvDeleteRequest{
				DevEnvID:      env.ID,
				ProjectID:     projectID,
				EmittedAt:     now,
				RequestedBy:   "system:devenv-reconciler",
				CorrelationID: "",
			}, "")
			if err != nil {
				continue
			}
		}

		_, _ = store.UpdateState(ctx, projectID, env.ID, domain.DevEnvStateDeleted, env.DPJobName, env.DPNamespace)
		if audit != nil {
			_ = audit.Append(ctx, auditlog.Event{
				OccurredAt:   now,
				Actor:        "system:devenv-reconciler",
				Action:       auditDevEnvDeleted,
				ResourceType: "dev_environment",
				ResourceID:   env.ID,
				Payload: map[string]any{
					"service":    "experiments",
					"project_id": projectID,
				},
			})
		}
	}
	return nil
}
