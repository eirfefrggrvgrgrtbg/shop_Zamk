package reviews

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/http/pagination"
	"github.com/google/uuid"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Customer Endpoints
func (h *Handler) CreateCustomerReview(w http.ResponseWriter, r *http.Request) {
	val := r.Context().Value("userID")
	if val == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	userID := val.(uuid.UUID)
	orderIDStr := chi.URLParam(r, "orderId")
	orderItemIDStr := chi.URLParam(r, "orderItemId")

	orderID, err := uuid.Parse(orderIDStr)
	if err != nil {
		http.Error(w, "invalid order ID", http.StatusBadRequest)
		return
	}
	orderItemID, err := uuid.Parse(orderItemIDStr)
	if err != nil {
		http.Error(w, "invalid order item ID", http.StatusBadRequest)
		return
	}

	var req CreateReviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	rev, err := h.svc.CreateReview(r.Context(), userID, orderID, orderItemID, req)
	if err != nil {
		if err == ErrInvalidRating || err == ErrItemNotPurchased || err == ErrOrderNotDelivered {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err == ErrDuplicateReview {
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
		if err == ErrReviewNotFound {
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
		if err == ErrReviewNotFound {
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
		if err == ErrReviewNotFound {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
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
		if err == ErrReviewNotFound {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(mapToReviewResponse(rev))
}

// Public Endpoints (Usually handled through Catalog, but can expose directly)
func (h *Handler) GetPublicProductReviews(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "idOrSlug") // assuming productId for now
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "invalid product id", http.StatusBadRequest)
		return
	}

	page := pagination.FromRequest(r)
	reviews, err := h.svc.GetPublicProductReviews(r.Context(), id, page.Limit, page.Offset)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	res := make([]PublicReviewResponse, len(reviews))
	for i, v := range reviews {
		res[i] = mapToPublicReviewResponse(&v)
	}
	json.NewEncoder(w).Encode(PublicReviewListResponse{Items: res, TotalCount: len(res)})
}

func (h *Handler) GetPublicRatingSummary(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "idOrSlug")
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "invalid product id", http.StatusBadRequest)
		return
	}
	summary, err := h.svc.GetRatingSummary(r.Context(), id)
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
		Rating:            rev.Rating,
		Title:             rev.Title,
		Comment:           rev.Comment,
		Status:            rev.Status,
		CreatedAt:         rev.CreatedAt,
		PublishedAt:       rev.PublishedAt,
		ModerationComment: rev.ModerationComment,
	}
}

func mapToPublicReviewResponse(rev *ProductReview) PublicReviewResponse {
	return PublicReviewResponse{
		ID:         rev.ID,
		Rating:     rev.Rating,
		Title:      rev.Title,
		Comment:    rev.Comment,
		AuthorName: "Customer", // masked
		CreatedAt:  rev.CreatedAt,
	}
}
