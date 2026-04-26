package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/animus-labs/animus-go/closed/internal/repo"
)

type stubSessionStore struct {
	record               repo.SessionRecord
	active               []repo.SessionRecord
	includeCreated       bool
	revoked              bool
	revokedBy            string
	revokeReason         string
	revokedIDs           []string
	revokeBySubjectCount int
}

func (s *stubSessionStore) Create(ctx context.Context, record repo.SessionRecord) error {
	s.record = record
	return nil
}

func (s *stubSessionStore) Get(ctx context.Context, sessionID string) (repo.SessionRecord, error) {
	return s.record, nil
}

func (s *stubSessionStore) ListActiveBySubject(ctx context.Context, subject string, limit int) ([]repo.SessionRecord, error) {
	if s.includeCreated && s.record.SessionID != "" {
		out := make([]repo.SessionRecord, 0, len(s.active)+1)
		out = append(out, s.active...)
		out = append(out, s.record)
		return out, nil
	}
	return s.active, nil
}

func (s *stubSessionStore) UpdateLastSeen(ctx context.Context, sessionID string, at time.Time) error {
	return nil
}

func (s *stubSessionStore) Revoke(ctx context.Context, sessionID, revokedBy, reason string, at time.Time) (bool, error) {
	s.revoked = true
	s.revokedBy = revokedBy
	s.revokeReason = reason
	s.revokedIDs = append(s.revokedIDs, sessionID)
	return true, nil
}

func (s *stubSessionStore) RevokeBySubject(ctx context.Context, subject, revokedBy, reason string, at time.Time) (int, error) {
	return s.revokeBySubjectCount, nil
}

func TestSessionManagerExpiresSession(t *testing.T) {
	now := time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)
	store := &stubSessionStore{
		record: repo.SessionRecord{
			SessionID: "sess-1",
			Subject:   "user-1",
			ExpiresAt: now.Add(-1 * time.Minute),
		},
	}
	mgr := SessionManager{
		Store: store,
		Now:   func() time.Time { return now },
	}

	_, err := mgr.GetSession(context.Background(), "sess-1")
	if !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("expected unauthenticated, got %v", err)
	}
	if !store.revoked {
		t.Fatalf("expected session revoked on expiry")
	}
	if store.revokeReason != "expired" {
		t.Fatalf("expected revoke reason expired, got %q", store.revokeReason)
	}
}

func TestSessionManagerRejectsRevokedSession(t *testing.T) {
	now := time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)
	revokedAt := now.Add(-5 * time.Minute)
	store := &stubSessionStore{
		record: repo.SessionRecord{
			SessionID: "sess-2",
			Subject:   "user-2",
			ExpiresAt: now.Add(1 * time.Hour),
			RevokedAt: &revokedAt,
		},
	}
	mgr := SessionManager{
		Store: store,
		Now:   func() time.Time { return now },
	}

	_, err := mgr.GetSession(context.Background(), "sess-2")
	if !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("expected unauthenticated, got %v", err)
	}
	if store.revoked {
		t.Fatalf("expected revoked flag to remain false")
	}
}

func TestSessionManagerEnforcesMaxConcurrentSessions(t *testing.T) {
	now := time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)
	store := &stubSessionStore{
		active: []repo.SessionRecord{
			{
				SessionID: "sess-old",
				Subject:   "user-3",
				ExpiresAt: now.Add(2 * time.Hour),
				CreatedAt: now.Add(-1 * time.Hour),
			},
		},
		includeCreated: true,
	}
	mgr := SessionManager{
		Store:         store,
		MaxConcurrent: 1,
		Now:           func() time.Time { return now },
	}

	_, err := mgr.CreateSession(context.Background(), Identity{Subject: "user-3"}, "issuer", now.Add(2*time.Hour), "sha", SessionRequestMeta{})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	if len(store.revokedIDs) != 1 || store.revokedIDs[0] != "sess-old" {
		t.Fatalf("expected old session revoked, got %v", store.revokedIDs)
	}
	if store.revokeReason != "max_sessions" {
		t.Fatalf("expected revoke reason max_sessions, got %q", store.revokeReason)
	}
}
