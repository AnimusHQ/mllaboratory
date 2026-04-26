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

	"github.com/animus-labs/animus-go/closed/internal/platform/auditlog"
	"github.com/animus-labs/animus-go/closed/internal/platform/auth"
	"github.com/animus-labs/animus-go/closed/internal/platform/env"
	"github.com/animus-labs/animus-go/closed/internal/platform/httpserver"
	"github.com/animus-labs/animus-go/closed/internal/platform/objectstore"
	"github.com/animus-labs/animus-go/closed/internal/platform/postgres"
	"github.com/animus-labs/animus-go/closed/internal/platform/rbac"
	repopg "github.com/animus-labs/animus-go/closed/internal/repo/postgres"
	artifactsvc "github.com/animus-labs/animus-go/closed/internal/service/artifacts"
	storageobjectstore "github.com/animus-labs/animus-go/closed/internal/storage/objectstore"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	ctx := context.Background()
	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	addr := env.String("DATASET_REGISTRY_HTTP_ADDR", ":8081")
	shutdownTimeout, err := env.Duration("DATASET_REGISTRY_SHUTDOWN_TIMEOUT", 10*time.Second)
	if err != nil {
		logger.Error("invalid env", "error", err)
		os.Exit(2)
	}

	rbacAllowDirect, err := env.Bool("AUTH_RBAC_ALLOW_DIRECT_ROLES", true)
	if err != nil {
		logger.Error("invalid env", "error", err)
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

	roleBindings := repopg.NewRoleBindingStore(db)
	authAuditAppender := repopg.NewAuditAppender(db, nil)
	authorizer := rbac.Authorizer{
		Store:       roleBindings,
		Audit:       authAuditAppender,
		AllowDirect: rbacAllowDirect,
		RequiredRoleFor: func(r *http.Request) string {
			return requiredRoleForDatasetRegistry(r)
		},
	}
	authorize := func(r *http.Request, identity auth.Identity) error {
		if r != nil && r.URL.Path == "/projects" && (r.Method == http.MethodPost || r.Method == http.MethodGet) {
			clone := r.Clone(r.Context())
			clone.Header = clone.Header.Clone()
			clone.Header.Del("X-Project-Id")
			clone.Header.Del("X-Project-ID")
			r = clone
		}
		return authorizer.Authorize(r, identity)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", httpserver.Healthz("dataset-registry"))
	mux.HandleFunc(
		"/readyz",
		httpserver.ReadyzWithChecks(
			"dataset-registry",
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
	httpserver.RegisterMetrics(mux, "dataset-registry")

	uploadMaxMiB, err := env.Int("DATASET_REGISTRY_UPLOAD_MAX_MIB", 2048)
	if err != nil {
		logger.Error("invalid env", "error", err)
		os.Exit(2)
	}
	uploadTimeout, err := env.Duration("DATASET_REGISTRY_UPLOAD_TIMEOUT", 30*time.Minute)
	if err != nil {
		logger.Error("invalid env", "error", err)
		os.Exit(2)
	}

	artifactPresignTTL, err := env.Duration("DATASET_REGISTRY_ARTIFACT_PRESIGN_TTL", 10*time.Minute)
	if err != nil {
		logger.Error("invalid env", "error", err)
		os.Exit(2)
	}

	projectStore := repopg.NewProjectStore(db)
	datasetStore := repopg.NewDatasetStore(db)
	artifactStore := repopg.NewArtifactStore(db)
	auditAppender := repopg.NewAuditAppender(db, nil)

	service := newDatasetService(projectStore, datasetStore, auditAppender)
	artifactObjectStore, err := storageobjectstore.NewMinioStoreWithClient(storeClient)
	if err != nil {
		logger.Error("artifact object store init failed", "error", err)
		os.Exit(2)
	}
	artifactService, err := artifactsvc.NewService(artifactStore, artifactObjectStore, storeCfg.BucketArtifacts, artifactPresignTTL, auditAppender)
	if err != nil {
		logger.Error("artifact service init failed", "error", err)
		os.Exit(2)
	}

	api := newDatasetRegistryAPI(logger, db, storeClient, storeCfg, int64(uploadMaxMiB)<<20, uploadTimeout, service, artifactService)
	api.register(mux)

	projectResolver := func(r *http.Request, identity auth.Identity) (string, error) {
		if r.Method == http.MethodPost && r.URL.Path == "/projects" {
			return "", nil
		}
		if r.Method == http.MethodGet && r.URL.Path == "/projects" {
			return "", nil
		}
		return auth.RequireProjectIDResolver([]string{"/healthz", "/readyz"})(r, identity)
	}

	handler := auth.Middleware{
		Logger:         logger,
		Authenticator:  headersAuth,
		Authorize:      authorize,
		ProjectResolve: projectResolver,
		Audit: func(ctx context.Context, event auth.DenyEvent) error {
			auditCtx, cancel := context.WithTimeout(ctx, 750*time.Millisecond)
			defer cancel()
			return auditlog.InsertAuthDeny(auditCtx, db, "dataset-registry", event)
		},
		SkipPrefixes: []string{"/healthz", "/readyz"},
	}.Wrap(mux)

	cfg := httpserver.Config{
		Service:         "dataset-registry",
		Addr:            addr,
		ShutdownTimeout: shutdownTimeout,
	}

	if err := httpserver.Run(ctx, logger, cfg, httpserver.Wrap(logger, "dataset-registry", handler)); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("server failed", "error", err)
		os.Exit(1)
	}
}
