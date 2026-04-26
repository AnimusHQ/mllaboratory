package secrets

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Request struct {
	ProjectID string
	RunID     string
	Subject   string
	ClassRef  string
}

type Lease struct {
	LeaseID   string
	Env       map[string]string
	ExpiresAt time.Time
}

type Manager interface {
	Fetch(ctx context.Context, req Request) (Lease, error)
}

type NoopManager struct{}

func (NoopManager) Fetch(ctx context.Context, req Request) (Lease, error) {
	return Lease{Env: map[string]string{}, ExpiresAt: time.Now().UTC().Add(10 * time.Minute)}, nil
}

type StaticManager struct {
	values  map[string]map[string]string
	leaseTT time.Duration
}

func NewManager(cfg Config) (Manager, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	provider := strings.ToLower(strings.TrimSpace(cfg.Provider))
	switch provider {
	case "static":
		return &StaticManager{values: cfg.StaticMapping, leaseTT: cfg.LeaseTTL}, nil
	case "vault", "vault_k8s":
		return NewVaultK8sManager(cfg)
	default:
		return NoopManager{}, nil
	}
}

func (m *StaticManager) Fetch(ctx context.Context, req Request) (Lease, error) {
	if m == nil {
		return Lease{}, errors.New("secrets manager not initialized")
	}
	classRef := strings.TrimSpace(req.ClassRef)
	if classRef == "" {
		return Lease{Env: map[string]string{}, ExpiresAt: time.Now().UTC().Add(m.leaseTTL())}, nil
	}
	vals, ok := m.values[classRef]
	if !ok {
		return Lease{Env: map[string]string{}, ExpiresAt: time.Now().UTC().Add(m.leaseTTL())}, nil
	}
	env := make(map[string]string, len(vals))
	for k, v := range vals {
		key := strings.TrimSpace(k)
		if key == "" {
			continue
		}
		env[key] = v
	}
	return Lease{
		LeaseID:   uuid.NewString(),
		Env:       env,
		ExpiresAt: time.Now().UTC().Add(m.leaseTTL()),
	}, nil
}

func (m *StaticManager) leaseTTL() time.Duration {
	if m.leaseTT <= 0 {
		return 10 * time.Minute
	}
	return m.leaseTT
}
