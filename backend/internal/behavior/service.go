package behavior

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
)

type StructuralError struct {
	Code    string
	Message string
}

func (e *StructuralError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

var clientSafeEvents = map[string]bool{
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

		// Metadata validation: must be omitted or empty map
		if len(e.Metadata) > 0 {
			return &StructuralError{Code: "invalid_metadata", Message: "Custom metadata keys are prohibited in V1"}
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
	}

	// 3. Batch resolve canonical entity records
	validationData, err := s.repo.ValidateProductsAndVariants(ctx, variantIDs, productIDs)
	if err != nil {
		return nil, err
	}

	var validEvents []BehavioralEvent

	// 4. Validate entity relationships per event
	for _, e := range req.Events {
		// Skip if already rejected for invalid timestamp
		if e.OccurredAt.Before(pastBound) || e.OccurredAt.After(futureBound) {
			continue
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
		meta := json.RawMessage(`{}`)

		validEvents = append(validEvents, BehavioralEvent{
			ID:         e.EventID,
			EventType:  e.EventType,
			Source:     SourceClient,
			VisitorID:  &e.VisitorID,
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

	resp.Accepted = accepted
	resp.Duplicates = duplicates
	return resp, nil
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
