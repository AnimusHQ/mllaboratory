package auditexport

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/animus-labs/animus-go/closed/internal/domain"
)

type stubDeliveryStore struct {
	deliveries  []Delivery
	claimed     bool
	markDLQ     bool
	markDeliver bool
	markRetry   bool
	retryError  string
}

func (s *stubDeliveryStore) Backfill(ctx context.Context, sinkID string) error { return nil }
func (s *stubDeliveryStore) ClaimDue(ctx context.Context, now time.Time, inflightTimeout time.Duration, limit int) ([]Delivery, error) {
	if s.claimed {
		return nil, nil
	}
	s.claimed = true
	return s.deliveries, nil
}
func (s *stubDeliveryStore) MarkDelivered(ctx context.Context, deliveryID int64, deliveredAt time.Time) error {
	s.markDeliver = true
	return nil
}
func (s *stubDeliveryStore) MarkRetry(ctx context.Context, deliveryID int64, lastError string, nextAttemptAt time.Time) error {
	s.markRetry = true
	s.retryError = lastError
	return nil
}
func (s *stubDeliveryStore) MarkDLQ(ctx context.Context, deliveryID int64, reason string, lastError string, at time.Time) error {
	s.markDLQ = true
	return nil
}
func (s *stubDeliveryStore) Replay(ctx context.Context, deliveryID int64, scheduledAt time.Time) error {
	return nil
}
func (s *stubDeliveryStore) Get(ctx context.Context, deliveryID int64) (Delivery, error) {
	return Delivery{}, nil
}
func (s *stubDeliveryStore) List(ctx context.Context, status string, sinkID string, limit int) ([]Delivery, error) {
	return nil, nil
}
func (s *stubDeliveryStore) CountByStatus(ctx context.Context, status DeliveryStatus) (int, error) {
	return 0, nil
}

type stubAttemptStore struct {
	inserted bool
	last     DeliveryAttempt
}

func (s *stubAttemptStore) Insert(ctx context.Context, attempt DeliveryAttempt) (DeliveryAttempt, error) {
	s.inserted = true
	s.last = attempt
	return attempt, nil
}
func (s *stubAttemptStore) List(ctx context.Context, deliveryID int64, limit int) ([]DeliveryAttempt, error) {
	return nil, nil
}

type stubSinkStore struct {
	sink Sink
}

func (s *stubSinkStore) UpsertSink(ctx context.Context, sink Sink) (Sink, error)  { return sink, nil }
func (s *stubSinkStore) GetSink(ctx context.Context, sinkID string) (Sink, error) { return s.sink, nil }
func (s *stubSinkStore) ListSinks(ctx context.Context, limit int) ([]Sink, error) {
	return []Sink{s.sink}, nil
}

type stubEventStore struct {
	event domain.AuditEvent
	err   error
}

func (s *stubEventStore) GetEvent(ctx context.Context, eventID int64) (domain.AuditEvent, error) {
	if s.err != nil {
		return domain.AuditEvent{}, s.err
	}
	return s.event, nil
}

func TestWorkerMarksDLQAfterMaxAttempts(t *testing.T) {
	cfg := Config{Destination: "webhook", MaxAttempts: 1, RetryBaseDelay: time.Second, RetryMaxDelay: time.Second}
	payloadCfg := SinkConfig{WebhookURL: "https://example.test"}
	cfgBlob, _ := json.Marshal(payloadCfg)
	deliveryStore := &stubDeliveryStore{deliveries: []Delivery{{DeliveryID: 1, SinkID: "sink", EventID: 10, AttemptCount: 1}}}
	attemptStore := &stubAttemptStore{}
	sinkStore := &stubSinkStore{sink: Sink{SinkID: "sink", Destination: "webhook", Format: "ndjson", Config: cfgBlob, Enabled: true}}
	eventStore := &stubEventStore{event: domain.AuditEvent{EventID: 10, OccurredAt: time.Now().UTC()}}
	client := &stubHTTPDoer{resp: &http.Response{StatusCode: 500, Body: http.NoBody}}

	worker := NewWorker(deliveryStore, attemptStore, sinkStore, eventStore, nil, nil, cfg, WorkerDeps{HTTPClient: client})
	worker.processOnce(context.Background())

	if !deliveryStore.markDLQ {
		t.Fatalf("expected delivery to be marked DLQ")
	}
	if !attemptStore.inserted {
		t.Fatalf("expected attempt inserted")
	}
}

func TestWorkerMarksDeliveredOnSuccess(t *testing.T) {
	cfg := Config{Destination: "webhook", MaxAttempts: 3}
	payloadCfg := SinkConfig{WebhookURL: "https://example.test"}
	cfgBlob, _ := json.Marshal(payloadCfg)
	deliveryStore := &stubDeliveryStore{deliveries: []Delivery{{DeliveryID: 2, SinkID: "sink", EventID: 11, AttemptCount: 1}}}
	attemptStore := &stubAttemptStore{}
	sinkStore := &stubSinkStore{sink: Sink{SinkID: "sink", Destination: "webhook", Format: "ndjson", Config: cfgBlob, Enabled: true}}
	eventStore := &stubEventStore{event: domain.AuditEvent{EventID: 11, OccurredAt: time.Now().UTC()}}
	client := &stubHTTPDoer{resp: &http.Response{StatusCode: 200, Body: http.NoBody}}

	worker := NewWorker(deliveryStore, attemptStore, sinkStore, eventStore, nil, nil, cfg, WorkerDeps{HTTPClient: client})
	worker.processOnce(context.Background())

	if !deliveryStore.markDeliver {
		t.Fatalf("expected delivery to be marked delivered")
	}
	if !attemptStore.inserted {
		t.Fatalf("expected attempt inserted")
	}
}

func TestWorkerRetriesWhenEventUnavailable(t *testing.T) {
	cfg := Config{Destination: "webhook", MaxAttempts: 3, RetryBaseDelay: time.Second, RetryMaxDelay: time.Second}
	payloadCfg := SinkConfig{WebhookURL: "https://example.test"}
	cfgBlob, _ := json.Marshal(payloadCfg)
	deliveryStore := &stubDeliveryStore{deliveries: []Delivery{{DeliveryID: 3, SinkID: "sink", EventID: 12, AttemptCount: 0}}}
	attemptStore := &stubAttemptStore{}
	sinkStore := &stubSinkStore{sink: Sink{SinkID: "sink", Destination: "webhook", Format: "ndjson", Config: cfgBlob, Enabled: true}}
	eventStore := &stubEventStore{err: errors.New("db timeout")}

	worker := NewWorker(deliveryStore, attemptStore, sinkStore, eventStore, nil, nil, cfg, WorkerDeps{})
	worker.processOnce(context.Background())

	if !deliveryStore.markRetry {
		t.Fatalf("expected delivery to be scheduled for retry")
	}
	if deliveryStore.markDLQ || deliveryStore.markDeliver {
		t.Fatalf("expected retry only, got dlq=%v delivered=%v", deliveryStore.markDLQ, deliveryStore.markDeliver)
	}
}
