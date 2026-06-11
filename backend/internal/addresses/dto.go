package addresses

type CreateAddressRequest struct {
	Label         *string `json:"label,omitempty"`
	RecipientName string  `json:"recipientName" validate:"required"`
	Phone         string  `json:"phone" validate:"required"`
	City          string  `json:"city" validate:"required"`
	Street        string  `json:"street" validate:"required"`
	House         string  `json:"house" validate:"required"`
	Apartment     *string `json:"apartment,omitempty"`
	PostalCode    *string `json:"postalCode,omitempty"`
	Comment       *string `json:"comment,omitempty"`
	IsDefault     bool    `json:"isDefault"`
}

type UpdateAddressRequest struct {
	Label         *string `json:"label,omitempty"`
	RecipientName *string `json:"recipientName,omitempty"`
	Phone         *string `json:"phone,omitempty"`
	City          *string `json:"city,omitempty"`
	Street        *string `json:"street,omitempty"`
	House         *string `json:"house,omitempty"`
	Apartment     *string `json:"apartment,omitempty"`
	PostalCode    *string `json:"postalCode,omitempty"`
	Comment       *string `json:"comment,omitempty"`
	IsDefault     *bool   `json:"isDefault,omitempty"`
}
