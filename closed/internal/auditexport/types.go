package auditexport

import (
	"encoding/json"
	"time"
)

const DefaultSinkID = "default"

const (
	OutboxStatusPending   = "pending"
	OutboxStatusInflight  = "inflight"
	OutboxStatusRetry     = "retry"
	OutboxStatusDelivered = "delivered"
)

type DeliveryStatus string

const (
	DeliveryStatusPending   DeliveryStatus = "pending"
	DeliveryStatusInflight  DeliveryStatus = "inflight"
	DeliveryStatusRetry     DeliveryStatus = "retry"
	DeliveryStatusDelivered DeliveryStatus = "delivered"
	DeliveryStatusDLQ       DeliveryStatus = "dlq"
)

func (s DeliveryStatus) Valid() bool {
	switch s {
	case DeliveryStatusPending, DeliveryStatusInflight, DeliveryStatusRetry, DeliveryStatusDelivered, DeliveryStatusDLQ:
		return true
	default:
		return false
	}
}

type AttemptOutcome string

const (
	AttemptOutcomeSuccess          AttemptOutcome = "success"
	AttemptOutcomeRetry            AttemptOutcome = "retry"
	AttemptOutcomePermanentFailure AttemptOutcome = "permanent_failure"
)

func (o AttemptOutcome) Valid() bool {
	switch o {
	case AttemptOutcomeSuccess, AttemptOutcomeRetry, AttemptOutcomePermanentFailure:
		return true
	default:
		return false
	}
}

type Sink struct {
	SinkID      string
	Name        string
	Destination string
	Format      string
	Config      json.RawMessage
	Enabled     bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type OutboxRecord struct {
	OutboxID int64
	EventID  int64
	SinkID   string
	Attempt  int
}

type Delivery struct {
	DeliveryID    int64
	SinkID        string
	EventID       int64
	Status        DeliveryStatus
	AttemptCount  int
	NextAttemptAt time.Time
	LastError     string
	DLQReason     string
	DeliveredAt   *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type DeliveryAttempt struct {
	AttemptID   int64
	DeliveryID  int64
	AttemptedAt time.Time
	Outcome     AttemptOutcome
	StatusCode  *int
	Error       string
	LatencyMs   int
	CreatedAt   time.Time
}

type SinkConfig struct {
	WebhookURL     string            `json:"webhook_url,omitempty"`
	WebhookHeaders map[string]string `json:"webhook_headers,omitempty"`
	SyslogAddr     string            `json:"syslog_addr,omitempty"`
	SyslogProtocol string            `json:"syslog_protocol,omitempty"`
	SyslogTag      string            `json:"syslog_tag,omitempty"`
}
