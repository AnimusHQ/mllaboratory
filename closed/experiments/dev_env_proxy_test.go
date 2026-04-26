package main

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/animus-labs/animus-go/closed/internal/domain"
	"github.com/animus-labs/animus-go/closed/internal/platform/auth"
	"github.com/animus-labs/animus-go/closed/internal/repo/postgres"
)

type captureRoundTripper struct {
	last *http.Request
	resp *http.Response
	err  error
}

func (c *captureRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	c.last = req.Clone(req.Context())
	if c.err != nil {
		return nil, c.err
	}
	if c.resp != nil {
		return c.resp, nil
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(bytes.NewBufferString("ok")),
		Request:    req,
	}, nil
}

func TestDevEnvProxyRejectsExpiredSession(t *testing.T) {
	now := time.Now().UTC()
	store := newStubDevEnvStore()
	store.records["dev-1"] = postgres.DevEnvironmentRecord{
		Environment: domain.DevEnvironment{
			ID:          "dev-1",
			ProjectID:   "proj-1",
			State:       domain.DevEnvStateActive,
			ExpiresAt:   now.Add(10 * time.Minute),
			DPNamespace: "ns-1",
		},
	}
	sessions := &stubDevEnvSessionStore{
		byID: map[string]domain.DevEnvAccessSession{
			"sess-1": {
				SessionID: "sess-1",
				ProjectID: "proj-1",
				DevEnvID:  "dev-1",
				ExpiresAt: now.Add(-1 * time.Minute),
			},
		},
	}

	api := &experimentsAPI{
		devEnvStoreOverride:        store,
		devEnvSessionStoreOverride: sessions,
		devEnvServiceDomain:        "svc.cluster.local",
		devEnvCodeServerPort:       8080,
	}

	req := httptest.NewRequest(http.MethodGet, "/devenv-sessions/sess-1/proxy/", nil)
	req = req.WithContext(auth.ContextWithIdentity(req.Context(), auth.Identity{Subject: "user-1"}))
	req = req.WithContext(auth.ContextWithProjectID(req.Context(), "proj-1"))
	req.SetPathValue("session_id", "sess-1")
	req.SetPathValue("path", "")
	resp := httptest.NewRecorder()

	api.handleDevEnvProxy(resp, req)

	if resp.Code != http.StatusForbidden {
		t.Fatalf("status=%d want 403", resp.Code)
	}
}

func TestDevEnvProxyUsesTransport(t *testing.T) {
	now := time.Now().UTC()
	store := newStubDevEnvStore()
	store.records["dev-1"] = postgres.DevEnvironmentRecord{
		Environment: domain.DevEnvironment{
			ID:          "dev-1",
			ProjectID:   "proj-1",
			State:       domain.DevEnvStateActive,
			ExpiresAt:   now.Add(10 * time.Minute),
			DPNamespace: "ns-1",
		},
	}
	sessions := &stubDevEnvSessionStore{
		byID: map[string]domain.DevEnvAccessSession{
			"sess-1": {
				SessionID: "sess-1",
				ProjectID: "proj-1",
				DevEnvID:  "dev-1",
				ExpiresAt: now.Add(10 * time.Minute),
			},
		},
	}
	audit := &captureAudit{}
	transport := &captureRoundTripper{}
	api := &experimentsAPI{
		devEnvStoreOverride:        store,
		devEnvSessionStoreOverride: sessions,
		devEnvAuditOverride:        audit,
		devEnvServiceDomain:        "svc.cluster.local",
		devEnvCodeServerPort:       8080,
		devEnvAccessAuditInterval:  time.Minute,
		devEnvProxyTransport:       transport,
	}

	req := httptest.NewRequest(http.MethodGet, "/devenv-sessions/sess-1/proxy/editor?x=1", nil)
	req = req.WithContext(auth.ContextWithIdentity(req.Context(), auth.Identity{Subject: "user-1"}))
	req = req.WithContext(auth.ContextWithProjectID(req.Context(), "proj-1"))
	req.SetPathValue("session_id", "sess-1")
	req.SetPathValue("path", "editor")
	resp := httptest.NewRecorder()

	api.handleDevEnvProxy(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status=%d want 200", resp.Code)
	}
	if body := strings.TrimSpace(resp.Body.String()); body != "ok" {
		t.Fatalf("body=%q want ok", body)
	}
	if transport.last == nil {
		t.Fatalf("expected transport to be called")
	}
	if got := transport.last.URL.Host; got != "animus-devenv-dev-1.ns-1.svc.cluster.local:8080" {
		t.Fatalf("host=%q want service host", got)
	}
	if got := transport.last.URL.Path; got != "/editor" {
		t.Fatalf("path=%q want /editor", got)
	}
	if audit.count(auditDevEnvSessionAccess) != 1 {
		t.Fatalf("expected session access audit")
	}
}
