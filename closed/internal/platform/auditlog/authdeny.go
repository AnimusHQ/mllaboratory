package auditlog

import (
	"context"
	"database/sql"
	"net"
	"strings"

	"github.com/animus-labs/animus-go/closed/internal/platform/auth"
)

func InsertAuthDeny(ctx context.Context, db *sql.DB, service string, event auth.DenyEvent) error {
	actor := "anonymous"
	if strings.TrimSpace(event.Subject) != "" {
		actor = strings.TrimSpace(event.Subject)
	}

	var ip net.IP
	host, _, err := net.SplitHostPort(event.RemoteAddr)
	if err == nil {
		ip = net.ParseIP(host)
	}

	_, err = Insert(ctx, db, Event{
		OccurredAt:   event.Time,
		Actor:        actor,
		Action:       "auth." + strings.TrimSpace(event.Reason),
		ResourceType: "http",
		ResourceID:   event.Method + " " + event.Path,
		RequestID:    event.RequestID,
		IP:           ip,
		UserAgent:    event.UserAgent,
		Payload: map[string]any{
			"service": service,
			"status":  event.Status,
			"reason":  event.Reason,
			"error":   event.Error,
			"subject": event.Subject,
			"email":   event.Email,
			"roles":   event.Roles,
		},
	})
	return err
}
