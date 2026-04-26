package auth

import (
	"net/http"
	"testing"
)

func TestHasAtLeast(t *testing.T) {
	if !HasAtLeast([]string{"viewer"}, RoleViewer) {
		t.Fatalf("viewer should satisfy viewer")
	}
	if HasAtLeast([]string{"viewer"}, RoleEditor) {
		t.Fatalf("viewer should not satisfy editor")
	}
	if !HasAtLeast([]string{"editor"}, RoleViewer) {
		t.Fatalf("editor should satisfy viewer")
	}
	if !HasAtLeast([]string{"admin"}, RoleEditor) {
		t.Fatalf("admin should satisfy editor")
	}
}

func TestRequiredRoleForRequest(t *testing.T) {
	req, _ := http.NewRequest(http.MethodGet, "http://example.test/", nil)
	if got := RequiredRoleForRequest(req); got != RoleViewer {
		t.Fatalf("RequiredRoleForRequest(GET)=%q, want viewer", got)
	}
	req.Method = http.MethodPost
	if got := RequiredRoleForRequest(req); got != RoleEditor {
		t.Fatalf("RequiredRoleForRequest(POST)=%q, want editor", got)
	}
}
