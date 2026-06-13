package fulfillment

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/http/pagination"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/staff"
)

type FulfillmentListResponse struct {
	Items      []Fulfillment `json:"items"`
	TotalCount int           `json:"totalCount"`
}

func (h *Handler) writeError(w http.ResponseWriter, status int, code, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]string{
			"code":    code,
			"message": msg,
		},
	})
}

func (h *Handler) ListSellerFulfillments(w http.ResponseWriter, r *http.Request) {
	val := r.Context().Value("userID")
	if val == nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}
	sellerID := val.(uuid.UUID)

	page := pagination.FromRequest(r)
	statusParam := r.URL.Query().Get("status")
	var status *string
	if statusParam != "" {
		status = &statusParam
	}

	fulfillments, err := h.svc.ListSellerFulfillments(r.Context(), sellerID, page.Limit, page.Offset, status)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(FulfillmentListResponse{
		Items:      fulfillments,
		TotalCount: len(fulfillments), // Just returning length as total for simplicity in this phase
	})
}

func (h *Handler) GetSellerFulfillment(w http.ResponseWriter, r *http.Request) {
	val := r.Context().Value("userID")
	if val == nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}
	sellerID := val.(uuid.UUID)

	fulfillmentID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid ID")
		return
	}

	f, err := h.svc.GetSellerFulfillment(r.Context(), sellerID, fulfillmentID)
	if err != nil {
		if errors.Is(err, ErrFulfillmentNotFound) {
			h.writeError(w, http.StatusNotFound, "not_found", "Not found")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(f)
}

func (h *Handler) MarkSellerFulfillmentAssembling(w http.ResponseWriter, r *http.Request) {
	val := r.Context().Value("userID")
	if val == nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}
	userID := val.(uuid.UUID)

	fulfillmentID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid ID")
		return
	}

	err = h.svc.MarkSellerFulfillmentAssembling(r.Context(), userID, fulfillmentID)
	if err != nil {
		if err.Error() == "Сборка ещё не оплачена." || err.Error() == "Сборка уже отправлена или завершена." || err.Error() == "Сборку нельзя перевести в этот статус." {
			h.writeError(w, http.StatusBadRequest, "bad_request", err.Error())
			return
		}
		if errors.Is(err, ErrFulfillmentNotFound) {
			h.writeError(w, http.StatusNotFound, "not_found", "Not found")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	if h.auditRepo != nil {
		fid := fulfillmentID
		_ = h.auditRepo.RecordAudit(r.Context(), staff.AuditEvent{
			ActorUserID: userID,
			Action:      "fulfillment.status_update",
			EntityType:  "order_fulfillment",
			EntityID:    &fid,
			Metadata:    staff.SanitizeMetadata(map[string]any{"fromStatus": "paid", "toStatus": "assembling", "actorRole": "seller"}),
		})
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (h *Handler) MarkSellerFulfillmentPacked(w http.ResponseWriter, r *http.Request) {
	val := r.Context().Value("userID")
	if val == nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}
	userID := val.(uuid.UUID)

	fulfillmentID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid ID")
		return
	}

	err = h.svc.MarkSellerFulfillmentPacked(r.Context(), userID, fulfillmentID)
	if err != nil {
		if err.Error() == "Сборка ещё не оплачена." || err.Error() == "Сборка уже отправлена или завершена." || err.Error() == "Сборку нельзя перевести в этот статус." {
			h.writeError(w, http.StatusBadRequest, "bad_request", err.Error())
			return
		}
		if errors.Is(err, ErrFulfillmentNotFound) {
			h.writeError(w, http.StatusNotFound, "not_found", "Not found")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	if h.auditRepo != nil {
		fid := fulfillmentID
		_ = h.auditRepo.RecordAudit(r.Context(), staff.AuditEvent{
			ActorUserID: userID,
			Action:      "fulfillment.status_update",
			EntityType:  "order_fulfillment",
			EntityID:    &fid,
			Metadata:    staff.SanitizeMetadata(map[string]any{"fromStatus": "assembling", "toStatus": "packed", "actorRole": "seller"}),
		})
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (h *Handler) ListAdminFulfillments(w http.ResponseWriter, r *http.Request) {
	page := pagination.FromRequest(r)
	statusParam := r.URL.Query().Get("status")
	var status *string
	if statusParam != "" {
		status = &statusParam
	}

	fulfillments, err := h.svc.ListAdminFulfillments(r.Context(), page.Limit, page.Offset, status)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(FulfillmentListResponse{
		Items:      fulfillments,
		TotalCount: len(fulfillments),
	})
}

func (h *Handler) GetAdminFulfillment(w http.ResponseWriter, r *http.Request) {
	fulfillmentID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid ID")
		return
	}

	f, err := h.svc.GetAdminFulfillment(r.Context(), fulfillmentID)
	if err != nil {
		if errors.Is(err, ErrFulfillmentNotFound) {
			h.writeError(w, http.StatusNotFound, "not_found", "Not found")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(f)
}

func (h *Handler) GetAdminOrderFulfillments(w http.ResponseWriter, r *http.Request) {
	orderID, err := uuid.Parse(chi.URLParam(r, "orderId"))
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid Order ID")
		return
	}

	fulfillments, err := h.svc.GetOrderFulfillments(r.Context(), orderID)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(fulfillments)
}

func (h *Handler) GetCustomerOrderFulfillments(w http.ResponseWriter, r *http.Request) {
	val := r.Context().Value("userID")
	if val == nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}
	customerID := val.(uuid.UUID)

	orderID, err := uuid.Parse(chi.URLParam(r, "orderId"))
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid Order ID")
		return
	}

	fulfillments, err := h.svc.CustomerGetOrderFulfillments(r.Context(), customerID, orderID)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	var safeFulfillments []CustomerFulfillmentResponse
	for _, f := range fulfillments {
		var shipmentIDStr *string
		if f.ShipmentID != nil {
			s := f.ShipmentID.String()
			shipmentIDStr = &s
		}

		safeFulfillments = append(safeFulfillments, CustomerFulfillmentResponse{
			ID:             f.ID.String(),
			OrderID:        f.OrderID.String(),
			SellerID:       f.SellerID.String(),
			SellerName:     f.SellerName,
			Status:         f.Status,
			CreatedAt:      f.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			UpdatedAt:      f.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
			ShipmentID:     shipmentIDStr,
			ShipmentStatus: f.ShipmentStatus,
			Items:          f.Items,
		})
	}
	if safeFulfillments == nil {
		safeFulfillments = make([]CustomerFulfillmentResponse, 0)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(safeFulfillments)
}
