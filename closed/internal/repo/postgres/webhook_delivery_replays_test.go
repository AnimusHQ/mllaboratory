package postgres

import (
	"strings"
	"testing"
)

func TestWebhookDeliveryReplayInsertIdempotent(t *testing.T) {
	if !strings.Contains(insertWebhookDeliveryReplayQuery, "ON CONFLICT") {
		t.Fatalf("expected ON CONFLICT in replay insert query: %s", insertWebhookDeliveryReplayQuery)
	}
}
