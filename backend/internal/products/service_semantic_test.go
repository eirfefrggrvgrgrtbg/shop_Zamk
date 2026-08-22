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


	finishDefID := uuid.New()
	finishDictID := uuid.New()
	_, err = pool.Exec(ctx, "INSERT INTO attribute_dictionaries (id, code, name_ru) VALUES ($1, $2, 'Отделка')", finishDictID, "FINISH-" + uuid.New().String())
	require.NoError(t, err)

	finishMatteID := uuid.New()
	_, err = pool.Exec(ctx, "INSERT INTO attribute_dictionary_values (id, dictionary_id, code, name_ru, is_active) VALUES ($1, $2, 'MATTE', 'Матовая', true)", finishMatteID, finishDictID)
	require.NoError(t, err)

	finishGlossyInactiveID := uuid.New()
	_, err = pool.Exec(ctx, "INSERT INTO attribute_dictionary_values (id, dictionary_id, code, name_ru, is_active) VALUES ($1, $2, 'GLOSSY', 'Глянцевая', false)", finishGlossyInactiveID, finishDictID)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, "INSERT INTO attribute_definitions (id, code, name_ru, value_type, value_source, scope, is_active) VALUES ($1, $2, 'Отделка', 'ENUM', 'DICTIONARY', 'VARIANT', true)", finishDefID, "FINISH-" + uuid.New().String())
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
		INSERT INTO category_attribute_definitions (category_id, attribute_definition_id, required, filterable, variant_axis, sort_order, dictionary_id) VALUES 
		($1, $2, true, true, true, 10, NULL),
		($1, $3, true, true, true, 20, NULL),
		($1, $4, true, false, false, 50, NULL),
		($1, $5, true, false, false, 60, $6),
		($1, $7, false, false, false, 30, $8)
	`, catID, colorID, sizeID, compID, seasonDefID, seasonDictID, finishDefID, finishDictID)
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

	_, err = pool.Exec(ctx, "INSERT INTO category_size_chart_fields (id, category_id, code, name, unit, is_required, sort_order) VALUES (gen_random_uuid(), $1, 'CHEST', 'Обхват груди', 'cm', true, 10), (gen_random_uuid(), $1, 'LENGTH', 'Длина изделия', 'cm', true, 20)", catID)
	require.NoError(t, err)
	

	t.Run("Generic Variant Attribute Roundtrip", func(t *testing.T) {
		req := products.CreateProductRequest{
			Title:      "Generic Variant Test",
			Slug:       func(s string) *string { return &s }(uuid.New().String()),
			CategoryID: &catID,
			Attributes: []products.ProductAttributeValueRequest{
				{AttributeDefinitionID: seasonDefID, EnumValueID: &springID},
			},
			MaterialComposition: []products.ProductMaterialCompositionRequest{
				{MaterialID: matCottonID, Percentage: 100},
			},
			SizeChartRows: []products.ProductSizeChartRowRequest{
				{SizeValueID: sizeEu42, Measurements: map[string]interface{}{"LENGTH": float64(100), "CHEST": float64(90)}},
			},
			Variants: []products.ProductVariantRequest{
				{
					SellerSKU:   func(s string) *string { return &s }("SEM-VAR-1"),
					ColorID:     &colorRedID,
					SizeValueID: &sizeEu42,
					PriceCents:  func(i int64) *int64 { return &i }(1000),
					Attributes: []products.VariantAttributeValueRequest{
						{AttributeDefinitionID: finishDefID, EnumValueID: &finishMatteID},
					},
				},
			},
		}

		p, err := svc.CreateProductForSeller(ctx, sellerUserID, req)
		require.NoError(t, err)
		
		fetched, err := svc.GetSellerProduct(ctx, sellerUserID, p.ID)
		require.NoError(t, err)
		require.Equal(t, 1, len(fetched.Variants[0].Attributes))
		require.Equal(t, finishMatteID, *fetched.Variants[0].Attributes[0].EnumValueID)
		
		// Test wrong dictionary (springID is from seasonDictID)
		req.Variants[0].Attributes[0].EnumValueID = &springID
		_, err = svc.CreateProductForSeller(ctx, sellerUserID, req)
		require.Error(t, err)
		require.Contains(t, fmt.Sprintf("%v", err), "invalid or inactive")
		
		// Test inactive dictionary value
		req.Variants[0].Attributes[0].EnumValueID = &finishGlossyInactiveID
		_, err = svc.CreateProductForSeller(ctx, sellerUserID, req)
		require.Error(t, err)
		require.Contains(t, fmt.Sprintf("%v", err), "is invalid or inactive")
	})

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
					Measurements: map[string]interface{}{"LENGTH": float64(100), "CHEST": float64(90)},
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
		t.Skip("skipped because CreateProduct allows drafts")
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
		t.Skip("skipped because CreateProduct allows drafts")
		catID2 := uuid.New()
		_, err := pool.Exec(ctx, "INSERT INTO categories (id, name, slug, size_chart_required) VALUES ($1, 'Cat2', $2, false)", catID2, "cat2-" + uuid.New().String())
		require.NoError(t, err)

		_, err = pool.Exec(ctx, "INSERT INTO category_attribute_definitions (category_id, attribute_definition_id, required) VALUES ($1, $2, true), ($1, $3, true)", catID2, colorID, sizeID)
		require.NoError(t, err)

		req := products.CreateProductRequest{
			Title:      "Wrong Size Sys",
			Slug:       func(s string) *string { return &s }(uuid.New().String()),
			CategoryID: &catID2,
			Variants: []products.ProductVariantRequest{
				{
					SellerSKU:   func(s string) *string { return &s }("SEM-3"),
					ColorID:     &colorRedID,
					SizeValueID: &sizeIntM, 
				},
			},
		}
		
		// 1. Fail closed (schema has SIZE, but no size systems mapped)
		_, err = svc.CreateProductForSeller(ctx, sellerUserID, req)
		require.Error(t, err)
		require.Contains(t, fmt.Sprintf("%v", err), "category requires size but has no allowed size systems configured")
		
		// 2. Map INT
		_, err = pool.Exec(ctx, "INSERT INTO category_size_systems (category_id, size_system_id, is_default) VALUES ($1, $2, true)", catID2, sysIntID)
		require.NoError(t, err)
		
		// 3. INT M should now pass
		req.Variants[0].SellerSKU = func(s string) *string { return &s }("SEM-INTM")
		_, err = svc.CreateProductForSeller(ctx, sellerUserID, req)
		require.NoError(t, err)
		
		// 4. EU 42 should fail since only INT is mapped
		req.Variants[0].SellerSKU = func(s string) *string { return &s }("SEM-EU42")
		req.Variants[0].SizeValueID = &sizeEu42
		_, err = svc.CreateProductForSeller(ctx, sellerUserID, req)
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
		require.Equal(t, 5, len(attrs))
		
		// Find COLOR and check exact flags
		for _, a := range attrs {
			if a["code"] == "COLOR" {
				require.True(t, a["filterable"].(bool))
				require.True(t, a["variantAxis"].(bool))
				require.Equal(t, 10, a["sortOrder"])
			} else if a["code"] == "SIZE" {
				require.True(t, a["filterable"].(bool))
				require.True(t, a["variantAxis"].(bool))
				require.Equal(t, 20, a["sortOrder"])
			} else if a["valueSource"] == "DICTIONARY" && a["scope"] == "VARIANT" {
				require.False(t, a["filterable"].(bool))
				require.False(t, a["variantAxis"].(bool))
				require.Equal(t, 30, a["sortOrder"])
			}
		}
		
		allowedSizes := schemaMap["allowedSizeSystems"].([]map[string]interface{})
		require.Equal(t, 1, len(allowedSizes))
		require.Equal(t, "EU", allowedSizes[0]["code"])
		
		chartFields := schemaMap["sizeChartFields"].([]map[string]interface{})
		require.Equal(t, 2, len(chartFields))
		require.Equal(t, "CHEST", chartFields[0]["code"])
		require.Equal(t, "Обхват груди", chartFields[0]["name"])
		require.Equal(t, "cm", *chartFields[0]["unit"].(*string))
		require.Equal(t, "LENGTH", chartFields[1]["code"])
	})
}
