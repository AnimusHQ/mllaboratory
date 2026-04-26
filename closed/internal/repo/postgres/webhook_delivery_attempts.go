package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/animus-labs/animus-go/closed/internal/integrations/webhooks"
)

type WebhookDeliveryAttemptStore struct {
	db DB
}

const (
	insertWebhookDeliveryAttemptQuery = `INSERT INTO webhook_delivery_attempts (
			delivery_id,
			attempted_at,
			status_code,
			outcome,
			error,
			latency_ms,
			request_id,
			created_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		RETURNING id, delivery_id, attempted_at, status_code, outcome, error, latency_ms, request_id, created_at`
	listWebhookDeliveryAttemptsQuery = `SELECT id, delivery_id, attempted_at, status_code, outcome, error, latency_ms, request_id, created_at
		FROM webhook_delivery_attempts
		WHERE delivery_id = $1
		ORDER BY attempted_at DESC, id DESC
		LIMIT $2`
)

func NewWebhookDeliveryAttemptStore(db DB) *WebhookDeliveryAttemptStore {
	if db == nil {
		return nil
	}
	return &WebhookDeliveryAttemptStore{db: db}
}

func (s *WebhookDeliveryAttemptStore) Insert(ctx context.Context, attempt webhooks.Attempt) (webhooks.Attempt, error) {
	if s == nil || s.db == nil {
		return webhooks.Attempt{}, fmt.Errorf("webhook delivery attempt store not initialized")
	}
	attempt.DeliveryID = strings.TrimSpace(attempt.DeliveryID)
	attempt.Outcome = webhooks.AttemptOutcome(strings.TrimSpace(string(attempt.Outcome)))
	attempt.Error = strings.TrimSpace(attempt.Error)
	attempt.RequestID = strings.TrimSpace(attempt.RequestID)
	if attempt.DeliveryID == "" {
		return webhooks.Attempt{}, fmt.Errorf("delivery_id is required")
	}
	if attempt.Outcome == "" {
		return webhooks.Attempt{}, fmt.Errorf("outcome is required")
	}
	if attempt.AttemptedAt.IsZero() {
		attempt.AttemptedAt = time.Now().UTC()
	}
	createdAt := normalizeTime(attempt.CreatedAt)
	row := s.db.QueryRowContext(
		ctx,
		insertWebhookDeliveryAttemptQuery,
		attempt.DeliveryID,
		attempt.AttemptedAt.UTC(),
		statusCodeOrNull(attempt.StatusCode),
		string(attempt.Outcome),
		nullString(attempt.Error),
		nullableInt(attempt.LatencyMs),
		nullString(attempt.RequestID),
		createdAt,
	)
	return scanWebhookDeliveryAttempt(row)
}

func (s *WebhookDeliveryAttemptStore) List(ctx context.Context, deliveryID string, limit int) ([]webhooks.Attempt, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("webhook delivery attempt store not initialized")
	}
	deliveryID = strings.TrimSpace(deliveryID)
	if deliveryID == "" {
		return nil, fmt.Errorf("delivery_id is required")
	}
	if limit <= 0 {
		limit = 200
	}
	rows, err := s.db.QueryContext(ctx, listWebhookDeliveryAttemptsQuery, deliveryID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]webhooks.Attempt, 0)
	for rows.Next() {
		record, err := scanWebhookDeliveryAttempt(rows)
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

type attemptRowScanner interface {
	Scan(dest ...any) error
}

func scanWebhookDeliveryAttempt(row attemptRowScanner) (webhooks.Attempt, error) {
	var (
		record     webhooks.Attempt
		statusCode sql.NullInt64
		outcome    string
		errStr     sql.NullString
		latency    sql.NullInt64
		requestID  sql.NullString
		createdAt  time.Time
	)
	if err := row.Scan(&record.ID, &record.DeliveryID, &record.AttemptedAt, &statusCode, &outcome, &errStr, &latency, &requestID, &createdAt); err != nil {
		return webhooks.Attempt{}, err
	}
	if statusCode.Valid {
		value := int(statusCode.Int64)
		record.StatusCode = &value
	}
	record.Outcome = webhooks.AttemptOutcome(strings.TrimSpace(outcome))
	record.Error = strings.TrimSpace(errStr.String)
	if latency.Valid {
		record.LatencyMs = int(latency.Int64)
	}
	record.RequestID = strings.TrimSpace(requestID.String)
	record.CreatedAt = createdAt.UTC()
	return record, nil
}

func statusCodeOrNull(value *int) sql.NullInt64 {
	if value == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(*value), Valid: true}
}

func nullableInt(value int) sql.NullInt64 {
	if value == 0 {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(value), Valid: true}
}
