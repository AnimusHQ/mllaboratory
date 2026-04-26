package auth

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

type AuthorizeFunc func(r *http.Request, identity Identity) error

type DenyEvent struct {
	Time       time.Time
	Status     int
	Reason     string
	Error      string
	RequestID  string
	Method     string
	Path       string
	Subject    string
	Email      string
	Roles      []string
	RemoteAddr string
	UserAgent  string
}

type AuditFunc func(ctx context.Context, event DenyEvent) error

type Middleware struct {
	Logger         *slog.Logger
	Authenticator  Authenticator
	Authorize      AuthorizeFunc
	ProjectResolve ProjectResolver
	Audit          AuditFunc
	SkipPrefixes   []string
}

func (m Middleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, prefix := range m.SkipPrefixes {
			if strings.HasPrefix(r.URL.Path, prefix) {
				next.ServeHTTP(w, r)
				return
			}
		}

		identity, err := m.Authenticator.Authenticate(r.Context(), r)
		if err != nil {
			if errors.Is(err, ErrUnauthenticated) {
				m.logDeny(r, http.StatusUnauthorized, "unauthenticated", err)
				m.auditDeny(r, Identity{}, http.StatusUnauthorized, "unauthenticated", err)
				writeJSON(w, http.StatusUnauthorized, map[string]any{
					"error":      "unauthorized",
					"request_id": r.Header.Get("X-Request-Id"),
				})
				return
			}
			m.logDeny(r, http.StatusUnauthorized, "invalid_token", err)
			m.auditDeny(r, Identity{}, http.StatusUnauthorized, "invalid_token", err)
			writeJSON(w, http.StatusUnauthorized, map[string]any{
				"error":      "invalid_token",
				"request_id": r.Header.Get("X-Request-Id"),
			})
			return
		}

		if m.ProjectResolve != nil {
			projectID, err := m.ProjectResolve(r, identity)
			if err != nil {
				status := http.StatusBadRequest
				reason := "project_id_required"
				if errors.Is(err, ErrProjectRequired) {
					status = http.StatusBadRequest
					reason = "project_id_required"
				}
				m.logDeny(r, status, reason, err, "subject", identity.Subject)
				m.auditDeny(r, identity, status, reason, err)
				writeJSON(w, status, map[string]any{
					"error":      reason,
					"request_id": r.Header.Get("X-Request-Id"),
				})
				return
			}
			if strings.TrimSpace(projectID) != "" {
				r = r.WithContext(ContextWithProjectID(r.Context(), projectID))
			}
		}

		if m.Authorize != nil {
			if err := m.Authorize(r, identity); err != nil {
				if errors.Is(err, ErrForbidden) {
					m.logDeny(r, http.StatusForbidden, "forbidden", err, "subject", identity.Subject)
					m.auditDeny(r, identity, http.StatusForbidden, "forbidden", err)
					writeJSON(w, http.StatusForbidden, map[string]any{
						"error":      "forbidden",
						"request_id": r.Header.Get("X-Request-Id"),
					})
					return
				}
				m.logDeny(r, http.StatusForbidden, "forbidden", err, "subject", identity.Subject)
				m.auditDeny(r, identity, http.StatusForbidden, "forbidden", err)
				writeJSON(w, http.StatusForbidden, map[string]any{
					"error":      "forbidden",
					"request_id": r.Header.Get("X-Request-Id"),
				})
				return
			}
		}

		r = r.WithContext(ContextWithIdentity(r.Context(), identity))
		next.ServeHTTP(w, r)
	})
}

func (m Middleware) auditDeny(r *http.Request, identity Identity, status int, reason string, err error) {
	if m.Audit == nil {
		return
	}
	auditErr := m.Audit(r.Context(), DenyEvent{
		Time:       time.Now().UTC(),
		Status:     status,
		Reason:     reason,
		Error:      err.Error(),
		RequestID:  r.Header.Get("X-Request-Id"),
		Method:     r.Method,
		Path:       r.URL.Path,
		Subject:    identity.Subject,
		Email:      identity.Email,
		Roles:      identity.Roles,
		RemoteAddr: r.RemoteAddr,
		UserAgent:  r.UserAgent(),
	})
	if auditErr == nil || m.Logger == nil {
		return
	}
	m.Logger.Warn("audit deny failed", "request_id", r.Header.Get("X-Request-Id"), "error", auditErr.Error())
}

func (m Middleware) logDeny(r *http.Request, status int, reason string, err error, extra ...any) {
	if m.Logger == nil {
		return
	}
	fields := []any{
		"reason", reason,
		"status", status,
		"request_id", r.Header.Get("X-Request-Id"),
		"method", r.Method,
		"path", r.URL.Path,
		"error", err.Error(),
	}
	fields = append(fields, extra...)
	if status >= 500 {
		m.Logger.Error("auth deny", fields...)
		return
	}
	m.Logger.Warn("auth deny", fields...)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(true)
	_ = enc.Encode(body)
}

func MethodRoleAuthorizer() AuthorizeFunc {
	return func(r *http.Request, identity Identity) error {
		required := RequiredRoleForRequest(r)
		if HasAtLeast(identity.Roles, required) {
			return nil
		}
		return ErrForbidden
	}
}

func WithTimeout(timeout time.Duration, check func(context.Context) error) func(context.Context) error {
	return func(ctx context.Context) error {
		checkCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		return check(checkCtx)
	}
}
