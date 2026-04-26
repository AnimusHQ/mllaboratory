package postgres

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type WebhookDeliveryReplayStore struct {
	db DB
}

const (
	insertWebhookDeliveryReplayQuery = `INSERT INTO webhook_delivery_replays (
			delivery_id,
			replay_token,
			requested_at
		) VALUES ($1,$2,$3)
		ON CONFLICT (delivery_id, replay_token) DO NOTHING`
)

func NewWebhookDeliveryReplayStore(db DB) *WebhookDeliveryReplayStore {
	if db == nil {
		return nil
	}
	return &WebhookDeliveryReplayStore{db: db}
}

func (s *WebhookDeliveryReplayStore) Insert(ctx context.Context, deliveryID, token string, requestedAt time.Time) (bool, error) {
	if s == nil || s.db == nil {
		return false, fmt.Errorf("webhook delivery replay store not initialized")
	}
	deliveryID = strings.TrimSpace(deliveryID)
	token = strings.TrimSpace(token)
	if deliveryID == "" || token == "" {
		return false, fmt.Errorf("delivery_id and replay_token are required")
	}
	if requestedAt.IsZero() {
		requestedAt = time.Now().UTC()
	}
	res, err := s.db.ExecContext(ctx, insertWebhookDeliveryReplayQuery, deliveryID, token, requestedAt.UTC())
	if err != nil {
		return false, err
	}
	rows, _ := res.RowsAffected()
	return rows > 0, nil
}
