package auditexport

import (
	"context"
	"time"

	"github.com/animus-labs/animus-go/closed/internal/domain"
)

type SinkStore interface {
	UpsertSink(ctx context.Context, sink Sink) (Sink, error)
	GetSink(ctx context.Context, sinkID string) (Sink, error)
	ListSinks(ctx context.Context, limit int) ([]Sink, error)
}

type DeliveryStore interface {
	Backfill(ctx context.Context, sinkID string) error
	ClaimDue(ctx context.Context, now time.Time, inflightTimeout time.Duration, limit int) ([]Delivery, error)
	MarkDelivered(ctx context.Context, deliveryID int64, deliveredAt time.Time) error
	MarkRetry(ctx context.Context, deliveryID int64, lastError string, nextAttemptAt time.Time) error
	MarkDLQ(ctx context.Context, deliveryID int64, reason string, lastError string, at time.Time) error
	Replay(ctx context.Context, deliveryID int64, scheduledAt time.Time) error
	Get(ctx context.Context, deliveryID int64) (Delivery, error)
	List(ctx context.Context, status string, sinkID string, limit int) ([]Delivery, error)
	CountByStatus(ctx context.Context, status DeliveryStatus) (int, error)
}

type AttemptStore interface {
	Insert(ctx context.Context, attempt DeliveryAttempt) (DeliveryAttempt, error)
	List(ctx context.Context, deliveryID int64, limit int) ([]DeliveryAttempt, error)
}

type ReplayStore interface {
	Insert(ctx context.Context, deliveryID int64, token string, requestedAt time.Time) (bool, error)
}

type EventStore interface {
	GetEvent(ctx context.Context, eventID int64) (domain.AuditEvent, error)
}
