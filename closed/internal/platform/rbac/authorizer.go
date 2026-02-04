package rbac

import (
	"context"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/animus-labs/animus-go/closed/internal/domain"
	"github.com/animus-labs/animus-go/closed/internal/platform/auth"
	"github.com/animus-labs/animus-go/closed/internal/repo"
)

type RequiredRoleFunc func(r *http.Request) string

type Authorizer struct {
	Store           BindingStore
	Audit           repo.AuditEventAppender
	AllowDirect     bool
	Now             func() time.Time
	RequiredRoleFor RequiredRoleFunc
}

func (a Authorizer) Authorize(r *http.Request, identity auth.Identity) error {
	if r == nil {
		return auth.ErrForbidden
	}
	required := RequiredRoleFromRequest(r)
	if a.RequiredRoleFor != nil {
		required = a.RequiredRoleFor(r)
	}
	if strings.TrimSpace(required) == "" {
		return nil
	}

	if IsRunToken(identity) {
		if strings.EqualFold(required, auth.RoleAdmin) {
			projectID := projectFromRequest(r)
			auditAccessDenied(r.Context(), a.Audit, r, identity, projectID, "", required, a.Now)
			return auth.ErrForbidden
		}
		return nil
	}

	projectID := projectFromRequest(r)
	role, _, err := ResolveRole(r.Context(), a.Store, projectID, identity, a.AllowDirect)
	if err != nil {
		return auth.ErrForbidden
	}
	if HasAtLeast(role, required) {
		return nil
	}
	auditAccessDenied(r.Context(), a.Audit, r, identity, projectID, role, required, a.Now)
	return auth.ErrForbidden
}

func RequiredRoleFromRequest(r *http.Request) string {
	switch r.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return auth.RoleViewer
	default:
		return auth.RoleEditor
	}
}

func projectFromRequest(r *http.Request) string {
	if r == nil {
		return ""
	}
	if projectID, ok := auth.ProjectIDFromContext(r.Context()); ok && strings.TrimSpace(projectID) != "" {
		return projectID
	}
	return auth.ProjectIDFromRequest(r)
}

func auditAccessDenied(ctx context.Context, audit repo.AuditEventAppender, r *http.Request, identity auth.Identity, projectID, role, required string, now func() time.Time) {
	if audit == nil || r == nil {
		return
	}
	when := time.Now().UTC()
	if now != nil {
		when = now().UTC()
	}

	var ip net.IP
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		ip = net.ParseIP(host)
	}

	_, _ = audit.Append(ctx, domain.AuditEvent{
		OccurredAt:   when,
		Actor:        strings.TrimSpace(identity.Subject),
		Action:       "access.denied",
		ResourceType: "http",
		ResourceID:   r.Method + " " + r.URL.Path,
		RequestID:    r.Header.Get("X-Request-Id"),
		IP:           ip,
		UserAgent:    r.UserAgent(),
		Payload: domain.Metadata{
			"project_id":    strings.TrimSpace(projectID),
			"subject":       strings.TrimSpace(identity.Subject),
			"email":         strings.TrimSpace(identity.Email),
			"roles":         identity.Roles,
			"required_role": strings.TrimSpace(required),
			"effective_role": strings.TrimSpace(role),
			"path":           r.URL.Path,
			"method":         r.Method,
		},
	})
}
