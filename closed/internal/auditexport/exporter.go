package auditexport

import (
	"context"

	"github.com/animus-labs/animus-go/closed/internal/domain"
)

// Exporter sends audit events to external systems.
type Exporter interface {
	Export(ctx context.Context, event domain.AuditEvent) error
}

// NoopExporter is a stub exporter for append-only pipelines.
type NoopExporter struct{}

func (NoopExporter) Export(ctx context.Context, event domain.AuditEvent) error {
	return nil
}
