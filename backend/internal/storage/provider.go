package storage

import (
	"context"
	"io"
)

type Provider interface {
	UploadImage(ctx context.Context, reader io.Reader, objectSize int64, objectKey string, contentType string) (*StoredObject, error)
	DeleteObject(ctx context.Context, objectKey string) error
	BuildPublicURL(objectKey string) string
}
