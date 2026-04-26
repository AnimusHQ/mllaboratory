package auditexport

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/animus-labs/animus-go/closed/internal/platform/secrets"
)

const defaultAuditExportSigningKey = "AUDIT_EXPORT_SIGNING_SECRET"

type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

type WebhookConnector struct {
	Client     HTTPDoer
	Secrets    secrets.Manager
	SigningKey string
}

var (
	errSecretUnavailable = errors.New("signing secret unavailable")
	errSecretKeyMissing  = errors.New("signing secret key missing")
)

func (c WebhookConnector) Deliver(ctx context.Context, sinkID string, eventID int64, cfg SinkConfig, payload []byte) DeliveryResult {
	url := strings.TrimSpace(cfg.WebhookURL)
	if url == "" {
		return DeliveryResult{Outcome: AttemptOutcomePermanentFailure, Error: "webhook_url_required"}
	}
	body := append(append([]byte{}, payload...), '\n')
	client := c.Client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return DeliveryResult{Outcome: AttemptOutcomePermanentFailure, Error: "webhook_request_invalid"}
	}
	req.Header.Set("Content-Type", "application/x-ndjson")
	if strings.TrimSpace(sinkID) != "" && eventID > 0 {
		req.Header.Set("Idempotency-Key", fmt.Sprintf("%s:%d", sinkID, eventID))
	}
	if eventID > 0 {
		req.Header.Set("X-Audit-Event-Id", fmt.Sprintf("%d", eventID))
	}
	for key, value := range cfg.WebhookHeaders {
		name := strings.TrimSpace(key)
		if name == "" {
			continue
		}
		req.Header.Set(name, strings.TrimSpace(value))
	}
	if strings.TrimSpace(cfg.WebhookSecretRef) != "" {
		secret, err := c.resolveSigningSecret(ctx, cfg.WebhookSecretRef, cfg.WebhookSigningKey)
		if err != nil {
			if errors.Is(err, errSecretKeyMissing) || errors.Is(err, errSecretUnavailable) {
				return DeliveryResult{Outcome: AttemptOutcomePermanentFailure, Error: "signing_secret_unavailable"}
			}
			return DeliveryResult{Outcome: AttemptOutcomeRetry, Error: "signing_secret_fetch_failed"}
		}
		signature, err := signPayload(secret, payload)
		if err != nil {
			return DeliveryResult{Outcome: AttemptOutcomePermanentFailure, Error: "signing_failed"}
		}
		req.Header.Set("X-Animus-Signature", signature)
	}

	start := time.Now()
	resp, err := client.Do(req)
	latency := time.Since(start)
	if err != nil {
		return DeliveryResult{Outcome: AttemptOutcomeRetry, Error: "webhook_transport_error", Latency: latency}
	}
	defer resp.Body.Close()
	statusCode := resp.StatusCode
	result := DeliveryResult{StatusCode: &statusCode, Latency: latency}
	if statusCode >= 200 && statusCode < 300 {
		result.Outcome = AttemptOutcomeSuccess
		return result
	}
	result.Error = fmt.Sprintf("http_status_%d", statusCode)
	if statusCode == http.StatusTooManyRequests || statusCode >= 500 {
		result.Outcome = AttemptOutcomeRetry
		return result
	}
	result.Outcome = AttemptOutcomePermanentFailure
	return result
}

func (c WebhookConnector) resolveSigningSecret(ctx context.Context, classRef string, key string) (string, error) {
	classRef = strings.TrimSpace(classRef)
	if classRef == "" {
		return "", nil
	}
	key = strings.TrimSpace(key)
	if key == "" {
		key = strings.TrimSpace(c.SigningKey)
	}
	if key == "" {
		key = defaultAuditExportSigningKey
	}
	value, err := secrets.FetchIntegrationSecret(ctx, c.Secrets, secrets.IntegrationSecret{
		Scope:    secrets.IntegrationScopeAuditExport,
		ClassRef: classRef,
		Key:      key,
	})
	if err != nil {
		switch {
		case errors.Is(err, secrets.ErrIntegrationSecretUnavailable):
			return "", errSecretUnavailable
		case errors.Is(err, secrets.ErrIntegrationSecretKeyMissing):
			return "", errSecretKeyMissing
		default:
			return "", err
		}
	}
	return value, nil
}
