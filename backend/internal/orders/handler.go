package orders

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/http/pagination"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/marketing"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/staff"
)

type Handler struct {
	service   *Service
	validator *validator.Validate
	auditRepo *staff.AuditRepository
}

func NewHandler(service *Service) *Handler {
	return &Handler{
		service:   service,
		validator: validator.New(),
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

func (h *Handler) CreateOrder(w http.ResponseWriter, r *http.Request) {
	val := r.Context().Value("userID")
	if val == nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}
	userID := val.(uuid.UUID)

	var req CreateOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	if err := h.validator.Struct(req); err != nil {
		h.writeError(w, http.StatusBadRequest, "validation_error", "Проверьте данные получателя: имя, телефон, email и адрес доставки.")
		return
	}

	idempStr := r.Header.Get("Idempotency-Key")
	var idempotencyKey *uuid.UUID
	if idempStr != "" {
		id, err := uuid.Parse(idempStr)
		if err == nil {
			idempotencyKey = &id
		}
	}

	order, err := h.service.CreateOrder(r.Context(), userID, req, idempotencyKey)
	if err != nil {
		var dderr *DeliveryDetailsError
		if errors.As(err, &dderr) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": "validation_error", "message": dderr.Message, "field": dderr.Field}})
			return
		}
		// Cart states the buyer can fix: stable codes and readable messages; the cart is kept, nothing is reserved.
		switch {
		case errors.Is(err, ErrEmptyCart):
			h.writeError(w, http.StatusBadRequest, "empty_cart", "Корзина пуста. Добавьте товары, чтобы оформить заказ.")
			return
		case errors.Is(err, ErrInsufficientStock):
			h.writeError(w, http.StatusConflict, "insufficient_stock", "Недостаточно товара на складе. Вернитесь в корзину и измените количество.")
			return
		case errors.Is(err, ErrProductNotPublished) || errors.Is(err, ErrVariantNotFound):
			h.writeError(w, http.StatusConflict, "item_unavailable", "Один из товаров больше недоступен. Вернитесь в корзину и удалите его.")
			return
		case errors.Is(err, ErrInvalidDeliveryMethod):
			h.writeError(w, http.StatusBadRequest, "invalid_delivery_method", "Выбранный способ доставки недоступен. Выберите другой.")
			return
		case errors.Is(err, ErrFirstPaymentPending):
			h.writeError(w, http.StatusConflict, "payment_in_progress", firstPaymentPendingMessage(err))
			return
		}
		if errors.Is(err, marketing.ErrPromoNotFound) || errors.Is(err, marketing.ErrPromoInactive) || errors.Is(err, marketing.ErrPromoNotStarted) ||
			errors.Is(err, marketing.ErrPromoExpired) || errors.Is(err, marketing.ErrPromoMinSubtotal) || errors.Is(err, marketing.ErrPromoMinQuantity) || errors.Is(err, marketing.ErrPromoMinDistinctProducts) || errors.Is(err, marketing.ErrPromoGlobalLimit) ||
			errors.Is(err, marketing.ErrPromoCustomerLimit) || errors.Is(err, marketing.ErrPromoFirstOrderOnly) || errors.Is(err, marketing.ErrPromoRepeatCustomerRequired) || errors.Is(err, marketing.ErrPromoNotApplicable) ||
			errors.Is(err, marketing.ErrPromoBudgetExhausted) || errors.Is(err, marketing.ErrPromoFullDiscount) {
			h.writeError(w, http.StatusBadRequest, "invalid_order", err.Error())
			return
		}
		if errors.Is(err, ErrSelfPurchase) {
			h.writeError(w, http.StatusUnprocessableEntity, "self_purchase_not_allowed", "Нельзя оформить заказ на товары собственного магазина")
			return
		}
		if errors.Is(err, ErrIdempotencyKeyConflict) {
			h.writeError(w, http.StatusConflict, "conflict", "Данные заказа изменились после отправки. Проверьте их и оформите заказ ещё раз.")
			return
		}
		// Unexpected failure: logged for the team, never shown to the buyer (no database or internal text).
		slog.ErrorContext(r.Context(), "create order failed", "error", err)
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Не удалось оформить заказ. Попробуйте ещё раз.")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(order)
}

type ValidatePromoRequest struct {
	Code string `json:"code" validate:"required"`
}

func (h *Handler) ValidatePromo(w http.ResponseWriter, r *http.Request) {
	val := r.Context().Value("userID")
	if val == nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}
	userID := val.(uuid.UUID)

	var req ValidatePromoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}
	if strings.TrimSpace(req.Code) == "" {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "Promo code cannot be empty")
		return
	}

	calc, err := h.service.ValidateCartPromo(r.Context(), userID, req.Code)
	if err != nil {
		if errors.Is(err, ErrEmptyCart) || errors.Is(err, ErrProductNotPublished) || errors.Is(err, ErrVariantNotFound) ||
			errors.Is(err, marketing.ErrPromoNotFound) || errors.Is(err, marketing.ErrPromoInactive) || errors.Is(err, marketing.ErrPromoNotStarted) ||
			errors.Is(err, marketing.ErrPromoExpired) || errors.Is(err, marketing.ErrPromoMinSubtotal) || errors.Is(err, marketing.ErrPromoMinQuantity) || errors.Is(err, marketing.ErrPromoMinDistinctProducts) || errors.Is(err, marketing.ErrPromoGlobalLimit) ||
			errors.Is(err, marketing.ErrPromoCustomerLimit) || errors.Is(err, marketing.ErrPromoFirstOrderOnly) || errors.Is(err, marketing.ErrPromoRepeatCustomerRequired) || errors.Is(err, marketing.ErrPromoNotApplicable) ||
			errors.Is(err, marketing.ErrPromoBudgetExhausted) || errors.Is(err, marketing.ErrPromoFullDiscount) {
			h.writeError(w, http.StatusBadRequest, "invalid_promo", err.Error())
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(newPromoQuoteResponse(calc))
}

// PromoQuoteResponse is the customer-safe preview of a promo code. It deliberately exposes only the
// code and the discount the customer gets: seller, campaign, funding and commission data stay internal.
// The preview is advisory; CreateOrder recalculates and re-validates the promo on the server.
type PromoQuoteResponse struct {
	Code          string `json:"code"`
	DiscountCents int64  `json:"discountCents"`
}

func newPromoQuoteResponse(calc *marketing.CheckoutPromoCalculation) PromoQuoteResponse {
	return PromoQuoteResponse{Code: calc.Code, DiscountCents: calc.TotalOrderDiscountCents}
}

func (h *Handler) CancelCustomerOrder(w http.ResponseWriter, r *http.Request) {
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

	if err := h.service.CancelCustomerOrder(r.Context(), userID, orderID); err != nil {
		if errors.Is(err, ErrOrderNotFound) {
			h.writeError(w, http.StatusNotFound, "not_found", "Order not found")
			return
		}
		if errors.Is(err, ErrOrderAlreadyCancelled) {
			h.writeError(w, http.StatusBadRequest, "order_already_cancelled", "Order is already cancelled")
			return
		}
		if errors.Is(err, ErrOrderNotCancellable) {
			h.writeError(w, http.StatusBadRequest, "not_cancellable", err.Error())
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to cancel order")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) GetCustomerOrder(w http.ResponseWriter, r *http.Request) {
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

	order, err := h.service.GetCustomerOrder(r.Context(), userID, orderID)
	if err != nil {
		if errors.Is(err, ErrOrderNotFound) {
			h.writeError(w, http.StatusNotFound, "not_found", "Order not found")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to get order")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(order)
}

func (h *Handler) ListCustomerOrders(w http.ResponseWriter, r *http.Request) {
	val := r.Context().Value("userID")
	if val == nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}
	userID := val.(uuid.UUID)

	page := pagination.FromRequest(r)
	orders, err := h.service.ListCustomerOrders(r.Context(), userID, page.Limit, page.Offset)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to list orders")
		return
	}

	resp := OrderListResponse{
		Items:      orders,
		TotalCount: len(orders),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// ---------------------------------------------------------
// Admin Operations
// ---------------------------------------------------------

func (h *Handler) GetAdminOrder(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	orderID, err := uuid.Parse(idStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid order ID")
		return
	}

	order, err := h.service.GetAdminOrder(r.Context(), orderID)
	if err != nil {
		if errors.Is(err, ErrOrderNotFound) {
			h.writeError(w, http.StatusNotFound, "not_found", "Order not found")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to get order")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(order)
}

func (h *Handler) ListAdminOrders(w http.ResponseWriter, r *http.Request) {
	page := pagination.FromRequest(r)
	
	q := r.URL.Query().Get("q")
	status := r.URL.Query().Get("status")
	paymentStatus := r.URL.Query().Get("paymentStatus")
	fulfillmentStatus := r.URL.Query().Get("fulfillmentStatus")
	sourceType := r.URL.Query().Get("sourceType")
	sellerId := r.URL.Query().Get("sellerId")

	orders, total, err := h.service.ListAdminOrders(r.Context(), q, status, paymentStatus, fulfillmentStatus, sourceType, sellerId, page.Limit, page.Offset)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to list orders")
		return
	}

	resp := AdminOrderListResponse{
		Items:      orders,
		TotalCount: total,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) CancelAdminOrder(w http.ResponseWriter, r *http.Request) {
	val := r.Context().Value("userID")
	if val == nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}
	adminID := val.(uuid.UUID)

	idStr := chi.URLParam(r, "id")
	orderID, err := uuid.Parse(idStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid order ID")
		return
	}

	var req CancelAdminOrderRequest
	if r.Body != nil && r.ContentLength != 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
			h.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
			return
		}
	}

	if err := h.service.CancelAdminOrder(r.Context(), adminID, orderID, req); err != nil {
		if errors.Is(err, ErrOrderNotFound) {
			h.writeError(w, http.StatusNotFound, "not_found", "Order not found")
			return
		}
		if errors.Is(err, ErrOrderAlreadyCancelled) {
			h.writeError(w, http.StatusBadRequest, "order_already_cancelled", err.Error())
			return
		}
		if errors.Is(err, ErrOrderNotCancellable) {
			h.writeError(w, http.StatusBadRequest, "not_cancellable", err.Error())
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to cancel order")
		return
	}

	if h.auditRepo != nil {
		oid := orderID
		actorID := adminID
		reason := req.Reason
		comment := req.Comment
		go func() {
			_ = h.auditRepo.RecordAudit(context.Background(), staff.AuditEvent{
				ActorUserID: actorID,
				Action:      "order.cancel",
				EntityType:  "order",
				EntityID:    &oid,
				Metadata:    staff.SanitizeMetadata(map[string]any{"reason": reason, "comment": comment}),
			})
		}()
	}

	w.WriteHeader(http.StatusNoContent)
}

// ---------------------------------------------------------
// Seller Operations
// ---------------------------------------------------------

func (h *Handler) GetSellerOrder(w http.ResponseWriter, r *http.Request) {
	val := r.Context().Value("userID")
	if val == nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}
	sellerID := val.(uuid.UUID)

	idStr := chi.URLParam(r, "id")
	orderID, err := uuid.Parse(idStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid order ID")
		return
	}

	order, err := h.service.GetSellerOrder(r.Context(), sellerID, orderID)
	if err != nil {
		if errors.Is(err, ErrOrderNotFound) {
			h.writeError(w, http.StatusNotFound, "not_found", "Order not found")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to get order")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(order)
}

func (h *Handler) ListSellerOrders(w http.ResponseWriter, r *http.Request) {
	val := r.Context().Value("userID")
	if val == nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}
	sellerID := val.(uuid.UUID)

	page := pagination.FromRequest(r)
	filter := r.URL.Query().Get("filter")
	if filter == "" {
		filter = SellerOrderFilterAll
	}
	orders, total, err := h.service.ListSellerOrders(r.Context(), sellerID, filter, page.Limit, page.Offset)
	if err != nil {
		if errors.Is(err, ErrInvalidSellerOrderFilter) {
			h.writeError(w, http.StatusBadRequest, "invalid_filter", "Invalid order filter")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to list orders")
		return
	}

	resp := SellerOrderListResponse{
		Items:      orders,
		TotalCount: total,
		Limit:      page.Limit,
		Offset:     page.Offset,
		Filter:     filter,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) GetSellerOrderSummary(w http.ResponseWriter, r *http.Request) {
	val := r.Context().Value("userID")
	if val == nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}
	sellerID := val.(uuid.UUID)

	summary, err := h.service.GetSellerOrderSummary(r.Context(), sellerID)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to get summary")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(summary)
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

func (h *Handler) GetAdminOrderTimeline(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	orderID, err := uuid.Parse(idStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid order ID")
		return
	}

	timeline, err := h.service.GetAdminOrderTimeline(r.Context(), orderID)
	if err != nil {
		if errors.Is(err, ErrOrderNotFound) {
			h.writeError(w, http.StatusNotFound, "order_not_found", "Order not found")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Internal server error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(timeline)
}

// firstPaymentPendingMessage tells the customer which order to pay (or let lapse) before a new order is accepted.
func firstPaymentPendingMessage(err error) string {
	var pending *FirstPaymentPendingError
	if errors.As(err, &pending) && pending.OrderNumber != "" {
		return "Оплата заказа " + pending.OrderNumber + " ещё не завершена. Завершите её или отмените на странице оплаты, затем оформите новый заказ. Заказ доступен в разделе «Мои заказы»."
	}
	return "Оплата предыдущего заказа ещё не завершена. Завершите её или отмените на странице оплаты, затем оформите новый заказ. Заказ доступен в разделе «Мои заказы»."
}
