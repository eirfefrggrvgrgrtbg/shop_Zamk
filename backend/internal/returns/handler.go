package returns

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/auth"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/http/pagination"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/payments"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/staff"
	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"path/filepath"
	"strings"
)

type Handler struct {
	service   *Service
	validator *validator.Validate
	auditRepo *staff.AuditRepository
	appEnv    string
}

func NewHandler(service *Service, appEnv ...string) *Handler {
	env := "development"
	if len(appEnv) > 0 && appEnv[0] != "" {
		env = appEnv[0]
	}
	return &Handler{
		service:   service,
		validator: validator.New(),
		appEnv:    env,
	}
}

// WithAudit attaches an audit repository for fire-and-forget audit logging.
func (h *Handler) WithAudit(ar *staff.AuditRepository) *Handler {
	h.auditRepo = ar
	return h
}

// ---------------------------------------------------------
// Customer Operations
// ---------------------------------------------------------

func (h *Handler) CreateCustomerReturn(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserID(r.Context())
	if userID == uuid.Nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}

	idStr := chi.URLParam(r, "id")
	orderID, err := uuid.Parse(idStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid order ID")
		return
	}

	var req CreateReturnRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}
	if err := h.validator.Struct(req); err != nil {
		h.writeError(w, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	responses, err := h.service.CreateReturn(r.Context(), userID, orderID, req)
	if err != nil {
		switch {
		case errors.Is(err, ErrUnauthorized):
			h.writeError(w, http.StatusForbidden, "forbidden", "Unauthorized")
		case errors.Is(err, ErrOrderNotDelivered):
			h.writeError(w, http.StatusBadRequest, "order_not_delivered", err.Error())
		case errors.Is(err, ErrReturnWindowExpired):
			h.writeError(w, http.StatusBadRequest, "return_window_expired", err.Error())
		case errors.Is(err, ErrInvalidQuantity):
			h.writeError(w, http.StatusBadRequest, "invalid_quantity", err.Error())
		case errors.Is(err, ErrEvidenceRequired):
			h.writeError(w, http.StatusBadRequest, "evidence_required", err.Error())
		case errors.Is(err, ErrEvidenceTooMany):
			h.writeError(w, http.StatusBadRequest, "evidence_too_many", err.Error())
		case errors.Is(err, ErrCommentRequired):
			h.writeError(w, http.StatusBadRequest, "comment_required", err.Error())
		case errors.Is(err, ErrEvidenceNotFound):
			h.writeError(w, http.StatusBadRequest, "evidence_not_found", err.Error())
		case errors.Is(err, ErrEvidenceAlreadyBound):
			h.writeError(w, http.StatusBadRequest, "evidence_already_used", err.Error())
		case errors.Is(err, ErrEvidenceDuplicate):
			h.writeError(w, http.StatusBadRequest, "evidence_duplicate", err.Error())
		case errors.Is(err, ErrEvidenceInvalidFormat):
			h.writeError(w, http.StatusBadRequest, "evidence_invalid_format", err.Error())
		default:
			h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to create return")
		}
		return
	}

	var resp CreateReturnResponse
	resp.Returns = responses
	if len(responses) > 0 {
		resp.ReturnResponse = responses[0]
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) GetCustomerReturn(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserID(r.Context())
	if userID == uuid.Nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}

	idStr := chi.URLParam(r, "id")
	returnID, err := uuid.Parse(idStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid return ID")
		return
	}

	retResp, err := h.service.GetCustomerReturn(r.Context(), userID, returnID)
	if err != nil {
		if errors.Is(err, ErrReturnNotFound) {
			h.writeError(w, http.StatusNotFound, "not_found", "Return not found")
			return
		}
		if errors.Is(err, ErrUnauthorized) {
			h.writeError(w, http.StatusForbidden, "forbidden", "Unauthorized")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to get return")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(retResp)
}

func (h *Handler) ListCustomerReturns(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserID(r.Context())
	if userID == uuid.Nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}

	page := pagination.FromRequest(r)
	returnsList, total, err := h.service.ListCustomerReturns(r.Context(), userID, page.Limit, page.Offset)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to list returns")
		return
	}

	resp := ReturnListResponse{Items: returnsList, TotalCount: total}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// ---------------------------------------------------------
// Admin Operations
// ---------------------------------------------------------

func (h *Handler) ListAdminReturns(w http.ResponseWriter, r *http.Request) {
	page := pagination.FromRequest(r)
	returnsList, total, err := h.service.ListAdminReturns(r.Context(), page.Limit, page.Offset)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to list returns")
		return
	}

	resp := AdminReturnListResponse{Items: returnsList, TotalCount: total}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) GetAdminReturn(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	returnID, err := uuid.Parse(idStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid return ID")
		return
	}

	resp, err := h.service.GetAdminReturn(r.Context(), returnID)
	if err != nil {
		if errors.Is(err, ErrReturnNotFound) {
			h.writeError(w, http.StatusNotFound, "not_found", "Return not found")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to get return")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) UpdateAdminReturnStatus(w http.ResponseWriter, r *http.Request) {
	adminID := auth.GetUserID(r.Context())
	if adminID == uuid.Nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}

	idStr := chi.URLParam(r, "id")
	returnID, err := uuid.Parse(idStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid return ID")
		return
	}

	var req UpdateReturnStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}
	if err := h.validator.Struct(req); err != nil {
		h.writeError(w, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	err = h.service.UpdateReturnStatus(r.Context(), adminID, returnID, req)
	if err != nil {
		if errors.Is(err, ErrReturnNotFound) {
			h.writeError(w, http.StatusNotFound, "not_found", "Return not found")
			return
		}
		if errors.Is(err, ErrRejectReasonRequired) {
			h.writeError(w, http.StatusBadRequest, "rejection_reason_required", "Rejection reason is required")
			return
		}
		if errors.Is(err, ErrReturnNotArrived) {
			h.writeError(w, http.StatusBadRequest, "return_not_arrived", "Return shipment has not arrived at ZAMK")
			return
		}

		if errors.Is(err, ErrInvalidStatusTransition) {
			h.writeError(w, http.StatusBadRequest, "invalid_transition", err.Error())
			return
		}
		if errors.Is(err, ErrRejectReasonRequired) {
			h.writeError(w, http.StatusBadRequest, "reason_required", err.Error())
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to update return status")
		return
	}

	if h.auditRepo != nil {
		rid := returnID
		actorID := adminID
		newStatus := req.Status
		go func() {
			_ = h.auditRepo.RecordAudit(context.Background(), staff.AuditEvent{
				ActorUserID: actorID,
				Action:      "return.status_update",
				EntityType:  "return",
				EntityID:    &rid,
				Metadata:    staff.SanitizeMetadata(map[string]any{"newStatus": newStatus}),
			})
		}()
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) CreateAdminRefund(w http.ResponseWriter, r *http.Request) {
	adminID := auth.GetUserID(r.Context())
	if adminID == uuid.Nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}

	idStr := chi.URLParam(r, "id")
	returnID, err := uuid.Parse(idStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid return ID")
		return
	}

	var req CreateRefundRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}
	if err := h.validator.Struct(req); err != nil {
		h.writeError(w, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	ref, err := h.service.CreateRefund(r.Context(), adminID, returnID, req)
	if err != nil {
		if errors.Is(err, ErrReturnNotFound) {
			h.writeError(w, http.StatusNotFound, "not_found", "Return not found")
			return
		}
		if errors.Is(err, ErrReturnNotReceived) {
			h.writeError(w, http.StatusBadRequest, "return_not_received", err.Error())
			return
		}
		if errors.Is(err, ErrReturnRejected) {
			h.writeError(w, http.StatusBadRequest, "return_rejected", err.Error())
			return
		}
		if errors.Is(err, ErrReturnAlreadyRefunded) {
			h.writeError(w, http.StatusBadRequest, "already_refunded", "Return is already refunded or completed")
			return
		}
		if errors.Is(err, ErrRefundNoEligibleItems) {
			h.writeError(w, http.StatusBadRequest, "no_eligible_items", err.Error())
			return
		}
		if errors.Is(err, ErrRefundAllocationInvariant) {
			h.writeError(w, http.StatusBadRequest, "refund_allocation_invariant", "Inconsistent order item allocation state")
			return
		}
		if errors.Is(err, ErrAmbiguousFundingPayment) || errors.Is(err, payments.ErrAmbiguousFundingPayment) {
			h.writeError(w, http.StatusUnprocessableEntity, "ambiguous_funding", "Ambiguous funding payment: multiple succeeded payments exist for order")
			return
		}
		if errors.Is(err, ErrPaymentNotFound) || errors.Is(err, payments.ErrPaymentNotFound) {
			h.writeError(w, http.StatusBadRequest, "payment_not_found", "Succeeded payment for order not found")
			return
		}
		if errors.Is(err, ErrRefundExceedsPaid) || errors.Is(err, payments.ErrRefundExceedsPaid) {
			h.writeError(w, http.StatusBadRequest, "refund_exceeds_paid", err.Error())
			return
		}
		if errors.Is(err, payments.ErrInvalidRefundAmount) {
			h.writeError(w, http.StatusBadRequest, "invalid_refund_amount", err.Error())
			return
		}
		log.Printf("CreateRefund failed: %v", err)
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to create refund")
		return
	}

	if h.auditRepo != nil {
		rid := returnID
		actorID := adminID
		go func() {
			_ = h.auditRepo.RecordAudit(context.Background(), staff.AuditEvent{
				ActorUserID: actorID,
				Action:      "refund.create",
				EntityType:  "refund",
				EntityID:    &rid,
				Metadata:    staff.SanitizeMetadata(map[string]any{}),
			})
		}()
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(ref)
}

func (h *Handler) GetAdminRefundQuote(w http.ResponseWriter, r *http.Request) {
	adminID := auth.GetUserID(r.Context())
	if adminID == uuid.Nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}

	idStr := chi.URLParam(r, "id")
	returnID, err := uuid.Parse(idStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid return ID")
		return
	}

	quote, err := h.service.CalculateRefundQuote(r.Context(), returnID)
	if err != nil {
		if errors.Is(err, ErrReturnNotFound) {
			h.writeError(w, http.StatusNotFound, "not_found", "Return not found")
			return
		}
		log.Printf("CalculateRefundQuote failed: %v", err)
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to calculate refund quote")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(quote)
}

func (h *Handler) ListAdminRefunds(w http.ResponseWriter, r *http.Request) {
	page := pagination.FromRequest(r)
	refunds, err := h.service.ListAdminRefunds(r.Context(), page.Limit, page.Offset)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to list refunds")
		return
	}

	resp := RefundListResponse{Items: refunds, TotalCount: len(refunds)}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) GetAdminRefund(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	refundID, err := uuid.Parse(idStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid refund ID")
		return
	}

	ref, err := h.service.GetAdminRefund(r.Context(), refundID)
	if err != nil {
		if errors.Is(err, ErrRefundNotFound) {
			h.writeError(w, http.StatusNotFound, "not_found", "Refund not found")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to get refund")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ref)
}

func (h *Handler) GetAdminReturnReceivingState(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	returnID, err := uuid.Parse(idStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid return ID")
		return
	}

	state, err := h.service.GetAdminReturnReceivingState(r.Context(), returnID)
	if err != nil {
		if errors.Is(err, ErrReturnNotFound) {
			h.writeError(w, http.StatusNotFound, "not_found", "Return not found")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to get receiving state")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(state)
}

func (h *Handler) StartReceiving(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	returnID, err := uuid.Parse(idStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid return ID")
		return
	}

	err = h.service.StartReceiving(r.Context(), returnID)
	if err != nil {
		if errors.Is(err, ErrReturnNotFound) {
			h.writeError(w, http.StatusNotFound, "not_found", "Return not found")
			return
		}
		if errors.Is(err, ErrReturnNotArrived) {
			h.writeError(w, http.StatusBadRequest, "return_not_arrived", "Return shipment has not arrived at ZAMK")
			return
		}

		if errors.Is(err, ErrInvalidStatusTransition) {
			h.writeError(w, http.StatusBadRequest, "invalid_transition", err.Error())
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to start receiving")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ScanReturnUnit(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	returnID, err := uuid.Parse(idStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid return ID")
		return
	}

	var req ScanReturnUnitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}
	if err := h.validator.Struct(req); err != nil {
		h.writeError(w, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	resp, err := h.service.ScanReturnUnit(r.Context(), returnID, req)
	if err != nil {
		if errors.Is(err, ErrReturnNotFound) {
			h.writeError(w, http.StatusNotFound, "not_found", "Return not found")
			return
		}
		if errors.Is(err, ErrReturnNotInReceiving) {
			h.writeError(w, http.StatusBadRequest, "invalid_state", err.Error())
			return
		}
		if errors.Is(err, ErrInvalidZMUForReturn) {
			h.writeError(w, http.StatusBadRequest, "invalid_zmu", err.Error())
			return
		}
		if errors.Is(err, ErrAllocationAlreadyBound) {
			h.writeError(w, http.StatusConflict, "already_bound", err.Error())
			return
		}
		if errors.Is(err, ErrQuantityExceeded) {
			h.writeError(w, http.StatusBadRequest, "quantity_exceeded", err.Error())
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to scan ZMU")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) InspectSerializedUnit(w http.ResponseWriter, r *http.Request) {
	returnIDStr := chi.URLParam(r, "id")
	returnID, err := uuid.Parse(returnIDStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid return ID")
		return
	}

	unitIDStr := chi.URLParam(r, "unitId")
	unitID, err := uuid.Parse(unitIDStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid unit ID")
		return
	}

	var req UpdateSerializedUnitInspectionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	err = h.service.InspectSerializedUnit(r.Context(), returnID, unitID, req)
	if err != nil {
		if errors.Is(err, ErrReturnNotFound) {
			h.writeError(w, http.StatusNotFound, "not_found", "Return not found")
			return
		}
		if errors.Is(err, ErrUnitNotInReturn) {
			h.writeError(w, http.StatusBadRequest, "unit_not_in_return", err.Error())
			return
		}
		if errors.Is(err, ErrReturnNotInReceiving) {
			h.writeError(w, http.StatusBadRequest, "invalid_state", err.Error())
			return
		}
		if errors.Is(err, ErrInvalidDisposition) {
			h.writeError(w, http.StatusBadRequest, "invalid_disposition", err.Error())
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to update unit inspection")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) InspectLegacyItem(w http.ResponseWriter, r *http.Request) {
	returnIDStr := chi.URLParam(r, "id")
	returnID, err := uuid.Parse(returnIDStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid return ID")
		return
	}

	itemIDStr := chi.URLParam(r, "itemId")
	itemID, err := uuid.Parse(itemIDStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid item ID")
		return
	}

	var req UpdateLegacyItemInspectionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	err = h.service.InspectLegacyItem(r.Context(), returnID, itemID, req)
	if err != nil {
		if errors.Is(err, ErrReturnNotFound) {
			h.writeError(w, http.StatusNotFound, "not_found", "Return or item not found")
			return
		}
		if errors.Is(err, ErrUnitNotInReturn) {
			h.writeError(w, http.StatusBadRequest, "item_not_in_return", err.Error())
			return
		}
		if errors.Is(err, ErrReturnNotInReceiving) {
			h.writeError(w, http.StatusBadRequest, "invalid_state", err.Error())
			return
		}
		if errors.Is(err, ErrItemNotLegacy) {
			h.writeError(w, http.StatusBadRequest, "item_not_legacy", err.Error())
			return
		}
		if errors.Is(err, ErrInvalidInspectionQuantity) {
			h.writeError(w, http.StatusBadRequest, "invalid_quantity", err.Error())
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to update legacy inspection")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) FinalizeReceiving(w http.ResponseWriter, r *http.Request) {
	returnIDStr := chi.URLParam(r, "id")
	returnID, err := uuid.Parse(returnIDStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid return ID")
		return
	}

	err = h.service.FinalizeReceiving(r.Context(), returnID)
	if err != nil {
		if errors.Is(err, ErrReturnNotFound) {
			h.writeError(w, http.StatusNotFound, "not_found", "Return not found")
			return
		}
		if errors.Is(err, ErrReturnNotInReceiving) {
			h.writeError(w, http.StatusBadRequest, "invalid_state", err.Error())
			return
		}
		if errors.Is(err, ErrFinalizeMissingDisposition) {
			h.writeError(w, http.StatusBadRequest, "missing_disposition", err.Error())
			return
		}
		if errors.Is(err, ErrInvalidUnitState) {
			h.writeError(w, http.StatusBadRequest, "invalid_unit_state", err.Error())
			return
		}
		if errors.Is(err, ErrInvalidInspectionQuantity) {
			h.writeError(w, http.StatusBadRequest, "invalid_quantity", err.Error())
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to finalize receiving")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ---------------------------------------------------------
// Seller Operations
// ---------------------------------------------------------

func (h *Handler) ListSellerReturns(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserID(r.Context())
	if userID == uuid.Nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}

	page := pagination.FromRequest(r)
	items, err := h.service.ListSellerReturns(r.Context(), userID, page.Limit, page.Offset)
	if err != nil {
		if errors.Is(err, ErrUnauthorized) {
			h.writeError(w, http.StatusForbidden, "forbidden", "Must be a seller")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to list seller returns")
		return
	}

	resp := SellerReturnListResponse{Items: items, TotalCount: len(items)}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) GetSellerReturn(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserID(r.Context())
	if userID == uuid.Nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}

	idStr := chi.URLParam(r, "id")
	returnID, err := uuid.Parse(idStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid return ID")
		return
	}

	items, err := h.service.GetSellerReturn(r.Context(), userID, returnID)
	if err != nil {
		if errors.Is(err, ErrReturnNotFound) {
			h.writeError(w, http.StatusNotFound, "not_found", "Return not found or has no matching items")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to get seller return")
		return
	}

	resp := SellerReturnListResponse{Items: items, TotalCount: len(items)}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// ---------------------------------------------------------
// Helper
// ---------------------------------------------------------

func (h *Handler) writeError(w http.ResponseWriter, statusCode int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]string{
			"code":    code,
			"message": message,
		},
	})
}

func (h *Handler) UploadReturnEvidence(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID := auth.GetUserID(ctx)
	if userID == uuid.Nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}

	maxMemory := int64(10) * 1024 * 1024
	if err := r.ParseMultipartForm(maxMemory); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "invalid form data")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		file, header, err = r.FormFile("image")
		if err != nil {
			h.writeError(w, http.StatusBadRequest, "missing_file", "missing file in form")
			return
		}
	}
	defer file.Close()

	if header.Size > maxMemory {
		h.writeError(w, http.StatusBadRequest, "file_too_large", "file exceeds maximum allowed size")
		return
	}

	ext := strings.ToLower(filepath.Ext(header.Filename))
	validExts := map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".webp": true}
	if !validExts[ext] {
		h.writeError(w, http.StatusBadRequest, "invalid_file_type", "only jpeg, png, and webp are allowed")
		return
	}

	contentType := header.Header.Get("Content-Type")
	validMimes := map[string]bool{"image/jpeg": true, "image/png": true, "image/webp": true}
	if !validMimes[contentType] {
		h.writeError(w, http.StatusBadRequest, "invalid_file_type", "only jpeg, png, and webp are allowed")
		return
	}

	resp, err := h.service.UploadReturnEvidence(ctx, userID, file, header.Filename, header.Size, contentType)
	if err != nil {
		if errors.Is(err, ErrEvidenceInvalidFormat) {
			h.writeError(w, http.StatusBadRequest, "invalid_file_type", err.Error())
			return
		}
		h.writeError(w, http.StatusInternalServerError, "upload_failed", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) DeleteCustomerReturnEvidence(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	customerID := auth.GetUserID(ctx)
	if customerID == uuid.Nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}

	evidenceIDStr := chi.URLParam(r, "evidenceId")
	evidenceID, err := uuid.Parse(evidenceIDStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_evidence_id", "invalid evidence id")
		return
	}

	err = h.service.DeleteStagedEvidence(ctx, customerID, evidenceID)
	if err != nil {
		switch {
		case errors.Is(err, ErrEvidenceNotFound):
			h.writeError(w, http.StatusNotFound, "evidence_not_found", "evidence not found")
		case errors.Is(err, ErrEvidenceAlreadyBound):
			h.writeError(w, http.StatusBadRequest, "evidence_already_bound", "cannot delete bound return evidence")
		default:
			h.writeError(w, http.StatusInternalServerError, "delete_failed", err.Error())
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func mapReturnShipmentResponse(shipment *ReturnShipment) *ReturnShipmentResponse {
	if shipment == nil {
		return nil
	}
	var pickupDTO *PickupAddressDTO
	if len(shipment.PickupAddress) > 0 {
		var p PickupAddressDTO
		if err := json.Unmarshal(shipment.PickupAddress, &p); err == nil {
			pickupDTO = &p
		}
	}
	return &ReturnShipmentResponse{
		ID:                     shipment.ID,
		Provider:               shipment.Provider,
		Method:                 shipment.Method,
		TrackingNumber:         shipment.TrackingNumber,
		ProviderShipmentID:     shipment.ProviderShipmentID,
		Status:                 shipment.Status,
		SelectedCDEKOfficeCode: shipment.SelectedCDEKOfficeCode,
		CustomerName:           shipment.CustomerName,
		CustomerPhone:          shipment.CustomerPhone,
		PickupAddress:          pickupDTO,
		CDEKOfficeAddress:      shipment.CDEKOfficeAddress,
	}
}

func (h *Handler) GetCustomerReturnShipment(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserID(r.Context())
	if userID == uuid.Nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}
	idStr := chi.URLParam(r, "id")
	returnID, err := uuid.Parse(idStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid return ID")
		return
	}
	shipment, err := h.service.GetCustomerReturnShipment(r.Context(), userID, returnID)
	if err != nil {
		switch {
		case errors.Is(err, ErrReturnNotFound):
			h.writeError(w, http.StatusNotFound, "not_found", "Return not found")
		case errors.Is(err, ErrUnauthorized):
			h.writeError(w, http.StatusForbidden, "forbidden", "Unauthorized")
		default:
			h.writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		}
		return
	}
	if shipment == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{"shipment": nil})
		return
	}
	resp := mapReturnShipmentResponse(shipment)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{"shipment": resp})
}

func (h *Handler) CreateCustomerReturnShipment(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserID(r.Context())
	if userID == uuid.Nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}
	idStr := chi.URLParam(r, "id")
	returnID, err := uuid.Parse(idStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid return ID")
		return
	}
	var req CreateReturnShipmentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_body", "Invalid request body")
		return
	}
	shipment, err := h.service.CreateCustomerReturnShipment(r.Context(), userID, returnID, req)
	if err != nil {
		switch {
		case errors.Is(err, ErrReturnNotFound):
			h.writeError(w, http.StatusNotFound, "not_found", "Return not found")
		case errors.Is(err, ErrUnauthorized):
			h.writeError(w, http.StatusForbidden, "forbidden", "Unauthorized")
		case errors.Is(err, ErrReturnNotApproved):
			h.writeError(w, http.StatusBadRequest, "not_approved", "Return must be in approved status")
		case errors.Is(err, ErrShipmentAlreadyExists):
			h.writeError(w, http.StatusConflict, "shipment_exists", "An active shipment already exists for this return")
		case errors.Is(err, ErrCDEKOfficeRequired):
			h.writeError(w, http.StatusBadRequest, "cdek_office_required", "CDEK office code is required for office delivery")
		case errors.Is(err, ErrInvalidCDEKOffice):
			h.writeError(w, http.StatusBadRequest, "cdek_office_invalid", "Invalid or unknown CDEK office code")
		case errors.Is(err, ErrCourierInfoRequired):
			h.writeError(w, http.StatusBadRequest, "courier_info_required", "Customer name, phone, and pickup address are required for courier delivery")
		case errors.Is(err, ErrCDEKNotConfigured):
			h.writeError(w, http.StatusServiceUnavailable, "cdek_not_configured", "CDEK logistics is not configured")
		default:
			h.writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		}
		return
	}
	resp := mapReturnShipmentResponse(shipment)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{"shipment": resp})
}

func (h *Handler) GetCDEKOffices(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserID(r.Context())
	if userID == uuid.Nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}
	offices, err := h.service.ListCDEKOffices(r.Context())
	if err != nil {
		switch {
		case errors.Is(err, ErrCDEKNotConfigured):
			h.writeError(w, http.StatusServiceUnavailable, "cdek_not_configured", "CDEK logistics is not configured")
		default:
			h.writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		}
		return
	}
	dtos := make([]CDEKOfficeDTO, 0, len(offices))
	for _, o := range offices {
		var wh *string
		if o.WorkingHours != "" {
			hCopy := o.WorkingHours
			wh = &hCopy
		}
		dtos = append(dtos, CDEKOfficeDTO{Code: o.Code, Address: o.Address, Name: o.Name, WorkingHours: wh})
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{"offices": dtos})
}

func (h *Handler) GetAdminReturnTimeline(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	returnID, err := uuid.Parse(idStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid return ID")
		return
	}

	timeline, err := h.service.GetAdminTimeline(r.Context(), returnID)
	if err != nil {
		if errors.Is(err, ErrReturnNotFound) {
			h.writeError(w, http.StatusNotFound, "return_not_found", "Return not found")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Internal server error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(timeline)
}

func (h *Handler) SimulateCreateReturnShipment(w http.ResponseWriter, r *http.Request) {
	if h.appEnv == "production" {
		h.writeError(w, http.StatusForbidden, "dev_tool_disabled", "Dev logistics simulator is disabled in production")
		return
	}

	idStr := chi.URLParam(r, "id")
	returnID, err := uuid.Parse(idStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid return ID")
		return
	}

	shipment, err := h.service.CreateSimulatedReturnShipment(r.Context(), returnID)
	if err != nil {
		switch {
		case errors.Is(err, ErrReturnNotFound):
			h.writeError(w, http.StatusNotFound, "not_found", "Return not found")
		case errors.Is(err, ErrReturnNotApproved):
			h.writeError(w, http.StatusBadRequest, "not_approved", "Return must be in approved status to simulate shipment")
		case errors.Is(err, ErrShipmentAlreadyExists):
			h.writeError(w, http.StatusConflict, "shipment_exists", "An active shipment already exists for this return")
		default:
			h.writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		}
		return
	}

	resp := mapReturnShipmentResponse(shipment)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{"shipment": resp})
}

func (h *Handler) SimulateAdvanceReturnShipment(w http.ResponseWriter, r *http.Request) {
	if h.appEnv == "production" {
		h.writeError(w, http.StatusForbidden, "dev_tool_disabled", "Dev logistics simulator is disabled in production")
		return
	}

	idStr := chi.URLParam(r, "id")
	returnID, err := uuid.Parse(idStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid return ID")
		return
	}

	shipment, err := h.service.AdvanceSimulatedReturnShipment(r.Context(), returnID)
	if err != nil {
		switch {
		case errors.Is(err, ErrReturnNotFound), errors.Is(err, ErrShipmentNotFound):
			h.writeError(w, http.StatusNotFound, "not_found", "Shipment or return not found")
		case errors.Is(err, ErrInvalidShipmentTransition):
			h.writeError(w, http.StatusBadRequest, "invalid_transition", "Cannot advance shipment beyond its terminal state or invalid state")
		default:
			h.writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		}
		return
	}

	resp := mapReturnShipmentResponse(shipment)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{"shipment": resp})
}
