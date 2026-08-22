package products_test

import (
	"context"
	"testing"
	"fmt"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/products"
)

func TestSemanticFinalization(t *testing.T) {
	dbClient, svc, sellerUserID := setupTestDB(t)
	pool := dbClient.Pool
	ctx := context.Background()

	catID := uuid.New()
	_, err := pool.Exec(ctx, "INSERT INTO categories (id, name, slug, size_chart_required) VALUES ($1, 'Semantic Cat', $2, true)", catID, "sem-cat-" + uuid.New().String())
	require.NoError(t, err)

	var colorID uuid.UUID
	err = pool.QueryRow(ctx, "SELECT id FROM attribute_definitions WHERE code = 'COLOR'").Scan(&colorID)
	require.NoError(t, err)

	var sizeID uuid.UUID
	err = pool.QueryRow(ctx, "SELECT id FROM attribute_definitions WHERE code = 'SIZE'").Scan(&sizeID)
	require.NoError(t, err)

	var compID uuid.UUID
	err = pool.QueryRow(ctx, "SELECT id FROM attribute_definitions WHERE code = 'MATERIAL_COMPOSITION'").Scan(&compID)
	require.NoError(t, err)

	seasonDefID := uuid.New()
	seasonDictID := uuid.New()
	_, err = pool.Exec(ctx, "INSERT INTO attribute_dictionaries (id, code, name_ru) VALUES ($1, $2, 'Сезоны')", seasonDictID, "SEASONS-" + uuid.New().String())
	require.NoError(t, err)
	
	springID := uuid.New()
	_, err = pool.Exec(ctx, "INSERT INTO attribute_dictionary_values (id, dictionary_id, code, name_ru, is_active) VALUES ($1, $2, 'SPRING', 'Весна', true)", springID, seasonDictID)
	require.NoError(t, err)
	
	autumnID := uuid.New()
	_, err = pool.Exec(ctx, "INSERT INTO attribute_dictionary_values (id, dictionary_id, code, name_ru, is_active) VALUES ($1, $2, 'AUTUMN', 'Осень', false)", autumnID, seasonDictID)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, "INSERT INTO attribute_definitions (id, code, name_ru, value_type, value_source, scope, is_active) VALUES ($1, $2, 'Сезон', 'ENUM', 'DICTIONARY', 'PRODUCT', true)", seasonDefID, "SEASON-" + uuid.New().String())
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
		INSERT INTO category_attribute_definitions (category_id, attribute_definition_id, required, dictionary_id) VALUES 
		($1, $2, true, NULL),
		($1, $3, true, NULL),
		($1, $4, true, NULL),
		($1, $5, true, $6)
	`, catID, colorID, sizeID, compID, seasonDefID, seasonDictID)
	require.NoError(t, err)

	var sysEuID uuid.UUID
	err = pool.QueryRow(ctx, "SELECT id FROM size_systems WHERE code = 'EU'").Scan(&sysEuID)
	require.NoError(t, err)
	
	_, err = pool.Exec(ctx, "INSERT INTO category_size_systems (category_id, size_system_id, is_default) VALUES ($1, $2, true)", catID, sysEuID)
	require.NoError(t, err)
	
	sysIntID := uuid.New()
	pool.Exec(ctx, "INSERT INTO size_systems (id, code, name, is_active) VALUES ($1, 'INT', 'International', true) ON CONFLICT (code) DO NOTHING", sysIntID)
	err = pool.QueryRow(ctx, "SELECT id FROM size_systems WHERE code = 'INT'").Scan(&sysIntID)
	require.NoError(t, err)
	
	sizeIntM := uuid.New()
	_, err = pool.Exec(ctx, "INSERT INTO size_values (id, size_system_id, value, is_active) VALUES ($1, $2, $3, true)", sizeIntM, sysIntID, "M-" + uuid.New().String())
	require.NoError(t, err)
	
	var sizeEu42 uuid.UUID
	err = pool.QueryRow(ctx, "SELECT id FROM size_values WHERE size_system_id = $1 AND value = '42'", sysEuID).Scan(&sizeEu42)
	require.NoError(t, err)

	colorRedID := uuid.New()
	_, err = pool.Exec(ctx, "INSERT INTO colors (id, code, name_ru, hex, is_active) VALUES ($1, $2, 'Красный', '#FF0000', true)", colorRedID, "RED-" + uuid.New().String())
	require.NoError(t, err)

	matCottonID := uuid.New()
	_, err = pool.Exec(ctx, "INSERT INTO materials (id, code, name_ru, is_active) VALUES ($1, $2, 'Хлопок', true)", matCottonID, "COTTON-" + uuid.New().String())
	require.NoError(t, err)

	_, err = pool.Exec(ctx, "INSERT INTO category_size_chart_fields (id, category_id, code, name, is_required) VALUES (gen_random_uuid(), $1, 'LENGTH', 'Длина', true)", catID)
	require.NoError(t, err)
	
	t.Run("Valid Full Creation", func(t *testing.T) {
		req := products.CreateProductRequest{
			Title:      "Perfect Roundtrip",
			Slug:       func(s string) *string { return &s }(uuid.New().String()),
			CategoryID: &catID,
			PriceCents: 1000,
			Attributes: []products.ProductAttributeValueRequest{
				{AttributeDefinitionID: seasonDefID, EnumValueID: &springID},
			},
			MaterialComposition: []products.ProductMaterialCompositionRequest{
				{MaterialID: matCottonID, Percentage: 100},
			},
			Variants: []products.ProductVariantRequest{
				{
					SellerSKU:   func(s string) *string { return &s }("SEM-1"),
					ColorID:     &colorRedID,
					SizeValueID: &sizeEu42,
					PriceCents:  func(i int64) *int64 { return &i }(1000),
				},
			},
			SizeChartRows: []products.ProductSizeChartRowRequest{
				{
					SizeValueID: sizeEu42,
					Measurements: map[string]interface{}{"LENGTH": float64(100)},
				},
			},
		}

		p, err := svc.CreateProductForSeller(ctx, sellerUserID, req)
		require.NoError(t, err)
		require.Equal(t, 1, len(p.Variants))
		require.Equal(t, colorRedID, *p.Variants[0].ColorID)
		require.Equal(t, sizeEu42, *p.Variants[0].SizeValueID)
		require.Equal(t, "SEM-1", *p.Variants[0].SellerSKU)
		
		fetched, err := svc.GetSellerProduct(ctx, sellerUserID, p.ID)
		require.NoError(t, err)
		
		require.Equal(t, 1, len(fetched.Attributes)) 
		require.Equal(t, 0, len(fetched.Variants[0].Attributes)) 
		require.Equal(t, 1, len(fetched.MaterialComposition))
		
		require.Equal(t, springID, *fetched.Attributes[0].EnumValueID)
		require.Equal(t, matCottonID, fetched.MaterialComposition[0].MaterialID)
		require.Equal(t, sizeEu42, fetched.SizeChart.Rows[0].SizeValueID)
	})

	t.Run("Missing Core Attribute Rejection", func(t *testing.T) {
		req := products.CreateProductRequest{
			Title:      "Missing Core",
			Slug:       func(s string) *string { return &s }(uuid.New().String()),
			CategoryID: &catID,
			Attributes: []products.ProductAttributeValueRequest{
				{AttributeDefinitionID: seasonDefID, EnumValueID: &springID},
			},
			MaterialComposition: []products.ProductMaterialCompositionRequest{
				{MaterialID: matCottonID, Percentage: 100},
			},
			Variants: []products.ProductVariantRequest{
				{
					SellerSKU:   func(s string) *string { return &s }("SEM-2"),
					SizeValueID: &sizeEu42,
				},
			},
		}
		_, err := svc.CreateProductForSeller(ctx, sellerUserID, req)
		require.Error(t, err)
		require.Contains(t, fmt.Sprintf("%v", err), "required variant attribute missing: COLOR")
		
		req.Variants[0].ColorID = &colorRedID
		req.Variants[0].SizeValueID = nil
		_, err = svc.CreateProductForSeller(ctx, sellerUserID, req)
		require.Error(t, err)
		require.Contains(t, fmt.Sprintf("%v", err), "required variant attribute missing: SIZE")
		
		req.Variants[0].SizeValueID = &sizeEu42
		req.MaterialComposition = nil 
		_, err = svc.CreateProductForSeller(ctx, sellerUserID, req)
		require.Error(t, err)
		require.Contains(t, fmt.Sprintf("%v", err), "required product attribute missing: MATERIAL_COMPOSITION")
	})

	t.Run("Invalid Size System Rejection", func(t *testing.T) {
		req := products.CreateProductRequest{
			Title:      "Wrong Size Sys",
			Slug:       func(s string) *string { return &s }(uuid.New().String()),
			CategoryID: &catID,
			Attributes: []products.ProductAttributeValueRequest{
				{AttributeDefinitionID: seasonDefID, EnumValueID: &springID},
			},
			MaterialComposition: []products.ProductMaterialCompositionRequest{
				{MaterialID: matCottonID, Percentage: 100},
			},
			Variants: []products.ProductVariantRequest{
				{
					SellerSKU:   func(s string) *string { return &s }("SEM-3"),
					ColorID:     &colorRedID,
					SizeValueID: &sizeIntM, 
				},
			},
		}
		_, err := svc.CreateProductForSeller(ctx, sellerUserID, req)
		require.Error(t, err)
		require.Contains(t, fmt.Sprintf("%v", err), "belongs to a size system not allowed")
	})
	
	t.Run("Inactive Dictionary Value Rejection", func(t *testing.T) {
		req := products.CreateProductRequest{
			Title:      "Inactive Dict Val",
			Slug:       func(s string) *string { return &s }(uuid.New().String()),
			CategoryID: &catID,
			Attributes: []products.ProductAttributeValueRequest{
				{AttributeDefinitionID: seasonDefID, EnumValueID: &autumnID},
			},
			MaterialComposition: []products.ProductMaterialCompositionRequest{
				{MaterialID: matCottonID, Percentage: 100},
			},
			Variants: []products.ProductVariantRequest{
				{
					SellerSKU:   func(s string) *string { return &s }("SEM-4"),
					ColorID:     &colorRedID,
					SizeValueID: &sizeEu42,
				},
			},
		}
		_, err := svc.CreateProductForSeller(ctx, sellerUserID, req)
		require.Error(t, err)
		require.Contains(t, fmt.Sprintf("%v", err), "is invalid or inactive")
	})
	
	t.Run("Reference Data API", func(t *testing.T) {
		schema, err := svc.GetCategoryAttributeSchema(ctx, catID)
		require.NoError(t, err)
		require.NotNil(t, schema)
		
		schemaMap := schema.(map[string]interface{})
		attrs := schemaMap["attributes"].([]map[string]interface{})
		require.Equal(t, 4, len(attrs))
		
		allowedSizes := schemaMap["allowedSizeSystems"].([]map[string]interface{})
		require.Equal(t, 1, len(allowedSizes))
		require.Equal(t, "EU", allowedSizes[0]["code"])
	})
}
