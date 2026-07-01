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
	roleVal := r.Context().Value("role")
	if userIDVal == nil || roleVal == nil {
		h.writeError(w, http.StatusForbidden, "forbidden", "Access denied")
		return
	}
	userID := userIDVal.(uuid.UUID)
	role := roleVal.(string)
	if role != "admin" && role != "owner" {
		h.writeError(w, http.StatusForbidden, "forbidden", "Access denied")
		return
	}

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
	userIDVal := r.Context().Value("userID")
	roleVal := r.Context().Value("role")
	if userIDVal == nil || roleVal == nil {
		h.writeError(w, http.StatusForbidden, "forbidden", "Access denied")
		return
	}
	role := roleVal.(string)
	if role != "admin" && role != "owner" {
		h.writeError(w, http.StatusForbidden, "forbidden", "Access denied")
		return
	}

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
	roleVal := r.Context().Value("role")
	if userIDVal == nil || roleVal == nil {
		h.writeError(w, http.StatusForbidden, "forbidden", "Access denied")
		return
	}
	userID := userIDVal.(uuid.UUID)
	role := roleVal.(string)
	if role != "admin" && role != "owner" {
		h.writeError(w, http.StatusForbidden, "forbidden", "Access denied")
		return
	}

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

// STUBS FOR OTHER ENDPOINTS
func (h *AdminHandler) GetAuctions(w http.ResponseWriter, r *http.Request) {}
func (h *AdminHandler) GetAuction(w http.ResponseWriter, r *http.Request) {}
func (h *AdminHandler) UpdateAuction(w http.ResponseWriter, r *http.Request) {}
func (h *AdminHandler) PublishAuction(w http.ResponseWriter, r *http.Request) {}
func (h *AdminHandler) PauseAuction(w http.ResponseWriter, r *http.Request) {}
func (h *AdminHandler) ResumeAuction(w http.ResponseWriter, r *http.Request) {}
func (h *AdminHandler) CancelAuction(w http.ResponseWriter, r *http.Request) {}
func (h *AdminHandler) UpdateLot(w http.ResponseWriter, r *http.Request) {}
func (h *AdminHandler) GetLotBids(w http.ResponseWriter, r *http.Request) {}
func (h *AdminHandler) MarkLotUnpaid(w http.ResponseWriter, r *http.Request) {}
func (h *AdminHandler) MoveToDirectSale(w http.ResponseWriter, r *http.Request) {}
