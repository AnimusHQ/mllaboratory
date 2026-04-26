package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type DPEventStore struct {
	db DB
}

const (
	insertRunDPEventQuery = `INSERT INTO run_dp_events (
			event_id,
			run_id,
			project_id,
			event_type,
			payload,
			emitted_at,
			received_at,
			integrity_sha256
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		ON CONFLICT (event_id) DO NOTHING`
	selectRunDPEventByIDQuery = `SELECT event_id, run_id, project_id, event_type, payload, emitted_at, received_at, integrity_sha256
		FROM run_dp_events
		WHERE event_id = $1`
	selectRunDPLatestByTypeQuery = `SELECT event_id, emitted_at, payload
		FROM run_dp_events
		WHERE project_id = $1 AND run_id = $2 AND event_type = $3
		ORDER BY emitted_at DESC, received_at DESC
		LIMIT 1`

	insertRunDispatchQuery = `INSERT INTO run_dispatches (
			dispatch_id,
			run_id,
			project_id,
			idempotency_key,
			dp_base_url,
			status,
			last_error,
			spec_hash,
			requested_at,
			requested_by,
			updated_at,
			integrity_sha256
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		ON CONFLICT (project_id, idempotency_key) DO NOTHING
		RETURNING dispatch_id, run_id, project_id, idempotency_key, dp_base_url, status, last_error, spec_hash, requested_at, requested_by, updated_at, integrity_sha256`
	selectRunDispatchByIdempotencyQuery = `SELECT dispatch_id, run_id, project_id, idempotency_key, dp_base_url, status, last_error, spec_hash, requested_at, requested_by, updated_at, integrity_sha256
		FROM run_dispatches
		WHERE project_id = $1 AND idempotency_key = $2`
	selectRunDispatchByRunIDQuery = `SELECT dispatch_id, run_id, project_id, idempotency_key, dp_base_url, status, last_error, spec_hash, requested_at, requested_by, updated_at, integrity_sha256
		FROM run_dispatches
		WHERE project_id = $1 AND run_id = $2`
	updateRunDispatchStatusQuery = `UPDATE run_dispatches
		SET status = $1, last_error = $2, updated_at = $3
		WHERE dispatch_id = $4`
	selectRunDispatchesByStatusBase = `SELECT dispatch_id, run_id, project_id, idempotency_key, dp_base_url, status, last_error, spec_hash, requested_at, requested_by, updated_at, integrity_sha256
		FROM run_dispatches`
)

func NewDPEventStore(db DB) *DPEventStore {
	if db == nil {
		return nil
	}
	return &DPEventStore{db: db}
}

type RunDPEventRecord struct {
	EventID      string
	RunID        string
	ProjectID    string
	EventType    string
	Payload      json.RawMessage
	EmittedAt    time.Time
	ReceivedAt   time.Time
	IntegritySHA string
}

type RunDPEventSummary struct {
	EventID   string
	EmittedAt time.Time
	Payload   json.RawMessage
}

func (s *DPEventStore) InsertEvent(ctx context.Context, record RunDPEventRecord) (bool, error) {
	if s == nil || s.db == nil {
		return false, fmt.Errorf("dp event store not initialized")
	}
	record.EventID = strings.TrimSpace(record.EventID)
	record.RunID = strings.TrimSpace(record.RunID)
	record.ProjectID = strings.TrimSpace(record.ProjectID)
	record.EventType = strings.TrimSpace(record.EventType)
	if record.EventID == "" || record.RunID == "" || record.ProjectID == "" || record.EventType == "" {
		return false, fmt.Errorf("event_id, run_id, project_id, event_type are required")
	}
	if record.EmittedAt.IsZero() {
		return false, fmt.Errorf("emitted_at is required")
	}
	if record.Payload == nil {
		record.Payload = json.RawMessage(`{}`)
	}
	if err := requireIntegrity(record.IntegritySHA); err != nil {
		return false, err
	}
	receivedAt := normalizeTime(record.ReceivedAt)
	if receivedAt.IsZero() {
		receivedAt = time.Now().UTC()
	}
	res, err := s.db.ExecContext(
		ctx,
		insertRunDPEventQuery,
		record.EventID,
		record.RunID,
		record.ProjectID,
		record.EventType,
		record.Payload,
		record.EmittedAt.UTC(),
		receivedAt,
		record.IntegritySHA,
	)
	if err != nil {
		return false, fmt.Errorf("insert dp event: %w", err)
	}
	rows, _ := res.RowsAffected()
	return rows > 0, nil
}

func (s *DPEventStore) GetEvent(ctx context.Context, eventID string) (RunDPEventRecord, error) {
	if s == nil || s.db == nil {
		return RunDPEventRecord{}, fmt.Errorf("dp event store not initialized")
	}
	eventID = strings.TrimSpace(eventID)
	if eventID == "" {
		return RunDPEventRecord{}, fmt.Errorf("event_id is required")
	}
	var record RunDPEventRecord
	row := s.db.QueryRowContext(ctx, selectRunDPEventByIDQuery, eventID)
	if err := row.Scan(&record.EventID, &record.RunID, &record.ProjectID, &record.EventType, &record.Payload, &record.EmittedAt, &record.ReceivedAt, &record.IntegritySHA); err != nil {
		return RunDPEventRecord{}, handleNotFound(err)
	}
	return record, nil
}

func (s *DPEventStore) LatestEventByType(ctx context.Context, projectID, runID, eventType string) (RunDPEventSummary, error) {
	if s == nil || s.db == nil {
		return RunDPEventSummary{}, fmt.Errorf("dp event store not initialized")
	}
	projectID = strings.TrimSpace(projectID)
	runID = strings.TrimSpace(runID)
	eventType = strings.TrimSpace(eventType)
	if projectID == "" || runID == "" || eventType == "" {
		return RunDPEventSummary{}, fmt.Errorf("project_id, run_id, event_type are required")
	}
	var summary RunDPEventSummary
	row := s.db.QueryRowContext(ctx, selectRunDPLatestByTypeQuery, projectID, runID, eventType)
	if err := row.Scan(&summary.EventID, &summary.EmittedAt, &summary.Payload); err != nil {
		return RunDPEventSummary{}, handleNotFound(err)
	}
	return summary, nil
}

type RunDispatchRecord struct {
	DispatchID     string
	RunID          string
	ProjectID      string
	IdempotencyKey string
	DPBaseURL      string
	Status         string
	LastError      sql.NullString
	SpecHash       string
	RequestedAt    time.Time
	RequestedBy    string
	UpdatedAt      time.Time
	IntegritySHA   string
}

func (s *DPEventStore) CreateDispatch(ctx context.Context, record RunDispatchRecord) (RunDispatchRecord, bool, error) {
	if s == nil || s.db == nil {
		return RunDispatchRecord{}, false, fmt.Errorf("dp event store not initialized")
	}
	record.DispatchID = strings.TrimSpace(record.DispatchID)
	record.RunID = strings.TrimSpace(record.RunID)
	record.ProjectID = strings.TrimSpace(record.ProjectID)
	record.IdempotencyKey = strings.TrimSpace(record.IdempotencyKey)
	record.DPBaseURL = strings.TrimSpace(record.DPBaseURL)
	record.Status = strings.TrimSpace(record.Status)
	record.RequestedBy = strings.TrimSpace(record.RequestedBy)
	record.SpecHash = strings.TrimSpace(record.SpecHash)
	if record.DispatchID == "" || record.RunID == "" || record.ProjectID == "" || record.IdempotencyKey == "" {
		return RunDispatchRecord{}, false, fmt.Errorf("dispatch_id, run_id, project_id, idempotency_key are required")
	}
	if record.DPBaseURL == "" || record.Status == "" || record.RequestedBy == "" || record.SpecHash == "" {
		return RunDispatchRecord{}, false, fmt.Errorf("dp_base_url, status, requested_by, spec_hash are required")
	}
	if record.RequestedAt.IsZero() {
		return RunDispatchRecord{}, false, fmt.Errorf("requested_at is required")
	}
	if err := requireIntegrity(record.IntegritySHA); err != nil {
		return RunDispatchRecord{}, false, err
	}
	updatedAt := normalizeTime(record.UpdatedAt)
	if updatedAt.IsZero() {
		updatedAt = record.RequestedAt.UTC()
	}
	row := s.db.QueryRowContext(
		ctx,
		insertRunDispatchQuery,
		record.DispatchID,
		record.RunID,
		record.ProjectID,
		record.IdempotencyKey,
		record.DPBaseURL,
		record.Status,
		nullIfEmpty(record.LastError.String),
		record.SpecHash,
		record.RequestedAt.UTC(),
		record.RequestedBy,
		updatedAt,
		record.IntegritySHA,
	)
	var out RunDispatchRecord
	if err := row.Scan(&out.DispatchID, &out.RunID, &out.ProjectID, &out.IdempotencyKey, &out.DPBaseURL, &out.Status, &out.LastError, &out.SpecHash, &out.RequestedAt, &out.RequestedBy, &out.UpdatedAt, &out.IntegritySHA); err != nil {
		if err != sql.ErrNoRows {
			return RunDispatchRecord{}, false, fmt.Errorf("insert run dispatch: %w", err)
		}
		existing, err := s.GetDispatchByIdempotencyKey(ctx, record.ProjectID, record.IdempotencyKey)
		if err != nil {
			return RunDispatchRecord{}, false, err
		}
		return existing, false, nil
	}
	return out, true, nil
}

func (s *DPEventStore) GetDispatchByIdempotencyKey(ctx context.Context, projectID, idempotencyKey string) (RunDispatchRecord, error) {
	if s == nil || s.db == nil {
		return RunDispatchRecord{}, fmt.Errorf("dp event store not initialized")
	}
	projectID = strings.TrimSpace(projectID)
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if projectID == "" || idempotencyKey == "" {
		return RunDispatchRecord{}, fmt.Errorf("project_id and idempotency_key are required")
	}
	row := s.db.QueryRowContext(ctx, selectRunDispatchByIdempotencyQuery, projectID, idempotencyKey)
	var record RunDispatchRecord
	if err := row.Scan(&record.DispatchID, &record.RunID, &record.ProjectID, &record.IdempotencyKey, &record.DPBaseURL, &record.Status, &record.LastError, &record.SpecHash, &record.RequestedAt, &record.RequestedBy, &record.UpdatedAt, &record.IntegritySHA); err != nil {
		return RunDispatchRecord{}, handleNotFound(err)
	}
	return record, nil
}

func (s *DPEventStore) GetDispatchByRunID(ctx context.Context, projectID, runID string) (RunDispatchRecord, error) {
	if s == nil || s.db == nil {
		return RunDispatchRecord{}, fmt.Errorf("dp event store not initialized")
	}
	projectID = strings.TrimSpace(projectID)
	runID = strings.TrimSpace(runID)
	if projectID == "" || runID == "" {
		return RunDispatchRecord{}, fmt.Errorf("project_id and run_id are required")
	}
	row := s.db.QueryRowContext(ctx, selectRunDispatchByRunIDQuery, projectID, runID)
	var record RunDispatchRecord
	if err := row.Scan(&record.DispatchID, &record.RunID, &record.ProjectID, &record.IdempotencyKey, &record.DPBaseURL, &record.Status, &record.LastError, &record.SpecHash, &record.RequestedAt, &record.RequestedBy, &record.UpdatedAt, &record.IntegritySHA); err != nil {
		return RunDispatchRecord{}, handleNotFound(err)
	}
	return record, nil
}

func (s *DPEventStore) UpdateDispatchStatus(ctx context.Context, dispatchID, status, lastError string, updatedAt time.Time) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("dp event store not initialized")
	}
	dispatchID = strings.TrimSpace(dispatchID)
	status = strings.TrimSpace(status)
	if dispatchID == "" || status == "" {
		return fmt.Errorf("dispatch_id and status are required")
	}
	updatedAt = normalizeTime(updatedAt)
	if updatedAt.IsZero() {
		updatedAt = time.Now().UTC()
	}
	res, err := s.db.ExecContext(ctx, updateRunDispatchStatusQuery, status, nullIfEmpty(lastError), updatedAt, dispatchID)
	if err != nil {
		return fmt.Errorf("update run dispatch status: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("update run dispatch status: %w", err)
	}
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *DPEventStore) ListDispatchesByStatus(ctx context.Context, statuses []string, limit int) ([]RunDispatchRecord, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("dp event store not initialized")
	}
	if len(statuses) == 0 {
		return nil, fmt.Errorf("statuses are required")
	}
	args := make([]any, 0, len(statuses)+1)
	clauses := make([]string, 0, len(statuses))
	for _, status := range statuses {
		value := strings.TrimSpace(status)
		if value == "" {
			continue
		}
		args = append(args, value)
		clauses = append(clauses, fmt.Sprintf("status = $%d", len(args)))
	}
	if len(clauses) == 0 {
		return nil, fmt.Errorf("statuses are required")
	}
	query := selectRunDispatchesByStatusBase + " WHERE " + strings.Join(clauses, " OR ") + " ORDER BY requested_at DESC"
	if limit > 0 {
		args = append(args, limit)
		query += fmt.Sprintf(" LIMIT $%d", len(args))
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list run dispatches: %w", err)
	}
	defer rows.Close()

	out := make([]RunDispatchRecord, 0)
	for rows.Next() {
		var record RunDispatchRecord
		if err := rows.Scan(&record.DispatchID, &record.RunID, &record.ProjectID, &record.IdempotencyKey, &record.DPBaseURL, &record.Status, &record.LastError, &record.SpecHash, &record.RequestedAt, &record.RequestedBy, &record.UpdatedAt, &record.IntegritySHA); err != nil {
			return nil, fmt.Errorf("scan run dispatch: %w", err)
		}
		out = append(out, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list run dispatches: %w", err)
	}
	return out, nil
}
