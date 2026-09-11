package products

import "fmt"

// MinStorefrontFreeSellableUnits defines the canonical minimum free sellable units across active variants
// required for a product to be publicly visible on the storefront (Shop catalog and PDP).
const MinStorefrontFreeSellableUnits = 2

// CanonicalProductFreeStockSQL returns the authoritative SQL expression that calculates
// the total canonical free sellable units across active variants for a given product ID column/expression.
func CanonicalProductFreeStockSQL(productIDExpr string) string {
	return fmt.Sprintf(`(
		SELECT COALESCE(SUM(GREATEST(0, ii.total_stock - ii.reserved_stock)), 0)
		FROM product_variants pv
		LEFT JOIN inventory_items ii ON pv.id = ii.product_variant_id
		WHERE pv.product_id = %s AND pv.is_active = true
	)`, productIDExpr)
}

// CanonicalVariantFreeStock returns the canonical free sellable quantity for a variant.
func CanonicalVariantFreeStock(totalStock, reservedStock int) int {
	free := totalStock - reservedStock
	if free < 0 {
		return 0
	}
	return free
}

// CanonicalProductActiveFreeStock returns the sum of canonical free sellable units across all active variants.
func CanonicalProductActiveFreeStock(variants []ProductVariant) int {
	total := 0
	for _, v := range variants {
		if v.IsActive && v.HasInventoryRecord {
			total += CanonicalVariantFreeStock(v.TotalStock, v.ReservedStock)
		}
	}
	return total
}

type VisibilityResult struct {
	ActualVisibility  bool     `json:"actualVisibility"`
	VisibilityReasons []string `json:"visibilityReasons"`
}

type PublishEligibilityResult struct {
	IsEligible         bool     `json:"isEligible"`
	EligibilityReasons []string `json:"eligibilityReasons"`
}

// CalculateActualVisibility calculates current visibility on storefront (includes product_hidden if hidden).
func CalculateActualVisibility(p *Product) VisibilityResult {
	reasonsMap := make(map[string]bool)

	// 1. Seller active check
	if p.SellerStatus != nil && *p.SellerStatus != "active" {
		reasonsMap["seller_inactive"] = true
	} else if p.SellerStatus == nil && !p.SellerIsActive {
		reasonsMap["seller_inactive"] = true
	}

	// 2. Product status checks
	if p.Status == StatusHidden {
		reasonsMap["product_hidden"] = true
	} else if p.Status == StatusBlocked {
		reasonsMap["product_blocked"] = true
	} else if p.Status != StatusPublished && p.Status != StatusApproved {
		reasonsMap["moderation_required"] = true
	}

	// 3. Active variants check
	if p.ActiveVariantsCount == 0 {
		reasonsMap["no_active_variants"] = true
	}

	// 4. Price check
	if p.PriceCents <= 0 {
		reasonsMap["invalid_price"] = true
	}

	// 5. Canonical free stock check (>= MinStorefrontFreeSellableUnits active available)
	activeAvailableStock := CanonicalProductActiveFreeStock(p.Variants)
	if len(p.Variants) == 0 && p.ActiveVariantsCount > 0 {
		activeAvailableStock = p.AvailableStock
	}
	if activeAvailableStock < MinStorefrontFreeSellableUnits {
		reasonsMap["low_stock"] = true
	}

	orderedKeys := []string{
		"seller_inactive",
		"product_hidden",
		"product_blocked",
		"moderation_required",
		"no_active_variants",
		"invalid_price",
		"low_stock",
	}

	var reasons []string
	for _, k := range orderedKeys {
		if reasonsMap[k] {
			reasons = append(reasons, k)
		}
	}

	if reasons == nil {
		reasons = []string{}
	}

	actualVisibility := len(reasons) == 0 && (p.Status == StatusPublished || p.Status == StatusApproved)

	return VisibilityResult{
		ActualVisibility:  actualVisibility,
		VisibilityReasons: reasons,
	}
}

// ValidatePublishEligibility evaluates whether a hidden/approved product is eligible for publication to storefront.
// It DOES NOT treat product_hidden as a blocking reason because publication transitions hidden -> published.
func ValidatePublishEligibility(p *Product) PublishEligibilityResult {
	reasonsMap := make(map[string]bool)

	// 1. Status transition check
	if p.Status == StatusBlocked {
		reasonsMap["product_blocked"] = true
	} else if p.Status != StatusApproved && p.Status != StatusHidden && p.Status != StatusPublished {
		reasonsMap["moderation_required"] = true
	}

	// 2. Seller active check
	if p.SellerStatus != nil && *p.SellerStatus != "active" {
		reasonsMap["seller_inactive"] = true
	} else if p.SellerStatus == nil && !p.SellerIsActive {
		reasonsMap["seller_inactive"] = true
	}

	// 3. Active variants check
	if p.ActiveVariantsCount == 0 {
		reasonsMap["no_active_variants"] = true
	}

	// 4. Price check
	if p.PriceCents <= 0 {
		reasonsMap["invalid_price"] = true
	}

	orderedKeys := []string{
		"seller_inactive",
		"product_blocked",
		"moderation_required",
		"no_active_variants",
		"invalid_price",
	}

	var reasons []string
	for _, k := range orderedKeys {
		if reasonsMap[k] {
			reasons = append(reasons, k)
		}
	}

	if reasons == nil {
		reasons = []string{}
	}

	return PublishEligibilityResult{
		IsEligible:         len(reasons) == 0,
		EligibilityReasons: reasons,
	}
}
