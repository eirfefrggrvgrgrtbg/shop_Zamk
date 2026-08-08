package supplies

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (r *Repository) GetSupplyByQRToken(ctx context.Context, token string) (*Supply, error) {
	query := `
		SELECT id, supply_number, seller_id, status, handoff_method, carrier_name, tracking_number, expected_arrival_date,
			qr_token, created_at, shipped_at, arrived_at, receiving_started_at, completed_at, updated_at
		FROM seller_supplies
		WHERE qr_token = $1
	`
	var s Supply
	err := r.db.QueryRow(ctx, query, token).Scan(
		&s.ID, &s.SupplyNumber, &s.SellerID, &s.Status, &s.HandoffMethod, &s.CarrierName, &s.TrackingNumber, &s.ExpectedArrivalDate,
		&s.QRToken, &s.CreatedAt, &s.ShippedAt, &s.ArrivedAt, &s.ReceivingStartedAt, &s.CompletedAt, &s.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrSupplyNotFound
		}
		return nil, fmt.Errorf("failed to get supply by qr: %w", err)
	}
	return &s, nil
}

func (r *Repository) StartReceivingSession(ctx context.Context, session *ReceivingSession, supplyItems []SupplyItem) error {
	query := `
		INSERT INTO supply_receiving_sessions (
			id, supply_id, status, version, started_at, started_by_staff_id, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	_, err := r.db.Exec(ctx, query,
		session.ID, session.SupplyID, session.Status, session.Version, session.StartedAt, session.StartedByStaffID, session.CreatedAt, session.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to insert session: %w", err)
	}

	for _, item := range session.Items {
		queryItem := `
			INSERT INTO supply_receiving_items (
				id, session_id, supply_item_id, variant_id, sku, barcode, product_title,
				expected_quantity, scanned_quantity, damaged_quantity, unexpected_quantity, created_at, updated_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		`
		_, err := r.db.Exec(ctx, queryItem,
			item.ID, item.SessionID, item.SupplyItemID, item.VariantID, item.SKU, item.Barcode, item.ProductTitle,
			item.ExpectedQuantity, item.ScannedQuantity, item.DamagedQuantity, item.UnexpectedQuantity, item.CreatedAt, item.UpdatedAt,
		)
		if err != nil {
			return fmt.Errorf("failed to insert session item: %w", err)
		}
	}

	return nil
}

func (r *Repository) GetActiveSession(ctx context.Context, supplyID uuid.UUID) (*ReceivingSession, error) {
	query := `
		SELECT id, supply_id, status, version, started_at, started_by_staff_id, completed_at, created_at, updated_at
		FROM supply_receiving_sessions
		WHERE supply_id = $1 AND status = 'active'
	`
	var s ReceivingSession
	err := r.db.QueryRow(ctx, query, supplyID).Scan(
		&s.ID, &s.SupplyID, &s.Status, &s.Version, &s.StartedAt, &s.StartedByStaffID, &s.CompletedAt, &s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrSessionNotFound
		}
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	itemsQuery := `
		SELECT id, session_id, supply_item_id, variant_id, sku, barcode, product_title,
			expected_quantity, scanned_quantity, damaged_quantity, unexpected_quantity, created_at, updated_at
		FROM supply_receiving_items
		WHERE session_id = $1
	`
	rows, err := r.db.Query(ctx, itemsQuery, s.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session items: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var i ReceivingItem
		err := rows.Scan(
			&i.ID, &i.SessionID, &i.SupplyItemID, &i.VariantID, &i.SKU, &i.Barcode, &i.ProductTitle,
			&i.ExpectedQuantity, &i.ScannedQuantity, &i.DamagedQuantity, &i.UnexpectedQuantity, &i.CreatedAt, &i.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan session item: %w", err)
		}
		s.Items = append(s.Items, i)
	}

	return &s, nil
}

func (r *Repository) GetSessionByID(ctx context.Context, sessionID uuid.UUID) (*ReceivingSession, error) {
	query := `
		SELECT id, supply_id, status, version, started_at, started_by_staff_id, completed_at, created_at, updated_at
		FROM supply_receiving_sessions
		WHERE id = $1
	`
	var s ReceivingSession
	err := r.db.QueryRow(ctx, query, sessionID).Scan(
		&s.ID, &s.SupplyID, &s.Status, &s.Version, &s.StartedAt, &s.StartedByStaffID, &s.CompletedAt, &s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrSessionNotFound
		}
		return nil, fmt.Errorf("failed to get session by id: %w", err)
	}

	itemsQuery := `
		SELECT id, session_id, supply_item_id, variant_id, sku, barcode, product_title,
			expected_quantity, scanned_quantity, damaged_quantity, unexpected_quantity, created_at, updated_at
		FROM supply_receiving_items
		WHERE session_id = $1
	`
	rows, err := r.db.Query(ctx, itemsQuery, s.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session items: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var i ReceivingItem
		err := rows.Scan(
			&i.ID, &i.SessionID, &i.SupplyItemID, &i.VariantID, &i.SKU, &i.Barcode, &i.ProductTitle,
			&i.ExpectedQuantity, &i.ScannedQuantity, &i.DamagedQuantity, &i.UnexpectedQuantity, &i.CreatedAt, &i.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan session item: %w", err)
		}
		s.Items = append(s.Items, i)
	}

	return &s, nil
}

func (r *Repository) LockSessionForUpdate(ctx context.Context, sessionID uuid.UUID) error {
	query := `SELECT id FROM supply_receiving_sessions WHERE id = $1 FOR UPDATE`
	var id uuid.UUID
	err := r.db.QueryRow(ctx, query, sessionID).Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrSessionNotFound
		}
		return fmt.Errorf("failed to lock session: %w", err)
	}
	return nil
}

func (r *Repository) LockSupplyForUpdate(ctx context.Context, supplyID uuid.UUID) error {
	query := `SELECT id FROM seller_supplies WHERE id = $1 FOR UPDATE`
	var id uuid.UUID
	err := r.db.QueryRow(ctx, query, supplyID).Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrSupplyNotFound
		}
		return fmt.Errorf("failed to lock supply: %w", err)
	}
	return nil
}

func (r *Repository) AddReceivingScan(ctx context.Context, sessionID uuid.UUID, itemID uuid.UUID, staffID *uuid.UUID, quantity int, isDamage bool) error {
	now := time.Now().UTC()
	scanQuery := `
		INSERT INTO supply_receiving_scans (id, session_id, supply_receiving_item_id, staff_id, quantity, is_damage, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err := r.db.Exec(ctx, scanQuery, uuid.New(), sessionID, itemID, staffID, quantity, isDamage, now)
	if err != nil {
		return fmt.Errorf("failed to insert scan: %w", err)
	}

	var updateQuery string
	if isDamage {
		updateQuery = `UPDATE supply_receiving_items SET damaged_quantity = damaged_quantity + $1, updated_at = $2 WHERE id = $3`
	} else {
		updateQuery = `UPDATE supply_receiving_items SET scanned_quantity = scanned_quantity + $1, updated_at = $2 WHERE id = $3`
	}

	_, err = r.db.Exec(ctx, updateQuery, quantity, now, itemID)
	if err != nil {
		return fmt.Errorf("failed to update item: %w", err)
	}

	return nil
}

func (r *Repository) CompleteReceivingSession(ctx context.Context, sessionID uuid.UUID) error {
	query := `UPDATE supply_receiving_sessions SET status = 'completed', completed_at = now(), updated_at = now() WHERE id = $1 AND status = 'active'`
	res, err := r.db.Exec(ctx, query, sessionID)
	if err != nil {
		return fmt.Errorf("failed to complete session: %w", err)
	}
	if res.RowsAffected() == 0 {
		return ErrSessionNotFound
	}
	return nil
}

func (r *Repository) FinalizeSupplyItem(ctx context.Context, itemID uuid.UUID, accepted, damaged, missing, extra int) error {
	query := `
		UPDATE seller_supply_items
		SET accepted_quantity = $1, damaged_quantity = $2, missing_quantity = $3, extra_quantity = $4, updated_at = now()
		WHERE id = $5
	`
	_, err := r.db.Exec(ctx, query, accepted, damaged, missing, extra, itemID)
	if err != nil {
		return fmt.Errorf("failed to finalize supply item: %w", err)
	}
	return nil
}



func (r *Repository) UpdateInventoryStock(ctx context.Context, variantID uuid.UUID, quantityDelta int, refID uuid.UUID) error {
	// First get the inventory item ID, product_id, and seller_id
	queryGet := `SELECT id, product_id, seller_id FROM inventory_items WHERE product_variant_id = $1`
	var invID, prodID, sellerID uuid.UUID
	err := r.db.QueryRow(ctx, queryGet, variantID).Scan(&invID, &prodID, &sellerID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			queryProd := `SELECT p.id, p.seller_id FROM product_variants v JOIN products p ON v.product_id = p.id WHERE v.id = $1`
			if err := r.db.QueryRow(ctx, queryProd, variantID).Scan(&prodID, &sellerID); err != nil {
				return fmt.Errorf("failed to find product for variant: %w", err)
			}
			// Create it if it doesn't exist
			invID = uuid.New()
			insertQuery := `INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock, created_at, updated_at) VALUES ($1, $2, $3, $4, 0, 0, now(), now())`
			_, err = r.db.Exec(ctx, insertQuery, invID, prodID, variantID, sellerID)
			if err != nil {
				return fmt.Errorf("failed to create inventory item: %w", err)
			}
		} else {
			return fmt.Errorf("failed to check inventory item: %w", err)
		}
	}

	queryUpdate := `
		UPDATE inventory_items 
		SET total_stock = total_stock + $1, updated_at = now()
		WHERE id = $2
	`
	_, err = r.db.Exec(ctx, queryUpdate, quantityDelta, invID)
	if err != nil {
		return fmt.Errorf("failed to update stock: %w", err)
	}

	movID := uuid.New()
	movQuery := `INSERT INTO stock_movements (id, inventory_item_id, product_id, product_variant_id, seller_id, type, quantity, reference_type, reference_id, created_at) VALUES ($1, $2, $3, $4, $5, 'receipt', $6, 'supply', $7, now())`
	_, err = r.db.Exec(ctx, movQuery, movID, invID, prodID, variantID, sellerID, quantityDelta, refID)
	if err != nil {
		return fmt.Errorf("failed to insert stock movement: %w", err)
	}

	return nil
}
