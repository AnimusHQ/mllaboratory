package domain

import "time"

// Metadata is an unstructured metadata container for domain entities.
type Metadata map[string]any

func (m Metadata) Clone() Metadata {
	if m == nil {
		return Metadata{}
	}
	copy := make(Metadata, len(m))
	for k, v := range m {
		copy[k] = v
	}
	return copy
}

// Base holds common fields for immutable entities.
type Base struct {
	ID              string
	ProjectID       string
	CreatedAt       time.Time
	CreatedBy       string
	IntegritySHA256 string
	Metadata        Metadata
}
