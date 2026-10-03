package marketing

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

const RequiredSalesEligibilityThreshold = 35

type Service struct {
	repo *Repository
	pool *pgxpool.Pool
}

func NewService(repo *Repository, pool *pgxpool.Pool) *Service {
	return &Service{
		repo: repo,
		pool: pool,
	}
}

// GetSellerEligibility evaluates seller eligibility for co-funding (35 historical paid sales) and open campaign status.
func (s *Service) GetSellerEligibility(ctx context.Context, sellerID uuid.UUID) (*SellerEligibilityResponse, error) {
	salesCount, err := s.repo.CountSuccessfulSellerSales(ctx, sellerID)
	if err != nil {
		return nil, err
	}

	hasOpen, err := s.repo.HasOpenPlatformCampaign(ctx, sellerID)
	if err != nil {
		return nil, err
	}

	return &SellerEligibilityResponse{
		SellerID:                sellerID,
		SuccessfulSalesCount:    salesCount,
		RequiredSalesThreshold:  RequiredSalesEligibilityThreshold,
		IsEligible:              salesCount >= RequiredSalesEligibilityThreshold,
		HasOpenPlatformCampaign: hasOpen,
	}, nil
}

// CreateSellerCampaign creates a purely seller-funded campaign (no ZAMK approval required).
func (s *Service) CreateSellerCampaign(ctx context.Context, sellerID uuid.UUID, req CreateSellerCampaignRequest) (*MarketingCampaign, error) {
	campaign := &MarketingCampaign{
		ID:                       uuid.New(),
		SellerID:                 sellerID,
		Title:                    strings.TrimSpace(req.Title),
		Description:              req.Description,
		FundingMode:              FundingModeSeller,
		Status:                   CampaignStatusDraft,
		DiscountType:             req.DiscountType,
		SellerDiscountBps:        req.SellerDiscountBps,
		SellerDiscountFixedCents: req.SellerDiscountFixedCents,
		StartsAt:                 req.StartsAt,
		EndsAt:                   req.EndsAt,
	}

	if err := ValidateCampaign(campaign); err != nil {
		return nil, err
	}

	if err := s.repo.CreateCampaign(ctx, campaign); err != nil {
		return nil, err
	}

	return campaign, nil
}

// CreateCofundedApplication creates a seller co-funding application in draft state.
func (s *Service) CreateCofundedApplication(ctx context.Context, sellerID uuid.UUID, req CreateCofundedApplicationRequest) (*MarketingCampaign, error) {
	eligibility, err := s.GetSellerEligibility(ctx, sellerID)
	if err != nil {
		return nil, err
	}
	if !eligibility.IsEligible {
		return nil, fmt.Errorf("seller is not eligible for co-funding: has %d sales, requires %d", eligibility.SuccessfulSalesCount, RequiredSalesEligibilityThreshold)
	}
	if eligibility.HasOpenPlatformCampaign {
		return nil, ErrOpenPlatformFundingExists
	}

	campaign := &MarketingCampaign{
		ID:                          uuid.New(),
		SellerID:                    sellerID,
		Title:                       strings.TrimSpace(req.Title),
		Description:                 req.Description,
		FundingMode:                 FundingModeCofunded,
		Status:                      CampaignStatusDraft,
		DiscountType:                DiscountTypePercent, // ZAMK/COFUNDED is strictly percent in V1
		SellerDiscountBps:           req.SellerDiscountBps,
		RequestedZamkShareBps:       req.RequestedZamkShareBps,
		RequestedZamkBudgetCapCents: req.RequestedZamkBudgetCapCents,
		StartsAt:                    req.StartsAt,
		EndsAt:                      req.EndsAt,
	}

	if err := ValidateCampaign(campaign); err != nil {
		return nil, err
	}

	if err := s.repo.CreateCampaign(ctx, campaign); err != nil {
		return nil, err
	}

	return campaign, nil
}

// SubmitCampaign transitions a draft campaign to submitted status.
func (s *Service) SubmitCampaign(ctx context.Context, sellerID uuid.UUID, campaignID uuid.UUID) (*MarketingCampaign, error) {
	c, err := s.repo.GetCampaignByID(ctx, campaignID)
	if err != nil {
		return nil, err
	}
	if c.SellerID != sellerID {
		return nil, ErrSellerUnauthorized
	}
	if c.Status != CampaignStatusDraft {
		return nil, fmt.Errorf("only draft campaigns can be submitted; current status is %s", c.Status)
	}

	if c.FundingMode == FundingModeCofunded || c.FundingMode == FundingModeZamk {
		eligibility, err := s.GetSellerEligibility(ctx, sellerID)
		if err != nil {
			return nil, err
		}
		if !eligibility.IsEligible {
			return nil, fmt.Errorf("seller is not eligible for co-funding")
		}
	}

	if err := s.repo.SubmitCampaign(ctx, campaignID, sellerID); err != nil {
		return nil, err
	}

	return s.repo.GetCampaignByID(ctx, campaignID)
}

// AdminReviewCampaign allows Admin to approve, counter-offer, or reject a submitted campaign.
func (s *Service) AdminReviewCampaign(ctx context.Context, staffUserID uuid.UUID, campaignID uuid.UUID, req AdminReviewCampaignRequest) (*MarketingCampaign, error) {
	c, err := s.repo.GetCampaignByID(ctx, campaignID)
	if err != nil {
		return nil, err
	}
	if c.Status != CampaignStatusSubmitted {
		return nil, fmt.Errorf("campaign is not in submitted status (current: %s)", c.Status)
	}

	switch req.Action {
	case "approve":
		appShare := c.RequestedZamkShareBps
		if req.ApprovedZamkShareBps != nil {
			appShare = *req.ApprovedZamkShareBps
		}
		appBudget := c.RequestedZamkBudgetCapCents
		if req.ApprovedZamkBudgetCents != nil {
			appBudget = *req.ApprovedZamkBudgetCents
		}

		isCounterOffer, err := EvaluateAdminDecision(c, appShare, appBudget)
		if err != nil {
			return nil, err
		}

		if isCounterOffer {
			if err := s.repo.AdminCounterOfferCampaign(ctx, campaignID, staffUserID, appShare, appBudget, req.AdminComment); err != nil {
				return nil, err
			}
		} else {
			if err := s.repo.AdminApproveCampaign(ctx, campaignID, staffUserID, appShare, appBudget, req.AdminComment); err != nil {
				return nil, err
			}
		}

	case "reject":
		reason := "admin_rejected"
		if req.RejectionReason != nil && *req.RejectionReason != "" {
			reason = *req.RejectionReason
		}
		if err := s.repo.AdminRejectCampaign(ctx, campaignID, staffUserID, reason, req.AdminComment); err != nil {
			return nil, err
		}

	default:
		return nil, fmt.Errorf("invalid review action: %s", req.Action)
	}

	return s.repo.GetCampaignByID(ctx, campaignID)
}

// SellerAcceptCounterOffer allows seller to accept admin-adjusted financial terms.
func (s *Service) SellerAcceptCounterOffer(ctx context.Context, sellerID uuid.UUID, campaignID uuid.UUID) (*MarketingCampaign, error) {
	c, err := s.repo.GetCampaignByID(ctx, campaignID)
	if err != nil {
		return nil, err
	}
	if c.SellerID != sellerID {
		return nil, ErrSellerUnauthorized
	}
	if c.Status != CampaignStatusCounterOffered {
		return nil, ErrCounterOfferInvalidState
	}

	if err := s.repo.SellerAcceptCounterOffer(ctx, campaignID, sellerID); err != nil {
		return nil, err
	}

	return s.repo.GetCampaignByID(ctx, campaignID)
}

// SellerRejectCounterOffer allows seller to cancel campaign upon counter-offer.
func (s *Service) SellerRejectCounterOffer(ctx context.Context, sellerID uuid.UUID, campaignID uuid.UUID) (*MarketingCampaign, error) {
	c, err := s.repo.GetCampaignByID(ctx, campaignID)
	if err != nil {
		return nil, err
	}
	if c.SellerID != sellerID {
		return nil, ErrSellerUnauthorized
	}
	if c.Status != CampaignStatusCounterOffered {
		return nil, ErrCounterOfferInvalidState
	}

	if err := s.repo.SellerRejectCounterOffer(ctx, campaignID, sellerID); err != nil {
		return nil, err
	}

	return s.repo.GetCampaignByID(ctx, campaignID)
}

// CreatePromoCode creates a promo code attached to an active or approved campaign.
func (s *Service) CreatePromoCode(ctx context.Context, sellerID uuid.UUID, req CreatePromoCodeRequest) (*PromoCode, error) {
	c, err := s.repo.GetCampaignByID(ctx, req.CampaignID)
	if err != nil {
		return nil, err
	}
	if c.SellerID != sellerID {
		return nil, ErrSellerUnauthorized
	}

	audienceType := req.AudienceType
	if audienceType == "" {
		audienceType = AudienceAllCustomers
	}
	if !IsValidAudienceType(audienceType) {
		return nil, ErrInvalidAudienceType
	}

	promo := &PromoCode{
		ID:                      uuid.New(),
		CampaignID:              req.CampaignID,
		SellerID:                sellerID,
		Code:                    strings.ToUpper(strings.TrimSpace(req.Code)),
		DiscountType:            req.DiscountType,
		DiscountValueBps:        req.DiscountValueBps,
		DiscountValueFixedCents: req.DiscountValueFixedCents,
		MinOrderSubtotalCents:   req.MinOrderSubtotalCents,
		MinEligibleQuantity:     req.MinEligibleQuantity,
		MinDistinctProducts:     req.MinDistinctProducts,
		GlobalUsageLimit:        req.GlobalUsageLimit,
		PerCustomerUsageLimit:   req.PerCustomerUsageLimit,
		AudienceType:            audienceType,
		IsActive:                true,
		StartsAt:                req.StartsAt,
		EndsAt:                  req.EndsAt,
	}
	if promo.PerCustomerUsageLimit <= 0 {
		promo.PerCustomerUsageLimit = 1
	}

	if err := ValidatePromoCode(promo); err != nil {
		return nil, err
	}

	if err := s.repo.CreatePromoCode(ctx, promo); err != nil {
		return nil, err
	}

	return promo, nil
}

// RecordProductPriceChange records a forward-only product price change audit entry.
func (s *Service) RecordProductPriceChange(ctx context.Context, productID uuid.UUID, variantID *uuid.UUID, oldPrice, newPrice int64, changedBy *uuid.UUID, source PriceChangeSource, reason *string) (*ProductPriceHistory, error) {
	if oldPrice < 0 || newPrice < 0 {
		return nil, ErrInvalidPriceHistory
	}

	h := &ProductPriceHistory{
		ID:               uuid.New(),
		ProductID:        productID,
		ProductVariantID: variantID,
		OldPriceCents:    oldPrice,
		NewPriceCents:    newPrice,
		ChangedByUserID:  changedBy,
		Source:           source,
		Reason:           reason,
		CreatedAt:        time.Now().UTC(),
	}

	if err := s.repo.RecordPriceChange(ctx, h); err != nil {
		return nil, err
	}

	return h, nil
}

// PromotedOrderItemInput provides item details necessary to calculate and reserve promo discounts.
type PromotedOrderItemInput struct {
	OrderItemID        uuid.UUID
	ProductID          uuid.UUID
	ProductVariantID   uuid.UUID
	SellerID           uuid.UUID
	BaseUnitPriceCents int64
	Quantity           int
}

// ValidateAndCalculateCheckoutPromoTx validates a promo code against order items and customer, computing the exact financial split.
func (s *Service) ValidateAndCalculateCheckoutPromoTx(
	ctx context.Context,
	tx DBExecutor,
	customerID uuid.UUID,
	code string,
	items []PromotedOrderItemInput,
	commissionBps int,
	now time.Time,
) (*CheckoutPromoCalculation, error) {
	normalized := strings.TrimSpace(code)
	if normalized == "" {
		return nil, ErrPromoNotFound
	}

	// 1. Lock promo code row
	promo, err := s.repo.GetPromoCodeByCodeTx(ctx, tx, normalized)
	if err != nil {
		return nil, err
	}
	promo, err = s.repo.GetPromoCodeForUpdateTx(ctx, tx, promo.ID)
	if err != nil {
		return nil, err
	}

	// 2. Validate promo status and windows
	if !promo.IsActive {
		return nil, ErrPromoInactive
	}
	if promo.StartsAt != nil && now.Before(*promo.StartsAt) {
		return nil, ErrPromoNotStarted
	}
	if promo.EndsAt != nil && now.After(*promo.EndsAt) {
		return nil, ErrPromoExpired
	}

	// 3. Lock campaign row
	campaign, err := s.repo.GetCampaignForUpdateTx(ctx, tx, promo.CampaignID)
	if err != nil {
		return nil, err
	}
	if campaign.Status != CampaignStatusActive && campaign.Status != CampaignStatusApproved {
		return nil, ErrPromoInactive
	}
	if campaign.StartsAt != nil && now.Before(*campaign.StartsAt) {
		return nil, ErrPromoNotStarted
	}
	if campaign.EndsAt != nil && now.After(*campaign.EndsAt) {
		return nil, ErrPromoExpired
	}

	// 4. Global usage limit check (consumed + active reservations)
	if promo.GlobalUsageLimit != nil {
		globalCount, err := s.repo.CountPromoUsageGlobalTx(ctx, tx, promo.ID)
		if err != nil {
			return nil, err
		}
		if globalCount >= *promo.GlobalUsageLimit {
			return nil, ErrPromoGlobalLimit
		}
	}

	// 5. Per-customer usage limit check
	customerCount, err := s.repo.CountPromoUsageCustomerTx(ctx, tx, promo.ID, customerID)
	if err != nil {
		return nil, err
	}
	if customerCount >= promo.PerCustomerUsageLimit {
		return nil, ErrPromoCustomerLimit
	}

	// 6. Audience verification
	switch promo.AudienceType {
	case AudienceFirstPaidOrder:
		paidCount, err := s.repo.CountCustomerSuccessfullyPaidOrdersTx(ctx, tx, customerID, uuid.Nil)
		if err != nil {
			return nil, err
		}
		if paidCount > 0 {
			return nil, ErrPromoFirstOrderOnly
		}
		hasActive, err := s.repo.HasActiveFirstOrderReservationTx(ctx, tx, customerID, uuid.Nil)
		if err != nil {
			return nil, err
		}
		if hasActive {
			return nil, ErrPromoFirstOrderOnly
		}
	case AudienceRepeatCustomer:
		paidCount, err := s.repo.CountCustomerSuccessfullyPaidOrdersTx(ctx, tx, customerID, uuid.Nil)
		if err != nil {
			return nil, err
		}
		if paidCount == 0 {
			return nil, ErrPromoRepeatCustomerRequired
		}
	case AudienceAllCustomers:
		// no customer paid-history restriction
	default:
		return nil, ErrInvalidAudienceType
	}

	// 7. Load targets and filter eligible items (strictly belonging to promo's seller, matching scope and exclusions)
	targets, err := s.repo.GetPromoCodeProductTargetsTx(ctx, tx, promo.ID)
	if err != nil {
		return nil, err
	}
	productIncludeMap := make(map[uuid.UUID]bool)
	productExcludeMap := make(map[uuid.UUID]bool)
	for _, t := range targets {
		if t.TargetType == TargetTypeInclude {
			productIncludeMap[t.ProductID] = true
		} else if t.TargetType == TargetTypeExclude {
			productExcludeMap[t.ProductID] = true
		}
	}

	categoryTargets, err := s.repo.GetPromoCodeCategoryTargetsTx(ctx, tx, promo.ID)
	if err != nil {
		return nil, err
	}
	var incCatIDs []uuid.UUID
	var excCatIDs []uuid.UUID
	for _, ct := range categoryTargets {
		if ct.TargetType == TargetTypeInclude {
			incCatIDs = append(incCatIDs, ct.CategoryID)
		} else if ct.TargetType == TargetTypeExclude {
			excCatIDs = append(excCatIDs, ct.CategoryID)
		}
	}

	// If category scope or category exclusions exist, resolve product category IDs and category subtrees
	var productCategoryMap map[uuid.UUID]uuid.UUID
	categoryIncludeMap := make(map[uuid.UUID]bool)
	categoryExcludeMap := make(map[uuid.UUID]bool)

	if promo.ProductScope == ProductScopeSelectedCategories || len(excCatIDs) > 0 {
		allProdIDs := make([]uuid.UUID, 0, len(items))
		for _, it := range items {
			if it.SellerID == promo.SellerID {
				allProdIDs = append(allProdIDs, it.ProductID)
			}
		}
		if len(allProdIDs) > 0 {
			productCategoryMap, err = s.repo.GetProductCategoryIDsTx(ctx, tx, allProdIDs)
			if err != nil {
				return nil, err
			}
		}

		if len(incCatIDs) > 0 {
			expandedInc, err := s.repo.ResolveCategorySubtreeIDsTx(ctx, tx, incCatIDs, true)
			if err != nil {
				return nil, err
			}
			for _, cid := range expandedInc {
				categoryIncludeMap[cid] = true
			}
		}

		if len(excCatIDs) > 0 {
			expandedExc, err := s.repo.ResolveCategorySubtreeIDsTx(ctx, tx, excCatIDs, false)
			if err != nil {
				return nil, err
			}
			for _, cid := range expandedExc {
				categoryExcludeMap[cid] = true
			}
		}
	}

	var eligibleItems []PromotedOrderItemInput
	var eligibleSubtotal int64
	for _, it := range items {
		if it.SellerID != promo.SellerID {
			continue
		}
		// 1. Explicit product exclusion is the final catalog-level veto
		if productExcludeMap[it.ProductID] {
			continue
		}
		// 2. Category exclusion always wins
		prodCatID := productCategoryMap[it.ProductID]
		if categoryExcludeMap[prodCatID] {
			continue
		}
		// 3. Scope evaluation
		if promo.ProductScope == ProductScopeSelectedProducts {
			if !productIncludeMap[it.ProductID] {
				continue
			}
		} else if promo.ProductScope == ProductScopeSelectedCategories {
			// Fail-closed: zero effective included categories or category not matched => ineligible
			if !categoryIncludeMap[prodCatID] {
				continue
			}
		}
		// ENTIRE_STORE allows all seller products not removed by product or category exclusions

		eligibleItems = append(eligibleItems, it)
		eligibleSubtotal += it.BaseUnitPriceCents * int64(it.Quantity)
	}
	if len(eligibleItems) == 0 {
		return nil, ErrPromoNotApplicable
	}

	// 8. Minimum subtotal and quantity checks against eligible items only
	if eligibleSubtotal < promo.MinOrderSubtotalCents {
		return nil, ErrPromoMinSubtotal
	}

	var eligibleQuantity int64
	for _, it := range eligibleItems {
		eligibleQuantity += int64(it.Quantity)
	}
	if promo.MinEligibleQuantity != nil && eligibleQuantity < int64(*promo.MinEligibleQuantity) {
		return nil, ErrPromoMinQuantity
	}

	if promo.MinDistinctProducts != nil {
		distinctProducts := make(map[uuid.UUID]struct{})
		for _, it := range eligibleItems {
			distinctProducts[it.ProductID] = struct{}{}
		}
		if len(distinctProducts) < *promo.MinDistinctProducts {
			return nil, ErrPromoMinDistinctProducts
		}
	}

	// 9. Compute line economics
	var promotedLines []PromotedLineResult
	var totalSellerDiscount int64
	var totalZamkSubsidy int64
	var totalCustomerPaid int64

	if campaign.DiscountType == DiscountTypePercent {
		sellerBps := campaign.SellerDiscountBps
		zamkBps := campaign.ApprovedZamkShareBps

		for _, it := range eligibleItems {
			lineRes, err := CalculateLineSplitDiscount(it.BaseUnitPriceCents, it.Quantity, sellerBps, zamkBps, commissionBps)
			if err != nil {
				return nil, err
			}
			promotedLines = append(promotedLines, PromotedLineResult{
				OrderItemID:                 it.OrderItemID,
				ProductID:                   it.ProductID,
				ProductVariantID:            it.ProductVariantID,
				SellerID:                    it.SellerID,
				BaseUnitPriceCents:          lineRes.BaseUnitPriceCents,
				Quantity:                    lineRes.Quantity,
				SellerDiscountUnitCents:     lineRes.SellerDiscountUnitCents,
				ZamkSubsidyUnitCents:        lineRes.ZamkSubsidyUnitCents,
				CustomerPaidUnitPriceCents:  lineRes.CustomerPaidUnitPriceCents,
				CommissionBaseUnitCents:     lineRes.CommissionBaseUnitCents,
				TotalSellerDiscountCents:    lineRes.TotalSellerDiscountCents,
				TotalZamkSubsidyCents:       lineRes.TotalZamkSubsidyCents,
				TotalCustomerPaidCents:      lineRes.TotalCustomerPaidCents,
				TotalCommissionBaseCents:    lineRes.TotalCommissionBaseCents,
				CommissionRateBps:           lineRes.CommissionRateBps,
				TotalCommissionChargedCents: lineRes.TotalCommissionChargedCents,
			})
			totalSellerDiscount += lineRes.TotalSellerDiscountCents
			totalZamkSubsidy += lineRes.TotalZamkSubsidyCents
			totalCustomerPaid += lineRes.TotalCustomerPaidCents
		}

		// Apply maximum discount cap if order-level discount exceeds cap
		if promo.MaxDiscountCents != nil && *promo.MaxDiscountCents > 0 && totalSellerDiscount > *promo.MaxDiscountCents {
			targetDiscount := *promo.MaxDiscountCents
			fixedInputs := make([]PromotedItemInput, len(eligibleItems))
			for i, it := range eligibleItems {
				fixedInputs[i] = PromotedItemInput{
					ID:                 it.OrderItemID.String(),
					BaseUnitPriceCents: it.BaseUnitPriceCents,
					Quantity:           it.Quantity,
					CommissionRateBps:  commissionBps,
				}
			}
			fixedResults, err := AllocateSellerFixedDiscount(targetDiscount, fixedInputs)
			if err != nil {
				return nil, err
			}
			promotedLines = nil
			totalSellerDiscount = 0
			totalZamkSubsidy = 0
			totalCustomerPaid = 0
			for i, lineRes := range fixedResults {
				it := eligibleItems[i]
				promotedLines = append(promotedLines, PromotedLineResult{
					OrderItemID:                 it.OrderItemID,
					ProductID:                   it.ProductID,
					ProductVariantID:            it.ProductVariantID,
					SellerID:                    it.SellerID,
					BaseUnitPriceCents:          lineRes.BaseUnitPriceCents,
					Quantity:                    lineRes.Quantity,
					SellerDiscountUnitCents:     lineRes.SellerDiscountUnitCents,
					ZamkSubsidyUnitCents:        0,
					CustomerPaidUnitPriceCents:  lineRes.CustomerPaidUnitPriceCents,
					CommissionBaseUnitCents:     lineRes.CommissionBaseUnitCents,
					TotalSellerDiscountCents:    lineRes.TotalSellerDiscountCents,
					TotalZamkSubsidyCents:       0,
					TotalCustomerPaidCents:      lineRes.TotalCustomerPaidCents,
					TotalCommissionBaseCents:    lineRes.TotalCommissionBaseCents,
					CommissionRateBps:           lineRes.CommissionRateBps,
					TotalCommissionChargedCents: lineRes.TotalCommissionChargedCents,
				})
				totalSellerDiscount += lineRes.TotalSellerDiscountCents
				totalCustomerPaid += lineRes.TotalCustomerPaidCents
			}
		}
	} else if campaign.DiscountType == DiscountTypeFixed {
		discountToAllocate := promo.DiscountValueFixedCents
		if promo.MaxDiscountCents != nil && *promo.MaxDiscountCents > 0 && *promo.MaxDiscountCents < discountToAllocate {
			discountToAllocate = *promo.MaxDiscountCents
		}
		fixedInputs := make([]PromotedItemInput, len(eligibleItems))
		for i, it := range eligibleItems {
			fixedInputs[i] = PromotedItemInput{
				ID:                 it.OrderItemID.String(),
				BaseUnitPriceCents: it.BaseUnitPriceCents,
				Quantity:           it.Quantity,
				CommissionRateBps:  commissionBps,
			}
		}
		fixedResults, err := AllocateSellerFixedDiscount(discountToAllocate, fixedInputs)
		if err != nil {
			return nil, err
		}
		for i, lineRes := range fixedResults {
			it := eligibleItems[i]
			promotedLines = append(promotedLines, PromotedLineResult{
				OrderItemID:                 it.OrderItemID,
				ProductID:                   it.ProductID,
				ProductVariantID:            it.ProductVariantID,
				SellerID:                    it.SellerID,
				BaseUnitPriceCents:          lineRes.BaseUnitPriceCents,
				Quantity:                    lineRes.Quantity,
				SellerDiscountUnitCents:     lineRes.SellerDiscountUnitCents,
				ZamkSubsidyUnitCents:        0,
				CustomerPaidUnitPriceCents:  lineRes.CustomerPaidUnitPriceCents,
				CommissionBaseUnitCents:     lineRes.CommissionBaseUnitCents,
				TotalSellerDiscountCents:    lineRes.TotalSellerDiscountCents,
				TotalZamkSubsidyCents:       0,
				TotalCustomerPaidCents:      lineRes.TotalCustomerPaidCents,
				TotalCommissionBaseCents:    lineRes.TotalCommissionBaseCents,
				CommissionRateBps:           lineRes.CommissionRateBps,
				TotalCommissionChargedCents: lineRes.TotalCommissionChargedCents,
			})
			totalSellerDiscount += lineRes.TotalSellerDiscountCents
			totalCustomerPaid += lineRes.TotalCustomerPaidCents
		}
	}

	// 10. Platform budget check
	if totalZamkSubsidy > 0 {
		if campaign.ZamkSpentCents+campaign.ZamkReservedCents+totalZamkSubsidy > campaign.ApprovedZamkBudgetCapCents {
			return nil, ErrPromoBudgetExhausted
		}
	}

	return &CheckoutPromoCalculation{
		PromoCodeID:              promo.ID,
		CampaignID:               campaign.ID,
		SellerID:                 promo.SellerID,
		FundingMode:              campaign.FundingMode,
		DiscountType:             campaign.DiscountType,
		Code:                     promo.Code,
		IsFirstOrder:             promo.AudienceType == AudienceFirstPaidOrder,
		TotalSellerDiscountCents: totalSellerDiscount,
		TotalZamkSubsidyCents:    totalZamkSubsidy,
		TotalCustomerPaidCents:   totalCustomerPaid,
		TotalOrderDiscountCents:  totalSellerDiscount + totalZamkSubsidy,
		PromotedLines:            promotedLines,
	}, nil
}

// ReserveCheckoutPromoTx creates the durable promo_code_usages reservation and immutable order_item_promotions snapshots.
func (s *Service) ReserveCheckoutPromoTx(
	ctx context.Context,
	tx DBExecutor,
	orderID uuid.UUID,
	customerID uuid.UUID,
	calc *CheckoutPromoCalculation,
	expiresAt time.Time,
) error {
	if calc == nil {
		return nil
	}

	// 1. Atomically reserve platform budget if subsidized
	if calc.TotalZamkSubsidyCents > 0 {
		ok, err := s.repo.ReserveZamkBudgetTx(ctx, tx, calc.CampaignID, calc.TotalZamkSubsidyCents)
		if err != nil {
			return err
		}
		if !ok {
			return ErrPromoBudgetExhausted
		}
	}

	// 2. Insert promo_code_usages record
	usage := &PromoUsage{
		ID:                  uuid.New(),
		PromoCodeID:         calc.PromoCodeID,
		CampaignID:          calc.CampaignID,
		OrderID:             orderID,
		UserID:              customerID,
		Status:              UsageStatusReserved,
		SubsidyCents:        calc.TotalZamkSubsidyCents,
		SellerDiscountCents: calc.TotalSellerDiscountCents,
		ReservedAt:          time.Now().UTC(),
		ExpiresAt:           expiresAt,
		IsFirstOrder:        calc.IsFirstOrder,
	}
	if err := s.repo.CreatePromoCodeUsageTx(ctx, tx, usage); err != nil {
		return err
	}

	// 3. Insert immutable order_item_promotions snapshots for all promoted items
	for _, line := range calc.PromotedLines {
		snap := &OrderItemPromotion{
			ID:                          uuid.New(),
			OrderItemID:                 line.OrderItemID,
			OrderID:                     orderID,
			SellerID:                    line.SellerID,
			CampaignID:                  calc.CampaignID,
			PromoCodeID:                 calc.PromoCodeID,
			BaseUnitPriceCents:          line.BaseUnitPriceCents,
			SellerDiscountUnitCents:     line.SellerDiscountUnitCents,
			ZamkSubsidyUnitCents:        line.ZamkSubsidyUnitCents,
			CustomerPaidUnitPriceCents:  line.CustomerPaidUnitPriceCents,
			CommissionBaseUnitCents:     line.CommissionBaseUnitCents,
			Quantity:                    line.Quantity,
			TotalSellerDiscountCents:    line.TotalSellerDiscountCents,
			TotalZamkSubsidyCents:       line.TotalZamkSubsidyCents,
			TotalCustomerPaidCents:      line.TotalCustomerPaidCents,
			TotalCommissionBaseCents:    line.TotalCommissionBaseCents,
			CommissionRateBps:           line.CommissionRateBps,
			TotalCommissionChargedCents: line.TotalCommissionChargedCents,
			CreatedAt:                   time.Now().UTC(),
		}
		if err := s.repo.CreateOrderItemPromotionTx(ctx, tx, snap); err != nil {
			return err
		}
	}

	return nil
}

// ConsumePromoForOrderTx consumes a promo reservation upon payment success.
func (s *Service) ConsumePromoForOrderTx(ctx context.Context, tx DBExecutor, orderID uuid.UUID) error {
	usage, err := s.repo.GetPromoCodeUsageByOrderIDForUpdateTx(ctx, tx, orderID)
	if err != nil {
		return err
	}
	if usage == nil {
		return nil // Order has no promo
	}
	if usage.Status == UsageStatusConsumed {
		return nil // Duplicate webhook, idempotent no-op
	}
	if usage.Status != UsageStatusReserved {
		return ErrPromoUsageNotReserved
	}

	// Consume platform budget
	if usage.SubsidyCents > 0 {
		if err := s.repo.ConsumeZamkBudgetTx(ctx, tx, usage.CampaignID, usage.SubsidyCents); err != nil {
			return err
		}
	}

	// Transition to consumed
	_, err = s.repo.SetPromoCodeUsageStatusTx(ctx, tx, usage.ID, UsageStatusReserved, UsageStatusConsumed, time.Now().UTC())
	return err
}

// ReleasePromoForOrderTx releases a promo reservation upon payment failure, cancellation, or expiration.
func (s *Service) ReleasePromoForOrderTx(ctx context.Context, tx DBExecutor, orderID uuid.UUID, reason string) error {
	usage, err := s.repo.GetPromoCodeUsageByOrderIDForUpdateTx(ctx, tx, orderID)
	if err != nil {
		return err
	}
	if usage == nil {
		return nil // Order has no promo
	}
	if usage.Status == UsageStatusConsumed {
		return nil // Order was already successfully paid, never release consumed promo!
	}
	if usage.Status == UsageStatusReleased || usage.Status == UsageStatusExpired {
		return nil // Already released, idempotent no-op
	}
	if usage.Status != UsageStatusReserved {
		return nil
	}

	// Release platform reserved budget
	if usage.SubsidyCents > 0 {
		if err := s.repo.ReleaseZamkBudgetTx(ctx, tx, usage.CampaignID, usage.SubsidyCents); err != nil {
			return err
		}
	}

	// Transition to released
	_, err = s.repo.SetPromoCodeUsageStatusTx(ctx, tx, usage.ID, UsageStatusReserved, UsageStatusReleased, time.Now().UTC())
	return err
}

// ReleasePromoForOrder releases a promo reservation on pool executor.
func (s *Service) ReleasePromoForOrder(ctx context.Context, orderID uuid.UUID, reason string) error {
	return s.ReleasePromoForOrderTx(ctx, s.pool, orderID, reason)
}

// EnsureOrderPromoHold ensures that promo hold is valid before payment initiation, releasing the hold if first-order eligibility was lost.
func (s *Service) EnsureOrderPromoHold(ctx context.Context, orderID uuid.UUID) error {
	err := s.EnsureOrderPromoHoldTx(ctx, s.pool, orderID)
	if errors.Is(err, ErrPromoFirstOrderOnly) {
		_ = s.ReleasePromoForOrderTx(ctx, s.pool, orderID, "first_order_ineligible")
	}
	return err
}

// EnsureOrderPromoHoldTx guarantees that an awaiting_payment order retains valid promo reservation before payment initiation.
func (s *Service) EnsureOrderPromoHoldTx(ctx context.Context, tx DBExecutor, orderID uuid.UUID) error {
	usage, err := s.repo.GetPromoCodeUsageByOrderIDForUpdateTx(ctx, tx, orderID)
	if err != nil {
		return err
	}
	if usage == nil {
		return nil // Order has no promo
	}
	if usage.Status == UsageStatusConsumed {
		return nil
	}
	if usage.Status == UsageStatusReserved {
		if usage.IsFirstOrder {
			paidCount, err := s.repo.CountCustomerPaidOrdersTx(ctx, tx, usage.UserID, orderID)
			if err != nil {
				return err
			}
			if paidCount > 0 {
				_ = s.ReleasePromoForOrderTx(ctx, tx, orderID, "first_order_ineligible")
				return ErrPromoFirstOrderOnly
			}
		}
		return nil // Hold is active and valid
	}

	// If released, attempt to reacquire hold on payment retry
	promo, err := s.repo.GetPromoCodeForUpdateTx(ctx, tx, usage.PromoCodeID)
	if err != nil {
		return err
	}
	if !promo.IsActive || (promo.EndsAt != nil && time.Now().UTC().After(*promo.EndsAt)) {
		return ErrPromoExpired
	}

	campaign, err := s.repo.GetCampaignForUpdateTx(ctx, tx, usage.CampaignID)
	if err != nil {
		return err
	}
	if campaign.Status != CampaignStatusActive && campaign.Status != CampaignStatusApproved {
		return ErrPromoInactive
	}

	if promo.GlobalUsageLimit != nil {
		gCount, err := s.repo.CountPromoUsageGlobalTx(ctx, tx, promo.ID)
		if err != nil {
			return err
		}
		if gCount >= *promo.GlobalUsageLimit {
			return ErrPromoGlobalLimit
		}
	}

	cCount, err := s.repo.CountPromoUsageCustomerTx(ctx, tx, promo.ID, usage.UserID)
	if err != nil {
		return err
	}
	if cCount >= promo.PerCustomerUsageLimit {
		return ErrPromoCustomerLimit
	}

	if usage.IsFirstOrder {
		paidCount, err := s.repo.CountCustomerPaidOrdersTx(ctx, tx, usage.UserID, orderID)
		if err != nil {
			return err
		}
		if paidCount > 0 {
			return ErrPromoFirstOrderOnly
		}
	}

	if usage.SubsidyCents > 0 {
		ok, err := s.repo.ReserveZamkBudgetTx(ctx, tx, usage.CampaignID, usage.SubsidyCents)
		if err != nil {
			return err
		}
		if !ok {
			return ErrPromoBudgetExhausted
		}
	}

	_, err = s.repo.SetPromoCodeUsageStatusTx(ctx, tx, usage.ID, usage.Status, UsageStatusReserved, time.Now().UTC())
	return err
}

// CreateSellerPromotion atomically provisions a backing SELLER-funded campaign and promo code row.
func (s *Service) CreateSellerPromotion(ctx context.Context, sellerID uuid.UUID, req CreateSellerPromoRequest) (*SellerPromoResponse, error) {
	code := strings.ToUpper(strings.TrimSpace(req.Code))
	if code == "" {
		return nil, fmt.Errorf("promo code cannot be blank")
	}

	// Validate discount representations
	switch req.DiscountType {
	case DiscountTypePercent:
		if req.DiscountValueBps <= 0 || req.DiscountValueBps > 10000 {
			return nil, fmt.Errorf("%w: percent discount must be between 1 and 10000 bps", ErrInvalidDiscountValue)
		}
		if req.DiscountValueFixedCents != 0 {
			return nil, fmt.Errorf("%w: fixed cents must be zero for percent discount", ErrInvalidDiscountValue)
		}
	case DiscountTypeFixed:
		if req.DiscountValueFixedCents <= 0 {
			return nil, fmt.Errorf("%w: fixed discount must be positive cents", ErrInvalidDiscountValue)
		}
		if req.DiscountValueBps != 0 {
			return nil, fmt.Errorf("%w: percent bps must be zero for fixed discount", ErrInvalidDiscountValue)
		}
	default:
		return nil, ErrInvalidDiscountType
	}

	if req.MinOrderSubtotalCents < 0 {
		return nil, fmt.Errorf("min order subtotal cannot be negative")
	}

	perCustLimit := req.PerCustomerUsageLimit
	if perCustLimit <= 0 {
		perCustLimit = 1
	}

	if req.GlobalUsageLimit != nil && *req.GlobalUsageLimit <= 0 {
		return nil, ErrInvalidLimit
	}

	now := time.Now().UTC()
	if req.EndsAt != nil && now.After(*req.EndsAt) {
		return nil, ErrPromoAlreadyExpired
	}
	if req.StartsAt != nil && req.EndsAt != nil && !req.EndsAt.After(*req.StartsAt) {
		return nil, ErrInvalidDates
	}

	productScope := ProductScopeEntireStore
	if req.ProductScope != nil {
		productScope = *req.ProductScope
	}
	if productScope != ProductScopeEntireStore && productScope != ProductScopeSelectedProducts && productScope != ProductScopeSelectedCategories {
		return nil, ErrInvalidProductScope
	}

	if productScope == ProductScopeSelectedProducts {
		if len(req.IncludedProductIDs) == 0 {
			return nil, fmt.Errorf("SELECTED_PRODUCTS requires at least one included product")
		}
		if len(req.IncludedCategoryIDs) > 0 {
			return nil, fmt.Errorf("SELECTED_PRODUCTS cannot specify included categories")
		}
		if len(req.ExcludedCategoryIDs) > 0 {
			return nil, ErrCategoryNotAllowed
		}
	} else if productScope == ProductScopeSelectedCategories {
		if len(req.IncludedCategoryIDs) == 0 {
			return nil, ErrCategoryRequiresInclude
		}
		if len(req.IncludedProductIDs) > 0 {
			return nil, fmt.Errorf("SELECTED_CATEGORIES cannot specify included products")
		}
	} else if productScope == ProductScopeEntireStore {
		if len(req.IncludedProductIDs) > 0 {
			return nil, fmt.Errorf("ENTIRE_STORE cannot specify included products")
		}
		if len(req.IncludedCategoryIDs) > 0 {
			return nil, fmt.Errorf("ENTIRE_STORE cannot specify included categories")
		}
	}

	// Validate duplicate IDs in product includes and excludes
	incMap := make(map[uuid.UUID]struct{}, len(req.IncludedProductIDs))
	for _, id := range req.IncludedProductIDs {
		if _, exists := incMap[id]; exists {
			return nil, fmt.Errorf("%w: duplicate product ID in included products", ErrInvalidProductScope)
		}
		incMap[id] = struct{}{}
	}

	excMap := make(map[uuid.UUID]struct{}, len(req.ExcludedProductIDs))
	for _, id := range req.ExcludedProductIDs {
		if _, exists := excMap[id]; exists {
			return nil, fmt.Errorf("%w: duplicate product ID in excluded products", ErrInvalidProductScope)
		}
		if _, inInc := incMap[id]; inInc {
			return nil, ErrProductConflict
		}
		excMap[id] = struct{}{}
	}

	// Validate duplicate IDs in category includes and excludes
	catIncMap := make(map[uuid.UUID]struct{}, len(req.IncludedCategoryIDs))
	for _, id := range req.IncludedCategoryIDs {
		if _, exists := catIncMap[id]; exists {
			return nil, fmt.Errorf("duplicate category ID in included categories")
		}
		catIncMap[id] = struct{}{}
	}

	catExcMap := make(map[uuid.UUID]struct{}, len(req.ExcludedCategoryIDs))
	for _, id := range req.ExcludedCategoryIDs {
		if _, exists := catExcMap[id]; exists {
			return nil, fmt.Errorf("duplicate category ID in excluded categories")
		}
		if _, inInc := catIncMap[id]; inInc {
			return nil, ErrCategoryConflict
		}
		catExcMap[id] = struct{}{}
	}

	if req.MaxDiscountCents != nil && *req.MaxDiscountCents <= 0 {
		return nil, ErrInvalidMaxDiscount
	}
	if req.MinEligibleQuantity != nil && *req.MinEligibleQuantity <= 0 {
		return nil, ErrInvalidMinQuantity
	}
	if req.MinDistinctProducts != nil && *req.MinDistinctProducts <= 0 {
		return nil, ErrInvalidMinDistinctProducts
	}

	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Validate that all specified products belong to seller
	allTargetIDs := make([]uuid.UUID, 0, len(req.IncludedProductIDs)+len(req.ExcludedProductIDs))
	allTargetIDs = append(allTargetIDs, req.IncludedProductIDs...)
	allTargetIDs = append(allTargetIDs, req.ExcludedProductIDs...)
	if len(allTargetIDs) > 0 {
		owned, err := s.repo.ValidateProductsBelongToSellerTx(ctx, tx, sellerID, allTargetIDs)
		if err != nil {
			return nil, err
		}
		if !owned {
			return nil, ErrProductNotOwnedBySeller
		}
	}

	// Validate that all specified categories exist and are active
	allCatIDs := make([]uuid.UUID, 0, len(req.IncludedCategoryIDs)+len(req.ExcludedCategoryIDs))
	allCatIDs = append(allCatIDs, req.IncludedCategoryIDs...)
	allCatIDs = append(allCatIDs, req.ExcludedCategoryIDs...)
	if len(allCatIDs) > 0 {
		valid, err := s.repo.ValidateCategoriesExistTx(ctx, tx, allCatIDs)
		if err != nil {
			return nil, err
		}
		if !valid {
			return nil, ErrInvalidCategory
		}
	}

	// 1. Backing campaign: funding_mode = seller, status = active, zero ZAMK authority
	campaign := &MarketingCampaign{
		ID:                          uuid.New(),
		SellerID:                    sellerID,
		Title:                       "Промокод " + code,
		FundingMode:                 FundingModeSeller,
		Status:                      CampaignStatusActive,
		DiscountType:                req.DiscountType,
		StartsAt:                    req.StartsAt,
		EndsAt:                      req.EndsAt,
		RequestedZamkShareBps:       0,
		RequestedZamkBudgetCapCents: 0,
		ApprovedZamkShareBps:        0,
		ApprovedZamkBudgetCapCents:  0,
		ZamkReservedCents:          0,
		ZamkSpentCents:             0,
	}
	if req.DiscountType == DiscountTypePercent {
		campaign.SellerDiscountBps = req.DiscountValueBps
		campaign.SellerDiscountFixedCents = 0
	} else {
		campaign.SellerDiscountFixedCents = req.DiscountValueFixedCents
		campaign.SellerDiscountBps = 0
	}

	if err := ValidateCampaign(campaign); err != nil {
		return nil, err
	}
	if err := s.repo.CreateCampaignTx(ctx, tx, campaign); err != nil {
		return nil, err
	}

	// 2. Promo code linked to campaign
	audienceType := req.AudienceType
	if audienceType == "" {
		audienceType = AudienceAllCustomers
	}
	if !IsValidAudienceType(audienceType) {
		return nil, ErrInvalidAudienceType
	}

	promo := &PromoCode{
		ID:                      uuid.New(),
		CampaignID:              campaign.ID,
		SellerID:                sellerID,
		Code:                    code,
		DiscountType:            req.DiscountType,
		DiscountValueBps:        campaign.SellerDiscountBps,
		DiscountValueFixedCents: campaign.SellerDiscountFixedCents,
		MinOrderSubtotalCents:   req.MinOrderSubtotalCents,
		GlobalUsageLimit:        req.GlobalUsageLimit,
		PerCustomerUsageLimit:   perCustLimit,
		AudienceType:            audienceType,
		ProductScope:            productScope,
		MaxDiscountCents:        req.MaxDiscountCents,
		MinEligibleQuantity:     req.MinEligibleQuantity,
		MinDistinctProducts:     req.MinDistinctProducts,
		IsActive:                isActive,
		StartsAt:                req.StartsAt,
		EndsAt:                  req.EndsAt,
	}

	if err := ValidatePromoCode(promo); err != nil {
		return nil, err
	}
	if err := s.repo.CreatePromoCodeTx(ctx, tx, promo); err != nil {
		return nil, err
	}

	// 3. Persist product targets
	var targets []PromoCodeProductTarget
	for _, id := range req.IncludedProductIDs {
		targets = append(targets, PromoCodeProductTarget{
			PromoCodeID: promo.ID,
			ProductID:   id,
			TargetType:  TargetTypeInclude,
		})
	}
	for _, id := range req.ExcludedProductIDs {
		targets = append(targets, PromoCodeProductTarget{
			PromoCodeID: promo.ID,
			ProductID:   id,
			TargetType:  TargetTypeExclude,
		})
	}
	if len(targets) > 0 {
		if err := s.repo.CreatePromoCodeProductTargetsTx(ctx, tx, targets); err != nil {
			return nil, err
		}
	}

	// 4. Persist category targets
	var catTargets []PromoCodeCategoryTarget
	for _, id := range req.IncludedCategoryIDs {
		catTargets = append(catTargets, PromoCodeCategoryTarget{
			PromoCodeID: promo.ID,
			CategoryID:  id,
			TargetType:  TargetTypeInclude,
		})
	}
	for _, id := range req.ExcludedCategoryIDs {
		catTargets = append(catTargets, PromoCodeCategoryTarget{
			PromoCodeID: promo.ID,
			CategoryID:  id,
			TargetType:  TargetTypeExclude,
		})
	}
	if len(catTargets) > 0 {
		if err := s.repo.CreatePromoCodeCategoryTargetsTx(ctx, tx, catTargets); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit promotion creation: %w", err)
	}

	includedIDs := req.IncludedProductIDs
	if includedIDs == nil {
		includedIDs = []uuid.UUID{}
	}
	excludedIDs := req.ExcludedProductIDs
	if excludedIDs == nil {
		excludedIDs = []uuid.UUID{}
	}
	includedCatIDs := req.IncludedCategoryIDs
	if includedCatIDs == nil {
		includedCatIDs = []uuid.UUID{}
	}
	excludedCatIDs := req.ExcludedCategoryIDs
	if excludedCatIDs == nil {
		excludedCatIDs = []uuid.UUID{}
	}

	return &SellerPromoResponse{
		ID:                      promo.ID,
		CampaignID:              promo.CampaignID,
		SellerID:                promo.SellerID,
		Code:                    promo.Code,
		DiscountType:            promo.DiscountType,
		DiscountValueBps:        promo.DiscountValueBps,
		DiscountValueFixedCents: promo.DiscountValueFixedCents,
		MinOrderSubtotalCents:   promo.MinOrderSubtotalCents,
		GlobalUsageLimit:        promo.GlobalUsageLimit,
		PerCustomerUsageLimit:   promo.PerCustomerUsageLimit,
		AudienceType:            promo.AudienceType,
		ProductScope:            promo.ProductScope,
		IncludedProductIDs:      includedIDs,
		ExcludedProductIDs:      excludedIDs,
		IncludedCategoryIDs:     includedCatIDs,
		ExcludedCategoryIDs:     excludedCatIDs,
		MaxDiscountCents:        promo.MaxDiscountCents,
		MinEligibleQuantity:     promo.MinEligibleQuantity,
		MinDistinctProducts:     promo.MinDistinctProducts,
		IsActive:                promo.IsActive,
		StartsAt:                promo.StartsAt,
		EndsAt:                  promo.EndsAt,
		ReservedUsageCount:      0,
		ConsumedUsageCount:      0,
		CreatedAt:               promo.CreatedAt,
		UpdatedAt:               promo.UpdatedAt,
		Status:                  DerivePromoStatus(promo, 0, 0, now),
	}, nil
}

// ListSellerPromotions retrieves all promotions for an authenticated seller with derived status.
func (s *Service) ListSellerPromotions(ctx context.Context, sellerID uuid.UUID) ([]SellerPromoResponse, error) {
	items, err := s.repo.ListSellerPromos(ctx, sellerID)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	res := make([]SellerPromoResponse, len(items))
	for i, it := range items {
		inc := it.IncludedProductIDs
		if inc == nil {
			inc = []uuid.UUID{}
		}
		exc := it.ExcludedProductIDs
		if exc == nil {
			exc = []uuid.UUID{}
		}
		incCat := it.IncludedCategoryIDs
		if incCat == nil {
			incCat = []uuid.UUID{}
		}
		excCat := it.ExcludedCategoryIDs
		if excCat == nil {
			excCat = []uuid.UUID{}
		}
		res[i] = SellerPromoResponse{
			ID:                      it.ID,
			CampaignID:              it.CampaignID,
			SellerID:                it.SellerID,
			Code:                    it.Code,
			DiscountType:            it.DiscountType,
			DiscountValueBps:        it.DiscountValueBps,
			DiscountValueFixedCents: it.DiscountValueFixedCents,
			MinOrderSubtotalCents:   it.MinOrderSubtotalCents,
			GlobalUsageLimit:        it.GlobalUsageLimit,
			PerCustomerUsageLimit:   it.PerCustomerUsageLimit,
			AudienceType:            it.AudienceType,
			ProductScope:            it.ProductScope,
			IncludedProductIDs:      inc,
			ExcludedProductIDs:      exc,
			IncludedCategoryIDs:     incCat,
			ExcludedCategoryIDs:     excCat,
			MaxDiscountCents:        it.MaxDiscountCents,
			MinEligibleQuantity:     it.MinEligibleQuantity,
			MinDistinctProducts:     it.MinDistinctProducts,
			IsActive:                it.IsActive,
			StartsAt:                it.StartsAt,
			EndsAt:                  it.EndsAt,
			ReservedUsageCount:      it.ReservedCount,
			ConsumedUsageCount:      it.ConsumedCount,
			CreatedAt:               it.CreatedAt,
			UpdatedAt:               it.UpdatedAt,
			Status:                  DerivePromoStatus(&it.PromoCode, it.ReservedCount, it.ConsumedCount, now),
		}
	}
	return res, nil
}

// UpdateSellerPromotion updates mutable operational limits or active status for a seller promo code.
func (s *Service) UpdateSellerPromotion(ctx context.Context, sellerID, promoID uuid.UUID, req UpdateSellerPromoRequest) (*SellerPromoResponse, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// 1. Lock promo row for update strictly scoped to authenticated seller
	promo, err := s.repo.GetSellerPromoForUpdateTx(ctx, tx, promoID, sellerID)
	if err != nil {
		return nil, err
	}

	// 2. Query committed usages (both reserved and consumed count against capacity)
	reservedCount, consumedCount, err := s.repo.CountPromoUsageCommittedTx(ctx, tx, promoID)
	if err != nil {
		return nil, err
	}
	totalCommitted := reservedCount + consumedCount

	// 3. Validate and apply global usage limit update
	if req.GlobalUsageLimit != nil {
		if *req.GlobalUsageLimit <= 0 {
			return nil, ErrInvalidLimit
		}
		if *req.GlobalUsageLimit < totalCommitted {
			return nil, fmt.Errorf("%w: requested %d, currently committed %d (reserved: %d, consumed: %d)",
				ErrGlobalLimitBelowUsage, *req.GlobalUsageLimit, totalCommitted, reservedCount, consumedCount)
		}
		promo.GlobalUsageLimit = req.GlobalUsageLimit
	}

	// 4. Validate and apply per-customer limit update
	if req.PerCustomerUsageLimit != nil {
		if *req.PerCustomerUsageLimit < 1 {
			return nil, ErrInvalidLimit
		}
		maxCustUsage, err := s.repo.GetMaxCustomerCommittedUsageTx(ctx, tx, promoID)
		if err != nil {
			return nil, err
		}
		if *req.PerCustomerUsageLimit < maxCustUsage {
			return nil, fmt.Errorf("%w: requested %d, existing customer usage is %d",
				ErrCustomerLimitBelowUsage, *req.PerCustomerUsageLimit, maxCustUsage)
		}
		promo.PerCustomerUsageLimit = *req.PerCustomerUsageLimit
	}

	// 5. Update active toggle
	if req.IsActive != nil {
		promo.IsActive = *req.IsActive
	}

	// 6. Update dates
	if req.StartsAt != nil {
		promo.StartsAt = req.StartsAt
	}
	if req.EndsAt != nil {
		promo.EndsAt = req.EndsAt
	}
	if promo.StartsAt != nil && promo.EndsAt != nil && !promo.EndsAt.After(*promo.StartsAt) {
		return nil, ErrInvalidDates
	}

	// 7. Persist changes
	if err := s.repo.UpdatePromoCodeMutableFieldsTx(ctx, tx, promo); err != nil {
		return nil, err
	}

	// 8. Load targets
	targets, err := s.repo.GetPromoCodeProductTargetsTx(ctx, tx, promo.ID)
	if err != nil {
		return nil, err
	}
	inc := []uuid.UUID{}
	exc := []uuid.UUID{}
	for _, t := range targets {
		if t.TargetType == TargetTypeInclude {
			inc = append(inc, t.ProductID)
		} else if t.TargetType == TargetTypeExclude {
			exc = append(exc, t.ProductID)
		}
	}

	catTargets, err := s.repo.GetPromoCodeCategoryTargetsTx(ctx, tx, promo.ID)
	if err != nil {
		return nil, err
	}
	incCat := []uuid.UUID{}
	excCat := []uuid.UUID{}
	for _, ct := range catTargets {
		if ct.TargetType == TargetTypeInclude {
			incCat = append(incCat, ct.CategoryID)
		} else if ct.TargetType == TargetTypeExclude {
			excCat = append(excCat, ct.CategoryID)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit promotion update: %w", err)
	}

	now := time.Now().UTC()
	return &SellerPromoResponse{
		ID:                      promo.ID,
		CampaignID:              promo.CampaignID,
		SellerID:                promo.SellerID,
		Code:                    promo.Code,
		DiscountType:            promo.DiscountType,
		DiscountValueBps:        promo.DiscountValueBps,
		DiscountValueFixedCents: promo.DiscountValueFixedCents,
		MinOrderSubtotalCents:   promo.MinOrderSubtotalCents,
		GlobalUsageLimit:        promo.GlobalUsageLimit,
		PerCustomerUsageLimit:   promo.PerCustomerUsageLimit,
		AudienceType:            promo.AudienceType,
		ProductScope:            promo.ProductScope,
		IncludedProductIDs:      inc,
		ExcludedProductIDs:      exc,
		IncludedCategoryIDs:     incCat,
		ExcludedCategoryIDs:     excCat,
		MaxDiscountCents:        promo.MaxDiscountCents,
		MinEligibleQuantity:     promo.MinEligibleQuantity,
		MinDistinctProducts:     promo.MinDistinctProducts,
		IsActive:                promo.IsActive,
		StartsAt:                promo.StartsAt,
		EndsAt:                  promo.EndsAt,
		ReservedUsageCount:      reservedCount,
		ConsumedUsageCount:      consumedCount,
		CreatedAt:               promo.CreatedAt,
		UpdatedAt:               promo.UpdatedAt,
		Status:                  DerivePromoStatus(promo, reservedCount, consumedCount, now),
	}, nil
}
