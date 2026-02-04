package postgres

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/animus-labs/animus-go/closed/internal/auditexport"
)

type AuditExportReplayStore struct {
	db DB
}

const (
	insertAuditExportReplayQuery = `INSERT INTO audit_export_replays (
			delivery_id,
			replay_token,
			requested_at
		) VALUES ($1,$2,$3)
		ON CONFLICT (delivery_id, replay_token) DO NOTHING`
)

func NewAuditExportReplayStore(db DB) *AuditExportReplayStore {
	if db == nil {
		return nil
	}
	return &AuditExportReplayStore{db: db}
}

func (s *AuditExportReplayStore) Insert(ctx context.Context, deliveryID int64, token string, requestedAt time.Time) (bool, error) {
	if s == nil || s.db == nil {
		return false, fmt.Errorf("audit export replay store not initialized")
	}
	if deliveryID <= 0 {
		return false, fmt.Errorf("delivery_id is required")
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return false, fmt.Errorf("replay_token is required")
	}
	if requestedAt.IsZero() {
		requestedAt = time.Now().UTC()
	}
	res, err := s.db.ExecContext(ctx, insertAuditExportReplayQuery, deliveryID, token, requestedAt.UTC())
	if err != nil {
		return false, err
	}
	rows, _ := res.RowsAffected()
	return rows > 0, nil
}

var _ auditexport.ReplayStore = (*AuditExportReplayStore)(nil)
