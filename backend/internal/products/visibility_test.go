package products

import (
	"testing"
)

func TestValidatePublishEligibility(t *testing.T) {
	activeSeller := "active"
	inactiveSeller := "suspended"

	baseValidProduct := func() *Product {
		return &Product{
			Status:              StatusHidden,
			SellerStatus:        &activeSeller,
			SellerIsActive:      true,
			ActiveVariantsCount: 1,
			PriceCents:          5000,
			HasInventoryRecord:  true,
			AvailableStock:      10,
		}
	}

	t.Run("1. hidden + all conditions met => eligible", func(t *testing.T) {
		p := baseValidProduct()
		res := ValidatePublishEligibility(p)
		if !res.IsEligible {
			t.Fatalf("Expected eligible, got ineligible with reasons: %v", res.EligibilityReasons)
		}
		if len(res.EligibilityReasons) != 0 {
			t.Fatalf("Expected 0 reasons, got: %v", res.EligibilityReasons)
		}
	})

	t.Run("2. hidden + seller inactive => seller_inactive", func(t *testing.T) {
		p := baseValidProduct()
		p.SellerStatus = &inactiveSeller
		p.SellerIsActive = false
		res := ValidatePublishEligibility(p)
		if res.IsEligible {
			t.Fatalf("Expected ineligible")
		}
		if len(res.EligibilityReasons) != 1 || res.EligibilityReasons[0] != "seller_inactive" {
			t.Fatalf("Expected [seller_inactive], got: %v", res.EligibilityReasons)
		}
	})

	t.Run("3. hidden + no inventory => no_inventory", func(t *testing.T) {
		p := baseValidProduct()
		p.HasInventoryRecord = false
		p.AvailableStock = 0
		res := ValidatePublishEligibility(p)
		if res.IsEligible {
			t.Fatalf("Expected ineligible")
		}
		if len(res.EligibilityReasons) != 1 || res.EligibilityReasons[0] != "no_inventory" {
			t.Fatalf("Expected [no_inventory], got: %v", res.EligibilityReasons)
		}
	})

	t.Run("4. hidden + zero stock => out_of_stock", func(t *testing.T) {
		p := baseValidProduct()
		p.HasInventoryRecord = true
		p.AvailableStock = 0
		res := ValidatePublishEligibility(p)
		if res.IsEligible {
			t.Fatalf("Expected ineligible")
		}
		if len(res.EligibilityReasons) != 1 || res.EligibilityReasons[0] != "out_of_stock" {
			t.Fatalf("Expected [out_of_stock], got: %v", res.EligibilityReasons)
		}
	})

	t.Run("5. blocked => product_blocked", func(t *testing.T) {
		p := baseValidProduct()
		p.Status = StatusBlocked
		res := ValidatePublishEligibility(p)
		if res.IsEligible {
			t.Fatalf("Expected ineligible")
		}
		if len(res.EligibilityReasons) != 1 || res.EligibilityReasons[0] != "product_blocked" {
			t.Fatalf("Expected [product_blocked], got: %v", res.EligibilityReasons)
		}
	})

	t.Run("6. pending_moderation => moderation_required", func(t *testing.T) {
		p := baseValidProduct()
		p.Status = StatusPendingModeration
		res := ValidatePublishEligibility(p)
		if res.IsEligible {
			t.Fatalf("Expected ineligible")
		}
		if len(res.EligibilityReasons) != 1 || res.EligibilityReasons[0] != "moderation_required" {
			t.Fatalf("Expected [moderation_required], got: %v", res.EligibilityReasons)
		}
	})
}
