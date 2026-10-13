package vision

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"syscall"

	"github.com/minio/minio-go/v7"
)

const MaxImageSizeBytes = 5 * 1024 * 1024 // 5MB

var (
	ErrUnsupportedImageFormat = errors.New("unsupported image format")
	ErrImageTooLarge          = errors.New("image size exceeds limit")
	ErrImageMissing           = errors.New("image missing from storage")

	// Canonical classified errors (VISION.3B1)
	ErrMediaMissing     = errors.New("media_missing: object not found in storage")
	ErrMediaInvalid     = errors.New("media_invalid: corrupt, oversized, or unsupported media")
	ErrStorageTransient = errors.New("storage_transient: temporary storage or network failure")
)

type StorageProvider interface {
	DownloadObject(ctx context.Context, objectKey string) ([]byte, error)
}

type BoundedStorageProvider interface {
	DownloadObjectBounded(ctx context.Context, objectKey string, maxBytes int64) ([]byte, error)
}

type VisionImageLoader interface {
	LoadImages(ctx context.Context, snapshot ProductVisionSnapshot) ([]VisionInputImage, error)
}

type DefaultImageLoader struct {
	storage StorageProvider
}

func NewImageLoader(storage StorageProvider) *DefaultImageLoader {
	return &DefaultImageLoader{storage: storage}
}

func (l *DefaultImageLoader) LoadImages(ctx context.Context, snapshot ProductVisionSnapshot) ([]VisionInputImage, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	inputs := make([]VisionInputImage, 0, len(snapshot.Images))

	for _, img := range snapshot.Images {
		// Prefer rendition key, fallback to source object key
		key := img.RenditionObjectKey
		if key == "" {
			key = img.ObjectKey
		}

		// Empty keys MUST fail closed (BLOCKER 2)
		if key == "" {
			return nil, fmt.Errorf("%w: image %s has empty object key and rendition key", ErrMediaInvalid, img.ImageID)
		}

		// Bounded read (BLOCKER 3)
		var rawBytes []byte
		var err error

		if bounded, ok := l.storage.(BoundedStorageProvider); ok {
			rawBytes, err = bounded.DownloadObjectBounded(ctx, key, MaxImageSizeBytes)
		} else {
			rawBytes, err = l.storage.DownloadObject(ctx, key)
		}

		if err != nil {
			// Classify error (BLOCKER 1)
			classifiedErr := classifyStorageError(ctx, err, key)
			return nil, classifiedErr
		}

		if len(rawBytes) == 0 {
			return nil, fmt.Errorf("%w: image %s (%s) is empty", ErrMediaInvalid, img.ImageID, key)
		}

		if len(rawBytes) > MaxImageSizeBytes {
			return nil, fmt.Errorf("%w: image %s (%s) exceeds max size %d bytes", ErrMediaInvalid, img.ImageID, key, MaxImageSizeBytes)
		}

		mimeType := http.DetectContentType(rawBytes)
		if mimeType != "image/jpeg" && mimeType != "image/png" && mimeType != "image/webp" {
			return nil, fmt.Errorf("%w: image %s (%s) unsupported MIME type %s", ErrMediaInvalid, img.ImageID, key, mimeType)
		}

		inputs = append(inputs, VisionInputImage{
			ImageID:  img.ImageID,
			MIMEType: mimeType,
			Bytes:    rawBytes,
		})
	}

	return inputs, nil
}

func classifyStorageError(ctx context.Context, err error, key string) error {
	// D. Context cancelled / deadline exceeded
	if errors.Is(err, context.Canceled) || errors.Is(ctx.Err(), context.Canceled) {
		return context.Canceled
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return context.DeadlineExceeded
	}

	// B. Bounded size violation -> media_invalid
	if errors.Is(err, ErrImageTooLarge) || err.Error() == "file too large" {
		return fmt.Errorf("%w: %s file too large", ErrMediaInvalid, key)
	}

	// A. Object definitively missing / not found -> media_missing
	var minioErr minio.ErrorResponse
	if errors.As(err, &minioErr) {
		if minioErr.Code == "NoSuchKey" || minioErr.StatusCode == http.StatusNotFound {
			return fmt.Errorf("%w: %s (%v)", ErrMediaMissing, key, err)
		}
		if minioErr.StatusCode >= 500 {
			return fmt.Errorf("%w: S3 server error %d for %s (%v)", ErrStorageTransient, minioErr.StatusCode, key, err)
		}
	}
	if errors.Is(err, ErrImageMissing) || err.Error() == "object not found" {
		return fmt.Errorf("%w: %s (%v)", ErrMediaMissing, key, err)
	}

	// C. Timeout / temporary network / S3 5xx / temporary dependency failure -> storage_transient
	var netErr net.Error
	if errors.As(err, &netErr) {
		return fmt.Errorf("%w: network error for %s (%v)", ErrStorageTransient, key, err)
	}
	if errors.Is(err, syscall.ECONNREFUSED) || errors.Is(err, syscall.ECONNRESET) || os.IsTimeout(err) {
		return fmt.Errorf("%w: connection/timeout error for %s (%v)", ErrStorageTransient, key, err)
	}

	return fmt.Errorf("%w: storage dependency failure for %s (%v)", ErrStorageTransient, key, err)
}
