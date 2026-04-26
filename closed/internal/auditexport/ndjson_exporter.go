package auditexport

import (
	"context"
	"encoding/json"
	"io"
	"net"

	"github.com/animus-labs/animus-go/closed/internal/domain"
)

// NDJSONExporter writes audit events as newline-delimited JSON.
type NDJSONExporter struct {
	enc *json.Encoder
}

func NewNDJSONExporter(w io.Writer) *NDJSONExporter {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(true)
	return &NDJSONExporter{enc: enc}
}

func (e *NDJSONExporter) Export(ctx context.Context, event domain.AuditEvent) error {
	return e.enc.Encode(exportEventFromDomain(event))
}

type exportEvent struct {
	EventID         int64           `json:"event_id"`
	OccurredAt      string          `json:"occurred_at"`
	Actor           string          `json:"actor"`
	Action          string          `json:"action"`
	ResourceType    string          `json:"resource_type"`
	ResourceID      string          `json:"resource_id"`
	RequestID       string          `json:"request_id,omitempty"`
	IP              string          `json:"ip,omitempty"`
	UserAgent       string          `json:"user_agent,omitempty"`
	Payload         json.RawMessage `json:"payload"`
	IntegritySHA256 string          `json:"integrity_sha256"`
}

func exportEventFromDomain(event domain.AuditEvent) exportEvent {
	payload, _ := json.Marshal(event.Payload)
	return exportEvent{
		EventID:         event.EventID,
		OccurredAt:      event.OccurredAt.UTC().Format(timeFormatRFC3339Nano),
		Actor:           event.Actor,
		Action:          event.Action,
		ResourceType:    event.ResourceType,
		ResourceID:      event.ResourceID,
		RequestID:       event.RequestID,
		IP:              ipString(event.IP),
		UserAgent:       event.UserAgent,
		Payload:         payload,
		IntegritySHA256: event.IntegritySHA256,
	}
}

func ipString(ip net.IP) string {
	if ip == nil {
		return ""
	}
	return ip.String()
}

const timeFormatRFC3339Nano = "2006-01-02T15:04:05.999999999Z07:00"
