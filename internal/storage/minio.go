package storage

import (
	"context"
	"io"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type MinioClient struct {
	client *minio.Client
	bucket string
}

type MinioConfig struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	Bucket    string
	Region    string
}

func NewMinioClient(cfg MinioConfig) (*MinioClient, error) {
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: false,
	})
	if err != nil {
		return nil, err
	}

	return &MinioClient{client: client, bucket: cfg.Bucket}, nil
}

func (m *MinioClient) BucketExists(ctx context.Context) (bool, error) {
	return m.client.BucketExists(ctx, m.bucket)
}

func (m *MinioClient) PresignedPutURL(ctx context.Context, objectKey string, ttl time.Duration) (string, time.Time, error) {
	url, err := m.client.PresignedPutObject(ctx, m.bucket, objectKey, ttl)
	if err != nil {
		return "", time.Time{}, err
	}
	return url.String(), time.Now().Add(ttl), nil
}

func (m *MinioClient) GetObject(ctx context.Context, objectKey string) (io.ReadCloser, error) {
	return m.client.GetObject(ctx, m.bucket, objectKey, minio.GetObjectOptions{})
}

func (m *MinioClient) DeleteObject(ctx context.Context, objectKey string) error {
	return m.client.RemoveObject(ctx, m.bucket, objectKey, minio.RemoveObjectOptions{})
}
