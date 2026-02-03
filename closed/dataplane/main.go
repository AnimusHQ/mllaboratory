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

	"github.com/animus-labs/animus-go/closed/internal/platform/auth"
	"github.com/animus-labs/animus-go/closed/internal/platform/env"
	"github.com/animus-labs/animus-go/closed/internal/platform/httpserver"
	"github.com/animus-labs/animus-go/closed/internal/platform/k8s"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	ctx := context.Background()
	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	addr := env.String("DATAPLANE_HTTP_ADDR", ":8086")
	shutdownTimeout, err := env.Duration("DATAPLANE_SHUTDOWN_TIMEOUT", 10*time.Second)
	if err != nil {
		logger.Error("invalid env", "error", err)
		os.Exit(2)
	}

	internalAuthSecret := env.String("ANIMUS_INTERNAL_AUTH_SECRET", "")
	headersAuth, err := auth.NewGatewayHeadersAuthenticator(internalAuthSecret)
	if err != nil {
		logger.Error("invalid internal auth config", "error", err)
		os.Exit(2)
	}

	cpBaseURL := env.String("ANIMUS_CONTROL_PLANE_URL", "")
	if cpBaseURL == "" {
		logger.Error("missing control plane url", "env", "ANIMUS_CONTROL_PLANE_URL")
		os.Exit(2)
	}

	client, err := k8s.NewInClusterClient()
	if err != nil {
		logger.Error("k8s client init failed", "error", err)
		os.Exit(2)
	}
	namespace := env.String("ANIMUS_DATAPLANE_K8S_NAMESPACE", "")
	jobTTLSeconds, err := env.Int("ANIMUS_DATAPLANE_JOB_TTL_SECONDS", 3600)
	if err != nil {
		logger.Error("invalid job ttl seconds", "error", err)
		os.Exit(2)
	}
	jobServiceAccount := env.String("ANIMUS_DATAPLANE_JOB_SERVICE_ACCOUNT", "")
	heartbeatInterval, err := env.Duration("ANIMUS_DATAPLANE_HEARTBEAT_INTERVAL", 15*time.Second)
	if err != nil {
		logger.Error("invalid heartbeat interval", "error", err)
		os.Exit(2)
	}
	pollInterval, err := env.Duration("ANIMUS_DATAPLANE_STATUS_POLL_INTERVAL", 10*time.Second)
	if err != nil {
		logger.Error("invalid status poll interval", "error", err)
		os.Exit(2)
	}

	cpClient, err := newControlPlaneClient(cpBaseURL, internalAuthSecret)
	if err != nil {
		logger.Error("control plane client init failed", "error", err)
		os.Exit(2)
	}

	api := newDataplaneAPI(logger, cpClient, client, dataplaneConfig{
		Namespace:         namespace,
		JobTTLSeconds:     int32(jobTTLSeconds),
		JobServiceAccount: jobServiceAccount,
		HeartbeatInterval: heartbeatInterval,
		PollInterval:      pollInterval,
	})

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", httpserver.Healthz("dataplane"))
	mux.HandleFunc("/readyz", httpserver.Readyz("dataplane"))
	api.register(mux)

	authorizer := auth.MethodRoleAuthorizer()

	handler := auth.Middleware{
		Logger:        logger,
		Authenticator: headersAuth,
		Authorize:     authorizer,
		SkipPrefixes:  []string{"/healthz", "/readyz"},
	}.Wrap(mux)

	cfg := httpserver.Config{
		Service:         "dataplane",
		Addr:            addr,
		ShutdownTimeout: shutdownTimeout,
	}

	if err := httpserver.Run(ctx, logger, cfg, httpserver.Wrap(logger, "dataplane", handler)); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("server failed", "error", err)
		os.Exit(1)
	}
}
