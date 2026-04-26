package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/animus-labs/animus-go/closed/internal/domain"
	"github.com/animus-labs/animus-go/closed/internal/platform/auditlog"
	"github.com/animus-labs/animus-go/closed/internal/platform/auth"
	"github.com/animus-labs/animus-go/closed/internal/platform/rbac"
	"github.com/animus-labs/animus-go/closed/internal/repo"
)

func (api *experimentsAPI) handleDevEnvProxy(w http.ResponseWriter, r *http.Request) {
	identity, ok := auth.IdentityFromContext(r.Context())
	if !ok || strings.TrimSpace(identity.Subject) == "" {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	if rbac.IsRunToken(identity) {
		api.writeError(w, r, http.StatusForbidden, "forbidden")
		return
	}

	sessionID := strings.TrimSpace(r.PathValue("session_id"))
	if sessionID == "" {
		api.writeError(w, r, http.StatusBadRequest, "session_id_required")
		return
	}
	projectID, ok := auth.ProjectIDFromContext(r.Context())
	if !ok || strings.TrimSpace(projectID) == "" {
		api.writeError(w, r, http.StatusBadRequest, "project_id_required")
		return
	}

	sessionStore := api.devEnvSessionStore()
	if sessionStore == nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	session, err := sessionStore.GetBySessionID(r.Context(), projectID, sessionID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			api.writeError(w, r, http.StatusNotFound, "not_found")
			return
		}
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	now := time.Now().UTC()
	if session.ExpiresAt.Before(now) || session.ExpiresAt.Equal(now) {
		api.writeError(w, r, http.StatusForbidden, "session_expired")
		return
	}

	store := api.devEnvStore()
	if store == nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	record, err := store.Get(r.Context(), projectID, session.DevEnvID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			api.writeError(w, r, http.StatusNotFound, "not_found")
			return
		}
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	if record.Environment.State != domain.DevEnvStateActive {
		api.writeError(w, r, http.StatusConflict, "devenv_not_active")
		return
	}
	if !record.Environment.ExpiresAt.IsZero() && (record.Environment.ExpiresAt.Before(now) || record.Environment.ExpiresAt.Equal(now)) {
		api.writeError(w, r, http.StatusConflict, "devenv_expired")
		return
	}
	namespace := strings.TrimSpace(record.Environment.DPNamespace)
	if namespace == "" {
		api.writeError(w, r, http.StatusConflict, "devenv_not_ready")
		return
	}

	path := strings.TrimSpace(r.PathValue("path"))
	if path == "" {
		path = "/"
	} else if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	serviceName := devEnvServiceName(record.Environment.ID)
	targetHost := devEnvServiceHost(serviceName, namespace, api.devEnvServiceDomain)
	port := api.devEnvCodeServerPort
	if port <= 0 {
		port = 8080
	}
	targetURL := &url.URL{
		Scheme:   "http",
		Host:     fmt.Sprintf("%s:%d", targetHost, port),
		Path:     path,
		RawQuery: r.URL.RawQuery,
	}

	auditInterval := api.devEnvAccessAuditInterval
	if auditInterval <= 0 {
		auditInterval = time.Minute
	}
	if updated, err := store.UpdateLastAccess(r.Context(), projectID, record.Environment.ID, now, auditInterval); err == nil && updated {
		_ = api.appendDevEnvAudit(r.Context(), auditlog.Event{
			OccurredAt:   now,
			Actor:        identity.Subject,
			Action:       auditDevEnvSessionAccess,
			ResourceType: "dev_environment",
			ResourceID:   record.Environment.ID,
			RequestID:    r.Header.Get("X-Request-Id"),
			IP:           requestIP(r.RemoteAddr),
			UserAgent:    r.UserAgent(),
			Payload: map[string]any{
				"service":    "experiments",
				"project_id": projectID,
				"session_id": sessionID,
			},
		})
	}

	proxy := api.devEnvReverseProxy(targetURL)
	proxy.ServeHTTP(w, r)
}

func (api *experimentsAPI) devEnvReverseProxy(target *url.URL) *httputil.ReverseProxy {
	transport := api.devEnvProxyTransport
	if transport == nil {
		transport = http.DefaultTransport
	}
	return &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			sanitizeDevEnvProxyHeaders(req.Header)
			req.URL.Scheme = target.Scheme
			req.URL.Host = target.Host
			req.URL.Path = target.Path
			req.URL.RawQuery = target.RawQuery
			req.Host = target.Host
		},
		Transport: transport,
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			api.writeError(w, r, http.StatusBadGateway, "devenv_proxy_failed")
		},
	}
}

func sanitizeDevEnvProxyHeaders(headers http.Header) {
	for name := range headers {
		lower := strings.ToLower(name)
		if strings.HasPrefix(lower, "x-animus-") {
			headers.Del(name)
			continue
		}
		if lower == "authorization" || lower == "cookie" || lower == "x-project-id" {
			headers.Del(name)
		}
	}
}

func devEnvServiceName(devEnvID string) string {
	base := "animus-devenv-" + sanitizeDevEnvName(devEnvID)
	if len(base) <= 63 {
		return base
	}
	return shortenK8sName(base, devEnvID)
}

func sanitizeDevEnvName(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "devenv"
	}
	out := make([]rune, 0, len(value))
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			out = append(out, r)
		case r >= '0' && r <= '9':
			out = append(out, r)
		case r == '-':
			out = append(out, r)
		default:
			out = append(out, '-')
		}
	}
	clean := strings.Trim(string(out), "-")
	if clean == "" {
		return "devenv"
	}
	return clean
}

func shortenK8sName(base, seed string) string {
	sum := sha256Sum(seed)
	suffix := sum[:12]
	trim := 63 - len(suffix) - 1
	if trim < 1 {
		trim = 1
	}
	return base[:trim] + "-" + suffix
}

func sha256Sum(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func devEnvServiceHost(serviceName, namespace, domain string) string {
	host := strings.TrimSpace(serviceName) + "." + strings.TrimSpace(namespace)
	domain = strings.Trim(strings.TrimSpace(domain), ".")
	if domain == "" {
		return host
	}
	return host + "." + domain
}
