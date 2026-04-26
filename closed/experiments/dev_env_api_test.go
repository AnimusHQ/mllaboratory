package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/animus-labs/animus-go/closed/internal/dataplane"
	"github.com/animus-labs/animus-go/closed/internal/domain"
	"github.com/animus-labs/animus-go/closed/internal/platform/auth"
	"github.com/animus-labs/animus-go/closed/internal/platform/rbac"
	"github.com/animus-labs/animus-go/closed/internal/repo"
	"github.com/animus-labs/animus-go/closed/internal/repo/postgres"
)

type testAuthenticator struct {
	identity auth.Identity
	err      error
}

func (a *testAuthenticator) Authenticate(ctx context.Context, r *http.Request) (auth.Identity, error) {
	return a.identity, a.err
}

type stubBindingStore struct {
	rolesByProject map[string][]repo.RoleBindingRecord
}

func (s stubBindingStore) ListBySubjects(ctx context.Context, projectID string, subjects []repo.RoleBindingSubject) ([]repo.RoleBindingRecord, error) {
	if s.rolesByProject == nil {
		return nil, nil
	}
	return s.rolesByProject[projectID], nil
}

func TestDevEnvCreateIdempotent(t *testing.T) {
	store := newStubDevEnvStore()
	policyStore := &stubDevEnvPolicyStore{}
	audit := &captureAudit{}
	dp := &stubDevEnvDPClient{provisionResp: dataplane.DevEnvProvisionResponse{Accepted: true, JobName: "job-1", Namespace: "ns"}}
	envDef := testEnvDefinition()

	api := &experimentsAPI{
		devEnvStoreOverride:        store,
		devEnvPolicyStoreOverride:  policyStore,
		devEnvSessionStoreOverride: &stubDevEnvSessionStore{},
		devEnvDPClientOverride:     dp,
		devEnvAuditOverride:        audit,
		environmentStoreOverride:   stubEnvDefinitionStore{record: postgres.EnvironmentDefinitionRecord{Definition: envDef}},
		devEnvPolicySnapshotOverride: func(ctx context.Context, projectID string, identity auth.Identity, envLock domain.EnvLock) (domain.PolicySnapshot, error) {
			return testPolicySnapshot(), nil
		},
		devEnvDefaultTTL: time.Hour,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /projects/{project_id}/dev-environments", api.handleCreateDevEnvironment)

	body := `{"templateRef":"tmpl-1","repoUrl":"https://github.com/acme/repo","refType":"branch","refValue":"main","ttlSeconds":3600}`
	req := httptest.NewRequest(http.MethodPost, "/projects/proj-1/dev-environments", bytes.NewBufferString(body))
	req.Header.Set("Idempotency-Key", "idem-1")
	req = req.WithContext(auth.ContextWithIdentity(req.Context(), auth.Identity{Subject: "user-1"}))
	resp := httptest.NewRecorder()
	mux.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status=%d want 200", resp.Code)
	}
	var first devEnvResponse
	if err := json.NewDecoder(resp.Body).Decode(&first); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !first.Created {
		t.Fatalf("expected created=true")
	}

	req2 := httptest.NewRequest(http.MethodPost, "/projects/proj-1/dev-environments", bytes.NewBufferString(body))
	req2.Header.Set("Idempotency-Key", "idem-1")
	req2 = req2.WithContext(auth.ContextWithIdentity(req2.Context(), auth.Identity{Subject: "user-1"}))
	resp2 := httptest.NewRecorder()
	mux.ServeHTTP(resp2, req2)

	if resp2.Code != http.StatusOK {
		t.Fatalf("status=%d want 200", resp2.Code)
	}
	var second devEnvResponse
	if err := json.NewDecoder(resp2.Body).Decode(&second); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if second.Created {
		t.Fatalf("expected created=false on idempotent call")
	}
	if first.Environment.ID != second.Environment.ID {
		t.Fatalf("expected same dev env id, got %s and %s", first.Environment.ID, second.Environment.ID)
	}
	if policyStore == nil || len(policyStore.inserts) != 1 {
		t.Fatalf("expected single policy snapshot insert, got %d", len(policyStore.inserts))
	}
	if audit.count(auditDevEnvCreated) != 1 {
		t.Fatalf("expected single created audit, got %d", audit.count(auditDevEnvCreated))
	}
}

func TestDevEnvAccessEmitsAudit(t *testing.T) {
	store := newStubDevEnvStore()
	store.records["dev-1"] = postgres.DevEnvironmentRecord{Environment: domain.DevEnvironment{ID: "dev-1", ProjectID: "proj-1", State: domain.DevEnvStateActive}}
	sessions := &stubDevEnvSessionStore{}
	audit := &captureAudit{}
	dp := &stubDevEnvDPClient{accessResp: dataplane.DevEnvAccessResponse{Ready: true, Message: "ok"}}

	api := &experimentsAPI{
		devEnvStoreOverride:        store,
		devEnvSessionStoreOverride: sessions,
		devEnvDPClientOverride:     dp,
		devEnvAuditOverride:        audit,
		devEnvAccessTTL:            5 * time.Minute,
	}

	req := httptest.NewRequest(http.MethodPost, "/projects/proj-1/devenvs/dev-1:open-ide-session", bytes.NewBufferString(`{}`))
	req = req.WithContext(auth.ContextWithIdentity(req.Context(), auth.Identity{Subject: "user-1"}))
	resp := httptest.NewRecorder()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.SetPathValue("project_id", "proj-1")
		r.SetPathValue("dev_env_id", "dev-1")
		api.handleAccessDevEnvironment(w, r)
	})
	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status=%d want 200", resp.Code)
	}
	if len(sessions.sessions) != 1 {
		t.Fatalf("expected session insert")
	}
	if audit.count(auditDevEnvSessionOpened) != 1 {
		t.Fatalf("expected session opened audit")
	}
}

func TestDevEnvAccessDeniedWithoutRBAC(t *testing.T) {
	store := newStubDevEnvStore()
	api := &experimentsAPI{devEnvStoreOverride: store}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.SetPathValue("project_id", "proj-1")
		r.SetPathValue("dev_env_id", "dev-1")
		api.handleAccessDevEnvironment(w, r)
	})

	authn := &testAuthenticator{identity: auth.Identity{Subject: "user-1"}}
	authorizer := rbac.Authorizer{Store: stubBindingStore{rolesByProject: map[string][]repo.RoleBindingRecord{"proj-1": {{Role: auth.RoleViewer}}}}, AllowDirect: false}

	h := auth.Middleware{
		Authenticator: authn,
		Authorize:     authorizer.Authorize,
		ProjectResolve: func(r *http.Request, identity auth.Identity) (string, error) {
			return "proj-1", nil
		},
	}.Wrap(handler)

	req := httptest.NewRequest(http.MethodPost, "/projects/proj-1/devenvs/dev-1:open-ide-session", bytes.NewBufferString(`{}`))
	resp := httptest.NewRecorder()
	h.ServeHTTP(resp, req)

	if resp.Code != http.StatusForbidden {
		t.Fatalf("status=%d want 403", resp.Code)
	}
	if store.getCalls != 0 {
		t.Fatalf("handler should not be invoked when unauthorized")
	}
}

func TestDevEnvCreateRejectsRepoAllowlist(t *testing.T) {
	store := newStubDevEnvStore()
	policyStore := &stubDevEnvPolicyStore{}
	audit := &captureAudit{}
	dp := &stubDevEnvDPClient{provisionResp: dataplane.DevEnvProvisionResponse{Accepted: true, JobName: "job-1", Namespace: "ns"}}
	envDef := testEnvDefinition()

	api := &experimentsAPI{
		devEnvStoreOverride:        store,
		devEnvPolicyStoreOverride:  policyStore,
		devEnvSessionStoreOverride: &stubDevEnvSessionStore{},
		devEnvDPClientOverride:     dp,
		devEnvAuditOverride:        audit,
		environmentStoreOverride:   stubEnvDefinitionStore{record: postgres.EnvironmentDefinitionRecord{Definition: envDef}},
		devEnvPolicySnapshotOverride: func(ctx context.Context, projectID string, identity auth.Identity, envLock domain.EnvLock) (domain.PolicySnapshot, error) {
			return testPolicySnapshot(), nil
		},
		devEnvDefaultTTL:    time.Hour,
		devEnvRepoAllowlist: []repoAllowlistEntry{{host: "github.com", pathPrefix: "allowed"}},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /projects/{project_id}/dev-environments", api.handleCreateDevEnvironment)

	body := `{"templateRef":"tmpl-1","repoUrl":"https://github.com/other/repo","refType":"branch","refValue":"main","ttlSeconds":3600}`
	req := httptest.NewRequest(http.MethodPost, "/projects/proj-1/dev-environments", bytes.NewBufferString(body))
	req.Header.Set("Idempotency-Key", "idem-1")
	req = req.WithContext(auth.ContextWithIdentity(req.Context(), auth.Identity{Subject: "user-1"}))
	resp := httptest.NewRecorder()
	mux.ServeHTTP(resp, req)

	if resp.Code != http.StatusForbidden {
		t.Fatalf("status=%d want 403", resp.Code)
	}
}
