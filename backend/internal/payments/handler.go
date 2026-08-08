package payments

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type Handler struct {
	service *Service
	appEnv  string
}

func NewHandler(service *Service, appEnv string) *Handler {
	return &Handler{service: service, appEnv: appEnv}
}

func (h *Handler) CreatePayment(w http.ResponseWriter, r *http.Request) {
	val := r.Context().Value("userID")
	if val == nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}
	userID := val.(uuid.UUID)

	idStr := chi.URLParam(r, "id")
	orderID, err := uuid.Parse(idStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid order ID")
		return
	}

	var req CreatePaymentRequest
	// Optionally decode body if it exists, otherwise use default
	if r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			h.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
			return
		}
	}
	if req.Method == "" {
		req.Method = "unknown"
	}

	resp, err := h.service.CreatePayment(r.Context(), userID, orderID, req.Method)
	if err != nil {
		if errors.Is(err, ErrOrderNotFound) {
			h.writeError(w, http.StatusNotFound, "not_found", "Order not found")
			return
		}
		if errors.Is(err, ErrOrderNotAwaitingPayment) {
			h.writeError(w, http.StatusBadRequest, "invalid_order_status", err.Error())
			return
		}
		if errors.Is(err, ErrPaymentMethodUnavailable) {
			h.writeError(w, http.StatusBadRequest, "payment_method_unavailable", err.Error())
			return
		}
		log.Printf("CreatePayment Error: %v", err)
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to create payment")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) HandleTBankWebhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "Failed to read body")
		return
	}
	defer r.Body.Close()

	headers := make(map[string]string)
	for k, v := range r.Header {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}

	if err := h.service.HandleWebhook(r.Context(), headers, body); err != nil {
		if errors.Is(err, ErrInvalidSignature) {
			h.writeError(w, http.StatusBadRequest, "invalid_signature", "Invalid signature")
			return
		}
		if errors.Is(err, ErrPaymentAlreadyProcessed) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
			return
		}
		
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to process webhook: " + err.Error())
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func (h *Handler) ListAdminPayments(w http.ResponseWriter, r *http.Request) {
	limitStr := r.URL.Query().Get("limit")
	limit := 50
	var err error
	if limitStr != "" {
		limit, err = strconv.Atoi(limitStr)
		if err != nil || limit < 1 || limit > 100 {
			h.writeError(w, http.StatusBadRequest, "validation_error", "Limit must be between 1 and 100")
			return
		}
	}

	offsetStr := r.URL.Query().Get("offset")
	offset := 0
	if offsetStr != "" {
		offset, err = strconv.Atoi(offsetStr)
		if err != nil || offset < 0 {
			h.writeError(w, http.StatusBadRequest, "validation_error", "Offset must be non-negative")
			return
		}
	}

	q := r.URL.Query().Get("q")
	
	status := r.URL.Query().Get("status")
	if status != "" {
		validStatuses := map[string]bool{"created": true, "pending": true, "succeeded": true, "failed": true, "cancelled": true}
		if !validStatuses[status] {
			h.writeError(w, http.StatusBadRequest, "validation_error", "Invalid status")
			return
		}
	}

	provider := r.URL.Query().Get("provider")
	if provider != "" && provider != "tbank" {
		h.writeError(w, http.StatusBadRequest, "validation_error", "Invalid provider")
		return
	}

	method := r.URL.Query().Get("paymentMethod")
	if method != "" {
		validMethods := map[string]bool{"tpay": true, "spb": true, "card": true}
		if !validMethods[method] {
			h.writeError(w, http.StatusBadRequest, "validation_error", "Invalid payment method")
			return
		}
	}

	mode := r.URL.Query().Get("integrationMode")
	if mode != "" && mode != "mock" && mode != "api" {
		h.writeError(w, http.StatusBadRequest, "validation_error", "Invalid integration mode")
		return
	}

	refundState := r.URL.Query().Get("refundState")
	if refundState != "" {
		validRefundStates := map[string]bool{"none": true, "pending": true, "partial": true, "full": true, "partial_pending": true, "full_pending": true}
		if !validRefundStates[refundState] {
			h.writeError(w, http.StatusBadRequest, "validation_error", "Invalid refund state")
			return
		}
	}

	probCode := r.URL.Query().Get("problemCode")
	if probCode != "" {
		validProbCodes := map[string]bool{
			"PAID_ORDER_WITHOUT_SUCCEEDED_PAYMENT": true,
			"SUCCEEDED_PAYMENT_ORDER_NOT_PAID": true,
			"MULTIPLE_SUCCEEDED_PAYMENTS": true,
			"AMOUNT_MISMATCH": true,
			"STUCK_PENDING": true,
			"INVALID_WEBHOOK_SIGNATURE": true,
			"UNPROCESSED_WEBHOOK": true,
		}
		if !validProbCodes[probCode] {
			h.writeError(w, http.StatusBadRequest, "validation_error", "Invalid problem code")
			return
		}
	}

	hasProblemStr := r.URL.Query().Get("hasProblem")
	if hasProblemStr != "" && hasProblemStr != "true" && hasProblemStr != "false" {
		h.writeError(w, http.StatusBadRequest, "validation_error", "hasProblem must be true or false")
		return
	}
	hasProblem := hasProblemStr == "true"

	sort := r.URL.Query().Get("sort")
	if sort != "" {
		validSorts := map[string]bool{"createdAt": true, "updatedAt": true, "amount": true, "paymentNumber": true, "status": true}
		if !validSorts[sort] {
			h.writeError(w, http.StatusBadRequest, "validation_error", "Invalid sort field")
			return
		}
	}

	direction := r.URL.Query().Get("direction")
	if direction != "" {
		if direction != "asc" && direction != "desc" {
			h.writeError(w, http.StatusBadRequest, "validation_error", "Invalid sort direction")
			return
		}
	}

	dateFrom := r.URL.Query().Get("dateFrom")
	dateTo := r.URL.Query().Get("dateTo")
	var tFrom, tTo time.Time
	
	if dateFrom != "" {
		tFrom, err = time.Parse(time.RFC3339, dateFrom)
		if err != nil {
			h.writeError(w, http.StatusBadRequest, "validation_error", "Invalid dateFrom format (must be RFC3339)")
			return
		}
	}
	if dateTo != "" {
		tTo, err = time.Parse(time.RFC3339, dateTo)
		if err != nil {
			h.writeError(w, http.StatusBadRequest, "validation_error", "Invalid dateTo format (must be RFC3339)")
			return
		}
	}
	if dateFrom != "" && dateTo != "" {
		if tFrom.After(tTo) {
			h.writeError(w, http.StatusBadRequest, "validation_error", "dateFrom cannot be after dateTo")
			return
		}
	}

	var amountFrom, amountTo int64
	amountFromStr := r.URL.Query().Get("amountFromCents")
	amountToStr := r.URL.Query().Get("amountToCents")

	if amountFromStr != "" {
		amountFrom, err = strconv.ParseInt(amountFromStr, 10, 64)
		if err != nil || amountFrom < 0 {
			h.writeError(w, http.StatusBadRequest, "validation_error", "amountFromCents must be a valid non-negative integer")
			return
		}
	}
	if amountToStr != "" {
		amountTo, err = strconv.ParseInt(amountToStr, 10, 64)
		if err != nil || amountTo < 0 {
			h.writeError(w, http.StatusBadRequest, "validation_error", "amountToCents must be a valid non-negative integer")
			return
		}
	}
	if amountFromStr != "" && amountToStr != "" {
		if amountFrom > amountTo {
			h.writeError(w, http.StatusBadRequest, "validation_error", "amountFromCents cannot be greater than amountToCents")
			return
		}
	}

	payments, total, err := h.service.ListAdminPayments(r.Context(), q, status, provider, method, mode, refundState, probCode, dateFrom, dateTo, amountFrom, amountTo, hasProblem, sort, direction, limit, offset)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to list payments")
		return
	}

	resp := AdminPaymentListResponse{
		Items:      payments,
		TotalCount: total,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) GetAdminPayment(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	paymentID, err := uuid.Parse(idStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid payment ID")
		return
	}

	detail, err := h.service.GetAdminPaymentDetail(r.Context(), paymentID)
	if err != nil {
		if errors.Is(err, ErrPaymentNotFound) {
			h.writeError(w, http.StatusNotFound, "not_found", "Payment not found")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to get payment detail")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(detail)
}

func (h *Handler) GetDevMockPayment(w http.ResponseWriter, r *http.Request) {
	if h.appEnv == "production" {
		h.writeError(w, http.StatusForbidden, "forbidden", "Dev mock endpoints are disabled in production")
		return
	}

	idStr := chi.URLParam(r, "id")
	paymentID, err := uuid.Parse(idStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid payment ID")
		return
	}

	payment, err := h.service.GetPaymentByID(r.Context(), paymentID)
	if err != nil {
		if errors.Is(err, ErrPaymentNotFound) {
			h.writeError(w, http.StatusNotFound, "not_found", "Payment not found")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to get payment")
		return
	}

	if payment.IntegrationMode != "mock" {
		h.writeError(w, http.StatusBadRequest, "invalid_mode", "Payment is not in mock integration mode")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(payment)
}

func (h *Handler) ProcessDevMockAction(w http.ResponseWriter, r *http.Request) {
	if h.appEnv == "production" {
		h.writeError(w, http.StatusForbidden, "forbidden", "Dev mock endpoints are disabled in production")
		return
	}

	idStr := chi.URLParam(r, "id")
	paymentID, err := uuid.Parse(idStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid payment ID")
		return
	}

	action := chi.URLParam(r, "action")
	if action != "confirm" && action != "reject" && action != "cancel" {
		h.writeError(w, http.StatusBadRequest, "invalid_action", "Invalid action: must be confirm, reject, or cancel")
		return
	}

	if err := h.service.ProcessMockPaymentAction(r.Context(), paymentID, action); err != nil {
		if errors.Is(err, ErrPaymentNotFound) {
			h.writeError(w, http.StatusNotFound, "not_found", "Payment not found")
			return
		}
		if errors.Is(err, ErrPaymentAlreadyProcessed) {
			h.writeError(w, http.StatusBadRequest, "already_processed", err.Error())
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to process mock action: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

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
