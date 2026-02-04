package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/animus-labs/animus-go/closed/internal/platform/rbac"
	"github.com/animus-labs/animus-go/closed/internal/repo"
)

type stubBindingStore struct {
	rolesByProject map[string][]repo.RoleBindingRecord
	err            error
}

func (s stubBindingStore) ListBySubjects(ctx context.Context, projectID string, subjects []repo.RoleBindingSubject) ([]repo.RoleBindingRecord, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.rolesByProject == nil {
		return nil, nil
	}
	return s.rolesByProject[projectID], nil
}

func TestMiddleware_DeniesCrossProjectAccess(t *testing.T) {
	store := stubBindingStore{
		rolesByProject: map[string][]repo.RoleBindingRecord{
			"proj-1": {{Role: RoleAdmin}},
		},
	}
	authorizer := rbac.Authorizer{Store: store, AllowDirect: false}
	authn := &testAuthenticator{identity: Identity{Subject: "user-1"}}

	h := Middleware{
		Authenticator: authn,
		Authorize:     authorizer.Authorize,
		ProjectResolve: func(r *http.Request, identity Identity) (string, error) {
			return "proj-2", nil
		},
	}.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "http://example.test/resource", nil)
	req.Header.Set("X-Request-Id", "rid-cross")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d, want 403", rec.Code)
	}
}

func TestMiddleware_DeniesRoleDowngrade(t *testing.T) {
	store := stubBindingStore{
		rolesByProject: map[string][]repo.RoleBindingRecord{
			"proj-1": {{Role: RoleViewer}},
		},
	}
	authorizer := rbac.Authorizer{Store: store, AllowDirect: false}
	authn := &testAuthenticator{identity: Identity{Subject: "user-1"}}

	h := Middleware{
		Authenticator: authn,
		Authorize:     authorizer.Authorize,
		ProjectResolve: func(r *http.Request, identity Identity) (string, error) {
			return "proj-1", nil
		},
	}.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "http://example.test/resource", nil)
	req.Header.Set("X-Request-Id", "rid-downgrade")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d, want 403", rec.Code)
	}
}

func TestMiddleware_ProjectRequired(t *testing.T) {
	authn := &testAuthenticator{identity: Identity{Subject: "user-1"}}
	h := Middleware{
		Authenticator: authn,
		ProjectResolve: func(r *http.Request, identity Identity) (string, error) {
			return "", ErrProjectRequired
		},
	}.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "http://example.test/resource", nil)
	req.Header.Set("X-Request-Id", "rid-project")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want 400", rec.Code)
	}
}
