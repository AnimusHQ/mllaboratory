package postgres

import (
	"strings"
	"testing"
)

func TestWebhookDeliveriesEnqueueIsIdempotent(t *testing.T) {
	if !strings.Contains(insertWebhookDeliveryQuery, "ON CONFLICT") {
		t.Fatalf("expected ON CONFLICT in enqueue query: %s", insertWebhookDeliveryQuery)
	}
}

func TestWebhookDeliveriesQueriesProjectScoped(t *testing.T) {
	queries := []string{
		selectWebhookDeliveryQuery,
		listWebhookDeliveriesQuery,
		updateWebhookDeliveryQuery,
	}
	for _, query := range queries {
		if !strings.Contains(query, "project_id") {
			t.Fatalf("expected project scoping in query: %s", query)
		}
	}
}

func TestWebhookDeliveriesClaimOrdering(t *testing.T) {
	if !strings.Contains(claimWebhookDeliveriesQuery, "ORDER BY") {
		t.Fatalf("expected order by in claim query")
	}
}
