package products_test

import (
	"context"
	"fmt"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/products"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestDraftAndModeration(t *testing.T) {
	dbClient, svc, sellerUserID := setupBlockATestDB(t)
	pool := dbClient.Pool
	ctx := context.Background()

	// 1. Create a strict category with required attributes, variants, etc.
	catID := uuid.New()
	_, err := pool.Exec(ctx, "INSERT INTO categories (id, name, slug, size_chart_required) VALUES ($1, 'Strict Category', $2, true)", catID, "strict-"+uuid.New().String())
	require.NoError(t, err)

	// Fetch color attribute definition
	var colorID uuid.UUID
	err = pool.QueryRow(ctx, "SELECT id FROM attribute_definitions WHERE code = 'COLOR'").Scan(&colorID)
	require.NoError(t, err)

	// Add color to category schema as REQUIRED
	_, err = pool.Exec(ctx, "INSERT INTO category_attribute_definitions (category_id, attribute_definition_id, required, filterable, variant_axis) VALUES ($1, $2, true, true, true)", catID, colorID)
	require.NoError(t, err)

	t.Run("Save Incomplete Draft Successfully", func(t *testing.T) {
		req := products.CreateProductRequest{
			Title:      "Incomplete Product",
			CategoryID: &catID,
			// No sizes, no color, no size chart, no variants
		}

		p, err := svc.CreateProductForSeller(ctx, sellerUserID, req)
		require.NoError(t, err)
		require.Equal(t, products.StatusDraft, p.Status)

		// Submit should fail strict validation
		submitReq := products.SubmitProductModerationRequest{}
		err = svc.SubmitProductToModeration(ctx, sellerUserID, p.ID, submitReq)
		require.Error(t, err)
		require.Contains(t, fmt.Sprintf("%v", err), "moderation validation failed")
	})
}

func TestCreateProduct_RejectsNonLeafCategory(t *testing.T) {
	dbClient, svc, sellerID := setupBlockATestDB(t)
	pool := dbClient.Pool
	ctx := context.Background()

	// Create parent
	parentID := uuid.New()
	_, err := pool.Exec(ctx, "INSERT INTO categories (id, name, slug, sort_order) VALUES ($1, 'Parent123', $2, 1)", parentID, parentID.String())
	require.NoError(t, err)

	// Create child
	childID := uuid.New()
	_, err = pool.Exec(ctx, "INSERT INTO categories (id, parent_id, name, slug, sort_order) VALUES ($1, $2, 'Child123', $3, 2)", childID, parentID, childID.String())
	require.NoError(t, err)

	// Try creating with parent
	_, err = svc.CreateProductForSeller(ctx, sellerID, products.CreateProductRequest{
		Title:       "Test",
		Description: nil,
		CategoryID:  &parentID,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "cannot use non-leaf category")
}

func TestCreateProduct_RejectsInactiveCategory(t *testing.T) {
	dbClient, svc, sellerID := setupBlockATestDB(t)
	pool := dbClient.Pool
	ctx := context.Background()

	// Create inactive leaf
	inactiveID := uuid.New()
	_, err := pool.Exec(ctx, "INSERT INTO categories (id, name, slug, sort_order, is_active) VALUES ($1, 'Inactive Cat', $2, 1, false)", inactiveID, inactiveID.String())
	require.NoError(t, err)

	// Try creating with inactive category
	_, err = svc.CreateProductForSeller(ctx, sellerID, products.CreateProductRequest{
		Title:      "Test",
		CategoryID: &inactiveID,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "cannot use inactive category")
}

func TestCompositionValidation(t *testing.T) {
	dbClient, svc, sellerID := setupBlockATestDB(t)
	pool := dbClient.Pool
	ctx := context.Background()

	// Get a valid material
	var materialID uuid.UUID
	err := pool.QueryRow(ctx, "SELECT id FROM materials WHERE is_active = true LIMIT 1").Scan(&materialID)
	require.NoError(t, err)

	// Inactive material
	inactiveMatID := uuid.New()
	_, err = pool.Exec(ctx, "INSERT INTO materials (id, code, name_ru, sort_order, is_active) VALUES ($1, $2, 'INACT', 99, false)", inactiveMatID, inactiveMatID.String())
	require.NoError(t, err)

	// Helper for creating req
	makeReq := func(comps []products.ProductMaterialCompositionRequest) products.CreateProductRequest {
		return products.CreateProductRequest{
			Title:               "Test " + uuid.New().String(),
			MaterialComposition: comps,
		}
	}

	t.Run("percentage <= 0 rejected", func(t *testing.T) {
		_, err := svc.CreateProductForSeller(ctx, sellerID, makeReq([]products.ProductMaterialCompositionRequest{
			{MaterialID: materialID, Percentage: 0},
		}))
		require.ErrorContains(t, err, "percentage must be > 0 and <= 100")
	})

	t.Run("percentage > 100 rejected", func(t *testing.T) {
		_, err := svc.CreateProductForSeller(ctx, sellerID, makeReq([]products.ProductMaterialCompositionRequest{
			{MaterialID: materialID, Percentage: 101},
		}))
		require.ErrorContains(t, err, "percentage must be > 0 and <= 100")
	})

	t.Run("duplicate material rejected", func(t *testing.T) {
		_, err := svc.CreateProductForSeller(ctx, sellerID, makeReq([]products.ProductMaterialCompositionRequest{
			{MaterialID: materialID, Percentage: 40},
			{MaterialID: materialID, Percentage: 60},
		}))
		require.ErrorContains(t, err, "duplicate material entry")
	})

	t.Run("draft total > 100 rejected", func(t *testing.T) {
		var matID2 uuid.UUID
		err := pool.QueryRow(ctx, "SELECT id FROM materials WHERE is_active = true AND id != $1 LIMIT 1", materialID).Scan(&matID2)
		require.NoError(t, err)
		_, err = svc.CreateProductForSeller(ctx, sellerID, makeReq([]products.ProductMaterialCompositionRequest{
			{MaterialID: materialID, Percentage: 60},
			{MaterialID: matID2, Percentage: 50},
		}))
		require.ErrorContains(t, err, "total material composition exceeds 100")
	})

	t.Run("unknown/inactive material rejected", func(t *testing.T) {
		_, err := svc.CreateProductForSeller(ctx, sellerID, makeReq([]products.ProductMaterialCompositionRequest{
			{MaterialID: uuid.New(), Percentage: 100},
		}))
		require.ErrorContains(t, err, "unknown material")

		_, err = svc.CreateProductForSeller(ctx, sellerID, makeReq([]products.ProductMaterialCompositionRequest{
			{MaterialID: inactiveMatID, Percentage: 100},
		}))
		require.ErrorContains(t, err, "inactive material")
	})
}

func TestDraftSaveSemantics(t *testing.T) {
	dbClient, svc, sellerID := setupBlockATestDB(t)
	pool := dbClient.Pool
	ctx := context.Background()

	t.Run("incomplete draft can save before Photo upload", func(t *testing.T) {
		req := products.CreateProductRequest{
			Title:    "Minimal Draft " + uuid.New().String(),
			Currency: "RUB", // As now required by frontend
		}
		p, err := svc.CreateProductForSeller(ctx, sellerID, req)
		require.NoError(t, err)
		require.NotEmpty(t, p.ID)

		// Assert no inventory is created from draft save
		var invCount int
		err = pool.QueryRow(ctx, "SELECT count(*) FROM inventory_items WHERE product_id = $1", p.ID).Scan(&invCount)
		require.NoError(t, err)
		require.Equal(t, 0, invCount, "No inventory should be created for draft save")
	})

	t.Run("draft with composition <100 can save", func(t *testing.T) {
		var materialID uuid.UUID
		err := pool.QueryRow(ctx, "SELECT id FROM materials WHERE is_active = true LIMIT 1").Scan(&materialID)
		require.NoError(t, err)

		req := products.CreateProductRequest{
			Title:    "Draft Comp " + uuid.New().String(),
			Currency: "RUB",
			MaterialComposition: []products.ProductMaterialCompositionRequest{
				{MaterialID: materialID, Percentage: 50},
			},
		}
		p, err := svc.CreateProductForSeller(ctx, sellerID, req)
		require.NoError(t, err)
		require.NotEmpty(t, p.ID)
	})
}

func TestDraftUpdate_PhotoToReviewLifecycle(t *testing.T) {
	dbClient, svc, sellerID := setupBlockATestDB(t)
	pool := dbClient.Pool
	ctx := context.Background()

	// Get a valid leaf category
	var catID uuid.UUID
	err := pool.QueryRow(ctx, "SELECT id FROM categories WHERE is_active = true AND NOT EXISTS (SELECT 1 FROM categories c2 WHERE c2.parent_id = categories.id) LIMIT 1").Scan(&catID)
	require.NoError(t, err)

	var colorID uuid.UUID
	err = pool.QueryRow(ctx, "SELECT id FROM colors WHERE is_active = true LIMIT 1").Scan(&colorID)
	require.NoError(t, err)

	var sizeValueID uuid.UUID
	err = pool.QueryRow(ctx, "SELECT id FROM size_values WHERE is_active = true LIMIT 1").Scan(&sizeValueID)
	require.NoError(t, err)

	// 1. Initial creation at Photo step (autosave)
	createReq := products.CreateProductRequest{
		Title:      "Photo Step Draft " + uuid.New().String(),
		CategoryID: &catID,
		Currency:   "RUB",
		Variants: []products.ProductVariantRequest{
			{
				ColorID:     &colorID,
				SizeValueID: &sizeValueID,
			},
		},
	}
	p1, err := svc.CreateProductForSeller(ctx, sellerID, createReq)
	require.NoError(t, err)
	require.NotEmpty(t, p1.ID)
	require.Len(t, p1.Variants, 1)
	initialVariantID := p1.Variants[0].ID
	initialBarcode := p1.Variants[0].Barcode
	require.NotEmpty(t, initialVariantID)
	require.NotEmpty(t, initialBarcode)

	// 2. Review step: update latest state without variant IDs
	price := int64(150000)
	sku := "SKU-" + uuid.New().String()[:8]
	titleUpd := "Updated Title " + uuid.New().String()
	updateReq := products.UpdateProductRequest{
		Title:      &titleUpd,
		CategoryID: &catID,
		Variants: []products.ProductVariantRequest{
			{
				ColorID:     &colorID,
				SizeValueID: &sizeValueID,
				PriceCents:  &price,
				SellerSKU:   &sku,
			},
		},
	}

	p2, err := svc.UpdateProductForSeller(ctx, sellerID, p1.ID, updateReq)
	require.NoError(t, err)
	require.Equal(t, p1.ID, p2.ID, "Same product ID must be preserved")
	require.Len(t, p2.Variants, 1)
	require.Equal(t, initialVariantID, p2.Variants[0].ID, "Variant ID must be preserved via canonical combination matching")
	require.Equal(t, initialBarcode, p2.Variants[0].Barcode, "Barcode must be preserved")
	require.Equal(t, &price, p2.Variants[0].PriceCents)
	require.Equal(t, &sku, p2.Variants[0].SellerSKU)

	// Assert no inventory created from save
	var invCount int
	err = pool.QueryRow(ctx, "SELECT count(*) FROM inventory_items WHERE product_id = $1", p1.ID).Scan(&invCount)
	require.NoError(t, err)
	require.Equal(t, 0, invCount, "No inventory should be created from draft save/update")
}

func TestSubmitModeration_DuplicateAndLifecycle(t *testing.T) {
	dbClient, svc, sellerID := setupBlockATestDB(t)
	pool := dbClient.Pool
	ctx := context.Background()

	// Get a valid leaf category
	var catID uuid.UUID
	err := pool.QueryRow(ctx, "SELECT id FROM categories WHERE is_active = true AND NOT EXISTS (SELECT 1 FROM categories c2 WHERE c2.parent_id = categories.id) LIMIT 1").Scan(&catID)
	require.NoError(t, err)

	// 1. Create a product draft without variants
	createReq := products.CreateProductRequest{
		Title:      "Draft Incomplete " + uuid.New().String(),
		CategoryID: &catID,
		Currency:   "RUB",
	}
	p, err := svc.CreateProductForSeller(ctx, sellerID, createReq)
	require.NoError(t, err)

	// 2. Moderation validation failure
	err = svc.SubmitProductToModeration(ctx, sellerID, p.ID, products.SubmitProductModerationRequest{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "moderation validation failed")

	// Draft remains in draft status after failed submission
	pSaved, err := svc.GetSellerProduct(ctx, sellerID, p.ID)
	require.NoError(t, err)
	require.Equal(t, products.StatusDraft, pSaved.Status)

	// 3. Second call on already transitioned product
	// Set status to pending_moderation manually to test double submission rejection
	_, err = pool.Exec(ctx, "UPDATE products SET status = 'pending_moderation' WHERE id = $1", p.ID)
	require.NoError(t, err)

	err = svc.SubmitProductToModeration(ctx, sellerID, p.ID, products.SubmitProductModerationRequest{})
	require.Error(t, err)
	require.ErrorIs(t, err, products.ErrInvalidStatusTransition, "Submitting product already in pending_moderation must fail")
}

func TestCategorySchema_RepresentativeCategories(t *testing.T) {
	dbClient, svc, _ := setupBlockATestDB(t)
	pool := dbClient.Pool
	ctx := context.Background()

	// 1. Tops: Майки и топы
	var tankTopID uuid.UUID
	err := pool.QueryRow(ctx, "SELECT id FROM categories WHERE slug = 'tank-tops'").Scan(&tankTopID)
	require.NoError(t, err)
	schemaTopsRaw, err := svc.GetCategoryAttributeSchema(ctx, tankTopID)
	require.NoError(t, err)
	schemaTops := schemaTopsRaw.(map[string]interface{})
	require.True(t, schemaTops["sizeChartRequired"].(bool))
	fieldsTops := schemaTops["sizeChartFields"].([]map[string]interface{})
	require.Len(t, fieldsTops, 2)
	require.Equal(t, "CHEST", fieldsTops[0]["code"])
	require.Equal(t, "LENGTH", fieldsTops[1]["code"])

	// 2. Tops with sleeves: Худи
	var hoodieID uuid.UUID
	err = pool.QueryRow(ctx, "SELECT id FROM categories WHERE slug = 'hoodies'").Scan(&hoodieID)
	require.NoError(t, err)
	schemaHoodieRaw, err := svc.GetCategoryAttributeSchema(ctx, hoodieID)
	require.NoError(t, err)
	schemaHoodie := schemaHoodieRaw.(map[string]interface{})
	require.True(t, schemaHoodie["sizeChartRequired"].(bool))
	fieldsHoodie := schemaHoodie["sizeChartFields"].([]map[string]interface{})
	require.Len(t, fieldsHoodie, 3)

	// 3. Bottoms: Брюки
	var pantsID uuid.UUID
	err = pool.QueryRow(ctx, "SELECT id FROM categories WHERE slug = 'pants'").Scan(&pantsID)
	require.NoError(t, err)
	schemaPantsRaw, err := svc.GetCategoryAttributeSchema(ctx, pantsID)
	require.NoError(t, err)
	schemaPants := schemaPantsRaw.(map[string]interface{})
	require.True(t, schemaPants["sizeChartRequired"].(bool))
	fieldsPants := schemaPants["sizeChartFields"].([]map[string]interface{})
	require.Len(t, fieldsPants, 3)
	require.Equal(t, "WAIST", fieldsPants[0]["code"])
	require.Equal(t, "HIPS", fieldsPants[1]["code"])
	require.Equal(t, "LENGTH", fieldsPants[2]["code"])

	// 4. Footwear: Кроссовки
	var sneakersID uuid.UUID
	err = pool.QueryRow(ctx, "SELECT id FROM categories WHERE slug = 'sneakers'").Scan(&sneakersID)
	require.NoError(t, err)
	schemaSneakersRaw, err := svc.GetCategoryAttributeSchema(ctx, sneakersID)
	require.NoError(t, err)
	schemaSneakers := schemaSneakersRaw.(map[string]interface{})
	require.True(t, schemaSneakers["sizeChartRequired"].(bool))
	fieldsSneakers := schemaSneakers["sizeChartFields"].([]map[string]interface{})
	require.Len(t, fieldsSneakers, 1)
	require.Equal(t, "FOOT_LENGTH", fieldsSneakers[0]["code"])

	// 5. Accessory: Кепки
	var capsID uuid.UUID
	err = pool.QueryRow(ctx, "SELECT id FROM categories WHERE slug = 'caps'").Scan(&capsID)
	require.NoError(t, err)
	schemaCapsRaw, err := svc.GetCategoryAttributeSchema(ctx, capsID)
	require.NoError(t, err)
	schemaCaps := schemaCapsRaw.(map[string]interface{})
	require.False(t, schemaCaps["sizeChartRequired"].(bool))
	fieldsCaps := schemaCaps["sizeChartFields"].([]map[string]interface{})
	require.Empty(t, fieldsCaps)
}

func TestDraftSave_CategoryWithoutOrWithIncompleteSizeChart(t *testing.T) {
	dbClient, svc, sellerID := setupBlockATestDB(t)
	pool := dbClient.Pool
	ctx := context.Background()

	var tankTopID uuid.UUID
	err := pool.QueryRow(ctx, "SELECT id FROM categories WHERE slug = 'tank-tops'").Scan(&tankTopID)
	require.NoError(t, err)

	// 1. Create draft without size chart (incomplete draft autosave)
	p, err := svc.CreateProductForSeller(ctx, sellerID, products.CreateProductRequest{
		Title:      "Tank Top Draft",
		CategoryID: &tankTopID,
		Currency:   "RUB",
	})
	require.NoError(t, err)
	require.Equal(t, products.StatusDraft, p.Status)

	// 2. Update draft still without size chart
	updatedTitle := "Tank Top Draft Updated"
	updateReq := products.UpdateProductRequest{
		Title:       &updatedTitle,
		Description: p.Description,
		CategoryID:  &tankTopID,
	}
	pUpdated, err := svc.UpdateProductForSeller(ctx, sellerID, p.ID, updateReq)
	require.NoError(t, err)
	require.Equal(t, p.ID, pUpdated.ID)

	// 3. Moderation should reject when required size chart / attributes are missing
	err = svc.SubmitProductToModeration(ctx, sellerID, p.ID, products.SubmitProductModerationRequest{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "moderation validation failed")
}

func TestProductRejectionAndResubmissionFlow(t *testing.T) {
	dbClient, svc, sellerID := setupBlockATestDB(t)
	pool := dbClient.Pool
	ctx := context.Background()

	// 1. Create a leaf category without size chart requirement for simplicity
	var catID uuid.UUID
	err := pool.QueryRow(ctx, "SELECT c.id FROM categories c WHERE c.size_chart_required = false AND c.is_active = true AND NOT EXISTS (SELECT 1 FROM categories sub WHERE sub.parent_id = c.id) LIMIT 1").Scan(&catID)
	require.NoError(t, err)

	// Fetch color
	var colorID uuid.UUID
	err = pool.QueryRow(ctx, "SELECT id FROM colors WHERE is_active = true LIMIT 1").Scan(&colorID)
	require.NoError(t, err)

	// Fetch size value
	var sizeValueID uuid.UUID
	err = pool.QueryRow(ctx, "SELECT id FROM size_values WHERE is_active = true LIMIT 1").Scan(&sizeValueID)
	require.NoError(t, err)

	// Fetch material
	var materialID uuid.UUID
	err = pool.QueryRow(ctx, "SELECT id FROM materials WHERE is_active = true LIMIT 1").Scan(&materialID)
	require.NoError(t, err)

	title := "Rejection Test Product " + uuid.New().String()
	sku := "REJ-SKU-" + uuid.New().String()
	p, err := svc.CreateProductForSeller(ctx, sellerID, products.CreateProductRequest{
		Title:      title,
		CategoryID: &catID,
		Currency:   "RUB",
		PriceCents: 2000,
		MaterialComposition: []products.ProductMaterialCompositionRequest{
			{MaterialID: materialID, Percentage: 100},
		},
		Variants: []products.ProductVariantRequest{
			{
				SellerSKU:   &sku,
				ColorID:     &colorID,
				SizeValueID: &sizeValueID,
				PriceCents:  func() *int64 { v := int64(2000); return &v }(),
			},
		},
	})
	require.NoError(t, err)

	// Submit to moderation
	err = svc.SubmitProductToModeration(ctx, sellerID, p.ID, products.SubmitProductModerationRequest{})
	require.NoError(t, err)

	// Admin rejects product with reason
	adminUserID := uuid.New()
	_, err = pool.Exec(ctx, "INSERT INTO users (id, name, email, password_hash, role) VALUES ($1, 'Admin User', $2, 'hash', 'admin')", adminUserID, "admin-"+uuid.New().String()+"@test.local")
	require.NoError(t, err)

	rejectionReason := "Качественные фотографии не соответствуют правилам каталога"
	err = svc.RejectProduct(ctx, adminUserID, p.ID, rejectionReason)
	require.NoError(t, err)

	// C. Rejection reason persists
	pRejected, err := svc.GetSellerProduct(ctx, sellerID, p.ID)
	require.NoError(t, err)
	require.Equal(t, products.StatusRejected, pRejected.Status)
	require.NotNil(t, pRejected.ModerationComment)
	require.Equal(t, rejectionReason, *pRejected.ModerationComment)

	logs, err := svc.GetProductModerationHistory(ctx, p.SellerID, p.ID)
	require.NoError(t, err)
	require.NotEmpty(t, logs)
	require.Equal(t, products.StatusRejected, logs[0].ToStatus)
	require.NotNil(t, logs[0].Comment)
	require.Equal(t, rejectionReason, *logs[0].Comment)

	// D. Rejected product can be corrected and resubmitted
	newTitle := "Corrected Rejection Test Product " + uuid.New().String()
	pUpdated, err := svc.UpdateProductForSeller(ctx, sellerID, p.ID, products.UpdateProductRequest{
		Title: &newTitle,
	})
	require.NoError(t, err)
	require.Equal(t, newTitle, pUpdated.Title)

	// Resubmit to moderation
	err = svc.SubmitProductToModeration(ctx, sellerID, p.ID, products.SubmitProductModerationRequest{})
	require.NoError(t, err)

	pResubmitted, err := svc.GetSellerProduct(ctx, sellerID, p.ID)
	require.NoError(t, err)
	require.Equal(t, products.StatusPendingModeration, pResubmitted.Status)
}

func TestAdminDossierCanonicalFieldsAndPreview(t *testing.T) {
	dbClient, svc, sellerID := setupBlockATestDB(t)
	pool := dbClient.Pool
	ctx := context.Background()

	// 1. Fetch category, color, size, material
	var catID uuid.UUID
	err := pool.QueryRow(ctx, "SELECT c.id FROM categories c WHERE c.is_active = true AND NOT EXISTS (SELECT 1 FROM categories sub WHERE sub.parent_id = c.id) LIMIT 1").Scan(&catID)
	require.NoError(t, err)

	var colorID uuid.UUID
	var colorName string
	err = pool.QueryRow(ctx, "SELECT id, name_ru FROM colors WHERE is_active = true LIMIT 1").Scan(&colorID, &colorName)
	require.NoError(t, err)

	var sizeValueID uuid.UUID
	var sizeValue string
	err = pool.QueryRow(ctx, "SELECT id, value FROM size_values WHERE is_active = true LIMIT 1").Scan(&sizeValueID, &sizeValue)
	require.NoError(t, err)

	var materialID uuid.UUID
	var materialName string
	err = pool.QueryRow(ctx, "SELECT id, name_ru FROM materials WHERE is_active = true LIMIT 1").Scan(&materialID, &materialName)
	require.NoError(t, err)

	title := "Dossier Test Product " + uuid.New().String()
	sku := "DOSSIER-SKU-" + uuid.New().String()
	imageUrl := "http://localhost:9000/zamk-local/products/test.png"
	p, err := svc.CreateProductForSeller(ctx, sellerID, products.CreateProductRequest{
		Title:        title,
		CategoryID:   &catID,
		Currency:     "RUB",
		PriceCents:   450000,
		MainImageURL: &imageUrl,
		MaterialComposition: []products.ProductMaterialCompositionRequest{
			{MaterialID: materialID, Percentage: 100},
		},
		Variants: []products.ProductVariantRequest{
			{
				SellerSKU:   &sku,
				ColorID:     &colorID,
				SizeValueID: &sizeValueID,
				ShadeName:   ptr("Deep Shade"),
				PriceCents:  func() *int64 { v := int64(450000); return &v }(),
			},
		},
		SizeChartRows: []products.ProductSizeChartRowRequest{
			{
				SizeValueID: sizeValueID,
				Measurements: map[string]interface{}{
					"CHEST":  float64(100),
					"LENGTH": float64(70),
					"SLEEVE": float64(60),
				},
			},
		},
	})
	require.NoError(t, err)

	// A. Admin product detail returns canonical variant color/size/SKU/barcode
	adminProd, err := svc.GetAdminProductDetail(ctx, p.ID)
	require.NoError(t, err)
	require.NotEmpty(t, adminProd.Variants)
	v := adminProd.Variants[0]
	require.NotNil(t, v.Color)
	require.Equal(t, colorName, *v.Color)
	require.NotNil(t, v.Size)
	require.Equal(t, sizeValue, *v.Size)
	require.NotNil(t, v.SellerSKU)
	require.Equal(t, sku, *v.SellerSKU)
	require.NotNil(t, v.SKU)
	require.Equal(t, sku, *v.SKU)
	require.NotNil(t, v.Barcode)
	require.NotEmpty(t, *v.Barcode)
	require.NotNil(t, v.ShadeName)
	require.Equal(t, "Deep Shade", *v.ShadeName)

	// Material composition and size chart resolved names
	require.NotEmpty(t, adminProd.MaterialComposition)
	require.NotNil(t, adminProd.MaterialComposition[0].MaterialName)
	require.Equal(t, materialName, *adminProd.MaterialComposition[0].MaterialName)

	require.NotNil(t, adminProd.SizeChart)
	require.NotEmpty(t, adminProd.SizeChart.Rows)
	require.NotNil(t, adminProd.SizeChart.Rows[0].SizeValueName)
	require.Equal(t, sizeValue, *adminProd.SizeChart.Rows[0].SizeValueName)

	// B. Admin media DTO returns browser-consumable media reference
	require.NotNil(t, adminProd.MainImageURL)
	require.Equal(t, imageUrl, *adminProd.MainImageURL)

	// C. Pending Product preview link works without publishing Product
	adminUserID := uuid.New()
	_, err = pool.Exec(ctx, "INSERT INTO users (id, name, email, password_hash, role) VALUES ($1, 'Admin Preview', $2, 'hash', 'admin')", adminUserID, "admin-prev-"+uuid.New().String()+"@test.local")
	require.NoError(t, err)

	token, err := svc.CreateProductPreviewLink(ctx, adminUserID, p.ID)
	require.NoError(t, err)
	require.NotEmpty(t, token)

	previewProduct, err := svc.GetProductPreviewByToken(ctx, token)
	require.NoError(t, err)
	require.Equal(t, p.ID, previewProduct.ID)
	require.Equal(t, title, previewProduct.Title)

	// D. Pending Product remains excluded from public catalog
	publicList, err := svc.ListPublicProducts(ctx, products.PublicProductFilter{}, 100, 0)
	require.NoError(t, err)
	for _, item := range publicList.Items {
		require.NotEqual(t, p.ID, item.ID, "Pending product must NOT appear in public catalog")
	}

	// E. Zero-stock approved Product remains excluded from public catalog
	err = svc.SubmitProductToModeration(ctx, sellerID, p.ID, products.SubmitProductModerationRequest{})
	require.NoError(t, err)

	err = svc.ApproveProduct(ctx, adminUserID, p.ID, ptr("Approved for testing"))
	require.NoError(t, err)

	pApproved, err := svc.GetAdminProductDetail(ctx, p.ID)
	require.NoError(t, err)
	require.Equal(t, products.StatusApproved, pApproved.Status)
	require.Equal(t, 0, pApproved.AvailableStock)

	publicListAfterApprove, err := svc.ListPublicProducts(ctx, products.PublicProductFilter{}, 100, 0)
	require.NoError(t, err)
	for _, item := range publicListAfterApprove.Items {
		require.NotEqual(t, p.ID, item.ID, "Approved zero-stock product must NOT appear in public catalog")
	}
}
