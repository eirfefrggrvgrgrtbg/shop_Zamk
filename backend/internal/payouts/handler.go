package payouts

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/auth"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/staff"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type Handler struct {
	svc      *Service
	auditSvc *staff.AuditRepository
	staffSvc *staff.Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) WithAudit(ar *staff.AuditRepository) *Handler {
	h.auditSvc = ar
	return h
}
func (h *Handler) WithStaffSvc(svc *staff.Service) *Handler {
	h.staffSvc = svc
	return h
}

func (h *Handler) writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"code": code, "message": message})
}

// --- Seller Handlers ---
func (h *Handler) GetSellerBalance(w http.ResponseWriter, r *http.Request) {
	sellerID, err := auth.GetSellerID(r.Context())
	if err != nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}
	resp, err := h.svc.GetSellerBalance(r.Context(), sellerID)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) ListSellerLedger(w http.ResponseWriter, r *http.Request) {
	sellerID, err := auth.GetSellerID(r.Context())
	if err != nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}
	
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	limit := 50
	offset := (page - 1) * limit

	list, count, err := h.svc.ListSellerLedger(r.Context(), sellerID, limit, offset)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(LedgerListResponse{Items: list, TotalCount: count})
}

func (h *Handler) ListSellerPayouts(w http.ResponseWriter, r *http.Request) {
	sellerID, err := auth.GetSellerID(r.Context())
	if err != nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	limit := 50
	offset := (page - 1) * limit

	list, count, err := h.svc.ListSellerPayoutBatches(r.Context(), sellerID, limit, offset)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(PayoutBatchListResponse{Items: list, TotalCount: count})
}

// Admin Handlers
func (h *Handler) writeProcessPayoutError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrPayoutBatchNotFound):
		h.writeError(w, http.StatusNotFound, "not_found", "payout batch not found")
	case errors.Is(err, ErrPayoutBatchAlreadyPaid):
		h.writeError(w, http.StatusConflict, "already_paid", "payout batch is already paid")
	case errors.Is(err, ErrPayoutBatchHasOpenReturns):
		h.writeError(w, http.StatusBadRequest, "open_return_not_covered", "payout batch contains earnings with uncovered open returns")
	case errors.Is(err, ErrPayoutBatchNotProcessable):
		h.writeError(w, http.StatusBadRequest, "not_processable", err.Error())
	default:
		h.writeError(w, http.StatusBadRequest, "process_failed", err.Error())
	}
}

func (h *Handler) writeHoldPayoutError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrPayoutBatchNotFound):
		h.writeError(w, http.StatusNotFound, "not_found", "payout batch not found")
	case errors.Is(err, ErrPayoutBatchAlreadyPaid):
		h.writeError(w, http.StatusConflict, "already_paid", "payout batch is already paid")
	case errors.Is(err, ErrPayoutBatchNotHoldable):
		h.writeError(w, http.StatusBadRequest, "not_holdable", err.Error())
	default:
		h.writeError(w, http.StatusBadRequest, "hold_failed", err.Error())
	}
}

func (h *Handler) CreatePayoutBatch(w http.ResponseWriter, r *http.Request) {
	sellerIDStr := r.URL.Query().Get("seller_id")
	if sellerIDStr == "" {
		h.writeError(w, http.StatusBadRequest, "bad_request", "seller_id required")
		return
	}
	sellerID, err := uuid.Parse(sellerIDStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "bad_request", "invalid seller_id")
		return
	}

	batch, err := h.svc.CreatePayoutBatchForSeller(r.Context(), sellerID)
	if err != nil {
		if errors.Is(err, ErrNothingToPay) {
			h.writeError(w, http.StatusBadRequest, "nothing_to_pay", "no positive available balance")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "error", "failed to create payout batch")
		return
	}

	if h.auditSvc != nil {
		adminID := auth.GetUserID(r.Context())
		_ = h.auditSvc.RecordAudit(r.Context(), staff.AuditEvent{
			ActorUserID: adminID,
			Action:      "payout_batch.created",
			EntityType:  "payout_batch",
			EntityID:    &batch.ID,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(batch)
}

func (h *Handler) ListAdminPayouts(w http.ResponseWriter, r *http.Request) {
	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		if val, err := strconv.Atoi(l); err == nil && val > 0 {
			limit = val
			if limit > 100 {
				limit = 100
			}
		} else {
			h.writeError(w, http.StatusBadRequest, "invalid_limit", "invalid limit")
			return
		}
	}
	offset := 0
	if o := r.URL.Query().Get("offset"); o != "" {
		if val, err := strconv.Atoi(o); err == nil && val >= 0 {
			offset = val
		} else {
			h.writeError(w, http.StatusBadRequest, "invalid_offset", "invalid offset")
			return
		}
	} else if p := r.URL.Query().Get("page"); p != "" {
		if val, err := strconv.Atoi(p); err == nil && val >= 1 {
			offset = (val - 1) * limit
		}
	}

	filter := PayoutFilter{
		Q:        r.URL.Query().Get("q"),
		SellerID: r.URL.Query().Get("sellerId"),
		Status:   r.URL.Query().Get("status"),
	}

	if filter.Status != "" && !AdminPayoutStatuses[filter.Status] {
		h.writeError(w, http.StatusBadRequest, "invalid_status", "invalid status filter")
		return
	}

	if filter.SellerID != "" {
		if _, err := uuid.Parse(filter.SellerID); err != nil {
			h.writeError(w, http.StatusBadRequest, "invalid_seller_id", "invalid sellerId filter")
			return
		}
	}

	items, total, err := h.svc.ListAdminPayoutBatches(r.Context(), filter, limit, offset)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "internal_error", "failed to list payouts")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"items":      items,
		"totalCount": total,
	})
}

func (h *Handler) GetAdminPayoutDetail(w http.ResponseWriter, r *http.Request) {
	batchID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "bad_request", "invalid payout batch id")
		return
	}

	detail, err := h.svc.GetAdminPayoutBatchDetail(r.Context(), batchID)
	if err != nil {
		if errors.Is(err, ErrPayoutBatchNotFound) {
			h.writeError(w, http.StatusNotFound, "not_found", "payout batch not found")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", "failed to get payout detail")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(detail)
}

func (h *Handler) ProcessPayoutBatch(w http.ResponseWriter, r *http.Request) {
	batchID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "bad_request", "invalid payout batch id")
		return
	}

	if err := h.svc.ProcessPayoutBatch(r.Context(), batchID); err != nil {
		h.writeProcessPayoutError(w, err)
		return
	}

	if h.auditSvc != nil {
		adminID := auth.GetUserID(r.Context())
		_ = h.auditSvc.RecordAudit(r.Context(), staff.AuditEvent{
			ActorUserID: adminID,
			Action:      "payout_batch.processed",
			EntityType:  "payout_batch",
			EntityID:    &batchID,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "paid"})
}

func (h *Handler) HoldPayoutBatch(w http.ResponseWriter, r *http.Request) {
	batchID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "bad_request", "invalid payout batch id")
		return
	}

	if err := h.svc.HoldPayoutBatch(r.Context(), batchID); err != nil {
		h.writeHoldPayoutError(w, err)
		return
	}

	if h.auditSvc != nil {
		adminID := auth.GetUserID(r.Context())
		_ = h.auditSvc.RecordAudit(r.Context(), staff.AuditEvent{
			ActorUserID: adminID,
			Action:      "payout_batch.held",
			EntityType:  "payout_batch",
			EntityID:    &batchID,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "held"})
}

type UpdatePayoutStatusRequest struct {
	Status  string  `json:"status"`
	Comment *string `json:"comment,omitempty"`
}

func (h *Handler) UpdatePayoutBatchStatus(w http.ResponseWriter, r *http.Request) {
	batchID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "bad_request", "invalid payout batch id")
		return
	}

	var req UpdatePayoutStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "bad_request", "invalid payload")
		return
	}

	switch req.Status {
	case "paid":
		// Only staff with payouts.mark_paid are permitted to transition a batch to paid.
		// payouts.update alone is strictly insufficient.
		hasMarkPaid := false
		if h.staffSvc != nil {
			adminID := auth.GetUserID(r.Context())
			if ok, err := h.staffSvc.HasPermission(r.Context(), adminID, "payouts.mark_paid"); err == nil && ok {
				hasMarkPaid = true
			}
		} else if granted, ok := r.Context().Value("grantedPermissions").(map[string]bool); ok && granted["payouts.mark_paid"] {
			hasMarkPaid = true
		}
		if !hasMarkPaid {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"error":   "insufficient_permissions",
				"code":    "insufficient_permissions",
				"message": "Недостаточно прав: для отметки выплаты как paid требуется право payouts.mark_paid",
			})
			return
		}

		if err := h.svc.ProcessPayoutBatch(r.Context(), batchID); err != nil {
			h.writeProcessPayoutError(w, err)
			return
		}
		if h.auditSvc != nil {
			adminID := auth.GetUserID(r.Context())
			_ = h.auditSvc.RecordAudit(r.Context(), staff.AuditEvent{
				ActorUserID: adminID,
				Action:      "payout_batch.processed",
				EntityType:  "payout_batch",
				EntityID:    &batchID,
			})
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "paid"})
	case "held":
		if err := h.svc.HoldPayoutBatch(r.Context(), batchID); err != nil {
			h.writeHoldPayoutError(w, err)
			return
		}
		if h.auditSvc != nil {
			adminID := auth.GetUserID(r.Context())
			_ = h.auditSvc.RecordAudit(r.Context(), staff.AuditEvent{
				ActorUserID: adminID,
				Action:      "payout_batch.held",
				EntityType:  "payout_batch",
				EntityID:    &batchID,
			})
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "held"})
	default:
		h.writeError(w, http.StatusBadRequest, "invalid_status_transition", "direct status transition not allowed; supported actions: paid, held")
	}
}

func (h *Handler) GetAdminSellerCommissionHistory(w http.ResponseWriter, r *http.Request) {
	sellerID, _ := uuid.Parse(chi.URLParam(r, "id"))
	history, err := h.svc.ListCommissionHistory(r.Context(), sellerID)
	if err != nil {
		h.writeError(w, 500, "error", err.Error())
		return
	}
	json.NewEncoder(w).Encode(history)
}

func (h *Handler) SetAdminSellerCommission(w http.ResponseWriter, r *http.Request) {
	sellerID, _ := uuid.Parse(chi.URLParam(r, "id"))
	
	var req AdminSellerCommissionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, 400, "bad_request", "invalid payload")
		return
	}
	
	adminID := auth.GetUserID(r.Context()) // Assuming admin is logged in
	
	if err := h.svc.SetCommissionRate(r.Context(), sellerID, req, adminID); err != nil {
		h.writeError(w, 500, "error", err.Error())
		return
	}
	w.WriteHeader(200)
}

func (h *Handler) GetAdminPayoutSummary(w http.ResponseWriter, r *http.Request) {
	summary, err := h.svc.GetAdminPayoutSummary(r.Context())
	if err != nil {
		h.writeError(w, 500, "error", err.Error())
		return
	}
	json.NewEncoder(w).Encode(summary)
}
