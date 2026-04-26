package auth

import (
	"net/http"
	"testing"
)

func TestSafeReturnTo(t *testing.T) {
	cfg := Config{}
	if got := SafeReturnTo("", cfg); got != "/console" {
		t.Fatalf("SafeReturnTo()=%q, want /console", got)
	}
	if got := SafeReturnTo("/app", cfg); got != "/app" {
		t.Fatalf("SafeReturnTo()=%q, want /app", got)
	}
	if got := SafeReturnTo("https://evil.test/phish", cfg); got != "/console" {
		t.Fatalf("SafeReturnTo()=%q, want /console", got)
	}
	if got := SafeReturnTo("//evil", cfg); got != "/console" {
		t.Fatalf("SafeReturnTo()=%q, want /console", got)
	}
	cfg.AllowedReturnToOrigins = []string{"https://good.test"}
	if got := SafeReturnTo("https://good.test/console", cfg); got != "https://good.test/console" {
		t.Fatalf("SafeReturnTo()=%q, want https://good.test/console", got)
	}
	cfg = Config{PublicBaseURL: "https://public.test"}
	if got := SafeReturnTo("https://public.test/console", cfg); got != "https://public.test/console" {
		t.Fatalf("SafeReturnTo()=%q, want https://public.test/console", got)
	}
}

func TestCanonicalRedirectURL(t *testing.T) {
	cfg := Config{PublicBaseURL: "https://gateway.test:8443"}
	req, _ := http.NewRequest(http.MethodGet, "http://localhost:8080/auth/login?return_to=/console", nil)
	req.Host = "localhost:8080"
	if got := canonicalRedirectURL(req, cfg); got != "https://gateway.test:8443/auth/login?return_to=/console" {
		t.Fatalf("canonicalRedirectURL()=%q, want canonical gateway host", got)
	}

	req2, _ := http.NewRequest(http.MethodGet, "https://gateway.test:8443/auth/login", nil)
	req2.Host = "gateway.test:8443"
	if got := canonicalRedirectURL(req2, cfg); got != "" {
		t.Fatalf("canonicalRedirectURL()=%q, want empty", got)
	}
}
