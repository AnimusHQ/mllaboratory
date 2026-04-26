package objectstore

import (
	"context"
	"io"
	"time"
)

// Store abstracts S3-compatible object storage.
type Store interface {
	Put(ctx context.Context, bucket, key string, body io.Reader, size int64, contentType string) error
	Get(ctx context.Context, bucket, key string) (io.ReadCloser, ObjectInfo, error)
	Stat(ctx context.Context, bucket, key string) (ObjectInfo, error)
	Delete(ctx context.Context, bucket, key string) error
	PresignPut(ctx context.Context, bucket, key string, ttl time.Duration) (string, error)
	PresignGet(ctx context.Context, bucket, key string, ttl time.Duration) (string, error)
}

type ObjectInfo struct {
	Key          string
	Size         int64
	ETag         string
	ContentType  string
	LastModified time.Time
}
