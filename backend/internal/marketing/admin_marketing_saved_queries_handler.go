package marketing

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/auth"
)

func (h *Handler) getOwnerID(r *http.Request) (uuid.UUID, error) {
	userID := auth.GetUserID(r.Context())
	if userID == uuid.Nil {
		return uuid.Nil, errors.New("unauthorized")
	}
	return userID, nil
}

func (h *Handler) ListSavedQueries(w http.ResponseWriter, r *http.Request) {

	ownerID, err := h.getOwnerID(r)
	if err != nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", err.Error())
		return
	}

	queries, err := h.service.SavedQueriesRepo.ListByOwner(r.Context(), ownerID)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, queries)
}

func (h *Handler) GetSavedQuery(w http.ResponseWriter, r *http.Request) {
	ownerID, err := h.getOwnerID(r)
	if err != nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", err.Error())
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "invalid id")
		return
	}

	sq, err := h.service.SavedQueriesRepo.GetByID(r.Context(), id, ownerID)
	if err != nil {
		if errors.Is(err, ErrSavedQueryNotFound) {
			h.writeError(w, http.StatusNotFound, "not_found", "saved query not found")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, sq)
}

func (h *Handler) CreateSavedQuery(w http.ResponseWriter, r *http.Request) {
	ownerID, err := h.getOwnerID(r)
	if err != nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", err.Error())
		return
	}

	var req SavedQueryCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "invalid request body")
		return
	}

	// Validate name
	req.Name = req.Name
	if len(req.Name) < 1 || len(req.Name) > 120 {
		h.writeError(w, http.StatusBadRequest, "invalid_name", "name must be between 1 and 120 characters")
		return
	}

	// Validate description
	if req.Description != nil && len(*req.Description) > 500 {
		h.writeError(w, http.StatusBadRequest, "invalid_description", "description must be <= 500 characters")
		return
	}

	// Validate query spec
	var queryReq QueryRequest
	if err := json.Unmarshal(req.QuerySpec, &queryReq); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_query_spec", "malformed query specification")
		return
	}

	if valErr := ValidateQueryRequest(queryReq); valErr != nil {
		h.writeError(w, http.StatusBadRequest, valErr.Code, valErr.Message)
		return
	}

	canonicalSpecBytes, err := json.Marshal(queryReq)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "internal_error", "failed to process query spec")
		return
	}

	sq := &SavedQuery{
		Name:         strings.TrimSpace(req.Name),
		Description:  req.Description,
		QueryVersion: queryReq.Version,
		QuerySpec:    canonicalSpecBytes,
		CreatedBy:    ownerID,
	}

	if err := h.service.SavedQueriesRepo.Create(r.Context(), sq); err != nil {
		if errors.Is(err, ErrSavedQueryConflict) {
			h.writeError(w, http.StatusConflict, "conflict", "saved query with this name already exists")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	h.writeJSON(w, http.StatusCreated, sq)
}

func (h *Handler) UpdateSavedQuery(w http.ResponseWriter, r *http.Request) {
	ownerID, err := h.getOwnerID(r)
	if err != nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", err.Error())
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "invalid id")
		return
	}

	// We read body bytes to distinguish omitted vs null
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "invalid request body")
		return
	}

	var req SavedQueryUpdateRequest
	if err := json.Unmarshal(bodyBytes, &req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "invalid request body")
		return
	}

	var rawMap map[string]json.RawMessage
	_ = json.Unmarshal(bodyBytes, &rawMap)

	// Fetch existing query to patch
	existingSq, err := h.service.SavedQueriesRepo.GetByID(r.Context(), id, ownerID)
	if err != nil {
		if errors.Is(err, ErrSavedQueryNotFound) {
			h.writeError(w, http.StatusNotFound, "not_found", "saved query not found")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	// Name patch
	newName := existingSq.Name
	if _, ok := rawMap["name"]; ok {
		if req.Name == nil {
			h.writeError(w, http.StatusBadRequest, "invalid_name", "name must not be null")
			return
		}
		newName = strings.TrimSpace(*req.Name)
		if len(newName) < 1 || len(newName) > 120 {
			h.writeError(w, http.StatusBadRequest, "invalid_name", "name must be between 1 and 120 characters")
			return
		}
	}

	// Description patch
	newDescription := existingSq.Description
	if _, ok := rawMap["description"]; ok {
		newDescription = req.Description
		if newDescription != nil && len(*newDescription) > 500 {
			h.writeError(w, http.StatusBadRequest, "invalid_description", "description must be <= 500 characters")
			return
		}
	}

	// QuerySpec patch
	newQuerySpec := existingSq.QuerySpec
	newQueryVersion := existingSq.QueryVersion
	if _, ok := rawMap["querySpec"]; ok {
		var queryReq QueryRequest
		if err := json.Unmarshal(req.QuerySpec, &queryReq); err != nil {
			h.writeError(w, http.StatusBadRequest, "invalid_query_spec", "malformed query specification")
			return
		}

		if valErr := ValidateQueryRequest(queryReq); valErr != nil {
			h.writeError(w, http.StatusBadRequest, valErr.Code, valErr.Message)
			return
		}

		canonicalSpecBytes, err := json.Marshal(queryReq)
		if err != nil {
			h.writeError(w, http.StatusInternalServerError, "internal_error", "failed to process query spec")
			return
		}
		newQuerySpec = canonicalSpecBytes
		newQueryVersion = queryReq.Version
	}

	sq := &SavedQuery{
		ID:           id,
		Name:         newName,
		Description:  newDescription,
		QueryVersion: newQueryVersion,
		QuerySpec:    newQuerySpec,
		CreatedBy:    ownerID,
	}

	if err := h.service.SavedQueriesRepo.Update(r.Context(), sq); err != nil {
		if errors.Is(err, ErrSavedQueryNotFound) {
			h.writeError(w, http.StatusNotFound, "not_found", "saved query not found")
			return
		}
		if errors.Is(err, ErrSavedQueryConflict) {
			h.writeError(w, http.StatusConflict, "conflict", "saved query with this name already exists")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	// Fetch the fully updated query to return
	updatedSq, err := h.service.SavedQueriesRepo.GetByID(r.Context(), id, ownerID)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, updatedSq)
}

func (h *Handler) DeleteSavedQuery(w http.ResponseWriter, r *http.Request) {
	ownerID, err := h.getOwnerID(r)
	if err != nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", err.Error())
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "invalid id")
		return
	}

	if err := h.service.SavedQueriesRepo.Delete(r.Context(), id, ownerID); err != nil {
		if errors.Is(err, ErrSavedQueryNotFound) {
			h.writeError(w, http.StatusNotFound, "not_found", "saved query not found")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
