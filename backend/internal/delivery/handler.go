package delivery

import (
	"encoding/json"
	"net/http"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) GetPublicMethods(w http.ResponseWriter, r *http.Request) {
	methods, err := h.service.GetActiveMethods(r.Context())
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": "internal_error", "message": "Failed to get delivery methods"}})
		return
	}

	var dtos []PublicDeliveryMethodDTO
	for _, m := range methods {
		dtos = append(dtos, PublicDeliveryMethodDTO{
			ID:               m.ID,
			Code:             m.Code,
			Name:             m.Name,
			Description:      m.Description,
			PriceCents:       m.PriceCents,
			EstimatedDaysMin: m.EstimatedDaysMin,
			EstimatedDaysMax: m.EstimatedDaysMax,
		})
	}

	if dtos == nil {
		dtos = []PublicDeliveryMethodDTO{}
	}

	resp := PublicDeliveryMethodListResponse{
		Items: dtos,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
