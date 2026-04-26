package webhooks

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

type EventType string

const (
	EventRunFinished           EventType = "RunFinished"
	EventModelApproved         EventType = "ModelApproved"
	EventDatasetVersionCreated EventType = "DatasetVersionCreated"
)

type DeliveryStatus string

const (
	DeliveryStatusPending   DeliveryStatus = "PENDING"
	DeliveryStatusDelivered DeliveryStatus = "DELIVERED"
	DeliveryStatusFailed    DeliveryStatus = "FAILED"
	DeliveryStatusDisabled  DeliveryStatus = "DISABLED"
)

type AttemptOutcome string

const (
	AttemptOutcomeSuccess          AttemptOutcome = "SUCCESS"
	AttemptOutcomeRetry            AttemptOutcome = "RETRY"
	AttemptOutcomePermanentFailure AttemptOutcome = "PERMANENT_FAILURE"
)

type SubjectRef struct {
	RunID            string `json:"run_id,omitempty"`
	ModelVersionID   string `json:"model_version_id,omitempty"`
	DatasetVersionID string `json:"dataset_version_id,omitempty"`
}

type Payload struct {
	EventID   string            `json:"event_id"`
	EventType EventType         `json:"event_type"`
	EmittedAt time.Time         `json:"emitted_at"`
	ProjectID string            `json:"project_id"`
	Subject   SubjectRef        `json:"subject"`
	Links     map[string]string `json:"api_links,omitempty"`
}

type Subscription struct {
	ID         string
	ProjectID  string
	Name       string
	TargetURL  string
	Enabled    bool
	EventTypes []EventType
	SecretRef  string
	Headers    map[string]string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type Delivery struct {
	ID             string
	ProjectID      string
	SubscriptionID string
	EventID        string
	EventType      EventType
	Payload        []byte
	Status         DeliveryStatus
	NextAttemptAt  time.Time
	AttemptCount   int
	LastError      string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type Attempt struct {
	ID          int64
	DeliveryID  string
	AttemptedAt time.Time
	StatusCode  *int
	Outcome     AttemptOutcome
	Error       string
	LatencyMs   int
	RequestID   string
	CreatedAt   time.Time
}

type Replay struct {
	ID          int64
	DeliveryID  string
	Token       string
	RequestedAt time.Time
}

func (t EventType) String() string {
	return string(t)
}

func (t EventType) Valid() bool {
	switch t {
	case EventRunFinished, EventModelApproved, EventDatasetVersionCreated:
		return true
	default:
		return false
	}
}

func NormalizeEventTypes(input []string) ([]EventType, error) {
	if len(input) == 0 {
		return nil, fmt.Errorf("event_types are required")
	}
	unique := map[EventType]struct{}{}
	out := make([]EventType, 0, len(input))
	for _, value := range input {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		et := EventType(value)
		if !et.Valid() {
			return nil, fmt.Errorf("unsupported event type: %s", value)
		}
		if _, ok := unique[et]; ok {
			continue
		}
		unique[et] = struct{}{}
		out = append(out, et)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("event_types are required")
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out, nil
}
