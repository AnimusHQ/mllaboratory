package domain

import (
	"errors"
	"strings"
	"time"
)

// Run represents a single execution run.
type Run struct {
	ID               string
	ProjectID        string
	ExperimentID     string
	DatasetVersionID string
	Status           string
	StartedAt        time.Time
	EndedAt          *time.Time
	GitRepo          string
	GitCommit        string
	GitRef           string
	Params           Metadata
	Metrics          Metadata
	ArtifactsPrefix  string
	IntegritySHA256  string
}

// PipelineRun represents an orchestrated pipeline execution.
type PipelineRun struct {
	ID              string
	ProjectID       string
	PipelineID      string
	Status          string
	StartedAt       time.Time
	EndedAt         *time.Time
	Metadata        Metadata
	IntegritySHA256 string
}

func (r Run) Validate() error {
	if strings.TrimSpace(r.ID) == "" {
		return errors.New("run id is required")
	}
	if strings.TrimSpace(r.ProjectID) == "" {
		return errors.New("project id is required")
	}
	if strings.TrimSpace(r.ExperimentID) == "" {
		return errors.New("experiment id is required")
	}
	if strings.TrimSpace(r.Status) == "" {
		return errors.New("status is required")
	}
	if strings.TrimSpace(r.IntegritySHA256) == "" {
		return errors.New("integrity sha256 is required")
	}
	return nil
}
