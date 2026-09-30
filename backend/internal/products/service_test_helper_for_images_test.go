package products_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/products"
)

func injectValidMainImage(t *testing.T, ctx context.Context, repo *products.Repository, productID uuid.UUID) {
	variants, err := repo.GetProductVariants(ctx, productID)
	require.NoError(t, err)

	activeColors := make(map[uuid.UUID]bool)
	var activeColorList []uuid.UUID
	for _, v := range variants {
		if v.IsActive && v.ColorID != nil && *v.ColorID != uuid.Nil {
			if !activeColors[*v.ColorID] {
				activeColors[*v.ColorID] = true
				activeColorList = append(activeColorList, *v.ColorID)
			}
		}
	}

	existingImages, err := repo.GetProductImages(ctx, productID)
	require.NoError(t, err)

	for _, img := range existingImages {
		_ = repo.DeleteProductImage(ctx, img.ID)
	}

	totalNeeded := 3
	if len(activeColorList) > totalNeeded {
		totalNeeded = len(activeColorList)
	}

	for i := 0; i < totalNeeded; i++ {
		var colID *uuid.UUID
		if len(activeColorList) > 0 {
			col := activeColorList[i%len(activeColorList)]
			colID = &col
		}
		imgID := uuid.New()
		img := &products.ProductImage{
			ID:        imgID,
			ProductID: productID,
			ImageURL:  fmt.Sprintf("https://storage.zamk.test/img_%s_%d.jpg", productID.String()[:8], i),
			IsMain:    (i == 0),
			ColorID:   colID,
			SortOrder: i,
		}
		err = repo.AddProductImage(ctx, img)
		require.NoError(t, err)

		rendKey := fmt.Sprintf("rend_%s_%d.jpg", productID.String()[:8], i)
		err = repo.UpdateProductImageCrop(ctx, imgID, 0.1, 0.1, 0.8, 1.0, "https://storage.zamk.test/"+rendKey, rendKey)
		require.NoError(t, err)
	}
}
