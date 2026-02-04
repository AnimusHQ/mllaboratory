package postgres

import (
	"strings"
	"testing"
)

func TestAuditExportAttemptsListOrdering(t *testing.T) {
	if !strings.Contains(listAuditExportAttemptsQuery, "ORDER BY") {
		t.Fatalf("expected order by in list query")
	}
}
