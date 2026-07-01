package auctions

import (
	"encoding/json"
	"log/slog"
	"net/http"
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

// STUBS
func (h *PublicHandler) GetHomepageAuctions(w http.ResponseWriter, r *http.Request) {}
func (h *PublicHandler) GetNavHighlightAuctions(w http.ResponseWriter, r *http.Request) {}
func (h *PublicHandler) GetAuctionLots(w http.ResponseWriter, r *http.Request) {}
func (h *PublicHandler) GetAuctionLot(w http.ResponseWriter, r *http.Request) {}
