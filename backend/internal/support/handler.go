package support

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

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

// -------------------------------------------------------------
// Customer Handlers
// -------------------------------------------------------------

func (h *Handler) GetCustomerConversation(w http.ResponseWriter, r *http.Request) {
	customerID := auth.GetUserID(r.Context())
	if customerID == uuid.Nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Customer authorization required")
		return
	}

	detail, err := h.service.GetCustomerConversation(r.Context(), customerID)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	h.writeJSON(w, http.StatusOK, detail)
}

func (h *Handler) SendCustomerMessage(w http.ResponseWriter, r *http.Request) {
	customerID := auth.GetUserID(r.Context())
	if customerID == uuid.Nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Customer authorization required")
		return
	}

	var req SendMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "bad_request", "Invalid request body")
		return
	}

	msg, err := h.service.SendCustomerMessage(r.Context(), customerID, req)
	if err != nil {
		switch {
		case errors.Is(err, ErrEmptyMessage):
			h.writeError(w, http.StatusBadRequest, "empty_message", err.Error())
		case errors.Is(err, ErrInvalidContext):
			h.writeError(w, http.StatusForbidden, "invalid_context", err.Error())
		case errors.Is(err, ErrAttachmentInvalid), errors.Is(err, ErrAttachmentCountExceeded):
			h.writeError(w, http.StatusBadRequest, "invalid_attachments", err.Error())
		default:
			h.writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		}
		return
	}

	h.writeJSON(w, http.StatusCreated, msg)
}

func (h *Handler) UploadCustomerAttachment(w http.ResponseWriter, r *http.Request) {
	customerID := auth.GetUserID(r.Context())
	if customerID == uuid.Nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Customer authorization required")
		return
	}

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_file", "File too large or invalid")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "missing_file", "File is required")
		return
	}
	defer file.Close()

	att, err := h.service.UploadCustomerAttachment(r.Context(), customerID, file, header.Filename, header.Header.Get("Content-Type"), header.Size)
	if err != nil {
		switch {
		case errors.Is(err, ErrAttachmentTooLarge):
			h.writeError(w, http.StatusBadRequest, "file_too_large", err.Error())
		case errors.Is(err, ErrAttachmentInvalid):
			h.writeError(w, http.StatusBadRequest, "invalid_file_type", err.Error())
		default:
			h.writeError(w, http.StatusInternalServerError, "upload_error", err.Error())
		}
		return
	}

	h.writeJSON(w, http.StatusCreated, att)
}

func (h *Handler) DownloadCustomerAttachment(w http.ResponseWriter, r *http.Request) {
	customerID := auth.GetUserID(r.Context())
	if customerID == uuid.Nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Customer authorization required")
		return
	}

	attID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid attachment ID")
		return
	}

	data, contentType, err := h.service.DownloadAttachment(r.Context(), attID, customerID, false, nil)
	if err != nil {
		switch {
		case errors.Is(err, ErrAttachmentNotFound):
			h.writeError(w, http.StatusNotFound, "not_found", "Attachment not found")
		case errors.Is(err, ErrForbidden):
			h.writeError(w, http.StatusForbidden, "forbidden", "Forbidden attachment access")
		default:
			h.writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		}
		return
	}

	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (h *Handler) MarkCustomerRead(w http.ResponseWriter, r *http.Request) {
	customerID := auth.GetUserID(r.Context())
	if customerID == uuid.Nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Customer authorization required")
		return
	}

	detail, err := h.service.GetCustomerConversation(r.Context(), customerID)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	if err := h.service.MarkRequesterRead(r.Context(), customerID, detail.Conversation.ID); err != nil {
		h.writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	h.writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// -------------------------------------------------------------
// Seller Handlers
// -------------------------------------------------------------

func (h *Handler) GetSellerConversation(w http.ResponseWriter, r *http.Request) {
	memberUserID := auth.GetUserID(r.Context())
	sellerID, err := auth.GetSellerID(r.Context())
	if err != nil || sellerID == uuid.Nil || memberUserID == uuid.Nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Seller authorization required")
		return
	}

	detail, err := h.service.GetSellerConversation(r.Context(), sellerID, memberUserID)
	if err != nil {
		if errors.Is(err, ErrForbidden) {
			h.writeError(w, http.StatusForbidden, "forbidden", err.Error())
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	h.writeJSON(w, http.StatusOK, detail)
}

func (h *Handler) SendSellerMessage(w http.ResponseWriter, r *http.Request) {
	memberUserID := auth.GetUserID(r.Context())
	sellerID, err := auth.GetSellerID(r.Context())
	if err != nil || sellerID == uuid.Nil || memberUserID == uuid.Nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Seller authorization required")
		return
	}

	var req SendMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "bad_request", "Invalid request body")
		return
	}

	msg, err := h.service.SendSellerMessage(r.Context(), sellerID, memberUserID, req)
	if err != nil {
		switch {
		case errors.Is(err, ErrForbidden):
			h.writeError(w, http.StatusForbidden, "forbidden", err.Error())
		case errors.Is(err, ErrEmptyMessage):
			h.writeError(w, http.StatusBadRequest, "empty_message", err.Error())
		case errors.Is(err, ErrInvalidContext):
			h.writeError(w, http.StatusForbidden, "invalid_context", err.Error())
		case errors.Is(err, ErrAttachmentInvalid), errors.Is(err, ErrAttachmentCountExceeded):
			h.writeError(w, http.StatusBadRequest, "invalid_attachments", err.Error())
		default:
			h.writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		}
		return
	}

	h.writeJSON(w, http.StatusCreated, msg)
}

func (h *Handler) UploadSellerAttachment(w http.ResponseWriter, r *http.Request) {
	memberUserID := auth.GetUserID(r.Context())
	sellerID, err := auth.GetSellerID(r.Context())
	if err != nil || sellerID == uuid.Nil || memberUserID == uuid.Nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Seller authorization required")
		return
	}

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_file", "File too large or invalid")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "missing_file", "File is required")
		return
	}
	defer file.Close()

	att, err := h.service.UploadSellerAttachment(r.Context(), sellerID, memberUserID, file, header.Filename, header.Header.Get("Content-Type"), header.Size)
	if err != nil {
		switch {
		case errors.Is(err, ErrForbidden):
			h.writeError(w, http.StatusForbidden, "forbidden", err.Error())
		case errors.Is(err, ErrAttachmentTooLarge):
			h.writeError(w, http.StatusBadRequest, "file_too_large", err.Error())
		case errors.Is(err, ErrAttachmentInvalid):
			h.writeError(w, http.StatusBadRequest, "invalid_file_type", err.Error())
		default:
			h.writeError(w, http.StatusInternalServerError, "upload_error", err.Error())
		}
		return
	}

	h.writeJSON(w, http.StatusCreated, att)
}

func (h *Handler) DownloadSellerAttachment(w http.ResponseWriter, r *http.Request) {
	memberUserID := auth.GetUserID(r.Context())
	sellerID, err := auth.GetSellerID(r.Context())
	if err != nil || sellerID == uuid.Nil || memberUserID == uuid.Nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Seller authorization required")
		return
	}

	attID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid attachment ID")
		return
	}

	data, contentType, err := h.service.DownloadAttachment(r.Context(), attID, memberUserID, false, &sellerID)
	if err != nil {
		switch {
		case errors.Is(err, ErrAttachmentNotFound):
			h.writeError(w, http.StatusNotFound, "not_found", "Attachment not found")
		case errors.Is(err, ErrForbidden):
			h.writeError(w, http.StatusForbidden, "forbidden", "Forbidden attachment access")
		default:
			h.writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		}
		return
	}

	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (h *Handler) MarkSellerRead(w http.ResponseWriter, r *http.Request) {
	memberUserID := auth.GetUserID(r.Context())
	sellerID, err := auth.GetSellerID(r.Context())
	if err != nil || sellerID == uuid.Nil || memberUserID == uuid.Nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Seller authorization required")
		return
	}

	detail, err := h.service.GetSellerConversation(r.Context(), sellerID, memberUserID)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	if err := h.service.MarkRequesterRead(r.Context(), memberUserID, detail.Conversation.ID); err != nil {
		h.writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	h.writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// -------------------------------------------------------------
// Admin / Staff Handlers
// -------------------------------------------------------------

func (h *Handler) ListCategories(w http.ResponseWriter, r *http.Request) {
	scope := RequesterScope(r.URL.Query().Get("scope"))
	cats, err := h.service.ListCategories(r.Context(), scope)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	h.writeJSON(w, http.StatusOK, cats)
}

func (h *Handler) ListConversations(w http.ResponseWriter, r *http.Request) {
	staffUserID := auth.GetUserID(r.Context())
	filter := RequesterType(r.URL.Query().Get("filter"))

	convs, err := h.service.ListConversations(r.Context(), filter, staffUserID)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	h.writeJSON(w, http.StatusOK, convs)
}

func (h *Handler) GetAdminConversation(w http.ResponseWriter, r *http.Request) {
	staffUserID := auth.GetUserID(r.Context())
	convID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid conversation ID")
		return
	}

	detail, err := h.service.GetAdminConversation(r.Context(), convID, staffUserID)
	if err != nil {
		if errors.Is(err, ErrConversationNotFound) {
			h.writeError(w, http.StatusNotFound, "not_found", "Conversation not found")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	h.writeJSON(w, http.StatusOK, detail)
}

func (h *Handler) SendStaffReply(w http.ResponseWriter, r *http.Request) {
	staffUserID := auth.GetUserID(r.Context())
	convID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid conversation ID")
		return
	}

	var req StaffReplyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "bad_request", "Invalid request body")
		return
	}

	msg, err := h.service.SendStaffReply(r.Context(), convID, staffUserID, req)
	if err != nil {
		switch {
		case errors.Is(err, ErrEmptyMessage):
			h.writeError(w, http.StatusBadRequest, "empty_message", err.Error())
		case errors.Is(err, ErrConversationNotFound):
			h.writeError(w, http.StatusNotFound, "not_found", "Conversation not found")
		case errors.Is(err, ErrAttachmentInvalid), errors.Is(err, ErrAttachmentCountExceeded):
			h.writeError(w, http.StatusBadRequest, "invalid_attachments", err.Error())
		default:
			h.writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		}
		return
	}

	h.writeJSON(w, http.StatusCreated, msg)
}

func (h *Handler) CreateInternalNote(w http.ResponseWriter, r *http.Request) {
	staffUserID := auth.GetUserID(r.Context())
	convID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid conversation ID")
		return
	}

	var req CreateInternalNoteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "bad_request", "Invalid request body")
		return
	}

	note, err := h.service.CreateInternalNote(r.Context(), convID, staffUserID, req)
	if err != nil {
		switch {
		case errors.Is(err, ErrForbidden):
			h.writeError(w, http.StatusForbidden, "forbidden", err.Error())
		case errors.Is(err, ErrEmptyMessage):
			h.writeError(w, http.StatusBadRequest, "empty_note", err.Error())
		default:
			h.writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		}
		return
	}

	h.writeJSON(w, http.StatusCreated, note)
}

func (h *Handler) MarkStaffRead(w http.ResponseWriter, r *http.Request) {
	staffUserID := auth.GetUserID(r.Context())
	convID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid conversation ID")
		return
	}

	if err := h.service.MarkStaffRead(r.Context(), staffUserID, convID); err != nil {
		h.writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	h.writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) CompleteSession(w http.ResponseWriter, r *http.Request) {
	convID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid conversation ID")
		return
	}

	if err := h.service.CompleteSession(r.Context(), convID); err != nil {
		if errors.Is(err, ErrNoActiveSession) || errors.Is(err, ErrSessionNotFound) {
			h.writeError(w, http.StatusConflict, "no_active_session", "No active session to complete")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	h.writeJSON(w, http.StatusOK, map[string]string{"status": "completed"})
}

func (h *Handler) ReopenSession(w http.ResponseWriter, r *http.Request) {
	sessionID, err := uuid.Parse(chi.URLParam(r, "sessionId"))
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid session ID")
		return
	}

	if err := h.service.ReopenSession(r.Context(), sessionID); err != nil {
		if errors.Is(err, ErrActiveSessionAlreadyExists) {
			h.writeError(w, http.StatusConflict, "active_session_exists", "Cannot reopen: active session already exists")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	h.writeJSON(w, http.StatusOK, map[string]string{"status": "reopened"})
}

func (h *Handler) UpdateSession(w http.ResponseWriter, r *http.Request) {
	convID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid conversation ID")
		return
	}

	var req UpdateSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "bad_request", "Invalid request body")
		return
	}

	if err := h.service.UpdateSession(r.Context(), convID, req); err != nil {
		switch {
		case errors.Is(err, ErrInvalidPriority):
			h.writeError(w, http.StatusBadRequest, "invalid_priority", err.Error())
		case errors.Is(err, ErrCategoryNotFound):
			h.writeError(w, http.StatusBadRequest, "category_not_found", err.Error())
		case errors.Is(err, ErrInvalidCategoryScope):
			h.writeError(w, http.StatusBadRequest, "invalid_category_scope", err.Error())
		case errors.Is(err, ErrInvalidAssignee):
			h.writeError(w, http.StatusBadRequest, "invalid_assignee", err.Error())
		default:
			h.writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		}
		return
	}
	h.writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (h *Handler) DownloadAdminAttachment(w http.ResponseWriter, r *http.Request) {
	staffUserID := auth.GetUserID(r.Context())
	attID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid attachment ID")
		return
	}

	data, contentType, err := h.service.DownloadAttachment(r.Context(), attID, staffUserID, true, nil)
	if err != nil {
		if errors.Is(err, ErrAttachmentNotFound) {
			h.writeError(w, http.StatusNotFound, "not_found", "Attachment not found")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}
