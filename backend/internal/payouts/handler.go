package payouts

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/http/pagination"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/staff"
)

type Handler struct {
	service   *Service
	auditRepo *staff.AuditRepository
	staffSvc  *staff.Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// WithAudit attaches an audit repository for fire-and-forget audit logging.
func (h *Handler) WithAudit(ar *staff.AuditRepository) *Handler {
	h.auditRepo = ar
	return h
}

// WithStaffSvc attaches a staff service for handler-level permission checks.
func (h *Handler) WithStaffSvc(svc *staff.Service) *Handler {
	h.staffSvc = svc
	return h
}

func (h *Handler) writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{
		"error":   code,
		"message": message,
	})
}

// Seller Endpoints

func (h *Handler) GetSellerBalance(w http.ResponseWriter, r *http.Request) {
	val := r.Context().Value("userID")
	if val == nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}
	userID := val.(uuid.UUID)

	balance, err := h.service.GetSellerBalance(r.Context(), userID)
	if err != nil {
		if errors.Is(err, ErrUnauthorized) {
			h.writeError(w, http.StatusForbidden, "forbidden", "Must be a seller")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to get seller balance")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(balance)
}

func (h *Handler) ListSellerPayouts(w http.ResponseWriter, r *http.Request) {
	val := r.Context().Value("userID")
	if val == nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}
	userID := val.(uuid.UUID)

	page := pagination.FromRequest(r)
	payouts, err := h.service.ListSellerPayouts(r.Context(), userID, page.Limit, page.Offset)
	if err != nil {
		if errors.Is(err, ErrUnauthorized) {
			h.writeError(w, http.StatusForbidden, "forbidden", "Must be a seller")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to list payouts")
		return
	}

	resp := PayoutListResponse{Items: payouts, TotalCount: len(payouts)}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) RequestPayout(w http.ResponseWriter, r *http.Request) {
	val := r.Context().Value("userID")
	if val == nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}
	userID := val.(uuid.UUID)

	var req PayoutRequestDto
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid JSON payload")
		return
	}
	if req.AmountCents <= 0 {
		h.writeError(w, http.StatusBadRequest, "invalid_amount", "Payout amount must be greater than zero")
		return
	}

	payout, err := h.service.RequestPayout(r.Context(), userID, req)
	if err != nil {
		if errors.Is(err, ErrInsufficientBalance) {
			h.writeError(w, http.StatusConflict, "insufficient_balance", "Insufficient balance for payout")
			return
		}
		if errors.Is(err, ErrUnauthorized) {
			h.writeError(w, http.StatusForbidden, "forbidden", "Must be a seller")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to request payout")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(PayoutResponse{Payout: *payout})
}

// Admin Endpoints

func (h *Handler) ListAdminPayouts(w http.ResponseWriter, r *http.Request) {
	page := pagination.FromRequest(r)
	payouts, err := h.service.ListAdminPayouts(r.Context(), page.Limit, page.Offset)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to list payouts")
		return
	}

	resp := PayoutListResponse{Items: payouts, TotalCount: len(payouts)}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) GetAdminPayout(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	payoutID, err := uuid.Parse(idStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid payout ID")
		return
	}

	payout, err := h.service.GetAdminPayout(r.Context(), payoutID)
	if err != nil {
		if errors.Is(err, ErrPayoutNotFound) {
			h.writeError(w, http.StatusNotFound, "not_found", "Payout not found")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to get payout")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(PayoutResponse{Payout: *payout})
}

func (h *Handler) UpdateAdminPayoutStatus(w http.ResponseWriter, r *http.Request) {
	val := r.Context().Value("userID")
	if val == nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}
	adminUserID := val.(uuid.UUID)

	idStr := chi.URLParam(r, "id")
	payoutID, err := uuid.Parse(idStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid payout ID")
		return
	}

	var req UpdatePayoutStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid JSON payload")
		return
	}

	if req.Status != "approved" && req.Status != "rejected" && req.Status != "paid" && req.Status != "cancelled" {
		h.writeError(w, http.StatusBadRequest, "invalid_status", "Invalid status")
		return
	}

	// Handler-level dynamic permission check based on target status
	if h.staffSvc != nil {
		var requiredPerm string
		switch req.Status {
		case "approved":
			requiredPerm = "payouts.approve"
		case "rejected":
			requiredPerm = "payouts.reject"
		case "paid", "mark_paid":
			requiredPerm = "payouts.mark_paid"
		default:
			requiredPerm = "payouts.approve"
		}
		ok, permErr := h.staffSvc.HasPermission(r.Context(), adminUserID, requiredPerm)
		if permErr != nil {
			h.writeError(w, http.StatusInternalServerError, "internal_error", "Permission check failed")
			return
		}
		if !ok {
			h.writeError(w, http.StatusForbidden, "insufficient_permissions", "Недостаточно прав")
			return
		}
	}

	err = h.service.UpdatePayoutStatus(r.Context(), payoutID, adminUserID, req)
	if err != nil {
		if errors.Is(err, ErrInvalidPayoutStatus) {
			h.writeError(w, http.StatusBadRequest, "invalid_status_transition", "Invalid status transition")
			return
		}
		if errors.Is(err, ErrPayoutNotFound) {
			h.writeError(w, http.StatusNotFound, "not_found", "Payout not found")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to update payout status")
		return
	}

	if h.auditRepo != nil {
		actorEmail, _ := r.Context().Value("email").(string)
		actorRole, _ := r.Context().Value("role").(string)
		pid := payoutID
		actorID := adminUserID
		go func() {
			_ = h.auditRepo.RecordAudit(context.Background(), staff.AuditEvent{
				ActorUserID: actorID,
				ActorEmail:  actorEmail,
				ActorRole:   actorRole,
				Action:      "payout.status_update",
				EntityType:  "payout",
				EntityID:    &pid,
				Metadata:    staff.SanitizeMetadata(map[string]any{"newStatus": req.Status}),
			})
		}()
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) TriggerAvailability(w http.ResponseWriter, r *http.Request) {
	// For E2E testing ONLY
	var req struct {
		DaysToSimulate int `json:"daysToSimulate"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid JSON")
		return
	}

	simulatedTime := time.Now().AddDate(0, 0, req.DaysToSimulate)
	processed, err := h.service.MakeSellerFundsAvailable(r.Context(), simulatedTime, 1000)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to trigger availability")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"processed": processed,
	})
}
