package objectstore

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

func NewMinIOClient(cfg Config) (*minio.Client, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	opts := &minio.Options{
		Creds:     credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure:    cfg.UseSSL,
		Region:    cfg.Region,
		Transport: newTransport(),
	}
	return minio.New(cfg.Endpoint, opts)
}

func EnsureBuckets(ctx context.Context, client *minio.Client, cfg Config) error {
	if err := ensureBucket(ctx, client, cfg.BucketDatasets, cfg.Region); err != nil {
		return fmt.Errorf("ensure datasets bucket: %w", err)
	}
	if err := ensureBucket(ctx, client, cfg.BucketArtifacts, cfg.Region); err != nil {
		return fmt.Errorf("ensure artifacts bucket: %w", err)
	}
	return nil
}

func CheckBuckets(ctx context.Context, client *minio.Client, cfg Config) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	datasetsExists, err := client.BucketExists(ctx, cfg.BucketDatasets)
	if err != nil {
		return fmt.Errorf("datasets bucket exists: %w", err)
	}
	if !datasetsExists {
		return fmt.Errorf("datasets bucket missing: %s", cfg.BucketDatasets)
	}

	artifactsExists, err := client.BucketExists(ctx, cfg.BucketArtifacts)
	if err != nil {
		return fmt.Errorf("artifacts bucket exists: %w", err)
	}
	if !artifactsExists {
		return fmt.Errorf("artifacts bucket missing: %s", cfg.BucketArtifacts)
	}
	return nil
}

func ensureBucket(ctx context.Context, client *minio.Client, bucket string, region string) error {
	exists, err := client.BucketExists(ctx, bucket)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	return client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{Region: region})
}

func newTransport() *http.Transport {
	dialer := &net.Dialer{
		Timeout:   5 * time.Second,
		KeepAlive: 30 * time.Second,
	}
	return &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           dialer.DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
}
