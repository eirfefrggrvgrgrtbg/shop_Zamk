package products

import (
	"encoding/json"
	"net/http"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func (h *Handler) GetCategorySchema(w http.ResponseWriter, r *http.Request) {
	catIDStr := chi.URLParam(r, "id")
	catID, err := uuid.Parse(catIDStr)
	if err != nil {
		http.Error(w, "invalid category id", http.StatusBadRequest)
		return
	}
	schema, err := h.service.GetCategoryAttributeSchema(r.Context(), catID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(schema)
}

func (h *Handler) ListColors(w http.ResponseWriter, r *http.Request) {
	colors, err := h.service.ListColors(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(colors)
}

func (h *Handler) ListMaterials(w http.ResponseWriter, r *http.Request) {
	materials, err := h.service.ListMaterials(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(materials)
}

func (h *Handler) ListSizeSystems(w http.ResponseWriter, r *http.Request) {
	systems, err := h.service.ListSizeSystems(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(systems)
}

func (h *Handler) ListSizeValues(w http.ResponseWriter, r *http.Request) {
	sysIDStr := chi.URLParam(r, "id")
	sysID, err := uuid.Parse(sysIDStr)
	if err != nil {
		http.Error(w, "invalid size system id", http.StatusBadRequest)
		return
	}
	values, err := h.service.ListSizeValues(r.Context(), sysID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(values)
}

func (h *Handler) ListDictionaryValues(w http.ResponseWriter, r *http.Request) {
	dictIDStr := chi.URLParam(r, "id")
	dictID, err := uuid.Parse(dictIDStr)
	if err != nil {
		http.Error(w, "invalid dictionary id", http.StatusBadRequest)
		return
	}
	values, err := h.service.GetDictionaryValues(r.Context(), dictID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(values)
}
