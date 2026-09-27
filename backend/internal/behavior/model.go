package behavior

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type EventSource string

const (
	SourceClient EventSource = "client"
	SourceServer EventSource = "server"
)

type BehavioralEvent struct {
	ID          uuid.UUID       `db:"id"`
	EventType   string          `db:"event_type"`
	Source      EventSource     `db:"source"`
	VisitorID   *uuid.UUID      `db:"visitor_id"`
	UserID      *uuid.UUID      `db:"user_id"`
	ProductID   *uuid.UUID      `db:"product_id"`
	VariantID   *uuid.UUID      `db:"variant_id"`
	CategoryID  *uuid.UUID      `db:"category_id"`
	OrderID     *uuid.UUID      `db:"order_id"`
	ReturnID    *uuid.UUID      `db:"return_id"`
	OrderItemID *uuid.UUID      `db:"order_item_id"`
	Quantity    *int            `db:"quantity"`
	Placement   *string         `db:"placement"`
	Route       *string         `db:"route"`
	OccurredAt  time.Time       `db:"occurred_at"`
	ReceivedAt  time.Time       `db:"received_at"`
	Metadata    json.RawMessage `db:"metadata"`
}
