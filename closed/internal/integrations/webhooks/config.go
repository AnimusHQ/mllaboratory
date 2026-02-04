package webhooks

import (
	"fmt"
	"strings"
	"time"

	"github.com/animus-labs/animus-go/closed/internal/platform/env"
)

type Config struct {
	EnabledFlag       bool
	BatchSize         int
	PollInterval      time.Duration
	RetryBaseDelay    time.Duration
	RetryMaxDelay     time.Duration
	InflightTimeout   time.Duration
	MaxAttempts       int
	WorkerConcurrency int
	HTTPTimeout       time.Duration
	SigningSecretKey  string
}

func ConfigFromEnv() (Config, error) {
	enabled, err := env.Bool("ANIMUS_WEBHOOKS_ENABLED", true)
	if err != nil {
		return Config{}, err
	}
	batchSize, err := env.Int("ANIMUS_WEBHOOK_BATCH_SIZE", 50)
	if err != nil {
		return Config{}, err
	}
	pollInterval, err := env.Duration("ANIMUS_WEBHOOK_POLL_INTERVAL", 5*time.Second)
	if err != nil {
		return Config{}, err
	}
	retryBase, err := env.Duration("ANIMUS_WEBHOOK_RETRY_BASE", 5*time.Second)
	if err != nil {
		return Config{}, err
	}
	retryMax, err := env.Duration("ANIMUS_WEBHOOK_RETRY_MAX", 5*time.Minute)
	if err != nil {
		return Config{}, err
	}
	inflightTimeout, err := env.Duration("ANIMUS_WEBHOOK_INFLIGHT_TIMEOUT", 2*time.Minute)
	if err != nil {
		return Config{}, err
	}
	maxAttempts, err := env.Int("ANIMUS_WEBHOOK_MAX_ATTEMPTS", 10)
	if err != nil {
		return Config{}, err
	}
	concurrency, err := env.Int("ANIMUS_WEBHOOK_WORKER_CONCURRENCY", 4)
	if err != nil {
		return Config{}, err
	}
	httpTimeout, err := env.Duration("ANIMUS_WEBHOOK_HTTP_TIMEOUT", 10*time.Second)
	if err != nil {
		return Config{}, err
	}
	secretKey := strings.TrimSpace(env.String("ANIMUS_WEBHOOK_SIGNING_SECRET_KEY", "WEBHOOK_SIGNING_SECRET"))

	cfg := Config{
		EnabledFlag:       enabled,
		BatchSize:         batchSize,
		PollInterval:      pollInterval,
		RetryBaseDelay:    retryBase,
		RetryMaxDelay:     retryMax,
		InflightTimeout:   inflightTimeout,
		MaxAttempts:       maxAttempts,
		WorkerConcurrency: concurrency,
		HTTPTimeout:       httpTimeout,
		SigningSecretKey:  secretKey,
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) Validate() error {
	if c.BatchSize <= 0 {
		return fmt.Errorf("webhook batch size must be positive")
	}
	if c.PollInterval <= 0 {
		return fmt.Errorf("webhook poll interval must be positive")
	}
	if c.RetryBaseDelay <= 0 {
		return fmt.Errorf("webhook retry base must be positive")
	}
	if c.RetryMaxDelay < c.RetryBaseDelay {
		return fmt.Errorf("webhook retry max must be >= base")
	}
	if c.InflightTimeout <= 0 {
		return fmt.Errorf("webhook inflight timeout must be positive")
	}
	if c.MaxAttempts <= 0 {
		return fmt.Errorf("webhook max attempts must be positive")
	}
	if c.WorkerConcurrency <= 0 {
		return fmt.Errorf("webhook worker concurrency must be positive")
	}
	if c.HTTPTimeout <= 0 {
		return fmt.Errorf("webhook http timeout must be positive")
	}
	return nil
}

func (c Config) Enabled() bool {
	return c.EnabledFlag
}
