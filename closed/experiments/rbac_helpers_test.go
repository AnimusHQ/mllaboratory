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

func TestExperimentsRequiredRoleModelTransitions(t *testing.T) {
	paths := []string{
		"/projects/proj-1/model-versions/ver-1:approve",
		"/projects/proj-1/model-versions/ver-1:deprecate",
		"/projects/proj-1/model-versions/ver-1:export",
	}
	for _, path := range paths {
		req := httptest.NewRequest(http.MethodPost, path, nil)
		if got := experimentsRequiredRole(req); got != auth.RoleAdmin {
			t.Fatalf("path %s expected admin role, got %s", path, got)
		}
	}
}

func TestExperimentsRequiredRoleRoleBindings(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/projects/proj-1/role-bindings", nil)
	if got := experimentsRequiredRole(req); got != auth.RoleAdmin {
		t.Fatalf("expected admin role, got %s", got)
	}
}
