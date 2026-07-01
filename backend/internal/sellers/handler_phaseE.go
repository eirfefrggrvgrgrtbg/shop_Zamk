package sellers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/staff"
)

func (h *Handler) GetAdminSellerDetail(w http.ResponseWriter, r *http.Request) {
	sellerIDStr := chi.URLParam(r, "id")
	sellerID, err := uuid.Parse(sellerIDStr)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid seller ID")
		return
	}
	resp, err := h.service.GetSellerDetail(r.Context(), sellerID)
	if err != nil {
		if errors.Is(err, ErrSellerNotFound) {
			h.respondError(w, http.StatusNotFound, err.Error())
			return
		}
		h.respondError(w, http.StatusInternalServerError, "failed to get seller detail")
		return
	}
	h.respondJSON(w, http.StatusOK, resp)
}

func (h *Handler) VerifySeller(w http.ResponseWriter, r *http.Request) {
	sellerIDStr := chi.URLParam(r, "id")
	sellerID, err := uuid.Parse(sellerIDStr)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid seller ID")
		return
	}
	actorID, _ := r.Context().Value("userID").(uuid.UUID)
	resp, err := h.service.VerifySeller(r.Context(), sellerID, actorID)
	if err != nil {
		var mfe *VerifyMissingFieldsError
		if errors.As(err, &mfe) {
			h.respondJSON(w, http.StatusUnprocessableEntity, VerifySellerMissingFieldsError{
				Error:         "seller profile is incomplete",
				MissingFields: mfe.Fields,
			})
			return
		}
		if errors.Is(err, ErrSellerNotFound) {
			h.respondError(w, http.StatusNotFound, err.Error())
			return
		}
		if errors.Is(err, ErrSellerNotPending) {
			h.respondError(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		h.respondError(w, http.StatusInternalServerError, "failed to verify seller")
		return
	}
	if h.auditRepo != nil {
		actorEmail, _ := r.Context().Value("email").(string)
		actorRole, _ := r.Context().Value("role").(string)
		sid := sellerID
		go func() {
			_ = h.auditRepo.RecordAudit(context.Background(), staff.AuditEvent{
				ActorUserID: actorID, ActorEmail: actorEmail, ActorRole: actorRole,
				Action: "seller.verify", EntityType: "seller", EntityID: &sid,
				Metadata: staff.SanitizeMetadata(map[string]any{"newStatus": "active"}),
			})
		}()
	}
	h.respondJSON(w, http.StatusOK, resp)
}

func (h *Handler) GetSellerStatusHistory(w http.ResponseWriter, r *http.Request) {
	sellerIDStr := chi.URLParam(r, "id")
	sellerID, err := uuid.Parse(sellerIDStr)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid seller ID")
		return
	}
	items, err := h.service.GetStatusHistory(r.Context(), sellerID)
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, "failed to get status history")
		return
	}
	if items == nil {
		items = []SellerStatusHistoryItem{}
	}
	h.respondJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) ListSellerWarnings(w http.ResponseWriter, r *http.Request) {
	sellerIDStr := chi.URLParam(r, "id")
	sellerID, err := uuid.Parse(sellerIDStr)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid seller ID")
		return
	}
	items, err := h.service.ListWarnings(r.Context(), sellerID)
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, "failed to list warnings")
		return
	}
	if items == nil {
		items = []WarningResponse{}
	}
	h.respondJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) CreateSellerWarning(w http.ResponseWriter, r *http.Request) {
	sellerIDStr := chi.URLParam(r, "id")
	sellerID, err := uuid.Parse(sellerIDStr)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid seller ID")
		return
	}
	var req CreateWarningRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Type == "" || req.Title == "" || req.Message == "" || req.Severity == "" {
		h.respondError(w, http.StatusBadRequest, "type, title, message and severity are required")
		return
	}
	actorID, _ := r.Context().Value("userID").(uuid.UUID)
	wr, err := h.service.CreateWarning(r.Context(), sellerID, req, actorID)
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, "failed to create warning")
		return
	}
	if h.auditRepo != nil {
		actorEmail, _ := r.Context().Value("email").(string)
		actorRole, _ := r.Context().Value("role").(string)
		sid := sellerID
		go func() {
			_ = h.auditRepo.RecordAudit(context.Background(), staff.AuditEvent{
				ActorUserID: actorID, ActorEmail: actorEmail, ActorRole: actorRole,
				Action: "seller.warning_create", EntityType: "seller", EntityID: &sid,
				Metadata: staff.SanitizeMetadata(map[string]any{"type": req.Type, "severity": req.Severity}),
			})
		}()
	}
	h.respondJSON(w, http.StatusCreated, wr)
}

func (h *Handler) ResolveSellerWarning(w http.ResponseWriter, r *http.Request) {
	sellerIDStr := chi.URLParam(r, "id")
	sellerID, err := uuid.Parse(sellerIDStr)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid seller ID")
		return
	}
	warningIDStr := chi.URLParam(r, "warningId")
	warningID, err := uuid.Parse(warningIDStr)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid warning ID")
		return
	}
	var req ResolveWarningRequest
	_ = json.NewDecoder(r.Body).Decode(&req)
	actorID, _ := r.Context().Value("userID").(uuid.UUID)
	if err := h.service.ResolveWarning(r.Context(), sellerID, warningID, req, actorID); err != nil {
		if errors.Is(err, ErrWarningNotFound) {
			h.respondError(w, http.StatusNotFound, err.Error())
			return
		}
		h.respondError(w, http.StatusInternalServerError, "failed to resolve warning")
		return
	}
	if h.auditRepo != nil {
		actorEmail, _ := r.Context().Value("email").(string)
		actorRole, _ := r.Context().Value("role").(string)
		sid := sellerID
		go func() {
			_ = h.auditRepo.RecordAudit(context.Background(), staff.AuditEvent{
				ActorUserID: actorID, ActorEmail: actorEmail, ActorRole: actorRole,
				Action: "seller.warning_resolve", EntityType: "seller", EntityID: &sid,
				Metadata: staff.SanitizeMetadata(map[string]any{"warningId": warningID.String()}),
			})
		}()
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) CancelSellerWarning(w http.ResponseWriter, r *http.Request) {
	sellerIDStr := chi.URLParam(r, "id")
	sellerID, err := uuid.Parse(sellerIDStr)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid seller ID")
		return
	}
	warningIDStr := chi.URLParam(r, "warningId")
	warningID, err := uuid.Parse(warningIDStr)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid warning ID")
		return
	}
	actorID, _ := r.Context().Value("userID").(uuid.UUID)
	if err := h.service.CancelWarning(r.Context(), sellerID, warningID, actorID); err != nil {
		if errors.Is(err, ErrWarningNotFound) {
			h.respondError(w, http.StatusNotFound, err.Error())
			return
		}
		h.respondError(w, http.StatusInternalServerError, "failed to cancel warning")
		return
	}
	if h.auditRepo != nil {
		actorEmail, _ := r.Context().Value("email").(string)
		actorRole, _ := r.Context().Value("role").(string)
		sid := sellerID
		go func() {
			_ = h.auditRepo.RecordAudit(context.Background(), staff.AuditEvent{
				ActorUserID: actorID, ActorEmail: actorEmail, ActorRole: actorRole,
				Action: "seller.warning_cancel", EntityType: "seller", EntityID: &sid,
				Metadata: staff.SanitizeMetadata(map[string]any{"warningId": warningID.String()}),
			})
		}()
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ListSellerViolations(w http.ResponseWriter, r *http.Request) {
	sellerIDStr := chi.URLParam(r, "id")
	sellerID, err := uuid.Parse(sellerIDStr)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid seller ID")
		return
	}
	items, err := h.service.ListViolations(r.Context(), sellerID)
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, "failed to list violations")
		return
	}
	if items == nil {
		items = []ViolationResponse{}
	}
	h.respondJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) CreateSellerViolation(w http.ResponseWriter, r *http.Request) {
	sellerIDStr := chi.URLParam(r, "id")
	sellerID, err := uuid.Parse(sellerIDStr)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid seller ID")
		return
	}
	var req CreateViolationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Type == "" || req.Title == "" || req.Description == "" || req.Severity == "" {
		h.respondError(w, http.StatusBadRequest, "type, title, description and severity are required")
		return
	}
	actorID, _ := r.Context().Value("userID").(uuid.UUID)
	vr, err := h.service.CreateViolation(r.Context(), sellerID, req, actorID)
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, "failed to create violation")
		return
	}
	if h.auditRepo != nil {
		actorEmail, _ := r.Context().Value("email").(string)
		actorRole, _ := r.Context().Value("role").(string)
		sid := sellerID
		go func() {
			_ = h.auditRepo.RecordAudit(context.Background(), staff.AuditEvent{
				ActorUserID: actorID, ActorEmail: actorEmail, ActorRole: actorRole,
				Action: "seller.violation_create", EntityType: "seller", EntityID: &sid,
				Metadata: staff.SanitizeMetadata(map[string]any{"type": req.Type, "severity": req.Severity, "countsForPenalty": req.CountsForPenalty}),
			})
		}()
	}
	h.respondJSON(w, http.StatusCreated, vr)
}

func (h *Handler) ResolveSellerViolation(w http.ResponseWriter, r *http.Request) {
	sellerIDStr := chi.URLParam(r, "id")
	sellerID, err := uuid.Parse(sellerIDStr)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid seller ID")
		return
	}
	violationIDStr := chi.URLParam(r, "violationId")
	violationID, err := uuid.Parse(violationIDStr)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid violation ID")
		return
	}
	var req ResolveViolationRequest
	_ = json.NewDecoder(r.Body).Decode(&req)
	actorID, _ := r.Context().Value("userID").(uuid.UUID)
	if err := h.service.ResolveViolation(r.Context(), sellerID, violationID, req, actorID); err != nil {
		if errors.Is(err, ErrViolationNotFound) {
			h.respondError(w, http.StatusNotFound, err.Error())
			return
		}
		h.respondError(w, http.StatusInternalServerError, "failed to resolve violation")
		return
	}
	if h.auditRepo != nil {
		actorEmail, _ := r.Context().Value("email").(string)
		actorRole, _ := r.Context().Value("role").(string)
		sid := sellerID
		go func() {
			_ = h.auditRepo.RecordAudit(context.Background(), staff.AuditEvent{
				ActorUserID: actorID, ActorEmail: actorEmail, ActorRole: actorRole,
				Action: "seller.violation_resolve", EntityType: "seller", EntityID: &sid,
				Metadata: staff.SanitizeMetadata(map[string]any{"violationId": violationID.String()}),
			})
		}()
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) CancelSellerViolation(w http.ResponseWriter, r *http.Request) {
	sellerIDStr := chi.URLParam(r, "id")
	sellerID, err := uuid.Parse(sellerIDStr)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid seller ID")
		return
	}
	violationIDStr := chi.URLParam(r, "violationId")
	violationID, err := uuid.Parse(violationIDStr)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid violation ID")
		return
	}
	actorID, _ := r.Context().Value("userID").(uuid.UUID)
	if err := h.service.CancelViolation(r.Context(), sellerID, violationID, actorID); err != nil {
		if errors.Is(err, ErrViolationNotFound) {
			h.respondError(w, http.StatusNotFound, err.Error())
			return
		}
		h.respondError(w, http.StatusInternalServerError, "failed to cancel violation")
		return
	}
	if h.auditRepo != nil {
		actorEmail, _ := r.Context().Value("email").(string)
		actorRole, _ := r.Context().Value("role").(string)
		sid := sellerID
		go func() {
			_ = h.auditRepo.RecordAudit(context.Background(), staff.AuditEvent{
				ActorUserID: actorID, ActorEmail: actorEmail, ActorRole: actorRole,
				Action: "seller.violation_cancel", EntityType: "seller", EntityID: &sid,
				Metadata: staff.SanitizeMetadata(map[string]any{"violationId": violationID.String()}),
			})
		}()
	}
	w.WriteHeader(http.StatusNoContent)
}
