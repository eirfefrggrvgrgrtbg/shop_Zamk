package returns

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/auth"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func (h *Handler) GetAdminReturnMessages(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	returnID, err := uuid.Parse(idStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "invalid return ID")
		return
	}

	messages, err := h.service.GetReturnMessages(r.Context(), returnID)
	if err != nil {
		if errors.Is(err, ErrReturnNotFound) {
			h.writeError(w, http.StatusNotFound, "return_not_found", "Return not found")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ReturnConversationResponse{Messages: messages})
}

func (h *Handler) GetCustomerReturnMessages(w http.ResponseWriter, r *http.Request) {
	customerID := auth.GetUserID(r.Context())
	idStr := chi.URLParam(r, "id")
	returnID, err := uuid.Parse(idStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "invalid return ID")
		return
	}

	// First verify it belongs to customer
	ret, err := h.service.GetCustomerReturn(r.Context(), customerID, returnID)
	if err != nil {
		h.writeError(w, http.StatusNotFound, "return_not_found", "Return not found")
		return
	}

	messages, err := h.service.GetReturnMessages(r.Context(), ret.ID)
	if err != nil {
		if errors.Is(err, ErrReturnNotFound) {
			h.writeError(w, http.StatusNotFound, "return_not_found", "Return not found")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ReturnConversationResponse{Messages: messages})
}

func (h *Handler) SendAdminReturnMessage(w http.ResponseWriter, r *http.Request) {
	adminID := auth.GetUserID(r.Context())
	idStr := chi.URLParam(r, "id")
	returnID, err := uuid.Parse(idStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "invalid return ID")
		return
	}

	var req AdminSendReturnMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "invalid JSON body")
		return
	}

	if err := h.service.SendAdminReturnMessage(r.Context(), returnID, adminID, req.Message, req.NeedsResponse, req.AttachmentIDs); err != nil {
		switch {
		case errors.Is(err, ErrReturnMessageRequired):
			h.writeError(w, http.StatusBadRequest, "message_required", "message is required")
		case errors.Is(err, ErrReturnTerminalState):
			h.writeError(w, http.StatusBadRequest, "return_terminal_state", "return is in a terminal state")
		case errors.Is(err, ErrReturnNotRequestableInfo):
			h.writeError(w, http.StatusBadRequest, "return_not_requestable_info", "return is not in requested status")
		case errors.Is(err, ErrReturnMessageTooManyAttachments):
			h.writeError(w, http.StatusBadRequest, "return_message_too_many_attachments", "maximum 6 attachments allowed")
		case errors.Is(err, ErrReturnMessageAttachmentNotOwned):
			h.writeError(w, http.StatusBadRequest, "return_message_attachment_not_owned", "attachment not found or not owned by user")
		case errors.Is(err, ErrReturnMessageAttachmentInvalid):
			h.writeError(w, http.StatusBadRequest, "invalid_file_type", "invalid attachment")
		case errors.Is(err, ErrReturnNotFound):
			h.writeError(w, http.StatusNotFound, "return_not_found", "Return not found")
		default:
			h.writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) SendCustomerReturnMessage(w http.ResponseWriter, r *http.Request) {
	customerID := auth.GetUserID(r.Context())
	idStr := chi.URLParam(r, "id")
	returnID, err := uuid.Parse(idStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "invalid return ID")
		return
	}

	var req CustomerSendReturnMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "invalid JSON body")
		return
	}

	if err := h.service.SendCustomerReturnMessage(r.Context(), returnID, customerID, req.Message, req.AttachmentIDs); err != nil {
		switch {
		case errors.Is(err, ErrReturnMessageRequired):
			h.writeError(w, http.StatusBadRequest, "message_required", "message is required")
		case errors.Is(err, ErrReturnTerminalState):
			h.writeError(w, http.StatusBadRequest, "return_terminal_state", "return is in a terminal state")
		case errors.Is(err, ErrReturnMessageTooManyAttachments):
			h.writeError(w, http.StatusBadRequest, "return_message_too_many_attachments", "maximum 6 attachments allowed")
		case errors.Is(err, ErrReturnMessageAttachmentNotOwned):
			h.writeError(w, http.StatusBadRequest, "return_message_attachment_not_owned", "attachment not found or not owned by user")
		case errors.Is(err, ErrReturnMessageAttachmentInvalid):
			h.writeError(w, http.StatusBadRequest, "invalid_file_type", "invalid attachment")
		case errors.Is(err, ErrReturnNotFound):
			h.writeError(w, http.StatusNotFound, "return_not_found", "Return not found")
		default:
			h.writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) UploadAdminReturnMessageAttachment(w http.ResponseWriter, r *http.Request) {
	adminID := auth.GetUserID(r.Context())
	idStr := chi.URLParam(r, "id")
	returnID, err := uuid.Parse(idStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "invalid return ID")
		return
	}

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		h.writeError(w, http.StatusBadRequest, "file_too_large", "file too large")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_file", "invalid file")
		return
	}
	defer file.Close()

	if header.Size > 10<<20 {
		h.writeError(w, http.StatusBadRequest, "file_too_large", "file exceeds maximum allowed size of 10MB")
		return
	}

	contentType := header.Header.Get("Content-Type")

	res, err := h.service.UploadAdminReturnMessageAttachment(r.Context(), returnID, adminID, file, header.Filename, contentType, header.Size)
	if err != nil {
		switch {
		case errors.Is(err, ErrReturnNotFound):
			h.writeError(w, http.StatusNotFound, "return_not_found", "Return not found")
		case errors.Is(err, ErrReturnTerminalState):
			h.writeError(w, http.StatusBadRequest, "return_terminal_state", "return is in a terminal state")
		case errors.Is(err, ErrReturnMessageAttachmentTooLarge):
			h.writeError(w, http.StatusBadRequest, "file_too_large", "file exceeds maximum size limit of 10MB")
		case errors.Is(err, ErrReturnMessageAttachmentInvalid):
			h.writeError(w, http.StatusBadRequest, "invalid_file_type", "invalid file type or format")
		default:
			h.writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(res)
}

func (h *Handler) UploadCustomerReturnMessageAttachment(w http.ResponseWriter, r *http.Request) {
	customerID := auth.GetUserID(r.Context())
	idStr := chi.URLParam(r, "id")
	returnID, err := uuid.Parse(idStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "invalid return ID")
		return
	}

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		h.writeError(w, http.StatusBadRequest, "file_too_large", "file too large")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_file", "invalid file")
		return
	}
	defer file.Close()

	if header.Size > 10<<20 {
		h.writeError(w, http.StatusBadRequest, "file_too_large", "file exceeds maximum allowed size of 10MB")
		return
	}

	contentType := header.Header.Get("Content-Type")

	res, err := h.service.UploadCustomerReturnMessageAttachment(r.Context(), returnID, customerID, file, header.Filename, contentType, header.Size)
	if err != nil {
		switch {
		case errors.Is(err, ErrReturnNotFound):
			h.writeError(w, http.StatusNotFound, "return_not_found", "Return not found")
		case errors.Is(err, ErrReturnTerminalState):
			h.writeError(w, http.StatusBadRequest, "return_terminal_state", "return is in a terminal state")
		case errors.Is(err, ErrReturnMessageAttachmentTooLarge):
			h.writeError(w, http.StatusBadRequest, "file_too_large", "file exceeds maximum size limit of 10MB")
		case errors.Is(err, ErrReturnMessageAttachmentInvalid):
			h.writeError(w, http.StatusBadRequest, "invalid_file_type", "invalid file type or format")
		default:
			h.writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(res)
}
