package httpserver

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestWrap_SetsRequestIDHeader_WhenMissing(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	h := Wrap(logger, "testsvc", mux)

	req := httptest.NewRequest(http.MethodGet, "http://example.test/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if got := rec.Header().Get("X-Request-Id"); got == "" {
		t.Fatalf("expected X-Request-Id response header")
	}
}

func TestWrap_PreservesRequestIDHeader_WhenProvided(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	h := Wrap(logger, "testsvc", mux)

	req := httptest.NewRequest(http.MethodGet, "http://example.test/", nil)
	req.Header.Set("X-Request-Id", "rid-123")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if got := rec.Header().Get("X-Request-Id"); got != "rid-123" {
		t.Fatalf("X-Request-Id=%q, want rid-123", got)
	}
}

func TestWrap_RecoversPanic(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { panic("boom") })
	h := Wrap(logger, "testsvc", mux)

	req := httptest.NewRequest(http.MethodGet, "http://example.test/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d, want 500", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type=%q, want application/json", ct)
	}
}

func TestReadyzWithChecks_OK(t *testing.T) {
	handler := ReadyzWithChecks("testsvc", ReadinessCheck{
		Name: "always-ok",
		Check: func(ctx context.Context) error {
			return nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "http://example.test/readyz", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "\"status\":\"ready\"") {
		t.Fatalf("expected ready status in response: %s", rec.Body.String())
	}
}

func TestReadyzWithChecks_Fail(t *testing.T) {
	handler := ReadyzWithChecks("testsvc", ReadinessCheck{
		Name: "always-fail",
		Check: func(ctx context.Context) error {
			return context.Canceled
		},
	})

	req := httptest.NewRequest(http.MethodGet, "http://example.test/readyz", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d, want 503", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "\"status\":\"not_ready\"") {
		t.Fatalf("expected not_ready status in response: %s", rec.Body.String())
	}
}
