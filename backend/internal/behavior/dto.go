package behavior

import (
	"time"

	"github.com/google/uuid"
)

type EventIngestionRequest struct {
	Events []IngestionEvent `json:"events"`
}

// IngestionEvent uses strict JSON decoding to drop unknown fields via unmarshaler, or we can just ignore.
// Note: userId, orderId, returnId, orderItemId, source, receivedAt, categoryId are intentionally missing from this struct.
// This guarantees clients cannot inject them via standard json.Unmarshal.
type IngestionEvent struct {
	EventID    uuid.UUID              `json:"eventId"`
	EventType  string                 `json:"eventType"`
	VisitorID  uuid.UUID              `json:"visitorId"`
	SessionID  *uuid.UUID             `json:"sessionId,omitempty"`
	OccurredAt time.Time              `json:"occurredAt"`
	ProductID  *uuid.UUID             `json:"productId,omitempty"`
	VariantID  *uuid.UUID             `json:"variantId,omitempty"`
	Quantity   *int                   `json:"quantity,omitempty"`
	Placement  *string                `json:"placement,omitempty"`
	Route      *string                `json:"route,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

type EventIngestionResponse struct {
	Accepted   int             `json:"accepted"`
	Duplicates int             `json:"duplicates"`
	Rejected   []RejectedEvent `json:"rejected"`
}

type RejectedEvent struct {
	EventID uuid.UUID `json:"eventId"`
	Code    string    `json:"code"`
}
