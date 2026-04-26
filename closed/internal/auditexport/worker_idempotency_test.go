package auditexport

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/animus-labs/animus-go/closed/internal/domain"
	"github.com/animus-labs/animus-go/closed/internal/testsupport"
)

type memoryDeliveryStore struct {
	mu             sync.Mutex
	deliveries     map[int64]Delivery
	markDLQCount   int
	markRetryCount int
}

func newMemoryDeliveryStore(delivery Delivery) *memoryDeliveryStore {
	return &memoryDeliveryStore{
		deliveries: map[int64]Delivery{delivery.DeliveryID: delivery},
	}
}

func (s *memoryDeliveryStore) Backfill(ctx context.Context, sinkID string) error {
	return nil
}

func (s *memoryDeliveryStore) ClaimDue(ctx context.Context, now time.Time, inflightTimeout time.Duration, limit int) ([]Delivery, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Delivery, 0, 1)
	for id, delivery := range s.deliveries {
		if delivery.Status == DeliveryStatusDelivered || delivery.Status == DeliveryStatusDLQ {
			continue
		}
		if delivery.NextAttemptAt.After(now) {
			continue
		}
		delivery.AttemptCount++
		delivery.Status = DeliveryStatusInflight
		delivery.UpdatedAt = now
		delivery.NextAttemptAt = now.Add(inflightTimeout)
		s.deliveries[id] = delivery
		out = append(out, delivery)
	}
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (s *memoryDeliveryStore) MarkDelivered(ctx context.Context, deliveryID int64, deliveredAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delivery := s.deliveries[deliveryID]
	delivery.Status = DeliveryStatusDelivered
	delivery.DeliveredAt = &deliveredAt
	delivery.UpdatedAt = deliveredAt
	s.deliveries[deliveryID] = delivery
	return nil
}

func (s *memoryDeliveryStore) MarkRetry(ctx context.Context, deliveryID int64, lastError string, nextAttemptAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delivery := s.deliveries[deliveryID]
	delivery.Status = DeliveryStatusRetry
	delivery.LastError = lastError
	delivery.NextAttemptAt = nextAttemptAt
	delivery.UpdatedAt = nextAttemptAt
	s.deliveries[deliveryID] = delivery
	s.markRetryCount++
	return nil
}

func (s *memoryDeliveryStore) MarkDLQ(ctx context.Context, deliveryID int64, reason string, lastError string, at time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delivery := s.deliveries[deliveryID]
	delivery.Status = DeliveryStatusDLQ
	delivery.DLQReason = reason
	delivery.LastError = lastError
	delivery.UpdatedAt = at
	s.deliveries[deliveryID] = delivery
	s.markDLQCount++
	return nil
}

func (s *memoryDeliveryStore) Replay(ctx context.Context, deliveryID int64, scheduledAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delivery := s.deliveries[deliveryID]
	delivery.Status = DeliveryStatusPending
	delivery.NextAttemptAt = scheduledAt
	delivery.UpdatedAt = scheduledAt
	delivery.LastError = ""
	delivery.DLQReason = ""
	s.deliveries[deliveryID] = delivery
	return nil
}

func (s *memoryDeliveryStore) Get(ctx context.Context, deliveryID int64) (Delivery, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delivery, ok := s.deliveries[deliveryID]
	if !ok {
		return Delivery{}, errors.New("not found")
	}
	return delivery, nil
}

func (s *memoryDeliveryStore) List(ctx context.Context, status string, sinkID string, limit int) ([]Delivery, error) {
	return nil, nil
}

func (s *memoryDeliveryStore) CountByStatus(ctx context.Context, status DeliveryStatus) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	count := 0
	for _, delivery := range s.deliveries {
		if delivery.Status == status {
			count++
		}
	}
	return count, nil
}

type idempotencySinkStore struct {
	sink Sink
}

func (s idempotencySinkStore) UpsertSink(ctx context.Context, sink Sink) (Sink, error) {
	return s.sink, nil
}

func (s idempotencySinkStore) GetSink(ctx context.Context, sinkID string) (Sink, error) {
	return s.sink, nil
}

func (s idempotencySinkStore) ListSinks(ctx context.Context, limit int) ([]Sink, error) {
	return []Sink{s.sink}, nil
}

type idempotencyEventStore struct {
	event domain.AuditEvent
	err   error
}

func (s idempotencyEventStore) GetEvent(ctx context.Context, eventID int64) (domain.AuditEvent, error) {
	if s.err != nil {
		return domain.AuditEvent{}, s.err
	}
	return s.event, nil
}

type idempotencyAttemptStore struct {
	attempts []DeliveryAttempt
}

func (s *idempotencyAttemptStore) Insert(ctx context.Context, attempt DeliveryAttempt) (DeliveryAttempt, error) {
	s.attempts = append(s.attempts, attempt)
	return attempt, nil
}

func (s *idempotencyAttemptStore) List(ctx context.Context, deliveryID int64, limit int) ([]DeliveryAttempt, error) {
	return s.attempts, nil
}

type failingDialer struct {
	err error
}

func (d failingDialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	return nil, d.err
}

func TestWorkerMovesToDLQOnceAfterRetries(t *testing.T) {
	now := time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC)
	clock := testsupport.NewManualClock(now)
	delivery := Delivery{
		DeliveryID:    1,
		SinkID:        "sink-1",
		EventID:       42,
		Status:        DeliveryStatusPending,
		AttemptCount:  0,
		NextAttemptAt: now,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	store := newMemoryDeliveryStore(delivery)
	attempts := &idempotencyAttemptStore{}
	sinkCfg, err := json.Marshal(SinkConfig{SyslogAddr: "syslog:1234", SyslogProtocol: "tcp"})
	if err != nil {
		t.Fatalf("marshal sink config: %v", err)
	}
	sinkStore := idempotencySinkStore{sink: Sink{
		SinkID:      "sink-1",
		Name:        "default",
		Destination: "syslog",
		Format:      "ndjson",
		Config:      sinkCfg,
		Enabled:     true,
	}}
	eventStore := idempotencyEventStore{event: domain.AuditEvent{EventID: 42}}

	worker := NewWorker(store, attempts, sinkStore, eventStore, nil, nil, Config{
		Destination:    "syslog",
		MaxAttempts:    2,
		RetryBaseDelay: time.Second,
		RetryMaxDelay:  time.Second,
		BatchSize:      1,
	}, WorkerDeps{
		SyslogDialer: failingDialer{err: errors.New("dial failed")},
	})
	worker.now = clock.Now
	worker.syslog.Now = clock.Now

	worker.processOnce(context.Background())
	updated, err := store.Get(context.Background(), 1)
	if err != nil {
		t.Fatalf("get delivery: %v", err)
	}
	if updated.Status != DeliveryStatusRetry {
		t.Fatalf("expected retry status after first failure, got %s", updated.Status)
	}
	if store.markDLQCount != 0 {
		t.Fatalf("expected no dlq after first failure")
	}

	clock.Set(updated.NextAttemptAt)
	worker.processOnce(context.Background())
	updated, err = store.Get(context.Background(), 1)
	if err != nil {
		t.Fatalf("get delivery: %v", err)
	}
	if updated.Status != DeliveryStatusDLQ {
		t.Fatalf("expected dlq after retries, got %s", updated.Status)
	}
	if store.markDLQCount != 1 {
		t.Fatalf("expected dlq marked once, got %d", store.markDLQCount)
	}

	clock.Advance(time.Second)
	worker.processOnce(context.Background())
	if store.markDLQCount != 1 {
		t.Fatalf("expected dlq mark to remain once, got %d", store.markDLQCount)
	}
	if len(attempts.attempts) != 2 {
		t.Fatalf("expected 2 attempts recorded, got %d", len(attempts.attempts))
	}
}
