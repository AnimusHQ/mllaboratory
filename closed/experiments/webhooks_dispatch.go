package main

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/animus-labs/animus-go/closed/internal/integrations/webhooks"
	repopg "github.com/animus-labs/animus-go/closed/internal/repo/postgres"
	"github.com/google/uuid"
)

func (api *experimentsAPI) enqueueWebhookRunFinished(ctx context.Context, actor, requestID, projectID, runID string, emittedAt time.Time) error {
	payload, err := webhooks.RunFinishedPayload(projectID, runID, emittedAt)
	if err != nil {
		return err
	}
	return api.enqueueWebhookPayload(ctx, actor, requestID, payload)
}

func (api *experimentsAPI) enqueueWebhookPayload(ctx context.Context, actor, requestID string, payload webhooks.Payload) error {
	if api == nil || api.db == nil {
		return nil
	}
	if !api.webhookConfig.Enabled() {
		return nil
	}
	projectID := strings.TrimSpace(payload.ProjectID)
	if projectID == "" || strings.TrimSpace(payload.EventID) == "" || !payload.EventType.Valid() {
		return errors.New("invalid webhook payload")
	}

	subStore := repopg.NewWebhookSubscriptionStore(api.db)
	deliveryStore := repopg.NewWebhookDeliveryStore(api.db)
	if subStore == nil || deliveryStore == nil {
		return errors.New("webhook store unavailable")
	}

	subs, err := subStore.ListEnabledByEvent(ctx, projectID, payload.EventType)
	if err != nil {
		return err
	}
	if len(subs) == 0 {
		return nil
	}

	payloadJSON, err := webhooks.PayloadJSON(payload)
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	var lastErr error
	for _, sub := range subs {
		delivery := webhooks.Delivery{
			ID:             uuid.NewString(),
			ProjectID:      projectID,
			SubscriptionID: sub.ID,
			EventID:        payload.EventID,
			EventType:      payload.EventType,
			Payload:        payloadJSON,
			Status:         webhooks.DeliveryStatusPending,
			NextAttemptAt:  now,
			AttemptCount:   0,
			CreatedAt:      now,
			UpdatedAt:      now,
		}
		record, inserted, err := deliveryStore.Enqueue(ctx, delivery)
		if err != nil {
			lastErr = err
			continue
		}
		if inserted {
			api.appendWebhookAuditWithContext(ctx, requestID, auditWebhookDeliveryEnqueued, actor, map[string]any{
				"project_id":      projectID,
				"delivery_id":     record.ID,
				"subscription_id": record.SubscriptionID,
				"event_id":        record.EventID,
				"event_type":      record.EventType.String(),
			})
		}
	}
	return lastErr
}
