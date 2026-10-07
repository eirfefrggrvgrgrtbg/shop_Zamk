package behavior

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type Repository struct {
	db *postgres.Client
}

func NewRepository(db *postgres.Client) *Repository {
	return &Repository{db: db}
}

// InsertEvents batch inserts events.
func (r *Repository) InsertEvents(ctx context.Context, events []BehavioralEvent) (int, int, error) {
	if len(events) == 0 {
		return 0, 0, nil
	}

	query := `
		INSERT INTO behavioral_events (
			id, event_type, source, visitor_id, session_id, user_id,
			product_id, variant_id, category_id, order_id, return_id, order_item_id,
			quantity, placement, route, occurred_at, received_at, metadata
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10, $11, $12,
			$13, $14, $15, $16, $17, $18
		)
		ON CONFLICT (id) DO NOTHING
	`

	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return 0, 0, err
	}
	defer tx.Rollback(ctx)

	accepted := 0
	for _, e := range events {
		res, err := tx.Exec(ctx, query,
			e.ID, e.EventType, e.Source, e.VisitorID, e.SessionID, e.UserID,
			e.ProductID, e.VariantID, e.CategoryID, e.OrderID, e.ReturnID, e.OrderItemID,
			e.Quantity, e.Placement, e.Route, e.OccurredAt, e.ReceivedAt, e.Metadata,
		)
		if err != nil {
			return 0, 0, err
		}
		if res.RowsAffected() == 1 {
			accepted++
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, 0, err
	}

	duplicates := len(events) - accepted
	return accepted, duplicates, nil
}

// InsertEventsTx inserts events within an existing transaction.
func (r *Repository) InsertEventsTx(ctx context.Context, tx pgx.Tx, events []BehavioralEvent) error {
	if len(events) == 0 {
		return nil
	}
	query := `
		INSERT INTO behavioral_events (
			id, event_type, source, visitor_id, session_id, user_id,
			product_id, variant_id, category_id, order_id, return_id, order_item_id,
			quantity, placement, route, occurred_at, received_at, metadata
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10, $11, $12,
			$13, $14, $15, $16, $17, $18
		)
		ON CONFLICT (id) DO NOTHING
	`
	for _, e := range events {
		_, err := tx.Exec(ctx, query,
			e.ID, e.EventType, e.Source, e.VisitorID, e.SessionID, e.UserID,
			e.ProductID, e.VariantID, e.CategoryID, e.OrderID, e.ReturnID, e.OrderItemID,
			e.Quantity, e.Placement, e.Route, e.OccurredAt, e.ReceivedAt, e.Metadata,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

// VariantRecord represents an existing variant and its canonical product/category.
type VariantRecord struct {
	ProductID  uuid.UUID
	CategoryID *uuid.UUID
}

// ProductRecord represents an existing product and its canonical category.
type ProductRecord struct {
	CategoryID *uuid.UUID
}

// ValidationData contains preloaded entity records for batch validation.
type ValidationData struct {
	Variants map[uuid.UUID]VariantRecord
	Products map[uuid.UUID]ProductRecord
}

// ValidateProductsAndVariants performs batch queries to check variants and products, resolving canonical category IDs.
func (r *Repository) ValidateProductsAndVariants(ctx context.Context, variantIDs []uuid.UUID, productIDs []uuid.UUID) (ValidationData, error) {
	data := ValidationData{
		Variants: make(map[uuid.UUID]VariantRecord),
		Products: make(map[uuid.UUID]ProductRecord),
	}

	if len(variantIDs) > 0 {
		queryVariants := `
			SELECT v.id, v.product_id, p.category_id
			FROM product_variants v
			JOIN products p ON p.id = v.product_id
			WHERE v.id = ANY($1)
		`
		rows, err := r.db.Pool.Query(ctx, queryVariants, variantIDs)
		if err != nil {
			return data, err
		}
		defer rows.Close()

		for rows.Next() {
			var vID uuid.UUID
			var pID uuid.UUID
			var cID *uuid.UUID
			if err := rows.Scan(&vID, &pID, &cID); err != nil {
				return data, err
			}
			data.Variants[vID] = VariantRecord{
				ProductID:  pID,
				CategoryID: cID,
			}
		}
	}

	if len(productIDs) > 0 {
		queryProducts := `
			SELECT id, category_id
			FROM products
			WHERE id = ANY($1)
		`
		rows, err := r.db.Pool.Query(ctx, queryProducts, productIDs)
		if err != nil {
			return data, err
		}
		defer rows.Close()

		for rows.Next() {
			var pID uuid.UUID
			var cID *uuid.UUID
			if err := rows.Scan(&pID, &cID); err != nil {
				return data, err
			}
			data.Products[pID] = ProductRecord{
				CategoryID: cID,
			}
		}
	}

	return data, nil
}

// ProductBehaviorSnapshot represents a verified canonical product row and its nullable category_id.
type ProductBehaviorSnapshot struct {
	CategoryID *uuid.UUID
}

func (r *Repository) ResolveCategoriesTx(ctx context.Context, tx pgx.Tx, productIDs []uuid.UUID) (map[uuid.UUID]ProductBehaviorSnapshot, error) {
	res := make(map[uuid.UUID]ProductBehaviorSnapshot, len(productIDs))
	if len(productIDs) == 0 {
		return res, nil
	}
	query := `SELECT id, category_id FROM products WHERE id = ANY($1)`
	rows, err := tx.Query(ctx, query, productIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var id uuid.UUID
		var catID *uuid.UUID
		if err := rows.Scan(&id, &catID); err != nil {
			return nil, err
		}
		res[id] = ProductBehaviorSnapshot{CategoryID: catID}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for _, pid := range productIDs {
		if _, ok := res[pid]; !ok {
			return nil, fmt.Errorf("ResolveCategoriesTx: canonical product %s not found", pid)
		}
	}
	return res, nil
}

// GetSessionsByIDs fetches existing analytics sessions for given session IDs.
func (r *Repository) GetSessionsByIDs(ctx context.Context, sessionIDs []uuid.UUID) (map[uuid.UUID]AnalyticsSession, error) {
	result := make(map[uuid.UUID]AnalyticsSession, len(sessionIDs))
	if len(sessionIDs) == 0 {
		return result, nil
	}

	query := `
		SELECT id, visitor_id, user_id, started_at, last_seen_at, landing_path, referrer,
		       source, medium, utm_source, utm_medium, utm_campaign, utm_term, utm_content,
		       campaign_id, attribution_captured_at, attribution_expires_at
		FROM analytics_sessions
		WHERE id = ANY($1)
	`
	rows, err := r.db.Pool.Query(ctx, query, sessionIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var s AnalyticsSession
		if err := rows.Scan(
			&s.ID, &s.VisitorID, &s.UserID, &s.StartedAt, &s.LastSeenAt,
			&s.LandingPath, &s.Referrer, &s.Source, &s.Medium,
			&s.UTMSource, &s.UTMMedium, &s.UTMCampaign, &s.UTMTerm, &s.UTMContent,
			&s.CampaignID, &s.AttributionCapturedAt, &s.AttributionExpiresAt,
		); err != nil {
			return nil, err
		}
		result[s.ID] = s
	}

	return result, rows.Err()
}

// UpsertSessions inserts or updates analytics sessions based on ingested events.
// DIRECT touch: updates last_seen_at, associates server-inferred user_id, PRESERVES existing non-direct attribution.
// NEW NON-DIRECT touch: replaces attribution fields, updates attribution_captured_at and attribution_expires_at.
// Fails closed if session belongs to a different visitor_id.
func (r *Repository) UpsertSessions(ctx context.Context, sessions []AnalyticsSession) error {
	if len(sessions) == 0 {
		return nil
	}

	query := `
		INSERT INTO analytics_sessions (
			id, visitor_id, user_id, started_at, last_seen_at,
			landing_path, referrer, source, medium,
			utm_source, utm_medium, utm_campaign, utm_term, utm_content,
			campaign_id, attribution_captured_at, attribution_expires_at
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9,
			$10, $11, $12, $13, $14,
			$15, $16, $17
		)
		ON CONFLICT (id) DO UPDATE SET
			last_seen_at = GREATEST(analytics_sessions.last_seen_at, EXCLUDED.last_seen_at),
			user_id = CASE
				WHEN analytics_sessions.user_id IS NOT NULL AND EXCLUDED.user_id IS NOT NULL AND analytics_sessions.user_id != EXCLUDED.user_id
					THEN analytics_sessions.user_id
				ELSE COALESCE(analytics_sessions.user_id, EXCLUDED.user_id)
			END,
			landing_path = COALESCE(analytics_sessions.landing_path, EXCLUDED.landing_path),
			source = CASE WHEN (EXCLUDED.utm_source IS NOT NULL OR EXCLUDED.source IS NOT NULL OR EXCLUDED.campaign_id IS NOT NULL) THEN EXCLUDED.source ELSE analytics_sessions.source END,
			medium = CASE WHEN (EXCLUDED.utm_source IS NOT NULL OR EXCLUDED.source IS NOT NULL OR EXCLUDED.campaign_id IS NOT NULL) THEN EXCLUDED.medium ELSE analytics_sessions.medium END,
			utm_source = CASE WHEN (EXCLUDED.utm_source IS NOT NULL OR EXCLUDED.source IS NOT NULL OR EXCLUDED.campaign_id IS NOT NULL) THEN EXCLUDED.utm_source ELSE analytics_sessions.utm_source END,
			utm_medium = CASE WHEN (EXCLUDED.utm_source IS NOT NULL OR EXCLUDED.source IS NOT NULL OR EXCLUDED.campaign_id IS NOT NULL) THEN EXCLUDED.utm_medium ELSE analytics_sessions.utm_medium END,
			utm_campaign = CASE WHEN (EXCLUDED.utm_source IS NOT NULL OR EXCLUDED.source IS NOT NULL OR EXCLUDED.campaign_id IS NOT NULL) THEN EXCLUDED.utm_campaign ELSE analytics_sessions.utm_campaign END,
			utm_term = CASE WHEN (EXCLUDED.utm_source IS NOT NULL OR EXCLUDED.source IS NOT NULL OR EXCLUDED.campaign_id IS NOT NULL) THEN EXCLUDED.utm_term ELSE analytics_sessions.utm_term END,
			utm_content = CASE WHEN (EXCLUDED.utm_source IS NOT NULL OR EXCLUDED.source IS NOT NULL OR EXCLUDED.campaign_id IS NOT NULL) THEN EXCLUDED.utm_content ELSE analytics_sessions.utm_content END,
			referrer = CASE WHEN (EXCLUDED.utm_source IS NOT NULL OR EXCLUDED.source IS NOT NULL OR EXCLUDED.campaign_id IS NOT NULL) THEN EXCLUDED.referrer ELSE COALESCE(analytics_sessions.referrer, EXCLUDED.referrer) END,
			campaign_id = CASE WHEN (EXCLUDED.utm_source IS NOT NULL OR EXCLUDED.source IS NOT NULL OR EXCLUDED.campaign_id IS NOT NULL) THEN EXCLUDED.campaign_id ELSE analytics_sessions.campaign_id END,
			attribution_captured_at = CASE WHEN (EXCLUDED.utm_source IS NOT NULL OR EXCLUDED.source IS NOT NULL OR EXCLUDED.campaign_id IS NOT NULL) THEN EXCLUDED.attribution_captured_at ELSE analytics_sessions.attribution_captured_at END,
			attribution_expires_at = CASE WHEN (EXCLUDED.utm_source IS NOT NULL OR EXCLUDED.source IS NOT NULL OR EXCLUDED.campaign_id IS NOT NULL) THEN EXCLUDED.attribution_expires_at ELSE analytics_sessions.attribution_expires_at END
		WHERE analytics_sessions.visitor_id = EXCLUDED.visitor_id
	`

	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	for _, s := range sessions {
		_, err := tx.Exec(ctx, query,
			s.ID, s.VisitorID, s.UserID, s.StartedAt, s.LastSeenAt,
			s.LandingPath, s.Referrer, s.Source, s.Medium,
			s.UTMSource, s.UTMMedium, s.UTMCampaign, s.UTMTerm, s.UTMContent,
			s.CampaignID, s.AttributionCapturedAt, s.AttributionExpiresAt,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

// GetActiveAttribution finds the latest valid non-direct attribution for a visitor within the 30-day window.
// If the latest non-direct attribution is expired relative to referenceTime, or none exists, it returns nil, nil.
func (r *Repository) GetActiveAttribution(ctx context.Context, visitorID uuid.UUID, referenceTime time.Time) (*AnalyticsSession, error) {
	query := `
		SELECT id, visitor_id, user_id, started_at, last_seen_at, landing_path, referrer,
		       source, medium, utm_source, utm_medium, utm_campaign, utm_term, utm_content,
		       campaign_id, attribution_captured_at, attribution_expires_at
		FROM analytics_sessions
		WHERE visitor_id = $1
		  AND attribution_captured_at IS NOT NULL
		  AND attribution_expires_at IS NOT NULL
		  AND attribution_expires_at > $2
		ORDER BY attribution_captured_at DESC, started_at DESC
		LIMIT 1
	`
	var s AnalyticsSession
	err := r.db.Pool.QueryRow(ctx, query, visitorID, referenceTime).Scan(
		&s.ID, &s.VisitorID, &s.UserID, &s.StartedAt, &s.LastSeenAt,
		&s.LandingPath, &s.Referrer, &s.Source, &s.Medium,
		&s.UTMSource, &s.UTMMedium, &s.UTMCampaign, &s.UTMTerm, &s.UTMContent,
		&s.CampaignID, &s.AttributionCapturedAt, &s.AttributionExpiresAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &s, nil
}

// CreateOrderAttributionSnapshot creates an immutable order attribution snapshot.
func (r *Repository) CreateOrderAttributionSnapshot(ctx context.Context, db postgres.DBTX, attr OrderAttribution) error {
	query := `
		INSERT INTO order_attributions (
			order_id, session_id, visitor_id,
			source, medium, utm_source, utm_medium, utm_campaign, utm_term, utm_content,
			campaign_id, promo_code_id, attributed_at
		) VALUES (
			$1, $2, $3,
			$4, $5, $6, $7, $8, $9, $10,
			$11, $12, $13
		)
		ON CONFLICT (order_id) DO NOTHING
	`
	_, err := db.Exec(ctx, query,
		attr.OrderID, attr.SessionID, attr.VisitorID,
		attr.Source, attr.Medium, attr.UTMSource, attr.UTMMedium, attr.UTMCampaign, attr.UTMTerm, attr.UTMContent,
		attr.CampaignID, attr.PromoCodeID, attr.AttributedAt,
	)
	return err
}

// GetOrderAttribution retrieves the immutable order attribution snapshot for an order.
func (r *Repository) GetOrderAttribution(ctx context.Context, orderID uuid.UUID) (*OrderAttribution, error) {
	query := `
		SELECT order_id, session_id, visitor_id,
		       source, medium, utm_source, utm_medium, utm_campaign, utm_term, utm_content,
		       campaign_id, promo_code_id, attributed_at
		FROM order_attributions
		WHERE order_id = $1
	`
	var a OrderAttribution
	err := r.db.Pool.QueryRow(ctx, query, orderID).Scan(
		&a.OrderID, &a.SessionID, &a.VisitorID,
		&a.Source, &a.Medium, &a.UTMSource, &a.UTMMedium, &a.UTMCampaign, &a.UTMTerm, &a.UTMContent,
		&a.CampaignID, &a.PromoCodeID, &a.AttributedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &a, nil
}
