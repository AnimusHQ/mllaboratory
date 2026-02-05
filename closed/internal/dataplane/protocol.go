package dataplane

import (
	"time"

	"github.com/animus-labs/animus-go/closed/internal/domain"
)

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

type DevEnvProvisionRequest struct {
	DevEnvID             string                      `json:"devEnvId"`
	ProjectID            string                      `json:"projectId"`
	TemplateRef          string                      `json:"templateRef"`
	TemplateVersion      int                         `json:"templateVersion,omitempty"`
	TemplateIntegritySHA string                      `json:"templateIntegritySha256,omitempty"`
	RepoURL              string                      `json:"repoUrl,omitempty"`
	RefType              string                      `json:"refType,omitempty"`
	RefValue             string                      `json:"refValue,omitempty"`
	CommitPin            string                      `json:"commitPin,omitempty"`
	ImageName            string                      `json:"imageName,omitempty"`
	ImageRef             string                      `json:"imageRef"`
	ResourceDefaults     domain.EnvironmentResources `json:"resourceDefaults,omitempty"`
	ResourceLimits       domain.EnvironmentResources `json:"resourceLimits,omitempty"`
	NetworkClassRef      string                      `json:"networkClassRef,omitempty"`
	SecretAccessClassRef string                      `json:"secretAccessClassRef,omitempty"`
	TTLSeconds           int64                       `json:"ttlSeconds"`
	EmittedAt            time.Time                   `json:"emittedAt"`
	RequestedBy          string                      `json:"requestedBy,omitempty"`
	CorrelationID        string                      `json:"correlationId,omitempty"`
}

type DevEnvProvisionResponse struct {
	DevEnvID  string `json:"devEnvId"`
	ProjectID string `json:"projectId"`
	Accepted  bool   `json:"accepted"`
	JobName   string `json:"jobName,omitempty"`
	Namespace string `json:"namespace,omitempty"`
	Message   string `json:"message,omitempty"`
}

type DevEnvAccessRequest struct {
	DevEnvID      string    `json:"devEnvId"`
	ProjectID     string    `json:"projectId"`
	SessionID     string    `json:"sessionId"`
	EmittedAt     time.Time `json:"emittedAt"`
	CorrelationID string    `json:"correlationId,omitempty"`
}

type DevEnvAccessResponse struct {
	DevEnvID  string `json:"devEnvId"`
	ProjectID string `json:"projectId"`
	Ready     bool   `json:"ready"`
	JobName   string `json:"jobName,omitempty"`
	Namespace string `json:"namespace,omitempty"`
	Message   string `json:"message,omitempty"`
}

type DevEnvDeleteRequest struct {
	DevEnvID      string    `json:"devEnvId"`
	ProjectID     string    `json:"projectId"`
	EmittedAt     time.Time `json:"emittedAt"`
	RequestedBy   string    `json:"requestedBy,omitempty"`
	CorrelationID string    `json:"correlationId,omitempty"`
}

type DevEnvDeleteResponse struct {
	DevEnvID  string `json:"devEnvId"`
	ProjectID string `json:"projectId"`
	Deleted   bool   `json:"deleted"`
	Message   string `json:"message,omitempty"`
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
