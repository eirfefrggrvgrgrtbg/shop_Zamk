package auctions

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type CustomerHandler struct {
	repo    *Repository
	service *Service
	logger  *slog.Logger
}

func NewCustomerHandler(repo *Repository, service *Service, logger *slog.Logger) *CustomerHandler {
	return &CustomerHandler{
		repo:    repo,
		service: service,
		logger:  logger,
	}
}

func (h *CustomerHandler) writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]string{
			"code":    code,
			"message": message,
		},
	})
}

// POST /api/customer/auction-lots/{id}/bid
func (h *CustomerHandler) PlaceBid(w http.ResponseWriter, r *http.Request) {
	userIDVal := r.Context().Value("userID")
	roleVal := r.Context().Value("role")
	if userIDVal == nil || roleVal == nil {
		h.writeError(w, http.StatusForbidden, "forbidden", "Access denied")
		return
	}
	userID := userIDVal.(uuid.UUID)
	role := roleVal.(string)
	if role != "customer" {
		h.writeError(w, http.StatusForbidden, "forbidden", "Only customers can place bids")
		return
	}

	lotIDStr := chi.URLParam(r, "id")
	lotID, err := uuid.Parse(lotIDStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid lot ID")
		return
	}

	var req BidRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid JSON format")
		return
	}

	resp, err := h.service.PlaceBid(r.Context(), lotID, userID, req)
	if err != nil {
		if errors.Is(err, ErrAuctionNotStarted) || errors.Is(err, ErrAuctionEnded) ||
			errors.Is(err, ErrBiddingDisabled) || errors.Is(err, ErrLotUnavailable) ||
			errors.Is(err, ErrAlreadyLeading) || errors.Is(err, ErrInvalidBidAmount) ||
			errors.Is(err, ErrTooManyBids) || errors.Is(err, ErrDuplicateIdempotency) {
			h.writeError(w, http.StatusBadRequest, "bid_error", err.Error())
			return
		}
		h.logger.Error("failed to place bid", "error", err)
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to place bid")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

// GET /api/customer/auction-wins
func (h *CustomerHandler) GetAuctionWins(w http.ResponseWriter, r *http.Request) {
	userIDVal := r.Context().Value("userID")
	if userIDVal == nil {
		h.writeError(w, http.StatusForbidden, "forbidden", "Access denied")
		return
	}
	userID := userIDVal.(uuid.UUID)

	lots, err := h.service.GetCustomerWins(r.Context(), userID)
	if err != nil {
		h.logger.Error("failed to get customer wins", "error", err)
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to get wins")
		return
	}
	if lots == nil {
		lots = []AuctionLot{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(lots)
}

// POST /api/customer/auction-lots/{id}/create-order
func (h *CustomerHandler) CreateOrderForLot(w http.ResponseWriter, r *http.Request) {
	userIDVal := r.Context().Value("userID")
	if userIDVal == nil {
		h.writeError(w, http.StatusForbidden, "forbidden", "Access denied")
		return
	}
	userID := userIDVal.(uuid.UUID)

	lotIDStr := chi.URLParam(r, "id")
	lotID, err := uuid.Parse(lotIDStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid lot ID")
		return
	}

	result, err := h.service.CreateOrderForLot(r.Context(), lotID, userID)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "order_error", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}
