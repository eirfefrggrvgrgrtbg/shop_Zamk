package marketing

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// ListAdminCampaigns returns a list of all campaigns with their tracking link counts.
func (h *Handler) ListAdminCampaigns(w http.ResponseWriter, r *http.Request) {
	campaigns, err := h.service.ListAdminCampaigns(r.Context())
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	if campaigns == nil {
		campaigns = []CampaignDetailView{}
	}
	h.writeJSON(w, http.StatusOK, campaigns)
}

// GetAdminCampaign returns campaign details by ID.
func (h *Handler) GetAdminCampaign(w http.ResponseWriter, r *http.Request) {
	campaignIDStr := chi.URLParam(r, "id")
	campaignID, err := uuid.Parse(campaignIDStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid campaign ID")
		return
	}

	campaign, err := h.service.GetAdminCampaign(r.Context(), campaignID)
	if err != nil {
		if errors.Is(err, ErrCampaignNotFound) {
			h.writeError(w, http.StatusNotFound, "not_found", "Campaign not found")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, campaign)
}

// CreateAdminCampaign handles creating a new marketing campaign by an admin.
func (h *Handler) CreateAdminCampaign(w http.ResponseWriter, r *http.Request) {
	var req AdminCreateCampaignRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}
	if req.Title == "" || req.FundingMode == "" || req.DiscountType == "" {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "title, fundingMode, and discountType are required")
		return
	}

	campaign := &MarketingCampaign{
		SellerID:                 req.SellerID,
		Title:                    req.Title,
		Description:              req.Description,
		FundingMode:              req.FundingMode,
		Status:                   CampaignStatusApproved, // Admin created campaigns start as approved
		CampaignChannel:          req.CampaignChannel,
		CampaignType:             req.CampaignType,
		PlannedBudgetCents:       req.PlannedBudgetCents,
		DiscountType:             req.DiscountType,
		SellerDiscountBps:        req.SellerDiscountBps,
		SellerDiscountFixedCents: req.SellerDiscountFixedCents,
		StartsAt:                 req.StartsAt,
		EndsAt:                   req.EndsAt,
	}

	if err := h.service.CreateAdminCampaign(r.Context(), campaign); err != nil {
		if errors.Is(err, ErrInvalidFundingMode) || errors.Is(err, ErrInvalidCampaignStatus) ||
			errors.Is(err, ErrInvalidDiscountType) || errors.Is(err, ErrInvalidCampaignChannel) ||
			errors.Is(err, ErrInvalidCampaignType) || errors.Is(err, ErrInvalidPlannedBudget) {
			h.writeError(w, http.StatusBadRequest, "invalid_parameter", err.Error())
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	// The repository insert has committed before emitting semantic success.
	h.logger.InfoContext(r.Context(), "marketing.advertising_campaign.created", "campaign_id", campaign.ID, "purpose", campaign.Purpose)
	h.writeJSON(w, http.StatusCreated, campaign)
}

// UpdateAdminCampaign handles updating an existing marketing campaign.
func (h *Handler) UpdateAdminCampaign(w http.ResponseWriter, r *http.Request) {
	campaignIDStr := chi.URLParam(r, "id")
	campaignID, err := uuid.Parse(campaignIDStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid campaign ID")
		return
	}

	var req AdminUpdateCampaignRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	updated, err := h.service.UpdateAdminCampaign(r.Context(), campaignID, req)
	if err != nil {
		if errors.Is(err, ErrCampaignNotFound) {
			h.writeError(w, http.StatusNotFound, "not_found", "Campaign not found")
			return
		}
		if errors.Is(err, ErrInvalidCampaignChannel) || errors.Is(err, ErrInvalidCampaignType) ||
			errors.Is(err, ErrInvalidPlannedBudget) || errors.Is(err, ErrInvalidCampaignStatus) {
			h.writeError(w, http.StatusBadRequest, "invalid_parameter", err.Error())
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	h.logger.InfoContext(r.Context(), "marketing.advertising_campaign.updated", "campaign_id", updated.ID, "purpose", updated.Purpose)
	h.writeJSON(w, http.StatusOK, updated)
}

// ListTrackingLinks returns all tracking links for a campaign.
func (h *Handler) ListTrackingLinks(w http.ResponseWriter, r *http.Request) {
	campaignIDStr := chi.URLParam(r, "id")
	campaignID, err := uuid.Parse(campaignIDStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid campaign ID")
		return
	}

	links, err := h.service.ListCampaignTrackingLinks(r.Context(), campaignID)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	if links == nil {
		links = []CampaignTrackingLink{}
	}

	h.writeJSON(w, http.StatusOK, links)
}

// CreateTrackingLink creates a new tracking link for a campaign.
func (h *Handler) CreateTrackingLink(w http.ResponseWriter, r *http.Request) {
	campaignIDStr := chi.URLParam(r, "id")
	campaignID, err := uuid.Parse(campaignIDStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid campaign ID")
		return
	}

	var req AdminCreateTrackingLinkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}
	if req.TargetType == "" {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "targetType is required")
		return
	}

	link := &CampaignTrackingLink{
		CampaignID:      campaignID,
		TargetType:      req.TargetType,
		TargetProductID: req.TargetProductID,
		TargetSellerID:  req.TargetSellerID,
		LandingPath:     req.LandingPath,
		PromoCodeID:     req.PromoCodeID,
	}

	if err := h.service.CreateTrackingLink(r.Context(), link); err != nil {
		if errors.Is(err, ErrCampaignNotFound) || errors.Is(err, ErrPromoCodeNotFound) {
			h.writeError(w, http.StatusNotFound, "not_found", err.Error())
			return
		}
		if errors.Is(err, ErrInvalidTargetConfig) || errors.Is(err, ErrInvalidLandingPath) ||
			errors.Is(err, ErrSellerUnauthorized) {
			h.writeError(w, http.StatusBadRequest, "invalid_parameter", err.Error())
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	h.writeJSON(w, http.StatusCreated, link)
}

// DisableTrackingLink disables a tracking link.
func (h *Handler) DisableTrackingLink(w http.ResponseWriter, r *http.Request) {
	campaignIDStr := chi.URLParam(r, "id")
	campaignID, err := uuid.Parse(campaignIDStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid campaign ID")
		return
	}

	linkIDStr := chi.URLParam(r, "linkId")
	linkID, err := uuid.Parse(linkIDStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid link ID")
		return
	}

	link, err := h.service.DisableTrackingLink(r.Context(), campaignID, linkID)
	if err != nil {
		if errors.Is(err, ErrTrackingLinkNotFound) {
			h.writeError(w, http.StatusNotFound, "not_found", "Tracking link not found")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, link)
}
