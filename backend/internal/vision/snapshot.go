package vision

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
)

// ComputeSnapshotHash generates a deterministic SHA-256 fingerprint
// of the logical product state that matters for vision analysis.
// It explicitly ignores image raw bytes and relies strictly on stable
// product metadata, declared categories/colors/options, and canonical media identities.
func ComputeSnapshotHash(snap ProductVisionSnapshot) string {
	type imageRef struct {
		ImageID            string   `json:"imageId"`
		ObjectKey          string   `json:"objectKey"`
		RenditionObjectKey string   `json:"renditionObjectKey,omitempty"`
		CropX              *float64 `json:"cropX,omitempty"`
		CropY              *float64 `json:"cropY,omitempty"`
		CropWidth          *float64 `json:"cropWidth,omitempty"`
		CropHeight         *float64 `json:"cropHeight,omitempty"`
		SortOrder          int      `json:"sortOrder"`
		IsMain             bool     `json:"isMain"`
		ColorID            *string  `json:"colorId,omitempty"`
	}

	type canonical struct {
		ProductID            string            `json:"productId"`
		ProductRevisionID    *string           `json:"productRevisionId,omitempty"`
		VisionContentVersion int64             `json:"visionContentVersion"`
		Title                string            `json:"title"`
		Description          string            `json:"description"`
		DeclaredCategoryID   string            `json:"declaredCategoryId"`
		DeclaredCategory     string            `json:"declaredCategory"`
		DeclaredColors       []string          `json:"declaredColors"`
		DeclaredOptions      map[string]string `json:"declaredOptions"`
		Images               []imageRef        `json:"images"`
	}

	c := canonical{
		ProductID:            snap.ProductID.String(),
		VisionContentVersion: snap.VisionContentVersion,
		Title:                snap.Title,
		Description:          snap.Description,
		DeclaredCategoryID:   snap.DeclaredCategoryID.String(),
		DeclaredCategory:     snap.DeclaredCategory,
		DeclaredOptions:      snap.DeclaredOptions,
	}

	if snap.ProductRevisionID != nil {
		revStr := snap.ProductRevisionID.String()
		c.ProductRevisionID = &revStr
	}

	// Sort colors for determinism
	if len(snap.DeclaredColors) > 0 {
		colors := make([]string, len(snap.DeclaredColors))
		for i, col := range snap.DeclaredColors {
			colors[i] = col.String()
		}
		sort.Strings(colors)
		c.DeclaredColors = colors
	}

	// Sort images by SortOrder, then ImageID for determinism
	if len(snap.Images) > 0 {
		imgs := make([]imageRef, len(snap.Images))
		for i, img := range snap.Images {
			ref := imageRef{
				ImageID:            img.ImageID.String(),
				ObjectKey:          img.ObjectKey,
				RenditionObjectKey: img.RenditionObjectKey,
				CropX:              img.CropX,
				CropY:              img.CropY,
				CropWidth:          img.CropWidth,
				CropHeight:         img.CropHeight,
				SortOrder:          img.SortOrder,
				IsMain:             img.IsMain,
			}
			if img.ColorID != nil {
				cIDStr := img.ColorID.String()
				ref.ColorID = &cIDStr
			}
			imgs[i] = ref
		}
		sort.Slice(imgs, func(i, j int) bool {
			if imgs[i].SortOrder != imgs[j].SortOrder {
				return imgs[i].SortOrder < imgs[j].SortOrder
			}
			return imgs[i].ImageID < imgs[j].ImageID
		})
		c.Images = imgs
	}

	b, _ := json.Marshal(c)
	hash := sha256.Sum256(b)
	return hex.EncodeToString(hash[:])
}

// ComputeIdempotencyKey constructs a unique key for a specific inference payload.
// Explicitly incorporates provider, model, prompt_version, schema_version,
// taxonomy_context_hash, and snapshot_hash.
func ComputeIdempotencyKey(provider, modelID, promptVersion, schemaVersion, taxonomyContextHash, snapshotHash string) string {
	raw := provider + "|" + modelID + "|" + promptVersion + "|" + schemaVersion + "|" + taxonomyContextHash + "|" + snapshotHash
	hash := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(hash[:])
}
