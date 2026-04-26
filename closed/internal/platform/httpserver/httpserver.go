package httpserver

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/animus-labs/animus-go/closed/internal/platform/requestid"
)

type Config struct {
	Service         string
	Addr            string
	ShutdownTimeout time.Duration
}

func Wrap(logger *slog.Logger, service string, next http.Handler) http.Handler {
	ensureHTTPMetricsRegistered()
	return recoverMiddleware(logger, requestLogMiddleware(logger, service, requestIDMiddleware(service, next)))
}

func Run(ctx context.Context, logger *slog.Logger, cfg Config, handler http.Handler) error {
	if cfg.Service == "" {
		return errors.New("service is required")
	}
	if cfg.Addr == "" {
		return errors.New("addr is required")
	}
	if cfg.ShutdownTimeout <= 0 {
		cfg.ShutdownTimeout = 10 * time.Second
	}

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Minute,
		WriteTimeout:      0,
		IdleTimeout:       60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("http server listening", "service", cfg.Service, "addr", cfg.Addr)
		errCh <- srv.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shutdown: %w", err)
		}
		return nil
	case err := <-errCh:
		return err
	}
}

func Healthz(service string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"service": service,
			"status":  "ok",
		})
	}
}

func Readyz(service string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"service": service,
			"status":  "ready",
		})
	}
}

type ReadinessCheck struct {
	Name  string
	Check func(context.Context) error
}

func ReadyzWithChecks(service string, checks ...ReadinessCheck) http.HandlerFunc {
	type checkResult struct {
		Name       string `json:"name"`
		Status     string `json:"status"`
		DurationMs int64  `json:"duration_ms"`
		Error      string `json:"error,omitempty"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		results := make([]checkResult, 0, len(checks))
		overallOK := true

		for _, check := range checks {
			start := time.Now()
			err := check.Check(r.Context())
			status := "ok"
			var errMsg string
			if err != nil {
				overallOK = false
				status = "fail"
				errMsg = err.Error()
			}
			results = append(results, checkResult{
				Name:       check.Name,
				Status:     status,
				DurationMs: time.Since(start).Milliseconds(),
				Error:      errMsg,
			})
		}

		if overallOK {
			writeJSON(w, http.StatusOK, map[string]any{
				"service": service,
				"status":  "ready",
				"checks":  results,
			})
			return
		}
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"service": service,
			"status":  "not_ready",
			"checks":  results,
		})
	}
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(true)
	_ = enc.Encode(body)
}

type ctxKeyRequestID struct{}

func RequestIDFromContext(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(ctxKeyRequestID{}).(string)
	return v, ok
}

func requestIDMiddleware(service string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimSpace(r.Header.Get("X-Request-Id"))
		if id == "" {
			newID, err := requestid.New()
			if err != nil {
				newID = fmt.Sprintf("%s-%d", service, time.Now().UnixNano())
			}
			id = newID
		}

		r.Header.Set("X-Request-Id", id)
		w.Header().Set("X-Request-Id", id)
		r = r.WithContext(context.WithValue(r.Context(), ctxKeyRequestID{}, id))
		next.ServeHTTP(w, r)
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(statusCode int) {
	w.status = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *statusWriter) Flush() {
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (w *statusWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("hijacker not supported")
	}
	return hijacker.Hijack()
}

func (w *statusWriter) Push(target string, opts *http.PushOptions) error {
	pusher, ok := w.ResponseWriter.(http.Pusher)
	if !ok {
		return http.ErrNotSupported
	}
	return pusher.Push(target, opts)
}

func (w *statusWriter) ReadFrom(r io.Reader) (int64, error) {
	if rf, ok := w.ResponseWriter.(io.ReaderFrom); ok {
		return rf.ReadFrom(r)
	}
	return io.Copy(w.ResponseWriter, r)
}

func requestLogMiddleware(logger *slog.Logger, service string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(sw, r)

		requestID, _ := RequestIDFromContext(r.Context())
		recordHTTPMetrics(service, r.Method, sw.status, time.Since(start))
		attrs := []any{
			"request_id", requestID,
			"method", r.Method,
			"path", r.URL.Path,
			"status", sw.status,
			"duration_ms", time.Since(start).Milliseconds(),
		}
		if sw.status >= 500 {
			logger.Error("http request", attrs...)
			return
		}
		logger.Info("http request", attrs...)
	})
}

func recoverMiddleware(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				requestID, _ := RequestIDFromContext(r.Context())
				logger.Error("panic recovered", "request_id", requestID, "panic", v)
				writeJSON(w, http.StatusInternalServerError, map[string]any{
					"error":      "internal_server_error",
					"request_id": requestID,
				})
			}
		}()
		next.ServeHTTP(w, r)
	})
}
