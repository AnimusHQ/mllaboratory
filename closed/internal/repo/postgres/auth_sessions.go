package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/animus-labs/animus-go/closed/internal/repo"
)

type SessionStore struct {
	db DB
}

const (
	insertSessionQuery = `INSERT INTO auth_sessions (
		session_id,
		subject,
		email,
		roles,
		issuer,
		created_at,
		expires_at,
		last_seen_at,
		revoked_at,
		revoked_by,
		revoke_reason,
		id_token_sha256,
		user_agent,
		ip,
		metadata
	) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)`

	selectSessionQuery = `SELECT session_id, subject, email, roles, issuer, created_at, expires_at,
		last_seen_at, revoked_at, revoked_by, revoke_reason, id_token_sha256, user_agent, ip, metadata
		FROM auth_sessions WHERE session_id = $1`
)

func NewSessionStore(db DB) *SessionStore {
	if db == nil {
		return nil
	}
	return &SessionStore{db: db}
}

func (s *SessionStore) Create(ctx context.Context, record repo.SessionRecord) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("session store not initialized")
	}
	record.SessionID = strings.TrimSpace(record.SessionID)
	record.Subject = strings.TrimSpace(record.Subject)
	record.Email = strings.TrimSpace(record.Email)
	record.Issuer = strings.TrimSpace(record.Issuer)
	record.RevokedBy = strings.TrimSpace(record.RevokedBy)
	record.RevokeReason = strings.TrimSpace(record.RevokeReason)
	record.IDTokenSHA256 = strings.TrimSpace(record.IDTokenSHA256)
	record.UserAgent = strings.TrimSpace(record.UserAgent)
	record.IP = strings.TrimSpace(record.IP)

	if record.SessionID == "" || record.Subject == "" {
		return fmt.Errorf("session_id and subject are required")
	}
	if record.ExpiresAt.IsZero() {
		return fmt.Errorf("expires_at is required")
	}
	if record.CreatedAt.IsZero() {
		record.CreatedAt = time.Now().UTC()
	}

	rolesJSON, err := json.Marshal(normalizeRoles(record.Roles))
	if err != nil {
		return fmt.Errorf("marshal roles: %w", err)
	}
	metadataJSON, err := encodeMetadata(record.Metadata)
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}

	var lastSeen sql.NullTime
	if record.LastSeenAt != nil && !record.LastSeenAt.IsZero() {
		lastSeen = sql.NullTime{Time: record.LastSeenAt.UTC(), Valid: true}
	}
	var revokedAt sql.NullTime
	if record.RevokedAt != nil && !record.RevokedAt.IsZero() {
		revokedAt = sql.NullTime{Time: record.RevokedAt.UTC(), Valid: true}
	}

	_, err = s.db.ExecContext(
		ctx,
		insertSessionQuery,
		record.SessionID,
		record.Subject,
		nullIfEmpty(record.Email),
		rolesJSON,
		nullIfEmpty(record.Issuer),
		record.CreatedAt.UTC(),
		record.ExpiresAt.UTC(),
		lastSeen,
		revokedAt,
		nullIfEmpty(record.RevokedBy),
		nullIfEmpty(record.RevokeReason),
		nullIfEmpty(record.IDTokenSHA256),
		nullIfEmpty(record.UserAgent),
		nullIfEmpty(record.IP),
		metadataJSON,
	)
	if err != nil {
		return fmt.Errorf("insert session: %w", err)
	}
	return nil
}

func (s *SessionStore) Get(ctx context.Context, sessionID string) (repo.SessionRecord, error) {
	if s == nil || s.db == nil {
		return repo.SessionRecord{}, fmt.Errorf("session store not initialized")
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return repo.SessionRecord{}, fmt.Errorf("session_id is required")
	}
	row := s.db.QueryRowContext(ctx, selectSessionQuery, sessionID)
	return scanSession(row)
}

func (s *SessionStore) ListActiveBySubject(ctx context.Context, subject string, limit int) ([]repo.SessionRecord, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("session store not initialized")
	}
	subject = strings.TrimSpace(subject)
	if subject == "" {
		return nil, fmt.Errorf("subject is required")
	}
	if limit <= 0 {
		limit = 25
	}
	rows, err := s.db.QueryContext(
		ctx,
		`SELECT session_id, subject, email, roles, issuer, created_at, expires_at,
			last_seen_at, revoked_at, revoked_by, revoke_reason, id_token_sha256, user_agent, ip, metadata
		 FROM auth_sessions
		 WHERE subject = $1 AND revoked_at IS NULL AND expires_at > $2
		 ORDER BY created_at ASC
		 LIMIT $3`,
		subject,
		time.Now().UTC(),
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list active sessions: %w", err)
	}
	defer rows.Close()

	out := make([]repo.SessionRecord, 0)
	for rows.Next() {
		rec, err := scanSession(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list active sessions: %w", err)
	}
	return out, nil
}

func (s *SessionStore) UpdateLastSeen(ctx context.Context, sessionID string, at time.Time) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("session store not initialized")
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return fmt.Errorf("session_id is required")
	}
	if at.IsZero() {
		at = time.Now().UTC()
	}
	_, err := s.db.ExecContext(
		ctx,
		`UPDATE auth_sessions SET last_seen_at = $1 WHERE session_id = $2`,
		at.UTC(),
		sessionID,
	)
	if err != nil {
		return fmt.Errorf("update session last_seen: %w", err)
	}
	return nil
}

func (s *SessionStore) Revoke(ctx context.Context, sessionID, revokedBy, reason string, at time.Time) (bool, error) {
	if s == nil || s.db == nil {
		return false, fmt.Errorf("session store not initialized")
	}
	sessionID = strings.TrimSpace(sessionID)
	revokedBy = strings.TrimSpace(revokedBy)
	reason = strings.TrimSpace(reason)
	if sessionID == "" {
		return false, fmt.Errorf("session_id is required")
	}
	if at.IsZero() {
		at = time.Now().UTC()
	}
	res, err := s.db.ExecContext(
		ctx,
		`UPDATE auth_sessions
		 SET revoked_at = $1,
		     revoked_by = $2,
		     revoke_reason = $3
		 WHERE session_id = $4 AND revoked_at IS NULL`,
		at.UTC(),
		nullIfEmpty(revokedBy),
		nullIfEmpty(reason),
		sessionID,
	)
	if err != nil {
		return false, fmt.Errorf("revoke session: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("revoke session: %w", err)
	}
	return rows > 0, nil
}

func (s *SessionStore) RevokeBySubject(ctx context.Context, subject, revokedBy, reason string, at time.Time) (int, error) {
	if s == nil || s.db == nil {
		return 0, fmt.Errorf("session store not initialized")
	}
	subject = strings.TrimSpace(subject)
	revokedBy = strings.TrimSpace(revokedBy)
	reason = strings.TrimSpace(reason)
	if subject == "" {
		return 0, fmt.Errorf("subject is required")
	}
	if at.IsZero() {
		at = time.Now().UTC()
	}
	res, err := s.db.ExecContext(
		ctx,
		`UPDATE auth_sessions
		 SET revoked_at = $1,
		     revoked_by = $2,
		     revoke_reason = $3
		 WHERE subject = $4 AND revoked_at IS NULL`,
		at.UTC(),
		nullIfEmpty(revokedBy),
		nullIfEmpty(reason),
		subject,
	)
	if err != nil {
		return 0, fmt.Errorf("revoke sessions by subject: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("revoke sessions by subject: %w", err)
	}
	return int(rows), nil
}

type sessionScanner interface {
	Scan(dest ...any) error
}

func scanSession(row sessionScanner) (repo.SessionRecord, error) {
	var record repo.SessionRecord
	var (
		email     sql.NullString
		issuer    sql.NullString
		rolesRaw  []byte
		lastSeen  sql.NullTime
		revokedAt sql.NullTime
		revokedBy sql.NullString
		reason    sql.NullString
		tokenSHA  sql.NullString
		userAgent sql.NullString
		ip        sql.NullString
		metadata  []byte
	)
	if err := row.Scan(
		&record.SessionID,
		&record.Subject,
		&email,
		&rolesRaw,
		&issuer,
		&record.CreatedAt,
		&record.ExpiresAt,
		&lastSeen,
		&revokedAt,
		&revokedBy,
		&reason,
		&tokenSHA,
		&userAgent,
		&ip,
		&metadata,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return repo.SessionRecord{}, repo.ErrNotFound
		}
		return repo.SessionRecord{}, fmt.Errorf("scan session: %w", err)
	}
	if email.Valid {
		record.Email = strings.TrimSpace(email.String)
	}
	if issuer.Valid {
		record.Issuer = strings.TrimSpace(issuer.String)
	}
	if lastSeen.Valid {
		t := lastSeen.Time.UTC()
		record.LastSeenAt = &t
	}
	if revokedAt.Valid {
		t := revokedAt.Time.UTC()
		record.RevokedAt = &t
	}
	if revokedBy.Valid {
		record.RevokedBy = strings.TrimSpace(revokedBy.String)
	}
	if reason.Valid {
		record.RevokeReason = strings.TrimSpace(reason.String)
	}
	if tokenSHA.Valid {
		record.IDTokenSHA256 = strings.TrimSpace(tokenSHA.String)
	}
	if userAgent.Valid {
		record.UserAgent = strings.TrimSpace(userAgent.String)
	}
	if ip.Valid {
		record.IP = strings.TrimSpace(ip.String)
	}
	if len(rolesRaw) > 0 {
		var roles []string
		if err := json.Unmarshal(rolesRaw, &roles); err == nil {
			record.Roles = normalizeRoles(roles)
		}
	}
	decoded, err := decodeMetadata(metadata)
	if err != nil {
		return repo.SessionRecord{}, fmt.Errorf("decode metadata: %w", err)
	}
	record.Metadata = decoded
	return record, nil
}

func normalizeRoles(input []string) []string {
	if len(input) == 0 {
		return nil
	}
	out := make([]string, 0, len(input))
	seen := map[string]struct{}{}
	for _, role := range input {
		role = strings.ToLower(strings.TrimSpace(role))
		if role == "" {
			continue
		}
		if _, ok := seen[role]; ok {
			continue
		}
		seen[role] = struct{}{}
		out = append(out, role)
	}
	return out
}
