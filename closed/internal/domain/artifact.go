package domain

import (
	"errors"
	"strings"
	"time"
)

// Artifact represents a project-scoped artifact stored in object storage.
type Artifact struct {
	ID              string
	ProjectID       string
	Kind            string
	ContentType     string
	ObjectKey       string
	SHA256          string
	SizeBytes       int64
	Metadata        Metadata
	RetentionUntil  *time.Time
	LegalHold       bool
	CreatedAt       time.Time
	CreatedBy       string
	IntegritySHA256 string
}

func (a Artifact) Validate() error {
	if strings.TrimSpace(a.ID) == "" {
		return errors.New("artifact id is required")
	}
	if strings.TrimSpace(a.ProjectID) == "" {
		return errors.New("project id is required")
	}
	if strings.TrimSpace(a.Kind) == "" {
		return errors.New("artifact kind is required")
	}
	if strings.TrimSpace(a.ObjectKey) == "" {
		return errors.New("object key is required")
	}
	if strings.TrimSpace(a.SHA256) == "" {
		return errors.New("sha256 is required")
	}
	return nil
}
