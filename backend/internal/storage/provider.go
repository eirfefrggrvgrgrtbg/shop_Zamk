package storage

import (
	"context"
	"io"
)

type Provider interface {
	UploadImage(ctx context.Context, reader io.Reader, objectSize int64, objectKey string, contentType string) (*StoredObject, error)
	DownloadObject(ctx context.Context, objectKey string) ([]byte, error)
	DownloadObjectBounded(ctx context.Context, objectKey string, maxBytes int64) ([]byte, error)
	DeleteObject(ctx context.Context, objectKey string) error
	BuildPublicURL(objectKey string) string
}
