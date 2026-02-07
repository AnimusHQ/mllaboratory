package auth

import (
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
