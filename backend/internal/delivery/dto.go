package delivery

import "github.com/google/uuid"

type PublicDeliveryMethodDTO struct {
	ID               uuid.UUID `json:"id"`
	Code             string    `json:"code"`
	Name             string    `json:"name"`
	Description      *string   `json:"description,omitempty"`
	PriceCents       int64     `json:"priceCents"`
	EstimatedDaysMin *int      `json:"estimatedDaysMin,omitempty"`
	EstimatedDaysMax *int      `json:"estimatedDaysMax,omitempty"`
}

type PublicDeliveryMethodListResponse struct {
	Items []PublicDeliveryMethodDTO `json:"items"`
}
