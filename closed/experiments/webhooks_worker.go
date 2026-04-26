package main

import (
	"context"
	"log/slog"

	"github.com/animus-labs/animus-go/closed/internal/integrations/webhooks"
)

func startWebhookDispatcher(ctx context.Context, logger *slog.Logger, worker *webhooks.Worker) {
	if worker == nil {
		return
	}
	go worker.Run(ctx)
	if logger != nil {
		logger.Info("webhook dispatcher started")
	}
}
