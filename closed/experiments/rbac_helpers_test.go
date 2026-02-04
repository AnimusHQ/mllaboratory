package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/animus-labs/animus-go/closed/internal/platform/auth"
)

func TestExperimentsRequiredRoleWebhooks(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/projects/proj-1/webhooks/subscriptions", nil)
	if got := experimentsRequiredRole(req); got != auth.RoleAdmin {
		t.Fatalf("expected admin role, got %s", got)
	}
}
