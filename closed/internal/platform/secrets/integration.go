package secrets

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// RuntimeSecret captures DP-only runtime secret access intent.
type RuntimeSecret struct {
	ProjectID string
	RunID     string
	Subject   string
	ClassRef  string
}

type IntegrationScope string

const (
	IntegrationScopeWebhook     IntegrationScope = "webhook"
	IntegrationScopeAuditExport IntegrationScope = "audit_export"
)

// IntegrationSecret captures CP integration secret access intent.
type IntegrationSecret struct {
	ProjectID string
	Scope     IntegrationScope
	ClassRef  string
	Key       string
}

var (
	ErrIntegrationSecretUnavailable = errors.New("integration secret unavailable")
	ErrIntegrationSecretKeyMissing  = errors.New("integration secret key missing")
	errIntegrationSecretScope       = errors.New("integration secret scope required")
)

func FetchIntegrationSecret(ctx context.Context, manager Manager, req IntegrationSecret) (string, error) {
	classRef := strings.TrimSpace(req.ClassRef)
	if classRef == "" {
		return "", nil
	}
	if manager == nil {
		return "", ErrIntegrationSecretUnavailable
	}
	scope := strings.TrimSpace(string(req.Scope))
	if scope == "" {
		return "", errIntegrationSecretScope
	}
	lease, err := manager.Fetch(ctx, Request{
		ProjectID: strings.TrimSpace(req.ProjectID),
		Subject:   scope,
		ClassRef:  classRef,
	})
	if err != nil {
		return "", err
	}
	key := strings.TrimSpace(req.Key)
	if key == "" {
		return "", ErrIntegrationSecretKeyMissing
	}
	value := strings.TrimSpace(lease.Env[key])
	if value == "" {
		return "", fmt.Errorf("%w: %s", ErrIntegrationSecretKeyMissing, key)
	}
	return value, nil
}
