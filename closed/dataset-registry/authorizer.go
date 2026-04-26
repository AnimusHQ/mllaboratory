package main

import (
	"net/http"

	"github.com/animus-labs/animus-go/closed/internal/platform/auth"
	"github.com/animus-labs/animus-go/closed/internal/platform/rbac"
)

func requiredRoleForDatasetRegistry(r *http.Request) string {
	if r == nil {
		return auth.RoleEditor
	}
	if r.Method == http.MethodPost && r.URL.Path == "/projects" {
		return auth.RoleAdmin
	}
	return rbac.RequiredRoleFromRequest(r)
}
