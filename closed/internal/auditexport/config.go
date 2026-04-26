package auditexport

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/animus-labs/animus-go/closed/internal/platform/env"
)

// Config controls audit export format and destination.
type Config struct {
	Format           string
	Destination      string
	WebhookURL       string
	WebhookHeaders   map[string]string
	WebhookSecretRef string
	SigningSecretKey string
	SyslogAddr       string
	SyslogProtocol   string
	SyslogTag        string

	BatchSize         int
	PollInterval      time.Duration
	RetryBaseDelay    time.Duration
	RetryMaxDelay     time.Duration
	InflightTimeout   time.Duration
	HTTPTimeout       time.Duration
	MaxAttempts       int
	WorkerConcurrency int
}

func ConfigFromEnv() (Config, error) {
	headersRaw := strings.TrimSpace(env.String("AUDIT_EXPORT_WEBHOOK_HEADERS_JSON", ""))
	headers := map[string]string{}
	if headersRaw != "" {
		if err := json.Unmarshal([]byte(headersRaw), &headers); err != nil {
			return Config{}, fmt.Errorf("invalid AUDIT_EXPORT_WEBHOOK_HEADERS_JSON: %w", err)
		}
	}
	batchSize, err := env.Int("AUDIT_EXPORT_BATCH_SIZE", 50)
	if err != nil {
		return Config{}, err
	}
	pollInterval, err := env.Duration("AUDIT_EXPORT_POLL_INTERVAL", 5*time.Second)
	if err != nil {
		return Config{}, err
	}
	retryBase, err := env.Duration("AUDIT_EXPORT_RETRY_BASE", 5*time.Second)
	if err != nil {
		return Config{}, err
	}
	retryMax, err := env.Duration("AUDIT_EXPORT_RETRY_MAX", 5*time.Minute)
	if err != nil {
		return Config{}, err
	}
	inflightTimeout, err := env.Duration("AUDIT_EXPORT_INFLIGHT_TIMEOUT", 2*time.Minute)
	if err != nil {
		return Config{}, err
	}
	httpTimeout, err := env.Duration("AUDIT_EXPORT_HTTP_TIMEOUT", 10*time.Second)
	if err != nil {
		return Config{}, err
	}
	maxAttempts, err := env.Int("AUDIT_EXPORT_MAX_ATTEMPTS", 10)
	if err != nil {
		return Config{}, err
	}
	workerConcurrency, err := env.Int("AUDIT_EXPORT_WORKER_CONCURRENCY", 4)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		Format:            env.String("AUDIT_EXPORT_FORMAT", "ndjson"),
		Destination:       env.String("AUDIT_EXPORT_DESTINATION", "none"),
		WebhookURL:        env.String("AUDIT_EXPORT_WEBHOOK_URL", ""),
		WebhookHeaders:    headers,
		WebhookSecretRef:  strings.TrimSpace(env.String("AUDIT_EXPORT_WEBHOOK_SECRET_REF", "")),
		SigningSecretKey:  strings.TrimSpace(env.String("AUDIT_EXPORT_SIGNING_SECRET_KEY", "")),
		SyslogAddr:        env.String("AUDIT_EXPORT_SYSLOG_ADDR", ""),
		SyslogProtocol:    env.String("AUDIT_EXPORT_SYSLOG_PROTOCOL", "udp"),
		SyslogTag:         env.String("AUDIT_EXPORT_SYSLOG_TAG", "animus-audit"),
		BatchSize:         batchSize,
		PollInterval:      pollInterval,
		RetryBaseDelay:    retryBase,
		RetryMaxDelay:     retryMax,
		InflightTimeout:   inflightTimeout,
		HTTPTimeout:       httpTimeout,
		MaxAttempts:       maxAttempts,
		WorkerConcurrency: workerConcurrency,
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) Validate() error {
	format := strings.ToLower(strings.TrimSpace(c.Format))
	destination := normalizeDestination(c.Destination)
	if format == "" {
		format = "ndjson"
	}
	if format != "ndjson" {
		return fmt.Errorf("unsupported audit export format: %s", format)
	}
	if destination == "none" {
		return nil
	}
	switch destination {
	case "webhook":
		if strings.TrimSpace(c.WebhookURL) == "" {
			return fmt.Errorf("audit export webhook url required")
		}
	case "syslog", "syslog_tcp", "syslog_udp":
		if strings.TrimSpace(c.SyslogAddr) == "" {
			return fmt.Errorf("audit export syslog addr required")
		}
		proto := strings.ToLower(strings.TrimSpace(c.SyslogProtocol))
		if proto == "" {
			proto = "udp"
		}
		if proto != "udp" && proto != "tcp" {
			return fmt.Errorf("unsupported syslog protocol: %s", proto)
		}
	default:
		return fmt.Errorf("unsupported audit export destination: %s", destination)
	}
	if c.BatchSize <= 0 {
		return fmt.Errorf("audit export batch size must be positive")
	}
	if c.PollInterval <= 0 {
		return fmt.Errorf("audit export poll interval must be positive")
	}
	if c.RetryBaseDelay <= 0 {
		return fmt.Errorf("audit export retry base must be positive")
	}
	if c.RetryMaxDelay < c.RetryBaseDelay {
		return fmt.Errorf("audit export retry max must be >= base")
	}
	if c.InflightTimeout <= 0 {
		return fmt.Errorf("audit export inflight timeout must be positive")
	}
	if c.HTTPTimeout <= 0 {
		return fmt.Errorf("audit export http timeout must be positive")
	}
	if c.MaxAttempts <= 0 {
		return fmt.Errorf("audit export max attempts must be positive")
	}
	if c.WorkerConcurrency <= 0 {
		return fmt.Errorf("audit export worker concurrency must be positive")
	}
	return nil
}

func (c Config) Enabled() bool {
	return normalizeDestination(c.Destination) != "none"
}

func normalizeDestination(value string) string {
	dest := strings.ToLower(strings.TrimSpace(value))
	switch dest {
	case "", "none", "disabled", "off":
		return "none"
	case "http", "https":
		return "webhook"
	default:
		return dest
	}
}
