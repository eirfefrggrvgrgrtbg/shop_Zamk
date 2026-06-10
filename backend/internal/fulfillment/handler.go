package fulfillment

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/http/pagination"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/staff"
	"github.com/google/uuid"
)

type Handler struct {
	svc       *Service
	auditRepo *staff.AuditRepository
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// WithAudit attaches an audit repository for fire-and-forget audit logging.
func (h *Handler) WithAudit(ar *staff.AuditRepository) *Handler {
	h.auditRepo = ar
	return h
}

func (h *Handler) CreateShipment(w http.ResponseWriter, r *http.Request) {
	val := r.Context().Value("userID")
	if val == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	adminID := val.(uuid.UUID)

	orderIDStr := chi.URLParam(r, "id")
	orderID, err := uuid.Parse(orderIDStr)
	if err != nil {
		http.Error(w, "invalid order id", http.StatusBadRequest)
		return
	}

	var req CreateShipmentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	shipment, err := h.svc.CreateShipment(r.Context(), adminID, orderID, req)
	if err != nil {
		if errors.Is(err, ErrOrderNotPaid) || errors.Is(err, ErrShipmentExists) {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(shipment)
}

func (h *Handler) ListAdminShipments(w http.ResponseWriter, r *http.Request) {
	page := pagination.FromRequest(r)
	shipments, err := h.svc.ListAdminShipments(r.Context(), page.Limit, page.Offset)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(shipments)
}

func (h *Handler) GetAdminShipment(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "invalid shipment id", http.StatusBadRequest)
		return
	}

	shipment, err := h.svc.GetAdminShipment(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrShipmentNotFound) {
			http.Error(w, "shipment not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(shipment)
}

func (h *Handler) UpdateShipmentStatus(w http.ResponseWriter, r *http.Request) {
	val := r.Context().Value("userID")
	if val == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	adminID := val.(uuid.UUID)

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "invalid shipment id", http.StatusBadRequest)
		return
	}

	var req UpdateShipmentStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := h.svc.UpdateShipmentStatus(r.Context(), adminID, id, req); err != nil {
		if errors.Is(err, ErrInvalidStatus) {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if errors.Is(err, ErrShipmentNotFound) {
			http.Error(w, "shipment not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if h.auditRepo != nil {
		sid := id
		actorID := adminID
		newStatus := req.Status
		go func() {
			_ = h.auditRepo.RecordAudit(context.Background(), staff.AuditEvent{
				ActorUserID: actorID,
				Action:      "shipment.status_update",
				EntityType:  "shipment",
				EntityID:    &sid,
				Metadata:    staff.SanitizeMetadata(map[string]any{"newStatus": newStatus}),
			})
		}()
	}

	w.WriteHeader(http.StatusOK)
}

func (h *Handler) GetCustomerShipment(w http.ResponseWriter, r *http.Request) {
	val := r.Context().Value("userID")
	if val == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	userID := val.(uuid.UUID)

	orderIDStr := chi.URLParam(r, "id")
	orderID, err := uuid.Parse(orderIDStr)
	if err != nil {
		http.Error(w, "invalid order id", http.StatusBadRequest)
		return
	}

	shipment, err := h.svc.GetCustomerShipment(r.Context(), userID, orderID)
	if err != nil {
		if errors.Is(err, ErrUnauthorized) || errors.Is(err, ErrShipmentNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(shipment)
}

// Seller visibility struct
type SellerShipmentResponse struct {
	Status         string    `json:"status"`
	Carrier        *string   `json:"carrier"`
	TrackingNumber *string   `json:"trackingNumber"`
	TrackingUrl    *string   `json:"trackingUrl"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

func (h *Handler) GetSellerShipment(w http.ResponseWriter, r *http.Request) {
	val := r.Context().Value("userID")
	if val == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	sellerID := val.(uuid.UUID)

	orderIDStr := chi.URLParam(r, "id")
	orderID, err := uuid.Parse(orderIDStr)
	if err != nil {
		http.Error(w, "invalid order id", http.StatusBadRequest)
		return
	}

	shipment, err := h.svc.GetSellerShipment(r.Context(), sellerID, orderID)
	if err != nil {
		if errors.Is(err, ErrUnauthorized) || errors.Is(err, ErrShipmentNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	resp := SellerShipmentResponse{
		Status:         shipment.Status,
		Carrier:        shipment.Carrier,
		TrackingNumber: shipment.TrackingNumber,
		TrackingUrl:    shipment.TrackingUrl,
		UpdatedAt:      shipment.UpdatedAt,
	}
	json.NewEncoder(w).Encode(resp)
}
