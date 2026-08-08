package sellers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/http/pagination"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/staff"
)

var emailRegex = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)

type Handler struct {
	service   *Service
	auditRepo *staff.AuditRepository
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// WithAudit attaches an audit repository for fire-and-forget audit logging.
func (h *Handler) WithAudit(ar *staff.AuditRepository) *Handler {
	h.auditRepo = ar
	return h
}

func (h *Handler) CreateSellerByAdmin(w http.ResponseWriter, r *http.Request) {
	var req CreateSellerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.OwnerName == "" || req.OwnerEmail == "" {
		h.respondError(w, http.StatusBadRequest, "missing required fields (ownerName, ownerEmail)")
		return
	}

	res, err := h.service.CreateSellerByAdmin(r.Context(), &req)
	if err != nil {
		if errors.Is(err, ErrUserExistsPrompt) {
			h.respondJSON(w, http.StatusConflict, map[string]string{
				"error": "USER_EXISTS_PROMPT",
				"message": "User with this email already exists. Grant seller access?",
			})
			return
		}
		if errors.Is(err, ErrSellerAlreadyExists) {
			respData := map[string]interface{}{
				"error":   "SELLER_ALREADY_EXISTS",
				"message": "This user is already a seller.",
			}
			if res != nil {
				respData["sellerId"] = res.Seller.ID
				respData["ownerName"] = res.OwnerUser.Name
				respData["ownerEmail"] = res.OwnerUser.Email
				respData["status"] = res.Seller.Status
			}
			h.respondJSON(w, http.StatusConflict, respData)
			return
		}
		if errors.Is(err, ErrDuplicateSlug) || errors.Is(err, ErrDuplicateEmail) {
			h.respondError(w, http.StatusConflict, err.Error())
			return
		}
		h.respondError(w, http.StatusInternalServerError, "failed to create seller: " + err.Error())
		return
	}

	if h.auditRepo != nil {
		actorID, _ := r.Context().Value("userID").(uuid.UUID)
		actorEmail, _ := r.Context().Value("email").(string)
		actorRole, _ := r.Context().Value("role").(string)
		sellerID := res.Seller.ID
		go func() {
			_ = h.auditRepo.RecordAudit(context.Background(), staff.AuditEvent{
				ActorUserID: actorID,
				ActorEmail:  actorEmail,
				ActorRole:   actorRole,
				Action:      "seller.create_access",
				EntityType:  "seller",
				EntityID:    &sellerID,
				Metadata:    staff.SanitizeMetadata(map[string]any{"ownerEmail": req.OwnerEmail}),
			})
		}()
	}

	h.respondJSON(w, http.StatusCreated, res)
}

func (h *Handler) ListSellers(w http.ResponseWriter, r *http.Request) {
	page := pagination.FromRequest(r)
	q := r.URL.Query()
	
	filter := SellersFilter{
		Limit:     page.Limit,
		Offset:    page.Offset,
		Query:     q.Get("search"),
		Store:     q.Get("store"),
		Problems:  q.Get("problems"),
		PerformanceCategory: q.Get("performanceCategory"),
		Sort:      q.Get("sort"),
		Direction: q.Get("direction"),
	}
	
	if q.Has("status") {
		filter.Status = q["status"] // this handles multi-select arrays if using ?status=1&status=2
	} else if st := q.Get("status"); st != "" && st != "all" {
		filter.Status = []string{st}
	}

	// Parse optional int/float/bool parameters manually
	parseStrToIntPtr := func(key string) *int {
		if v := q.Get(key); v != "" {
			var i int
			if _, err := fmt.Sscanf(v, "%d", &i); err == nil {
				return &i
			}
		}
		return nil
	}
	parseFloatPtr := func(key string) *float64 {
		if v := q.Get(key); v != "" {
			var f float64
			if _, err := fmt.Sscanf(v, "%f", &f); err == nil {
				return &f
			}
		}
		return nil
	}
	parseBoolPtr := func(key string) *bool {
		if v := q.Get(key); v != "" {
			b := (v == "true" || v == "1")
			return &b
		}
		return nil
	}
	parseInt64Ptr := func(key string) *int64 {
		if v := q.Get(key); v != "" {
			var i int64
			if _, err := fmt.Sscanf(v, "%d", &i); err == nil {
				return &i
			}
		}
		return nil
	}

	filter.RatingMin = parseFloatPtr("ratingMin")
	filter.RatingMax = parseFloatPtr("ratingMax")
	filter.HasReviews = parseBoolPtr("hasReviews")
	filter.PerformanceMin = parseStrToIntPtr("performanceMin")
	filter.PerformanceMax = parseStrToIntPtr("performanceMax")
	filter.SalesGrossMin = parseInt64Ptr("salesGrossMin")
	filter.SalesGrossMax = parseInt64Ptr("salesGrossMax")
	filter.OrdersCountMin = parseStrToIntPtr("ordersCountMin")
	filter.OrdersCountMax = parseStrToIntPtr("ordersCountMax")
	filter.HasWarnings = parseBoolPtr("hasWarnings")
	filter.HasViolations = parseBoolPtr("hasViolations")
	filter.Blocked = parseBoolPtr("blocked")

	res, err := h.service.ListSellers(r.Context(), filter)
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, "failed to list sellers: "+err.Error())
		return
	}
	h.respondJSON(w, http.StatusOK, res)
}

func (h *Handler) UpdateSellerStatus(w http.ResponseWriter, r *http.Request) {
	sellerIDStr := chi.URLParam(r, "id")
	sellerID, err := uuid.Parse(sellerIDStr)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid seller ID")
		return
	}

	var req UpdateSellerStatusWithReasonRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	actorID, _ := r.Context().Value("userID").(uuid.UUID)
	if err := h.service.UpdateSellerStatusWithHistory(r.Context(), sellerID, req.Status, req.Reason, actorID); err != nil {
		if errors.Is(err, ErrSellerNotFound) {
			h.respondError(w, http.StatusNotFound, err.Error())
			return
		}
		if errors.Is(err, ErrReasonRequired) {
			h.respondError(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		h.respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	if h.auditRepo != nil {
		actorEmail, _ := r.Context().Value("email").(string)
		actorRole, _ := r.Context().Value("role").(string)
		sid := sellerID
		go func() {
			_ = h.auditRepo.RecordAudit(context.Background(), staff.AuditEvent{
				ActorUserID: actorID,
				ActorEmail:  actorEmail,
				ActorRole:   actorRole,
				Action:      "seller.status_update",
				EntityType:  "seller",
				EntityID:    &sid,
				Metadata:    staff.SanitizeMetadata(map[string]any{"newStatus": req.Status, "reason": req.Reason}),
			})
		}()
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) GetSellerMe(w http.ResponseWriter, r *http.Request) {
	val := r.Context().Value("userID")
	if val == nil {
		h.respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	userID, ok := val.(uuid.UUID)
	if !ok {
		h.respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	res, err := h.service.GetSellerMe(r.Context(), userID)
	if err != nil {
		if errors.Is(err, ErrSellerUserNotFound) {
			h.respondError(w, http.StatusNotFound, "seller profile not found for user")
			return
		}
		h.respondError(w, http.StatusInternalServerError, "failed to get seller profile")
		return
	}

	h.respondJSON(w, http.StatusOK, res)
}

func (h *Handler) UpdateSellerProfile(w http.ResponseWriter, r *http.Request) {
	val := r.Context().Value("userID")
	if val == nil {
		h.respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	userID, ok := val.(uuid.UUID)
	if !ok {
		h.respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req UpdateSellerProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.ContactEmail != nil && *req.ContactEmail != "" && !emailRegex.MatchString(*req.ContactEmail) {
		h.respondError(w, http.StatusBadRequest, "invalid contact email format")
		return
	}

	res, err := h.service.UpdateSellerProfile(r.Context(), userID, &req)
	if err != nil {
		if errors.Is(err, ErrSellerUserNotFound) || errors.Is(err, ErrSellerNotFound) {
			h.respondError(w, http.StatusNotFound, "seller profile not found")
			return
		}
		if errors.Is(err, ErrDuplicateSlug) {
			h.respondError(w, http.StatusConflict, "slug already taken")
			return
		}
		h.respondError(w, http.StatusInternalServerError, "failed to update seller profile")
		return
	}

	h.respondJSON(w, http.StatusOK, res)
}

func (h *Handler) CompleteOnboarding(w http.ResponseWriter, r *http.Request) {
	val := r.Context().Value("userID")
	if val == nil {
		h.respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	userID, ok := val.(uuid.UUID)
	if !ok {
		h.respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	err := h.service.CompleteOnboarding(r.Context(), userID)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ResetOwnerPassword(w http.ResponseWriter, r *http.Request) {
	sellerIDStr := chi.URLParam(r, "id")
	sellerID, err := uuid.Parse(sellerIDStr)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid seller ID")
		return
	}

	tempPassword, err := h.service.ResetOwnerPassword(r.Context(), sellerID)
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, "failed to reset password")
		return
	}

	h.respondJSON(w, http.StatusOK, map[string]string{"temporaryPassword": tempPassword})
}

func (h *Handler) respondJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if data != nil {
		_ = json.NewEncoder(w).Encode(data)
	}
}

func (h *Handler) respondError(w http.ResponseWriter, status int, message string) {
	h.respondJSON(w, status, map[string]string{"error": message})
}

// RequireActiveSeller is a middleware that ensures the current user is an active seller
func (h *Handler) RequireActiveSeller(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		val := r.Context().Value("userID")
		if val == nil {
			h.respondError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		userID, ok := val.(uuid.UUID)
		if !ok {
			h.respondError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		res, err := h.service.GetSellerMe(r.Context(), userID)
		if err != nil {
			if errors.Is(err, ErrSellerUserNotFound) {
				h.respondError(w, http.StatusForbidden, "seller profile not found")
				return
			}
			h.respondError(w, http.StatusInternalServerError, "failed to get seller profile")
			return
		}

		if res.Seller.Status != "active" {
			h.respondError(w, http.StatusForbidden, "seller account is not active")
			return
		}
        
        ctx := context.WithValue(r.Context(), "sellerID", res.Seller.ID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
