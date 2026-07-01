package products

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/http/pagination"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/sellers"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/staff"
	"github.com/google/uuid"
)

type Handler struct {
	service        *Service
	sellersService *sellers.Service
	validator      *validator.Validate
	auditRepo      *staff.AuditRepository
}

func NewHandler(service *Service, sellersService *sellers.Service) *Handler {
	return &Handler{
		service:        service,
		sellersService: sellersService,
		validator:      validator.New(),
	}
}

// WithAudit attaches an audit repository for fire-and-forget audit logging.
func (h *Handler) WithAudit(ar *staff.AuditRepository) *Handler {
	h.auditRepo = ar
	return h
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

func (h *Handler) getUserID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	val := r.Context().Value("userID")
	if val == nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Missing user context")
		return uuid.Nil, false
	}
	userID, ok := val.(uuid.UUID)
	if !ok {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Invalid user context")
		return uuid.Nil, false
	}
	return userID, true
}

func (h *Handler) parseUUIDParam(w http.ResponseWriter, r *http.Request, param string) (uuid.UUID, bool) {
	idStr := chi.URLParam(r, param)
	id, err := uuid.Parse(idStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid ID format")
		return uuid.Nil, false
	}
	return id, true
}

// ---------------------------------------------------------
// Seller Handlers
// ---------------------------------------------------------

func (h *Handler) CreateProduct(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.getUserID(w, r)
	if !ok {
		return
	}

	var req CreateProductRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	if err := h.validator.Struct(req); err != nil {
		h.writeError(w, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	prod, err := h.service.CreateProductForSeller(r.Context(), userID, req)
	if err != nil {
		if errors.Is(err, ErrDuplicateSlug) {
			h.writeError(w, http.StatusConflict, "duplicate_slug", "Product slug already exists")
			return
		}
		if errors.Is(err, ErrSellerNotFound) {
			h.writeError(w, http.StatusForbidden, "forbidden", "Seller profile not found")
			return
		}
		if errors.Is(err, ErrSellerBlocked) {
			h.writeError(w, http.StatusForbidden, "seller_blocked", "Магазин заблокирован или архивирован. Действие недоступно.")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to create product")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(prod)
}

func (h *Handler) ListSellerProducts(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.getUserID(w, r)
	if !ok {
		return
	}

	page := pagination.FromRequest(r)
	resp, err := h.service.ListSellerProducts(r.Context(), userID, page.Limit, page.Offset)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to list products")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) GetSellerProduct(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.getUserID(w, r)
	if !ok {
		return
	}
	productID, ok := h.parseUUIDParam(w, r, "id")
	if !ok {
		return
	}

	prod, err := h.service.GetSellerProduct(r.Context(), userID, productID)
	if err != nil {
		if errors.Is(err, ErrProductNotFound) {
			h.writeError(w, http.StatusNotFound, "not_found", "Product not found")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to get product")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(prod)
}

func (h *Handler) UpdateProduct(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.getUserID(w, r)
	if !ok {
		return
	}
	productID, ok := h.parseUUIDParam(w, r, "id")
	if !ok {
		return
	}

	var req UpdateProductRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	if err := h.validator.Struct(req); err != nil {
		h.writeError(w, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	prod, err := h.service.UpdateProductForSeller(r.Context(), userID, productID, req)
	if err != nil {
		if errors.Is(err, ErrProductNotFound) {
			h.writeError(w, http.StatusNotFound, "not_found", "Product not found")
			return
		}
		if errors.Is(err, ErrInvalidStatusTransition) {
			h.writeError(w, http.StatusUnprocessableEntity, "invalid_status", err.Error())
			return
		}
		if errors.Is(err, ErrDuplicateSlug) {
			h.writeError(w, http.StatusConflict, "duplicate_slug", "Product slug already exists")
			return
		}
		if errors.Is(err, ErrSellerBlocked) {
			h.writeError(w, http.StatusForbidden, "seller_blocked", "Магазин заблокирован или архивирован. Действие недоступно.")
			return
		}
		if errors.Is(err, ErrProductNotEditable) {
			h.writeError(w, http.StatusConflict, "product_not_editable", "Действие с товаром недоступно в его текущем статусе.")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to update product")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(prod)
}

func (h *Handler) DeleteDraftProduct(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.getUserID(w, r)
	if !ok {
		return
	}
	productID, ok := h.parseUUIDParam(w, r, "id")
	if !ok {
		return
	}

	err := h.service.DeleteSellerDraftProduct(r.Context(), userID, productID)
	if err != nil {
		if errors.Is(err, ErrProductNotFound) {
			h.writeError(w, http.StatusNotFound, "not_found", "Product not found or not in draft/rejected state")
			return
		}
		if errors.Is(err, ErrSellerBlocked) {
			h.writeError(w, http.StatusForbidden, "seller_blocked", "Магазин заблокирован или архивирован. Действие недоступно.")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to delete product")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) GetModerationHistory(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.getUserID(w, r)
	if !ok {
		return
	}
	productID, ok := h.parseUUIDParam(w, r, "id")
	if !ok {
		return
	}

	logs, err := h.service.GetProductModerationHistory(r.Context(), userID, productID)
	if err != nil {
		if errors.Is(err, ErrProductNotFound) {
			h.writeError(w, http.StatusNotFound, "not_found", "Product not found")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to retrieve moderation history")
		return
	}

	items := make([]ModerationHistoryItem, 0, len(logs))
	for _, l := range logs {
		items = append(items, ModerationHistoryItem{
			ID:         l.ID,
			ProductID:  l.ProductID,
			FromStatus: l.FromStatus,
			ToStatus:   l.ToStatus,
			Comment:    l.Comment,
			CreatedAt:  l.CreatedAt.Format(time.RFC3339),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ModerationHistoryResponse{Items: items})
}

func (h *Handler) SubmitForModeration(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.getUserID(w, r)
	if !ok {
		return
	}
	productID, ok := h.parseUUIDParam(w, r, "id")
	if !ok {
		return
	}

	var req SubmitProductModerationRequest
	if r.Body != nil && r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			h.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
			return
		}
	}

	err := h.service.SubmitProductToModeration(r.Context(), userID, productID, req)
	if err != nil {
		if errors.Is(err, ErrProductNotFound) {
			h.writeError(w, http.StatusNotFound, "not_found", "Product not found")
			return
		}
		if errors.Is(err, ErrInvalidStatusTransition) {
			h.writeError(w, http.StatusUnprocessableEntity, "invalid_status", err.Error())
			return
		}
		if errors.Is(err, ErrSellerBlocked) {
			h.writeError(w, http.StatusForbidden, "seller_blocked", "Магазин заблокирован или архивирован. Действие недоступно.")
			return
		}
		if errors.Is(err, ErrSellerNotActive) {
			h.writeError(w, http.StatusForbidden, "seller_not_active", "Магазин ещё не активирован. Отправка товаров на модерацию будет доступна после проверки.")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to submit product")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// ---------------------------------------------------------
// Admin Handlers
// ---------------------------------------------------------

func (h *Handler) ListAdminProducts(w http.ResponseWriter, r *http.Request) {
	page := pagination.FromRequest(r)
	resp, err := h.service.ListAdminProducts(r.Context(), page.Limit, page.Offset)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to list products")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) ListModerationProducts(w http.ResponseWriter, r *http.Request) {
	page := pagination.FromRequest(r)
	resp, err := h.service.ListProductsForModeration(r.Context(), page.Limit, page.Offset)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to list products")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) AdminApproveProduct(w http.ResponseWriter, r *http.Request) {
	h.handleAdminModerationAction(w, r, "product.approve", func(ctx context.Context, adminID, productID uuid.UUID, comment *string) error {
		return h.service.ApproveProduct(ctx, adminID, productID, comment)
	})
}

func (h *Handler) AdminPublishProduct(w http.ResponseWriter, r *http.Request) {
	h.handleAdminModerationAction(w, r, "product.publish", func(ctx context.Context, adminID, productID uuid.UUID, comment *string) error {
		return h.service.PublishProduct(ctx, adminID, productID, comment)
	})
}

func (h *Handler) AdminHideProduct(w http.ResponseWriter, r *http.Request) {
	h.handleAdminModerationAction(w, r, "product.hide", func(ctx context.Context, adminID, productID uuid.UUID, comment *string) error {
		return h.service.HideProduct(ctx, adminID, productID, comment)
	})
}

func (h *Handler) AdminBlockProduct(w http.ResponseWriter, r *http.Request) {
	h.handleAdminModerationAction(w, r, "product.block", func(ctx context.Context, adminID, productID uuid.UUID, comment *string) error {
		return h.service.BlockProduct(ctx, adminID, productID, comment)
	})
}

func (h *Handler) AdminRejectProduct(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.getUserID(w, r)
	if !ok {
		return
	}
	productID, ok := h.parseUUIDParam(w, r, "id")
	if !ok {
		return
	}

	var req RejectProductRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}
	if err := h.validator.Struct(req); err != nil {
		h.writeError(w, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	err := h.service.RejectProduct(r.Context(), userID, productID, req.Comment)
	if err != nil {
		if errors.Is(err, ErrProductNotFound) {
			h.writeError(w, http.StatusNotFound, "not_found", "Product not found")
			return
		}
		if errors.Is(err, ErrInvalidStatusTransition) {
			h.writeError(w, http.StatusUnprocessableEntity, "invalid_status", err.Error())
			return
		}
		if errors.Is(err, ErrRejectionReasonRequired) {
			h.writeError(w, http.StatusBadRequest, "reason_required", err.Error())
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to reject product")
		return
	}

	// Audit on success
	if h.auditRepo != nil {
		pid := productID
		actorID := userID
		go func() {
			_ = h.auditRepo.RecordAudit(context.Background(), staff.AuditEvent{
				ActorUserID: actorID,
				Action:      "product.reject",
				EntityType:  "product",
				EntityID:    &pid,
				Metadata:    staff.SanitizeMetadata(map[string]any{"action": "reject"}),
			})
		}()
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (h *Handler) handleAdminModerationAction(w http.ResponseWriter, r *http.Request, auditAction string, action func(context.Context, uuid.UUID, uuid.UUID, *string) error) {
	userID, ok := h.getUserID(w, r)
	if !ok {
		return
	}
	productID, ok := h.parseUUIDParam(w, r, "id")
	if !ok {
		return
	}

	var req AdminProductModerationRequest
	if r.Body != nil && r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			h.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
			return
		}
	}

	err := action(r.Context(), userID, productID, req.Comment)
	if err != nil {
		if errors.Is(err, ErrProductNotFound) {
			h.writeError(w, http.StatusNotFound, "not_found", "Product not found")
			return
		}
		if errors.Is(err, ErrInvalidStatusTransition) {
			h.writeError(w, http.StatusUnprocessableEntity, "invalid_status", err.Error())
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to perform moderation action")
		return
	}

	// Audit on success
	if h.auditRepo != nil && auditAction != "" {
		pid := productID
		actorID := userID
		go func() {
			_ = h.auditRepo.RecordAudit(context.Background(), staff.AuditEvent{
				ActorUserID: actorID,
				Action:      auditAction,
				EntityType:  "product",
				EntityID:    &pid,
				Metadata:    staff.SanitizeMetadata(map[string]any{"action": auditAction}),
			})
		}()
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// ---------------------------------------------------------
// Public Handlers
// ---------------------------------------------------------

func (h *Handler) ListPublicProducts(w http.ResponseWriter, r *http.Request) {
	page := pagination.FromRequest(r)
	
	filter := PublicProductFilter{}
	q := r.URL.Query().Get("q")
	if q != "" {
		filter.Query = &q
	}
	
	if catID := r.URL.Query().Get("categoryId"); catID != "" && catID != "all" {
		if id, err := uuid.Parse(catID); err == nil {
			filter.CategoryID = &id
		}
	}
	
	if brandID := r.URL.Query().Get("brandId"); brandID != "" && brandID != "all" {
		if id, err := uuid.Parse(brandID); err == nil {
			filter.BrandID = &id
		}
	}
	
	if sort := r.URL.Query().Get("sort"); sort != "" {
		filter.Sort = &sort
	}

	// We can parse minPriceCents and maxPriceCents, but we need strconv.
	// Since we haven't imported strconv in handler.go directly, let's just parse it using fmt or simple string parsing, wait, strconv is better.
	// I will just add strconv import or simply use Sscanf.
	if minPrice := r.URL.Query().Get("minPriceCents"); minPrice != "" {
		var min int64
		if _, err := fmt.Sscanf(minPrice, "%d", &min); err == nil {
			filter.MinPriceCents = &min
		}
	}

	if maxPrice := r.URL.Query().Get("maxPriceCents"); maxPrice != "" {
		var max int64
		if _, err := fmt.Sscanf(maxPrice, "%d", &max); err == nil {
			filter.MaxPriceCents = &max
		}
	}

	if inStock := r.URL.Query().Get("inStock"); inStock == "true" {
		b := true
		filter.InStock = &b
	}

	resp, err := h.service.ListPublicProducts(r.Context(), filter, page.Limit, page.Offset)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to list products")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) GetPublicProduct(w http.ResponseWriter, r *http.Request) {
	idOrSlug := chi.URLParam(r, "idOrSlug")
	if idOrSlug == "" {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "Missing id or slug")
		return
	}

	prod, err := h.service.GetPublicProduct(r.Context(), idOrSlug)
	if err != nil {
		if errors.Is(err, ErrProductNotFound) {
			h.writeError(w, http.StatusNotFound, "not_found", "Product not found")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to get product")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(prod)
}

func (h *Handler) GetPublicSellerStore(w http.ResponseWriter, r *http.Request) {
	idOrSlug := chi.URLParam(r, "idOrSlug")
	if idOrSlug == "" {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "Missing seller id or slug")
		return
	}

	seller, err := h.sellersService.GetPublicSeller(r.Context(), idOrSlug)
	if err != nil {
		h.writeError(w, http.StatusNotFound, "not_found", "Seller not found or not available")
		return
	}

	filter := PublicProductFilter{SellerID: &seller.ID}
	
	if q := r.URL.Query().Get("q"); q != "" {
		filter.Query = &q
	}
	if minPriceStr := r.URL.Query().Get("minPriceCents"); minPriceStr != "" {
		var p int64
		fmt.Sscanf(minPriceStr, "%d", &p)
		filter.MinPriceCents = &p
	}
	if maxPriceStr := r.URL.Query().Get("maxPriceCents"); maxPriceStr != "" {
		var p int64
		fmt.Sscanf(maxPriceStr, "%d", &p)
		filter.MaxPriceCents = &p
	}
	if inStockStr := r.URL.Query().Get("inStock"); inStockStr == "true" {
		t := true
		filter.InStock = &t
	}
	if sort := r.URL.Query().Get("sort"); sort != "" {
		filter.Sort = &sort
	}

	p := pagination.FromRequest(r)
	limit, offset := p.Limit, p.Offset

	listResp, err := h.service.ListPublicProducts(r.Context(), filter, limit, offset)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to get seller products")
		return
	}

	type sellerStoreResponse struct {
		Seller   any `json:"seller"`
		Products any `json:"products"`
	}

	productsData := map[string]any{
		"items":      listResp.Items,
		"limit":      limit,
		"offset":     offset,
		"totalCount": listResp.TotalCount,
	}

	resp := sellerStoreResponse{
		Seller:   seller,
		Products: productsData,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
