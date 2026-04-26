package auth

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

func TestInternalAuthSignature_Verify(t *testing.T) {
	secret := "test-secret"
	ts := "1700000000"
	method := "GET"
	path := "/api/dataset-registry/datasets"
	requestID := "rid-1"
	subject := "alice"
	email := "alice@example.test"
	roles := "admin,viewer"

	sig, err := ComputeInternalAuthSignature(secret, ts, method, path, requestID, subject, email, roles)
	if err != nil {
		t.Fatalf("ComputeInternalAuthSignature() err=%v", err)
	}
	if err := VerifyInternalAuthSignature(secret, ts, method, path, requestID, subject, email, roles, sig); err != nil {
		t.Fatalf("VerifyInternalAuthSignature() err=%v", err)
	}
	if err := VerifyInternalAuthSignature(secret, ts, http.MethodPost, path, requestID, subject, email, roles, sig); err == nil {
		t.Fatalf("expected signature verification to fail when method changes")
	}
}

func TestInternalAuthTimestamp_Verify(t *testing.T) {
	now := time.Unix(1700000000, 0).UTC()
	if err := VerifyInternalAuthTimestamp("1700000000", now, 5*time.Minute); err != nil {
		t.Fatalf("VerifyInternalAuthTimestamp() err=%v", err)
	}
	if err := VerifyInternalAuthTimestamp("1690000000", now, 5*time.Minute); err == nil {
		t.Fatalf("expected timestamp to be rejected")
	}
}

func TestGatewayHeadersAuthenticator(t *testing.T) {
	secret := "test-secret"
	authn, err := NewGatewayHeadersAuthenticator(secret)
	if err != nil {
		t.Fatalf("NewGatewayHeadersAuthenticator() err=%v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "http://example.test/datasets", nil)
	req.Header.Set("X-Request-Id", "rid-2")
	req.Header.Set(HeaderSubject, "alice")
	req.Header.Set(HeaderEmail, "alice@example.test")
	req.Header.Set(HeaderRoles, "admin,viewer")

	ts := strconv.FormatInt(time.Now().UTC().Unix(), 10)
	sig, err := ComputeInternalAuthSignature(secret, ts, req.Method, req.URL.Path, req.Header.Get("X-Request-Id"), "alice", "alice@example.test", "admin,viewer")
	if err != nil {
		t.Fatalf("ComputeInternalAuthSignature() err=%v", err)
	}
	req.Header.Set(HeaderInternalAuthTimestamp, ts)
	req.Header.Set(HeaderInternalAuthSignature, sig)

	identity, err := authn.Authenticate(req.Context(), req)
	if err != nil {
		t.Fatalf("Authenticate() err=%v", err)
	}
	if identity.Subject != "alice" {
		t.Fatalf("Subject=%q, want alice", identity.Subject)
	}
	if len(identity.Roles) != 2 {
		t.Fatalf("Roles=%v, want 2 roles", identity.Roles)
	}
}
