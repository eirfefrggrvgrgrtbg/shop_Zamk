package behavior

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
)

type StructuralError struct {
	Code    string
	Message string
}

func (e *StructuralError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

type TokenResolver func(ctx context.Context, token string) (*uuid.UUID, error)

type Service struct {
	repo *Repository
	campaignResolver TokenResolver
	resolver TokenResolver
}

type EventWriter interface {
	InsertServerEventsTx(ctx context.Context, tx pgx.Tx, events []BehavioralEvent) error
	ResolveCategoriesTx(ctx context.Context, tx pgx.Tx, productIDs []uuid.UUID) (map[uuid.UUID]ProductBehaviorSnapshot, error)
}

func NewService(repo *Repository, resolvers ...TokenResolver) *Service {
	var resolver TokenResolver
	if len(resolvers) > 0 {
		resolver = resolvers[0]
	}
	return &Service{repo: repo, campaignResolver: resolver}
}


var clientSafeEvents = map[string]bool{
	"session_started":          true,
	"page_view":                true,
	"catalog_impression":       true,
	"product_view":             true,
	"product_variant_selected": true,
	"favorite_added":           true,
	"favorite_removed":         true,
	"add_to_cart":              true,
	"remove_from_cart":         true,
	"checkout_started":         true,
}

var serverOnlyEvents = map[string]bool{
	"order_paid":       true,
	"order_delivered":  true,
	"return_requested": true,
}

var allowedMetadataKeys = map[string]int{
	"referrer":     1024,
	"landing_path": 1024,
	"source":       255,
	"medium":       255,
	"utm_source":   255,
	"utm_medium":   255,
	"utm_campaign": 255,
	"utm_term":     255,
	"utm_content":  255,
	"zamk_token":   255,
}

const (
	maxBatchSize       = 50
	maxPlacementLength = 100
	maxRouteLength     = 255
	maxQuantity        = 100
)

// ValidateStructural performs all zero-tolerance structural checks on the request and its events.
// Any violation rejects the whole request with an HTTP 400.
func (s *Service) ValidateStructural(req *EventIngestionRequest) error {
	if len(req.Events) == 0 {
		return &StructuralError{Code: "empty_batch", Message: "Batch must contain at least one event"}
	}
	if len(req.Events) > maxBatchSize {
		return &StructuralError{Code: "batch_too_large", Message: "Maximum 50 events per batch"}
	}

	for _, e := range req.Events {
		if e.EventID == uuid.Nil {
			return &StructuralError{Code: "invalid_event_id", Message: "eventId must be a valid non-nil UUID"}
		}
		if e.VisitorID == uuid.Nil {
			return &StructuralError{Code: "invalid_visitor_id", Message: "visitorId must be a valid non-nil UUID"}
		}
		if e.OccurredAt.IsZero() {
			return &StructuralError{Code: "invalid_occurred_at", Message: "occurredAt must be provided"}
		}

		// Event type validation
		if serverOnlyEvents[e.EventType] {
			return &StructuralError{Code: "forbidden_event_type", Message: "Server-only event type cannot be submitted via public ingestion"}
		}
		if !clientSafeEvents[e.EventType] {
			return &StructuralError{Code: "unknown_event_type", Message: "Unknown event type: " + e.EventType}
		}

		// Metadata validation: allowed only for session/page view attribution properties with whitelisted keys
		if len(e.Metadata) > 0 {
			if e.EventType != "session_started" && e.EventType != "page_view" {
				return &StructuralError{Code: "invalid_metadata", Message: "Custom metadata is only allowed for session/page attribution"}
			}
			for k, v := range e.Metadata {
				maxLen, ok := allowedMetadataKeys[k]
				if !ok {
					return &StructuralError{Code: "invalid_metadata", Message: fmt.Sprintf("unsupported or forbidden metadata key: %s", k)}
				}
				if v != nil {
					strVal, isStr := v.(string)
					if !isStr {
						return &StructuralError{Code: "invalid_metadata", Message: fmt.Sprintf("metadata value for key %s must be a string", k)}
					}
					if len(strVal) > maxLen {
						return &StructuralError{Code: "invalid_metadata", Message: fmt.Sprintf("metadata key %s exceeds maximum length %d", k, maxLen)}
					}
				}
			}
		}

		// Quantity validation
		if e.Quantity != nil {
			if *e.Quantity < 1 || *e.Quantity > maxQuantity {
				return &StructuralError{Code: "invalid_quantity", Message: "quantity must be between 1 and 100"}
			}
		}

		// Placement validation
		if e.Placement != nil {
			if len(*e.Placement) > maxPlacementLength {
				return &StructuralError{Code: "invalid_placement", Message: "placement exceeds maximum length of 100"}
			}
		}

		// Route validation
		if e.Route != nil {
			r := *e.Route
			if len(r) > maxRouteLength {
				return &StructuralError{Code: "invalid_route", Message: "route exceeds maximum length of 255"}
			}
			if strings.HasPrefix(r, "http://") || strings.HasPrefix(r, "https://") || strings.Contains(r, "://") {
				return &StructuralError{Code: "invalid_route", Message: "absolute URLs are not allowed in route"}
			}
			parsed, err := url.Parse(r)
			if err != nil || parsed.Host != "" {
				return &StructuralError{Code: "invalid_route", Message: "invalid route path"}
			}
		}

		// Event-specific required entity semantics
		switch e.EventType {
		case "catalog_impression", "product_view", "favorite_added", "favorite_removed":
			if e.ProductID == nil || *e.ProductID == uuid.Nil {
				return &StructuralError{Code: "missing_product_id", Message: e.EventType + " requires productId"}
			}
		case "product_variant_selected":
			if e.ProductID == nil || *e.ProductID == uuid.Nil {
				return &StructuralError{Code: "missing_product_id", Message: e.EventType + " requires productId"}
			}
			if e.VariantID == nil || *e.VariantID == uuid.Nil {
				return &StructuralError{Code: "missing_variant_id", Message: e.EventType + " requires variantId"}
			}
		case "add_to_cart", "remove_from_cart":
			if e.ProductID == nil || *e.ProductID == uuid.Nil {
				return &StructuralError{Code: "missing_product_id", Message: e.EventType + " requires productId"}
			}
			if e.VariantID == nil || *e.VariantID == uuid.Nil {
				return &StructuralError{Code: "missing_variant_id", Message: e.EventType + " requires variantId"}
			}
			if e.Quantity == nil {
				return &StructuralError{Code: "missing_quantity", Message: e.EventType + " requires quantity"}
			}
		case "checkout_started":
			// No required entity fields
		}
	}

	return nil
}

func (s *Service) IngestEvents(ctx context.Context, userID *uuid.UUID, req EventIngestionRequest) (*EventIngestionResponse, error) {
	// 1. Structural validation
	if err := s.ValidateStructural(&req); err != nil {
		return nil, err
	}

	resp := &EventIngestionResponse{
		Rejected: make([]RejectedEvent, 0),
	}

	var productIDs []uuid.UUID
	var variantIDs []uuid.UUID
	sessionIDMap := make(map[uuid.UUID]bool)
	var sessionIDs []uuid.UUID

	now := time.Now().UTC()
	pastBound := now.Add(-24 * time.Hour)
	futureBound := now.Add(5 * time.Minute)

	// 2. Filter out timestamp-invalid events (per-event rejection)
	for _, e := range req.Events {
		if e.OccurredAt.Before(pastBound) || e.OccurredAt.After(futureBound) {
			resp.Rejected = append(resp.Rejected, RejectedEvent{EventID: e.EventID, Code: "invalid_occurred_at"})
			continue
		}

		if e.ProductID != nil {
			productIDs = append(productIDs, *e.ProductID)
		}
		if e.VariantID != nil {
			variantIDs = append(variantIDs, *e.VariantID)
		}
		if e.SessionID != nil && !sessionIDMap[*e.SessionID] {
			sessionIDMap[*e.SessionID] = true
			sessionIDs = append(sessionIDs, *e.SessionID)
		}
	}

	// 3. Batch resolve canonical entity records and existing sessions
	validationData, err := s.repo.ValidateProductsAndVariants(ctx, variantIDs, productIDs)
	if err != nil {
		return nil, err
	}

	existingSessions, err := s.repo.GetSessionsByIDs(ctx, sessionIDs)
	if err != nil {
		return nil, err
	}

	var validEvents []BehavioralEvent
	batchSessionVisitors := make(map[uuid.UUID]uuid.UUID)

	// 4. Validate entity relationships and session ownership per event
	for _, e := range req.Events {
		// Skip if already rejected for invalid timestamp
		if e.OccurredAt.Before(pastBound) || e.OccurredAt.After(futureBound) {
			continue
		}

		// Session ownership validation (fail closed)
		if e.SessionID != nil {
			// Intra-batch session visitor consistency
			if prevVisitor, seen := batchSessionVisitors[*e.SessionID]; seen {
				if prevVisitor != e.VisitorID {
					resp.Rejected = append(resp.Rejected, RejectedEvent{EventID: e.EventID, Code: "session_visitor_mismatch"})
					continue
				}
			} else {
				batchSessionVisitors[*e.SessionID] = e.VisitorID
			}

			// Existing DB session ownership checks
			if existingSess, exists := existingSessions[*e.SessionID]; exists {
				if existingSess.VisitorID != e.VisitorID {
					resp.Rejected = append(resp.Rejected, RejectedEvent{EventID: e.EventID, Code: "session_visitor_mismatch"})
					continue
				}
				if existingSess.UserID != nil && userID != nil && *existingSess.UserID != *userID {
					resp.Rejected = append(resp.Rejected, RejectedEvent{EventID: e.EventID, Code: "session_user_mismatch"})
					continue
				}
			}
		}

		var canonicalCategoryID *uuid.UUID

		if e.VariantID != nil {
			variantRec, ok := validationData.Variants[*e.VariantID]
			if !ok {
				resp.Rejected = append(resp.Rejected, RejectedEvent{EventID: e.EventID, Code: "unknown_variant"})
				continue
			}
			if e.ProductID != nil && variantRec.ProductID != *e.ProductID {
				resp.Rejected = append(resp.Rejected, RejectedEvent{EventID: e.EventID, Code: "variant_product_mismatch"})
				continue
			}
			canonicalCategoryID = variantRec.CategoryID
		} else if e.ProductID != nil {
			prodRec, ok := validationData.Products[*e.ProductID]
			if !ok {
				resp.Rejected = append(resp.Rejected, RejectedEvent{EventID: e.EventID, Code: "unknown_product"})
				continue
			}
			canonicalCategoryID = prodRec.CategoryID
		}

		safeRoute := s.cleanRoute(e.Route)
		var meta json.RawMessage
		if len(e.Metadata) > 0 {
			if b, err := json.Marshal(e.Metadata); err == nil {
				meta = b
			} else {
				meta = json.RawMessage(`{}`)
			}
		} else {
			meta = json.RawMessage(`{}`)
		}

		validEvents = append(validEvents, BehavioralEvent{
			ID:         e.EventID,
			EventType:  e.EventType,
			Source:     SourceClient,
			VisitorID:  &e.VisitorID,
			SessionID:  e.SessionID,
			UserID:     userID,
			ProductID:  e.ProductID,
			VariantID:  e.VariantID,
			CategoryID: canonicalCategoryID,
			Quantity:   e.Quantity,
			Placement:  e.Placement,
			Route:      safeRoute,
			OccurredAt: e.OccurredAt,
			ReceivedAt: now,
			Metadata:   meta,
		})
	}

	// 5. Insert valid events
	accepted, duplicates, err := s.repo.InsertEvents(ctx, validEvents)
	if err != nil {
		return nil, err
	}

	// 6. Upsert sessions for all valid events with session_id
	var sessions []AnalyticsSession
	for _, e := range validEvents {
		if e.SessionID != nil {
			var utmSource, utmMedium, utmCampaign, utmTerm, utmContent, referrer, landingPath, source, medium, zamkToken *string

			if len(e.Metadata) > 0 {
				var metaMap map[string]interface{}
				if err := json.Unmarshal(e.Metadata, &metaMap); err == nil {
					if val, ok := metaMap["utm_source"].(string); ok && val != "" { utmSource = &val }
					if val, ok := metaMap["utm_medium"].(string); ok && val != "" { utmMedium = &val }
					if val, ok := metaMap["utm_campaign"].(string); ok && val != "" { utmCampaign = &val }
					if val, ok := metaMap["utm_term"].(string); ok && val != "" { utmTerm = &val }
					if val, ok := metaMap["zamk_token"].(string); ok && val != "" {
						if len(val) >= 8 && len(val) <= 128 && isValidTokenCharset(val) {
							zamkToken = &val
						}
					}
					if val, ok := metaMap["referrer"].(string); ok && val != "" { referrer = &val }
					if val, ok := metaMap["landing_path"].(string); ok && val != "" { landingPath = &val }
					if val, ok := metaMap["source"].(string); ok && val != "" { source = &val }
					if val, ok := metaMap["medium"].(string); ok && val != "" { medium = &val }
				}
			}

			if source == nil && utmSource != nil {
				source = utmSource
			}
			if medium == nil && utmMedium != nil {
				medium = utmMedium
			}


			var newCampaignID *uuid.UUID
			if zamkToken != nil && s.campaignResolver != nil {
				cID, err := s.campaignResolver(ctx, *zamkToken)
				if err == nil {
					newCampaignID = cID
				}
			}

			// True Last-Non-Direct-Touch: compute attribution timestamps only for non-direct touches
			var capturedAt, expiresAt *time.Time
			if (utmSource != nil && *utmSource != "") || (source != nil && *source != "") || newCampaignID != nil {

				c := e.OccurredAt
				exp := CalculateAttributionExpiry(c)
				capturedAt = &c
				expiresAt = &exp
			}

			if landingPath == nil {
				landingPath = e.Route
			}

			sessions = append(sessions, AnalyticsSession{
				ID:                    *e.SessionID,
				VisitorID:             *e.VisitorID,
				UserID:                e.UserID,
				StartedAt:             e.OccurredAt,
				LastSeenAt:            e.OccurredAt,
				LandingPath:           landingPath,
				Referrer:              referrer,
				Source:                source,
				Medium:                medium,
				UTMSource:             utmSource,
				UTMMedium:             utmMedium,
				UTMCampaign:           utmCampaign,
				CampaignID:            newCampaignID,
				UTMTerm:               utmTerm,
				UTMContent:            utmContent,
				AttributionCapturedAt: capturedAt,
				AttributionExpiresAt:  expiresAt,
			})
		}
	}

	if len(sessions) > 0 {
		if err := s.repo.UpsertSessions(ctx, sessions); err != nil {
			return nil, err
		}
	}

	resp.Accepted = accepted
	resp.Duplicates = duplicates
	return resp, nil
}

// GetActiveAttribution resolves the active unexpired non-direct attribution for a visitor.
func (s *Service) GetActiveAttribution(ctx context.Context, visitorID uuid.UUID, referenceTime time.Time) (*AnalyticsSession, error) {
	return s.repo.GetActiveAttribution(ctx, visitorID, referenceTime)
}

var (
	ErrSessionNotFound        = errors.New("analytics session not found")
	ErrSessionVisitorMismatch = errors.New("session visitor mismatch")
	ErrSessionUserMismatch    = errors.New("session bound to another user")
)

// ValidateAndBindSessionTx binds a user ID to a session if unbound, ensuring the session belongs to the visitor.
func (s *Service) ValidateAndBindSessionTx(ctx context.Context, db postgres.DBTX, visitorID, sessionID, userID uuid.UUID) error {
	query := `
		SELECT visitor_id, user_id FROM analytics_sessions
		WHERE id = $1 FOR UPDATE
	`
	var dbVisitorID uuid.UUID
	var dbUserID *uuid.UUID
	err := db.QueryRow(ctx, query, sessionID).Scan(&dbVisitorID, &dbUserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrSessionNotFound
		}
		return err
	}
	if dbVisitorID != visitorID {
		return ErrSessionVisitorMismatch
	}
	if dbUserID != nil && *dbUserID != userID {
		return ErrSessionUserMismatch
	}
	if dbUserID == nil {
		updateQuery := `UPDATE analytics_sessions SET user_id = $2 WHERE id = $1`
		_, err = db.Exec(ctx, updateQuery, sessionID, userID)
		return err
	}
	return nil
}

// RecordOrderAttributionTx validates client analytics context and records the immutable order attribution snapshot inside tx.
// Expected validation failures (nonexistent session, visitor mismatch, session bound to another user) degrade safely to
// 'missing' attribution semantics without mutating session ownership or attaching foreign attribution.
// Unexpected database failures return the error to allow canonical transaction rollback.
func (s *Service) RecordOrderAttributionTx(
	ctx context.Context,
	db postgres.DBTX,
	orderID uuid.UUID,
	userID uuid.UUID,
	clientVisitorID *uuid.UUID,
	clientSessionID *uuid.UUID,
	promoCodeID *uuid.UUID,
	orderTime time.Time,
) (*OrderAttribution, error) {
	var validVisitorID *uuid.UUID
	var validSessionID *uuid.UUID

	if clientVisitorID != nil && clientSessionID != nil && *clientVisitorID != uuid.Nil && *clientSessionID != uuid.Nil {
		err := s.ValidateAndBindSessionTx(ctx, db, *clientVisitorID, *clientSessionID, userID)
		if err == nil {
			validVisitorID = clientVisitorID
			validSessionID = clientSessionID
		} else if errors.Is(err, ErrSessionNotFound) || errors.Is(err, ErrSessionVisitorMismatch) || errors.Is(err, ErrSessionUserMismatch) || errors.Is(err, pgx.ErrNoRows) {
			// Expected domain/telemetry validation failure -> safely degrade to missing
			validVisitorID = nil
			validSessionID = nil
		} else {
			// Unexpected database failure -> return error to trigger transaction rollback
			return nil, err
		}
	}

	return s.CreateOrderAttributionFromActiveTx(ctx, db, orderID, validVisitorID, validSessionID, promoCodeID, orderTime)
}

// CreateOrderAttributionFromActiveTx creates an immutable order attribution snapshot inside a transaction.
func (s *Service) CreateOrderAttributionFromActiveTx(ctx context.Context, db postgres.DBTX, orderID uuid.UUID, visitorID *uuid.UUID, sessionID *uuid.UUID, promoCodeID *uuid.UUID, orderTime time.Time) (*OrderAttribution, error) {
	attr := OrderAttribution{
		OrderID:      orderID,
		PromoCodeID:  promoCodeID,
		AttributedAt: orderTime,
	}

	if visitorID == nil {
		// Unattributed missing context
		missingSource := "missing"
		attr.Source = &missingSource
		attr.VisitorID = nil
		attr.SessionID = nil
	} else {
		attr.VisitorID = visitorID
		attr.SessionID = sessionID
		active, err := s.repo.GetActiveAttribution(ctx, *visitorID, orderTime)
		if err != nil {
			return nil, err
		}
		if active != nil {
			attr.SessionID = &active.ID
			attr.Source = active.Source
			attr.Medium = active.Medium
			attr.UTMSource = active.UTMSource
			attr.UTMMedium = active.UTMMedium
			attr.UTMCampaign = active.UTMCampaign
			attr.UTMTerm = active.UTMTerm
			attr.UTMContent = active.UTMContent
			attr.CampaignID = active.CampaignID
		} else {
			directSource := "direct"
			attr.Source = &directSource
		}
	}

	if err := s.repo.CreateOrderAttributionSnapshot(ctx, db, attr); err != nil {
		return nil, err
	}
	return &attr, nil // Return the constructed attr since it was successfully saved
}

// CreateOrderAttributionFromActive creates an immutable order attribution snapshot using active unexpired attribution.
func (s *Service) CreateOrderAttributionFromActive(ctx context.Context, orderID uuid.UUID, visitorID uuid.UUID, promoCodeID *uuid.UUID, orderTime time.Time) (*OrderAttribution, error) {
	return s.CreateOrderAttributionFromActiveTx(ctx, s.repo.db.Pool, orderID, &visitorID, nil, promoCodeID, orderTime)
}

// GetOrderAttribution retrieves the immutable order attribution snapshot.
func (s *Service) GetOrderAttribution(ctx context.Context, orderID uuid.UUID) (*OrderAttribution, error) {
	return s.repo.GetOrderAttribution(ctx, orderID)
}

func (s *Service) cleanRoute(route *string) *string {
	if route == nil {
		return nil
	}
	r := *route
	if idx := strings.Index(r, "?"); idx != -1 {
		r = r[:idx]
	}
	if idx := strings.Index(r, "#"); idx != -1 {
		r = r[:idx]
	}
	parsed, err := url.Parse(r)
	if err == nil && parsed.Path != "" {
		r = parsed.Path
	}
	if len(r) > maxRouteLength {
		r = r[:maxRouteLength]
	}
	if len(r) == 0 {
		return nil
	}
	return &r
}

// InsertServerEventsTx handles inserting canonical server business events.
func (s *Service) InsertServerEventsTx(ctx context.Context, tx pgx.Tx, events []BehavioralEvent) error {
	for _, e := range events {
		if !serverOnlyEvents[e.EventType] {
			return fmt.Errorf("InsertServerEventsTx: %s is not a server-only event type", e.EventType)
		}
		if e.Source != SourceServer {
			return fmt.Errorf("InsertServerEventsTx: source must be server")
		}
	}
	return s.repo.InsertEventsTx(ctx, tx, events)
}

func (s *Service) ResolveCategoriesTx(ctx context.Context, tx pgx.Tx, productIDs []uuid.UUID) (map[uuid.UUID]ProductBehaviorSnapshot, error) {
	return s.repo.ResolveCategoriesTx(ctx, tx, productIDs)
}

func isValidTokenCharset(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || c == '-') {
			return false
		}
	}
	return true
}
