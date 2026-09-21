package products_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/products"
)

func TestCreateProductRequest_FieldByFieldHashMutation(t *testing.T) {
	catID := uuid.New()
	brandID := uuid.New()
	colorID := uuid.New()
	sizeValueID := uuid.New()
	attrDefID := uuid.New()
	materialID := uuid.New()

	sku := "SKU-BASE"
	slug := "slug-base"
	desc := "desc-base"
	gender := "unisex"
	color := "black"
	matText := "cotton"
	care := "hand wash"
	shade := "matte black"
	textVal := "val-base"
	oldPrice := int64(2000)

	baseReq := func() products.CreateProductRequest {
		return products.CreateProductRequest{
			Title:            "Base Title",
			Slug:             &slug,
			Description:      &desc,
			CategoryID:       &catID,
			BrandID:          &brandID,
			Gender:           &gender,
			Color:            &color,
			Material:         &matText,
			CareInstructions: &care,
			PriceCents:       1500,
			OldPriceCents:    &oldPrice,
			Currency:         "RUB",
			Variants: []products.ProductVariantRequest{
				{
					SellerSKU:   &sku,
					ColorID:     &colorID,
					SizeValueID: &sizeValueID,
					ShadeName:   &shade,
					PriceCents:  func() *int64 { v := int64(1500); return &v }(),
					Attributes: []products.VariantAttributeValueRequest{
						{AttributeDefinitionID: attrDefID, TextValue: &textVal},
					},
				},
			},
			Attributes: []products.ProductAttributeValueRequest{
				{AttributeDefinitionID: attrDefID, TextValue: &textVal},
			},
			MaterialComposition: []products.ProductMaterialCompositionRequest{
				{MaterialID: materialID, Percentage: 100},
			},
			SizeChartRows: []products.ProductSizeChartRowRequest{
				{SizeValueID: sizeValueID, Measurements: map[string]interface{}{"chest": 100}},
			},
			MainImageURL:    func() *string { s := "http://image1.jpg"; return &s }(),
			Images: []products.ProductImageRequest{
				{ImageURL: "http://image1.jpg"},
			},
		}
	}

	baseReqInst := baseReq()
	baseHash := baseReqInst.NormalizedHash()

	tests := []struct {
		field      string
		mutate     func(r *products.CreateProductRequest)
		shouldDiff bool
	}{
		{
			field: "Title",
			mutate: func(r *products.CreateProductRequest) {
				r.Title = "Mutated Title"
			},
			shouldDiff: true,
		},
		{
			field: "Slug",
			mutate: func(r *products.CreateProductRequest) {
				mutSlug := "mutated-slug"
				r.Slug = &mutSlug
			},
			shouldDiff: true,
		},
		{
			field: "Description",
			mutate: func(r *products.CreateProductRequest) {
				mutDesc := "mutated description"
				r.Description = &mutDesc
			},
			shouldDiff: true,
		},
		{
			field: "CategoryID",
			mutate: func(r *products.CreateProductRequest) {
				newCat := uuid.New()
				r.CategoryID = &newCat
			},
			shouldDiff: true,
		},
		{
			field: "BrandID",
			mutate: func(r *products.CreateProductRequest) {
				newBrand := uuid.New()
				r.BrandID = &newBrand
			},
			shouldDiff: true,
		},
		{
			field: "Gender",
			mutate: func(r *products.CreateProductRequest) {
				g := "female"
				r.Gender = &g
			},
			shouldDiff: true,
		},
		{
			field: "Color",
			mutate: func(r *products.CreateProductRequest) {
				c := "white"
				r.Color = &c
			},
			shouldDiff: true,
		},
		{
			field: "Material",
			mutate: func(r *products.CreateProductRequest) {
				m := "wool"
				r.Material = &m
			},
			shouldDiff: true,
		},
		{
			field: "CareInstructions",
			mutate: func(r *products.CreateProductRequest) {
				ci := "dry clean only"
				r.CareInstructions = &ci
			},
			shouldDiff: true,
		},
		{
			field: "PriceCents",
			mutate: func(r *products.CreateProductRequest) {
				r.PriceCents = 9999
			},
			shouldDiff: true,
		},
		{
			field: "OldPriceCents",
			mutate: func(r *products.CreateProductRequest) {
				v := int64(8888)
				r.OldPriceCents = &v
			},
			shouldDiff: true,
		},
		{
			field: "Currency",
			mutate: func(r *products.CreateProductRequest) {
				r.Currency = "USD"
			},
			shouldDiff: true,
		},
		{
			field: "MainImageURL",
			mutate: func(r *products.CreateProductRequest) {
				m := "http://mutated-image.jpg"
				r.MainImageURL = &m
			},
			shouldDiff: true,
		},
		{
			field: "Variant SellerSKU",
			mutate: func(r *products.CreateProductRequest) {
				s := "MUTATED-SKU"
				r.Variants[0].SellerSKU = &s
			},
			shouldDiff: true,
		},
		{
			field: "Variant SKU",
			mutate: func(r *products.CreateProductRequest) {
				s := "MUTATED-INTERNAL-SKU"
				r.Variants[0].SKU = &s
			},
			shouldDiff: true,
		},
		{
			field: "Variant Size",
			mutate: func(r *products.CreateProductRequest) {
				sz := "XL"
				r.Variants[0].Size = &sz
			},
			shouldDiff: true,
		},
		{
			field: "Variant Color",
			mutate: func(r *products.CreateProductRequest) {
				cl := "Deep Blue"
				r.Variants[0].Color = &cl
			},
			shouldDiff: true,
		},
		{
			field: "Variant ColorID",
			mutate: func(r *products.CreateProductRequest) {
				cid := uuid.New()
				r.Variants[0].ColorID = &cid
			},
			shouldDiff: true,
		},
		{
			field: "Variant SizeValueID",
			mutate: func(r *products.CreateProductRequest) {
				sid := uuid.New()
				r.Variants[0].SizeValueID = &sid
			},
			shouldDiff: true,
		},
		{
			field: "Variant ShadeName",
			mutate: func(r *products.CreateProductRequest) {
				sh := "glossy black"
				r.Variants[0].ShadeName = &sh
			},
			shouldDiff: true,
		},
		{
			field: "Variant OptionValues",
			mutate: func(r *products.CreateProductRequest) {
				r.Variants[0].OptionValues = map[string]interface{}{"fit": "oversized"}
			},
			shouldDiff: true,
		},
		{
			field: "Variant PriceCents",
			mutate: func(r *products.CreateProductRequest) {
				vp := int64(7777)
				r.Variants[0].PriceCents = &vp
			},
			shouldDiff: true,
		},
		{
			field: "Variant attributes",
			mutate: func(r *products.CreateProductRequest) {
				newText := "mutated-variant-attr"
				r.Variants[0].Attributes[0].TextValue = &newText
			},
			shouldDiff: true,
		},
		{
			field: "Product attributes",
			mutate: func(r *products.CreateProductRequest) {
				newText := "mutated-product-attr"
				r.Attributes[0].TextValue = &newText
			},
			shouldDiff: true,
		},
		{
			field: "MaterialComposition",
			mutate: func(r *products.CreateProductRequest) {
				r.MaterialComposition[0].Percentage = 50
			},
			shouldDiff: true,
		},
		{
			field: "SizeChartRows",
			mutate: func(r *products.CreateProductRequest) {
				r.SizeChartRows[0].Measurements["chest"] = 120
			},
			shouldDiff: true,
		},
		{
			field: "Images (Included in normalized hash under Option A)",
			mutate: func(r *products.CreateProductRequest) {
				r.Images = append(r.Images, products.ProductImageRequest{
					ImageURL: "http://another-image.jpg",
				})
			},
			shouldDiff: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.field, func(t *testing.T) {
			req := baseReq()
			tc.mutate(&req)
			mutHash := req.NormalizedHash()
			if tc.shouldDiff {
				require.NotEqual(t, baseHash, mutHash, "mutating field %s MUST produce a different hash", tc.field)
			} else {
				require.Equal(t, baseHash, mutHash, "mutating field %s MUST NOT affect the hash", tc.field)
			}
		})
	}
}

func TestCreateProductRequest_HashNormalizationEquivalence(t *testing.T) {
	catID := uuid.New()
	brandID := uuid.New()
	sku1 := "SKU-1"
	sku1Dup := "SKU-1"
	desc := "desc"
	descDup := "desc"
	colorID := uuid.New()
	attrDef1 := uuid.New()
	attrDef2 := uuid.New()
	mat1 := uuid.New()
	mat2 := uuid.New()
	size1 := uuid.New()
	size2 := uuid.New()

	val1 := "cotton"
	val1Dup := "cotton"
	val2 := "silk"

	baseReq := func() products.CreateProductRequest {
		return products.CreateProductRequest{
			Title:       "Test",
			Description: &desc,
			CategoryID:  &catID,
			BrandID:     &brandID,
			PriceCents:  1000,
			Currency:    "RUB",
			Variants: []products.ProductVariantRequest{
				{
					SellerSKU: &sku1,
					ColorID:   &colorID,
					Attributes: []products.VariantAttributeValueRequest{
						{AttributeDefinitionID: attrDef1, TextValue: &val1},
					},
				},
			},
			Attributes: []products.ProductAttributeValueRequest{
				{AttributeDefinitionID: attrDef1, TextValue: &val1},
				{AttributeDefinitionID: attrDef2, TextValue: &val2},
			},
			MaterialComposition: []products.ProductMaterialCompositionRequest{
				{MaterialID: mat1, Percentage: 60},
				{MaterialID: mat2, Percentage: 40},
			},
			SizeChartRows: []products.ProductSizeChartRowRequest{
				{SizeValueID: size1, Measurements: map[string]interface{}{"chest": 100}},
				{SizeValueID: size2, Measurements: map[string]interface{}{"chest": 104}},
			},
			Images: []products.ProductImageRequest{
				{ImageURL: "http://image1"},
			},
		}
	}

	t.Run("repeated same request => same hash", func(t *testing.T) {
		req1 := baseReq()
		req2 := baseReq()
		require.Equal(t, req1.NormalizedHash(), req2.NormalizedHash())
	})

	t.Run("independently allocated pointer values => same hash", func(t *testing.T) {
		req1 := baseReq()
		req2 := baseReq()
		req2.Description = &descDup
		req2.Variants[0].SellerSKU = &sku1Dup
		req2.Variants[0].Attributes[0].TextValue = &val1Dup
		req2.Attributes[0].TextValue = &val1Dup
		require.Equal(t, req1.NormalizedHash(), req2.NormalizedHash())
	})

	t.Run("description nil vs empty string => distinct hash (persists NULL vs '')", func(t *testing.T) {
		req1 := baseReq()
		req1.Description = nil

		req2 := baseReq()
		emptyStr := ""
		req2.Description = &emptyStr

		require.NotEqual(t, req1.NormalizedHash(), req2.NormalizedHash(), "nil description (NULL) and empty description ('') must have distinct hashes")
	})

	t.Run("slug nil vs empty string => same hash (both resolve to generateSlug(Title))", func(t *testing.T) {
		req1 := baseReq()
		req1.Slug = nil

		req2 := baseReq()
		emptyStr := ""
		req2.Slug = &emptyStr

		require.Equal(t, req1.NormalizedHash(), req2.NormalizedHash(), "nil slug and empty slug both resolve to generateSlug(Title) in persistence")
	})

	t.Run("empty currency defaults to RUB equivalent in persistence", func(t *testing.T) {
		req1 := baseReq()
		req1.Currency = ""

		req2 := baseReq()
		req2.Currency = "RUB"

		require.Equal(t, req1.NormalizedHash(), req2.NormalizedHash())
	})

	t.Run("non-semantic input ordering => same hash", func(t *testing.T) {
		sku2 := "SKU-2"
		req1 := baseReq()
		req1.Variants = append(req1.Variants, products.ProductVariantRequest{SellerSKU: &sku2})

		req2 := baseReq()
		// Swap variants order
		req2.Variants = []products.ProductVariantRequest{
			{SellerSKU: &sku2},
			{
				SellerSKU: &sku1,
				ColorID:   &colorID,
				Attributes: []products.VariantAttributeValueRequest{
					{AttributeDefinitionID: attrDef1, TextValue: &val1},
				},
			},
		}
		// Swap attributes order
		req2.Attributes = []products.ProductAttributeValueRequest{
			{AttributeDefinitionID: attrDef2, TextValue: &val2},
			{AttributeDefinitionID: attrDef1, TextValue: &val1},
		}
		// Swap materials order
		req2.MaterialComposition = []products.ProductMaterialCompositionRequest{
			{MaterialID: mat2, Percentage: 40},
			{MaterialID: mat1, Percentage: 60},
		}
		// Swap size chart order
		req2.SizeChartRows = []products.ProductSizeChartRowRequest{
			{SizeValueID: size2, Measurements: map[string]interface{}{"chest": 104}},
			{SizeValueID: size1, Measurements: map[string]interface{}{"chest": 100}},
		}

		require.Equal(t, req1.NormalizedHash(), req2.NormalizedHash())
	})
}
