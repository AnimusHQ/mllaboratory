package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/animus-labs/animus-go/closed/internal/auditexport"
	"github.com/animus-labs/animus-go/closed/internal/domain"
	"github.com/animus-labs/animus-go/closed/internal/platform/auditlog"
)

type AuditAppender struct {
	db       auditlog.QueryRower
	exporter auditexport.Exporter
	now      func() time.Time
}

func NewAuditAppender(db auditlog.QueryRower, exporter auditexport.Exporter) *AuditAppender {
	if db == nil {
		return nil
	}
	if exporter == nil {
		exporter = auditexport.NoopExporter{}
	}
	return &AuditAppender{db: db, exporter: exporter, now: time.Now}
}

func (a *AuditAppender) Append(ctx context.Context, event domain.AuditEvent) (int64, error) {
	if a == nil || a.db == nil {
		return 0, errors.New("audit appender not initialized")
	}
	if event.OccurredAt.IsZero() {
		event.OccurredAt = a.now().UTC()
	}
	payload := event.Payload
	if payload == nil {
		payload = domain.Metadata{}
	}
	id, err := auditlog.Insert(ctx, a.db, auditlog.Event{
		OccurredAt:   event.OccurredAt,
		Actor:        event.Actor,
		Action:       event.Action,
		ResourceType: event.ResourceType,
		ResourceID:   event.ResourceID,
		RequestID:    event.RequestID,
		IP:           event.IP,
		UserAgent:    event.UserAgent,
		Payload:      payload,
	})
	if err != nil {
		return 0, fmt.Errorf("append audit event: %w", err)
	}
	if a.exporter != nil {
		if err := a.exporter.Export(ctx, event); err != nil {
			return id, fmt.Errorf("export audit event: %w", err)
		}
	}
	return id, nil
}
