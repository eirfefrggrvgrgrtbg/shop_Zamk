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
	SessionID   *uuid.UUID      `db:"session_id"`
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

var serverEventNamespace = uuid.MustParse("017e2e36-f13d-4c3e-b83b-93f5451a44c5")

// DeterministicServerEventID generates a deterministic event ID based on business facts.
func DeterministicServerEventID(eventType string, businessID1, businessID2 uuid.UUID) uuid.UUID {
	name := eventType + ":" + businessID1.String() + ":" + businessID2.String()
	return uuid.NewSHA1(serverEventNamespace, []byte(name))
}

const (
	// SessionInactivityTimeout is the locked 30-minute inactivity threshold for analytics sessions.
	SessionInactivityTimeout = 30 * time.Minute

	// AttributionLifetime is the locked 30-day window for Last-Non-Direct-Touch attribution.
	AttributionLifetime = 30 * 24 * time.Hour
)

// CalculateAttributionExpiry computes the canonical expiry timestamp for non-direct attribution.
func CalculateAttributionExpiry(capturedAt time.Time) time.Time {
	return capturedAt.Add(AttributionLifetime)
}

// IsSessionExpired checks whether an analytics session has timed out due to inactivity.
func IsSessionExpired(lastSeenAt, now time.Time) bool {
	return now.Sub(lastSeenAt) > SessionInactivityTimeout
}

// IsAttributionExpired checks whether an attribution timestamp has passed its validity window.
func IsAttributionExpired(expiresAt *time.Time, now time.Time) bool {
	if expiresAt == nil {
		return true
	}
	return now.After(*expiresAt)
}

type AnalyticsSession struct {
	ID                    uuid.UUID  `db:"id"`
	VisitorID             uuid.UUID  `db:"visitor_id"`
	UserID                *uuid.UUID `db:"user_id"`
	StartedAt             time.Time  `db:"started_at"`
	LastSeenAt            time.Time  `db:"last_seen_at"`
	LandingPath           *string    `db:"landing_path"`
	Referrer              *string    `db:"referrer"`
	Source                *string    `db:"source"`
	Medium                *string    `db:"medium"`
	UTMSource             *string    `db:"utm_source"`
	UTMMedium             *string    `db:"utm_medium"`
	UTMCampaign           *string    `db:"utm_campaign"`
	UTMTerm               *string    `db:"utm_term"`
	UTMContent            *string    `db:"utm_content"`
	CampaignID            *uuid.UUID `db:"campaign_id"`
	AttributionCapturedAt *time.Time `db:"attribution_captured_at"`
	AttributionExpiresAt  *time.Time `db:"attribution_expires_at"`
}

type OrderAttribution struct {
	OrderID       uuid.UUID  `db:"order_id"`
	SessionID     *uuid.UUID `db:"session_id"`
	VisitorID     *uuid.UUID `db:"visitor_id"`
	Source        *string    `db:"source"`
	Medium        *string    `db:"medium"`
	UTMSource     *string    `db:"utm_source"`
	UTMMedium     *string    `db:"utm_medium"`
	UTMCampaign   *string    `db:"utm_campaign"`
	UTMTerm       *string    `db:"utm_term"`
	UTMContent    *string    `db:"utm_content"`
	CampaignID    *uuid.UUID `db:"campaign_id"`
	PromoCodeID   *uuid.UUID `db:"promo_code_id"`
	AttributedAt  time.Time  `db:"attributed_at"`
}
