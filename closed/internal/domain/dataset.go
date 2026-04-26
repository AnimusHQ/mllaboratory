package domain

import (
	"errors"
	"strings"
	"time"
)

// Dataset is a top-level dataset entity scoped to a project.
type Dataset struct {
	ID              string
	ProjectID       string
	Name            string
	Description     string
	Metadata        Metadata
	CreatedAt       time.Time
	CreatedBy       string
	IntegritySHA256 string
}

// DatasetVersion is an immutable snapshot of a dataset.
type DatasetVersion struct {
	ID              string
	ProjectID       string
	DatasetID       string
	QualityRuleID   string
	Ordinal         int64
	ContentSHA256   string
	ObjectKey       string
	SizeBytes       int64
	Metadata        Metadata
	CreatedAt       time.Time
	CreatedBy       string
	IntegritySHA256 string
}

func (d Dataset) Validate() error {
	if strings.TrimSpace(d.ID) == "" {
		return errors.New("dataset id is required")
	}
	if strings.TrimSpace(d.ProjectID) == "" {
		return errors.New("project id is required")
	}
	if strings.TrimSpace(d.Name) == "" {
		return errors.New("dataset name is required")
	}
	return nil
}

func (v DatasetVersion) Validate() error {
	if strings.TrimSpace(v.ID) == "" {
		return errors.New("dataset version id is required")
	}
	if strings.TrimSpace(v.ProjectID) == "" {
		return errors.New("project id is required")
	}
	if strings.TrimSpace(v.DatasetID) == "" {
		return errors.New("dataset id is required")
	}
	if strings.TrimSpace(v.ContentSHA256) == "" {
		return errors.New("content sha256 is required")
	}
	if strings.TrimSpace(v.ObjectKey) == "" {
		return errors.New("object key is required")
	}
	return nil
}
