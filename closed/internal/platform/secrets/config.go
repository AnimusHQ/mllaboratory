package secrets

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/animus-labs/animus-go/closed/internal/platform/env"
)

type Config struct {
	Provider      string
	LeaseTTL      time.Duration
	StaticMapping map[string]map[string]string
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
	cfg := Config{
		Provider:      provider,
		LeaseTTL:      time.Duration(leaseSeconds) * time.Second,
		StaticMapping: mapping,
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
	default:
		return fmt.Errorf("unsupported secrets provider: %s", provider)
	}
}
