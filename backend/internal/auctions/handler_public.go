package auctions

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

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

	for i := range events {
		lots, _ := h.repo.GetPublicLotsByAuctionID(r.Context(), events[i].ID)
		if lots == nil {
			lots = []AuctionLot{} // Ensure JSON sends [] instead of null
		}
		events[i].Lots = lots
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

	for i := range events {
		lots, _ := h.repo.GetPublicLotsByAuctionID(r.Context(), events[i].ID)
		if lots == nil {
			lots = []AuctionLot{}
		}
		events[i].Lots = lots
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

	for i := range events {
		lots, _ := h.repo.GetPublicLotsByAuctionID(r.Context(), events[i].ID)
		if lots == nil {
			lots = []AuctionLot{}
		}
		events[i].Lots = lots
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

	lots, err := h.repo.GetPublicLotsByAuctionID(r.Context(), auctionID)
	if err != nil {
		h.logger.Error("failed to list auction lots", "error", err)
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to fetch lots")
		return
	}
	if lots == nil {
		lots = []AuctionLot{}
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
	
	if lot.Status == LotStatusDraft || lot.Status == LotStatusCancelled {
		h.writeError(w, http.StatusNotFound, "not_found", "Lot not found")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(lot)
}

// GET /api/public/auctions/{id}/stream
func (h *PublicHandler) StreamAuction(w http.ResponseWriter, r *http.Request) {
	auctionIDStr := chi.URLParam(r, "id")
	auctionID, err := uuid.Parse(auctionIDStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid auction ID")
		return
	}

	event, err := h.repo.GetEventByID(r.Context(), auctionID)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to get auction")
		return
	}
	if event == nil || !event.IsPublic || event.Status == AuctionStatusDraft || event.Status == AuctionStatusCancelled {
		h.writeError(w, http.StatusNotFound, "not_found", "Auction not found")
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		h.writeError(w, http.StatusInternalServerError, "streaming_unsupported", "Streaming unsupported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := h.service.hub.Subscribe(auctionID)
	defer h.service.hub.Unsubscribe(auctionID, ch)

	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()

	// Initial heartbeat
	fmt.Fprintf(w, "event: ping\ndata: {}\n\n")
	flusher.Flush()

	for {
		select {
		case <-r.Context().Done():
			return
		case ev := <-ch:
			data, err := json.Marshal(ev)
			if err == nil {
				fmt.Fprintf(w, "event: %s\ndata: %s\n\n", ev.EventType, string(data))
				flusher.Flush()
			}
		case <-ticker.C:
			fmt.Fprintf(w, "event: ping\ndata: {}\n\n")
			flusher.Flush()
		}
	}
}
