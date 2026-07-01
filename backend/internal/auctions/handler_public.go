package auctions

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type PublicHandler struct {
	repo    *Repository
	service *Service
	logger  *slog.Logger
}

func NewPublicHandler(repo *Repository, service *Service, logger *slog.Logger) *PublicHandler {
	return &PublicHandler{
		repo:    repo,
		service: service,
		logger:  logger,
	}
}

func (h *PublicHandler) writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]string{
			"code":    code,
			"message": message,
		},
	})
}

// GET /api/public/auctions/active
func (h *PublicHandler) GetActiveAuctions(w http.ResponseWriter, r *http.Request) {
	events, err := h.repo.ListActiveAuctions(r.Context())
	if err != nil {
		h.logger.Error("failed to list active auctions", "error", err)
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to list active auctions")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(events)
}

// GET /api/public/auctions/homepage
func (h *PublicHandler) GetHomepageAuctions(w http.ResponseWriter, r *http.Request) {
	events, err := h.repo.ListHomepageAuctions(r.Context())
	if err != nil {
		h.logger.Error("failed to list homepage auctions", "error", err)
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to list homepage auctions")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(events)
}

// GET /api/public/auctions/nav-highlight
func (h *PublicHandler) GetNavHighlightAuctions(w http.ResponseWriter, r *http.Request) {
	events, err := h.repo.ListNavHighlightAuctions(r.Context())
	if err != nil {
		h.logger.Error("failed to list nav highlight auctions", "error", err)
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to list nav highlight auctions")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(events)
}

// GET /api/public/auctions/{id}/lots
func (h *PublicHandler) GetAuctionLots(w http.ResponseWriter, r *http.Request) {
	auctionIDStr := chi.URLParam(r, "id")
	auctionID, err := uuid.Parse(auctionIDStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid auction ID")
		return
	}

	lots, err := h.repo.GetLotsByAuctionID(r.Context(), auctionID)
	if err != nil {
		h.logger.Error("failed to list auction lots", "error", err)
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to fetch lots")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(lots)
}

// GET /api/public/auction-lots/{id}
func (h *PublicHandler) GetAuctionLot(w http.ResponseWriter, r *http.Request) {
	lotIDStr := chi.URLParam(r, "id")
	lotID, err := uuid.Parse(lotIDStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid lot ID")
		return
	}

	lot, err := h.repo.GetLotByIDWithDetails(r.Context(), lotID)
	if err != nil {
		h.logger.Error("failed to get auction lot", "error", err)
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to fetch lot")
		return
	}
	if lot == nil {
		h.writeError(w, http.StatusNotFound, "not_found", "Lot not found")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(lot)
}
