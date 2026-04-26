package objectstore

import (
	"context"
	"fmt"
	"io"
	"time"

	platformstore "github.com/animus-labs/animus-go/closed/internal/platform/objectstore"
	"github.com/minio/minio-go/v7"
)

type MinioStore struct {
	client *minio.Client
}

func NewMinioStore(cfg platformstore.Config) (*MinioStore, error) {
	client, err := platformstore.NewMinIOClient(cfg)
	if err != nil {
		return nil, err
	}
	return &MinioStore{client: client}, nil
}

func NewMinioStoreWithClient(client *minio.Client) (*MinioStore, error) {
	if client == nil {
		return nil, fmt.Errorf("minio client is required")
	}
	return &MinioStore{client: client}, nil
}

func (s *MinioStore) Put(ctx context.Context, bucket, key string, body io.Reader, size int64, contentType string) error {
	if s == nil || s.client == nil {
		return fmt.Errorf("minio store not initialized")
	}
	opts := minio.PutObjectOptions{ContentType: contentType}
	_, err := s.client.PutObject(ctx, bucket, key, body, size, opts)
	return err
}

func (s *MinioStore) Get(ctx context.Context, bucket, key string) (io.ReadCloser, ObjectInfo, error) {
	if s == nil || s.client == nil {
		return nil, ObjectInfo{}, fmt.Errorf("minio store not initialized")
	}
	info, err := s.Stat(ctx, bucket, key)
	if err != nil {
		return nil, ObjectInfo{}, err
	}
	obj, err := s.client.GetObject(ctx, bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, ObjectInfo{}, err
	}
	return obj, info, nil
}

func (s *MinioStore) Stat(ctx context.Context, bucket, key string) (ObjectInfo, error) {
	if s == nil || s.client == nil {
		return ObjectInfo{}, fmt.Errorf("minio store not initialized")
	}
	info, err := s.client.StatObject(ctx, bucket, key, minio.StatObjectOptions{})
	if err != nil {
		return ObjectInfo{}, err
	}
	return ObjectInfo{
		Key:          info.Key,
		Size:         info.Size,
		ETag:         info.ETag,
		ContentType:  info.ContentType,
		LastModified: info.LastModified,
	}, nil
}

func (s *MinioStore) Delete(ctx context.Context, bucket, key string) error {
	if s == nil || s.client == nil {
		return fmt.Errorf("minio store not initialized")
	}
	return s.client.RemoveObject(ctx, bucket, key, minio.RemoveObjectOptions{})
}

func (s *MinioStore) PresignPut(ctx context.Context, bucket, key string, ttl time.Duration) (string, error) {
	if s == nil || s.client == nil {
		return "", fmt.Errorf("minio store not initialized")
	}
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	u, err := s.client.PresignedPutObject(ctx, bucket, key, ttl)
	if err != nil {
		return "", err
	}
	return u.String(), nil
}

func (s *MinioStore) PresignGet(ctx context.Context, bucket, key string, ttl time.Duration) (string, error) {
	if s == nil || s.client == nil {
		return "", fmt.Errorf("minio store not initialized")
	}
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	u, err := s.client.PresignedGetObject(ctx, bucket, key, ttl, nil)
	if err != nil {
		return "", err
	}
	return u.String(), nil
}
