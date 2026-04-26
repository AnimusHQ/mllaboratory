package main

import (
	"context"
	"database/sql"
	"net/http"
	"strings"

	"github.com/animus-labs/animus-go/closed/internal/platform/auth"
	"github.com/animus-labs/animus-go/closed/internal/platform/rbac"
)

func experimentsRequiredRole(r *http.Request) string {
	path := strings.TrimSpace(r.URL.Path)
	switch {
	case strings.HasPrefix(path, "/policies"), strings.HasPrefix(path, "/policy-decisions"), strings.HasPrefix(path, "/policy-approvals"):
		return auth.RoleAdmin
	case strings.Contains(path, "/role-bindings"):
		return auth.RoleAdmin
	case strings.Contains(path, "/webhooks"):
		return auth.RoleAdmin
	case strings.Contains(path, "/model-versions/") && (strings.HasSuffix(path, ":approve") || strings.HasSuffix(path, ":deprecate") || strings.HasSuffix(path, ":export")):
		return auth.RoleAdmin
	}
	return rbac.RequiredRoleFromRequest(r)
}

func experimentsProjectResolver(db *sql.DB) auth.ProjectResolver {
	return func(r *http.Request, identity auth.Identity) (string, error) {
		path := strings.TrimSpace(r.URL.Path)
		if path == "" {
			return "", auth.ErrProjectRequired
		}

		if strings.HasPrefix(path, "/policies") || strings.HasPrefix(path, "/policy-decisions") || strings.HasPrefix(path, "/policy-approvals") ||
			strings.HasPrefix(path, "/model-images") || strings.HasPrefix(path, "/ci/") || strings.HasPrefix(path, "/gitlab/") {
			return "", nil
		}

		if projectID := auth.ProjectIDFromRequest(r); projectID != "" {
			return projectID, nil
		}

		if runID := strings.TrimSpace(r.PathValue("run_id")); runID != "" {
			return projectIDForRun(r.Context(), db, runID)
		}
		if experimentID := strings.TrimSpace(r.PathValue("experiment_id")); experimentID != "" {
			return projectIDForExperiment(r.Context(), db, experimentID)
		}
		if runID := strings.TrimSpace(r.URL.Query().Get("run_id")); runID != "" {
			return projectIDForRun(r.Context(), db, runID)
		}
		if sessionID := strings.TrimSpace(r.PathValue("session_id")); sessionID != "" {
			return projectIDForDevEnvSession(r.Context(), db, sessionID)
		}

		return "", auth.ErrProjectRequired
	}
}

func projectIDForRun(ctx context.Context, db *sql.DB, runID string) (string, error) {
	if db == nil {
		return "", auth.ErrProjectRequired
	}
	row := db.QueryRowContext(ctx, `SELECT project_id FROM experiment_runs WHERE run_id = $1`, strings.TrimSpace(runID))
	var projectID sql.NullString
	if err := row.Scan(&projectID); err != nil {
		return "", auth.ErrProjectRequired
	}
	return strings.TrimSpace(projectID.String), nil
}

func projectIDForExperiment(ctx context.Context, db *sql.DB, experimentID string) (string, error) {
	if db == nil {
		return "", auth.ErrProjectRequired
	}
	row := db.QueryRowContext(ctx, `SELECT project_id FROM experiments WHERE experiment_id = $1`, strings.TrimSpace(experimentID))
	var projectID sql.NullString
	if err := row.Scan(&projectID); err != nil {
		return "", auth.ErrProjectRequired
	}
	return strings.TrimSpace(projectID.String), nil
}

func projectIDForDevEnvSession(ctx context.Context, db *sql.DB, sessionID string) (string, error) {
	if db == nil {
		return "", auth.ErrProjectRequired
	}
	row := db.QueryRowContext(ctx, `SELECT project_id FROM dev_env_access_sessions WHERE session_id = $1`, strings.TrimSpace(sessionID))
	var projectID sql.NullString
	if err := row.Scan(&projectID); err != nil {
		return "", auth.ErrProjectRequired
	}
	return strings.TrimSpace(projectID.String), nil
}
