package secrets

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func newResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}

func writeTempJWT(t *testing.T, value string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(path, []byte(value), 0o600); err != nil {
		t.Fatalf("write jwt: %v", err)
	}
	return path
}

func TestVaultK8sFetchSuccess(t *testing.T) {
	jwt := "jwt-token"
	authToken := "vault-token"
	secretValue := "super-secret"
	jwtPath := writeTempJWT(t, jwt)

	authCalled := false
	secretCalled := false
	rt := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/v1/auth/kubernetes/login":
			authCalled = true
			var body map[string]string
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				return newResponse(http.StatusBadRequest, ""), nil
			}
			if body["role"] != "role" || body["jwt"] != jwt {
				return newResponse(http.StatusBadRequest, ""), nil
			}
			payload, _ := json.Marshal(map[string]any{
				"auth": map[string]any{
					"client_token":   authToken,
					"lease_duration": 60,
				},
			})
			return newResponse(http.StatusOK, string(payload)), nil
		case "/v1/secret/data/app":
			secretCalled = true
			if r.Header.Get("X-Vault-Token") != authToken {
				return newResponse(http.StatusForbidden, ""), nil
			}
			payload, _ := json.Marshal(map[string]any{
				"lease_id":       "lease-123",
				"lease_duration": 30,
				"data": map[string]any{
					"data": map[string]any{
						"API_KEY": secretValue,
					},
				},
			})
			return newResponse(http.StatusOK, string(payload)), nil
		default:
			return newResponse(http.StatusNotFound, ""), nil
		}
	})

	cfg := Config{
		Provider:      "vault_k8s",
		VaultAddr:     "http://vault.test",
		VaultRole:     "role",
		VaultAuthPath: "auth/kubernetes/login",
		VaultJWTPath:  jwtPath,
		VaultTimeout:  2 * time.Second,
		LeaseTTL:      10 * time.Second,
	}
	mgr, err := NewVaultK8sManager(cfg)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	mgr.http = &http.Client{Transport: rt}

	lease, err := mgr.Fetch(context.Background(), Request{ClassRef: "secret/data/app"})
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if !authCalled || !secretCalled {
		t.Fatalf("expected auth and secret calls")
	}
	if lease.Env["API_KEY"] != secretValue {
		t.Fatalf("unexpected secret value")
	}
	if lease.LeaseID != "lease-123" {
		t.Fatalf("unexpected lease id: %s", lease.LeaseID)
	}
	remaining := time.Until(lease.ExpiresAt)
	if remaining < 20*time.Second || remaining > 40*time.Second {
		t.Fatalf("unexpected lease ttl: %s", remaining)
	}
}

func TestVaultK8sFetchUnauthorizedDoesNotLeak(t *testing.T) {
	jwt := "jwt-secret"
	authToken := "vault-secret-token"
	secretLeak := "very-secret-value"
	jwtPath := writeTempJWT(t, jwt)

	rt := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/v1/auth/kubernetes/login":
			payload, _ := json.Marshal(map[string]any{
				"auth": map[string]any{
					"client_token":   authToken,
					"lease_duration": 60,
				},
			})
			return newResponse(http.StatusOK, string(payload)), nil
		case "/v1/secret/data/app":
			return newResponse(http.StatusForbidden, secretLeak), nil
		default:
			return newResponse(http.StatusNotFound, ""), nil
		}
	})

	cfg := Config{
		Provider:      "vault_k8s",
		VaultAddr:     "http://vault.test",
		VaultRole:     "role",
		VaultAuthPath: "auth/kubernetes/login",
		VaultJWTPath:  jwtPath,
		VaultTimeout:  2 * time.Second,
		LeaseTTL:      10 * time.Second,
	}
	mgr, err := NewVaultK8sManager(cfg)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	mgr.http = &http.Client{Transport: rt}

	_, err = mgr.Fetch(context.Background(), Request{ClassRef: "secret/data/app"})
	if err == nil {
		t.Fatalf("expected error")
	}
	errMsg := err.Error()
	if strings.Contains(errMsg, secretLeak) || strings.Contains(errMsg, authToken) || strings.Contains(errMsg, jwt) {
		t.Fatalf("error leaked secret data: %s", errMsg)
	}
}

func TestVaultK8sAuthTimeout(t *testing.T) {
	jwtPath := writeTempJWT(t, "jwt")
	rt := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return nil, context.DeadlineExceeded
	})

	cfg := Config{
		Provider:      "vault_k8s",
		VaultAddr:     "http://vault.test",
		VaultRole:     "role",
		VaultAuthPath: "auth/kubernetes/login",
		VaultJWTPath:  jwtPath,
		VaultTimeout:  5 * time.Millisecond,
	}
	mgr, err := NewVaultK8sManager(cfg)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	mgr.http = &http.Client{Transport: rt, Timeout: 5 * time.Millisecond}

	_, err = mgr.Fetch(context.Background(), Request{ClassRef: "secret/data/app"})
	if err == nil {
		t.Fatalf("expected timeout error")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline error, got: %v", err)
	}
}

func TestVaultK8sLeaseExpired(t *testing.T) {
	jwtPath := writeTempJWT(t, "jwt")
	secretCalled := false
	rt := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/v1/auth/kubernetes/login" {
			payload, _ := json.Marshal(map[string]any{
				"auth": map[string]any{
					"client_token":   "token",
					"lease_duration": 0,
				},
			})
			return newResponse(http.StatusOK, string(payload)), nil
		}
		secretCalled = true
		return newResponse(http.StatusNotFound, ""), nil
	})

	cfg := Config{
		Provider:      "vault_k8s",
		VaultAddr:     "http://vault.test",
		VaultRole:     "role",
		VaultAuthPath: "auth/kubernetes/login",
		VaultJWTPath:  jwtPath,
		VaultTimeout:  2 * time.Second,
		LeaseTTL:      0,
	}
	mgr, err := NewVaultK8sManager(cfg)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	mgr.http = &http.Client{Transport: rt}

	_, err = mgr.Fetch(context.Background(), Request{ClassRef: "secret/data/app"})
	if err == nil {
		t.Fatalf("expected lease expiry error")
	}
	if secretCalled {
		t.Fatalf("secret should not be requested when auth lease expired")
	}
}
