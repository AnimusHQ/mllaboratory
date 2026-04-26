package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net"
	"strings"
	"time"

	"github.com/animus-labs/animus-go/closed/internal/domain"
	"github.com/animus-labs/animus-go/closed/internal/repo"
	"github.com/google/uuid"
)

type SessionManager struct {
	Store         repo.SessionRepository
	Audit         repo.AuditEventAppender
	MaxConcurrent int
	Now           func() time.Time
}

type SessionRequestMeta struct {
	RequestID string
	UserAgent string
	RemoteIP  string
	Actor     string
}

func (m *SessionManager) CreateSession(ctx context.Context, identity Identity, issuer string, expiresAt time.Time, tokenSHA string, meta SessionRequestMeta) (repo.SessionRecord, error) {
	if m == nil || m.Store == nil {
		return repo.SessionRecord{}, ErrUnauthenticated
	}
	if expiresAt.IsZero() {
		expiresAt = time.Now().UTC().Add(1 * time.Hour)
	}

	now := time.Now().UTC()
	if m.Now != nil {
		now = m.Now().UTC()
	}

	record := repo.SessionRecord{
		SessionID:     uuid.NewString(),
		Subject:       strings.TrimSpace(identity.Subject),
		Email:         strings.TrimSpace(identity.Email),
		Roles:         identity.Roles,
		Issuer:        strings.TrimSpace(issuer),
		CreatedAt:     now,
		ExpiresAt:     expiresAt.UTC(),
		LastSeenAt:    &now,
		IDTokenSHA256: strings.TrimSpace(tokenSHA),
		UserAgent:     strings.TrimSpace(meta.UserAgent),
		IP:            strings.TrimSpace(meta.RemoteIP),
		Metadata:      domain.Metadata{},
	}
	if err := m.Store.Create(ctx, record); err != nil {
		return repo.SessionRecord{}, err
	}

	m.auditEvent(ctx, record, "auth.user_logged_in", meta, "")

	m.enforceMaxSessions(ctx, record, meta)
	return record, nil
}

func (m *SessionManager) GetSession(ctx context.Context, sessionID string) (repo.SessionRecord, error) {
	if m == nil || m.Store == nil {
		return repo.SessionRecord{}, ErrUnauthenticated
	}
	record, err := m.Store.Get(ctx, sessionID)
	if err != nil {
		return repo.SessionRecord{}, err
	}

	if record.RevokedAt != nil {
		return repo.SessionRecord{}, ErrUnauthenticated
	}

	now := time.Now().UTC()
	if m.Now != nil {
		now = m.Now().UTC()
	}
	if record.ExpiresAt.Before(now) || record.ExpiresAt.Equal(now) {
		_, _ = m.Store.Revoke(ctx, record.SessionID, "system", "expired", now)
		m.auditEvent(ctx, record, "auth.session_expired", SessionRequestMeta{}, "expired")
		return repo.SessionRecord{}, ErrUnauthenticated
	}

	_ = m.Store.UpdateLastSeen(ctx, record.SessionID, now)
	return record, nil
}

func (m *SessionManager) RevokeSession(ctx context.Context, sessionID, revokedBy, reason string, meta SessionRequestMeta) (bool, error) {
	if m == nil || m.Store == nil {
		return false, ErrUnauthenticated
	}
	now := time.Now().UTC()
	if m.Now != nil {
		now = m.Now().UTC()
	}
	updated, err := m.Store.Revoke(ctx, sessionID, revokedBy, reason, now)
	if err != nil {
		return false, err
	}
	if updated {
		rec, recErr := m.Store.Get(ctx, sessionID)
		if recErr == nil {
			action := "auth.user_logged_out"
			if strings.TrimSpace(reason) == "forced" {
				action = "auth.forced_logout"
			}
			m.auditEvent(ctx, rec, action, meta, reason)
		}
	}
	return updated, nil
}

func (m *SessionManager) RevokeBySubject(ctx context.Context, subject, revokedBy, reason string, meta SessionRequestMeta) (int, error) {
	if m == nil || m.Store == nil {
		return 0, ErrUnauthenticated
	}
	now := time.Now().UTC()
	if m.Now != nil {
		now = m.Now().UTC()
	}
	count, err := m.Store.RevokeBySubject(ctx, subject, revokedBy, reason, now)
	if err != nil {
		return 0, err
	}
	if count > 0 {
		m.auditSubjectEvent(ctx, subject, "auth.forced_logout", meta, reason, count)
	}
	return count, nil
}

func (m *SessionManager) enforceMaxSessions(ctx context.Context, record repo.SessionRecord, meta SessionRequestMeta) {
	if m == nil || m.Store == nil {
		return
	}
	limit := m.MaxConcurrent
	if limit <= 0 {
		return
	}

	active, err := m.Store.ListActiveBySubject(ctx, record.Subject, limit+5)
	if err != nil {
		return
	}
	if len(active) <= limit {
		return
	}

	revokeCount := len(active) - limit
	for _, session := range active {
		if revokeCount <= 0 {
			break
		}
		if session.SessionID == record.SessionID {
			continue
		}
		now := time.Now().UTC()
		if m.Now != nil {
			now = m.Now().UTC()
		}
		_, _ = m.Store.Revoke(ctx, session.SessionID, "system", "max_sessions", now)
		m.auditEvent(ctx, session, "auth.forced_logout", meta, "max_sessions")
		revokeCount--
	}
}

func (m *SessionManager) auditEvent(ctx context.Context, record repo.SessionRecord, action string, meta SessionRequestMeta, reason string) {
	if m == nil || m.Audit == nil {
		return
	}
	when := time.Now().UTC()
	if m.Now != nil {
		when = m.Now().UTC()
	}

	payload := domain.Metadata{
		"session_id": record.SessionID,
		"subject":    record.Subject,
		"email":      record.Email,
		"roles":      record.Roles,
		"reason":     strings.TrimSpace(reason),
	}
	if strings.TrimSpace(meta.UserAgent) != "" {
		payload["user_agent"] = strings.TrimSpace(meta.UserAgent)
	}
	if strings.TrimSpace(meta.RemoteIP) != "" {
		payload["ip"] = strings.TrimSpace(meta.RemoteIP)
	}

	actor := strings.TrimSpace(meta.Actor)
	if actor == "" {
		actor = record.Subject
	}
	_, _ = m.Audit.Append(ctx, domain.AuditEvent{
		OccurredAt:   when,
		Actor:        actor,
		Action:       action,
		ResourceType: "session",
		ResourceID:   record.SessionID,
		RequestID:    strings.TrimSpace(meta.RequestID),
		IP:           parseIP(meta.RemoteIP),
		UserAgent:    strings.TrimSpace(meta.UserAgent),
		Payload:      payload,
	})
}

func (m *SessionManager) auditSubjectEvent(ctx context.Context, subject, action string, meta SessionRequestMeta, reason string, count int) {
	if m == nil || m.Audit == nil {
		return
	}
	when := time.Now().UTC()
	if m.Now != nil {
		when = m.Now().UTC()
	}

	payload := domain.Metadata{
		"subject": subject,
		"reason":  strings.TrimSpace(reason),
		"count":   count,
	}

	actor := strings.TrimSpace(meta.Actor)
	if actor == "" {
		actor = subject
	}
	_, _ = m.Audit.Append(ctx, domain.AuditEvent{
		OccurredAt:   when,
		Actor:        actor,
		Action:       action,
		ResourceType: "session",
		ResourceID:   subject,
		RequestID:    strings.TrimSpace(meta.RequestID),
		IP:           parseIP(meta.RemoteIP),
		UserAgent:    strings.TrimSpace(meta.UserAgent),
		Payload:      payload,
	})
}

func TokenSHA256(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(trimmed))
	return hex.EncodeToString(sum[:])
}

func ParseRemoteIP(remoteAddr string) string {
	remoteAddr = strings.TrimSpace(remoteAddr)
	if remoteAddr == "" {
		return ""
	}
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(host)
}

func parseIP(raw string) net.IP {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	return net.ParseIP(raw)
}
