package rbac

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/animus-labs/animus-go/closed/internal/platform/auth"
	"github.com/animus-labs/animus-go/closed/internal/repo"
)

type stubBindingStore struct {
	bindings []repo.RoleBindingRecord
	err      error
}

func (s stubBindingStore) ListBySubjects(ctx context.Context, projectID string, subjects []repo.RoleBindingSubject) ([]repo.RoleBindingRecord, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.bindings, nil
}

func TestAuthorizerAllowsViewerForGet(t *testing.T) {
	store := stubBindingStore{bindings: []repo.RoleBindingRecord{{Role: auth.RoleViewer}}}
	authorizer := Authorizer{Store: store}

	req := httptest.NewRequest(http.MethodGet, "http://example.test/resource", nil)
	req = req.WithContext(auth.ContextWithProjectID(req.Context(), "proj-1"))

	if err := authorizer.Authorize(req, auth.Identity{Subject: "user-1"}); err != nil {
		t.Fatalf("expected allow, got %v", err)
	}

	req.Method = http.MethodPost
	if err := authorizer.Authorize(req, auth.Identity{Subject: "user-1"}); err == nil {
		t.Fatalf("expected forbidden for viewer on write")
	}
}

func TestAuthorizerUsesDirectRolesWhenAllowed(t *testing.T) {
	authorizer := Authorizer{AllowDirect: true}
	req := httptest.NewRequest(http.MethodPost, "http://example.test/resource", nil)
	req = req.WithContext(auth.ContextWithProjectID(req.Context(), "proj-1"))

	if err := authorizer.Authorize(req, auth.Identity{Subject: "user-1", Roles: []string{auth.RoleEditor}}); err != nil {
		t.Fatalf("expected allow via direct role, got %v", err)
	}
}

func TestAuthorizerBypassesRunToken(t *testing.T) {
	authorizer := Authorizer{}
	req := httptest.NewRequest(http.MethodPost, "http://example.test/resource", nil)

	if err := authorizer.Authorize(req, auth.Identity{Subject: "run:run-123"}); err != nil {
		t.Fatalf("expected run token bypass, got %v", err)
	}
}

func TestAuthorizerBlocksRunTokenForAdmin(t *testing.T) {
	authorizer := Authorizer{
		RequiredRoleFor: func(r *http.Request) string {
			return auth.RoleAdmin
		},
	}
	req := httptest.NewRequest(http.MethodPost, "http://example.test/admin", nil)

	if err := authorizer.Authorize(req, auth.Identity{Subject: "run:run-123"}); err == nil {
		t.Fatalf("expected admin to be blocked for run token")
	}
}

func TestResolveRolePrefersBindingsWhenDirectDisabled(t *testing.T) {
	store := stubBindingStore{bindings: []repo.RoleBindingRecord{{Role: auth.RoleAdmin}}}
	role, _, err := ResolveRole(context.Background(), store, "proj-1", auth.Identity{Subject: "user-1", Roles: []string{auth.RoleViewer}}, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if role != auth.RoleAdmin {
		t.Fatalf("expected admin from bindings, got %q", role)
	}
}
