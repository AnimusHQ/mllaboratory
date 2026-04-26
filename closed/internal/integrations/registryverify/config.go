package registryverify

import (
	"fmt"
	"time"

	"github.com/animus-labs/animus-go/closed/internal/platform/env"
)

type Config struct {
	DefaultMode     string
	DefaultProvider string
	VerifyTimeout   time.Duration
}

func ConfigFromEnv() (Config, error) {
	mode := env.String("ANIMUS_REGISTRY_POLICY_MODE", ModeAllowUnsigned)
	provider := env.String("ANIMUS_REGISTRY_POLICY_PROVIDER", ProviderNoop)
	timeout, err := env.Duration("ANIMUS_REGISTRY_VERIFY_TIMEOUT", 3*time.Second)
	if err != nil {
		return Config{}, err
	}
	cfg := Config{
		DefaultMode:     mode,
		DefaultProvider: provider,
		VerifyTimeout:   timeout,
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) DefaultPolicy() Policy {
	return Policy{Mode: c.DefaultMode, Provider: c.DefaultProvider}.Normalize()
}

func (c Config) Validate() error {
	mode := normalizeMode(c.DefaultMode)
	switch mode {
	case ModeAllowUnsigned, ModeDenyUnsigned, ModeVerifyOnly:
	default:
		return fmt.Errorf("unsupported registry policy mode: %s", c.DefaultMode)
	}
	return nil
}
