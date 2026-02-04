package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/animus-labs/animus-go/closed/internal/auditexport"
)

type AuditExportAttemptStore struct {
	db DB
}

const (
	insertAuditExportAttemptQuery = `INSERT INTO audit_export_attempts (
			delivery_id,
			attempted_at,
			outcome,
			status_code,
			error,
			latency_ms,
			created_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7)
		RETURNING attempt_id, delivery_id, attempted_at, outcome, status_code, error, latency_ms, created_at`
	listAuditExportAttemptsQuery = `SELECT attempt_id, delivery_id, attempted_at, outcome, status_code, error, latency_ms, created_at
		FROM audit_export_attempts
		WHERE delivery_id = $1
		ORDER BY attempted_at DESC, attempt_id DESC
		LIMIT $2`
)

func NewAuditExportAttemptStore(db DB) *AuditExportAttemptStore {
	if db == nil {
		return nil
	}
	return &AuditExportAttemptStore{db: db}
}

func (s *AuditExportAttemptStore) Insert(ctx context.Context, attempt auditexport.DeliveryAttempt) (auditexport.DeliveryAttempt, error) {
	if s == nil || s.db == nil {
		return auditexport.DeliveryAttempt{}, fmt.Errorf("audit export attempt store not initialized")
	}
	if attempt.DeliveryID <= 0 {
		return auditexport.DeliveryAttempt{}, fmt.Errorf("delivery_id is required")
	}
	attempt.Error = strings.TrimSpace(attempt.Error)
	attempt.Outcome = auditexport.AttemptOutcome(strings.TrimSpace(string(attempt.Outcome)))
	if !attempt.Outcome.Valid() {
		return auditexport.DeliveryAttempt{}, fmt.Errorf("outcome is required")
	}
	if attempt.AttemptedAt.IsZero() {
		attempt.AttemptedAt = time.Now().UTC()
	}
	createdAt := normalizeTime(attempt.CreatedAt)
	row := s.db.QueryRowContext(
		ctx,
		insertAuditExportAttemptQuery,
		attempt.DeliveryID,
		attempt.AttemptedAt.UTC(),
		string(attempt.Outcome),
		statusCodeOrNullInt(attempt.StatusCode),
		nullString(attempt.Error),
		nullableInt(attempt.LatencyMs),
		createdAt,
	)
	return scanAuditExportAttempt(row)
}

func (s *AuditExportAttemptStore) List(ctx context.Context, deliveryID int64, limit int) ([]auditexport.DeliveryAttempt, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("audit export attempt store not initialized")
	}
	if deliveryID <= 0 {
		return nil, fmt.Errorf("delivery_id is required")
	}
	if limit <= 0 {
		limit = 200
	}
	rows, err := s.db.QueryContext(ctx, listAuditExportAttemptsQuery, deliveryID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]auditexport.DeliveryAttempt, 0)
	for rows.Next() {
		record, err := scanAuditExportAttempt(rows)
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

type auditExportAttemptScanner interface {
	Scan(dest ...any) error
}

func scanAuditExportAttempt(row auditExportAttemptScanner) (auditexport.DeliveryAttempt, error) {
	var (
		record     auditexport.DeliveryAttempt
		outcome    string
		statusCode sql.NullInt64
		errStr     sql.NullString
		latency    sql.NullInt64
		createdAt  time.Time
	)
	if err := row.Scan(
		&record.AttemptID,
		&record.DeliveryID,
		&record.AttemptedAt,
		&outcome,
		&statusCode,
		&errStr,
		&latency,
		&createdAt,
	); err != nil {
		return auditexport.DeliveryAttempt{}, err
	}
	if statusCode.Valid {
		value := int(statusCode.Int64)
		record.StatusCode = &value
	}
	record.Outcome = auditexport.AttemptOutcome(strings.TrimSpace(outcome))
	record.Error = strings.TrimSpace(errStr.String)
	if latency.Valid {
		record.LatencyMs = int(latency.Int64)
	}
	record.CreatedAt = createdAt.UTC()
	return record, nil
}

func statusCodeOrNullInt(value *int) sql.NullInt64 {
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
