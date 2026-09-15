package personalization

import (
	"time"

	"github.com/google/uuid"
)

// CustomerProductView represents an aggregated view record of a product by a customer.
// product_view is a client-observed soft signal that does not alter product, stock, or order state.
type CustomerProductView struct {
	UserID       uuid.UUID `json:"userId"`
	ProductID    uuid.UUID `json:"productId"`
	LastViewedAt time.Time `json:"lastViewedAt"`
	ViewCount    int64     `json:"viewCount"`
}
