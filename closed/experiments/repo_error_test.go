package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/animus-labs/animus-go/closed/internal/repo"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestWriteRepoError(t *testing.T) {
	api := &experimentsAPI{
		logger: newTestLogger(t),
	}

	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{
			name:       "invalid transition",
			err:        repo.ErrInvalidTransition,
			wantStatus: http.StatusConflict,
			wantCode:   "invalid_transition",
		},
		{
			name:       "not found",
			err:        repo.ErrNotFound,
			wantStatus: http.StatusNotFound,
			wantCode:   "not_found",
		},
		{
			name:       "sql no rows",
			err:        sql.ErrNoRows,
			wantStatus: http.StatusNotFound,
			wantCode:   "not_found",
		},
		{
			name:       "foreign key violation",
			err:        &pgconn.PgError{Code: "23503"},
			wantStatus: http.StatusNotFound,
			wantCode:   "not_found",
		},
		{
			name:       "foreign key violation wrapped",
			err:        fmt.Errorf("wrap: %w", &pgconn.PgError{Code: "23503"}),
			wantStatus: http.StatusNotFound,
			wantCode:   "not_found",
		},
		{
			name:       "unique violation",
			err:        &pgconn.PgError{Code: "23505"},
			wantStatus: http.StatusConflict,
			wantCode:   "conflict",
		},
		{
			name:       "unique violation wrapped",
			err:        fmt.Errorf("wrap: %w", &pgconn.PgError{Code: "23505"}),
			wantStatus: http.StatusConflict,
			wantCode:   "conflict",
		},
		{
			name:       "internal error",
			err:        errors.New("boom"),
			wantStatus: http.StatusInternalServerError,
			wantCode:   "internal_error",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/projects/p1/runs/r1:plan", nil)
			req.Header.Set("X-Request-Id", "req-1")
			rec := httptest.NewRecorder()

			api.writeRepoError(rec, req, tc.err)

			if rec.Code != tc.wantStatus {
				t.Fatalf("expected status %d, got %d", tc.wantStatus, rec.Code)
			}
			body := parseErrorBody(t, rec.Body)
			if body["error"] != tc.wantCode {
				t.Fatalf("expected error %q, got %v", tc.wantCode, body["error"])
			}
			if body["request_id"] != "req-1" {
				t.Fatalf("expected request_id req-1, got %v", body["request_id"])
			}
		})
	}
}

func parseErrorBody(t *testing.T, r io.Reader) map[string]any {
	t.Helper()
	var body map[string]any
	dec := json.NewDecoder(r)
	if err := dec.Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	return body
}

func newTestLogger(t *testing.T) *slog.Logger {
	t.Helper()
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelInfo}))
}
