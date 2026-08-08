package payouts

import (
	"encoding/json"
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
func (h *Handler) CreatePayoutBatch(w http.ResponseWriter, r *http.Request) {
	// used for manual testing/E2E trigger
	sellerIDStr := r.URL.Query().Get("seller_id")
	if sellerIDStr == "" {
		h.writeError(w, 400, "bad_request", "seller_id required")
		return
	}
	sellerID, err := uuid.Parse(sellerIDStr)
	if err != nil {
		h.writeError(w, 400, "bad_request", "invalid seller_id")
		return
	}
	
	batch, err := h.svc.CreatePayoutBatchForSeller(r.Context(), sellerID)
	if err != nil {
		h.writeError(w, 500, "error", err.Error())
		return
	}
	
	json.NewEncoder(w).Encode(batch)
}

func (h *Handler) ProcessPayoutBatch(w http.ResponseWriter, r *http.Request) {
	batchID, _ := uuid.Parse(chi.URLParam(r, "id"))
	if err := h.svc.ProcessPayoutBatch(r.Context(), batchID); err != nil {
		h.writeError(w, 500, "error", err.Error())
		return
	}
	w.WriteHeader(200)
}

func (h *Handler) HoldPayoutBatch(w http.ResponseWriter, r *http.Request) {
	batchID, _ := uuid.Parse(chi.URLParam(r, "id"))
	if err := h.svc.HoldPayoutBatch(r.Context(), batchID); err != nil {
		h.writeError(w, 500, "error", err.Error())
		return
	}
	w.WriteHeader(200)
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
