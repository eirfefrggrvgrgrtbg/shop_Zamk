package reviews

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/http/pagination"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/staff"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/users"
)

type Handler struct {
	svc       *Service
	staffSvc  *staff.Service
	auditRepo *staff.AuditRepository
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) WithAudit(ar *staff.AuditRepository) *Handler {
	h.auditRepo = ar
	return h
}

func (h *Handler) WithStaffSvc(ss *staff.Service) *Handler {
	h.staffSvc = ss
	return h
}

// Customer Endpoints
func (h *Handler) CreateCustomerReview(w http.ResponseWriter, r *http.Request) {
	val := r.Context().Value("userID")
	if val == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	userID := val.(uuid.UUID)

	var req CreateReviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	var pathOrderID *uuid.UUID
	paramOrderID := chi.URLParam(r, "orderId")
	if paramOrderID != "" {
		parsed, err := uuid.Parse(paramOrderID)
		if err != nil {
			http.Error(w, "invalid orderId in url", http.StatusBadRequest)
			return
		}
		pathOrderID = &parsed
	}

	var orderItemID uuid.UUID
	paramOrderItemID := chi.URLParam(r, "orderItemId")
	if paramOrderItemID != "" {
		parsed, err := uuid.Parse(paramOrderItemID)
		if err != nil {
			http.Error(w, "invalid orderItemId in url", http.StatusBadRequest)
			return
		}
		orderItemID = parsed
	} else if req.OrderItemID != nil && *req.OrderItemID != uuid.Nil {
		orderItemID = *req.OrderItemID
	} else {
		http.Error(w, "orderItemId is required", http.StatusBadRequest)
		return
	}

	rev, err := h.svc.CreateReview(r.Context(), userID, orderItemID, pathOrderID, req)
	if err != nil {
		if errors.Is(err, ErrInvalidRating) || errors.Is(err, ErrReviewTextTooLong) || errors.Is(err, ErrOrderNotDelivered) || errors.Is(err, ErrItemNotPurchased) {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if errors.Is(err, ErrReviewAlreadyExists) {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(mapToReviewResponse(rev))
}

func (h *Handler) GetCustomerReviews(w http.ResponseWriter, r *http.Request) {
	val := r.Context().Value("userID")
	if val == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	userID := val.(uuid.UUID)

	page := pagination.FromRequest(r)
	reviews, err := h.svc.GetCustomerReviews(r.Context(), userID, page.Limit, page.Offset)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	res := make([]ReviewResponse, len(reviews))
	for i, v := range reviews {
		res[i] = mapToReviewResponse(&v)
	}
	json.NewEncoder(w).Encode(ReviewListResponse{Items: res, TotalCount: len(res)})
}

func (h *Handler) GetCustomerReview(w http.ResponseWriter, r *http.Request) {
	val := r.Context().Value("userID")
	if val == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	userID := val.(uuid.UUID)
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	rev, err := h.svc.GetCustomerReviewByID(r.Context(), userID, id)
	if err != nil {
		if errors.Is(err, ErrReviewNotFound) {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(mapToReviewResponse(rev))
}

// Admin Endpoints
func (h *Handler) GetAdminReviews(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	page := pagination.FromRequest(r)
	reviews, err := h.svc.GetAdminReviews(r.Context(), status, page.Limit, page.Offset)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	res := make([]ReviewResponse, len(reviews))
	for i, v := range reviews {
		res[i] = mapToReviewResponse(&v)
	}
	json.NewEncoder(w).Encode(ReviewListResponse{Items: res, TotalCount: len(res)})
}

func (h *Handler) GetAdminReview(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	rev, err := h.svc.GetAdminReviewByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrReviewNotFound) {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(mapToReviewResponse(rev))
}

func (h *Handler) ModerateReview(w http.ResponseWriter, r *http.Request) {
	val := r.Context().Value("userID")
	if val == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	adminID := val.(uuid.UUID)
	idStr := chi.URLParam(r, "id")
	action := chi.URLParam(r, "action") // approve, reject, hide, block

	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	// Handler-level dynamic permission check based on action
	if h.staffSvc != nil {
		permMap := map[string]string{
			"approve": "reviews.approve",
			"reject":  "reviews.reject",
			"hide":    "reviews.hide",
			"block":   "reviews.block",
		}
		requiredPerm, known := permMap[action]
		if !known {
			http.Error(w, "invalid action", http.StatusBadRequest)
			return
		}
		ok, permErr := h.staffSvc.HasPermission(r.Context(), adminID, requiredPerm)
		if permErr != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "internal_error", "message": "Permission check failed"})
			return
		}
		if !ok {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(map[string]string{"error": "insufficient_permissions", "message": "Недостаточно прав"})
			return
		}
	}

	var req AdminModerationRequest
	if r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
	}

	statusMap := map[string]string{
		"approve": "published",
		"reject":  "rejected",
		"hide":    "hidden",
		"block":   "blocked",
	}

	toStatus, ok := statusMap[action]
	if !ok {
		http.Error(w, "invalid action", http.StatusBadRequest)
		return
	}

	if toStatus == "rejected" && (req.Comment == nil || *req.Comment == "") {
		http.Error(w, "comment required for rejection", http.StatusBadRequest)
		return
	}

	err = h.svc.ModerateReview(r.Context(), adminID, id, toStatus, req.Comment)
	if err != nil {
		if errors.Is(err, ErrReviewNotFound) {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	// Audit on success
	if h.auditRepo != nil {
		rid := id
		actorID := adminID
		auditAction := "review." + action
		go func() {
			_ = h.auditRepo.RecordAudit(context.Background(), staff.AuditEvent{
				ActorUserID: actorID,
				Action:      auditAction,
				EntityType:  "review",
				EntityID:    &rid,
				Metadata:    staff.SanitizeMetadata(map[string]any{"action": action}),
			})
		}()
	}

	w.WriteHeader(http.StatusOK)
}

// Seller Endpoints
func (h *Handler) GetSellerReviews(w http.ResponseWriter, r *http.Request) {
	val := r.Context().Value("userID")
	if val == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	userID := val.(uuid.UUID)
	page := pagination.FromRequest(r)
	reviews, err := h.svc.GetSellerReviews(r.Context(), userID, page.Limit, page.Offset)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	res := make([]ReviewResponse, len(reviews))
	for i, v := range reviews {
		res[i] = mapToReviewResponse(&v)
	}
	json.NewEncoder(w).Encode(ReviewListResponse{Items: res, TotalCount: len(res)})
}

func (h *Handler) GetSellerReview(w http.ResponseWriter, r *http.Request) {
	val := r.Context().Value("userID")
	if val == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	userID := val.(uuid.UUID)
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	rev, err := h.svc.GetSellerReviewByID(r.Context(), userID, id)
	if err != nil {
		if errors.Is(err, ErrReviewNotFound) {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(mapToReviewResponse(rev))
}

// Public Endpoints
func (h *Handler) GetPublicProductReviews(w http.ResponseWriter, r *http.Request) {
	idOrSlug := chi.URLParam(r, "idOrSlug")
	if strings.TrimSpace(idOrSlug) == "" {
		http.Error(w, "invalid product id or slug", http.StatusBadRequest)
		return
	}

	productID, err := h.svc.ResolvePublishedProductID(r.Context(), idOrSlug)
	if err != nil {
		if errors.Is(err, ErrProductNotFound) {
			http.Error(w, "product not found", http.StatusNotFound)
			return
		}
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	page := pagination.FromRequest(r)
	reviews, err := h.svc.GetPublicProductReviews(r.Context(), productID, page.Limit, page.Offset)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	res := make([]PublicReviewResponse, len(reviews))
	for i, v := range reviews {
		res[i] = mapToPublicReviewResponse(&v)
	}

	summary, err := h.svc.GetRatingSummary(r.Context(), productID)
	var avg float64
	var count int
	if err == nil && summary != nil {
		avg = summary.Average
		count = summary.Count
	}

	json.NewEncoder(w).Encode(PublicReviewListResponse{
		Items:         res,
		AverageRating: avg,
		ReviewCount:   count,
		TotalCount:    count,
	})
}

func (h *Handler) GetPublicRatingSummary(w http.ResponseWriter, r *http.Request) {
	idOrSlug := chi.URLParam(r, "idOrSlug")
	if strings.TrimSpace(idOrSlug) == "" {
		http.Error(w, "invalid product id or slug", http.StatusBadRequest)
		return
	}

	productID, err := h.svc.ResolvePublishedProductID(r.Context(), idOrSlug)
	if err != nil {
		if errors.Is(err, ErrProductNotFound) {
			http.Error(w, "product not found", http.StatusNotFound)
			return
		}
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	summary, err := h.svc.GetRatingSummary(r.Context(), productID)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(summary)
}

func mapToReviewResponse(rev *ProductReview) ReviewResponse {
	return ReviewResponse{
		ID:                rev.ID,
		ProductID:         rev.ProductID,
		ProductVariantID:  rev.ProductVariantID,
		ProductTitle:      rev.ProductTitle,
		Rating:            rev.Rating,
		Title:             rev.Title,
		Comment:           rev.Comment,
		Text:              rev.Comment,
		Status:            rev.Status,
		CreatedAt:         rev.CreatedAt,
		PublishedAt:       rev.PublishedAt,
		ModerationComment: rev.ModerationComment,
	}
}

func mapToPublicReviewResponse(rev *PublicReviewRow) PublicReviewResponse {
	name := strings.TrimSpace(rev.ReviewerFirstName)
	if name == "" {
		name = "Покупатель"
	}
	return PublicReviewResponse{
		ID:                  rev.ID,
		Rating:              rev.Rating,
		Title:               rev.Title,
		Comment:             rev.Comment,
		Text:                rev.Comment,
		ReviewerDisplayName: name,
		AuthorName:          name,
		ProductTitle:        rev.OrderItemTitle,
		VariantSize:         rev.OrderItemSize,
		VariantColor:        rev.OrderItemColor,
		CreatedAt:           rev.CreatedAt,
	}
}

var _ = users.RoleCustomer
