package domain

import "time"

// CodeRef identifies the exact source used for execution.
type CodeRef struct {
	RepoURL   string `json:"repoUrl"`
	CommitSHA string `json:"commitSha"`
	Path      string `json:"path,omitempty"`
	SCMType   string `json:"scmType,omitempty"`
}

// EnvironmentDefinition describes a logical execution environment.
type EnvironmentDefinition struct {
	ID                     string                 `json:"environmentDefinitionId"`
	ProjectID              string                 `json:"projectId"`
	Name                   string                 `json:"name"`
	Version                int                    `json:"version"`
	Description            string                 `json:"description,omitempty"`
	BaseImages             []EnvironmentBaseImage `json:"baseImages"`
	ResourceDefaults       EnvironmentResources   `json:"resourceDefaults,omitempty"`
	ResourceLimits         EnvironmentResources   `json:"resourceLimits,omitempty"`
	AllowedAccelerators    []string               `json:"allowedAccelerators,omitempty"`
	NetworkClassRef        string                 `json:"networkClassRef,omitempty"`
	SecretAccessClassRef   string                 `json:"secretAccessClassRef,omitempty"`
	Status                 string                 `json:"status"`
	SupersedesDefinitionID string                 `json:"supersedesDefinitionId,omitempty"`
	Metadata               Metadata               `json:"metadata,omitempty"`
	CreatedAt              time.Time              `json:"createdAt"`
	CreatedBy              string                 `json:"createdBy,omitempty"`
	IntegritySHA256        string                 `json:"integritySha256,omitempty"`
}

// EnvironmentBaseImage ties a friendly name to a mutable base image reference.
type EnvironmentBaseImage struct {
	Name string `json:"name"`
	Ref  string `json:"ref"`
}

// EnvironmentResources captures CPU/GPU/memory defaults or limits.
type EnvironmentResources struct {
	CPU    string `json:"cpu,omitempty"`
	Memory string `json:"memory,omitempty"`
	GPU    int    `json:"gpu,omitempty"`
}

// EnvLock captures the immutable execution environment bindings.
type EnvLock struct {
	LockID                       string               `json:"lockId"`
	ProjectID                    string               `json:"projectId,omitempty"`
	EnvironmentDefinitionID      string               `json:"environmentDefinitionId"`
	EnvironmentDefinitionVersion int                  `json:"environmentDefinitionVersion"`
	Images                       []EnvironmentImage   `json:"images"`
	ResourceDefaults             EnvironmentResources `json:"resourceDefaults,omitempty"`
	ResourceLimits               EnvironmentResources `json:"resourceLimits,omitempty"`
	AllowedAccelerators          []string             `json:"allowedAccelerators,omitempty"`
	NetworkClassRef              string               `json:"networkClassRef,omitempty"`
	SecretAccessClassRef         string               `json:"secretAccessClassRef,omitempty"`
	DependencyChecksums          map[string]string    `json:"dependencyChecksums,omitempty"`
	SBOMRef                      string               `json:"sbomRef,omitempty"`
	EnvHash                      string               `json:"envHash"`
	CreatedAt                    time.Time            `json:"createdAt,omitempty"`
	CreatedBy                    string               `json:"createdBy,omitempty"`
	IntegritySHA256              string               `json:"integritySha256,omitempty"`
}

// EnvironmentImage is a fully-resolved image reference with digest.
type EnvironmentImage struct {
	Name   string `json:"name"`
	Ref    string `json:"ref"`
	Digest string `json:"digest"`
}

// PolicySnapshot captures the governance and policy context at run creation.
type PolicySnapshot struct {
	SnapshotVersion string                  `json:"snapshotVersion"`
	CapturedAt      time.Time               `json:"capturedAt"`
	CapturedBy      string                  `json:"capturedBy,omitempty"`
	RBAC            PolicySnapshotRBAC      `json:"rbac"`
	Retention       PolicySnapshotRetention `json:"retention,omitempty"`
	Network         PolicySnapshotNetwork   `json:"network,omitempty"`
	Secrets         PolicySnapshotSecrets   `json:"secrets,omitempty"`
	Templates       PolicySnapshotTemplates `json:"templates,omitempty"`
	Policies        []PolicySnapshotPolicy  `json:"policies"`
	SnapshotSHA256  string                  `json:"snapshotSha256"`
}

type PolicySnapshotRBAC struct {
	Subject   string   `json:"subject"`
	Roles     []string `json:"roles"`
	ProjectID string   `json:"projectId"`
	Decision  string   `json:"decision,omitempty"`
}

type PolicySnapshotRetention struct {
	Mode            string `json:"mode"`
	PolicyID        string `json:"policyId,omitempty"`
	PolicyVersionID string `json:"policyVersionId,omitempty"`
	PolicySHA256    string `json:"policySha256,omitempty"`
	LegalHold       bool   `json:"legalHold,omitempty"`
}

type PolicySnapshotNetwork struct {
	Mode      string   `json:"mode"`
	Allowlist []string `json:"allowlist,omitempty"`
	Denylist  []string `json:"denylist,omitempty"`
	ClassRef  string   `json:"classRef,omitempty"`
}

type PolicySnapshotSecrets struct {
	Mode     string `json:"mode"`
	ClassRef string `json:"classRef,omitempty"`
}

type PolicySnapshotTemplates struct {
	Mode               string   `json:"mode"`
	AllowedTemplateIDs []string `json:"allowedTemplateIds,omitempty"`
}

type PolicySnapshotPolicy struct {
	PolicyID        string `json:"policyId"`
	PolicyName      string `json:"policyName,omitempty"`
	PolicyVersionID string `json:"policyVersionId"`
	PolicyVersion   int    `json:"policyVersion,omitempty"`
	PolicySHA256    string `json:"policySha256"`
	Status          string `json:"status"`
}
