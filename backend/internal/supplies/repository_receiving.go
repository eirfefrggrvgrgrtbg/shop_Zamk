package supplies

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func (r *Repository) GetSupplyByQRToken(ctx context.Context, token string) (*Supply, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, ErrSupplyNotFound
	}
	query := `
		SELECT s.id, s.supply_number, s.seller_id, s.status, s.handoff_method, s.carrier_name, s.tracking_number, s.expected_arrival_date,
			s.qr_token, s.created_at, s.shipped_at, s.arrived_at, s.receiving_started_at, s.completed_at, s.updated_at,
			COALESCE(sel.brand_name, '') as seller_name
		FROM seller_supplies s
		LEFT JOIN sellers sel ON sel.id = s.seller_id
		LEFT JOIN seller_supply_boxes b ON b.supply_id = s.id
		WHERE s.qr_token = $1 OR UPPER(s.supply_number) = UPPER($1) OR b.qr_token = $1 OR UPPER(b.box_number) = UPPER($1)
		LIMIT 1
	`
	var s Supply
	err := r.db.QueryRow(ctx, query, token).Scan(
		&s.ID, &s.SupplyNumber, &s.SellerID, &s.Status, &s.HandoffMethod, &s.CarrierName, &s.TrackingNumber, &s.ExpectedArrivalDate,
		&s.QRToken, &s.CreatedAt, &s.ShippedAt, &s.ArrivedAt, &s.ReceivingStartedAt, &s.CompletedAt, &s.UpdatedAt,
		&s.SellerName,
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

func (r *Repository) GetSupplyReceivingMode(ctx context.Context, supplyID uuid.UUID) (string, error) {
	query := `
		SELECT
			COALESCE((SELECT SUM(expected_quantity) FROM seller_supply_items WHERE supply_id = $1), 0) AS expected,
			(SELECT COUNT(*) FROM inventory_units WHERE origin_supply_id = $1) AS actual
	`
	var expected, actual int
	err := r.db.QueryRow(ctx, query, supplyID).Scan(&expected, &actual)
	if err != nil {
		return "", fmt.Errorf("failed to get supply receiving mode: %w", err)
	}
	if actual == 0 {
		return "legacy", nil
	}
	if actual != expected {
		return "", ErrSupplyUnitIdentityMismatch
	}
	return "serialized", nil
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

	mode, err := r.GetSupplyReceivingMode(ctx, s.SupplyID)
	if err != nil {
		return nil, err
	}
	s.ReceivingMode = mode

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

	mode, err := r.GetSupplyReceivingMode(ctx, s.SupplyID)
	if err != nil {
		return nil, err
	}
	s.ReceivingMode = mode

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

func (r *Repository) FinalizeSupplyItem(ctx context.Context, itemID uuid.UUID, acceptedDelta, damagedDelta int) error {
	query := `
		UPDATE seller_supply_items
		SET 
			accepted_quantity = COALESCE(accepted_quantity, 0) + $1, 
			damaged_quantity = COALESCE(damaged_quantity, 0) + $2, 
			missing_quantity = GREATEST(0, expected_quantity - (COALESCE(accepted_quantity, 0) + $1) - (COALESCE(damaged_quantity, 0) + $2)), 
			extra_quantity = GREATEST(0, (COALESCE(accepted_quantity, 0) + $1) + (COALESCE(damaged_quantity, 0) + $2) - expected_quantity),
			updated_at = now()
		WHERE id = $3
	`
	_, err := r.db.Exec(ctx, query, acceptedDelta, damagedDelta, itemID)
	if err != nil {
		return fmt.Errorf("failed to finalize supply item: %w", err)
	}
	return nil
}

func (r *Repository) CheckSupplyDiscrepancies(ctx context.Context, supplyID uuid.UUID) (bool, error) {
	var count int
	err := r.db.QueryRow(ctx, "SELECT COUNT(*) FROM seller_supply_items WHERE supply_id = $1 AND (missing_quantity > 0 OR damaged_quantity > 0 OR extra_quantity > 0)", supplyID).Scan(&count)
	return count > 0, err
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

func (r *Repository) GetInventoryUnitByCode(ctx context.Context, unitCode string) (*InventoryUnit, error) {
	query := `
		SELECT id, unit_code, product_variant_id, origin_supply_id, origin_supply_item_id, origin_box_id, unit_index, status
		FROM inventory_units
		WHERE unit_code = $1
	`
	var u InventoryUnit
	err := r.db.QueryRow(ctx, query, unitCode).Scan(
		&u.ID, &u.UnitCode, &u.ProductVariantID, &u.OriginSupplyID, &u.OriginSupplyItemID, &u.OriginBoxID, &u.UnitIndex, &u.Status,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUnitNotFound
		}
		return nil, err
	}
	return &u, nil
}

func (r *Repository) GetEnrichedInventoryUnitByCode(ctx context.Context, unitCode string) (*EnrichedInventoryUnit, error) {
	query := `
		SELECT
			u.id,
			u.unit_code,
			u.product_variant_id,
			u.origin_supply_id,
			u.origin_supply_item_id,
			u.origin_box_id,
			u.unit_index,
			u.status,
			p.title AS product_title,
			COALESCE(c.name_ru, v.color) AS color_name,
			COALESCE(sv.value, v.size) AS size_name,
			COALESCE(v.seller_sku, v.sku) AS seller_sku,
			v.barcode AS variant_barcode
		FROM inventory_units u
		JOIN product_variants v ON v.id = u.product_variant_id
		JOIN products p ON p.id = v.product_id
		LEFT JOIN colors c ON c.id = v.color_id
		LEFT JOIN size_values sv ON sv.id = v.size_value_id
		WHERE u.unit_code = $1 OR UPPER(u.unit_code) = UPPER($1)
	`
	var u EnrichedInventoryUnit
	err := r.db.QueryRow(ctx, query, unitCode).Scan(
		&u.ID,
		&u.UnitCode,
		&u.ProductVariantID,
		&u.OriginSupplyID,
		&u.OriginSupplyItemID,
		&u.OriginBoxID,
		&u.UnitIndex,
		&u.Status,
		&u.ProductTitle,
		&u.ColorName,
		&u.SizeName,
		&u.SellerSKU,
		&u.VariantBarcode,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUnitNotFound
		}
		return nil, err
	}
	return &u, nil
}

func (r *Repository) AddSerializedReceivingScan(ctx context.Context, sessionID uuid.UUID, itemID uuid.UUID, unitID uuid.UUID, staffID *uuid.UUID, isDamage bool, condition string) (uuid.UUID, error) {
	now := time.Now().UTC()
	scanID := uuid.New()

	scanQuery := `
		INSERT INTO supply_receiving_scans (id, session_id, supply_receiving_item_id, staff_id, quantity, is_damage, created_at, inventory_unit_id, condition)
		VALUES ($1, $2, $3, $4, 1, $5, $6, $7, $8)
	`
	_, err := r.db.Exec(ctx, scanQuery, scanID, sessionID, itemID, staffID, isDamage, now, unitID, condition)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.ConstraintName == "idx_receiving_scans_unit_active" {
			return uuid.Nil, ErrUnitAlreadyScanned
		}
		return uuid.Nil, fmt.Errorf("failed to insert serialized scan: %w", err)
	}

	var updateQuery string
	if isDamage {
		updateQuery = `UPDATE supply_receiving_items SET damaged_quantity = damaged_quantity + 1, updated_at = $1 WHERE id = $2`
	} else {
		updateQuery = `UPDATE supply_receiving_items SET scanned_quantity = scanned_quantity + 1, updated_at = $1 WHERE id = $2`
	}

	_, err = r.db.Exec(ctx, updateQuery, now, itemID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to update item: %w", err)
	}

	return scanID, nil
}

func (r *Repository) GetReceivingSessionTotals(ctx context.Context, sessionID uuid.UUID) (expected, scanned, ok, damaged int, err error) {
	query := `
		SELECT
			COALESCE(SUM(expected_quantity), 0),
			COALESCE(SUM(scanned_quantity), 0),
			COALESCE(SUM(damaged_quantity), 0)
		FROM supply_receiving_items
		WHERE session_id = $1
	`
	err = r.db.QueryRow(ctx, query, sessionID).Scan(&expected, &ok, &damaged)
	if err != nil {
		return 0, 0, 0, 0, err
	}
	scanned = ok + damaged
	return expected, scanned, ok, damaged, nil
}

func (r *Repository) ListRecentSerializedScans(ctx context.Context, sessionID uuid.UUID, limit int) ([]SerializedRecentScanDTO, error) {
	if limit <= 0 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}
	query := `
		SELECT
			s.id AS scan_id,
			u.unit_code,
			COALESCE(s.condition, CASE WHEN s.is_damage THEN 'damaged' ELSE 'ok' END) AS condition,
			s.created_at AS scanned_at,
			s.voided_at,
			p.title AS product_title,
			COALESCE(c.name_ru, v.color) AS color_name,
			COALESCE(sv.value, v.size) AS size_name,
			COALESCE(v.seller_sku, v.sku) AS seller_sku,
			v.barcode AS variant_barcode
		FROM supply_receiving_scans s
		JOIN inventory_units u ON u.id = s.inventory_unit_id
		JOIN product_variants v ON v.id = u.product_variant_id
		JOIN products p ON p.id = v.product_id
		LEFT JOIN colors c ON c.id = v.color_id
		LEFT JOIN size_values sv ON sv.id = v.size_value_id
		WHERE s.session_id = $1 AND s.inventory_unit_id IS NOT NULL
		ORDER BY s.created_at DESC
		LIMIT $2
	`
	rows, err := r.db.Query(ctx, query, sessionID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query recent scans: %w", err)
	}
	defer rows.Close()

	var scans []SerializedRecentScanDTO
	for rows.Next() {
		var item SerializedRecentScanDTO
		err := rows.Scan(
			&item.ScanID,
			&item.UnitCode,
			&item.Condition,
			&item.ScannedAt,
			&item.VoidedAt,
			&item.ProductTitle,
			&item.ColorName,
			&item.SizeName,
			&item.SellerSKU,
			&item.VariantBarcode,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan recent scan: %w", err)
		}
		scans = append(scans, item)
	}
	if scans == nil {
		scans = []SerializedRecentScanDTO{}
	}
	return scans, nil
}

type ReceivingScanRecord struct {
	ID                    uuid.UUID
	SessionID             uuid.UUID
	SupplyReceivingItemID uuid.UUID
	StaffID               *uuid.UUID
	Quantity              int
	IsDamage              bool
	CreatedAt             time.Time
	InventoryUnitID       *uuid.UUID
	Condition             *string
	VoidedAt              *time.Time
	VoidedBy              *uuid.UUID
}

func (r *Repository) LockScanForUpdate(ctx context.Context, scanID uuid.UUID) (*ReceivingScanRecord, error) {
	query := `
		SELECT id, session_id, supply_receiving_item_id, staff_id, quantity, is_damage, created_at, inventory_unit_id, condition, voided_at, voided_by
		FROM supply_receiving_scans
		WHERE id = $1
		FOR UPDATE
	`
	var rec ReceivingScanRecord
	err := r.db.QueryRow(ctx, query, scanID).Scan(
		&rec.ID,
		&rec.SessionID,
		&rec.SupplyReceivingItemID,
		&rec.StaffID,
		&rec.Quantity,
		&rec.IsDamage,
		&rec.CreatedAt,
		&rec.InventoryUnitID,
		&rec.Condition,
		&rec.VoidedAt,
		&rec.VoidedBy,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrScanNotFound
		}
		return nil, fmt.Errorf("failed to lock scan: %w", err)
	}
	return &rec, nil
}

func (r *Repository) VoidSerializedScan(ctx context.Context, scanID uuid.UUID, staffID uuid.UUID, itemID uuid.UUID, isDamage bool) error {
	now := time.Now().UTC()
	query := `
		UPDATE supply_receiving_scans
		SET voided_at = $1, voided_by = $2
		WHERE id = $3 AND voided_at IS NULL
	`
	res, err := r.db.Exec(ctx, query, now, staffID, scanID)
	if err != nil {
		return fmt.Errorf("failed to void scan: %w", err)
	}
	if res.RowsAffected() == 0 {
		return ErrScanAlreadyVoided
	}

	var updateItemQuery string
	if isDamage {
		updateItemQuery = `UPDATE supply_receiving_items SET damaged_quantity = GREATEST(0, damaged_quantity - 1), updated_at = $1 WHERE id = $2`
	} else {
		updateItemQuery = `UPDATE supply_receiving_items SET scanned_quantity = GREATEST(0, scanned_quantity - 1), updated_at = $1 WHERE id = $2`
	}

	_, err = r.db.Exec(ctx, updateItemQuery, now, itemID)
	if err != nil {
		return fmt.Errorf("failed to update receiving item after void: %w", err)
	}

	return nil
}

func (r *Repository) LockUnitsForSupply(ctx context.Context, supplyID uuid.UUID) error {
	query := `SELECT id FROM inventory_units WHERE origin_supply_id = $1 FOR UPDATE`
	rows, err := r.db.Query(ctx, query, supplyID)
	if err != nil {
		return fmt.Errorf("failed to lock inventory units: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
	}
	return rows.Err()
}

func (r *Repository) FinalizeSerializedUnits(ctx context.Context, sessionID uuid.UUID) error {
	queryOK := `
		UPDATE inventory_units
		SET status = 'warehouse', receiving_session_id = $1, updated_at = now()
		WHERE id IN (
			SELECT inventory_unit_id
			FROM supply_receiving_scans
			WHERE session_id = $1
			  AND voided_at IS NULL
			  AND inventory_unit_id IS NOT NULL
			  AND (condition = 'ok' OR (condition IS NULL AND is_damage = false))
		)
	`
	_, err := r.db.Exec(ctx, queryOK, sessionID)
	if err != nil {
		return fmt.Errorf("failed to finalize warehouse units: %w", err)
	}

	queryDamaged := `
		UPDATE inventory_units
		SET status = 'damaged', receiving_session_id = $1, updated_at = now()
		WHERE id IN (
			SELECT inventory_unit_id
			FROM supply_receiving_scans
			WHERE session_id = $1
			  AND voided_at IS NULL
			  AND inventory_unit_id IS NOT NULL
			  AND (condition = 'damaged' OR (condition IS NULL AND is_damage = true))
		)
	`
	_, err = r.db.Exec(ctx, queryDamaged, sessionID)
	if err != nil {
		return fmt.Errorf("failed to finalize damaged units: %w", err)
	}

	return nil
}

func (r *Repository) CountRemainingExpectedUnitsForItem(ctx context.Context, itemID uuid.UUID) (int, error) {
	var count int
	err := r.db.QueryRow(ctx, "SELECT COUNT(*) FROM inventory_units WHERE origin_supply_item_id = $1 AND status = 'expected'", itemID).Scan(&count)
	return count, err
}
