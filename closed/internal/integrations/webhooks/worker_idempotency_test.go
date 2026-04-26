package webhooks

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/animus-labs/animus-go/closed/internal/platform/secrets"
	"github.com/animus-labs/animus-go/closed/internal/testsupport"
)

type queueDeliveryStore struct {
	mu         sync.Mutex
	deliveries map[string]Delivery
}

func newQueueDeliveryStore(delivery Delivery) *queueDeliveryStore {
	return &queueDeliveryStore{
		deliveries: map[string]Delivery{delivery.ID: delivery},
	}
}

func (s *queueDeliveryStore) ClaimDue(ctx context.Context, now time.Time, limit int, hold time.Duration) ([]Delivery, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Delivery, 0, 1)
	for _, delivery := range s.deliveries {
		if delivery.Status != DeliveryStatusPending {
			continue
		}
		if delivery.NextAttemptAt.After(now) {
			continue
		}
		out = append(out, delivery)
	}
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (s *queueDeliveryStore) Update(ctx context.Context, delivery Delivery) (Delivery, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deliveries[delivery.ID] = delivery
	return delivery, nil
}

func (s *queueDeliveryStore) get(id string) (Delivery, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delivery, ok := s.deliveries[id]
	return delivery, ok
}

type sequenceTransport struct {
	mu    sync.Mutex
	codes []int
}

func (s *sequenceTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	s.mu.Lock()
	code := http.StatusOK
	if len(s.codes) > 0 {
		code = s.codes[0]
		s.codes = s.codes[1:]
	}
	s.mu.Unlock()
	return &http.Response{
		StatusCode: code,
		Body:       io.NopCloser(bytes.NewBufferString("ok")),
		Header:     make(http.Header),
	}, nil
}

func TestWorkerIdempotentRetryThenSuccess(t *testing.T) {
	now := time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC)
	clock := testsupport.NewManualClock(now)
	delivery := baseDelivery(now)
	subscription := baseSubscription()

	attemptStore := &stubAttemptStore{}
	deliveryStore := newQueueDeliveryStore(delivery)
	worker := NewWorker(stubSubscriptionStore{sub: subscription}, deliveryStore, attemptStore, secrets.NoopManager{}, nil, nil, Config{
		EnabledFlag:    true,
		MaxAttempts:    3,
		RetryBaseDelay: 10 * time.Second,
		RetryMaxDelay:  1 * time.Minute,
	})
	worker.now = clock.Now
	worker.client = &http.Client{Transport: &sequenceTransport{codes: []int{http.StatusInternalServerError, http.StatusOK}}}

	worker.processOnce(context.Background())
	updated, ok := deliveryStore.get(delivery.ID)
	if !ok {
		t.Fatalf("expected delivery")
	}
	if updated.Status != DeliveryStatusPending {
		t.Fatalf("expected pending after retry, got %s", updated.Status)
	}
	if updated.AttemptCount != 1 {
		t.Fatalf("expected attempt_count=1, got %d", updated.AttemptCount)
	}
	if len(attemptStore.attempts) != 1 {
		t.Fatalf("expected 1 attempt, got %d", len(attemptStore.attempts))
	}

	clock.Set(updated.NextAttemptAt)
	worker.processOnce(context.Background())
	updated, ok = deliveryStore.get(delivery.ID)
	if !ok {
		t.Fatalf("expected delivery on second attempt")
	}
	if updated.Status != DeliveryStatusDelivered {
		t.Fatalf("expected delivered after success, got %s", updated.Status)
	}
	if updated.AttemptCount != 2 {
		t.Fatalf("expected attempt_count=2, got %d", updated.AttemptCount)
	}
	if len(attemptStore.attempts) != 2 {
		t.Fatalf("expected 2 attempts, got %d", len(attemptStore.attempts))
	}
	if len(deliveryStore.deliveries) != 1 {
		t.Fatalf("expected single delivery identity, got %d", len(deliveryStore.deliveries))
	}
}
