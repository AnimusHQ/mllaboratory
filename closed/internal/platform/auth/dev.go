package auth

import (
	"context"
	"net/http"
)

type Authenticator interface {
	Authenticate(ctx context.Context, r *http.Request) (Identity, error)
}

type DevAuthenticator struct {
	identity Identity
}

func NewDevAuthenticator(cfg Config) *DevAuthenticator {
	return &DevAuthenticator{
		identity: Identity{
			Subject: cfg.DevSubject,
			Email:   cfg.DevEmail,
			Roles:   cfg.DevRoles,
		},
	}
}

func (a *DevAuthenticator) Authenticate(ctx context.Context, r *http.Request) (Identity, error) {
	return a.identity, nil
}
