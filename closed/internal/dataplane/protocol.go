package dataplane

import "time"

const (
	EventTypeHeartbeat         = "heartbeat"
	EventTypeTerminal          = "terminal"
	EventTypeArtifactCommitted = "artifact_committed"
	EventTypeSecretAccessed    = "secret_accessed"
)

const (
	DispatchStatusRequested = "requested"
	DispatchStatusAccepted  = "accepted"
	DispatchStatusRejected  = "rejected"
	DispatchStatusRunning   = "running"
	DispatchStatusSucceeded = "succeeded"
	DispatchStatusFailed    = "failed"
	DispatchStatusCanceled  = "canceled"
	DispatchStatusError     = "error"
)

type RunExecutionRequest struct {
	RunID         string    `json:"runId"`
	ProjectID     string    `json:"projectId"`
	DispatchID    string    `json:"dispatchId"`
	EmittedAt     time.Time `json:"emittedAt"`
	RequestedBy   string    `json:"requestedBy,omitempty"`
	CorrelationID string    `json:"correlationId,omitempty"`
}

type RunExecutionResponse struct {
	RunID      string `json:"runId"`
	ProjectID  string `json:"projectId"`
	DispatchID string `json:"dispatchId"`
	Accepted   bool   `json:"accepted"`
	JobName    string `json:"jobName,omitempty"`
	Namespace  string `json:"namespace,omitempty"`
	Message    string `json:"message,omitempty"`
}

type RunExecutionStatus struct {
	RunID      string     `json:"runId"`
	ProjectID  string     `json:"projectId"`
	State      string     `json:"state"`
	JobName    string     `json:"jobName,omitempty"`
	Namespace  string     `json:"namespace,omitempty"`
	StartedAt  *time.Time `json:"startedAt,omitempty"`
	FinishedAt *time.Time `json:"finishedAt,omitempty"`
	Reason     string     `json:"reason,omitempty"`
}

type RunHeartbeat struct {
	EventID       string         `json:"eventId"`
	RunID         string         `json:"runId"`
	ProjectID     string         `json:"projectId"`
	EmittedAt     time.Time      `json:"emittedAt"`
	CorrelationID string         `json:"correlationId,omitempty"`
	Details       map[string]any `json:"details,omitempty"`
}

type RunHeartbeatResponse struct {
	Accepted  bool `json:"accepted"`
	Duplicate bool `json:"duplicate"`
}

type RunTerminalState struct {
	EventID       string         `json:"eventId"`
	RunID         string         `json:"runId"`
	ProjectID     string         `json:"projectId"`
	State         string         `json:"state"`
	EmittedAt     time.Time      `json:"emittedAt"`
	FinishedAt    *time.Time     `json:"finishedAt,omitempty"`
	Reason        string         `json:"reason,omitempty"`
	ExitCode      *int           `json:"exitCode,omitempty"`
	CorrelationID string         `json:"correlationId,omitempty"`
	Details       map[string]any `json:"details,omitempty"`
}

type RunTerminalResponse struct {
	Accepted  bool `json:"accepted"`
	Duplicate bool `json:"duplicate"`
}

type ArtifactCommitted struct {
	EventID       string         `json:"eventId"`
	RunID         string         `json:"runId"`
	ProjectID     string         `json:"projectId"`
	EmittedAt     time.Time      `json:"emittedAt"`
	CorrelationID string         `json:"correlationId,omitempty"`
	Payload       map[string]any `json:"payload,omitempty"`
}

type ArtifactCommittedResponse struct {
	Accepted  bool `json:"accepted"`
	Duplicate bool `json:"duplicate"`
}

type SecretAccessed struct {
	EventID       string         `json:"eventId"`
	RunID         string         `json:"runId"`
	ProjectID     string         `json:"projectId"`
	ClassRef      string         `json:"classRef,omitempty"`
	LeaseID       string         `json:"leaseId,omitempty"`
	Subject       string         `json:"subject,omitempty"`
	EmittedAt     time.Time      `json:"emittedAt"`
	CorrelationID string         `json:"correlationId,omitempty"`
	Details       map[string]any `json:"details,omitempty"`
}

type SecretAccessedResponse struct {
	Accepted  bool `json:"accepted"`
	Duplicate bool `json:"duplicate"`
}
