package auth

import (
	"context"
	"net/http"
	"strings"
	"time"
)

type RunTokenAuthenticator struct {
	Secret string
	Next   Authenticator
	Now    func() time.Time
}

func (a RunTokenAuthenticator) Authenticate(ctx context.Context, r *http.Request) (Identity, error) {
	authz := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(strings.ToLower(authz), "bearer ") {
		token := strings.TrimSpace(authz[len("bearer "):])
		if strings.HasPrefix(token, runTokenPrefix+".") {
			now := time.Now().UTC()
			if a.Now != nil {
				now = a.Now().UTC()
			}
			claims, err := VerifyRunToken(a.Secret, token, now)
			if err != nil {
				return Identity{}, ErrUnauthenticated
			}
			return Identity{
				Subject: RunTokenSubject(claims),
				Roles:   []string{RoleEditor},
			}, nil
		}
	}

	if a.Next == nil {
		return Identity{}, ErrUnauthenticated
	}
	return a.Next.Authenticate(ctx, r)
}
