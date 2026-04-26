package domain

import (
	"errors"
	"strings"
	"time"
)

// Project is a hard isolation boundary for an ML product.
type Project struct {
	ID              string
	Name            string
	Description     string
	Metadata        Metadata
	CreatedAt       time.Time
	CreatedBy       string
	IntegritySHA256 string
}

func (p Project) Validate() error {
	if strings.TrimSpace(p.ID) == "" {
		return errors.New("project id is required")
	}
	if strings.TrimSpace(p.Name) == "" {
		return errors.New("project name is required")
	}
	if strings.TrimSpace(p.IntegritySHA256) == "" {
		return errors.New("integrity sha256 is required")
	}
	return nil
}
