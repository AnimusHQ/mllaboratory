package domain

import (
	"errors"
	"strings"
	"time"
)

// ModelStatus represents the lifecycle state of a model.
type ModelStatus string

const (
	ModelStatusDraft      ModelStatus = "draft"
	ModelStatusValidated  ModelStatus = "validated"
	ModelStatusApproved   ModelStatus = "approved"
	ModelStatusDeprecated ModelStatus = "deprecated"
)

// Model is a versioned model entity with lifecycle state.
type Model struct {
	ID              string      `json:"modelId"`
	ProjectID       string      `json:"projectId"`
	Name            string      `json:"name"`
	Version         string      `json:"version"`
	Status          ModelStatus `json:"status"`
	ArtifactID      string      `json:"artifactId,omitempty"`
	Metadata        Metadata    `json:"metadata,omitempty"`
	CreatedAt       time.Time   `json:"createdAt"`
	CreatedBy       string      `json:"createdBy,omitempty"`
	IntegritySHA256 string      `json:"integritySha256,omitempty"`
}

func (m Model) Validate() error {
	if strings.TrimSpace(m.ID) == "" {
		return errors.New("model id is required")
	}
	if strings.TrimSpace(m.ProjectID) == "" {
		return errors.New("project id is required")
	}
	if strings.TrimSpace(m.Name) == "" {
		return errors.New("model name is required")
	}
	if !m.Status.Valid() {
		return errors.New("invalid model status")
	}
	return nil
}

func (s ModelStatus) Valid() bool {
	switch s {
	case ModelStatusDraft, ModelStatusValidated, ModelStatusApproved, ModelStatusDeprecated:
		return true
	default:
		return false
	}
}
