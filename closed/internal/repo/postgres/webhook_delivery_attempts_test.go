package postgres

import (
	"strings"
	"testing"
)

func TestWebhookDeliveryAttemptsQueryScoped(t *testing.T) {
	if !strings.Contains(listWebhookDeliveryAttemptsQuery, "delivery_id") {
		t.Fatalf("expected delivery_id filter in query: %s", listWebhookDeliveryAttemptsQuery)
	}
}
