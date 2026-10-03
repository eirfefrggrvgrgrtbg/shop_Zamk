package marketing

import (
	"fmt"
	"strings"
)

// ValidateCampaign checks all domain invariants for a marketing campaign.
func ValidateCampaign(c *MarketingCampaign) error {
	if strings.TrimSpace(c.Title) == "" {
		return fmt.Errorf("campaign title cannot be empty")
	}

	switch c.FundingMode {
	case FundingModeSeller, FundingModeZamk, FundingModeCofunded:
		// valid
	default:
		return ErrInvalidFundingMode
	}

	switch c.Status {
	case CampaignStatusDraft, CampaignStatusSubmitted, CampaignStatusCounterOffered,
		CampaignStatusApproved, CampaignStatusActive, CampaignStatusEnded,
		CampaignStatusRejected, CampaignStatusCancelled:
		// valid
	default:
		return ErrInvalidCampaignStatus
	}

	switch c.DiscountType {
	case DiscountTypePercent, DiscountTypeFixed:
		// valid
	default:
		return ErrInvalidDiscountType
	}

	// Invariant: ZAMK and COFUNDED promotions only support percentage discounts in V1
	if (c.FundingMode == FundingModeZamk || c.FundingMode == FundingModeCofunded) && c.DiscountType == DiscountTypeFixed {
		return ErrFixedDiscountNotAllowed
	}

	// Invariant: ZAMK share ceiling is 25% (2500 bps)
	if c.RequestedZamkShareBps > 2500 || c.ApprovedZamkShareBps > 2500 {
		return ErrZamkShareExceedsCeiling
	}
	if c.RequestedZamkShareBps < 0 || c.ApprovedZamkShareBps < 0 {
		return fmt.Errorf("ZAMK share bps cannot be negative")
	}

	// Invariant: SELLER-funded campaign cannot have ZAMK subsidy requested or approved, nor reserved/spent cents
	if c.FundingMode == FundingModeSeller {
		if c.RequestedZamkShareBps != 0 || c.RequestedZamkBudgetCapCents != 0 ||
			c.ApprovedZamkShareBps != 0 || c.ApprovedZamkBudgetCapCents != 0 ||
			c.ZamkReservedCents != 0 || c.ZamkSpentCents != 0 {
			return ErrSellerFundedCannotHaveZamk
		}
		if c.SellerDiscountBps <= 0 && c.SellerDiscountFixedCents <= 0 {
			return ErrSellerRequiresDiscount
		}
	}

	// Invariant: Pure ZAMK-funded campaign cannot have seller discount
	if c.FundingMode == FundingModeZamk {
		if c.SellerDiscountBps != 0 || c.SellerDiscountFixedCents != 0 {
			return ErrZamkCannotHaveSellerDiscount
		}
	}

	// Invariant: COFUNDED campaign requires positive seller discount percentage
	if c.FundingMode == FundingModeCofunded {
		if c.SellerDiscountBps <= 0 {
			return ErrCofundedRequiresSellerContribution
		}
	}

	// Invariant: Platform-funded campaigns: approved cannot exceed requested
	if c.FundingMode != FundingModeSeller {
		if c.ApprovedZamkShareBps > c.RequestedZamkShareBps {
			return ErrApprovedExceedsRequested
		}
		if c.ApprovedZamkBudgetCapCents > c.RequestedZamkBudgetCapCents {
			return ErrApprovedExceedsRequested
		}

		// Invariant: Unreviewed platform campaigns (draft, submitted) must have zero approved terms
		if c.Status == CampaignStatusDraft || c.Status == CampaignStatusSubmitted {
			if c.ApprovedZamkShareBps != 0 || c.ApprovedZamkBudgetCapCents != 0 {
				return ErrUnreviewedMustHaveZeroApproved
			}
		}

		// Invariant: Submitted platform campaigns must have positive requested terms
		if c.Status != CampaignStatusDraft {
			if c.RequestedZamkShareBps <= 0 || c.RequestedZamkBudgetCapCents <= 0 {
				return ErrSubmittedPlatformRequiresRequested
			}
		}
	}

	// Invariant: Approved/active platform-funded campaigns must have positive approved terms
	if (c.Status == CampaignStatusApproved || c.Status == CampaignStatusActive || c.Status == CampaignStatusEnded) &&
		c.FundingMode != FundingModeSeller {
		if c.ApprovedZamkShareBps <= 0 || c.ApprovedZamkBudgetCapCents <= 0 {
			return ErrInvalidBudgetCap
		}
	}

	// Invariant: Budget spend must not exceed approved cap
	if c.ApprovedZamkBudgetCapCents > 0 {
		if c.ZamkSpentCents > c.ApprovedZamkBudgetCapCents ||
			(c.ZamkSpentCents+c.ZamkReservedCents) > c.ApprovedZamkBudgetCapCents {
			return fmt.Errorf("ZAMK spend/reserved exceeds approved budget cap")
		}
	}

	if c.SellerDiscountBps < 0 || c.SellerDiscountBps > 10000 {
		return fmt.Errorf("seller discount bps out of range [0, 10000]")
	}
	if c.SellerDiscountFixedCents < 0 {
		return fmt.Errorf("seller discount fixed cents cannot be negative")
	}

	return nil
}

// EvaluateAdminDecision determines whether an Admin's approved terms match requested terms (Approved)
// or represent a counter-offer with reduced funding (CounterOffered).
func EvaluateAdminDecision(requested *MarketingCampaign, approvedShareBps int, approvedBudgetCapCents int64) (isCounterOffer bool, err error) {
	if approvedShareBps > requested.RequestedZamkShareBps {
		return false, fmt.Errorf("%w: approved share %d bps exceeds requested %d bps", ErrApprovedExceedsRequested, approvedShareBps, requested.RequestedZamkShareBps)
	}
	if approvedBudgetCapCents > requested.RequestedZamkBudgetCapCents {
		return false, fmt.Errorf("%w: approved budget %d exceeds requested %d", ErrApprovedExceedsRequested, approvedBudgetCapCents, requested.RequestedZamkBudgetCapCents)
	}
	if approvedShareBps <= 0 || approvedBudgetCapCents <= 0 {
		return false, ErrInvalidBudgetCap
	}

	// If exactly equal to requested terms -> Direct Approval
	if approvedShareBps == requested.RequestedZamkShareBps && approvedBudgetCapCents == requested.RequestedZamkBudgetCapCents {
		return false, nil
	}

	// If either share or budget was reduced by Admin -> Counter-Offer
	return true, nil
}

// ValidatePromoCode verifies promo code structural and uniqueness rules.
func ValidatePromoCode(p *PromoCode) error {
	normalized := strings.TrimSpace(p.Code)
	if normalized == "" {
		return fmt.Errorf("promo code cannot be blank")
	}

	switch p.DiscountType {
	case DiscountTypePercent, DiscountTypeFixed:
		// valid
	default:
		return ErrInvalidDiscountType
	}

	if p.DiscountValueBps < 0 || p.DiscountValueBps > 10000 {
		return fmt.Errorf("discount value bps out of range [0, 10000]")
	}
	if p.DiscountValueFixedCents < 0 {
		return fmt.Errorf("discount value fixed cents cannot be negative")
	}
	if p.MinOrderSubtotalCents < 0 {
		return fmt.Errorf("min order subtotal cannot be negative")
	}
	if p.PerCustomerUsageLimit <= 0 {
		return fmt.Errorf("per-customer usage limit must be at least 1")
	}
	if p.GlobalUsageLimit != nil && *p.GlobalUsageLimit <= 0 {
		return fmt.Errorf("global usage limit must be positive if specified")
	}
	if p.ProductScope != "" && p.ProductScope != ProductScopeEntireStore && p.ProductScope != ProductScopeSelectedProducts && p.ProductScope != ProductScopeSelectedCategories {
		return ErrInvalidProductScope
	}
	if p.MaxDiscountCents != nil && *p.MaxDiscountCents <= 0 {
		return ErrInvalidMaxDiscount
	}
	if p.MinEligibleQuantity != nil && *p.MinEligibleQuantity <= 0 {
		return ErrInvalidMinQuantity
	}
	if p.MinDistinctProducts != nil && *p.MinDistinctProducts <= 0 {
		return ErrInvalidMinDistinctProducts
	}
	if p.AudienceType == "" {
		p.AudienceType = AudienceAllCustomers
	}
	if !IsValidAudienceType(p.AudienceType) {
		return ErrInvalidAudienceType
	}

	return nil
}

// ValidateOrderItemPromotion checks mathematical and structural invariants for promotional order items.
func ValidateOrderItemPromotion(p *OrderItemPromotion) error {
	if p.BaseUnitPriceCents < 0 {
		return fmt.Errorf("%w: base unit price cannot be negative", ErrInvalidOrderItemPromotion)
	}
	if p.SellerDiscountUnitCents < 0 || p.ZamkSubsidyUnitCents < 0 {
		return fmt.Errorf("%w: discount components cannot be negative", ErrInvalidOrderItemPromotion)
	}
	if p.Quantity <= 0 {
		return fmt.Errorf("%w: quantity must be positive", ErrInvalidOrderItemPromotion)
	}

	expectedCustUnitPrice := p.BaseUnitPriceCents - p.SellerDiscountUnitCents - p.ZamkSubsidyUnitCents
	if expectedCustUnitPrice < 0 {
		return fmt.Errorf("%w: customer unit price is negative", ErrCustomerPriceNegative)
	}
	if p.CustomerPaidUnitPriceCents != expectedCustUnitPrice {
		return fmt.Errorf("%w: customer paid unit price %d does not match base (%d) - seller (%d) - zamk (%d) = %d",
			ErrInvalidOrderItemPromotion, p.CustomerPaidUnitPriceCents, p.BaseUnitPriceCents, p.SellerDiscountUnitCents, p.ZamkSubsidyUnitCents, expectedCustUnitPrice)
	}

	expectedCommBaseUnit := p.BaseUnitPriceCents - p.SellerDiscountUnitCents
	if expectedCommBaseUnit < 0 {
		expectedCommBaseUnit = 0
	}
	if p.CommissionBaseUnitCents != expectedCommBaseUnit {
		return fmt.Errorf("%w: commission base unit %d does not match expected %d", ErrInvalidOrderItemPromotion, p.CommissionBaseUnitCents, expectedCommBaseUnit)
	}

	expectedTotalSellerDiscount := p.SellerDiscountUnitCents * int64(p.Quantity)
	if p.TotalSellerDiscountCents != expectedTotalSellerDiscount {
		return fmt.Errorf("%w: total seller discount mismatch: got %d, expected %d", ErrInvalidOrderItemPromotion, p.TotalSellerDiscountCents, expectedTotalSellerDiscount)
	}

	expectedTotalZamkSubsidy := p.ZamkSubsidyUnitCents * int64(p.Quantity)
	if p.TotalZamkSubsidyCents != expectedTotalZamkSubsidy {
		return fmt.Errorf("%w: total zamk subsidy mismatch: got %d, expected %d", ErrInvalidOrderItemPromotion, p.TotalZamkSubsidyCents, expectedTotalZamkSubsidy)
	}

	expectedTotalCustomerPaid := p.CustomerPaidUnitPriceCents * int64(p.Quantity)
	if p.TotalCustomerPaidCents != expectedTotalCustomerPaid {
		return fmt.Errorf("%w: total customer paid mismatch: got %d, expected %d", ErrInvalidOrderItemPromotion, p.TotalCustomerPaidCents, expectedTotalCustomerPaid)
	}

	expectedTotalCommBase := expectedCommBaseUnit * int64(p.Quantity)
	if p.TotalCommissionBaseCents != expectedTotalCommBase {
		return fmt.Errorf("%w: total commission base %d does not match expected %d", ErrInvalidOrderItemPromotion, p.TotalCommissionBaseCents, expectedTotalCommBase)
	}

	if p.CommissionRateBps < 0 || p.CommissionRateBps > 10000 {
		return fmt.Errorf("%w: commission rate bps out of range [0, 10000]", ErrInvalidOrderItemPromotion)
	}
	if p.TotalCommissionChargedCents < 0 {
		return fmt.Errorf("%w: total commission charged cannot be negative", ErrInvalidOrderItemPromotion)
	}

	return nil
}
