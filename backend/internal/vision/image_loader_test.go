package vision_test

import (
	"context"
	"errors"
	"fmt"
	"net"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/vision"
)

var (
	// Minimal valid magic byte headers for MIME detection
	validJPEG = append([]byte("\xFF\xD8\xFF\xE0\x00\x10JFIF\x00\x01\x01\x01\x00`\x00`\x00\x00"), make([]byte, 100)...)
	validPNG  = append([]byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR"), make([]byte, 100)...)
	validWEBP = append([]byte("RIFF\x24\x00\x00\x00WEBPVP8 "), make([]byte, 100)...)
)

type mockStorageProvider struct {
	downloads     map[string][]byte
	downloadErr   map[string]error
	boundedCalled bool
}

func newMockStorage() *mockStorageProvider {
	return &mockStorageProvider{
		downloads:   make(map[string][]byte),
		downloadErr: make(map[string]error),
	}
}

func (m *mockStorageProvider) DownloadObject(ctx context.Context, objectKey string) ([]byte, error) {
	if err, ok := m.downloadErr[objectKey]; ok {
		return nil, err
	}
	if data, ok := m.downloads[objectKey]; ok {
		return data, nil
	}
	return nil, fmt.Errorf("object not found: %s", objectKey)
}

func (m *mockStorageProvider) DownloadObjectBounded(ctx context.Context, objectKey string, maxBytes int64) ([]byte, error) {
	m.boundedCalled = true
	if err, ok := m.downloadErr[objectKey]; ok {
		return nil, err
	}
	data, ok := m.downloads[objectKey]
	if !ok {
		return nil, fmt.Errorf("object not found: %s", objectKey)
	}
	if maxBytes > 0 && int64(len(data)) > maxBytes {
		return nil, errors.New("file too large")
	}
	return data, nil
}

func TestImageLoader_Suite(t *testing.T) {
	ctx := context.Background()

	// 1. RenditionObjectKey preferred
	t.Run("1. RenditionObjectKey preferred", func(t *testing.T) {
		storage := newMockStorage()
		storage.downloads["rendition.jpg"] = validJPEG
		storage.downloads["source.jpg"] = validPNG

		imgID := uuid.New()
		snap := vision.ProductVisionSnapshot{
			Images: []vision.SnapshotImage{
				{ImageID: imgID, ObjectKey: "source.jpg", RenditionObjectKey: "rendition.jpg"},
			},
		}

		loader := vision.NewImageLoader(storage)
		res, err := loader.LoadImages(ctx, snap)
		require.NoError(t, err)
		require.Len(t, res, 1)
		assert.Equal(t, imgID, res[0].ImageID)
		assert.Equal(t, "image/jpeg", res[0].MIMEType)
	})

	// 2. ObjectKey fallback
	t.Run("2. ObjectKey fallback", func(t *testing.T) {
		storage := newMockStorage()
		storage.downloads["source.png"] = validPNG

		imgID := uuid.New()
		snap := vision.ProductVisionSnapshot{
			Images: []vision.SnapshotImage{
				{ImageID: imgID, ObjectKey: "source.png", RenditionObjectKey: ""},
			},
		}

		loader := vision.NewImageLoader(storage)
		res, err := loader.LoadImages(ctx, snap)
		require.NoError(t, err)
		require.Len(t, res, 1)
		assert.Equal(t, imgID, res[0].ImageID)
		assert.Equal(t, "image/png", res[0].MIMEType)
	})

	// 3. Empty keys fail
	t.Run("3. Empty keys fail closed", func(t *testing.T) {
		storage := newMockStorage()
		snap := vision.ProductVisionSnapshot{
			Images: []vision.SnapshotImage{
				{ImageID: uuid.New(), ObjectKey: "", RenditionObjectKey: ""},
			},
		}

		loader := vision.NewImageLoader(storage)
		_, err := loader.LoadImages(ctx, snap)
		require.Error(t, err)
		require.ErrorIs(t, err, vision.ErrMediaInvalid)
	})

	// 4. ImageID preserved & 5. Ordering preserved
	t.Run("4 & 5. ImageID and ordering preserved", func(t *testing.T) {
		storage := newMockStorage()
		storage.downloads["1.jpg"] = validJPEG
		storage.downloads["2.png"] = validPNG
		storage.downloads["3.webp"] = validWEBP

		id1 := uuid.New()
		id2 := uuid.New()
		id3 := uuid.New()

		snap := vision.ProductVisionSnapshot{
			Images: []vision.SnapshotImage{
				{ImageID: id1, ObjectKey: "1.jpg"},
				{ImageID: id2, ObjectKey: "2.png"},
				{ImageID: id3, ObjectKey: "3.webp"},
			},
		}

		loader := vision.NewImageLoader(storage)
		res, err := loader.LoadImages(ctx, snap)
		require.NoError(t, err)
		require.Len(t, res, 3)
		assert.Equal(t, id1, res[0].ImageID)
		assert.Equal(t, "image/jpeg", res[0].MIMEType)
		assert.Equal(t, id2, res[1].ImageID)
		assert.Equal(t, "image/png", res[1].MIMEType)
		assert.Equal(t, id3, res[2].ImageID)
		assert.Equal(t, "image/webp", res[2].MIMEType)
	})

	// 6. JPEG accepted
	t.Run("6. JPEG accepted", func(t *testing.T) {
		storage := newMockStorage()
		storage.downloads["test.jpg"] = validJPEG
		snap := vision.ProductVisionSnapshot{Images: []vision.SnapshotImage{{ImageID: uuid.New(), ObjectKey: "test.jpg"}}}
		loader := vision.NewImageLoader(storage)
		res, err := loader.LoadImages(ctx, snap)
		require.NoError(t, err)
		assert.Equal(t, "image/jpeg", res[0].MIMEType)
	})

	// 7. PNG accepted
	t.Run("7. PNG accepted", func(t *testing.T) {
		storage := newMockStorage()
		storage.downloads["test.png"] = validPNG
		snap := vision.ProductVisionSnapshot{Images: []vision.SnapshotImage{{ImageID: uuid.New(), ObjectKey: "test.png"}}}
		loader := vision.NewImageLoader(storage)
		res, err := loader.LoadImages(ctx, snap)
		require.NoError(t, err)
		assert.Equal(t, "image/png", res[0].MIMEType)
	})

	// 8. WEBP accepted
	t.Run("8. WEBP accepted", func(t *testing.T) {
		storage := newMockStorage()
		storage.downloads["test.webp"] = validWEBP
		snap := vision.ProductVisionSnapshot{Images: []vision.SnapshotImage{{ImageID: uuid.New(), ObjectKey: "test.webp"}}}
		loader := vision.NewImageLoader(storage)
		res, err := loader.LoadImages(ctx, snap)
		require.NoError(t, err)
		assert.Equal(t, "image/webp", res[0].MIMEType)
	})

	// 9. Unsupported MIME rejected
	t.Run("9. Unsupported MIME rejected", func(t *testing.T) {
		storage := newMockStorage()
		storage.downloads["test.gif"] = []byte("GIF89a...")
		snap := vision.ProductVisionSnapshot{Images: []vision.SnapshotImage{{ImageID: uuid.New(), ObjectKey: "test.gif"}}}
		loader := vision.NewImageLoader(storage)
		_, err := loader.LoadImages(ctx, snap)
		require.Error(t, err)
		require.ErrorIs(t, err, vision.ErrMediaInvalid)
	})

	// 10. Empty object rejected
	t.Run("10. Empty object rejected", func(t *testing.T) {
		storage := newMockStorage()
		storage.downloads["empty.jpg"] = []byte{}
		snap := vision.ProductVisionSnapshot{Images: []vision.SnapshotImage{{ImageID: uuid.New(), ObjectKey: "empty.jpg"}}}
		loader := vision.NewImageLoader(storage)
		_, err := loader.LoadImages(ctx, snap)
		require.Error(t, err)
		require.ErrorIs(t, err, vision.ErrMediaInvalid)
	})

	// 11. Oversized object rejected BEFORE unbounded full allocation
	t.Run("11. Oversized object rejected", func(t *testing.T) {
		storage := newMockStorage()
		storage.downloads["huge.jpg"] = make([]byte, vision.MaxImageSizeBytes+100)
		snap := vision.ProductVisionSnapshot{Images: []vision.SnapshotImage{{ImageID: uuid.New(), ObjectKey: "huge.jpg"}}}
		loader := vision.NewImageLoader(storage)
		_, err := loader.LoadImages(ctx, snap)
		require.Error(t, err)
		require.ErrorIs(t, err, vision.ErrMediaInvalid)
		assert.True(t, storage.boundedCalled, "Bounded storage method must be invoked to protect memory")
	})

	// 12. NotFound vs transient storage distinguished
	t.Run("12. NotFound vs transient storage distinguished", func(t *testing.T) {
		storage := newMockStorage()
		storage.downloadErr["missing.jpg"] = errors.New("object not found")
		storage.downloadErr["transient.jpg"] = &net.OpError{Op: "dial", Err: errors.New("connection reset")}

		loader := vision.NewImageLoader(storage)

		// Missing -> ErrMediaMissing
		snapMissing := vision.ProductVisionSnapshot{Images: []vision.SnapshotImage{{ImageID: uuid.New(), ObjectKey: "missing.jpg"}}}
		_, err := loader.LoadImages(ctx, snapMissing)
		require.Error(t, err)
		require.ErrorIs(t, err, vision.ErrMediaMissing)

		// Transient -> ErrStorageTransient
		snapTransient := vision.ProductVisionSnapshot{Images: []vision.SnapshotImage{{ImageID: uuid.New(), ObjectKey: "transient.jpg"}}}
		_, err = loader.LoadImages(ctx, snapTransient)
		require.Error(t, err)
		require.ErrorIs(t, err, vision.ErrStorageTransient)
	})
}
