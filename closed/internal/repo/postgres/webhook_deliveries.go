package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/animus-labs/animus-go/closed/internal/integrations/webhooks"
)

type WebhookDeliveryStore struct {
	db DB
}

const (
	insertWebhookDeliveryQuery = `INSERT INTO webhook_deliveries (
			id,
			project_id,
			subscription_id,
			event_id,
			event_type,
			payload_jsonb,
			status,
			next_attempt_at,
			attempt_count,
			last_error,
			created_at,
			updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		ON CONFLICT (subscription_id, event_id) DO NOTHING
		RETURNING id, project_id, subscription_id, event_id, event_type, payload_jsonb, status, next_attempt_at, attempt_count, last_error, created_at, updated_at`
	claimWebhookDeliveriesQuery = `WITH cte AS (
		SELECT id
		FROM webhook_deliveries
		WHERE status = $1 AND next_attempt_at <= $2
		ORDER BY next_attempt_at ASC, created_at ASC, id ASC
		LIMIT $3
		FOR UPDATE SKIP LOCKED
	)
	UPDATE webhook_deliveries
	SET next_attempt_at = $4,
		updated_at = now()
	FROM cte
	WHERE webhook_deliveries.id = cte.id
	RETURNING webhook_deliveries.id, webhook_deliveries.project_id, webhook_deliveries.subscription_id,
		webhook_deliveries.event_id, webhook_deliveries.event_type, webhook_deliveries.payload_jsonb,
		webhook_deliveries.status, webhook_deliveries.next_attempt_at, webhook_deliveries.attempt_count,
		webhook_deliveries.last_error, webhook_deliveries.created_at, webhook_deliveries.updated_at`
	updateWebhookDeliveryQuery = `UPDATE webhook_deliveries
		SET status = $3,
			next_attempt_at = $4,
			attempt_count = $5,
			last_error = $6,
			updated_at = $7
		WHERE project_id = $1 AND id = $2
		RETURNING id, project_id, subscription_id, event_id, event_type, payload_jsonb, status, next_attempt_at, attempt_count, last_error, created_at, updated_at`
	selectWebhookDeliveryQuery = `SELECT id, project_id, subscription_id, event_id, event_type, payload_jsonb, status, next_attempt_at, attempt_count, last_error, created_at, updated_at
		FROM webhook_deliveries
		WHERE project_id = $1 AND id = $2`
	listWebhookDeliveriesQuery = `SELECT id, project_id, subscription_id, event_id, event_type, payload_jsonb, status, next_attempt_at, attempt_count, last_error, created_at, updated_at
		FROM webhook_deliveries
		WHERE project_id = $1
			AND ($2 = '' OR event_type = $2)
			AND ($3 = '' OR status = $3)
		ORDER BY created_at DESC, id DESC
		LIMIT $4`
)

func NewWebhookDeliveryStore(db DB) *WebhookDeliveryStore {
	if db == nil {
		return nil
	}
	return &WebhookDeliveryStore{db: db}
}

func (s *WebhookDeliveryStore) Enqueue(ctx context.Context, delivery webhooks.Delivery) (webhooks.Delivery, bool, error) {
	if s == nil || s.db == nil {
		return webhooks.Delivery{}, false, fmt.Errorf("webhook delivery store not initialized")
	}
	delivery.ID = strings.TrimSpace(delivery.ID)
	delivery.ProjectID = strings.TrimSpace(delivery.ProjectID)
	delivery.SubscriptionID = strings.TrimSpace(delivery.SubscriptionID)
	delivery.EventID = strings.TrimSpace(delivery.EventID)
	delivery.LastError = strings.TrimSpace(delivery.LastError)
	if delivery.ID == "" || delivery.ProjectID == "" || delivery.SubscriptionID == "" || delivery.EventID == "" {
		return webhooks.Delivery{}, false, fmt.Errorf("id, project_id, subscription_id, event_id are required")
	}
	if !delivery.EventType.Valid() {
		return webhooks.Delivery{}, false, fmt.Errorf("event_type is required")
	}
	if delivery.Status == "" {
		delivery.Status = webhooks.DeliveryStatusPending
	}
	if delivery.NextAttemptAt.IsZero() {
		delivery.NextAttemptAt = time.Now().UTC()
	}
	payload := delivery.Payload
	if len(payload) == 0 {
		payload = []byte(`{}`)
	}
	createdAt := normalizeTime(delivery.CreatedAt)
	updatedAt := normalizeTime(delivery.UpdatedAt)
	row := s.db.QueryRowContext(
		ctx,
		insertWebhookDeliveryQuery,
		delivery.ID,
		delivery.ProjectID,
		delivery.SubscriptionID,
		delivery.EventID,
		delivery.EventType.String(),
		payload,
		string(delivery.Status),
		delivery.NextAttemptAt.UTC(),
		delivery.AttemptCount,
		nullString(delivery.LastError),
		createdAt,
		updatedAt,
	)
	record, err := scanWebhookDelivery(row)
	if err != nil {
		if errorsIsNoRows(err) {
			return webhooks.Delivery{}, false, nil
		}
		return webhooks.Delivery{}, false, err
	}
	return record, true, nil
}

func (s *WebhookDeliveryStore) ClaimDue(ctx context.Context, now time.Time, limit int, hold time.Duration) ([]webhooks.Delivery, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("webhook delivery store not initialized")
	}
	if limit <= 0 {
		limit = 10
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	holdUntil := now.Add(hold)
	rows, err := s.db.QueryContext(ctx, claimWebhookDeliveriesQuery, string(webhooks.DeliveryStatusPending), now.UTC(), limit, holdUntil.UTC())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]webhooks.Delivery, 0)
	for rows.Next() {
		record, err := scanWebhookDelivery(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, record)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *WebhookDeliveryStore) Update(ctx context.Context, delivery webhooks.Delivery) (webhooks.Delivery, error) {
	if s == nil || s.db == nil {
		return webhooks.Delivery{}, fmt.Errorf("webhook delivery store not initialized")
	}
	delivery.ID = strings.TrimSpace(delivery.ID)
	delivery.ProjectID = strings.TrimSpace(delivery.ProjectID)
	delivery.LastError = strings.TrimSpace(delivery.LastError)
	if delivery.ID == "" || delivery.ProjectID == "" {
		return webhooks.Delivery{}, fmt.Errorf("id and project_id are required")
	}
	if delivery.Status == "" {
		return webhooks.Delivery{}, fmt.Errorf("status is required")
	}
	if delivery.NextAttemptAt.IsZero() {
		delivery.NextAttemptAt = time.Now().UTC()
	}
	updatedAt := normalizeTime(delivery.UpdatedAt)
	row := s.db.QueryRowContext(
		ctx,
		updateWebhookDeliveryQuery,
		delivery.ProjectID,
		delivery.ID,
		string(delivery.Status),
		delivery.NextAttemptAt.UTC(),
		delivery.AttemptCount,
		nullString(delivery.LastError),
		updatedAt,
	)
	return scanWebhookDelivery(row)
}

func (s *WebhookDeliveryStore) Get(ctx context.Context, projectID, deliveryID string) (webhooks.Delivery, error) {
	if s == nil || s.db == nil {
		return webhooks.Delivery{}, fmt.Errorf("webhook delivery store not initialized")
	}
	projectID = strings.TrimSpace(projectID)
	deliveryID = strings.TrimSpace(deliveryID)
	if projectID == "" || deliveryID == "" {
		return webhooks.Delivery{}, fmt.Errorf("project_id and delivery_id are required")
	}
	row := s.db.QueryRowContext(ctx, selectWebhookDeliveryQuery, projectID, deliveryID)
	record, err := scanWebhookDelivery(row)
	if err != nil {
		return webhooks.Delivery{}, handleNotFound(err)
	}
	return record, nil
}

func (s *WebhookDeliveryStore) List(ctx context.Context, projectID string, eventType webhooks.EventType, status webhooks.DeliveryStatus, limit int) ([]webhooks.Delivery, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("webhook delivery store not initialized")
	}
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return nil, fmt.Errorf("project_id is required")
	}
	if limit <= 0 {
		limit = 200
	}
	var eventTypeValue string
	if eventType.Valid() {
		eventTypeValue = eventType.String()
	}
	statusValue := strings.TrimSpace(string(status))
	rows, err := s.db.QueryContext(ctx, listWebhookDeliveriesQuery, projectID, eventTypeValue, statusValue, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]webhooks.Delivery, 0)
	for rows.Next() {
		record, err := scanWebhookDelivery(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, record)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

type deliveryRowScanner interface {
	Scan(dest ...any) error
}

func scanWebhookDelivery(row deliveryRowScanner) (webhooks.Delivery, error) {
	var (
		record    webhooks.Delivery
		eventType string
		status    string
		payload   []byte
		lastError sql.NullString
		createdAt time.Time
		updatedAt time.Time
	)
	if err := row.Scan(&record.ID, &record.ProjectID, &record.SubscriptionID, &record.EventID, &eventType, &payload, &status, &record.NextAttemptAt, &record.AttemptCount, &lastError, &createdAt, &updatedAt); err != nil {
		return webhooks.Delivery{}, err
	}
	record.EventType = webhooks.EventType(strings.TrimSpace(eventType))
	record.Status = webhooks.DeliveryStatus(strings.TrimSpace(status))
	record.LastError = strings.TrimSpace(lastError.String)
	record.Payload = payload
	record.CreatedAt = createdAt.UTC()
	record.UpdatedAt = updatedAt.UTC()
	return record, nil
}

func errorsIsNoRows(err error) bool {
	return errors.Is(err, sql.ErrNoRows)
}
