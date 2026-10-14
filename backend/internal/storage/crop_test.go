package storage_test

import (
	"testing"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCropImageRequest_Validate(t *testing.T) {
	tests := []struct {
		name    string
		req     storage.CropImageRequest
		wantErr bool
	}{
		{
			name: "valid full crop",
			req: storage.CropImageRequest{
				CropX:      0,
				CropY:      0,
				CropWidth:  1.0,
				CropHeight: 1.0,
			},
			wantErr: false,
		},
		{
			name: "valid center crop normalized",
			req: storage.CropImageRequest{
				CropX:      0.1,
				CropY:      0.05,
				CropWidth:  0.8,
				CropHeight: 0.9,
			},
			wantErr: false,
		},
		{
			name: "invalid negative cropX",
			req: storage.CropImageRequest{
				CropX:      -0.1,
				CropY:      0,
				CropWidth:  0.8,
				CropHeight: 0.8,
			},
			wantErr: true,
		},
		{
			name: "invalid negative cropY",
			req: storage.CropImageRequest{
				CropX:      0,
				CropY:      -0.1,
				CropWidth:  0.8,
				CropHeight: 0.8,
			},
			wantErr: true,
		},
		{
			name: "invalid zero cropWidth",
			req: storage.CropImageRequest{
				CropX:      0,
				CropY:      0,
				CropWidth:  0,
				CropHeight: 0.8,
			},
			wantErr: true,
		},
		{
			name: "invalid zero cropHeight",
			req: storage.CropImageRequest{
				CropX:      0,
				CropY:      0,
				CropWidth:  0.8,
				CropHeight: 0,
			},
			wantErr: true,
		},
		{
			name: "invalid cropX out of bounds",
			req: storage.CropImageRequest{
				CropX:      1.1,
				CropY:      0,
				CropWidth:  0.5,
				CropHeight: 0.5,
			},
			wantErr: true,
		},
		{
			name: "invalid cropX + cropWidth > 1",
			req: storage.CropImageRequest{
				CropX:      0.6,
				CropY:      0,
				CropWidth:  0.6,
				CropHeight: 0.5,
			},
			wantErr: true,
		},
		{
			name: "invalid cropY + cropHeight > 1",
			req: storage.CropImageRequest{
				CropX:      0,
				CropY:      0.7,
				CropWidth:  0.5,
				CropHeight: 0.5,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()
			if tt.wantErr {
				require.Error(t, err)
				assert.Equal(t, storage.ErrInvalidCropParameters, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestCalculateDefault4x5Crop(t *testing.T) {
	t.Run("Exact 4:5 aspect portrait (e.g. 1000x1250)", func(t *testing.T) {
		crop := storage.CalculateDefault4x5Crop(1000, 1250)
		assert.Equal(t, 1.0, crop.CropWidth)
		assert.Equal(t, 1.0, crop.CropHeight)
		assert.Equal(t, 0.0, crop.CropX)
		assert.Equal(t, 0.0, crop.CropY)
		assert.NoError(t, crop.Validate())

		pixelW := float64(1000) * crop.CropWidth
		pixelH := float64(1250) * crop.CropHeight
		assert.InDelta(t, 0.8, pixelW/pixelH, 0.0001)
	})

	t.Run("Tall portrait (e.g. 1000x2000)", func(t *testing.T) {
		crop := storage.CalculateDefault4x5Crop(1000, 2000)
		assert.Equal(t, 1.0, crop.CropWidth)
		assert.InDelta(t, 0.625, crop.CropHeight, 0.0001) // (1000 / 0.8) / 2000 = 1250 / 2000 = 0.625
		assert.Equal(t, 0.0, crop.CropX)
		assert.InDelta(t, 0.1875, crop.CropY, 0.0001) // (1.0 - 0.625) / 2 = 0.1875
		assert.NoError(t, crop.Validate())

		pixelW := float64(1000) * crop.CropWidth
		pixelH := float64(2000) * crop.CropHeight
		assert.InDelta(t, 0.8, pixelW/pixelH, 0.0001)
	})

	t.Run("Square image (e.g. 1200x1200)", func(t *testing.T) {
		crop := storage.CalculateDefault4x5Crop(1200, 1200)
		assert.Equal(t, 1.0, crop.CropHeight)
		assert.InDelta(t, 0.8, crop.CropWidth, 0.0001) // (1200 * 0.8) / 1200 = 0.8
		assert.InDelta(t, 0.1, crop.CropX, 0.0001)     // (1.0 - 0.8) / 2 = 0.1
		assert.Equal(t, 0.0, crop.CropY)
		assert.NoError(t, crop.Validate())

		pixelW := float64(1200) * crop.CropWidth
		pixelH := float64(1200) * crop.CropHeight
		assert.InDelta(t, 0.8, pixelW/pixelH, 0.0001)
	})

	t.Run("Landscape image (e.g. 1600x1000)", func(t *testing.T) {
		crop := storage.CalculateDefault4x5Crop(1600, 1000)
		assert.Equal(t, 1.0, crop.CropHeight)
		assert.InDelta(t, 0.5, crop.CropWidth, 0.0001) // (1000 * 0.8) / 1600 = 800 / 1600 = 0.5
		assert.InDelta(t, 0.25, crop.CropX, 0.0001)    // (1.0 - 0.5) / 2 = 0.25
		assert.Equal(t, 0.0, crop.CropY)
		assert.NoError(t, crop.Validate())

		pixelW := float64(1600) * crop.CropWidth
		pixelH := float64(1000) * crop.CropHeight
		assert.InDelta(t, 0.8, pixelW/pixelH, 0.0001)
	})

	t.Run("Zero or invalid dimensions return empty crop", func(t *testing.T) {
		crop := storage.CalculateDefault4x5Crop(0, 0)
		assert.Equal(t, 0.0, crop.CropWidth)
		assert.Equal(t, 0.0, crop.CropHeight)
	})
}
