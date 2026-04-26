package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/animus-labs/animus-go/closed/internal/auditexport"
	"github.com/animus-labs/animus-go/closed/internal/domain"
)

type AuditExportStore struct {
	db DB
}

const (
	upsertAuditExportSinkQuery = `INSERT INTO audit_export_sinks (
		sink_id, name, destination, format, config, enabled, created_at, updated_at
	) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
	ON CONFLICT (sink_id) DO UPDATE SET
		name = EXCLUDED.name,
		destination = EXCLUDED.destination,
		format = EXCLUDED.format,
		config = EXCLUDED.config,
		enabled = EXCLUDED.enabled,
		updated_at = EXCLUDED.updated_at
	RETURNING sink_id, name, destination, format, config, enabled, created_at, updated_at`

	backfillOutboxQuery = `INSERT INTO audit_export_outbox (event_id, sink_id, status, next_attempt_at, created_at, updated_at)
		SELECT event_id, $1, $2, now(), now(), now()
		FROM audit_events
		WHERE action NOT LIKE 'audit.export.%'
		ON CONFLICT (event_id, sink_id) DO NOTHING`

	claimOutboxQuery = `WITH cte AS (
		SELECT o.outbox_id
		FROM audit_export_outbox o
		JOIN audit_export_sinks s ON s.sink_id = o.sink_id
		WHERE s.enabled = true
		  AND o.status IN ($1,$2,$3)
		  AND o.next_attempt_at <= $4
		ORDER BY o.next_attempt_at, o.outbox_id
		FOR UPDATE SKIP LOCKED
		LIMIT $5
	)
	UPDATE audit_export_outbox o
	SET status = $6,
		last_attempt_at = $4,
		attempt_count = o.attempt_count + 1,
		updated_at = $4,
		next_attempt_at = $7
	FROM cte
	WHERE o.outbox_id = cte.outbox_id
	RETURNING o.outbox_id, o.event_id, o.sink_id, o.attempt_count`

	selectAuditEventQuery = `SELECT event_id, occurred_at, actor, action, resource_type, resource_id, request_id, ip, user_agent, payload, integrity_sha256
		FROM audit_events
		WHERE event_id = $1`

	selectAuditExportSinkQuery = `SELECT sink_id, name, destination, format, config, enabled, created_at, updated_at
		FROM audit_export_sinks
		WHERE sink_id = $1`
	listAuditExportSinksQuery = `SELECT sink_id, name, destination, format, config, enabled, created_at, updated_at
		FROM audit_export_sinks
		ORDER BY sink_id ASC
		LIMIT $1`

	markOutboxDeliveredQuery = `UPDATE audit_export_outbox
		SET status = $1, delivered_at = $2, updated_at = $2, last_error = NULL
		WHERE outbox_id = $3`

	markOutboxFailedQuery = `UPDATE audit_export_outbox
		SET status = $1, next_attempt_at = $2, last_error = $3, updated_at = $2
		WHERE outbox_id = $4`
)

func NewAuditExportStore(db DB) *AuditExportStore {
	if db == nil {
		return nil
	}
	return &AuditExportStore{db: db}
}

func (s *AuditExportStore) UpsertSink(ctx context.Context, sink auditexport.Sink) (auditexport.Sink, error) {
	if s == nil || s.db == nil {
		return auditexport.Sink{}, fmt.Errorf("audit export store not initialized")
	}
	sink.SinkID = strings.TrimSpace(sink.SinkID)
	sink.Name = strings.TrimSpace(sink.Name)
	sink.Destination = strings.TrimSpace(sink.Destination)
	sink.Format = strings.TrimSpace(sink.Format)
	if sink.SinkID == "" || sink.Name == "" || sink.Destination == "" || sink.Format == "" {
		return auditexport.Sink{}, fmt.Errorf("sink_id, name, destination, format are required")
	}
	config := sink.Config
	if config == nil {
		config = json.RawMessage(`{}`)
	}
	createdAt := normalizeTime(sink.CreatedAt)
	updatedAt := normalizeTime(sink.UpdatedAt)
	row := s.db.QueryRowContext(ctx, upsertAuditExportSinkQuery, sink.SinkID, sink.Name, sink.Destination, sink.Format, config, sink.Enabled, createdAt, updatedAt)
	var out auditexport.Sink
	if err := row.Scan(&out.SinkID, &out.Name, &out.Destination, &out.Format, &out.Config, &out.Enabled, &out.CreatedAt, &out.UpdatedAt); err != nil {
		return auditexport.Sink{}, fmt.Errorf("upsert audit export sink: %w", err)
	}
	return out, nil
}

func (s *AuditExportStore) BackfillOutbox(ctx context.Context, sinkID string) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("audit export store not initialized")
	}
	sinkID = strings.TrimSpace(sinkID)
	if sinkID == "" {
		return fmt.Errorf("sink_id is required")
	}
	_, err := s.db.ExecContext(ctx, backfillOutboxQuery, sinkID, auditexport.OutboxStatusPending)
	if err != nil {
		return fmt.Errorf("backfill audit export outbox: %w", err)
	}
	return nil
}

func (s *AuditExportStore) ClaimDue(ctx context.Context, now time.Time, inflightTimeout time.Duration, limit int) ([]auditexport.OutboxRecord, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("audit export store not initialized")
	}
	if limit <= 0 {
		limit = 50
	}
	if inflightTimeout <= 0 {
		inflightTimeout = 2 * time.Minute
	}
	now = normalizeTime(now)
	next := now.Add(inflightTimeout)
	rows, err := s.db.QueryContext(ctx, claimOutboxQuery,
		auditexport.OutboxStatusPending,
		auditexport.OutboxStatusRetry,
		auditexport.OutboxStatusInflight,
		now,
		limit,
		auditexport.OutboxStatusInflight,
		next,
	)
	if err != nil {
		return nil, fmt.Errorf("claim audit export outbox: %w", err)
	}
	defer rows.Close()

	out := make([]auditexport.OutboxRecord, 0)
	for rows.Next() {
		var rec auditexport.OutboxRecord
		if err := rows.Scan(&rec.OutboxID, &rec.EventID, &rec.SinkID, &rec.Attempt); err != nil {
			return nil, fmt.Errorf("scan audit export outbox: %w", err)
		}
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("claim audit export outbox: %w", err)
	}
	return out, nil
}

func (s *AuditExportStore) GetEvent(ctx context.Context, eventID int64) (domain.AuditEvent, error) {
	if s == nil || s.db == nil {
		return domain.AuditEvent{}, fmt.Errorf("audit export store not initialized")
	}
	if eventID <= 0 {
		return domain.AuditEvent{}, fmt.Errorf("event_id is required")
	}
	row := s.db.QueryRowContext(ctx, selectAuditEventQuery, eventID)
	var (
		ev         domain.AuditEvent
		reqID      sql.NullString
		ipRaw      sql.NullString
		userAgent  sql.NullString
		payloadRaw []byte
	)
	if err := row.Scan(
		&ev.EventID,
		&ev.OccurredAt,
		&ev.Actor,
		&ev.Action,
		&ev.ResourceType,
		&ev.ResourceID,
		&reqID,
		&ipRaw,
		&userAgent,
		&payloadRaw,
		&ev.IntegritySHA256,
	); err != nil {
		return domain.AuditEvent{}, handleNotFound(err)
	}
	ev.RequestID = strings.TrimSpace(reqID.String)
	if ipRaw.Valid {
		ev.IP = net.ParseIP(strings.TrimSpace(ipRaw.String))
	}
	ev.UserAgent = strings.TrimSpace(userAgent.String)
	payload, err := decodeMetadata(payloadRaw)
	if err != nil {
		return domain.AuditEvent{}, err
	}
	ev.Payload = payload
	return ev, nil
}

func (s *AuditExportStore) GetSink(ctx context.Context, sinkID string) (auditexport.Sink, error) {
	if s == nil || s.db == nil {
		return auditexport.Sink{}, fmt.Errorf("audit export store not initialized")
	}
	sinkID = strings.TrimSpace(sinkID)
	if sinkID == "" {
		return auditexport.Sink{}, fmt.Errorf("sink_id is required")
	}
	row := s.db.QueryRowContext(ctx, selectAuditExportSinkQuery, sinkID)
	var out auditexport.Sink
	if err := row.Scan(&out.SinkID, &out.Name, &out.Destination, &out.Format, &out.Config, &out.Enabled, &out.CreatedAt, &out.UpdatedAt); err != nil {
		return auditexport.Sink{}, handleNotFound(err)
	}
	return out, nil
}

func (s *AuditExportStore) MarkDelivered(ctx context.Context, outboxID int64, deliveredAt time.Time) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("audit export store not initialized")
	}
	if outboxID <= 0 {
		return fmt.Errorf("outbox_id is required")
	}
	at := normalizeTime(deliveredAt)
	_, err := s.db.ExecContext(ctx, markOutboxDeliveredQuery, auditexport.OutboxStatusDelivered, at, outboxID)
	if err != nil {
		return fmt.Errorf("mark audit export delivered: %w", err)
	}
	return nil
}

func (s *AuditExportStore) MarkFailed(ctx context.Context, outboxID int64, lastError string, nextAttemptAt time.Time) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("audit export store not initialized")
	}
	if outboxID <= 0 {
		return fmt.Errorf("outbox_id is required")
	}
	if strings.TrimSpace(lastError) == "" {
		lastError = "export_failed"
	}
	next := normalizeTime(nextAttemptAt)
	_, err := s.db.ExecContext(ctx, markOutboxFailedQuery, auditexport.OutboxStatusRetry, next, strings.TrimSpace(lastError), outboxID)
	if err != nil {
		return fmt.Errorf("mark audit export failed: %w", err)
	}
	return nil
}

func (s *AuditExportStore) ListSinks(ctx context.Context, limit int) ([]auditexport.Sink, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("audit export store not initialized")
	}
	if limit <= 0 {
		limit = 200
	}
	rows, err := s.db.QueryContext(ctx, listAuditExportSinksQuery, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]auditexport.Sink, 0)
	for rows.Next() {
		var record auditexport.Sink
		if err := rows.Scan(&record.SinkID, &record.Name, &record.Destination, &record.Format, &record.Config, &record.Enabled, &record.CreatedAt, &record.UpdatedAt); err != nil {
			return nil, err
		}
		record.CreatedAt = record.CreatedAt.UTC()
		record.UpdatedAt = record.UpdatedAt.UTC()
		out = append(out, record)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

var _ auditexport.SinkStore = (*AuditExportStore)(nil)
