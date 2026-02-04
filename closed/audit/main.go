package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/animus-labs/animus-go/closed/internal/auditexport"
	"github.com/animus-labs/animus-go/closed/internal/platform/auditlog"
	"github.com/animus-labs/animus-go/closed/internal/platform/auth"
	"github.com/animus-labs/animus-go/closed/internal/platform/env"
	"github.com/animus-labs/animus-go/closed/internal/platform/httpserver"
	"github.com/animus-labs/animus-go/closed/internal/platform/postgres"
	"github.com/animus-labs/animus-go/closed/internal/platform/rbac"
	repopg "github.com/animus-labs/animus-go/closed/internal/repo/postgres"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	ctx := context.Background()
	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	addr := env.String("AUDIT_HTTP_ADDR", ":8085")
	shutdownTimeout, err := env.Duration("AUDIT_SHUTDOWN_TIMEOUT", 10*time.Second)
	if err != nil {
		logger.Error("invalid env", "error", err)
		os.Exit(2)
	}
	rbacAllowDirect, err := env.Bool("AUTH_RBAC_ALLOW_DIRECT_ROLES", true)
	if err != nil {
		logger.Error("invalid env", "error", err)
		os.Exit(2)
	}

	exportCfg, err := auditexport.ConfigFromEnv()
	if err != nil {
		logger.Error("invalid audit export config", "error", err)
		os.Exit(2)
	}

	dbCfg, err := postgres.ConfigFromEnv()
	if err != nil {
		logger.Error("invalid database config", "error", err)
		os.Exit(2)
	}
	db, err := postgres.Open(ctx, dbCfg)
	if err != nil {
		logger.Error("database unavailable", "error", err)
		os.Exit(1)
	}
	defer func() { _ = db.Close() }()

	internalAuthSecret := env.String("ANIMUS_INTERNAL_AUTH_SECRET", "")
	headersAuth, err := auth.NewGatewayHeadersAuthenticator(internalAuthSecret)
	if err != nil {
		logger.Error("invalid internal auth config", "error", err)
		os.Exit(2)
	}

	auditAppender := repopg.NewAuditAppender(db, nil)
	exportStore := repopg.NewAuditExportStore(db)
	if exportStore == nil {
		logger.Error("audit export store unavailable")
		os.Exit(1)
	}
	if sink, err := auditexport.DefaultSinkFromConfig(exportCfg, time.Now().UTC()); err == nil {
		if _, err := exportStore.UpsertSink(ctx, sink); err != nil {
			logger.Error("audit export sink upsert failed", "error", err)
			os.Exit(1)
		}
		if sink.Enabled {
			if err := exportStore.BackfillOutbox(ctx, sink.SinkID); err != nil {
				logger.Error("audit export backfill failed", "error", err)
				os.Exit(1)
			}
		}
	} else {
		logger.Error("audit export sink config failed", "error", err)
		os.Exit(2)
	}
	if exportCfg.Enabled() {
		worker := auditexport.NewWorker(exportStore, auditAppender, logger, exportCfg)
		go worker.Run(ctx)
	}
	authorizer := rbac.Authorizer{
		Audit:       auditAppender,
		AllowDirect: rbacAllowDirect,
		RequiredRoleFor: func(r *http.Request) string {
			return auth.RoleAdmin
		},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", httpserver.Healthz("audit"))
	mux.HandleFunc(
		"/readyz",
		httpserver.ReadyzWithChecks(
			"audit",
			httpserver.ReadinessCheck{
				Name: "postgres",
				Check: func(ctx context.Context) error {
					checkCtx, cancel := context.WithTimeout(ctx, 750*time.Millisecond)
					defer cancel()
					return db.PingContext(checkCtx)
				},
			},
		),
	)
	httpserver.RegisterMetrics(mux, "audit")

	api := newAuditAPI(logger, db, exportCfg)
	api.register(mux)

	handler := auth.Middleware{
		Logger:        logger,
		Authenticator: headersAuth,
		Authorize:     authorizer.Authorize,
		Audit: func(ctx context.Context, event auth.DenyEvent) error {
			auditCtx, cancel := context.WithTimeout(ctx, 750*time.Millisecond)
			defer cancel()
			return auditlog.InsertAuthDeny(auditCtx, db, "audit", event)
		},
		SkipPrefixes: []string{"/healthz", "/readyz"},
	}.Wrap(mux)

	cfg := httpserver.Config{
		Service:         "audit",
		Addr:            addr,
		ShutdownTimeout: shutdownTimeout,
	}

	if err := httpserver.Run(ctx, logger, cfg, httpserver.Wrap(logger, "audit", handler)); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("server failed", "error", err)
		os.Exit(1)
	}
}
