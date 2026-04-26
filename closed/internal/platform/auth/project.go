package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"
)

type ctxKeyProjectID struct{}

// ErrProjectRequired indicates a missing project scope for a request.
var ErrProjectRequired = errors.New("project_id_required")

// ProjectResolver extracts a project identifier for the request.
type ProjectResolver func(r *http.Request, identity Identity) (string, error)

func ContextWithProjectID(ctx context.Context, projectID string) context.Context {
	return context.WithValue(ctx, ctxKeyProjectID{}, strings.TrimSpace(projectID))
}

func ProjectIDFromContext(ctx context.Context) (string, bool) {
	value, ok := ctx.Value(ctxKeyProjectID{}).(string)
	return strings.TrimSpace(value), ok
}

// ProjectIDFromRequest checks path parameters and headers for project id.
func ProjectIDFromRequest(r *http.Request) string {
	if r == nil {
		return ""
	}
	if v := strings.TrimSpace(r.PathValue("project_id")); v != "" {
		return v
	}
	if v := strings.TrimSpace(r.Header.Get("X-Project-Id")); v != "" {
		return v
	}
	if v := strings.TrimSpace(r.Header.Get("X-Project-ID")); v != "" {
		return v
	}
	if v := strings.TrimSpace(r.URL.Query().Get("project_id")); v != "" {
		return v
	}
	return ""
}

// RequireProjectIDResolver enforces project scoping for requests except listed prefixes.
func RequireProjectIDResolver(skipPrefixes []string) ProjectResolver {
	return func(r *http.Request, identity Identity) (string, error) {
		for _, prefix := range skipPrefixes {
			if strings.HasPrefix(r.URL.Path, prefix) {
				return "", nil
			}
		}
		projectID := ProjectIDFromRequest(r)
		if projectID == "" {
			return "", ErrProjectRequired
		}
		return projectID, nil
	}
}
