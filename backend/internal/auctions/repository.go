package auctions

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// CreateEvent inserts a new auction event.
func (r *Repository) CreateEvent(ctx context.Context, e *AuctionEvent) error {
	query := `
		INSERT INTO auction_events (
			id, title, description, status, starts_at, ends_at, bid_step_cents, 
			payment_deadline_hours, anti_sniping_enabled, anti_sniping_trigger_seconds, 
			anti_sniping_extension_seconds, max_bids_per_user_per_lot_per_minute, 
			max_rejected_bids_per_user_per_minute, no_bids_policy, unpaid_winner_policy, 
			is_public, show_on_homepage, highlight_in_nav, bidding_enabled, created_by, 
			created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22
		)
	`
	_, err := r.db.Exec(ctx, query,
		e.ID, e.Title, e.Description, e.Status, e.StartsAt, e.EndsAt, e.BidStepCents,
		e.PaymentDeadlineHours, e.AntiSnipingEnabled, e.AntiSnipingTriggerSeconds,
		e.AntiSnipingExtensionSeconds, e.MaxBidsPerUserPerLotPerMinute,
		e.MaxRejectedBidsPerUserPerMinute, e.NoBidsPolicy, e.UnpaidWinnerPolicy,
		e.IsPublic, e.ShowOnHomepage, e.HighlightInNav, e.BiddingEnabled, e.CreatedBy,
		e.CreatedAt, e.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create auction event: %w", err)
	}
	return nil
}

// GetEventByID fetches an auction event without lots.
func (r *Repository) GetEventByID(ctx context.Context, id uuid.UUID) (*AuctionEvent, error) {
	query := `
		SELECT id, title, description, status, starts_at, ends_at, bid_step_cents,
			payment_deadline_hours, anti_sniping_enabled, anti_sniping_trigger_seconds,
			anti_sniping_extension_seconds, max_bids_per_user_per_lot_per_minute,
			max_rejected_bids_per_user_per_minute, no_bids_policy, unpaid_winner_policy,
			is_public, show_on_homepage, highlight_in_nav, bidding_enabled, created_by,
			created_at, updated_at
		FROM auction_events
		WHERE id = $1
	`
	row := r.db.QueryRow(ctx, query, id)
	var e AuctionEvent
	err := row.Scan(
		&e.ID, &e.Title, &e.Description, &e.Status, &e.StartsAt, &e.EndsAt, &e.BidStepCents,
		&e.PaymentDeadlineHours, &e.AntiSnipingEnabled, &e.AntiSnipingTriggerSeconds,
		&e.AntiSnipingExtensionSeconds, &e.MaxBidsPerUserPerLotPerMinute,
		&e.MaxRejectedBidsPerUserPerMinute, &e.NoBidsPolicy, &e.UnpaidWinnerPolicy,
		&e.IsPublic, &e.ShowOnHomepage, &e.HighlightInNav, &e.BiddingEnabled, &e.CreatedBy,
		&e.CreatedAt, &e.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get auction event: %w", err)
	}
	return &e, nil
}

// CreateLot inserts a new lot along with images and attributes.
func (r *Repository) CreateLot(ctx context.Context, lot *AuctionLot) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	query := `
		INSERT INTO auction_lots (
			id, auction_id, title, description, image_url, start_price_cents, 
			current_bid_cents, bid_step_cents, current_winner_user_id, status, 
			order_id, payment_deadline_at, can_relaunch, can_move_to_direct_sale, 
			direct_sale_price_cents, direct_sale_product_id, admin_note, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19
		)
	`
	_, err = tx.Exec(ctx, query,
		lot.ID, lot.AuctionID, lot.Title, lot.Description, lot.ImageURL, lot.StartPriceCents,
		lot.CurrentBidCents, lot.BidStepCents, lot.CurrentWinnerUserID, lot.Status,
		lot.OrderID, lot.PaymentDeadlineAt, lot.CanRelaunch, lot.CanMoveToDirectSale,
		lot.DirectSalePriceCents, lot.DirectSaleProductID, lot.AdminNote, lot.CreatedAt, lot.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to insert lot: %w", err)
	}

	for _, img := range lot.Images {
		_, err = tx.Exec(ctx, `INSERT INTO auction_lot_images (id, lot_id, image_url, sort_order, is_primary, created_at) VALUES ($1, $2, $3, $4, $5, $6)`,
			img.ID, lot.ID, img.ImageURL, img.SortOrder, img.IsPrimary, img.CreatedAt)
		if err != nil {
			return fmt.Errorf("failed to insert lot image: %w", err)
		}
	}

	for _, attr := range lot.Attributes {
		_, err = tx.Exec(ctx, `INSERT INTO auction_lot_attributes (id, lot_id, name, value, sort_order) VALUES ($1, $2, $3, $4, $5)`,
			attr.ID, lot.ID, attr.Name, attr.Value, attr.SortOrder)
		if err != nil {
			return fmt.Errorf("failed to insert lot attribute: %w", err)
		}
	}

	return tx.Commit(ctx)
}

// GetLotByID fetches a single lot (excluding heavy joins for simple lookups).
func (r *Repository) GetLotByID(ctx context.Context, id uuid.UUID) (*AuctionLot, error) {
	query := `
		SELECT id, auction_id, title, description, image_url, start_price_cents, 
			current_bid_cents, bid_step_cents, current_winner_user_id, status, 
			order_id, payment_deadline_at, can_relaunch, can_move_to_direct_sale, 
			direct_sale_price_cents, direct_sale_product_id, admin_note, created_at, updated_at
		FROM auction_lots
		WHERE id = $1
	`
	row := r.db.QueryRow(ctx, query, id)
	var l AuctionLot
	err := row.Scan(
		&l.ID, &l.AuctionID, &l.Title, &l.Description, &l.ImageURL, &l.StartPriceCents,
		&l.CurrentBidCents, &l.BidStepCents, &l.CurrentWinnerUserID, &l.Status,
		&l.OrderID, &l.PaymentDeadlineAt, &l.CanRelaunch, &l.CanMoveToDirectSale,
		&l.DirectSalePriceCents, &l.DirectSaleProductID, &l.AdminNote, &l.CreatedAt, &l.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get auction lot: %w", err)
	}
	return &l, nil
}

// UpdateEvent updates a subset of fields.
func (r *Repository) UpdateEvent(ctx context.Context, e *AuctionEvent) error {
	query := `
		UPDATE auction_events SET
			title = $2, description = $3, status = $4, starts_at = $5, ends_at = $6, 
			bid_step_cents = $7, payment_deadline_hours = $8, anti_sniping_enabled = $9, 
			anti_sniping_trigger_seconds = $10, anti_sniping_extension_seconds = $11, 
			max_bids_per_user_per_lot_per_minute = $12, max_rejected_bids_per_user_per_minute = $13, 
			no_bids_policy = $14, unpaid_winner_policy = $15, is_public = $16, show_on_homepage = $17, 
			highlight_in_nav = $18, bidding_enabled = $19, updated_at = now()
		WHERE id = $1
	`
	_, err := r.db.Exec(ctx, query,
		e.ID, e.Title, e.Description, e.Status, e.StartsAt, e.EndsAt, e.BidStepCents,
		e.PaymentDeadlineHours, e.AntiSnipingEnabled, e.AntiSnipingTriggerSeconds,
		e.AntiSnipingExtensionSeconds, e.MaxBidsPerUserPerLotPerMinute,
		e.MaxRejectedBidsPerUserPerMinute, e.NoBidsPolicy, e.UnpaidWinnerPolicy,
		e.IsPublic, e.ShowOnHomepage, e.HighlightInNav, e.BiddingEnabled,
	)
	return err
}

// UpdateEventStatus updates just the event status
func (r *Repository) UpdateEventStatus(ctx context.Context, id uuid.UUID, status AuctionStatus) error {
	_, err := r.db.Exec(ctx, "UPDATE auction_events SET status = $1, updated_at = now() WHERE id = $2", status, id)
	return err
}

// UpdateLot updates a subset of fields.
func (r *Repository) UpdateLot(ctx context.Context, l *AuctionLot) error {
	query := `
		UPDATE auction_lots SET
			title = $2, description = $3, start_price_cents = $4, bid_step_cents = $5,
			can_relaunch = $6, can_move_to_direct_sale = $7, direct_sale_price_cents = $8,
			admin_note = $9, updated_at = now()
		WHERE id = $1
	`
	_, err := r.db.Exec(ctx, query,
		l.ID, l.Title, l.Description, l.StartPriceCents, l.BidStepCents,
		l.CanRelaunch, l.CanMoveToDirectSale, l.DirectSalePriceCents,
		l.AdminNote,
	)
	return err
}

// UpdateLotStatus updates just the lot status
func (r *Repository) UpdateLotStatus(ctx context.Context, id uuid.UUID, status LotStatus) error {
	_, err := r.db.Exec(ctx, "UPDATE auction_lots SET status = $1, updated_at = now() WHERE id = $2", status, id)
	return err
}

// ExecTx runs a generic transaction for complex operations.
func (r *Repository) ExecTx(ctx context.Context, fn func(pgx.Tx) error) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// GetLotForUpdate locks the lot row.
func (r *Repository) GetLotForUpdate(ctx context.Context, tx pgx.Tx, lotID uuid.UUID) (*AuctionLot, error) {
	query := `
		SELECT id, auction_id, start_price_cents, current_bid_cents, bid_step_cents, 
			current_winner_user_id, status, payment_deadline_at, order_id,
			can_move_to_direct_sale, direct_sale_price_cents, can_relaunch
		FROM auction_lots 
		WHERE id = $1 FOR UPDATE
	`
	row := tx.QueryRow(ctx, query, lotID)
	var l AuctionLot
	err := row.Scan(&l.ID, &l.AuctionID, &l.StartPriceCents, &l.CurrentBidCents, &l.BidStepCents, &l.CurrentWinnerUserID, &l.Status, &l.PaymentDeadlineAt, &l.OrderID, &l.CanMoveToDirectSale, &l.DirectSalePriceCents, &l.CanRelaunch)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &l, nil
}

// GetEventForUpdate locks the event row.
func (r *Repository) GetEventForUpdate(ctx context.Context, tx pgx.Tx, eventID uuid.UUID) (*AuctionEvent, error) {
	query := `
		SELECT id, status, starts_at, ends_at, bidding_enabled, anti_sniping_enabled,
			anti_sniping_trigger_seconds, anti_sniping_extension_seconds, is_public
		FROM auction_events 
		WHERE id = $1 FOR UPDATE
	`
	row := tx.QueryRow(ctx, query, eventID)
	var e AuctionEvent
	err := row.Scan(&e.ID, &e.Status, &e.StartsAt, &e.EndsAt, &e.BiddingEnabled, &e.AntiSnipingEnabled, &e.AntiSnipingTriggerSeconds, &e.AntiSnipingExtensionSeconds, &e.IsPublic)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &e, nil
}

func (r *Repository) InsertBidTx(ctx context.Context, tx pgx.Tx, bid *AuctionBid) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO auction_bids (id, auction_id, lot_id, user_id, amount_cents, idempotency_key, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, bid.ID, bid.AuctionID, bid.LotID, bid.UserID, bid.AmountCents, bid.IdempotencyKey, bid.CreatedAt)
	return err
}

func (r *Repository) UpdateLotBidTx(ctx context.Context, tx pgx.Tx, lotID uuid.UUID, amountCents int64, winnerID uuid.UUID) error {
	_, err := tx.Exec(ctx, `
		UPDATE auction_lots 
		SET current_bid_cents = $1, current_winner_user_id = $2, updated_at = now() 
		WHERE id = $3
	`, amountCents, winnerID, lotID)
	return err
}

func (r *Repository) ExtendAuctionTx(ctx context.Context, tx pgx.Tx, eventID uuid.UUID, newEndsAt time.Time) error {
	_, err := tx.Exec(ctx, `
		UPDATE auction_events SET ends_at = $1, updated_at = now() WHERE id = $2
	`, newEndsAt, eventID)
	return err
}

func (r *Repository) InsertLogTx(ctx context.Context, tx pgx.Tx, l *AuctionLog) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO auction_logs (id, auction_id, lot_id, actor_user_id, action, metadata, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, l.ID, l.AuctionID, l.LotID, l.ActorUserID, l.Action, l.Metadata, l.CreatedAt)
	return err
}

func (r *Repository) LogSuspiciousEvent(ctx context.Context, s *AuctionSuspiciousEvent) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO auction_suspicious_events (id, auction_id, lot_id, user_id, reason, metadata, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, s.ID, s.AuctionID, s.LotID, s.UserID, s.Reason, s.Metadata, s.CreatedAt)
	return err
}

func (r *Repository) CheckIdempotencyKey(ctx context.Context, lotID, userID uuid.UUID, key string) (*AuctionBid, error) {
	query := `SELECT id, amount_cents FROM auction_bids WHERE lot_id = $1 AND user_id = $2 AND idempotency_key = $3 LIMIT 1`
	var b AuctionBid
	err := r.db.QueryRow(ctx, query, lotID, userID, key).Scan(&b.ID, &b.AmountCents)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &b, nil
}

func (r *Repository) ListActiveAuctions(ctx context.Context) ([]AuctionEvent, error) {
	query := `
		SELECT id, title, description, status, starts_at, ends_at, bid_step_cents, is_public, show_on_homepage, highlight_in_nav
		FROM auction_events
		WHERE status IN ('scheduled', 'live') AND is_public = true
		ORDER BY starts_at ASC
	`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []AuctionEvent
	for rows.Next() {
		var e AuctionEvent
		if err := rows.Scan(&e.ID, &e.Title, &e.Description, &e.Status, &e.StartsAt, &e.EndsAt, &e.BidStepCents, &e.IsPublic, &e.ShowOnHomepage, &e.HighlightInNav); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, nil
}

func (r *Repository) ListAllEventsAdmin(ctx context.Context) ([]AuctionEvent, error) {
	query := `
		SELECT id, title, description, status, starts_at, ends_at, bid_step_cents, is_public, show_on_homepage, highlight_in_nav
		FROM auction_events
		ORDER BY created_at DESC
	`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []AuctionEvent
	for rows.Next() {
		var e AuctionEvent
		if err := rows.Scan(&e.ID, &e.Title, &e.Description, &e.Status, &e.StartsAt, &e.EndsAt, &e.BidStepCents, &e.IsPublic, &e.ShowOnHomepage, &e.HighlightInNav); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, nil
}

func (r *Repository) GetEventByIDWithLots(ctx context.Context, id uuid.UUID) (*AuctionEvent, error) {
	event, err := r.GetEventByID(ctx, id)
	if err != nil || event == nil {
		return event, err
	}
	
	lots, err := r.GetLotsByAuctionID(ctx, id)
	if err != nil {
		return nil, err
	}
	event.Lots = lots
	return event, nil
}

func (r *Repository) GetLotsByAuctionID(ctx context.Context, id uuid.UUID) ([]AuctionLot, error) {
	query := `
		SELECT id, auction_id, title, description, image_url, start_price_cents, 
			current_bid_cents, bid_step_cents, current_winner_user_id, status, 
			order_id, payment_deadline_at, can_relaunch, can_move_to_direct_sale, 
			direct_sale_price_cents, direct_sale_product_id, admin_note, created_at, updated_at
		FROM auction_lots
		WHERE auction_id = $1
		ORDER BY created_at ASC
	`
	rows, err := r.db.Query(ctx, query, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var lots []AuctionLot
	for rows.Next() {
		var l AuctionLot
		if err := rows.Scan(
			&l.ID, &l.AuctionID, &l.Title, &l.Description, &l.ImageURL, &l.StartPriceCents,
			&l.CurrentBidCents, &l.BidStepCents, &l.CurrentWinnerUserID, &l.Status,
			&l.OrderID, &l.PaymentDeadlineAt, &l.CanRelaunch, &l.CanMoveToDirectSale,
			&l.DirectSalePriceCents, &l.DirectSaleProductID, &l.AdminNote, &l.CreatedAt, &l.UpdatedAt,
		); err != nil {
			return nil, err
		}
		lots = append(lots, l)
	}
	return lots, nil
}

func (r *Repository) GetPublicLotsByAuctionID(ctx context.Context, id uuid.UUID) ([]AuctionLot, error) {
	query := `
		SELECT id, auction_id, title, description, image_url, start_price_cents, 
			current_bid_cents, bid_step_cents, current_winner_user_id, status, 
			order_id, payment_deadline_at, can_relaunch, can_move_to_direct_sale, 
			direct_sale_price_cents, direct_sale_product_id, admin_note, created_at, updated_at
		FROM auction_lots
		WHERE auction_id = $1 AND status NOT IN ('draft', 'cancelled')
		ORDER BY created_at ASC
	`
	rows, err := r.db.Query(ctx, query, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var lots []AuctionLot
	for rows.Next() {
		var l AuctionLot
		if err := rows.Scan(
			&l.ID, &l.AuctionID, &l.Title, &l.Description, &l.ImageURL, &l.StartPriceCents,
			&l.CurrentBidCents, &l.BidStepCents, &l.CurrentWinnerUserID, &l.Status,
			&l.OrderID, &l.PaymentDeadlineAt, &l.CanRelaunch, &l.CanMoveToDirectSale,
			&l.DirectSalePriceCents, &l.DirectSaleProductID, &l.AdminNote, &l.CreatedAt, &l.UpdatedAt,
		); err != nil {
			return nil, err
		}
		lots = append(lots, l)
	}
	return lots, nil
}

func (r *Repository) GetLotByIDWithDetails(ctx context.Context, id uuid.UUID) (*AuctionLot, error) {
	lot, err := r.GetLotByID(ctx, id)
	if err != nil || lot == nil {
		return lot, err
	}

	// Images
	irows, err := r.db.Query(ctx, "SELECT id, lot_id, image_url, sort_order, is_primary, created_at FROM auction_lot_images WHERE lot_id = $1 ORDER BY sort_order ASC", id)
	if err != nil {
		return nil, err
	}
	defer irows.Close()
	var images []AuctionLotImage
	for irows.Next() {
		var img AuctionLotImage
		if err := irows.Scan(&img.ID, &img.LotID, &img.ImageURL, &img.SortOrder, &img.IsPrimary, &img.CreatedAt); err != nil {
			return nil, err
		}
		images = append(images, img)
	}
	lot.Images = images

	// Attributes
	arows, err := r.db.Query(ctx, "SELECT id, lot_id, name, value, sort_order FROM auction_lot_attributes WHERE lot_id = $1 ORDER BY sort_order ASC", id)
	if err != nil {
		return nil, err
	}
	defer arows.Close()
	var attrs []AuctionLotAttribute
	for arows.Next() {
		var attr AuctionLotAttribute
		if err := arows.Scan(&attr.ID, &attr.LotID, &attr.Name, &attr.Value, &attr.SortOrder); err != nil {
			return nil, err
		}
		attrs = append(attrs, attr)
	}
	lot.Attributes = attrs

	return lot, nil
}

func (r *Repository) ListHomepageAuctions(ctx context.Context) ([]AuctionEvent, error) {
	query := `
		SELECT id, title, description, status, starts_at, ends_at, bid_step_cents, is_public, show_on_homepage, highlight_in_nav
		FROM auction_events
		WHERE status IN ('scheduled', 'live') AND is_public = true AND show_on_homepage = true
		ORDER BY starts_at ASC
	`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []AuctionEvent
	for rows.Next() {
		var e AuctionEvent
		if err := rows.Scan(&e.ID, &e.Title, &e.Description, &e.Status, &e.StartsAt, &e.EndsAt, &e.BidStepCents, &e.IsPublic, &e.ShowOnHomepage, &e.HighlightInNav); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, nil
}

func (r *Repository) ListNavHighlightAuctions(ctx context.Context) ([]AuctionEvent, error) {
	query := `
		SELECT id, title, description, status, starts_at, ends_at, bid_step_cents, is_public, show_on_homepage, highlight_in_nav
		FROM auction_events
		WHERE status IN ('scheduled', 'live') AND is_public = true AND highlight_in_nav = true
		ORDER BY starts_at ASC
	`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []AuctionEvent
	for rows.Next() {
		var e AuctionEvent
		if err := rows.Scan(&e.ID, &e.Title, &e.Description, &e.Status, &e.StartsAt, &e.EndsAt, &e.BidStepCents, &e.IsPublic, &e.ShowOnHomepage, &e.HighlightInNav); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, nil
}

func (r *Repository) GetBidsByLotID(ctx context.Context, lotID uuid.UUID) ([]AuctionBid, error) {
	query := `
		SELECT id, auction_id, lot_id, user_id, amount_cents, idempotency_key, created_at
		FROM auction_bids
		WHERE lot_id = $1
		ORDER BY created_at DESC
	`
	rows, err := r.db.Query(ctx, query, lotID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bids []AuctionBid
	for rows.Next() {
		var b AuctionBid
		if err := rows.Scan(&b.ID, &b.AuctionID, &b.LotID, &b.UserID, &b.AmountCents, &b.IdempotencyKey, &b.CreatedAt); err != nil {
			return nil, err
		}
		bids = append(bids, b)
	}
	return bids, nil
}

func (r *Repository) GetAuctionWinsByUserID(ctx context.Context, userID uuid.UUID) ([]AuctionLot, error) {
	query := `
		SELECT id, auction_id, title, description, image_url, start_price_cents, 
			current_bid_cents, bid_step_cents, current_winner_user_id, status, 
			order_id, payment_deadline_at, can_relaunch, can_move_to_direct_sale, 
			direct_sale_price_cents, direct_sale_product_id, admin_note, created_at, updated_at
		FROM auction_lots
		WHERE current_winner_user_id = $1 AND status IN ('won_pending_payment', 'paid')
		ORDER BY updated_at DESC
	`
	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var lots []AuctionLot
	for rows.Next() {
		var l AuctionLot
		if err := rows.Scan(
			&l.ID, &l.AuctionID, &l.Title, &l.Description, &l.ImageURL, &l.StartPriceCents,
			&l.CurrentBidCents, &l.BidStepCents, &l.CurrentWinnerUserID, &l.Status,
			&l.OrderID, &l.PaymentDeadlineAt, &l.CanRelaunch, &l.CanMoveToDirectSale,
			&l.DirectSalePriceCents, &l.DirectSaleProductID, &l.AdminNote, &l.CreatedAt, &l.UpdatedAt,
		); err != nil {
			return nil, err
		}
		lots = append(lots, l)
	}
	return lots, nil
}

func (r *Repository) CreateAuctionOrderTx(ctx context.Context, tx pgx.Tx, orderID, userID, auctionID, lotID uuid.UUID, amountCents int64) error {
	var name, email, phone string
	err := tx.QueryRow(ctx, `SELECT COALESCE(first_name || ' ' || last_name, ''), email, COALESCE(phone, '') FROM users WHERE id = $1`, userID).Scan(&name, &email, &phone)
	if err != nil {
		return err
	}
	if name == " " || name == "" {
		name = "Auction Winner"
	}
	if phone == "" {
		phone = "-"
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO orders (id, user_id, status, total_price_cents, currency, customer_name, customer_phone, customer_email, delivery_address, created_at, updated_at)
		VALUES ($1, $2, 'awaiting_payment', $3, 'RUB', $4, $5, $6, '-', now(), now())
	`, orderID, userID, amountCents, name, phone, email)
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO order_status_history (id, order_id, to_status, actor_user_id, created_at)
		VALUES ($1, $2, 'awaiting_payment', $3, now())
	`, uuid.New(), orderID, userID)
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO auction_order_links (id, auction_id, lot_id, order_id, winner_user_id, amount_cents, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, 'awaiting_payment', now(), now())
	`, uuid.New(), auctionID, lotID, orderID, userID, amountCents)
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx, `
		UPDATE auction_lots SET order_id = $1, updated_at = now() WHERE id = $2
	`, orderID, lotID)
	
	return err
}

func (r *Repository) GetAuctionOrderLinkByOrderIDTx(ctx context.Context, tx pgx.Tx, orderID uuid.UUID) (*AuctionOrderLink, error) {
	var link AuctionOrderLink
	err := tx.QueryRow(ctx, `SELECT id, auction_id, lot_id, order_id, winner_user_id, amount_cents, status, created_at, updated_at FROM auction_order_links WHERE order_id = $1 LIMIT 1`, orderID).
		Scan(&link.ID, &link.AuctionID, &link.LotID, &link.OrderID, &link.WinnerUserID, &link.AmountCents, &link.Status, &link.CreatedAt, &link.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &link, nil
}

func (r *Repository) MarkAuctionOrderPaidTx(ctx context.Context, tx pgx.Tx, orderID uuid.UUID) error {
	_, err := tx.Exec(ctx, `UPDATE orders SET status = 'paid', updated_at = now() WHERE id = $1`, orderID)
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO order_status_history (id, order_id, from_status, to_status, actor_user_id, created_at)
		VALUES ($1, $2, 'awaiting_payment', 'paid', NULL, now())
	`, uuid.New(), orderID)
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx, `UPDATE auction_order_links SET status = 'paid', updated_at = now() WHERE order_id = $1`, orderID)
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx, `
		UPDATE auction_lots SET status = 'paid', updated_at = now() 
		WHERE id = (SELECT lot_id FROM auction_order_links WHERE order_id = $1)
	`, orderID)
	
	return err
}

// MoveLotToDirectSaleTx converts an auction lot to a ZAMK Platform direct sale product.
func (r *Repository) MoveLotToDirectSaleTx(ctx context.Context, tx pgx.Tx, lotID uuid.UUID, platformSellerID uuid.UUID) (uuid.UUID, error) {
	var l AuctionLot
	err := tx.QueryRow(ctx, "SELECT id, title, description, image_url, start_price_cents, direct_sale_price_cents FROM auction_lots WHERE id = $1 FOR UPDATE", lotID).
		Scan(&l.ID, &l.Title, &l.Description, &l.ImageURL, &l.StartPriceCents, &l.DirectSalePriceCents)
	if err != nil {
		return uuid.Nil, err
	}

	price := l.StartPriceCents
	if l.DirectSalePriceCents != nil {
		price = *l.DirectSalePriceCents
	}

	productID := uuid.New()
	slug := fmt.Sprintf("auction-lot-%s", productID.String()[:8])
	
	_, err = tx.Exec(ctx, `
		INSERT INTO products (
			id, seller_id, category_id, brand_id, title, slug, description, status, source,
			price_cents, currency, main_image_url, created_at, updated_at, published_at
		) VALUES (
			$1, $2, NULL, NULL, $3, $4, $5, 'published', 'auction_direct_sale',
			$6, 'RUB', $7, now(), now(), now()
		)
	`, productID, platformSellerID, l.Title, slug, l.Description, price, l.ImageURL)
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to create product: %w", err)
	}

	variantID := uuid.New()
	_, err = tx.Exec(ctx, `
		INSERT INTO product_variants (id, product_id, sku, price_cents, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, true, now(), now())
	`, variantID, productID, slug, price)
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to create variant: %w", err)
	}

	// Inventory
	inventoryID := uuid.New()
	_, err = tx.Exec(ctx, `
		INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, now(), now())
	`, inventoryID, productID, variantID, platformSellerID, 1, 0)
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to create inventory: %w", err)
	}

	// Images
	irows, err := tx.Query(ctx, "SELECT image_url, sort_order FROM auction_lot_images WHERE lot_id = $1 ORDER BY sort_order ASC", lotID)
	if err == nil {
		var imgs []struct {
			URL   *string
			Order int
		}
		for irows.Next() {
			var u *string
			var o int
			if irows.Scan(&u, &o) == nil {
				imgs = append(imgs, struct {
					URL   *string
					Order int
				}{URL: u, Order: o})
			}
		}
		irows.Close()
		
		for _, img := range imgs {
			_, _ = tx.Exec(ctx, `
				INSERT INTO product_images (id, product_id, image_url, sort_order, created_at)
				VALUES ($1, $2, $3, $4, now())
			`, uuid.New(), productID, img.URL, img.Order)
		}
	} else {
		irows.Close()
	}

	// Link back
	_, err = tx.Exec(ctx, `
		UPDATE auction_lots SET status = 'moved_to_direct_sale', direct_sale_product_id = $1, updated_at = now() WHERE id = $2
	`, productID, lotID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to update lot status: %w", err)
	}

	return productID, nil
}

type ExpiredLotResult struct {
	LotID              uuid.UUID
	AuctionID          uuid.UUID
	WinnerUserID       uuid.UUID
	StatusTransitioned bool
}

func (r *Repository) ExpireUnpaidAuctionLotsTx(ctx context.Context, tx pgx.Tx, now time.Time, limit int) ([]ExpiredLotResult, error) {
	// Select lots with FOR UPDATE SKIP LOCKED
	rows, err := tx.Query(ctx, `
		SELECT id, auction_id, current_winner_user_id, order_id
		FROM auction_lots
		WHERE status = $1
		  AND payment_deadline_at IS NOT NULL
		  AND payment_deadline_at < $2
		LIMIT $3
		FOR UPDATE SKIP LOCKED
	`, LotStatusWonPendingPayment, now, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query expired lots: %w", err)
	}
	defer rows.Close()

	var toProcess []struct {
		lotID     uuid.UUID
		auctionID uuid.UUID
		winnerID  uuid.UUID
		orderID   *uuid.UUID
	}

	for rows.Next() {
		var lotID, auctionID, winnerID uuid.UUID
		var orderID *uuid.UUID
		if err := rows.Scan(&lotID, &auctionID, &winnerID, &orderID); err != nil {
			return nil, err
		}
		toProcess = append(toProcess, struct {
			lotID     uuid.UUID
			auctionID uuid.UUID
			winnerID  uuid.UUID
			orderID   *uuid.UUID
		}{lotID, auctionID, winnerID, orderID})
	}
	rows.Close()

	var results []ExpiredLotResult

	for _, p := range toProcess {
		var orderPaid bool
		if p.orderID != nil {
			var orderStatus string
			err := tx.QueryRow(ctx, `SELECT status FROM orders WHERE id = $1`, *p.orderID).Scan(&orderStatus)
			if err == nil && orderStatus == "paid" {
				orderPaid = true
			}
		}

		if orderPaid {
			_, err = tx.Exec(ctx, `UPDATE auction_lots SET status = $1, updated_at = now() WHERE id = $2`, LotStatusPaid, p.lotID)
			if err != nil {
				return nil, err
			}
			results = append(results, ExpiredLotResult{
				LotID:              p.lotID,
				AuctionID:          p.auctionID,
				WinnerUserID:       p.winnerID,
				StatusTransitioned: false,
			})
			continue
		}

		_, err = tx.Exec(ctx, `UPDATE auction_lots SET status = $1, updated_at = now() WHERE id = $2`, LotStatusUnpaidManualReview, p.lotID)
		if err != nil {
			return nil, err
		}

		if p.orderID != nil {
			_, err = tx.Exec(ctx, `UPDATE auction_order_links SET status = 'expired', updated_at = now() WHERE lot_id = $1`, p.lotID)
			if err != nil {
				return nil, err
			}
		}

		b, _ := json.Marshal(map[string]interface{}{"reason": "payment_deadline_expired"})
		_, err = tx.Exec(ctx, `
			INSERT INTO auction_logs (id, auction_id, lot_id, actor_user_id, action, metadata, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, now())
		`, uuid.New(), p.auctionID, p.lotID, nil, "auction_payment_deadline_expired", b)
		if err != nil {
			return nil, err
		}

		results = append(results, ExpiredLotResult{
			LotID:              p.lotID,
			AuctionID:          p.auctionID,
			WinnerUserID:       p.winnerID,
			StatusTransitioned: true,
		})
	}

	return results, nil
}
