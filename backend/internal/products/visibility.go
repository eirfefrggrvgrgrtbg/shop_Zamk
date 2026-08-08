package products

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

	// 5. Inventory check
	if !p.HasInventoryRecord {
		reasonsMap["no_inventory"] = true
	} else if p.AvailableStock <= 0 {
		reasonsMap["out_of_stock"] = true
	}

	orderedKeys := []string{
		"seller_inactive",
		"product_hidden",
		"product_blocked",
		"moderation_required",
		"no_active_variants",
		"invalid_price",
		"no_inventory",
		"out_of_stock",
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

	// 5. Inventory check
	if !p.HasInventoryRecord {
		reasonsMap["no_inventory"] = true
	} else if p.AvailableStock <= 0 {
		reasonsMap["out_of_stock"] = true
	}

	orderedKeys := []string{
		"seller_inactive",
		"product_blocked",
		"moderation_required",
		"no_active_variants",
		"invalid_price",
		"no_inventory",
		"out_of_stock",
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
