package postgres

import (
	"strings"
	"testing"
)

func TestAuditExportDeliveriesBackfillIdempotent(t *testing.T) {
	if !strings.Contains(backfillAuditExportDeliveriesQuery, "ON CONFLICT") {
		t.Fatalf("expected ON CONFLICT in backfill query")
	}
}

func TestAuditExportDeliveriesClaimOrdering(t *testing.T) {
	if !strings.Contains(claimAuditExportDeliveriesQuery, "ORDER BY") {
		t.Fatalf("expected order by in claim query")
	}
}

func TestAuditExportDeliveriesListFilters(t *testing.T) {
	if !strings.Contains(listAuditExportDeliveriesQuery, "status") {
		t.Fatalf("expected status filter in list query")
	}
	if !strings.Contains(listAuditExportDeliveriesQuery, "sink_id") {
		t.Fatalf("expected sink_id filter in list query")
	}
}

func TestAuditExportDeliveriesReplayUpdatesStatus(t *testing.T) {
	if !strings.Contains(replayAuditExportDeliveryQuery, "status") {
		t.Fatalf("expected status clause in replay query")
	}
}
