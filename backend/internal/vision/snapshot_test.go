package vision

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestComputeSnapshotHash_Deterministic(t *testing.T) {
	pID := uuid.New()
	cID := uuid.New()
	revID := uuid.New()

	color1 := uuid.New()
	color2 := uuid.New()

	img1 := uuid.New()
	img2 := uuid.New()

	cropX := 10.5
	cropY := 20.0

	snap1 := ProductVisionSnapshot{
		ProductID:          pID,
		ProductRevisionID:  &revID,
		Title:              "T-Shirt",
		Description:        "A nice shirt",
		DeclaredCategoryID: cID,
		DeclaredCategory:   "Clothing",
		DeclaredColors:     []uuid.UUID{color1, color2},
		DeclaredOptions:    map[string]string{"Size": "M", "Fit": "Regular"},
		Images: []SnapshotImage{
			{ImageID: img1, ObjectKey: "img1.jpg", RenditionObjectKey: "rend1.jpg", CropX: &cropX, CropY: &cropY, SortOrder: 1, IsMain: true},
			{ImageID: img2, ObjectKey: "img2.jpg", SortOrder: 2, IsMain: false},
		},
	}

	snap2 := ProductVisionSnapshot{
		ProductID:          pID,
		ProductRevisionID:  &revID,
		Title:              "T-Shirt",
		Description:        "A nice shirt",
		DeclaredCategoryID: cID,
		DeclaredCategory:   "Clothing",
		DeclaredColors:     []uuid.UUID{color2, color1}, // Reversed order
		DeclaredOptions:    map[string]string{"Fit": "Regular", "Size": "M"}, // Map iteration/definition order
		Images: []SnapshotImage{
			{ImageID: img2, ObjectKey: "img2.jpg", SortOrder: 2, IsMain: false}, // Reversed order
			{ImageID: img1, ObjectKey: "img1.jpg", RenditionObjectKey: "rend1.jpg", CropX: &cropX, CropY: &cropY, SortOrder: 1, IsMain: true},
		},
	}

	hash1 := ComputeSnapshotHash(snap1)
	hash2 := ComputeSnapshotHash(snap2)

	require.Equal(t, hash1, hash2, "Snapshot hashes should be deterministic regardless of slice or map order")

	// Changing crop changes hash
	newCrop := 15.0
	snap3 := snap1
	snap3.Images = []SnapshotImage{
		{ImageID: img1, ObjectKey: "img1.jpg", RenditionObjectKey: "rend1.jpg", CropX: &newCrop, CropY: &cropY, SortOrder: 1, IsMain: true},
		{ImageID: img2, ObjectKey: "img2.jpg", SortOrder: 2, IsMain: false},
	}
	require.NotEqual(t, hash1, ComputeSnapshotHash(snap3))

	// Changing rendition key changes hash
	snap4 := snap1
	snap4.Images = []SnapshotImage{
		{ImageID: img1, ObjectKey: "img1.jpg", RenditionObjectKey: "rend2.jpg", CropX: &cropX, CropY: &cropY, SortOrder: 1, IsMain: true},
		{ImageID: img2, ObjectKey: "img2.jpg", SortOrder: 2, IsMain: false},
	}
	require.NotEqual(t, hash1, ComputeSnapshotHash(snap4))

	// Changing vision content version changes hash
	snap5 := snap1
	snap5.VisionContentVersion = 2
	require.NotEqual(t, hash1, ComputeSnapshotHash(snap5))
}

func TestComputeIdempotencyKey(t *testing.T) {
	key1 := ComputeIdempotencyKey("cloudru", "qwen", "v1", "s1", "tax1", "snap1")
	key2 := ComputeIdempotencyKey("cloudru", "qwen", "v1", "s1", "tax1", "snap1")
	require.Equal(t, key1, key2)

	// Different provider
	keyProvider := ComputeIdempotencyKey("other", "qwen", "v1", "s1", "tax1", "snap1")
	require.NotEqual(t, key1, keyProvider)

	// Different snapshot
	keySnap := ComputeIdempotencyKey("cloudru", "qwen", "v1", "s1", "tax1", "snap2")
	require.NotEqual(t, key1, keySnap)
}
