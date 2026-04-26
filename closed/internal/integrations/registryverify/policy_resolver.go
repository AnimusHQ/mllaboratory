package registryverify

import (
	"context"
	"errors"
	"strings"
)

var ErrPolicyNotFound = errors.New("registry policy not found")

type PolicyRecord struct {
	ProjectID string
	Mode      string
	Provider  string
}

type PolicyStore interface {
	Get(ctx context.Context, projectID string) (PolicyRecord, error)
}

type PolicyResolver struct {
	Default Policy
	Store   PolicyStore
}

func (r PolicyResolver) Resolve(ctx context.Context, projectID string) (Policy, error) {
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return r.Default.Normalize(), nil
	}
	if r.Store == nil {
		return r.Default.Normalize(), nil
	}
	rec, err := r.Store.Get(ctx, projectID)
	if err != nil {
		if errors.Is(err, ErrPolicyNotFound) {
			return r.Default.Normalize(), nil
		}
		return Policy{}, err
	}
	policy := Policy{Mode: rec.Mode, Provider: rec.Provider}.Normalize()
	return policy, nil
}
