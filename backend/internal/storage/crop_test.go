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
