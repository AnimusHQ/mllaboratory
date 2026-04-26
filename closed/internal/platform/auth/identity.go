package auth

import (
	"context"
)

type Identity struct {
	Subject string
	Email   string
	Roles   []string
}

type ctxKeyIdentity struct{}

func ContextWithIdentity(ctx context.Context, identity Identity) context.Context {
	return context.WithValue(ctx, ctxKeyIdentity{}, identity)
}

func IdentityFromContext(ctx context.Context) (Identity, bool) {
	v, ok := ctx.Value(ctxKeyIdentity{}).(Identity)
	return v, ok
}
