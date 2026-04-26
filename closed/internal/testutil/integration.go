package testutil

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"
	"time"

	platformstore "github.com/animus-labs/animus-go/closed/internal/platform/objectstore"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/minio/minio-go/v7"
)

type IntegrationConfig struct {
	DatabaseURL     string
	MinioEndpoint   string
	MinioAccessKey  string
	MinioSecretKey  string
	MinioRegion     string
	MinioUseSSL     bool
	BucketDatasets  string
	BucketArtifacts string
}

func RequireIntegrationConfig(t *testing.T) IntegrationConfig {
	t.Helper()
	if os.Getenv("ANIMUS_INTEGRATION") != "1" {
		t.Skip("integration tests disabled (set ANIMUS_INTEGRATION=1)")
	}
	cfg := IntegrationConfig{
		DatabaseURL:     envOr("ANIMUS_TEST_DATABASE_URL", envOr("DATABASE_URL", "")),
		MinioEndpoint:   envOr("ANIMUS_TEST_MINIO_ENDPOINT", envOr("ANIMUS_MINIO_ENDPOINT", "localhost:9000")),
		MinioAccessKey:  envOr("ANIMUS_TEST_MINIO_ACCESS_KEY", envOr("ANIMUS_MINIO_ACCESS_KEY", "animus")),
		MinioSecretKey:  envOr("ANIMUS_TEST_MINIO_SECRET_KEY", envOr("ANIMUS_MINIO_SECRET_KEY", "animusminio")),
		MinioRegion:     envOr("ANIMUS_TEST_MINIO_REGION", envOr("ANIMUS_MINIO_REGION", "us-east-1")),
		BucketDatasets:  envOr("ANIMUS_TEST_MINIO_BUCKET_DATASETS", envOr("ANIMUS_MINIO_BUCKET_DATASETS", "datasets")),
		BucketArtifacts: envOr("ANIMUS_TEST_MINIO_BUCKET_ARTIFACTS", envOr("ANIMUS_MINIO_BUCKET_ARTIFACTS", "artifacts")),
	}
	if cfg.DatabaseURL == "" {
		t.Fatalf("integration DATABASE_URL is required")
	}
	cfg.MinioUseSSL = strings.EqualFold(envOr("ANIMUS_TEST_MINIO_USE_SSL", envOr("ANIMUS_MINIO_USE_SSL", "false")), "true")
	return cfg
}

func OpenIntegrationDB(t *testing.T, databaseURL string) *sql.DB {
	t.Helper()
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(2 * time.Minute)
	if err := db.Ping(); err != nil {
		_ = db.Close()
		t.Fatalf("ping db: %v", err)
	}
	return db
}

func MinioClientFromConfig(t *testing.T, cfg IntegrationConfig) *minio.Client {
	t.Helper()
	storeCfg := platformstore.Config{
		Endpoint:        cfg.MinioEndpoint,
		AccessKey:       cfg.MinioAccessKey,
		SecretKey:       cfg.MinioSecretKey,
		Region:          cfg.MinioRegion,
		UseSSL:          cfg.MinioUseSSL,
		BucketDatasets:  cfg.BucketDatasets,
		BucketArtifacts: cfg.BucketArtifacts,
	}
	client, err := platformstore.NewMinIOClient(storeCfg)
	if err != nil {
		t.Fatalf("minio client: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := platformstore.EnsureBuckets(ctx, client, storeCfg); err != nil {
		t.Fatalf("ensure buckets: %v", err)
	}
	return client
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
