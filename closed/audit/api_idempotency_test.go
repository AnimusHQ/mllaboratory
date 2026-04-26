package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/animus-labs/animus-go/closed/internal/auditexport"
)

type stubExportReplayStore struct {
	mu     sync.Mutex
	tokens map[string]struct{}
}

func newStubExportReplayStore() *stubExportReplayStore {
	return &stubExportReplayStore{tokens: map[string]struct{}{}}
}

func (s *stubExportReplayStore) Insert(ctx context.Context, deliveryID int64, token string, requestedAt time.Time) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := token
	if _, ok := s.tokens[key]; ok {
		return false, nil
	}
	s.tokens[key] = struct{}{}
	return true, nil
}

type stubExportDeliveryStore struct {
	mu       sync.Mutex
	replayed int
}

func (s *stubExportDeliveryStore) Backfill(ctx context.Context, sinkID string) error {
	return nil
}

func (s *stubExportDeliveryStore) ClaimDue(ctx context.Context, now time.Time, inflightTimeout time.Duration, limit int) ([]auditexport.Delivery, error) {
	return nil, nil
}

func (s *stubExportDeliveryStore) MarkDelivered(ctx context.Context, deliveryID int64, deliveredAt time.Time) error {
	return nil
}

func (s *stubExportDeliveryStore) MarkRetry(ctx context.Context, deliveryID int64, lastError string, nextAttemptAt time.Time) error {
	return nil
}

func (s *stubExportDeliveryStore) MarkDLQ(ctx context.Context, deliveryID int64, reason string, lastError string, at time.Time) error {
	return nil
}

func (s *stubExportDeliveryStore) Replay(ctx context.Context, deliveryID int64, scheduledAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.replayed++
	return nil
}

func (s *stubExportDeliveryStore) Get(ctx context.Context, deliveryID int64) (auditexport.Delivery, error) {
	return auditexport.Delivery{}, nil
}

func (s *stubExportDeliveryStore) List(ctx context.Context, status string, sinkID string, limit int) ([]auditexport.Delivery, error) {
	return nil, nil
}

func (s *stubExportDeliveryStore) CountByStatus(ctx context.Context, status auditexport.DeliveryStatus) (int, error) {
	return 0, nil
}

func TestReplayExportDeliveryIdempotent(t *testing.T) {
	replayStore := newStubExportReplayStore()
	deliveryStore := &stubExportDeliveryStore{}
	api := &auditAPI{
		deliveries: deliveryStore,
		replays:    replayStore,
	}

	body := `{"replay_token":"tok-1"}`
	req := httptest.NewRequest(http.MethodPost, "/admin/audit/exports/dlq/1:replay", strings.NewReader(body))
	req.SetPathValue("delivery_id", "1")
	w := httptest.NewRecorder()
	api.handleReplayExportDelivery(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", w.Code)
	}
	if deliveryStore.replayed != 1 {
		t.Fatalf("expected replay scheduled once, got %d", deliveryStore.replayed)
	}

	req = httptest.NewRequest(http.MethodPost, "/admin/audit/exports/dlq/1:replay", strings.NewReader(body))
	req.SetPathValue("delivery_id", "1")
	w = httptest.NewRecorder()
	api.handleReplayExportDelivery(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202 on idempotent replay, got %d", w.Code)
	}
	if deliveryStore.replayed != 1 {
		t.Fatalf("expected replay to remain once, got %d", deliveryStore.replayed)
	}
}
