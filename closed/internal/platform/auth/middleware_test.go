package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type testAuthenticator struct {
	identity Identity
	err      error
	calls    int
}

func (a *testAuthenticator) Authenticate(ctx context.Context, r *http.Request) (Identity, error) {
	a.calls++
	return a.identity, a.err
}

func TestMiddleware_Unauthorized(t *testing.T) {
	authn := &testAuthenticator{err: ErrUnauthenticated}
	called := false
	h := Middleware{
		Authenticator: authn,
	}.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "http://example.test/api", nil)
	req.Header.Set("X-Request-Id", "rid-1")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if called {
		t.Fatalf("handler should not be called")
	}
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d, want 401", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if body["error"] != "unauthorized" {
		t.Fatalf("error=%v, want unauthorized", body["error"])
	}
	if body["request_id"] != "rid-1" {
		t.Fatalf("request_id=%v, want rid-1", body["request_id"])
	}
}

func TestMiddleware_InvalidToken(t *testing.T) {
	authn := &testAuthenticator{err: errors.New("bad token")}
	h := Middleware{
		Authenticator: authn,
	}.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "http://example.test/api", nil)
	req.Header.Set("X-Request-Id", "rid-2")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d, want 401", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if body["error"] != "invalid_token" {
		t.Fatalf("error=%v, want invalid_token", body["error"])
	}
}

func TestMiddleware_Forbidden(t *testing.T) {
	authn := &testAuthenticator{identity: Identity{Subject: "alice", Roles: []string{"viewer"}}}
	h := Middleware{
		Authenticator: authn,
		Authorize:     MethodRoleAuthorizer(),
	}.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "http://example.test/api", nil)
	req.Header.Set("X-Request-Id", "rid-3")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d, want 403", rec.Code)
	}
}

func TestMiddleware_SkipPrefix(t *testing.T) {
	authn := &testAuthenticator{err: ErrUnauthenticated}
	called := false
	h := Middleware{
		Authenticator: authn,
		SkipPrefixes:  []string{"/healthz"},
	}.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "http://example.test/healthz", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if !called {
		t.Fatalf("handler should be called")
	}
	if authn.calls != 0 {
		t.Fatalf("authenticator calls=%d, want 0", authn.calls)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200", rec.Code)
	}
}

func TestMiddleware_AuditOnDeny(t *testing.T) {
	authn := &testAuthenticator{err: ErrUnauthenticated}
	var got DenyEvent
	calls := 0
	h := Middleware{
		Authenticator: authn,
		Audit: func(ctx context.Context, event DenyEvent) error {
			calls++
			got = event
			return nil
		},
	}.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "http://example.test/api", nil)
	req.Header.Set("X-Request-Id", "rid-4")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if calls != 1 {
		t.Fatalf("audit calls=%d, want 1", calls)
	}
	if got.Reason != "unauthenticated" {
		t.Fatalf("Reason=%q, want unauthenticated", got.Reason)
	}
	if got.RequestID != "rid-4" {
		t.Fatalf("RequestID=%q, want rid-4", got.RequestID)
	}
}
