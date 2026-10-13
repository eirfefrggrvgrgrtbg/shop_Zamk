package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/config"
)

type S3Client struct {
	client *minio.Client
	cfg    *config.S3Config
}

func NewS3Client(cfg *config.S3Config) (*S3Client, error) {
	// minio-go expects endpoint without scheme
	endpoint := cfg.Endpoint
	if cfg.Port != "" {
		endpoint = fmt.Sprintf("%s:%s", endpoint, cfg.Port)
	}

	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
		Region: cfg.Region,
	})
	if err != nil {
		return nil, err
	}

	return &S3Client{
		client: client,
		cfg:    cfg,
	}, nil
}

func (s *S3Client) UploadImage(ctx context.Context, reader io.Reader, objectSize int64, objectKey string, contentType string) (*StoredObject, error) {
	_, err := s.client.PutObject(ctx, s.cfg.Bucket, objectKey, reader, objectSize, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUploadFailed, err)
	}

	return &StoredObject{
		ObjectURL: s.BuildPublicURL(objectKey),
		ObjectKey: objectKey,
		Size:      objectSize,
	}, nil
}

func (s *S3Client) DeleteObject(ctx context.Context, objectKey string) error {
	return s.client.RemoveObject(ctx, s.cfg.Bucket, objectKey, minio.RemoveObjectOptions{})
}

func (s *S3Client) BuildPublicURL(objectKey string) string {
	if s.cfg.PublicBaseURL != "" {
		return fmt.Sprintf("%s/%s", strings.TrimRight(s.cfg.PublicBaseURL, "/"), objectKey)
	}

	// Fallback to construct from endpoint
	scheme := "http"
	if s.cfg.UseSSL {
		scheme = "https"
	}
	endpoint := s.cfg.Endpoint
	if s.cfg.Port != "" {
		endpoint = fmt.Sprintf("%s:%s", endpoint, s.cfg.Port)
	}
	return fmt.Sprintf("%s://%s/%s/%s", scheme, endpoint, s.cfg.Bucket, objectKey)
}

func (s *S3Client) DownloadObject(ctx context.Context, objectKey string) ([]byte, error) {
	return s.DownloadObjectBounded(ctx, objectKey, 0)
}

func (s *S3Client) DownloadObjectBounded(ctx context.Context, objectKey string, maxBytes int64) ([]byte, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	obj, err := s.client.GetObject(ctx, s.cfg.Bucket, objectKey, minio.GetObjectOptions{})
	if err != nil {
		return nil, wrapMinioError(err)
	}
	defer obj.Close()

	stat, err := obj.Stat()
	if err != nil {
		return nil, wrapMinioError(err)
	}

	if maxBytes > 0 && stat.Size > maxBytes {
		return nil, ErrFileTooLarge
	}

	var reader io.Reader = obj
	if maxBytes > 0 {
		reader = io.LimitReader(obj, maxBytes+1)
	}

	buf := new(bytes.Buffer)
	n, err := buf.ReadFrom(reader)
	if err != nil {
		return nil, wrapMinioError(err)
	}

	if maxBytes > 0 && n > maxBytes {
		return nil, ErrFileTooLarge
	}

	return buf.Bytes(), nil
}

func wrapMinioError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) {
		return context.Canceled
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return context.DeadlineExceeded
	}
	var errResp minio.ErrorResponse
	if errors.As(err, &errResp) {
		if errResp.Code == "NoSuchKey" || errResp.StatusCode == 404 {
			return ErrObjectNotFound
		}
	}
	return err
}
