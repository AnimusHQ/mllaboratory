package postgres

import (
	"strings"
	"testing"
)

func TestWebhookSubscriptionQueriesProjectScoped(t *testing.T) {
	queries := []string{
		selectWebhookSubscriptionQuery,
		listWebhookSubscriptionsQuery,
		listWebhookSubscriptionsForEventQuery,
	}
	for _, query := range queries {
		if !strings.Contains(query, "project_id") {
			t.Fatalf("expected project scoping in query: %s", query)
		}
	}
}

func TestWebhookSubscriptionEventFilter(t *testing.T) {
	if !strings.Contains(listWebhookSubscriptionsForEventQuery, "ANY(event_types)") {
		t.Fatalf("expected event_types filter in query: %s", listWebhookSubscriptionsForEventQuery)
	}
}
