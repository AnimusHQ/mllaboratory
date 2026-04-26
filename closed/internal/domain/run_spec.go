package domain

import "time"

// RunSpec is an immutable execution snapshot that binds a PipelineSpec to concrete inputs.
type RunSpec struct {
	RunSpecVersion  string
	ProjectID       string
	PipelineSpec    PipelineSpec
	DatasetBindings map[string]string
	CodeRef         CodeRef
	EnvLock         EnvLock
	Parameters      Metadata
	PolicySnapshot  PolicySnapshot
	CreatedAt       time.Time
	CreatedBy       string
}
