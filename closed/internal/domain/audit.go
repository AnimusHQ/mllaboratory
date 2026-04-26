package domain

import (
	"errors"
	"net"
	"strings"
	"time"
)

// AuditEvent is an immutable audit record.
type AuditEvent struct {
	EventID         int64
	OccurredAt      time.Time
	Actor           string
	Action          string
	ResourceType    string
	ResourceID      string
	RequestID       string
	IP              net.IP
	UserAgent       string
	Payload         Metadata
	IntegritySHA256 string
}

func (e AuditEvent) Validate() error {
	if e.OccurredAt.IsZero() {
		return errors.New("occurred_at is required")
	}
	if strings.TrimSpace(e.Actor) == "" {
		return errors.New("actor is required")
	}
	if strings.TrimSpace(e.Action) == "" {
		return errors.New("action is required")
	}
	if strings.TrimSpace(e.ResourceType) == "" {
		return errors.New("resource_type is required")
	}
	if strings.TrimSpace(e.ResourceID) == "" {
		return errors.New("resource_id is required")
	}
	return nil
}
