package marketing_test

import (
	"testing"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/marketing"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateCampaign_FundingModesAndDiscounts(t *testing.T) {
	sellerID := uuid.New()

	// 1. SELLER-funded campaign: percent or fixed are both allowed
	sellerPct := &marketing.MarketingCampaign{
		Title:             "Seller 10% Off",
		SellerID:          sellerID,
		FundingMode:       marketing.FundingModeSeller,
		Status:            marketing.CampaignStatusDraft,
		DiscountType:      marketing.DiscountTypePercent,
		SellerDiscountBps: 1000,
	}
	assert.NoError(t, marketing.ValidateCampaign(sellerPct))

	sellerFixed := &marketing.MarketingCampaign{
		Title:                    "Seller 500 RUB Off",
		SellerID:                 sellerID,
		FundingMode:              marketing.FundingModeSeller,
		Status:                   marketing.CampaignStatusDraft,
		DiscountType:             marketing.DiscountTypeFixed,
		SellerDiscountFixedCents: 50000,
	}
	assert.NoError(t, marketing.ValidateCampaign(sellerFixed))

	// Seller campaign with zero discount must be rejected
	sellerZero := &marketing.MarketingCampaign{
		Title:        "Seller 0% Off",
		SellerID:     sellerID,
		FundingMode:  marketing.FundingModeSeller,
		Status:       marketing.CampaignStatusDraft,
		DiscountType: marketing.DiscountTypePercent,
	}
	assert.ErrorIs(t, marketing.ValidateCampaign(sellerZero), marketing.ErrSellerRequiresDiscount)

	// 2. SELLER-funded cannot request ZAMK money
	sellerWithZamk := &marketing.MarketingCampaign{
		Title:                 "Illegal Seller with ZAMK",
		SellerID:              sellerID,
		FundingMode:           marketing.FundingModeSeller,
		Status:                marketing.CampaignStatusDraft,
		DiscountType:          marketing.DiscountTypePercent,
		SellerDiscountBps:     1000,
		RequestedZamkShareBps: 500,
	}
	assert.ErrorIs(t, marketing.ValidateCampaign(sellerWithZamk), marketing.ErrSellerFundedCannotHaveZamk)

	// 3. COFUNDED campaign: FIXED discount is rejected in V1
	cofundedFixed := &marketing.MarketingCampaign{
		Title:                       "Illegal Co-funded Fixed",
		SellerID:                    sellerID,
		FundingMode:                 marketing.FundingModeCofunded,
		Status:                      marketing.CampaignStatusDraft,
		DiscountType:                marketing.DiscountTypeFixed,
		SellerDiscountFixedCents:    50000,
		RequestedZamkShareBps:       1000,
		RequestedZamkBudgetCapCents: 1000000,
	}
	assert.ErrorIs(t, marketing.ValidateCampaign(cofundedFixed), marketing.ErrFixedDiscountNotAllowed)

	// 4. COFUNDED campaign: Requires positive seller discount contribution
	cofundedNoSellerContrib := &marketing.MarketingCampaign{
		Title:                       "Illegal Co-funded Zero Seller Contrib",
		SellerID:                    sellerID,
		FundingMode:                 marketing.FundingModeCofunded,
		Status:                      marketing.CampaignStatusDraft,
		DiscountType:                marketing.DiscountTypePercent,
		SellerDiscountBps:           0,
		RequestedZamkShareBps:       1000,
		RequestedZamkBudgetCapCents: 1000000,
	}
	assert.ErrorIs(t, marketing.ValidateCampaign(cofundedNoSellerContrib), marketing.ErrCofundedRequiresSellerContribution)

	// 5. COFUNDED campaign: Percent discount valid
	cofundedPct := &marketing.MarketingCampaign{
		Title:                       "Valid Co-funded 15+10%",
		SellerID:                    sellerID,
		FundingMode:                 marketing.FundingModeCofunded,
		Status:                      marketing.CampaignStatusDraft,
		DiscountType:                marketing.DiscountTypePercent,
		SellerDiscountBps:           1500,
		RequestedZamkShareBps:       1000,
		RequestedZamkBudgetCapCents: 5000000,
	}
	assert.NoError(t, marketing.ValidateCampaign(cofundedPct))

	// 6. Pure ZAMK-funded: cannot have seller discount
	zamkWithSellerContrib := &marketing.MarketingCampaign{
		Title:                       "Illegal Pure ZAMK with Seller Discount",
		SellerID:                    sellerID,
		FundingMode:                 marketing.FundingModeZamk,
		Status:                      marketing.CampaignStatusDraft,
		DiscountType:                marketing.DiscountTypePercent,
		SellerDiscountBps:           500,
		RequestedZamkShareBps:       1000,
		RequestedZamkBudgetCapCents: 5000000,
	}
	assert.ErrorIs(t, marketing.ValidateCampaign(zamkWithSellerContrib), marketing.ErrZamkCannotHaveSellerDiscount)

	// 7. ZAMK share ceiling: > 2500 bps is rejected
	cofundedExceeds := &marketing.MarketingCampaign{
		Title:                       "Exceeds 25%",
		SellerID:                    sellerID,
		FundingMode:                 marketing.FundingModeCofunded,
		Status:                      marketing.CampaignStatusDraft,
		DiscountType:                marketing.DiscountTypePercent,
		SellerDiscountBps:           1000,
		RequestedZamkShareBps:       2501,
		RequestedZamkBudgetCapCents: 5000000,
	}
	assert.ErrorIs(t, marketing.ValidateCampaign(cofundedExceeds), marketing.ErrZamkShareExceedsCeiling)

	// 8. Submitted platform campaign must request positive terms
	submittedZeroReq := &marketing.MarketingCampaign{
		Title:                       "Submitted with 0 request",
		SellerID:                    sellerID,
		FundingMode:                 marketing.FundingModeCofunded,
		Status:                      marketing.CampaignStatusSubmitted,
		DiscountType:                marketing.DiscountTypePercent,
		SellerDiscountBps:           1000,
		RequestedZamkShareBps:       0,
		RequestedZamkBudgetCapCents: 0,
	}
	assert.ErrorIs(t, marketing.ValidateCampaign(submittedZeroReq), marketing.ErrSubmittedPlatformRequiresRequested)

	// 9. Unreviewed platform campaign cannot have approved values
	draftWithApproved := &marketing.MarketingCampaign{
		Title:                       "Draft with approved terms",
		SellerID:                    sellerID,
		FundingMode:                 marketing.FundingModeCofunded,
		Status:                      marketing.CampaignStatusDraft,
		DiscountType:                marketing.DiscountTypePercent,
		SellerDiscountBps:           1000,
		RequestedZamkShareBps:       1000,
		RequestedZamkBudgetCapCents: 1000000,
		ApprovedZamkShareBps:        1000,
		ApprovedZamkBudgetCapCents:  1000000,
	}
	assert.ErrorIs(t, marketing.ValidateCampaign(draftWithApproved), marketing.ErrUnreviewedMustHaveZeroApproved)
}

func TestEvaluateAdminDecision_CounterOfferLogic(t *testing.T) {
	sellerID := uuid.New()
	requested := &marketing.MarketingCampaign{
		ID:                          uuid.New(),
		SellerID:                    sellerID,
		FundingMode:                 marketing.FundingModeCofunded,
		Status:                      marketing.CampaignStatusSubmitted,
		RequestedZamkShareBps:       1500,    // 15% requested
		RequestedZamkBudgetCapCents: 5000000, // 50,000 RUB requested
	}

	// Case 1: Exact approval
	isCounter, err := marketing.EvaluateAdminDecision(requested, 1500, 5000000)
	require.NoError(t, err)
	assert.False(t, isCounter, "Exact terms match must be direct approval")

	// Case 2: Reduced share (10% instead of 15%) -> Counter-Offer
	isCounter, err = marketing.EvaluateAdminDecision(requested, 1000, 5000000)
	require.NoError(t, err)
	assert.True(t, isCounter, "Reduced share must trigger counter_offered status")

	// Case 3: Reduced budget (30,000 instead of 50,000) -> Counter-Offer
	isCounter, err = marketing.EvaluateAdminDecision(requested, 1500, 3000000)
	require.NoError(t, err)
	assert.True(t, isCounter, "Reduced budget must trigger counter_offered status")

	// Case 4: Admin attempts to approve MORE than requested -> Rejected by authority rule
	_, err = marketing.EvaluateAdminDecision(requested, 2000, 5000000)
	assert.ErrorIs(t, err, marketing.ErrApprovedExceedsRequested)

	_, err = marketing.EvaluateAdminDecision(requested, 1500, 6000000)
	assert.ErrorIs(t, err, marketing.ErrApprovedExceedsRequested)
}

func TestValidateOrderItemPromotion_Invariants(t *testing.T) {
	// Canonical 5000 RUB item
	valid := &marketing.OrderItemPromotion{
		ID:                          uuid.New(),
		OrderItemID:                 uuid.New(),
		OrderID:                     uuid.New(),
		SellerID:                    uuid.New(),
		CampaignID:                  uuid.New(),
		PromoCodeID:                 uuid.New(),
		BaseUnitPriceCents:          500000, // 5000 RUB
		SellerDiscountUnitCents:     75000,  // 750 RUB
		ZamkSubsidyUnitCents:        50000,  // 500 RUB
		CustomerPaidUnitPriceCents:  375000, // 3750 RUB (5000 - 750 - 500)
		CommissionBaseUnitCents:     425000, // 4250 RUB (5000 - 750)
		Quantity:                    2,
		TotalSellerDiscountCents:    150000, // 750 * 2 = 1500 RUB
		TotalZamkSubsidyCents:       100000, // 500 * 2 = 1000 RUB
		TotalCustomerPaidCents:      750000, // 3750 * 2 = 7500 RUB
		TotalCommissionBaseCents:    850000, // 4250 * 2 = 8500 RUB
		CommissionRateBps:           900,    // 9%
		TotalCommissionChargedCents: 76500,  // 850000 * 0.09 = 765 RUB
	}
	assert.NoError(t, marketing.ValidateOrderItemPromotion(valid))

	// Tampered customer unit price
	tamperedCustPrice := *valid
	tamperedCustPrice.CustomerPaidUnitPriceCents = 400000
	assert.Error(t, marketing.ValidateOrderItemPromotion(&tamperedCustPrice))

	// Tampered commission base (reducing it by ZAMK subsidy is illegal)
	tamperedCommBase := *valid
	tamperedCommBase.TotalCommissionBaseCents = 750000 // Illegally reduced by ZAMK subsidy
	assert.Error(t, marketing.ValidateOrderItemPromotion(&tamperedCommBase))

	// Tampered unit commission base
	tamperedCommUnit := *valid
	tamperedCommUnit.CommissionBaseUnitCents = 400000
	assert.Error(t, marketing.ValidateOrderItemPromotion(&tamperedCommUnit))
}

func TestValidatePromoCode_MinEligibleQuantity(t *testing.T) {
	zero := 0
	neg := -1
	pos := 3

	basePromo := func() *marketing.PromoCode {
		return &marketing.PromoCode{
			Code:                  "VALID10",
			DiscountType:          marketing.DiscountTypePercent,
			DiscountValueBps:      1000,
			PerCustomerUsageLimit: 1,
		}
	}

	// 1. NULL is valid
	pNull := basePromo()
	pNull.MinEligibleQuantity = nil
	assert.NoError(t, marketing.ValidatePromoCode(pNull))

	// 2. Positive integer is valid
	pPos := basePromo()
	pPos.MinEligibleQuantity = &pos
	assert.NoError(t, marketing.ValidatePromoCode(pPos))

	// 3. Zero is invalid
	pZero := basePromo()
	pZero.MinEligibleQuantity = &zero
	assert.ErrorIs(t, marketing.ValidatePromoCode(pZero), marketing.ErrInvalidMinQuantity)

	// 4. Negative is invalid
	pNeg := basePromo()
	pNeg.MinEligibleQuantity = &neg
	assert.ErrorIs(t, marketing.ValidatePromoCode(pNeg), marketing.ErrInvalidMinQuantity)
}
