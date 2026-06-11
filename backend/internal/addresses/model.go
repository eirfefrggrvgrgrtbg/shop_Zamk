package addresses

import (
	"time"

	"github.com/google/uuid"
)

type Address struct {
	ID            uuid.UUID `json:"id"`
	UserID        uuid.UUID `json:"userId"`
	Label         *string   `json:"label,omitempty"`
	RecipientName string    `json:"recipientName"`
	Phone         string    `json:"phone"`
	City          string    `json:"city"`
	Street        string    `json:"street"`
	House         string    `json:"house"`
	Apartment     *string   `json:"apartment,omitempty"`
	PostalCode    *string   `json:"postalCode,omitempty"`
	Comment       *string   `json:"comment,omitempty"`
	IsDefault     bool      `json:"isDefault"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}
