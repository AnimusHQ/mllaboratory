package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/animus-labs/animus-go/closed/internal/integrations/registryverify"
	"github.com/animus-labs/animus-go/closed/internal/integrations/webhooks"
	"github.com/animus-labs/animus-go/closed/internal/platform/auditlog"
	"github.com/animus-labs/animus-go/closed/internal/platform/auth"
	"github.com/animus-labs/animus-go/closed/internal/platform/env"
	"github.com/animus-labs/animus-go/closed/internal/platform/httpserver"
	"github.com/animus-labs/animus-go/closed/internal/platform/objectstore"
	"github.com/animus-labs/animus-go/closed/internal/platform/postgres"
	"github.com/animus-labs/animus-go/closed/internal/platform/rbac"
	"github.com/animus-labs/animus-go/closed/internal/platform/secrets"
	repopg "github.com/animus-labs/animus-go/closed/internal/repo/postgres"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	ctx := context.Background()
	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	addr := env.String("EXPERIMENTS_HTTP_ADDR", ":8083")
	shutdownTimeout, err := env.Duration("EXPERIMENTS_SHUTDOWN_TIMEOUT", 10*time.Second)
	if err != nil {
		logger.Error("invalid env", "error", err)
		os.Exit(2)
	}

	rbacAllowDirect, err := env.Bool("AUTH_RBAC_ALLOW_DIRECT_ROLES", true)
	if err != nil {
		logger.Error("invalid env", "error", err)
		os.Exit(2)
	}
	profile := strings.ToLower(strings.TrimSpace(env.String("ANIMUS_PROFILE", "dev")))
	if profile == "" {
		profile = "dev"
	}
	if profile != "dev" && profile != "production" {
		logger.Error("invalid profile", "profile", profile)
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

	storeCfg, err := objectstore.ConfigFromEnv()
	if err != nil {
		logger.Error("invalid object store config", "error", err)
		os.Exit(2)
	}
	storeClient, err := objectstore.NewMinIOClient(storeCfg)
	if err != nil {
		logger.Error("object store client init failed", "error", err)
		os.Exit(2)
	}
	startupCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	if err := objectstore.EnsureBuckets(startupCtx, storeClient, storeCfg); err != nil {
		cancel()
		logger.Error("object store unavailable", "error", err)
		os.Exit(1)
	}
	cancel()

	internalAuthSecret := env.String("ANIMUS_INTERNAL_AUTH_SECRET", "")
	headersAuth, err := auth.NewGatewayHeadersAuthenticator(internalAuthSecret)
	if err != nil {
		logger.Error("invalid internal auth config", "error", err)
		os.Exit(2)
	}

	ciWebhookSecret := env.String("ANIMUS_CI_WEBHOOK_SECRET", "")
	if ciWebhookSecret == "" {
		logger.Error("missing CI webhook secret", "env", "ANIMUS_CI_WEBHOOK_SECRET")
		os.Exit(2)
	}

	evidenceSigningSecret := strings.TrimSpace(env.String("ANIMUS_EVIDENCE_SIGNING_SECRET", ""))
	if evidenceSigningSecret == "" {
		evidenceSigningSecret = strings.TrimSpace(internalAuthSecret)
	}
	if evidenceSigningSecret == "" {
		logger.Error("missing evidence signing secret", "env", "ANIMUS_EVIDENCE_SIGNING_SECRET")
		os.Exit(2)
	}

	gitlabWebhookSecret := strings.TrimSpace(env.String("ANIMUS_GITLAB_WEBHOOK_SECRET", ""))

	webhookCfg, err := webhooks.ConfigFromEnv()
	if err != nil {
		logger.Error("invalid webhook config", "error", err)
		os.Exit(2)
	}

	secretsCfg, err := secrets.ConfigFromEnv()
	if err != nil {
		logger.Error("invalid secrets config", "error", err)
		os.Exit(2)
	}
	secretsManager, err := secrets.NewManager(secretsCfg)
	if err != nil {
		logger.Error("invalid secrets manager", "error", err)
		os.Exit(2)
	}

	runTokenTTL, err := env.Duration("ANIMUS_RUN_TOKEN_TTL", 12*time.Hour)
	if err != nil {
		logger.Error("invalid run token ttl", "error", err)
		os.Exit(2)
	}
	devEnvDefaultTTL, err := env.Duration("ANIMUS_DEVENV_TTL", 2*time.Hour)
	if err != nil {
		logger.Error("invalid devenv ttl", "error", err)
		os.Exit(2)
	}
	devEnvAccessTTL, err := env.Duration("ANIMUS_DEVENV_ACCESS_TTL", 15*time.Minute)
	if err != nil {
		logger.Error("invalid devenv access ttl", "error", err)
		os.Exit(2)
	}
	devEnvAccessAuditInterval, err := env.Duration("ANIMUS_DEVENV_ACCESS_AUDIT_INTERVAL", time.Minute)
	if err != nil {
		logger.Error("invalid devenv access audit interval", "error", err)
		os.Exit(2)
	}
	devEnvReconcileInterval, err := env.Duration("ANIMUS_DEVENV_RECONCILE_INTERVAL", 30*time.Second)
	if err != nil {
		logger.Error("invalid devenv reconcile interval", "error", err)
		os.Exit(2)
	}
	devEnvRepoAllowlist, err := parseRepoAllowlist(env.String("ANIMUS_DEVENV_REPO_ALLOWLIST", ""))
	if err != nil {
		logger.Error("invalid devenv repo allowlist", "error", err)
		os.Exit(2)
	}
	devEnvServiceDomain := env.String("ANIMUS_DEVENV_SERVICE_DOMAIN", "svc.cluster.local")
	devEnvCodeServerPort, err := env.Int("ANIMUS_DEVENV_CODE_SERVER_PORT", 8080)
	if err != nil {
		logger.Error("invalid devenv code server port", "error", err)
		os.Exit(2)
	}

	roleBindings := repopg.NewRoleBindingStore(db)
	auditAppender := repopg.NewAuditAppender(db, nil)
	authorizer := rbac.Authorizer{
		Store:           roleBindings,
		Audit:           auditAppender,
		AllowDirect:     rbacAllowDirect,
		RequiredRoleFor: experimentsRequiredRole,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", httpserver.Healthz("experiments"))
	mux.HandleFunc(
		"/readyz",
		httpserver.ReadyzWithChecks(
			"experiments",
			httpserver.ReadinessCheck{
				Name: "postgres",
				Check: func(ctx context.Context) error {
					checkCtx, cancel := context.WithTimeout(ctx, 750*time.Millisecond)
					defer cancel()
					return db.PingContext(checkCtx)
				},
			},
			httpserver.ReadinessCheck{
				Name: "minio",
				Check: func(ctx context.Context) error {
					checkCtx, cancel := context.WithTimeout(ctx, 750*time.Millisecond)
					defer cancel()
					return objectstore.CheckBuckets(checkCtx, storeClient, storeCfg)
				},
			},
		),
	)
	httpserver.RegisterMetricsProvider(webhooks.PrometheusMetrics)
	httpserver.RegisterMetrics(mux, "experiments")

	datapilotURL := env.String("ANIMUS_DATAPILOT_URL", "http://localhost:8080")
	dataplaneURL := env.String("ANIMUS_DATAPLANE_URL", "")
	dpReconcileInterval, err := env.Duration("ANIMUS_DP_RECONCILE_INTERVAL", 30*time.Second)
	if err != nil {
		logger.Error("invalid dp reconcile interval", "error", err)
		os.Exit(2)
	}
	dpHeartbeatStaleAfter, err := env.Duration("ANIMUS_DP_HEARTBEAT_STALE_AFTER", 2*time.Minute)
	if err != nil {
		logger.Error("invalid dp heartbeat stale after", "error", err)
		os.Exit(2)
	}
	registryCfg, err := registryverify.ConfigFromEnv()
	if err != nil {
		logger.Error("invalid registry config", "error", err)
		os.Exit(2)
	}
	registryPolicyResolver := registryverify.PolicyResolver{
		Default: registryCfg.DefaultPolicy(),
		Store:   repopg.NewRegistryPolicyStore(db),
	}
	registryProviders := map[string]registryverify.Provider{
		registryverify.ProviderNoop:       registryverify.NoopProvider{},
		registryverify.ProviderCosignStub: registryverify.CosignStubProvider{},
	}

	api := newExperimentsAPI(
		logger,
		db,
		storeClient,
		storeCfg,
		ciWebhookSecret,
		internalAuthSecret,
		runTokenTTL,
		datapilotURL,
		dataplaneURL,
		evidenceSigningSecret,
		gitlabWebhookSecret,
		registryPolicyResolver,
		registryCfg.VerifyTimeout,
		registryProviders,
		webhookCfg,
		devEnvDefaultTTL,
		devEnvAccessTTL,
		devEnvAccessAuditInterval,
		devEnvRepoAllowlist,
		devEnvServiceDomain,
		devEnvCodeServerPort,
	)
	api.register(mux)

	startDPReconciler(ctx, logger, db, dataplaneURL, internalAuthSecret, dpReconcileInterval, dpHeartbeatStaleAfter)
	startDevEnvReconciler(ctx, logger, db, dataplaneURL, internalAuthSecret, devEnvReconcileInterval)
	webhookWorker := webhooks.NewWorker(
		repopg.NewWebhookSubscriptionStore(db),
		repopg.NewWebhookDeliveryStore(db),
		repopg.NewWebhookDeliveryAttemptStore(db),
		secretsManager,
		auditAppender,
		logger,
		webhookCfg,
	)
	startWebhookDispatcher(ctx, logger, webhookWorker)

	handler := auth.Middleware{
		Logger:         logger,
		Authenticator:  headersAuth,
		Authorize:      authorizer.Authorize,
		ProjectResolve: experimentsProjectResolver(db),
		Audit: func(ctx context.Context, event auth.DenyEvent) error {
			auditCtx, cancel := context.WithTimeout(ctx, 750*time.Millisecond)
			defer cancel()
			return auditlog.InsertAuthDeny(auditCtx, db, "experiments", event)
		},
		SkipPrefixes: []string{"/healthz", "/readyz"},
	}.Wrap(mux)

	cfg := httpserver.Config{
		Service:         "experiments",
		Addr:            addr,
		ShutdownTimeout: shutdownTimeout,
	}

	if err := httpserver.Run(ctx, logger, cfg, httpserver.Wrap(logger, "experiments", handler)); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("server failed", "error", err)
		os.Exit(1)
	}
}
