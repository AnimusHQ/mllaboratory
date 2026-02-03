package auditlog

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/animus-labs/animus-go/closed/internal/platform/redaction"
)

type Event struct {
	OccurredAt   time.Time
	Actor        string
	Action       string
	ResourceType string
	ResourceID   string
	RequestID    string
	IP           net.IP
	UserAgent    string
	Payload      any
}

type QueryRower interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func (e Event) Validate() error {
	if e.OccurredAt.IsZero() {
		return errors.New("OccurredAt is required")
	}
	if strings.TrimSpace(e.Actor) == "" {
		return errors.New("Actor is required")
	}
	if strings.TrimSpace(e.Action) == "" {
		return errors.New("Action is required")
	}
	if strings.TrimSpace(e.ResourceType) == "" {
		return errors.New("ResourceType is required")
	}
	if strings.TrimSpace(e.ResourceID) == "" {
		return errors.New("ResourceID is required")
	}
	return nil
}

func Insert(ctx context.Context, q QueryRower, event Event) (int64, error) {
	if q == nil {
		return 0, errors.New("queryer is required")
	}
	if event.OccurredAt.IsZero() {
		event.OccurredAt = time.Now().UTC()
	}
	if err := event.Validate(); err != nil {
		return 0, err
	}

	payload := event.Payload
	if payload == nil {
		payload = map[string]any{}
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return 0, fmt.Errorf("marshal payload: %w", err)
	}
	payloadJSON = redaction.RedactJSON(payloadJSON)

	ipStr := strings.TrimSpace(event.IP.String())
	integrity, err := ComputeIntegritySHA256(event, payloadJSON)
	if err != nil {
		return 0, err
	}

	var requestID sql.NullString
	if strings.TrimSpace(event.RequestID) != "" {
		requestID = sql.NullString{String: strings.TrimSpace(event.RequestID), Valid: true}
	}
	var ip sql.NullString
	if ipStr != "" && ipStr != "<nil>" {
		ip = sql.NullString{String: ipStr, Valid: true}
	}
	var userAgent sql.NullString
	if strings.TrimSpace(event.UserAgent) != "" {
		userAgent = sql.NullString{String: strings.TrimSpace(event.UserAgent), Valid: true}
	}

	var id int64
	err = q.QueryRowContext(
		ctx,
		`INSERT INTO audit_events (
			occurred_at,
			actor,
			action,
			resource_type,
			resource_id,
			request_id,
			ip,
			user_agent,
			payload,
			integrity_sha256
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		RETURNING event_id`,
		event.OccurredAt.UTC(),
		strings.TrimSpace(event.Actor),
		strings.TrimSpace(event.Action),
		strings.TrimSpace(event.ResourceType),
		strings.TrimSpace(event.ResourceID),
		requestID,
		ip,
		userAgent,
		payloadJSON,
		integrity,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("insert audit event: %w", err)
	}
	return id, nil
}

func ComputeIntegritySHA256(event Event, payloadJSON []byte) (string, error) {
	type integrityInput struct {
		OccurredAt   time.Time       `json:"occurred_at"`
		Actor        string          `json:"actor"`
		Action       string          `json:"action"`
		ResourceType string          `json:"resource_type"`
		ResourceID   string          `json:"resource_id"`
		RequestID    string          `json:"request_id,omitempty"`
		IP           string          `json:"ip,omitempty"`
		UserAgent    string          `json:"user_agent,omitempty"`
		Payload      json.RawMessage `json:"payload"`
	}

	ipStr := strings.TrimSpace(event.IP.String())
	if ipStr == "<nil>" {
		ipStr = ""
	}

	in := integrityInput{
		OccurredAt:   event.OccurredAt.UTC(),
		Actor:        strings.TrimSpace(event.Actor),
		Action:       strings.TrimSpace(event.Action),
		ResourceType: strings.TrimSpace(event.ResourceType),
		ResourceID:   strings.TrimSpace(event.ResourceID),
		RequestID:    strings.TrimSpace(event.RequestID),
		IP:           ipStr,
		UserAgent:    strings.TrimSpace(event.UserAgent),
		Payload:      payloadJSON,
	}

	blob, err := json.Marshal(in)
	if err != nil {
		return "", fmt.Errorf("marshal integrity: %w", err)
	}
	sum := sha256.Sum256(blob)
	return hex.EncodeToString(sum[:]), nil
}
