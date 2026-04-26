package objectstore

import (
	"errors"
	"fmt"
	"strings"

	"github.com/animus-labs/animus-go/closed/internal/platform/env"
)

type Config struct {
	Endpoint        string
	AccessKey       string
	SecretKey       string
	Region          string
	UseSSL          bool
	BucketDatasets  string
	BucketArtifacts string
}

func ConfigFromEnv() (Config, error) {
	useSSL, err := env.Bool("ANIMUS_MINIO_USE_SSL", false)
	if err != nil {
		return Config{}, err
	}
	cfg := Config{
		Endpoint:        env.String("ANIMUS_MINIO_ENDPOINT", "localhost:9000"),
		AccessKey:       env.String("ANIMUS_MINIO_ACCESS_KEY", "animus"),
		SecretKey:       env.String("ANIMUS_MINIO_SECRET_KEY", "animusminio"),
		Region:          env.String("ANIMUS_MINIO_REGION", "us-east-1"),
		UseSSL:          useSSL,
		BucketDatasets:  env.String("ANIMUS_MINIO_BUCKET_DATASETS", "datasets"),
		BucketArtifacts: env.String("ANIMUS_MINIO_BUCKET_ARTIFACTS", "artifacts"),
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) Validate() error {
	if strings.TrimSpace(c.Endpoint) == "" {
		return errors.New("endpoint is required")
	}
	if strings.TrimSpace(c.AccessKey) == "" {
		return errors.New("access key is required")
	}
	if strings.TrimSpace(c.SecretKey) == "" {
		return errors.New("secret key is required")
	}
	if strings.TrimSpace(c.Region) == "" {
		return errors.New("region is required")
	}
	if strings.TrimSpace(c.BucketDatasets) == "" {
		return errors.New("datasets bucket is required")
	}
	if strings.TrimSpace(c.BucketArtifacts) == "" {
		return errors.New("artifacts bucket is required")
	}
	if strings.Contains(c.Endpoint, "://") {
		return fmt.Errorf("endpoint must not include scheme: %q", c.Endpoint)
	}
	return nil
}
