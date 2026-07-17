package delivery

import (
	"time"
	"github.com/google/uuid"
)

type DeliveryMethod struct {
	ID                 uuid.UUID `json:"id" db:"id"`
	Code               string    `json:"code" db:"code"`
	Name               string    `json:"name" db:"name"`
	Description        *string   `json:"description,omitempty" db:"description"`
	PriceCents         int64     `json:"priceCents" db:"price_cents"`
	EstimatedDaysMin   *int      `json:"estimatedDaysMin,omitempty" db:"estimated_days_min"`
	EstimatedDaysMax   *int      `json:"estimatedDaysMax,omitempty" db:"estimated_days_max"`
	IsActive           bool      `json:"isActive" db:"is_active"`
	SortOrder          int       `json:"sortOrder" db:"sort_order"`
	CreatedAt          time.Time `json:"createdAt" db:"created_at"`
	UpdatedAt          time.Time `json:"updatedAt" db:"updated_at"`
}
