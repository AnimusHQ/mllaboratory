package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"
)

type GatewayHeadersAuthenticator struct {
	Secret  string
	MaxSkew time.Duration
}

func NewGatewayHeadersAuthenticator(secret string) (*GatewayHeadersAuthenticator, error) {
	if strings.TrimSpace(secret) == "" {
		return nil, errors.New("ANIMUS_INTERNAL_AUTH_SECRET is required")
	}
	return &GatewayHeadersAuthenticator{
		Secret:  secret,
		MaxSkew: 5 * time.Minute,
	}, nil
}

func (a *GatewayHeadersAuthenticator) Authenticate(ctx context.Context, r *http.Request) (Identity, error) {
	subject := strings.TrimSpace(r.Header.Get(HeaderSubject))
	if subject == "" {
		return Identity{}, ErrUnauthenticated
	}

	email := strings.TrimSpace(r.Header.Get(HeaderEmail))
	rolesRaw := strings.TrimSpace(r.Header.Get(HeaderRoles))

	ts := strings.TrimSpace(r.Header.Get(HeaderInternalAuthTimestamp))
	sig := strings.TrimSpace(r.Header.Get(HeaderInternalAuthSignature))
	if ts == "" || sig == "" {
		return Identity{}, ErrUnauthenticated
	}

	if err := VerifyInternalAuthTimestamp(ts, time.Now().UTC(), a.MaxSkew); err != nil {
		return Identity{}, err
	}
	if err := VerifyInternalAuthSignature(
		a.Secret,
		ts,
		r.Method,
		r.URL.Path,
		r.Header.Get("X-Request-Id"),
		subject,
		email,
		rolesRaw,
		sig,
	); err != nil {
		return Identity{}, err
	}

	return Identity{
		Subject: subject,
		Email:   email,
		Roles:   parseCSV(rolesRaw),
	}, nil
}
