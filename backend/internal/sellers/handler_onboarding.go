package sellers

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func (h *Handler) getSellerID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	val := r.Context().Value("userID")
	if val == nil {
		h.respondError(w, http.StatusUnauthorized, "unauthorized")
		return uuid.Nil, false
	}
	userID, ok := val.(uuid.UUID)
	if !ok {
		h.respondError(w, http.StatusUnauthorized, "unauthorized")
		return uuid.Nil, false
	}

	res, err := h.service.GetSellerMe(r.Context(), userID)
	if err != nil {
		h.respondError(w, http.StatusNotFound, "seller profile not found")
		return uuid.Nil, false
	}
	return res.Seller.ID, true
}

func (h *Handler) getUserID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	val := r.Context().Value("userID")
	if val == nil {
		h.respondError(w, http.StatusUnauthorized, "unauthorized")
		return uuid.Nil, false
	}
	userID, ok := val.(uuid.UUID)
	if !ok {
		h.respondError(w, http.StatusUnauthorized, "unauthorized")
		return uuid.Nil, false
	}
	return userID, true
}

func (h *Handler) InviteSeller(w http.ResponseWriter, r *http.Request) {
	var req InviteSellerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	res, err := h.service.InviteSeller(r.Context(), &req)
	if err != nil {
		if err == ErrDuplicateEmail {
			h.respondError(w, http.StatusConflict, "Email already in use")
			return
		}
		h.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.respondJSON(w, http.StatusCreated, res)
}

func (h *Handler) GetOnboardingApplication(w http.ResponseWriter, r *http.Request) {
	sellerID, ok := h.getSellerID(w, r)
	if !ok {
		return
	}

	app, err := h.service.GetOnboardingApplication(r.Context(), sellerID)
	if err != nil {
		if err == ErrOnboardingNotFound {
			h.respondError(w, http.StatusNotFound, "Onboarding application not found")
			return
		}
		h.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.respondJSON(w, http.StatusOK, app)
}

func (h *Handler) UpdateOnboardingStep(w http.ResponseWriter, r *http.Request) {
	sellerID, ok := h.getSellerID(w, r)
	if !ok {
		return
	}

	var req UpdateOnboardingStepRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	app, err := h.service.UpdateOnboardingStep(r.Context(), sellerID, &req)
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.respondJSON(w, http.StatusOK, app)
}

func (h *Handler) SubmitOnboarding(w http.ResponseWriter, r *http.Request) {
	sellerID, ok := h.getSellerID(w, r)
	if !ok {
		return
	}

	var req SubmitOnboardingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	app, err := h.service.SubmitOnboarding(r.Context(), sellerID, &req)
	if err != nil {
		if err == ErrStoreSlugTaken || err == ErrBrandSlugTaken {
			h.respondError(w, http.StatusConflict, err.Error())
			return
		}
		h.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.respondJSON(w, http.StatusOK, app)
}

func (h *Handler) ListOnboardingApplications(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	apps, err := h.service.ListOnboardingApplications(r.Context(), status)
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.respondJSON(w, http.StatusOK, apps)
}

func (h *Handler) GetAdminOnboardingApplication(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "Invalid ID format")
		return
	}

	app, err := h.service.GetAdminOnboardingApplication(r.Context(), id)
	if err != nil {
		if err == ErrOnboardingNotFound {
			h.respondError(w, http.StatusNotFound, "Onboarding application not found")
			return
		}
		h.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.respondJSON(w, http.StatusOK, app)
}

func (h *Handler) RequestChangesOnboarding(w http.ResponseWriter, r *http.Request) {
	adminID, ok := h.getUserID(w, r)
	if !ok {
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "Invalid ID format")
		return
	}

	var req RequestChangesOnboardingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := h.service.RequestChangesOnboarding(r.Context(), adminID, id, &req); err != nil {
		h.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.respondJSON(w, http.StatusOK, map[string]string{"status": "changes_requested"})
}

func (h *Handler) RejectOnboarding(w http.ResponseWriter, r *http.Request) {
	adminID, ok := h.getUserID(w, r)
	if !ok {
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "Invalid ID format")
		return
	}

	var req RejectOnboardingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := h.service.RejectOnboarding(r.Context(), adminID, id, &req); err != nil {
		h.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.respondJSON(w, http.StatusOK, map[string]string{"status": "rejected"})
}

func (h *Handler) ApproveOnboarding(w http.ResponseWriter, r *http.Request) {
	adminID, ok := h.getUserID(w, r)
	if !ok {
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "Invalid ID format")
		return
	}

	var req ApproveOnboardingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := h.service.ApproveOnboarding(r.Context(), adminID, id, &req); err != nil {
		if err == ErrStoreSlugTaken || err == ErrBrandSlugTaken {
			h.respondError(w, http.StatusConflict, err.Error())
			return
		}
		h.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.respondJSON(w, http.StatusOK, map[string]string{"status": "approved"})
}
