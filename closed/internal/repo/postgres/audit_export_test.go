package postgres

import (
	"strings"
	"testing"
)

func TestAuditExportSinksListOrdering(t *testing.T) {
	if !strings.Contains(listAuditExportSinksQuery, "ORDER BY") {
		t.Fatalf("expected order by in list sinks query")
	}
}
