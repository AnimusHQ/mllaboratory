package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/animus-labs/animus-go/closed/internal/platform/auth"
)

func TestRequiredRoleForDatasetRegistry(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/projects", nil)
	if got := requiredRoleForDatasetRegistry(req); got != auth.RoleAdmin {
		t.Fatalf("POST /projects role=%q, want %q", got, auth.RoleAdmin)
	}

	req = httptest.NewRequest(http.MethodGet, "/projects", nil)
	if got := requiredRoleForDatasetRegistry(req); got != auth.RoleViewer {
		t.Fatalf("GET /projects role=%q, want %q", got, auth.RoleViewer)
	}

	req = httptest.NewRequest(http.MethodPost, "/datasets", nil)
	if got := requiredRoleForDatasetRegistry(req); got != auth.RoleEditor {
		t.Fatalf("POST /datasets role=%q, want %q", got, auth.RoleEditor)
	}
}
