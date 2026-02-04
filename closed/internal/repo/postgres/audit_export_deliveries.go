package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/animus-labs/animus-go/closed/internal/auditexport"
)

type AuditExportDeliveryStore struct {
	db DB
}

const (
	backfillAuditExportDeliveriesQuery = `INSERT INTO audit_export_deliveries (event_id, sink_id, status, next_attempt_at, created_at, updated_at)
		SELECT event_id, $1, $2, now(), now(), now()
		FROM audit_events
		WHERE action NOT LIKE 'audit.export.%'
		ON CONFLICT (sink_id, event_id) DO NOTHING`
	claimAuditExportDeliveriesQuery = `WITH cte AS (
		SELECT delivery_id
		FROM audit_export_deliveries
		WHERE status IN ($1,$2,$3)
			AND next_attempt_at <= $4
		ORDER BY next_attempt_at ASC, created_at ASC, delivery_id ASC
		FOR UPDATE SKIP LOCKED
		LIMIT $5
	)
	UPDATE audit_export_deliveries d
	SET status = $6,
		attempt_count = d.attempt_count + 1,
		updated_at = $4,
		next_attempt_at = $7
	FROM cte
	WHERE d.delivery_id = cte.delivery_id
	RETURNING d.delivery_id, d.sink_id, d.event_id, d.status, d.attempt_count, d.next_attempt_at,
		d.last_error, d.dlq_reason, d.delivered_at, d.created_at, d.updated_at`
	selectAuditExportDeliveryQuery = `SELECT delivery_id, sink_id, event_id, status, attempt_count, next_attempt_at, last_error,
		dlq_reason, delivered_at, created_at, updated_at
		FROM audit_export_deliveries
		WHERE delivery_id = $1`
	listAuditExportDeliveriesQuery = `SELECT delivery_id, sink_id, event_id, status, attempt_count, next_attempt_at, last_error,
		dlq_reason, delivered_at, created_at, updated_at
		FROM audit_export_deliveries
		WHERE ($1 = '' OR status = $1)
			AND ($2 = '' OR sink_id = $2)
		ORDER BY created_at DESC, delivery_id DESC
		LIMIT $3`
	markAuditExportDeliveryDeliveredQuery = `UPDATE audit_export_deliveries
		SET status = $1,
			delivered_at = $2,
			updated_at = $2,
			last_error = NULL,
			dlq_reason = NULL
		WHERE delivery_id = $3`
	markAuditExportDeliveryRetryQuery = `UPDATE audit_export_deliveries
		SET status = $1,
			next_attempt_at = $2,
			last_error = $3,
			updated_at = $2
		WHERE delivery_id = $4`
	markAuditExportDeliveryDLQQuery = `UPDATE audit_export_deliveries
		SET status = $1,
			dlq_reason = $2,
			last_error = $3,
			updated_at = $4
		WHERE delivery_id = $5`
	countAuditExportDeliveriesByStatusQuery = `SELECT COUNT(*) FROM audit_export_deliveries WHERE status = $1`
)

func NewAuditExportDeliveryStore(db DB) *AuditExportDeliveryStore {
	if db == nil {
		return nil
	}
	return &AuditExportDeliveryStore{db: db}
}

func (s *AuditExportDeliveryStore) Backfill(ctx context.Context, sinkID string) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("audit export delivery store not initialized")
	}
	sinkID = strings.TrimSpace(sinkID)
	if sinkID == "" {
		return fmt.Errorf("sink_id is required")
	}
	_, err := s.db.ExecContext(ctx, backfillAuditExportDeliveriesQuery, sinkID, string(auditexport.DeliveryStatusPending))
	if err != nil {
		return fmt.Errorf("backfill audit export deliveries: %w", err)
	}
	return nil
}

func (s *AuditExportDeliveryStore) ClaimDue(ctx context.Context, now time.Time, inflightTimeout time.Duration, limit int) ([]auditexport.Delivery, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("audit export delivery store not initialized")
	}
	if limit <= 0 {
		limit = 50
	}
	if inflightTimeout <= 0 {
		inflightTimeout = 2 * time.Minute
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	holdUntil := now.Add(inflightTimeout)
	rows, err := s.db.QueryContext(
		ctx,
		claimAuditExportDeliveriesQuery,
		string(auditexport.DeliveryStatusPending),
		string(auditexport.DeliveryStatusRetry),
		string(auditexport.DeliveryStatusInflight),
		now.UTC(),
		limit,
		string(auditexport.DeliveryStatusInflight),
		holdUntil.UTC(),
	)
	if err != nil {
		return nil, fmt.Errorf("claim audit export deliveries: %w", err)
	}
	defer rows.Close()

	out := make([]auditexport.Delivery, 0)
	for rows.Next() {
		record, err := scanAuditExportDelivery(rows)
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

func (s *AuditExportDeliveryStore) MarkDelivered(ctx context.Context, deliveryID int64, deliveredAt time.Time) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("audit export delivery store not initialized")
	}
	if deliveryID <= 0 {
		return fmt.Errorf("delivery_id is required")
	}
	at := normalizeTime(deliveredAt)
	_, err := s.db.ExecContext(ctx, markAuditExportDeliveryDeliveredQuery, string(auditexport.DeliveryStatusDelivered), at, deliveryID)
	if err != nil {
		return fmt.Errorf("mark audit export delivered: %w", err)
	}
	return nil
}

func (s *AuditExportDeliveryStore) MarkRetry(ctx context.Context, deliveryID int64, lastError string, nextAttemptAt time.Time) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("audit export delivery store not initialized")
	}
	if deliveryID <= 0 {
		return fmt.Errorf("delivery_id is required")
	}
	lastError = strings.TrimSpace(lastError)
	if lastError == "" {
		lastError = "export_failed"
	}
	next := normalizeTime(nextAttemptAt)
	_, err := s.db.ExecContext(ctx, markAuditExportDeliveryRetryQuery, string(auditexport.DeliveryStatusRetry), next, lastError, deliveryID)
	if err != nil {
		return fmt.Errorf("mark audit export retry: %w", err)
	}
	return nil
}

func (s *AuditExportDeliveryStore) MarkDLQ(ctx context.Context, deliveryID int64, reason string, lastError string, at time.Time) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("audit export delivery store not initialized")
	}
	if deliveryID <= 0 {
		return fmt.Errorf("delivery_id is required")
	}
	reason = strings.TrimSpace(reason)
	lastError = strings.TrimSpace(lastError)
	if reason == "" {
		reason = "export_failed"
	}
	if lastError == "" {
		lastError = "export_failed"
	}
	updated := normalizeTime(at)
	_, err := s.db.ExecContext(ctx, markAuditExportDeliveryDLQQuery, string(auditexport.DeliveryStatusDLQ), reason, lastError, updated, deliveryID)
	if err != nil {
		return fmt.Errorf("mark audit export dlq: %w", err)
	}
	return nil
}

func (s *AuditExportDeliveryStore) Get(ctx context.Context, deliveryID int64) (auditexport.Delivery, error) {
	if s == nil || s.db == nil {
		return auditexport.Delivery{}, fmt.Errorf("audit export delivery store not initialized")
	}
	if deliveryID <= 0 {
		return auditexport.Delivery{}, fmt.Errorf("delivery_id is required")
	}
	row := s.db.QueryRowContext(ctx, selectAuditExportDeliveryQuery, deliveryID)
	record, err := scanAuditExportDelivery(row)
	if err != nil {
		return auditexport.Delivery{}, handleNotFound(err)
	}
	return record, nil
}

func (s *AuditExportDeliveryStore) List(ctx context.Context, status string, sinkID string, limit int) ([]auditexport.Delivery, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("audit export delivery store not initialized")
	}
	status = strings.TrimSpace(status)
	sinkID = strings.TrimSpace(sinkID)
	if limit <= 0 {
		limit = 200
	}
	rows, err := s.db.QueryContext(ctx, listAuditExportDeliveriesQuery, status, sinkID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]auditexport.Delivery, 0)
	for rows.Next() {
		record, err := scanAuditExportDelivery(rows)
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

func (s *AuditExportDeliveryStore) CountByStatus(ctx context.Context, status auditexport.DeliveryStatus) (int, error) {
	if s == nil || s.db == nil {
		return 0, fmt.Errorf("audit export delivery store not initialized")
	}
	if !status.Valid() {
		return 0, fmt.Errorf("status is required")
	}
	row := s.db.QueryRowContext(ctx, countAuditExportDeliveriesByStatusQuery, string(status))
	var count int
	if err := row.Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

type auditExportDeliveryScanner interface {
	Scan(dest ...any) error
}

func scanAuditExportDelivery(row auditExportDeliveryScanner) (auditexport.Delivery, error) {
	var (
		record      auditexport.Delivery
		status      string
		lastError   sql.NullString
		dlqReason   sql.NullString
		deliveredAt sql.NullTime
		createdAt   time.Time
		updatedAt   time.Time
	)
	if err := row.Scan(
		&record.DeliveryID,
		&record.SinkID,
		&record.EventID,
		&status,
		&record.AttemptCount,
		&record.NextAttemptAt,
		&lastError,
		&dlqReason,
		&deliveredAt,
		&createdAt,
		&updatedAt,
	); err != nil {
		return auditexport.Delivery{}, err
	}
	record.Status = auditexport.DeliveryStatus(strings.TrimSpace(status))
	record.LastError = strings.TrimSpace(lastError.String)
	record.DLQReason = strings.TrimSpace(dlqReason.String)
	if deliveredAt.Valid {
		at := deliveredAt.Time.UTC()
		record.DeliveredAt = &at
	}
	record.CreatedAt = createdAt.UTC()
	record.UpdatedAt = updatedAt.UTC()
	record.NextAttemptAt = record.NextAttemptAt.UTC()
	return record, nil
}
