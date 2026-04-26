package secrets

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/animus-labs/animus-go/closed/internal/platform/env"
)

type Config struct {
	Provider       string
	LeaseTTL       time.Duration
	StaticMapping  map[string]map[string]string
	VaultAddr      string
	VaultRole      string
	VaultAuthPath  string
	VaultJWTPath   string
	VaultNamespace string
	VaultTimeout   time.Duration
}

func ConfigFromEnv() (Config, error) {
	provider := strings.ToLower(strings.TrimSpace(env.String("SECRETS_PROVIDER", "noop")))
	leaseSeconds, err := env.Int("SECRETS_LEASE_TTL_SECONDS", 600)
	if err != nil {
		return Config{}, err
	}
	mappingRaw := strings.TrimSpace(env.String("SECRETS_STATIC_JSON", ""))
	mapping := map[string]map[string]string{}
	if mappingRaw != "" {
		if err := json.Unmarshal([]byte(mappingRaw), &mapping); err != nil {
			return Config{}, fmt.Errorf("invalid SECRETS_STATIC_JSON: %w", err)
		}
	}
	vaultTimeout, err := env.Duration("SECRETS_VAULT_TIMEOUT", 5*time.Second)
	if err != nil {
		return Config{}, err
	}
	cfg := Config{
		Provider:       provider,
		LeaseTTL:       time.Duration(leaseSeconds) * time.Second,
		StaticMapping:  mapping,
		VaultAddr:      strings.TrimSpace(env.String("SECRETS_VAULT_ADDR", "")),
		VaultRole:      strings.TrimSpace(env.String("SECRETS_VAULT_ROLE", "")),
		VaultAuthPath:  strings.TrimSpace(env.String("SECRETS_VAULT_AUTH_PATH", "auth/kubernetes/login")),
		VaultJWTPath:   strings.TrimSpace(env.String("SECRETS_VAULT_JWT_PATH", "")),
		VaultNamespace: strings.TrimSpace(env.String("SECRETS_VAULT_NAMESPACE", "")),
		VaultTimeout:   vaultTimeout,
	}
	return cfg, cfg.Validate()
}

func (c Config) Validate() error {
	provider := strings.ToLower(strings.TrimSpace(c.Provider))
	if provider == "" {
		provider = "noop"
	}
	switch provider {
	case "noop", "static":
		return nil
	case "vault", "vault_k8s":
		if strings.TrimSpace(c.VaultAddr) == "" {
			return fmt.Errorf("vault addr is required")
		}
		if strings.TrimSpace(c.VaultRole) == "" {
			return fmt.Errorf("vault role is required")
		}
		return nil
	default:
		return fmt.Errorf("unsupported secrets provider: %s", provider)
	}
}
