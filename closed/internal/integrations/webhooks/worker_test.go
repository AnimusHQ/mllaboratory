package webhooks

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/animus-labs/animus-go/closed/internal/platform/secrets"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

type stubSubscriptionStore struct {
	sub Subscription
	err error
}

func (s stubSubscriptionStore) Get(ctx context.Context, projectID, subscriptionID string) (Subscription, error) {
	return s.sub, s.err
}

type stubDeliveryStore struct {
	updated []Delivery
}

func (s *stubDeliveryStore) ClaimDue(ctx context.Context, now time.Time, limit int, hold time.Duration) ([]Delivery, error) {
	return nil, nil
}

func (s *stubDeliveryStore) Update(ctx context.Context, delivery Delivery) (Delivery, error) {
	s.updated = append(s.updated, delivery)
	return delivery, nil
}

type stubAttemptStore struct {
	attempts  []Attempt
	insertErr error
}

func (s *stubAttemptStore) Insert(ctx context.Context, attempt Attempt) (Attempt, error) {
	if s.insertErr != nil {
		return Attempt{}, s.insertErr
	}
	s.attempts = append(s.attempts, attempt)
	return attempt, nil
}

type stubSecretsManager struct {
	lease secrets.Lease
	err   error
}

func (s stubSecretsManager) Fetch(ctx context.Context, req secrets.Request) (secrets.Lease, error) {
	if s.err != nil {
		return secrets.Lease{}, s.err
	}
	return s.lease, nil
}

func baseDelivery(now time.Time) Delivery {
	return Delivery{
		ID:             "delivery-1",
		ProjectID:      "proj-1",
		SubscriptionID: "sub-1",
		EventID:        "evt-1",
		EventType:      EventRunFinished,
		Payload:        []byte(`{"event_id":"evt-1"}`),
		Status:         DeliveryStatusPending,
		NextAttemptAt:  now,
		AttemptCount:   0,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

func baseSubscription() Subscription {
	return Subscription{
		ID:         "sub-1",
		ProjectID:  "proj-1",
		Name:       "test",
		TargetURL:  "https://example.com/hook",
		Enabled:    true,
		EventTypes: []EventType{EventRunFinished},
	}
}

func TestWorkerProcessDeliverySuccess(t *testing.T) {
	now := time.Date(2025, 2, 1, 12, 0, 0, 0, time.UTC)
	delivery := baseDelivery(now)
	subscription := baseSubscription()

	attemptStore := &stubAttemptStore{}
	deliveryStore := &stubDeliveryStore{}
	worker := NewWorker(stubSubscriptionStore{sub: subscription}, deliveryStore, attemptStore, secrets.NoopManager{}, nil, nil, Config{
		EnabledFlag: true,
		MaxAttempts: 3,
	})
	worker.now = func() time.Time { return now }

	var received *http.Request
	worker.client = &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			received = req
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewBufferString("ok")),
				Header:     make(http.Header),
			}, nil
		}),
	}

	worker.processDelivery(context.Background(), delivery)

	if len(attemptStore.attempts) != 1 {
		t.Fatalf("expected 1 attempt, got %d", len(attemptStore.attempts))
	}
	if len(deliveryStore.updated) == 0 {
		t.Fatalf("expected delivery update")
	}
	updated := deliveryStore.updated[len(deliveryStore.updated)-1]
	if updated.Status != DeliveryStatusDelivered {
		t.Fatalf("expected delivered status, got %s", updated.Status)
	}
	if updated.AttemptCount != 1 {
		t.Fatalf("expected attempt_count=1, got %d", updated.AttemptCount)
	}
	if received == nil {
		t.Fatalf("expected request")
	}
	if got := received.Header.Get("Idempotency-Key"); got == "" {
		t.Fatalf("expected idempotency header")
	}
	if got := received.Header.Get("X-Animus-Event-Id"); got != delivery.EventID {
		t.Fatalf("unexpected event id header: %s", got)
	}
}

func TestWorkerProcessDeliveryRetryOnServerError(t *testing.T) {
	now := time.Date(2025, 2, 1, 12, 0, 0, 0, time.UTC)
	delivery := baseDelivery(now)
	subscription := baseSubscription()

	attemptStore := &stubAttemptStore{}
	deliveryStore := &stubDeliveryStore{}
	worker := NewWorker(stubSubscriptionStore{sub: subscription}, deliveryStore, attemptStore, secrets.NoopManager{}, nil, nil, Config{
		EnabledFlag:    true,
		MaxAttempts:    3,
		RetryBaseDelay: 10 * time.Second,
		RetryMaxDelay:  2 * time.Minute,
	})
	worker.now = func() time.Time { return now }
	worker.client = &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusInternalServerError,
				Body:       io.NopCloser(bytes.NewBufferString("boom")),
				Header:     make(http.Header),
			}, nil
		}),
	}

	worker.processDelivery(context.Background(), delivery)

	if len(deliveryStore.updated) == 0 {
		t.Fatalf("expected delivery update")
	}
	updated := deliveryStore.updated[len(deliveryStore.updated)-1]
	if updated.Status != DeliveryStatusPending {
		t.Fatalf("expected pending status, got %s", updated.Status)
	}
	expected := now.Add(10 * time.Second)
	if !updated.NextAttemptAt.Equal(expected) {
		t.Fatalf("expected next_attempt_at %v, got %v", expected, updated.NextAttemptAt)
	}
}

func TestWorkerProcessDeliveryPermanentFailureOnClientError(t *testing.T) {
	now := time.Date(2025, 2, 1, 12, 0, 0, 0, time.UTC)
	delivery := baseDelivery(now)
	subscription := baseSubscription()

	attemptStore := &stubAttemptStore{}
	deliveryStore := &stubDeliveryStore{}
	worker := NewWorker(stubSubscriptionStore{sub: subscription}, deliveryStore, attemptStore, secrets.NoopManager{}, nil, nil, Config{
		EnabledFlag: true,
		MaxAttempts: 3,
	})
	worker.now = func() time.Time { return now }
	worker.client = &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusBadRequest,
				Body:       io.NopCloser(bytes.NewBufferString("bad")),
				Header:     make(http.Header),
			}, nil
		}),
	}

	worker.processDelivery(context.Background(), delivery)

	if len(deliveryStore.updated) == 0 {
		t.Fatalf("expected delivery update")
	}
	updated := deliveryStore.updated[len(deliveryStore.updated)-1]
	if updated.Status != DeliveryStatusFailed {
		t.Fatalf("expected failed status, got %s", updated.Status)
	}
}

func TestWorkerSignatureHeader(t *testing.T) {
	now := time.Date(2025, 2, 1, 12, 0, 0, 0, time.UTC)
	delivery := baseDelivery(now)
	subscription := baseSubscription()
	subscription.SecretRef = "secret/class"

	attemptStore := &stubAttemptStore{}
	deliveryStore := &stubDeliveryStore{}
	secretValue := "signing-secret"
	worker := NewWorker(stubSubscriptionStore{sub: subscription}, deliveryStore, attemptStore, stubSecretsManager{
		lease: secrets.Lease{Env: map[string]string{"WEBHOOK_SIGNING_SECRET": secretValue}},
	}, nil, nil, Config{
		EnabledFlag:      true,
		MaxAttempts:      3,
		SigningSecretKey: "WEBHOOK_SIGNING_SECRET",
		RetryBaseDelay:   5 * time.Second,
		RetryMaxDelay:    1 * time.Minute,
	})
	worker.now = func() time.Time { return now }

	var received *http.Request
	worker.client = &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			received = req
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewBufferString("ok")),
				Header:     make(http.Header),
			}, nil
		}),
	}

	worker.processDelivery(context.Background(), delivery)

	if received == nil {
		t.Fatalf("expected request")
	}
	expected, err := SignPayload(secretValue, delivery.Payload)
	if err != nil {
		t.Fatalf("sign payload: %v", err)
	}
	if got := received.Header.Get("X-Animus-Signature"); got != expected {
		t.Fatalf("unexpected signature header: %s", got)
	}
}

func TestBackoffDelay(t *testing.T) {
	base := 5 * time.Second
	max := 30 * time.Second
	if got := backoffDelay(1, base, max); got != base {
		t.Fatalf("attempt1 expected %v, got %v", base, got)
	}
	if got := backoffDelay(2, base, max); got != 10*time.Second {
		t.Fatalf("attempt2 expected 10s, got %v", got)
	}
	if got := backoffDelay(4, base, max); got != max {
		t.Fatalf("attempt4 expected %v, got %v", max, got)
	}
}

func TestWorkerRetryOnTransportError(t *testing.T) {
	now := time.Date(2025, 2, 1, 12, 0, 0, 0, time.UTC)
	delivery := baseDelivery(now)
	subscription := baseSubscription()

	attemptStore := &stubAttemptStore{}
	deliveryStore := &stubDeliveryStore{}
	worker := NewWorker(stubSubscriptionStore{sub: subscription}, deliveryStore, attemptStore, secrets.NoopManager{}, nil, nil, Config{
		EnabledFlag:    true,
		MaxAttempts:    3,
		RetryBaseDelay: 5 * time.Second,
		RetryMaxDelay:  1 * time.Minute,
	})
	worker.now = func() time.Time { return now }
	worker.client = &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			return nil, errors.New("dial failed")
		}),
	}

	worker.processDelivery(context.Background(), delivery)

	if len(deliveryStore.updated) == 0 {
		t.Fatalf("expected delivery update")
	}
	updated := deliveryStore.updated[len(deliveryStore.updated)-1]
	if updated.Status != DeliveryStatusPending {
		t.Fatalf("expected pending status on transport error")
	}
}

func TestWorkerRetryStormIncrementsAttempts(t *testing.T) {
	now := time.Date(2025, 2, 1, 12, 0, 0, 0, time.UTC)
	delivery := baseDelivery(now)
	subscription := baseSubscription()

	attemptStore := &stubAttemptStore{}
	deliveryStore := &stubDeliveryStore{}
	worker := NewWorker(stubSubscriptionStore{sub: subscription}, deliveryStore, attemptStore, secrets.NoopManager{}, nil, nil, Config{
		EnabledFlag:    true,
		MaxAttempts:    5,
		RetryBaseDelay: time.Second,
		RetryMaxDelay:  10 * time.Second,
	})
	worker.now = func() time.Time { return now }
	worker.client = &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusInternalServerError,
				Body:       io.NopCloser(bytes.NewBufferString("fail")),
				Header:     make(http.Header),
			}, nil
		}),
	}

	worker.processDelivery(context.Background(), delivery)
	if len(deliveryStore.updated) != 1 {
		t.Fatalf("expected delivery update after first retry, got %d", len(deliveryStore.updated))
	}
	first := deliveryStore.updated[0]
	if first.AttemptCount != 1 || first.Status != DeliveryStatusPending {
		t.Fatalf("expected attempt_count=1 pending, got %d %s", first.AttemptCount, first.Status)
	}

	worker.processDelivery(context.Background(), first)
	if len(deliveryStore.updated) != 2 {
		t.Fatalf("expected delivery update after second retry, got %d", len(deliveryStore.updated))
	}
	second := deliveryStore.updated[1]
	if second.AttemptCount != 2 {
		t.Fatalf("expected attempt_count=2, got %d", second.AttemptCount)
	}
	if !second.NextAttemptAt.After(first.NextAttemptAt) {
		t.Fatalf("expected next_attempt to increase on retry storm")
	}
}
