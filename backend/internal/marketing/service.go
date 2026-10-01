package marketing

import (
	"context"
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

	promo := &PromoCode{
		ID:                      uuid.New(),
		CampaignID:              req.CampaignID,
		SellerID:                sellerID,
		Code:                    strings.ToUpper(strings.TrimSpace(req.Code)),
		DiscountType:            req.DiscountType,
		DiscountValueBps:        req.DiscountValueBps,
		DiscountValueFixedCents: req.DiscountValueFixedCents,
		MinOrderSubtotalCents:   req.MinOrderSubtotalCents,
		GlobalUsageLimit:        req.GlobalUsageLimit,
		PerCustomerUsageLimit:   req.PerCustomerUsageLimit,
		FirstPaidOrderOnly:      req.FirstPaidOrderOnly,
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
