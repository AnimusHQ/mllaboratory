package domain

import "time"

const (
	DevEnvStateProvisioning = "provisioning"
	DevEnvStateActive       = "active"
	DevEnvStateFailed       = "failed"
	DevEnvStateExpired      = "expired"
	DevEnvStateDeleted      = "deleted"
)

const (
	DevEnvRefTypeBranch = "branch"
	DevEnvRefTypeTag    = "tag"
	DevEnvRefTypeCommit = "commit"
)

// DevEnvironment represents a governed interactive development environment.
type DevEnvironment struct {
	ID                        string     `json:"devEnvId"`
	ProjectID                 string     `json:"projectId"`
	TemplateRef               string     `json:"templateRef"`
	TemplateDefinitionID      string     `json:"templateDefinitionId,omitempty"`
	TemplateDefinitionVersion int        `json:"templateDefinitionVersion,omitempty"`
	TemplateIntegritySHA256   string     `json:"templateIntegritySha256,omitempty"`
	RepoURL                   string     `json:"repoUrl,omitempty"`
	RefType                   string     `json:"refType,omitempty"`
	RefValue                  string     `json:"refValue,omitempty"`
	CommitPin                 string     `json:"commitPin,omitempty"`
	ImageName                 string     `json:"imageName,omitempty"`
	ImageRef                  string     `json:"imageRef,omitempty"`
	TTLSeconds                int64      `json:"ttlSeconds"`
	State                     string     `json:"state"`
	CreatedAt                 time.Time  `json:"createdAt"`
	CreatedBy                 string     `json:"createdBy,omitempty"`
	ExpiresAt                 time.Time  `json:"expiresAt,omitempty"`
	LastAccessAt              *time.Time `json:"lastAccessAt,omitempty"`
	PolicySnapshotSHA256      string     `json:"policySnapshotSha256,omitempty"`
	DPJobName                 string     `json:"dpJobName,omitempty"`
	DPNamespace               string     `json:"dpNamespace,omitempty"`
	IntegritySHA256           string     `json:"integritySha256,omitempty"`
}

// DevEnvAccessSession is an immutable access session record for DevEnvironment.
type DevEnvAccessSession struct {
	SessionID string    `json:"sessionId"`
	ProjectID string    `json:"projectId"`
	DevEnvID  string    `json:"devEnvId"`
	IssuedAt  time.Time `json:"issuedAt"`
	ExpiresAt time.Time `json:"expiresAt"`
	IssuedBy  string    `json:"issuedBy,omitempty"`
}
