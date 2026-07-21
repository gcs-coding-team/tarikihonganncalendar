package storage

import (
	"context"
	"io"
	"time"
)

type Client interface {
	PresignedPutURL(ctx context.Context, objectKey string, ttl time.Duration) (string, time.Time, error)
	GetObject(ctx context.Context, objectKey string) (io.ReadCloser, error)
	DeleteObject(ctx context.Context, objectKey string) error
}
