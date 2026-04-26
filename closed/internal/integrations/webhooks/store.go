package webhooks

import (
	"context"
	"time"
)

type SubscriptionStore interface {
	Get(ctx context.Context, projectID, subscriptionID string) (Subscription, error)
}

type DeliveryStore interface {
	ClaimDue(ctx context.Context, now time.Time, limit int, hold time.Duration) ([]Delivery, error)
	Update(ctx context.Context, delivery Delivery) (Delivery, error)
}

type AttemptStore interface {
	Insert(ctx context.Context, attempt Attempt) (Attempt, error)
}
