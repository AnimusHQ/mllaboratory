package auth

import (
	"errors"
	"net/http"
	"strings"
)

var ErrForbidden = errors.New("forbidden")

const (
	RoleViewer = "viewer"
	RoleEditor = "editor"
	RoleAdmin  = "admin"
)

var roleLevels = map[string]int{
	RoleViewer: 1,
	RoleEditor: 2,
	RoleAdmin:  3,
}

func HasAtLeast(roles []string, required string) bool {
	requiredLevel := roleLevels[strings.ToLower(required)]
	if requiredLevel == 0 {
		return false
	}
	maxLevel := 0
	for _, role := range roles {
		level := roleLevels[strings.ToLower(strings.TrimSpace(role))]
		if level > maxLevel {
			maxLevel = level
		}
	}
	return maxLevel >= requiredLevel
}

func RequiredRoleForRequest(r *http.Request) string {
	switch r.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return RoleViewer
	default:
		return RoleEditor
	}
}
