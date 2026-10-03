package marketing

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/auth"
)

type Handler struct {
	service *Service
	logger  *slog.Logger
}

func NewHandler(service *Service, logger *slog.Logger) *Handler {
	return &Handler{
		service: service,
		logger:  logger,
	}
}

func (h *Handler) writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"code":    code,
		"message": message,
	})
}

func (h *Handler) writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

// ListSellerPromotions returns all promotions for the authenticated seller.
func (h *Handler) ListSellerPromotions(w http.ResponseWriter, r *http.Request) {
	sellerID, err := auth.GetSellerID(r.Context())
	if err != nil || sellerID == uuid.Nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Seller authorization required")
		return
	}

	promos, err := h.service.ListSellerPromotions(r.Context(), sellerID)
	if err != nil {
		h.logger.Error("failed to list seller promotions", "sellerId", sellerID, "error", err)
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to list promotions")
		return
	}

	if promos == nil {
		promos = []SellerPromoResponse{}
	}

	h.writeJSON(w, http.StatusOK, SellerPromotionsResponse{
		Items: promos,
		Count: len(promos),
	})
}

// CreateSellerPromotion creates a new self-funded promo code for the authenticated seller.
func (h *Handler) CreateSellerPromotion(w http.ResponseWriter, r *http.Request) {
	sellerID, err := auth.GetSellerID(r.Context())
	if err != nil || sellerID == uuid.Nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Seller authorization required")
		return
	}

	var rawMap map[string]json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&rawMap); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_json", "Invalid JSON request body")
		return
	}

	// Security: Verify client did not attempt to spoof seller_id or inject platform funding authority
	for k := range rawMap {
		normalized := strings.ToLower(strings.ReplaceAll(k, "_", ""))
		if normalized == "sellerid" {
			h.writeError(w, http.StatusBadRequest, "invalid_request", "seller_id cannot be specified; determined by authentication")
			return
		}
		if normalized == "fundingmode" {
			h.writeError(w, http.StatusBadRequest, "invalid_request", "funding_mode cannot be specified; seller promotions are self-funded")
			return
		}
		if strings.HasPrefix(normalized, "zamk") || strings.HasPrefix(normalized, "requestedzamk") || strings.HasPrefix(normalized, "approvedzamk") {
			h.writeError(w, http.StatusBadRequest, "invalid_request", "platform funding fields cannot be specified")
			return
		}
	}

	bodyBytes, err := json.Marshal(rawMap)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_json", "Invalid request body format")
		return
	}

	var req CreateSellerPromoRequest
	if err := json.Unmarshal(bodyBytes, &req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_json", "Failed to parse promo request")
		return
	}

	promo, err := h.service.CreateSellerPromotion(r.Context(), sellerID, req)
	if err != nil {
		if errors.Is(err, ErrPromoCodeDuplicate) {
			h.writeError(w, http.StatusConflict, "duplicate_code", "Промокод с таким кодом уже существует")
			return
		}
		if errors.Is(err, ErrPromoAlreadyExpired) {
			h.writeError(w, http.StatusBadRequest, "already_expired", "Нельзя создать промокод с датой окончания в прошлом")
			return
		}
		if errors.Is(err, ErrInvalidDates) {
			h.writeError(w, http.StatusBadRequest, "invalid_dates", "Дата окончания должна быть позже даты начала")
			return
		}
		if errors.Is(err, ErrInvalidDiscountType) || errors.Is(err, ErrInvalidDiscountValue) {
			h.writeError(w, http.StatusBadRequest, "invalid_discount", err.Error())
			return
		}
		if errors.Is(err, ErrInvalidLimit) {
			h.writeError(w, http.StatusBadRequest, "invalid_limit", err.Error())
			return
		}
		if errors.Is(err, ErrProductNotOwnedBySeller) {
			h.writeError(w, http.StatusBadRequest, "foreign_product", err.Error())
			return
		}
		if errors.Is(err, ErrProductConflict) {
			h.writeError(w, http.StatusBadRequest, "product_conflict", err.Error())
			return
		}
		if errors.Is(err, ErrCategoryConflict) {
			h.writeError(w, http.StatusBadRequest, "category_conflict", err.Error())
			return
		}
		if errors.Is(err, ErrInvalidCategory) {
			h.writeError(w, http.StatusBadRequest, "invalid_category", err.Error())
			return
		}
		if errors.Is(err, ErrCategoryRequiresInclude) {
			h.writeError(w, http.StatusBadRequest, "category_requires_include", err.Error())
			return
		}
		if errors.Is(err, ErrCategoryNotAllowed) {
			h.writeError(w, http.StatusBadRequest, "category_not_allowed", err.Error())
			return
		}
		if errors.Is(err, ErrInvalidProductScope) {
			h.writeError(w, http.StatusBadRequest, "invalid_scope", err.Error())
			return
		}
		if errors.Is(err, ErrInvalidMaxDiscount) {
			h.writeError(w, http.StatusBadRequest, "invalid_max_discount", err.Error())
			return
		}
		if errors.Is(err, ErrInvalidMinQuantity) {
			h.writeError(w, http.StatusBadRequest, "invalid_min_quantity", err.Error())
			return
		}
		if errors.Is(err, ErrInvalidMinDistinctProducts) {
			h.writeError(w, http.StatusBadRequest, "invalid_min_distinct_products", err.Error())
			return
		}
		if errors.Is(err, ErrInvalidAudienceType) {
			h.writeError(w, http.StatusBadRequest, "invalid_audience_type", err.Error())
			return
		}
		h.writeError(w, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	h.writeJSON(w, http.StatusCreated, promo)
}

// UpdateSellerPromotion mutates operational limits or active status for a seller promo code.
func (h *Handler) UpdateSellerPromotion(w http.ResponseWriter, r *http.Request) {
	sellerID, err := auth.GetSellerID(r.Context())
	if err != nil || sellerID == uuid.Nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Seller authorization required")
		return
	}

	idStr := chi.URLParam(r, "id")
	promoID, err := uuid.Parse(idStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid promo ID format")
		return
	}

	var rawMap map[string]json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&rawMap); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_json", "Invalid JSON request body")
		return
	}

	// Strictly reject any attempt to mutate immutable economic or identity fields
	forbiddenImmutable := map[string]bool{
		"code":                    true,
		"discounttype":            true,
		"discountvaluebps":        true,
		"discountvaluefixedcents": true,
		"minordersubtotalcents":   true,
		"audiencetype":            true,
		"sellerid":                true,
		"campaignid":              true,
		"fundingmode":             true,
		"productscope":            true,
		"includedproductids":      true,
		"excludedproductids":      true,
		"includedcategoryids":     true,
		"excludedcategoryids":     true,
		"categorytargets":         true,
		"categoryscope":           true,
		"maxdiscountcents":        true,
		"maxdiscountrub":          true,
		"mineligiblequantity":     true,
		"mindistinctproducts":     true,
		"producttargets":          true,
	}

	for k := range rawMap {
		normalized := strings.ToLower(strings.ReplaceAll(k, "_", ""))
		if forbiddenImmutable[normalized] || strings.HasPrefix(normalized, "zamk") {
			h.writeError(w, http.StatusBadRequest, "immutable_field", fmt.Sprintf("Field '%s' is immutable after creation", k))
			return
		}
	}

	bodyBytes, err := json.Marshal(rawMap)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_json", "Invalid request body format")
		return
	}

	var req UpdateSellerPromoRequest
	if err := json.Unmarshal(bodyBytes, &req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_json", "Failed to parse update request")
		return
	}

	updated, err := h.service.UpdateSellerPromotion(r.Context(), sellerID, promoID, req)
	if err != nil {
		if errors.Is(err, ErrPromoCodeNotFound) {
			// Canonical Anti-Enumeration: return 404 Not Found for non-existent or foreign seller promos
			h.writeError(w, http.StatusNotFound, "not_found", "Promo code not found")
			return
		}
		if errors.Is(err, ErrGlobalLimitBelowUsage) || errors.Is(err, ErrCustomerLimitBelowUsage) {
			h.writeError(w, http.StatusBadRequest, "limit_below_usage", err.Error())
			return
		}
		if errors.Is(err, ErrInvalidDates) {
			h.writeError(w, http.StatusBadRequest, "invalid_dates", "Дата окончания должна быть позже даты начала")
			return
		}
		if errors.Is(err, ErrInvalidLimit) {
			h.writeError(w, http.StatusBadRequest, "invalid_limit", err.Error())
			return
		}
		h.writeError(w, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, updated)
}
