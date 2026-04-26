package postgres

import (
	"strings"
	"testing"
)

func TestAuditExportReplaysIdempotent(t *testing.T) {
	if !strings.Contains(insertAuditExportReplayQuery, "ON CONFLICT") {
		t.Fatalf("expected ON CONFLICT in replay insert query")
	}
}
