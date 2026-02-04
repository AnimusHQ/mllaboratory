package webhooks

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/animus-labs/animus-go/closed/internal/domain"
	"github.com/animus-labs/animus-go/closed/internal/platform/redaction"
	"github.com/animus-labs/animus-go/closed/internal/platform/secrets"
	"github.com/animus-labs/animus-go/closed/internal/repo"
)

type Logger interface {
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
}

type Worker struct {
	subscriptions SubscriptionStore
	deliveries    DeliveryStore
	attempts      AttemptStore
	secrets       secrets.Manager
	audit         repo.AuditEventAppender
	logger        Logger
	cfg           Config
	now           func() time.Time
	client        *http.Client
}

func NewWorker(subscriptions SubscriptionStore, deliveries DeliveryStore, attempts AttemptStore, secretsManager secrets.Manager, audit repo.AuditEventAppender, logger Logger, cfg Config) *Worker {
	clientTimeout := cfg.HTTPTimeout
	if clientTimeout <= 0 {
		clientTimeout = 10 * time.Second
	}
	return &Worker{
		subscriptions: subscriptions,
		deliveries:    deliveries,
		attempts:      attempts,
		secrets:       secretsManager,
		audit:         audit,
		logger:        logger,
		cfg:           cfg,
		now:           time.Now,
		client:        &http.Client{Timeout: clientTimeout},
	}
}

func (w *Worker) Run(ctx context.Context) {
	if w == nil || w.deliveries == nil || w.subscriptions == nil || w.attempts == nil {
		return
	}
	if !w.cfg.Enabled() {
		return
	}
	pollInterval := w.cfg.PollInterval
	if pollInterval <= 0 {
		pollInterval = 5 * time.Second
	}
	ticker := time.NewTicker(pollInterval)
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
	batch, err := w.deliveries.ClaimDue(ctx, now, limit, inflight)
	if err != nil {
		w.logWarn("webhook claim failed", "error", err)
		return
	}
	if len(batch) == 0 {
		return
	}
	concurrency := w.cfg.WorkerConcurrency
	if concurrency <= 0 {
		concurrency = 4
	}
	sema := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	for _, delivery := range batch {
		delivery := delivery
		sema <- struct{}{}
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { <-sema }()
			w.processDelivery(ctx, delivery)
		}()
	}
	wg.Wait()
}

func (w *Worker) processDelivery(ctx context.Context, delivery Delivery) {
	now := w.now().UTC()
	attemptNumber := delivery.AttemptCount + 1
	maxAttempts := w.cfg.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 10
	}
	if attemptNumber > maxAttempts {
		w.recordAttempt(ctx, delivery, now, attemptNumber, AttemptOutcomePermanentFailure, nil, "max attempts exceeded", 0, "")
		w.updateDelivery(ctx, delivery, DeliveryStatusFailed, now, attemptNumber, "max attempts exceeded")
		w.auditFailed(ctx, delivery, attemptNumber, nil, "max attempts exceeded")
		return
	}

	subscription, err := w.subscriptions.Get(ctx, delivery.ProjectID, delivery.SubscriptionID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			reason := "subscription not found"
			w.recordAttempt(ctx, delivery, now, attemptNumber, AttemptOutcomePermanentFailure, nil, reason, 0, "")
			w.updateDelivery(ctx, delivery, DeliveryStatusDisabled, now, attemptNumber, reason)
			w.auditFailed(ctx, delivery, attemptNumber, nil, reason)
			return
		}
		w.recordAttempt(ctx, delivery, now, attemptNumber, AttemptOutcomeRetry, nil, err.Error(), 0, "")
		w.scheduleRetry(ctx, delivery, now, attemptNumber, err)
		return
	}
	if !subscription.Enabled {
		reason := "subscription disabled"
		w.recordAttempt(ctx, delivery, now, attemptNumber, AttemptOutcomePermanentFailure, nil, reason, 0, "")
		w.updateDelivery(ctx, delivery, DeliveryStatusDisabled, now, attemptNumber, reason)
		w.auditFailed(ctx, delivery, attemptNumber, nil, reason)
		return
	}

	payload := delivery.Payload
	if len(payload) == 0 {
		payload = []byte(`{}`)
	}

	req, err := w.buildRequest(ctx, delivery, subscription, payload)
	if err != nil {
		if isPermanentBuildError(err) {
			w.recordAttempt(ctx, delivery, now, attemptNumber, AttemptOutcomePermanentFailure, nil, err.Error(), 0, "")
			w.updateDelivery(ctx, delivery, DeliveryStatusFailed, now, attemptNumber, err.Error())
			w.auditFailed(ctx, delivery, attemptNumber, nil, err.Error())
			return
		}
		w.recordAttempt(ctx, delivery, now, attemptNumber, AttemptOutcomeRetry, nil, err.Error(), 0, "")
		w.scheduleRetry(ctx, delivery, now, attemptNumber, err)
		return
	}

	start := w.now()
	resp, reqErr := w.client.Do(req)
	latency := w.now().Sub(start)
	if resp != nil && resp.Body != nil {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
		_ = resp.Body.Close()
	}

	statusCode := responseStatusCode(resp)
	requestID := responseRequestID(resp)
	if reqErr != nil {
		w.recordAttempt(ctx, delivery, now, attemptNumber, AttemptOutcomeRetry, statusCode, reqErr.Error(), latency, requestID)
		w.scheduleRetry(ctx, delivery, now, attemptNumber, reqErr)
		return
	}

	outcome, terminal := classifyOutcome(statusCode)
	if outcome == AttemptOutcomeSuccess {
		w.recordAttempt(ctx, delivery, now, attemptNumber, outcome, statusCode, "", latency, requestID)
		w.updateDelivery(ctx, delivery, DeliveryStatusDelivered, now, attemptNumber, "")
		w.auditDelivered(ctx, delivery, attemptNumber, statusCode)
		return
	}
	if terminal {
		errMsg := fmt.Sprintf("response status %d", statusCode)
		w.recordAttempt(ctx, delivery, now, attemptNumber, AttemptOutcomePermanentFailure, statusCode, errMsg, latency, requestID)
		w.updateDelivery(ctx, delivery, DeliveryStatusFailed, now, attemptNumber, errMsg)
		w.auditFailed(ctx, delivery, attemptNumber, statusCode, errMsg)
		return
	}

	errMsg := fmt.Sprintf("response status %d", statusCode)
	w.recordAttempt(ctx, delivery, now, attemptNumber, AttemptOutcomeRetry, statusCode, errMsg, latency, requestID)
	w.scheduleRetry(ctx, delivery, now, attemptNumber, fmt.Errorf("status %d", statusCode))
}

func (w *Worker) buildRequest(ctx context.Context, delivery Delivery, subscription Subscription, payload []byte) (*http.Request, error) {
	url := strings.TrimSpace(subscription.TargetURL)
	if url == "" {
		return nil, errInvalidTarget
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("%w: %s", errInvalidTarget, err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Animus-Event-Id", delivery.EventID)
	req.Header.Set("X-Animus-Event-Type", delivery.EventType.String())
	req.Header.Set("X-Animus-Delivery-Id", delivery.ID)
	req.Header.Set("Idempotency-Key", idempotencyKey(delivery.EventID, delivery.SubscriptionID))
	for key, value := range subscription.Headers {
		k := strings.TrimSpace(key)
		if k == "" {
			continue
		}
		req.Header.Set(k, strings.TrimSpace(value))
	}
	if strings.TrimSpace(subscription.SecretRef) != "" {
		secret, err := w.resolveSigningSecret(ctx, delivery.ProjectID, subscription.SecretRef)
		if err != nil {
			return nil, err
		}
		signature, err := SignPayload(secret, payload)
		if err != nil {
			return nil, err
		}
		req.Header.Set("X-Animus-Signature", signature)
	}
	return req, nil
}

func (w *Worker) resolveSigningSecret(ctx context.Context, projectID, classRef string) (string, error) {
	classRef = strings.TrimSpace(classRef)
	if classRef == "" {
		return "", nil
	}
	if w.secrets == nil {
		return "", errSecretUnavailable
	}
	lease, err := w.secrets.Fetch(ctx, secrets.Request{
		ProjectID: strings.TrimSpace(projectID),
		Subject:   "webhook",
		ClassRef:  classRef,
	})
	if err != nil {
		return "", err
	}
	key := strings.TrimSpace(w.cfg.SigningSecretKey)
	if key == "" {
		key = "WEBHOOK_SIGNING_SECRET"
	}
	value := strings.TrimSpace(lease.Env[key])
	if value == "" {
		return "", fmt.Errorf("%w: %s", errSecretKeyMissing, key)
	}
	return value, nil
}

func (w *Worker) recordAttempt(ctx context.Context, delivery Delivery, attemptedAt time.Time, attemptNumber int, outcome AttemptOutcome, statusCode *int, errMsg string, latency time.Duration, requestID string) {
	if w == nil || w.attempts == nil {
		return
	}
	attempt := Attempt{
		DeliveryID:  delivery.ID,
		AttemptedAt: attemptedAt,
		StatusCode:  statusCode,
		Outcome:     outcome,
		Error:       sanitizeError(errMsg),
		LatencyMs:   int(latency / time.Millisecond),
		RequestID:   strings.TrimSpace(requestID),
		CreatedAt:   attemptedAt,
	}
	if _, err := w.attempts.Insert(ctx, attempt); err != nil {
		w.logWarn("webhook attempt insert failed", "delivery_id", delivery.ID, "project_id", delivery.ProjectID, "event_id", delivery.EventID, "error", err)
	}
	w.auditAttempt(ctx, delivery, attemptNumber, outcome, statusCode, errMsg)
}

func (w *Worker) updateDelivery(ctx context.Context, delivery Delivery, status DeliveryStatus, now time.Time, attemptCount int, errMsg string) {
	if w == nil || w.deliveries == nil {
		return
	}
	delivery.Status = status
	delivery.AttemptCount = attemptCount
	delivery.NextAttemptAt = now
	delivery.LastError = sanitizeError(errMsg)
	delivery.UpdatedAt = now
	if _, err := w.deliveries.Update(ctx, delivery); err != nil {
		w.logWarn("webhook delivery update failed", "delivery_id", delivery.ID, "project_id", delivery.ProjectID, "event_id", delivery.EventID, "error", err)
	}
}

func (w *Worker) scheduleRetry(ctx context.Context, delivery Delivery, now time.Time, attemptCount int, err error) {
	delay := backoffDelay(attemptCount, w.cfg.RetryBaseDelay, w.cfg.RetryMaxDelay)
	nextAttempt := now.Add(delay)
	delivery.Status = DeliveryStatusPending
	delivery.AttemptCount = attemptCount
	delivery.NextAttemptAt = nextAttempt
	delivery.LastError = sanitizeError(err.Error())
	delivery.UpdatedAt = now
	if _, updateErr := w.deliveries.Update(ctx, delivery); updateErr != nil {
		w.logWarn("webhook delivery retry update failed", "delivery_id", delivery.ID, "project_id", delivery.ProjectID, "event_id", delivery.EventID, "error", updateErr)
		return
	}
	w.logWarn("webhook delivery retry scheduled", "delivery_id", delivery.ID, "project_id", delivery.ProjectID, "event_id", delivery.EventID, "attempt", attemptCount, "error", err)
}

func (w *Worker) logWarn(msg string, args ...any) {
	if w != nil && w.logger != nil {
		w.logger.Warn(msg, args...)
	}
}

func (w *Worker) auditAttempt(ctx context.Context, delivery Delivery, attemptNumber int, outcome AttemptOutcome, statusCode *int, errMsg string) {
	if w == nil || w.audit == nil {
		return
	}
	payload := domain.Metadata{
		"project_id":      delivery.ProjectID,
		"subscription_id": delivery.SubscriptionID,
		"event_id":        delivery.EventID,
		"event_type":      delivery.EventType.String(),
		"attempt":         attemptNumber,
		"outcome":         string(outcome),
		"status_code":     statusCodeValue(statusCode),
		"error":           sanitizeError(errMsg),
	}
	_, _ = w.audit.Append(ctx, domain.AuditEvent{
		OccurredAt:   w.now().UTC(),
		Actor:        "system:webhook-dispatcher",
		Action:       "webhook.delivery.attempted",
		ResourceType: "webhook_delivery",
		ResourceID:   delivery.ID,
		Payload:      payload,
	})
}

func (w *Worker) auditDelivered(ctx context.Context, delivery Delivery, attemptNumber int, statusCode *int) {
	if w == nil || w.audit == nil {
		return
	}
	payload := domain.Metadata{
		"project_id":      delivery.ProjectID,
		"subscription_id": delivery.SubscriptionID,
		"event_id":        delivery.EventID,
		"event_type":      delivery.EventType.String(),
		"attempt":         attemptNumber,
		"status_code":     statusCodeValue(statusCode),
	}
	_, _ = w.audit.Append(ctx, domain.AuditEvent{
		OccurredAt:   w.now().UTC(),
		Actor:        "system:webhook-dispatcher",
		Action:       "webhook.delivery.delivered",
		ResourceType: "webhook_delivery",
		ResourceID:   delivery.ID,
		Payload:      payload,
	})
}

func (w *Worker) auditFailed(ctx context.Context, delivery Delivery, attemptNumber int, statusCode *int, errMsg string) {
	if w == nil || w.audit == nil {
		return
	}
	payload := domain.Metadata{
		"project_id":      delivery.ProjectID,
		"subscription_id": delivery.SubscriptionID,
		"event_id":        delivery.EventID,
		"event_type":      delivery.EventType.String(),
		"attempt":         attemptNumber,
		"status_code":     statusCodeValue(statusCode),
		"error":           sanitizeError(errMsg),
	}
	_, _ = w.audit.Append(ctx, domain.AuditEvent{
		OccurredAt:   w.now().UTC(),
		Actor:        "system:webhook-dispatcher",
		Action:       "webhook.delivery.failed",
		ResourceType: "webhook_delivery",
		ResourceID:   delivery.ID,
		Payload:      payload,
	})
}

func sanitizeError(msg string) string {
	clean := redaction.RedactString(strings.TrimSpace(msg))
	if len(clean) > 500 {
		return clean[:500]
	}
	return clean
}

func responseStatusCode(resp *http.Response) *int {
	if resp == nil {
		return nil
	}
	code := resp.StatusCode
	return &code
}

func statusCodeValue(code *int) any {
	if code == nil {
		return nil
	}
	return *code
}

func responseRequestID(resp *http.Response) string {
	if resp == nil {
		return ""
	}
	requestID := strings.TrimSpace(resp.Header.Get("X-Request-Id"))
	if requestID == "" {
		requestID = strings.TrimSpace(resp.Header.Get("X-Request-ID"))
	}
	return requestID
}

func classifyOutcome(statusCode *int) (AttemptOutcome, bool) {
	if statusCode == nil {
		return AttemptOutcomeRetry, false
	}
	code := *statusCode
	if code >= 200 && code <= 299 {
		return AttemptOutcomeSuccess, false
	}
	if code == http.StatusTooManyRequests {
		return AttemptOutcomeRetry, false
	}
	if code >= 500 {
		return AttemptOutcomeRetry, false
	}
	if code >= 400 {
		return AttemptOutcomePermanentFailure, true
	}
	return AttemptOutcomeRetry, false
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

var (
	errSecretUnavailable = errors.New("webhook signing secret unavailable")
	errSecretKeyMissing  = errors.New("webhook signing secret key missing")
	errInvalidTarget     = errors.New("webhook target url invalid")
)

func idempotencyKey(eventID, subscriptionID string) string {
	return strings.TrimSpace(eventID) + ":" + strings.TrimSpace(subscriptionID)
}

func isPermanentBuildError(err error) bool {
	return errors.Is(err, errInvalidTarget) || errors.Is(err, errSecretUnavailable) || errors.Is(err, errSecretKeyMissing)
}
