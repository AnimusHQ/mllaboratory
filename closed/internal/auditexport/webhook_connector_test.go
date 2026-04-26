package auditexport

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"testing"

	"github.com/animus-labs/animus-go/closed/internal/platform/secrets"
)

type stubHTTPDoer struct {
	resp *http.Response
	err  error
	req  *http.Request
	body []byte
}

func (s *stubHTTPDoer) Do(req *http.Request) (*http.Response, error) {
	s.req = req
	if req != nil && req.Body != nil {
		body, _ := io.ReadAll(req.Body)
		s.body = body
	}
	if s.err != nil {
		return nil, s.err
	}
	return s.resp, nil
}

type stubSecrets struct {
	value string
	key   string
	err   error
}

func (s stubSecrets) Fetch(ctx context.Context, req secrets.Request) (secrets.Lease, error) {
	if s.err != nil {
		return secrets.Lease{}, s.err
	}
	return secrets.Lease{Env: map[string]string{s.key: s.value}}, nil
}

func TestWebhookConnectorSuccessWithSignature(t *testing.T) {
	doer := &stubHTTPDoer{resp: &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(nil))}}
	secretValue := "topsecret"
	cfg := SinkConfig{
		WebhookURL:        "https://example.test",
		WebhookSecretRef:  "secret/ref",
		WebhookSigningKey: "SIGN_KEY",
	}
	connector := WebhookConnector{
		Client:  doer,
		Secrets: stubSecrets{value: secretValue, key: "SIGN_KEY"},
	}
	payload := []byte(`{"event":"ok"}`)
	result := connector.Deliver(context.Background(), "sink", 101, cfg, payload)
	if result.Outcome != AttemptOutcomeSuccess {
		t.Fatalf("expected success, got %s", result.Outcome)
	}
	if doer.req == nil {
		t.Fatalf("expected request")
	}
	signature := doer.req.Header.Get("X-Animus-Signature")
	expected, err := signPayload(secretValue, payload)
	if err != nil {
		t.Fatalf("sign payload: %v", err)
	}
	if signature != expected {
		t.Fatalf("expected signature %q, got %q", expected, signature)
	}
	if got := doer.req.Header.Get("Idempotency-Key"); got == "" {
		t.Fatalf("expected idempotency key")
	}
	if !bytes.HasSuffix(doer.body, []byte("\n")) {
		t.Fatalf("expected newline-terminated body")
	}
}

func TestWebhookConnectorStatusClassification(t *testing.T) {
	cases := []struct {
		status  int
		outcome AttemptOutcome
	}{
		{status: 500, outcome: AttemptOutcomeRetry},
		{status: 429, outcome: AttemptOutcomeRetry},
		{status: 400, outcome: AttemptOutcomePermanentFailure},
	}
	for _, tc := range cases {
		doer := &stubHTTPDoer{resp: &http.Response{StatusCode: tc.status, Body: io.NopCloser(bytes.NewReader(nil))}}
		connector := WebhookConnector{Client: doer}
		cfg := SinkConfig{WebhookURL: "https://example.test"}
		result := connector.Deliver(context.Background(), "sink", 1, cfg, []byte("{}"))
		if result.Outcome != tc.outcome {
			t.Fatalf("status %d expected %s, got %s", tc.status, tc.outcome, result.Outcome)
		}
	}
}

func TestWebhookConnectorMissingSecretKeyIsPermanent(t *testing.T) {
	doer := &stubHTTPDoer{resp: &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(nil))}}
	connector := WebhookConnector{
		Client:  doer,
		Secrets: stubSecrets{value: "", key: "SIGN_KEY"},
	}
	cfg := SinkConfig{WebhookURL: "https://example.test", WebhookSecretRef: "secret/ref", WebhookSigningKey: "SIGN_KEY"}
	result := connector.Deliver(context.Background(), "sink", 1, cfg, []byte("{}"))
	if result.Outcome != AttemptOutcomePermanentFailure {
		t.Fatalf("expected permanent failure, got %s", result.Outcome)
	}
}

func TestWebhookConnectorTransportErrorIsRetry(t *testing.T) {
	doer := &stubHTTPDoer{err: errors.New("network")}
	connector := WebhookConnector{Client: doer}
	cfg := SinkConfig{WebhookURL: "https://example.test"}
	result := connector.Deliver(context.Background(), "sink", 1, cfg, []byte("{}"))
	if result.Outcome != AttemptOutcomeRetry {
		t.Fatalf("expected retry, got %s", result.Outcome)
	}
}
