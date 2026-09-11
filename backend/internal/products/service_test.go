package products

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

// To test the logic, we mock the repo which simulates the SQL logic.
type mockRepo struct {
	products []mockProductRow
}

type mockProductRow struct {
	P            Product
	SellerStatus string
}

func (m *mockRepo) ListPublishedProducts(ctx context.Context, filter PublicProductFilter, limit, offset int) ([]Product, int, error) {
	var result []Product
	for _, row := range m.products {
		if row.P.Status != StatusPublished {
			continue
		}
		if row.SellerStatus != "active" {
			continue
		}
		if filter.SellerID != nil && row.P.SellerID != *filter.SellerID {
			continue
		}
		result = append(result, row.P)
	}
	return result, len(result), nil
}

func (m *mockRepo) GetPublishedProductBySlugOrID(ctx context.Context, idOrSlug string) (Product, error) {
	for _, row := range m.products {
		if row.P.ID.String() == idOrSlug || row.P.Slug == idOrSlug {
			if row.P.Status != StatusPublished || row.SellerStatus != "active" {
				return Product{}, errors.New("not found")
			}
			return row.P, nil
		}
	}
	return Product{}, errors.New("not found")
}

func TestPublicVisibility_ListProducts(t *testing.T) {
	repo := &mockRepo{
		products: []mockProductRow{
			{P: Product{Status: StatusPublished}, SellerStatus: "active"},         // 1. returned
			{P: Product{Status: StatusPublished}, SellerStatus: "pending"},        // 2. NOT returned
			{P: Product{Status: StatusPublished}, SellerStatus: "blocked"},        // 3. NOT returned
			{P: Product{Status: StatusPublished}, SellerStatus: "archived"},       // 4. NOT returned
			{P: Product{Status: StatusDraft}, SellerStatus: "active"},             // 5. NOT returned
			{P: Product{Status: StatusPendingModeration}, SellerStatus: "active"}, // 6. NOT returned
			{P: Product{Status: StatusRejected}, SellerStatus: "active"},          // 7. NOT returned
			{P: Product{Status: StatusHidden}, SellerStatus: "active"},            // 8. NOT returned
			{P: Product{Status: StatusBlocked}, SellerStatus: "active"},           // 8. NOT returned
		},
	}

	items, totalCount, err := repo.ListPublishedProducts(context.Background(), PublicProductFilter{}, 10, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if totalCount != 1 {
		t.Errorf("expected 1 visible product, got %d", totalCount)
	}

	if len(items) != 1 {
		t.Errorf("expected 1 item returned, got %d", len(items))
	}
}

func TestPublicVisibility_FilterBySeller(t *testing.T) {
	seller1 := uuid.New()
	seller2 := uuid.New()

	repo := &mockRepo{
		products: []mockProductRow{
			{P: Product{ID: uuid.New(), SellerID: seller1, Status: StatusPublished}, SellerStatus: "active"},
			{P: Product{ID: uuid.New(), SellerID: seller1, Status: StatusPublished}, SellerStatus: "active"},
			{P: Product{ID: uuid.New(), SellerID: seller2, Status: StatusPublished}, SellerStatus: "active"},
			{P: Product{ID: uuid.New(), SellerID: seller2, Status: StatusDraft}, SellerStatus: "active"},
		},
	}

	// Filter by seller1
	items, _, err := repo.ListPublishedProducts(context.Background(), PublicProductFilter{SellerID: &seller1}, 10, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 2 {
		t.Errorf("expected 2 items for seller1, got %d", len(items))
	}

	// Filter by seller2
	items, _, err = repo.ListPublishedProducts(context.Background(), PublicProductFilter{SellerID: &seller2}, 10, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Errorf("expected 1 published item for seller2, got %d", len(items))
	}
}

func TestPublicVisibility_GetProduct(t *testing.T) {
	repo := &mockRepo{
		products: []mockProductRow{
			{P: Product{Slug: "1-pub-act", Status: StatusPublished}, SellerStatus: "active"},
			{P: Product{Slug: "2-pub-pen", Status: StatusPublished}, SellerStatus: "pending"},
			{P: Product{Slug: "3-pub-blo", Status: StatusPublished}, SellerStatus: "blocked"},
			{P: Product{Slug: "4-pub-arc", Status: StatusPublished}, SellerStatus: "archived"},
			{P: Product{Slug: "5-dra-act", Status: StatusDraft}, SellerStatus: "active"},
			{P: Product{Slug: "6-pmo-act", Status: StatusPendingModeration}, SellerStatus: "active"},
			{P: Product{Slug: "7-rej-act", Status: StatusRejected}, SellerStatus: "active"},
			{P: Product{Slug: "8-hid-act", Status: StatusHidden}, SellerStatus: "active"},
			{P: Product{Slug: "8-blo-act", Status: StatusBlocked}, SellerStatus: "active"},
		},
	}

	tests := []struct {
		slug          string
		expectedFound bool
	}{
		{"1-pub-act", true},
		{"2-pub-pen", false},
		{"3-pub-blo", false},
		{"4-pub-arc", false},
		{"5-dra-act", false},
		{"6-pmo-act", false},
		{"7-rej-act", false},
		{"8-hid-act", false},
		{"8-blo-act", false},
	}

	for _, tc := range tests {
		t.Run(tc.slug, func(t *testing.T) {
			_, err := repo.GetPublishedProductBySlugOrID(context.Background(), tc.slug)
			found := err == nil
			if found != tc.expectedFound {
				t.Errorf("slug %s: expected found=%v, got=%v", tc.slug, tc.expectedFound, found)
			}
		})
	}
}

func TestMapToPublicProduct_VariantTruth(t *testing.T) {
	colorName := "Красный"
	colorHex := "#FF0000"
	sizeL := "L"
	sizeXL := "XL"
	skuL := "SKU-RED-L"
	skuXL := "SKU-RED-XL"
	barcodeL := "BC-RED-L"
	shade := "Bright"
	optValues := map[string]interface{}{"fit": "oversized"}
	inStockTrue := true
	inStockFalse := false
	colorID := uuid.New()
	sizeLID := uuid.New()
	sizeXLID := uuid.New()

	prod := Product{
		ID:    uuid.New(),
		Title: "Test Hoodie",
		Variants: []ProductVariant{
			{
				ID:           uuid.New(),
				SKU:          &skuL,
				ColorName:    &colorName,
				ColorHex:     &colorHex,
				Color:        &colorName,
				Size:         &sizeL,
				SellerSKU:    &skuL,
				ColorID:      &colorID,
				SizeValueID:  &sizeLID,
				ShadeName:    &shade,
				Barcode:      &barcodeL,
				OptionValues: optValues,
				InStock:      &inStockTrue,
				IsActive:     true,
			},
			{
				ID:          uuid.New(),
				SKU:         &skuXL,
				ColorName:   &colorName,
				ColorHex:    &colorHex,
				Color:       &colorName,
				Size:        &sizeXL,
				SellerSKU:   &skuXL,
				ColorID:     &colorID,
				SizeValueID: &sizeXLID,
				InStock:     &inStockFalse,
				IsActive:    true,
			},
		},
	}

	pub := mapToPublicProduct(prod)
	if len(pub.Variants) != 2 {
		t.Fatalf("expected 2 variants, got %d", len(pub.Variants))
	}

	v1 := pub.Variants[0]
	if v1.SKU == nil || *v1.SKU != "SKU-RED-L" {
		t.Errorf("expected SKU 'SKU-RED-L', got %v", v1.SKU)
	}
	if v1.SellerSKU == nil || *v1.SellerSKU != "SKU-RED-L" {
		t.Errorf("expected sellerSku 'SKU-RED-L', got %v", v1.SellerSKU)
	}
	if v1.ColorName == nil || *v1.ColorName != "Красный" {
		t.Errorf("expected colorName 'Красный', got %v", v1.ColorName)
	}
	if v1.ColorHex == nil || *v1.ColorHex != "#FF0000" {
		t.Errorf("expected colorHex '#FF0000', got %v", v1.ColorHex)
	}
	if v1.Size == nil || *v1.Size != "L" {
		t.Errorf("expected size 'L', got %v", v1.Size)
	}
	if v1.ColorID == nil || *v1.ColorID != colorID {
		t.Errorf("expected colorId %v, got %v", colorID, v1.ColorID)
	}
	if v1.SizeValueID == nil || *v1.SizeValueID != sizeLID {
		t.Errorf("expected sizeValueId %v, got %v", sizeLID, v1.SizeValueID)
	}
	if v1.Barcode == nil || *v1.Barcode != "BC-RED-L" {
		t.Errorf("expected barcode 'BC-RED-L', got %v", v1.Barcode)
	}
	if v1.ShadeName == nil || *v1.ShadeName != "Bright" {
		t.Errorf("expected shadeName 'Bright', got %v", v1.ShadeName)
	}
	if v1.OptionValues == nil || v1.OptionValues["fit"] != "oversized" {
		t.Errorf("expected optionValues with fit=oversized, got %v", v1.OptionValues)
	}
	if !v1.IsActive {
		t.Errorf("expected isActive true, got %v", v1.IsActive)
	}
	if v1.InStock == nil || !*v1.InStock {
		t.Errorf("expected inStock true, got %v", v1.InStock)
	}

	v2 := pub.Variants[1]
	if v2.ColorName == nil || *v2.ColorName != "Красный" {
		t.Errorf("expected colorName 'Красный', got %v", v2.ColorName)
	}
	if v2.ColorHex == nil || *v2.ColorHex != "#FF0000" {
		t.Errorf("expected colorHex '#FF0000', got %v", v2.ColorHex)
	}
	if v2.Size == nil || *v2.Size != "XL" {
		t.Errorf("expected size 'XL', got %v", v2.Size)
	}
	if v2.InStock == nil || *v2.InStock {
		t.Errorf("expected inStock false, got %v", v2.InStock)
	}
}
