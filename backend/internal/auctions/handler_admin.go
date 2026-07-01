package auctions

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type AdminHandler struct {
	repo    *Repository
	service *Service
	logger  *slog.Logger
}

func NewAdminHandler(repo *Repository, service *Service, logger *slog.Logger) *AdminHandler {
	return &AdminHandler{
		repo:    repo,
		service: service,
		logger:  logger,
	}
}

func (h *AdminHandler) writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]string{
			"code":    code,
			"message": message,
		},
	})
}

// POST /api/admin/auctions
func (h *AdminHandler) CreateAuction(w http.ResponseWriter, r *http.Request) {
	userIDVal := r.Context().Value("userID")
	if userIDVal == nil {
		h.writeError(w, http.StatusForbidden, "forbidden", "Access denied")
		return
	}
	userID := userIDVal.(uuid.UUID)

	var req AdminCreateAuctionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid JSON format")
		return
	}

	event := &AuctionEvent{
		ID:                              uuid.New(),
		Title:                           req.Title,
		Description:                     req.Description,
		Status:                          AuctionStatusDraft,
		StartsAt:                        req.StartsAt,
		EndsAt:                          req.EndsAt,
		BidStepCents:                    req.BidStepCents,
		PaymentDeadlineHours:            req.PaymentDeadlineHours,
		AntiSnipingEnabled:              req.AntiSnipingEnabled,
		AntiSnipingTriggerSeconds:       req.AntiSnipingTriggerSeconds,
		AntiSnipingExtensionSeconds:     req.AntiSnipingExtensionSeconds,
		MaxBidsPerUserPerLotPerMinute:   req.MaxBidsPerUserPerLotPerMinute,
		MaxRejectedBidsPerUserPerMinute: req.MaxRejectedBidsPerUserPerMinute,
		NoBidsPolicy:                    req.NoBidsPolicy,
		UnpaidWinnerPolicy:              req.UnpaidWinnerPolicy,
		IsPublic:                        req.IsPublic,
		ShowOnHomepage:                  req.ShowOnHomepage,
		HighlightInNav:                  req.HighlightInNav,
		BiddingEnabled:                  req.BiddingEnabled,
		CreatedBy:                       &userID,
		CreatedAt:                       time.Now(),
		UpdatedAt:                       time.Now(),
	}

	if err := h.repo.CreateEvent(r.Context(), event); err != nil {
		h.logger.Error("failed to create auction", "error", err)
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to create auction")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(event)
}

// POST /api/admin/auctions/{id}/lots
func (h *AdminHandler) CreateLot(w http.ResponseWriter, r *http.Request) {
	auctionIDStr := chi.URLParam(r, "id")
	auctionID, err := uuid.Parse(auctionIDStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid auction ID")
		return
	}

	var req AdminCreateLotRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid JSON format")
		return
	}

	lot := &AuctionLot{
		ID:                   uuid.New(),
		AuctionID:            auctionID,
		Title:                req.Title,
		Description:          req.Description,
		StartPriceCents:      req.StartPriceCents,
		BidStepCents:         req.BidStepCents,
		Status:               LotStatusDraft,
		CanRelaunch:          req.CanRelaunch,
		CanMoveToDirectSale:  req.CanMoveToDirectSale,
		DirectSalePriceCents: req.DirectSalePriceCents,
		AdminNote:            req.AdminNote,
		CreatedAt:            time.Now(),
		UpdatedAt:            time.Now(),
	}

	if len(req.Images) > 0 {
		lot.ImageURL = &req.Images[0].ImageURL
		for _, img := range req.Images {
			lot.Images = append(lot.Images, AuctionLotImage{
				ID:        uuid.New(),
				LotID:     lot.ID,
				ImageURL:  img.ImageURL,
				SortOrder: img.SortOrder,
				IsPrimary: img.IsPrimary,
				CreatedAt: time.Now(),
			})
		}
	}

	for _, attr := range req.Attributes {
		lot.Attributes = append(lot.Attributes, AuctionLotAttribute{
			ID:        uuid.New(),
			LotID:     lot.ID,
			Name:      attr.Name,
			Value:     attr.Value,
			SortOrder: attr.SortOrder,
		})
	}

	if err := h.repo.CreateLot(r.Context(), lot); err != nil {
		h.logger.Error("failed to create lot", "error", err)
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to create lot")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(lot)
}

// POST /api/admin/auctions/{id}/finalize
func (h *AdminHandler) FinalizeAuction(w http.ResponseWriter, r *http.Request) {
	userIDVal := r.Context().Value("userID")
	if userIDVal == nil {
		h.writeError(w, http.StatusForbidden, "forbidden", "Access denied")
		return
	}
	userID := userIDVal.(uuid.UUID)

	auctionIDStr := chi.URLParam(r, "id")
	auctionID, err := uuid.Parse(auctionIDStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid auction ID")
		return
	}

	if err := h.service.FinalizeAuction(r.Context(), auctionID, userID); err != nil {
		h.logger.Error("failed to finalize auction", "error", err)
		h.writeError(w, http.StatusBadRequest, "finalize_error", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

// GET /api/admin/auctions
func (h *AdminHandler) GetAuctions(w http.ResponseWriter, r *http.Request) {
	events, err := h.repo.ListAllEventsAdmin(r.Context())
	if err != nil {
		h.logger.Error("failed to list auctions", "error", err)
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to fetch auctions")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(events)
}

// GET /api/admin/auctions/{id}
func (h *AdminHandler) GetAuction(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid auction ID")
		return
	}

	event, err := h.repo.GetEventByIDWithLots(r.Context(), id)
	if err != nil {
		h.logger.Error("failed to get auction", "error", err)
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to fetch auction")
		return
	}
	if event == nil {
		h.writeError(w, http.StatusNotFound, "not_found", "Auction not found")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(event)
}

// PATCH /api/admin/auctions/{id}
func (h *AdminHandler) UpdateAuction(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid auction ID")
		return
	}

	var req AdminUpdateAuctionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid JSON format")
		return
	}

	err = h.service.UpdateEventAdmin(r.Context(), id, req)
	if err != nil {
		h.logger.Error("failed to update auction", "error", err)
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to update auction")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

// POST /api/admin/auctions/{id}/publish
func (h *AdminHandler) PublishAuction(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid auction ID")
		return
	}

	if err := h.service.UpdateEventStatus(r.Context(), id, AuctionStatusScheduled); err != nil {
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to publish auction")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

// POST /api/admin/auctions/{id}/pause
func (h *AdminHandler) PauseAuction(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid auction ID")
		return
	}

	if err := h.service.UpdateEventStatus(r.Context(), id, AuctionStatusPaused); err != nil {
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to pause auction")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

// POST /api/admin/auctions/{id}/resume
func (h *AdminHandler) ResumeAuction(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid auction ID")
		return
	}

	if err := h.service.UpdateEventStatus(r.Context(), id, AuctionStatusLive); err != nil {
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to resume auction")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

// POST /api/admin/auctions/{id}/cancel
func (h *AdminHandler) CancelAuction(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid auction ID")
		return
	}

	userIDVal := r.Context().Value("userID")
	adminID := userIDVal.(uuid.UUID)

	if err := h.service.CancelAuction(r.Context(), id, adminID); err != nil {
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to cancel auction")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

// PATCH /api/admin/auction-lots/{id}
func (h *AdminHandler) UpdateLot(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid lot ID")
		return
	}

	var req AdminUpdateLotRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid JSON format")
		return
	}

	err = h.service.UpdateLotAdmin(r.Context(), id, req)
	if err != nil {
		h.logger.Error("failed to update lot", "error", err)
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to update lot")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

// GET /api/admin/auction-lots/{id}/bids
func (h *AdminHandler) GetLotBids(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid lot ID")
		return
	}

	bids, err := h.repo.GetBidsByLotID(r.Context(), id)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to get bids")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(bids)
}

// POST /api/admin/auction-lots/{id}/mark-unpaid-review
func (h *AdminHandler) MarkLotUnpaid(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid lot ID")
		return
	}

	if err := h.service.UpdateLotStatus(r.Context(), id, LotStatusUnpaidManualReview); err != nil {
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to update lot status")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

// POST /api/admin/auction-lots/{id}/move-to-direct-sale
func (h *AdminHandler) MoveToDirectSale(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid lot ID")
		return
	}
	userIDVal := r.Context().Value("userID")
	adminID := userIDVal.(uuid.UUID)

	if err := h.service.MoveLotToDirectSale(r.Context(), id, adminID); err != nil {
		h.writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}
