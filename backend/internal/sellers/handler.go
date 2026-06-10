package sellers

import (
	"context"
	"encoding/json"
	"errors"
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

	if req.BrandName == "" || req.ContactEmail == "" || req.OwnerEmail == "" || req.TemporaryPassword == "" {
		h.respondError(w, http.StatusBadRequest, "missing required fields")
		return
	}

	res, err := h.service.CreateSellerByAdmin(r.Context(), &req)
	if err != nil {
		if errors.Is(err, ErrDuplicateSlug) || errors.Is(err, ErrDuplicateEmail) {
			h.respondError(w, http.StatusConflict, err.Error())
			return
		}
		h.respondError(w, http.StatusInternalServerError, "failed to create seller")
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
				Metadata:    staff.SanitizeMetadata(map[string]any{"ownerEmail": req.OwnerEmail, "brandName": req.BrandName}),
			})
		}()
	}

	h.respondJSON(w, http.StatusCreated, res)
}

func (h *Handler) ListSellers(w http.ResponseWriter, r *http.Request) {
	page := pagination.FromRequest(r)
	res, err := h.service.ListSellers(r.Context(), page.Limit, page.Offset)
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, "failed to list sellers")
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

	var req UpdateSellerStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.service.UpdateSellerStatus(r.Context(), sellerID, &req); err != nil {
		if errors.Is(err, ErrSellerNotFound) {
			h.respondError(w, http.StatusNotFound, err.Error())
			return
		}
		h.respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	if h.auditRepo != nil {
		actorID, _ := r.Context().Value("userID").(uuid.UUID)
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
				Metadata:    staff.SanitizeMetadata(map[string]any{"newStatus": req.Status}),
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
