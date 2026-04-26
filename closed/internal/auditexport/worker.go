package auditexport

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/animus-labs/animus-go/closed/internal/domain"
	"github.com/animus-labs/animus-go/closed/internal/platform/secrets"
	"github.com/animus-labs/animus-go/closed/internal/repo"
)

type Logger interface {
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
}

type Worker struct {
	deliveries DeliveryStore
	attempts   AttemptStore
	sinks      SinkStore
	events     EventStore
	audit      repo.AuditEventAppender
	logger     Logger
	cfg        Config
	now        func() time.Time
	webhook    WebhookConnector
	syslog     SyslogConnector
}

type WorkerDeps struct {
	HTTPClient   HTTPDoer
	SyslogDialer SyslogDialer
	Secrets      secrets.Manager
}

func NewWorker(deliveries DeliveryStore, attempts AttemptStore, sinks SinkStore, events EventStore, audit repo.AuditEventAppender, logger Logger, cfg Config, deps WorkerDeps) *Worker {
	now := time.Now
	httpTimeout := cfg.HTTPTimeout
	if httpTimeout <= 0 {
		httpTimeout = 10 * time.Second
	}
	client := deps.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: httpTimeout}
	}
	worker := &Worker{
		deliveries: deliveries,
		attempts:   attempts,
		sinks:      sinks,
		events:     events,
		audit:      audit,
		logger:     logger,
		cfg:        cfg,
		now:        now,
		webhook: WebhookConnector{
			Client:     client,
			Secrets:    deps.Secrets,
			SigningKey: strings.TrimSpace(cfg.SigningSecretKey),
		},
		syslog: SyslogConnector{
			Dialer: deps.SyslogDialer,
			Now:    now,
		},
	}
	return worker
}

func (w *Worker) Run(ctx context.Context) {
	if w == nil || w.deliveries == nil || w.sinks == nil || w.events == nil {
		return
	}
	if !w.cfg.Enabled() {
		return
	}
	if w.cfg.PollInterval <= 0 {
		w.cfg.PollInterval = 5 * time.Second
	}
	ticker := time.NewTicker(w.cfg.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.processOnce(ctx)
		}
	}
}

func (w *Worker) processOnce(ctx context.Context) {
	now := w.now().UTC()
	inflight := w.cfg.InflightTimeout
	if inflight <= 0 {
		inflight = 2 * time.Minute
	}
	limit := w.cfg.BatchSize
	if limit <= 0 {
		limit = 50
	}
	batch, err := w.deliveries.ClaimDue(ctx, now, inflight, limit)
	if err != nil {
		w.logWarn("audit export claim failed", "error", err)
		return
	}
	if len(batch) == 0 {
		return
	}
	workers := w.cfg.WorkerConcurrency
	if workers <= 0 {
		workers = 1
	}
	if workers == 1 || len(batch) == 1 {
		for _, job := range batch {
			w.handleDelivery(ctx, job)
		}
		return
	}

	jobs := make(chan Delivery)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				w.handleDelivery(ctx, job)
			}
		}()
	}
	for _, job := range batch {
		jobs <- job
	}
	close(jobs)
	wg.Wait()
}

func (w *Worker) handleDelivery(ctx context.Context, job Delivery) {
	if w == nil {
		return
	}
	now := w.now().UTC()
	sink, err := w.sinks.GetSink(ctx, job.SinkID)
	if err != nil {
		w.handlePermanentFailure(ctx, job, now, "sink_unavailable", "sink_unavailable")
		return
	}
	if !sink.Enabled {
		w.handlePermanentFailure(ctx, job, now, "sink_disabled", "sink_disabled")
		return
	}
	event, err := w.events.GetEvent(ctx, job.EventID)
	if err != nil {
		w.handleRetry(ctx, job, now, "event_unavailable")
		return
	}
	cfg, err := DecodeSinkConfig(sink)
	if err != nil {
		w.handlePermanentFailure(ctx, job, now, "sink_config_invalid", "sink_config_invalid")
		return
	}
	payload, err := EncodeEvent(event)
	if err != nil {
		w.handlePermanentFailure(ctx, job, now, "payload_encode_failed", "payload_encode_failed")
		return
	}

	start := time.Now()
	result := w.dispatch(ctx, sink, event, cfg, payload)
	if result.Latency == 0 {
		result.Latency = time.Since(start)
	}
	w.recordAttempt(ctx, sink, job, result, now)

	switch result.Outcome {
	case AttemptOutcomeSuccess:
		if err := w.deliveries.MarkDelivered(ctx, job.DeliveryID, now); err != nil {
			w.logWarn("audit export mark delivered failed", "delivery_id", job.DeliveryID, "error", err)
			return
		}
		w.auditDelivered(ctx, sink, event, job, result)
	case AttemptOutcomeRetry:
		if w.reachedMaxAttempts(job.AttemptCount) {
			w.handlePermanentFailure(ctx, job, now, "max_attempts_exceeded", result.Error)
			return
		}
		w.handleRetry(ctx, job, now, result.Error)
	case AttemptOutcomePermanentFailure:
		w.handlePermanentFailure(ctx, job, now, "permanent_failure", result.Error)
	default:
		w.handlePermanentFailure(ctx, job, now, "unknown_outcome", result.Error)
	}
}

func (w *Worker) dispatch(ctx context.Context, sink Sink, event domain.AuditEvent, cfg SinkConfig, payload []byte) DeliveryResult {
	dest := normalizeDestination(sink.Destination)
	switch dest {
	case "webhook":
		return w.webhook.Deliver(ctx, sink.SinkID, event.EventID, cfg, payload)
	case "syslog", "syslog_tcp", "syslog_udp":
		if dest == "syslog_tcp" {
			cfg.SyslogProtocol = "tcp"
		} else if dest == "syslog_udp" {
			cfg.SyslogProtocol = "udp"
		}
		return w.syslog.Deliver(ctx, cfg, payload)
	default:
		return DeliveryResult{Outcome: AttemptOutcomePermanentFailure, Error: "destination_invalid"}
	}
}

func (w *Worker) handleRetry(ctx context.Context, job Delivery, now time.Time, errMsg string) {
	delay := backoffDelay(job.AttemptCount, w.cfg.RetryBaseDelay, w.cfg.RetryMaxDelay)
	nextAttempt := now.Add(delay)
	if markErr := w.deliveries.MarkRetry(ctx, job.DeliveryID, sanitizeError(errMsg), nextAttempt); markErr != nil {
		w.logWarn("audit export retry mark failed", "delivery_id", job.DeliveryID, "error", markErr)
	}
}

func (w *Worker) handlePermanentFailure(ctx context.Context, job Delivery, now time.Time, reason string, errMsg string) {
	if markErr := w.deliveries.MarkDLQ(ctx, job.DeliveryID, sanitizeError(reason), sanitizeError(errMsg), now); markErr != nil {
		w.logWarn("audit export dlq mark failed", "delivery_id", job.DeliveryID, "error", markErr)
	}
	w.auditDLQ(ctx, job, reason, errMsg)
}

func (w *Worker) reachedMaxAttempts(attemptCount int) bool {
	maxAttempts := w.cfg.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 10
	}
	return attemptCount >= maxAttempts
}

func (w *Worker) recordAttempt(ctx context.Context, sink Sink, job Delivery, result DeliveryResult, attemptedAt time.Time) {
	if w == nil {
		return
	}
	recordAttemptMetrics(sink.Destination, result.Outcome, result.Latency)
	if w.attempts == nil {
		return
	}
	attempt := DeliveryAttempt{
		DeliveryID:  job.DeliveryID,
		AttemptedAt: attemptedAt,
		Outcome:     result.Outcome,
		StatusCode:  result.StatusCode,
		Error:       sanitizeError(result.Error),
		LatencyMs:   int(result.Latency / time.Millisecond),
		CreatedAt:   attemptedAt,
	}
	if _, err := w.attempts.Insert(ctx, attempt); err != nil {
		w.logWarn("audit export attempt insert failed", "delivery_id", job.DeliveryID, "error", err)
	}
	w.auditAttempt(ctx, sink, job, result)
}

func (w *Worker) auditAttempt(ctx context.Context, sink Sink, job Delivery, result DeliveryResult) {
	if w == nil || w.audit == nil {
		return
	}
	payload := domain.Metadata{
		"event_id":    job.EventID,
		"sink_id":     sink.SinkID,
		"destination": sink.Destination,
		"format":      sink.Format,
		"attempt":     job.AttemptCount,
		"delivery_id": job.DeliveryID,
		"outcome":     result.Outcome,
	}
	if result.StatusCode != nil {
		payload["status_code"] = *result.StatusCode
	}
	if result.Error != "" {
		payload["error"] = sanitizeError(result.Error)
	}
	_, _ = w.audit.Append(ctx, domain.AuditEvent{
		OccurredAt:   w.now().UTC(),
		Actor:        "system:audit-exporter",
		Action:       "audit.export.attempted",
		ResourceType: "audit_export",
		ResourceID:   fmt.Sprintf("%s:%d", sink.SinkID, job.EventID),
		Payload:      payload,
	})
}

func (w *Worker) auditDelivered(ctx context.Context, sink Sink, event domain.AuditEvent, job Delivery, result DeliveryResult) {
	if w == nil || w.audit == nil {
		return
	}
	payload := domain.Metadata{
		"event_id":    event.EventID,
		"sink_id":     sink.SinkID,
		"destination": sink.Destination,
		"format":      sink.Format,
		"attempt":     job.AttemptCount,
		"delivery_id": job.DeliveryID,
	}
	_, _ = w.audit.Append(ctx, domain.AuditEvent{
		OccurredAt:   w.now().UTC(),
		Actor:        "system:audit-exporter",
		Action:       "audit.export.delivered",
		ResourceType: "audit_export",
		ResourceID:   fmt.Sprintf("%s:%d", sink.SinkID, event.EventID),
		Payload:      payload,
	})
}

func (w *Worker) auditDLQ(ctx context.Context, job Delivery, reason string, errMsg string) {
	if w == nil || w.audit == nil {
		return
	}
	payload := domain.Metadata{
		"event_id":    job.EventID,
		"delivery_id": job.DeliveryID,
		"reason":      sanitizeError(reason),
	}
	if errMsg != "" {
		payload["error"] = sanitizeError(errMsg)
	}
	_, _ = w.audit.Append(ctx, domain.AuditEvent{
		OccurredAt:   w.now().UTC(),
		Actor:        "system:audit-exporter",
		Action:       "audit.export.dlq",
		ResourceType: "audit_export",
		ResourceID:   fmt.Sprintf("delivery:%d", job.DeliveryID),
		Payload:      payload,
	})
}

func (w *Worker) logWarn(msg string, args ...any) {
	if w != nil && w.logger != nil {
		w.logger.Warn(msg, args...)
	}
}

func backoffDelay(attempt int, base, max time.Duration) time.Duration {
	if base <= 0 {
		base = 5 * time.Second
	}
	if max <= 0 {
		max = 5 * time.Minute
	}
	if attempt <= 1 {
		if base > max {
			return max
		}
		return base
	}
	delay := base
	for i := 1; i < attempt; i++ {
		if delay >= max/2 {
			delay = max
			break
		}
		delay *= 2
	}
	if delay > max {
		delay = max
	}
	return delay
}

func sanitizeError(errMsg string) string {
	msg := strings.TrimSpace(errMsg)
	if len(msg) > 500 {
		return msg[:500]
	}
	return msg
}
