package inventory

import (
	"context"
	"errors"
	"fmt"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/common"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"sort"
	"strings"
	"time"
)

type Repository struct {
	db postgres.DBTX
}

func NewRepository(db postgres.DBTX) *Repository {
	return &Repository{db: db}
}

func (r *Repository) WithTx(tx pgx.Tx) *Repository {
	return &Repository{db: tx}
}

func (r *Repository) GetItemForUpdateByVariant(ctx context.Context, variantID uuid.UUID) (*Item, error) {
	query := `
		SELECT id, product_id, product_variant_id, seller_id, total_stock, reserved_stock, created_at, updated_at
		FROM inventory_items
		WHERE product_variant_id = $1
		FOR UPDATE
	`
	var i Item
	err := r.db.QueryRow(ctx, query, variantID).Scan(
		&i.ID, &i.ProductID, &i.ProductVariantID, &i.SellerID, &i.TotalStock, &i.ReservedStock, &i.CreatedAt, &i.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrInventoryItemNotFound
		}
		return nil, fmt.Errorf("failed to lock inventory item: %w", err)
	}
	i.ComputeAvailable()
	return &i, nil
}

func (r *Repository) GetItemByID(ctx context.Context, itemID uuid.UUID) (*Item, error) {
	query := `
		SELECT id, product_id, product_variant_id, seller_id, total_stock, reserved_stock, created_at, updated_at
		FROM inventory_items
		WHERE id = $1
	`
	var i Item
	err := r.db.QueryRow(ctx, query, itemID).Scan(
		&i.ID, &i.ProductID, &i.ProductVariantID, &i.SellerID, &i.TotalStock, &i.ReservedStock, &i.CreatedAt, &i.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrInventoryItemNotFound
		}
		return nil, fmt.Errorf("failed to get inventory item: %w", err)
	}
	i.ComputeAvailable()
	return &i, nil
}

func (r *Repository) GetItemByVariantID(ctx context.Context, variantID uuid.UUID) (*Item, error) {
	query := `
		SELECT id, product_id, product_variant_id, seller_id, total_stock, reserved_stock, created_at, updated_at
		FROM inventory_items
		WHERE product_variant_id = $1
	`
	var i Item
	err := r.db.QueryRow(ctx, query, variantID).Scan(
		&i.ID, &i.ProductID, &i.ProductVariantID, &i.SellerID, &i.TotalStock, &i.ReservedStock, &i.CreatedAt, &i.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrInventoryItemNotFound
		}
		return nil, fmt.Errorf("failed to get inventory item: %w", err)
	}
	i.ComputeAvailable()
	return &i, nil
}

func (r *Repository) CreateItem(ctx context.Context, i *Item) error {
	query := `
		INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	_, err := r.db.Exec(ctx, query,
		i.ID, i.ProductID, i.ProductVariantID, i.SellerID, i.TotalStock, i.ReservedStock, i.CreatedAt, i.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create inventory item: %w", err)
	}
	return nil
}

func (r *Repository) UpdateItemStock(ctx context.Context, i *Item) error {
	query := `
		UPDATE inventory_items
		SET total_stock = $1, reserved_stock = $2, updated_at = now()
		WHERE id = $3
	`
	res, err := r.db.Exec(ctx, query, i.TotalStock, i.ReservedStock, i.ID)
	if err != nil {
		return fmt.Errorf("failed to update inventory item: %w", err)
	}
	if res.RowsAffected() == 0 {
		return ErrInventoryItemNotFound
	}
	return nil
}

func (r *Repository) RecordMovement(ctx context.Context, m *StockMovement) error {
	query := `
		INSERT INTO stock_movements (id, inventory_item_id, product_id, product_variant_id, seller_id, type, quantity, reason, actor_user_id, reference_type, reference_id, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`
	_, err := r.db.Exec(ctx, query,
		m.ID, m.InventoryItemID, m.ProductID, m.ProductVariantID, m.SellerID, m.Type, m.Quantity, m.Reason, m.ActorUserID, m.ReferenceType, m.ReferenceID, m.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to record stock movement: %w", err)
	}
	return nil
}

func (r *Repository) CreateReservation(ctx context.Context, res *Reservation) error {
	query := `
		INSERT INTO reservations (id, inventory_item_id, product_id, product_variant_id, user_id, quantity, status, expires_at, order_id, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`
	_, err := r.db.Exec(ctx, query,
		res.ID, res.InventoryItemID, res.ProductID, res.ProductVariantID, res.UserID, res.Quantity, res.Status, res.ExpiresAt, res.OrderID, res.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create reservation: %w", err)
	}
	return nil
}

func (r *Repository) UpdateReservationStatus(ctx context.Context, res *Reservation) error {
	query := `
		UPDATE reservations
		SET status = $1, released_at = $2, order_id = $3
		WHERE id = $4
	`
	result, err := r.db.Exec(ctx, query, res.Status, res.ReleasedAt, res.OrderID, res.ID)
	if err != nil {
		return fmt.Errorf("failed to update reservation: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrReservationNotFound
	}
	return nil
}

func (r *Repository) GetReservationByIDForUpdate(ctx context.Context, id uuid.UUID) (*Reservation, error) {
	query := `
		SELECT id, inventory_item_id, product_id, product_variant_id, user_id, quantity, status, expires_at, order_id, created_at, released_at
		FROM reservations
		WHERE id = $1
		FOR UPDATE
	`
	var res Reservation
	err := r.db.QueryRow(ctx, query, id).Scan(
		&res.ID, &res.InventoryItemID, &res.ProductID, &res.ProductVariantID, &res.UserID, &res.Quantity, &res.Status, &res.ExpiresAt, &res.OrderID, &res.CreatedAt, &res.ReleasedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrReservationNotFound
		}
		return nil, fmt.Errorf("failed to get reservation: %w", err)
	}
	return &res, nil
}

// Listing operations

func (r *Repository) ListInventory(ctx context.Context, limit, offset int) ([]Item, error) {
	query := `
		SELECT id, product_id, product_variant_id, seller_id, total_stock, reserved_stock, created_at, updated_at
		FROM inventory_items
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`
	return r.listInventoryItems(ctx, query, limit, offset)
}

func (r *Repository) ListInventoryBySeller(ctx context.Context, sellerID uuid.UUID, limit, offset int) ([]Item, error) {
	query := `
		SELECT id, product_id, product_variant_id, seller_id, total_stock, reserved_stock, created_at, updated_at
		FROM inventory_items
		WHERE seller_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`
	return r.listInventoryItems(ctx, query, sellerID, limit, offset)
}

func (r *Repository) ListSellerInventoryRich(ctx context.Context, sellerID uuid.UUID, limit, offset int) ([]SellerInventoryItem, int, error) {
	baseQuery := `
		FROM inventory_items i
		JOIN products p ON i.product_id = p.id
		JOIN product_variants pv ON i.product_variant_id = pv.id
		WHERE i.seller_id = $1
	`
	args := []interface{}{sellerID}

	countQuery := "SELECT COUNT(*) " + baseQuery
	var totalCount int
	err := r.db.QueryRow(ctx, countQuery, args...).Scan(&totalCount)
	if err != nil {
		return nil, 0, err
	}

	selectQuery := `
		SELECT
			i.product_variant_id, i.product_id, p.title,
			(SELECT image_url FROM product_images WHERE product_id = p.id ORDER BY sort_order ASC, created_at ASC LIMIT 1) as image_url,
			pv.option_values, pv.size, pv.color, pv.sku,
			i.total_stock, i.reserved_stock,
			COALESCE((
				SELECT SUM(ssi.expected_quantity - ssi.accepted_quantity)
				FROM seller_supply_items ssi
				JOIN seller_supplies ss ON ssi.supply_id = ss.id
				WHERE ssi.variant_id = i.product_variant_id
				  AND ss.status IN ('ready_to_ship', 'shipped_by_seller', 'arrived_at_zamk', 'receiving')
			), 0) as inbound
	` + baseQuery + `
		ORDER BY p.title ASC, pv.sku ASC
		LIMIT $2 OFFSET $3
	`
	args = append(args, limit, offset)

	rows, err := r.db.Query(ctx, selectQuery, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var items []SellerInventoryItem
	for rows.Next() {
		var item SellerInventoryItem
		var imageUrl *string
		var optionValues map[string]interface{}
		var size, color, sku *string

		err := rows.Scan(
			&item.VariantID, &item.ProductID, &item.ProductTitle,
			&imageUrl, &optionValues, &size, &color, &sku,
			&item.OnHand, &item.Reserved, &item.Inbound,
		)
		if err != nil {
			return nil, 0, err
		}

		item.Image = imageUrl
		item.Available = item.OnHand - item.Reserved

		if sku != nil {
			item.SKU = *sku
		}

		// Merge legacy size/color into optionValues if optionValues is missing
		if optionValues == nil {
			optionValues = make(map[string]interface{})
		}
		if size != nil && *size != "" && optionValues["Размер"] == nil {
			optionValues["Размер"] = *size
		}
		if color != nil && *color != "" && optionValues["Цвет"] == nil {
			optionValues["Цвет"] = *color
		}
		item.OptionValues = optionValues

		items = append(items, item)
	}

	return items, totalCount, nil
}

func (r *Repository) listInventoryItems(ctx context.Context, query string, args ...any) ([]Item, error) {
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list inventory items: %w", err)
	}
	defer rows.Close()

	var items []Item
	for rows.Next() {
		var i Item
		if err := rows.Scan(&i.ID, &i.ProductID, &i.ProductVariantID, &i.SellerID, &i.TotalStock, &i.ReservedStock, &i.CreatedAt, &i.UpdatedAt); err != nil {
			return nil, err
		}
		i.ComputeAvailable()
		items = append(items, i)
	}
	return items, nil
}

func (r *Repository) ListMovementsByInventoryItemID(ctx context.Context, itemID uuid.UUID, limit, offset int) ([]StockMovement, error) {
	query := `
		SELECT id, inventory_item_id, product_id, product_variant_id, seller_id, type, quantity, reason, actor_user_id, reference_type, reference_id, created_at
		FROM stock_movements
		WHERE inventory_item_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := r.db.Query(ctx, query, itemID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list stock movements: %w", err)
	}
	defer rows.Close()

	var movs []StockMovement
	for rows.Next() {
		var m StockMovement
		if err := rows.Scan(
			&m.ID, &m.InventoryItemID, &m.ProductID, &m.ProductVariantID, &m.SellerID,
			&m.Type, &m.Quantity, &m.Reason, &m.ActorUserID, &m.ReferenceType, &m.ReferenceID, &m.CreatedAt,
		); err != nil {
			return nil, err
		}
		movs = append(movs, m)
	}
	return movs, nil
}

func FormatUnitStatus(s string) string {
	switch strings.ToLower(s) {
	case "warehouse":
		return "На складе"
	case "expected":
		return "Ожидается"
	case "damaged":
		return "Поврежден"
	case "written_off":
		return "Списан"
	case "shipped":
		return "Отгружен"
	default:
		return s
	}
}

func (r *Repository) ListAdminInventoryRich(ctx context.Context, q, sellerId, source, accountingMode, stockStatus string, lowStock bool, limit, offset int) ([]AdminInventoryItem, int, int, *PhysicalUnitContext, error) {
	qTrimmed := strings.TrimSpace(q)
	var unitCtx *PhysicalUnitContext

	if qTrimmed != "" {
		unitQuery := `
			SELECT iu.unit_code, iu.status, p.id, p.title, pv.id, COALESCE(pv.sku, ''), COALESCE(pv.size, ''), COALESCE(pv.color, '')
			FROM inventory_units iu
			JOIN product_variants pv ON pv.id = iu.product_variant_id
			JOIN products p ON p.id = pv.product_id
			WHERE iu.unit_code = $1 OR iu.unit_code = UPPER($1)
			LIMIT 1
		`
		var uCode, uStatus, pTitle, pSku, pSize, pColor string
		var pID, pvID uuid.UUID
		err := r.db.QueryRow(ctx, unitQuery, qTrimmed).Scan(&uCode, &uStatus, &pID, &pTitle, &pvID, &pSku, &pSize, &pColor)
		if err == nil {
			var variantParts []string
			if pSku != "" {
				variantParts = append(variantParts, pSku)
			}
			if pSize != "" {
				variantParts = append(variantParts, pSize)
			}
			if pColor != "" {
				variantParts = append(variantParts, pColor)
			}
			vLabel := strings.Join(variantParts, " / ")

			unitCtx = &PhysicalUnitContext{
				UnitCode:     uCode,
				Status:       uStatus,
				StatusLabel:  FormatUnitStatus(uStatus),
				ProductTitle: pTitle,
				VariantLabel: vLabel,
				SKU:          pSku,
				ProductID:    pID,
				VariantID:    pvID,
			}
		}
	}

	baseQuery := `
		FROM inventory_items i
		JOIN products p ON i.product_id = p.id
		JOIN product_variants pv ON i.product_variant_id = pv.id
		LEFT JOIN sellers su ON i.seller_id = su.id
		LEFT JOIN LATERAL (
			SELECT
				COUNT(*) FILTER (WHERE iu.status = 'warehouse') AS physical_warehouse,
				COUNT(*) FILTER (WHERE iu.status = 'warehouse' AND oia.id IS NOT NULL
					AND COALESCE(o.status, '') NOT IN ('delivered', 'cancelled', 'returned', 'refunded')
					AND COALESCE(f.status, '') NOT IN ('delivered', 'cancelled')) AS physical_allocated,
				COUNT(*) FILTER (WHERE iu.status = 'warehouse' AND oia.id IS NOT NULL
					AND (COALESCE(o.status, '') IN ('delivered', 'cancelled', 'returned', 'refunded')
					     OR COALESCE(f.status, '') IN ('delivered', 'cancelled'))) AS physical_stale_allocated,
				COUNT(*) FILTER (WHERE iu.status = 'warehouse' AND oia.id IS NOT NULL AND oia.picked_at IS NOT NULL
					AND COALESCE(o.status, '') NOT IN ('delivered', 'cancelled', 'returned', 'refunded')
					AND COALESCE(f.status, '') NOT IN ('delivered', 'cancelled')) AS physical_picked,
				COUNT(*) FILTER (WHERE iu.status = 'expected') AS physical_expected,
				COUNT(*) FILTER (WHERE iu.status = 'damaged') AS physical_damaged,
				COUNT(*) FILTER (WHERE iu.status = 'written_off') AS physical_written_off,
				COUNT(*) FILTER (WHERE iu.status = 'shipped') AS physical_shipped
			FROM inventory_units iu
			LEFT JOIN order_item_allocations oia
				ON oia.inventory_unit_id = iu.id
				AND oia.released_at IS NULL
			LEFT JOIN order_items oi ON oi.id = oia.order_item_id
			LEFT JOIN orders o ON o.id = oi.order_id
			LEFT JOIN order_fulfillments f ON f.id = oi.order_fulfillment_id
			WHERE iu.product_variant_id = i.product_variant_id
		) u ON true
		WHERE 1=1
	`
	args := []interface{}{}
	argID := 1

	if qTrimmed != "" {
		if unitCtx != nil {
			baseQuery += ` AND (p.title ILIKE $` + fmt.Sprintf("%d", argID) +
				` OR pv.sku ILIKE $` + fmt.Sprintf("%d", argID+1) +
				` OR COALESCE(pv.seller_sku, '') ILIKE $` + fmt.Sprintf("%d", argID+2) +
				` OR COALESCE(pv.barcode, '') ILIKE $` + fmt.Sprintf("%d", argID+3) +
				` OR i.product_variant_id = $` + fmt.Sprintf("%d", argID+4) + `)`
			args = append(args, "%"+qTrimmed+"%", "%"+qTrimmed+"%", "%"+qTrimmed+"%", "%"+qTrimmed+"%", unitCtx.VariantID)
			argID += 5
		} else {
			baseQuery += ` AND (p.title ILIKE $` + fmt.Sprintf("%d", argID) +
				` OR pv.sku ILIKE $` + fmt.Sprintf("%d", argID+1) +
				` OR COALESCE(pv.seller_sku, '') ILIKE $` + fmt.Sprintf("%d", argID+2) +
				` OR COALESCE(pv.barcode, '') ILIKE $` + fmt.Sprintf("%d", argID+3) +
				` OR EXISTS (SELECT 1 FROM inventory_units iu WHERE iu.product_variant_id = i.product_variant_id AND (iu.unit_code ILIKE $` + fmt.Sprintf("%d", argID+4) + ` OR iu.unit_code ILIKE $` + fmt.Sprintf("%d", argID+5) + `)))`
			args = append(args, "%"+qTrimmed+"%", "%"+qTrimmed+"%", "%"+qTrimmed+"%", "%"+qTrimmed+"%", "%"+qTrimmed+"%", "%"+strings.ToUpper(qTrimmed)+"%")
			argID += 6
		}
	}

	if sellerId != "" {
		baseQuery += ` AND i.seller_id = $` + fmt.Sprintf("%d", argID)
		args = append(args, sellerId)
		argID++
	}

	if source == "auction_direct_sale" {
		baseQuery += ` AND i.seller_id = '` + common.PlatformSellerIDStr + `'`
	} else if source == "seller" {
		baseQuery += ` AND i.seller_id != '` + common.PlatformSellerIDStr + `'`
	}

	switch accountingMode {
	case "serialized":
		baseQuery += ` AND (COALESCE(u.physical_warehouse, 0) > 0 AND (i.total_stock - COALESCE(u.physical_warehouse, 0)) = 0)`
	case "mixed":
		baseQuery += ` AND (COALESCE(u.physical_warehouse, 0) > 0 AND (i.total_stock - COALESCE(u.physical_warehouse, 0)) > 0)`
	case "legacy":
		baseQuery += ` AND (i.total_stock > 0 AND COALESCE(u.physical_warehouse, 0) = 0)`
	case "inconsistent":
		baseQuery += ` AND (
			COALESCE(u.physical_stale_allocated, 0) > 0
			OR COALESCE(u.physical_allocated, 0) > COALESCE(u.physical_warehouse, 0)
			OR COALESCE(u.physical_warehouse, 0) > i.total_stock
			OR COALESCE(u.physical_allocated, 0) > i.reserved_stock
			OR i.reserved_stock > i.total_stock
			OR (i.total_stock - COALESCE(u.physical_warehouse, 0)) < 0
		)`
	}

	switch stockStatus {
	case "available":
		baseQuery += ` AND (i.total_stock - i.reserved_stock) > 0`
	case "out_of_stock":
		baseQuery += ` AND (i.total_stock - i.reserved_stock) <= 0`
	case "has_reserved":
		baseQuery += ` AND i.reserved_stock > 0`
	case "has_inbound":
		baseQuery += ` AND COALESCE(u.physical_expected, 0) > 0`
	case "has_issue":
		baseQuery += ` AND (
			COALESCE(u.physical_stale_allocated, 0) > 0
			OR COALESCE(u.physical_allocated, 0) > COALESCE(u.physical_warehouse, 0)
			OR COALESCE(u.physical_warehouse, 0) > i.total_stock
			OR COALESCE(u.physical_allocated, 0) > i.reserved_stock
			OR i.reserved_stock > i.total_stock
			OR (i.total_stock - COALESCE(u.physical_warehouse, 0)) < 0
		)`
	}

	if lowStock {
		baseQuery += ` AND (i.total_stock - i.reserved_stock) <= 10`
	}

	countQuery := "SELECT COUNT(*) " + baseQuery
	var totalCount int
	err := r.db.QueryRow(ctx, countQuery, args...).Scan(&totalCount)
	if err != nil {
		return nil, 0, 0, nil, err
	}

	var issuesCount int
	issuesQuery := `
		SELECT COUNT(*) FROM inventory_items i
		LEFT JOIN LATERAL (
			SELECT
				COUNT(*) FILTER (WHERE iu.status = 'warehouse') AS physical_warehouse,
				COUNT(*) FILTER (WHERE iu.status = 'warehouse' AND oia.id IS NOT NULL
					AND COALESCE(o.status, '') NOT IN ('delivered', 'cancelled', 'returned', 'refunded')
					AND COALESCE(f.status, '') NOT IN ('delivered', 'cancelled')) AS physical_allocated,
				COUNT(*) FILTER (WHERE iu.status = 'warehouse' AND oia.id IS NOT NULL
					AND (COALESCE(o.status, '') IN ('delivered', 'cancelled', 'returned', 'refunded')
					     OR COALESCE(f.status, '') IN ('delivered', 'cancelled'))) AS physical_stale_allocated
			FROM inventory_units iu
			LEFT JOIN order_item_allocations oia ON oia.inventory_unit_id = iu.id AND oia.released_at IS NULL
			LEFT JOIN order_items oi ON oi.id = oia.order_item_id
			LEFT JOIN orders o ON o.id = oi.order_id
			LEFT JOIN order_fulfillments f ON f.id = oi.order_fulfillment_id
			WHERE iu.product_variant_id = i.product_variant_id
		) u ON true
		WHERE
			u.physical_stale_allocated > 0
			OR u.physical_allocated > u.physical_warehouse
			OR u.physical_warehouse > i.total_stock
			OR u.physical_allocated > i.reserved_stock
			OR i.reserved_stock > i.total_stock
			OR (i.total_stock - u.physical_warehouse) < 0
	`
	_ = r.db.QueryRow(ctx, issuesQuery).Scan(&issuesCount)

	selectQuery := `
		SELECT
			i.id, i.product_id, i.product_variant_id,
			p.title, p.slug, p.main_image_url,
			pv.size, pv.color, pv.sku, pv.seller_sku, pv.barcode,
			i.seller_id, su.brand_name,
			CASE WHEN i.seller_id::text = '` + common.PlatformSellerIDStr + `' THEN 'auction_direct_sale' ELSE 'seller' END,
			i.total_stock, i.reserved_stock, i.created_at, i.updated_at,
			COALESCE(u.physical_warehouse, 0),
			COALESCE(u.physical_allocated, 0),
			COALESCE(u.physical_stale_allocated, 0),
			COALESCE(u.physical_picked, 0),
			COALESCE(u.physical_expected, 0),
			COALESCE(u.physical_damaged, 0),
			COALESCE(u.physical_written_off, 0),
			COALESCE(u.physical_shipped, 0)
	` + baseQuery + `
		ORDER BY i.created_at DESC
		LIMIT $` + fmt.Sprintf("%d", argID) + ` OFFSET $` + fmt.Sprintf("%d", argID+1)

	args = append(args, limit, offset)

	rows, err := r.db.Query(ctx, selectQuery, args...)
	if err != nil {
		return nil, 0, 0, nil, err
	}
	defer rows.Close()

	var items []AdminInventoryItem
	for rows.Next() {
		var item AdminInventoryItem
		var pSlug string
		var pMainImg, size, color, sku, sellerSku, barcode *string
		var brandName *string
		var cAt, uAt time.Time
		var pWh, pAlloc, pStaleAlloc, pPicked, pExp, pDam, pWoff, pShip int

		err := rows.Scan(
			&item.ID, &item.ProductID, &item.ProductVariantID,
			&item.ProductTitle, &pSlug, &pMainImg,
			&size, &color, &sku, &sellerSku, &barcode,
			&item.SellerID, &brandName,
			&item.Source,
			&item.TotalStock, &item.ReservedStock, &cAt, &uAt,
			&pWh, &pAlloc, &pStaleAlloc, &pPicked, &pExp, &pDam, &pWoff, &pShip,
		)
		if err != nil {
			return nil, 0, 0, nil, err
		}

		aggTotal := item.TotalStock
		aggReserved := item.ReservedStock
		aggAvailable := aggTotal - aggReserved
		if aggAvailable < 0 {
			aggAvailable = 0
		}
		item.AvailableStock = aggAvailable

		agg := AggregateStock{
			Total:     aggTotal,
			Reserved:  aggReserved,
			Available: aggAvailable,
		}

		physFree := pWh - pAlloc
		if physFree < 0 {
			physFree = 0
		}
		phys := PhysicalStock{
			Warehouse:      pWh,
			Allocated:      pAlloc,
			Picked:         pPicked,
			Free:           physFree,
			Expected:       pExp,
			Damaged:        pDam,
			WrittenOff:     pWoff,
			Shipped:        pShip,
			StaleAllocated: pStaleAlloc,
		}

		legOnHand := aggTotal - pWh
		legReserved := aggReserved - pAlloc
		legAvailable := legOnHand - legReserved
		leg := LegacyStock{
			OnHand:    legOnHand,
			Reserved:  legReserved,
			Available: legAvailable,
		}

		mode, health := EvaluateInventoryHealth(agg, phys, leg)
		item.Aggregate = agg
		item.Physical = phys
		item.Legacy = leg
		item.AccountingMode = mode
		item.Health = health

		item.CreatedAt = cAt.Format(time.RFC3339)
		item.UpdatedAt = uAt.Format(time.RFC3339)

		variantParts := []string{}
		if sku != nil && *sku != "" {
			variantParts = append(variantParts, *sku)
		}
		if size != nil && *size != "" {
			variantParts = append(variantParts, *size)
		}
		if color != nil && *color != "" {
			variantParts = append(variantParts, *color)
		}

		item.VariantLabel = ""
		for idx, part := range variantParts {
			if idx > 0 {
				item.VariantLabel += " / "
			}
			item.VariantLabel += part
		}

		item.Product = ProductInfo{
			ID:           item.ProductID,
			Title:        item.ProductTitle,
			Slug:         pSlug,
			MainImageURL: pMainImg,
		}

		skuVal := ""
		if sku != nil {
			skuVal = *sku
		}
		sellerSkuVal := ""
		if sellerSku != nil {
			sellerSkuVal = *sellerSku
		}
		barcodeVal := ""
		if barcode != nil {
			barcodeVal = *barcode
		}
		sizeVal := ""
		if size != nil {
			sizeVal = *size
		}
		colorVal := ""
		if color != nil {
			colorVal = *color
		}

		item.Variant = VariantInfo{
			ID:        item.ProductVariantID,
			SKU:       skuVal,
			SellerSKU: sellerSkuVal,
			Barcode:   barcodeVal,
			Size:      sizeVal,
			Color:     colorVal,
			Label:     item.VariantLabel,
		}

		sellerNameVal := item.SellerID.String()
		if brandName != nil && *brandName != "" {
			sellerNameVal = *brandName
		}
		item.Seller = SellerInfo{
			ID:   item.SellerID,
			Name: sellerNameVal,
		}
		item.SellerName = sellerNameVal

		items = append(items, item)
	}

	return items, totalCount, issuesCount, unitCtx, nil
}

func (r *Repository) GetAdminInventoryItemRich(ctx context.Context, id uuid.UUID) (*AdminInventoryItem, error) {
	query := `
		SELECT
			i.id, i.product_id, i.product_variant_id,
			p.title, p.slug, p.main_image_url,
			pv.size, pv.color, pv.sku, pv.seller_sku, pv.barcode,
			i.seller_id, su.brand_name,
			CASE WHEN i.seller_id::text = '` + common.PlatformSellerIDStr + `' THEN 'auction_direct_sale' ELSE 'seller' END,
			i.total_stock, i.reserved_stock, i.created_at, i.updated_at,
			COALESCE(u.physical_warehouse, 0),
			COALESCE(u.physical_allocated, 0),
			COALESCE(u.physical_stale_allocated, 0),
			COALESCE(u.physical_picked, 0),
			COALESCE(u.physical_expected, 0),
			COALESCE(u.physical_damaged, 0),
			COALESCE(u.physical_written_off, 0),
			COALESCE(u.physical_shipped, 0)
		FROM inventory_items i
		JOIN products p ON i.product_id = p.id
		JOIN product_variants pv ON i.product_variant_id = pv.id
		LEFT JOIN sellers su ON i.seller_id = su.id
		LEFT JOIN LATERAL (
			SELECT
				COUNT(*) FILTER (WHERE iu.status = 'warehouse') AS physical_warehouse,
				COUNT(*) FILTER (WHERE iu.status = 'warehouse' AND oia.id IS NOT NULL
					AND COALESCE(o.status, '') NOT IN ('delivered', 'cancelled', 'returned', 'refunded')
					AND COALESCE(f.status, '') NOT IN ('delivered', 'cancelled')) AS physical_allocated,
				COUNT(*) FILTER (WHERE iu.status = 'warehouse' AND oia.id IS NOT NULL
					AND (COALESCE(o.status, '') IN ('delivered', 'cancelled', 'returned', 'refunded')
					     OR COALESCE(f.status, '') IN ('delivered', 'cancelled'))) AS physical_stale_allocated,
				COUNT(*) FILTER (WHERE iu.status = 'warehouse' AND oia.id IS NOT NULL AND oia.picked_at IS NOT NULL
					AND COALESCE(o.status, '') NOT IN ('delivered', 'cancelled', 'returned', 'refunded')
					AND COALESCE(f.status, '') NOT IN ('delivered', 'cancelled')) AS physical_picked,
				COUNT(*) FILTER (WHERE iu.status = 'expected') AS physical_expected,
				COUNT(*) FILTER (WHERE iu.status = 'damaged') AS physical_damaged,
				COUNT(*) FILTER (WHERE iu.status = 'written_off') AS physical_written_off,
				COUNT(*) FILTER (WHERE iu.status = 'shipped') AS physical_shipped
			FROM inventory_units iu
			LEFT JOIN order_item_allocations oia
				ON oia.inventory_unit_id = iu.id
				AND oia.released_at IS NULL
			LEFT JOIN order_items oi ON oi.id = oia.order_item_id
			LEFT JOIN orders o ON o.id = oi.order_id
			LEFT JOIN order_fulfillments f ON f.id = oi.order_fulfillment_id
			WHERE iu.product_variant_id = i.product_variant_id
		) u ON true
		WHERE i.id = $1
	`
	var item AdminInventoryItem
	var pSlug string
	var pMainImg, size, color, sku, sellerSku, barcode *string
	var brandName *string
	var cAt, uAt time.Time
	var pWh, pAlloc, pStaleAlloc, pPicked, pExp, pDam, pWoff, pShip int

	err := r.db.QueryRow(ctx, query, id).Scan(
		&item.ID, &item.ProductID, &item.ProductVariantID,
		&item.ProductTitle, &pSlug, &pMainImg,
		&size, &color, &sku, &sellerSku, &barcode,
		&item.SellerID, &brandName,
		&item.Source,
		&item.TotalStock, &item.ReservedStock, &cAt, &uAt,
		&pWh, &pAlloc, &pStaleAlloc, &pPicked, &pExp, &pDam, &pWoff, &pShip,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrInventoryItemNotFound
		}
		return nil, err
	}

	aggTotal := item.TotalStock
	aggReserved := item.ReservedStock
	aggAvailable := aggTotal - aggReserved
	if aggAvailable < 0 {
		aggAvailable = 0
	}
	item.AvailableStock = aggAvailable

	agg := AggregateStock{
		Total:     aggTotal,
		Reserved:  aggReserved,
		Available: aggAvailable,
	}

	physFree := pWh - pAlloc
	if physFree < 0 {
		physFree = 0
	}
	phys := PhysicalStock{
		Warehouse:      pWh,
		Allocated:      pAlloc,
		Picked:         pPicked,
		Free:           physFree,
		Expected:       pExp,
		Damaged:        pDam,
		WrittenOff:     pWoff,
		Shipped:        pShip,
		StaleAllocated: pStaleAlloc,
	}

	legOnHand := aggTotal - pWh
	legReserved := aggReserved - pAlloc
	legAvailable := legOnHand - legReserved
	leg := LegacyStock{
		OnHand:    legOnHand,
		Reserved:  legReserved,
		Available: legAvailable,
	}

	mode, health := EvaluateInventoryHealth(agg, phys, leg)
	item.Aggregate = agg
	item.Physical = phys
	item.Legacy = leg
	item.AccountingMode = mode
	item.Health = health

	item.CreatedAt = cAt.Format(time.RFC3339)
	item.UpdatedAt = uAt.Format(time.RFC3339)

	variantParts := []string{}
	if sku != nil && *sku != "" {
		variantParts = append(variantParts, *sku)
	}
	if size != nil && *size != "" {
		variantParts = append(variantParts, *size)
	}
	if color != nil && *color != "" {
		variantParts = append(variantParts, *color)
	}

	item.VariantLabel = ""
	for idx, part := range variantParts {
		if idx > 0 {
			item.VariantLabel += " / "
		}
		item.VariantLabel += part
	}

	item.Product = ProductInfo{
		ID:           item.ProductID,
		Title:        item.ProductTitle,
		Slug:         pSlug,
		MainImageURL: pMainImg,
	}

	skuVal := ""
	if sku != nil {
		skuVal = *sku
	}
	sellerSkuVal := ""
	if sellerSku != nil {
		sellerSkuVal = *sellerSku
	}
	barcodeVal := ""
	if barcode != nil {
		barcodeVal = *barcode
	}
	sizeVal := ""
	if size != nil {
		sizeVal = *size
	}
	colorVal := ""
	if color != nil {
		colorVal = *color
	}

	item.Variant = VariantInfo{
		ID:        item.ProductVariantID,
		SKU:       skuVal,
		SellerSKU: sellerSkuVal,
		Barcode:   barcodeVal,
		Size:      sizeVal,
		Color:     colorVal,
		Label:     item.VariantLabel,
	}

	sellerNameVal := item.SellerID.String()
	if brandName != nil && *brandName != "" {
		sellerNameVal = *brandName
	}
	item.Seller = SellerInfo{
		ID:   item.SellerID,
		Name: sellerNameVal,
	}
	item.SellerName = sellerNameVal

	unitsQuery := `
		SELECT
			iu.id,
			iu.unit_code,
			iu.status,
			iu.created_at,
			ss.id AS supply_id,
			ss.supply_number,
			ss.status AS supply_status,
			COALESCE(sc.created_at, srs.completed_at, srs.created_at) AS received_at,
			oia.id AS alloc_id,
			oia.picked_at,
			o.id AS order_id,
			o.order_number,
			o.status AS order_status,
			f.id AS fulfillment_id,
			f.status AS fulfillment_status
		FROM inventory_units iu
		LEFT JOIN seller_supplies ss ON ss.id = iu.origin_supply_id
		LEFT JOIN supply_receiving_sessions srs ON srs.id = iu.receiving_session_id
		LEFT JOIN LATERAL (
			SELECT created_at FROM supply_receiving_scans WHERE inventory_unit_id = iu.id ORDER BY created_at DESC LIMIT 1
		) sc ON true
		LEFT JOIN order_item_allocations oia ON oia.inventory_unit_id = iu.id AND oia.released_at IS NULL
		LEFT JOIN order_items oi ON oi.id = oia.order_item_id
		LEFT JOIN orders o ON o.id = oi.order_id
		LEFT JOIN order_fulfillments f ON f.id = oi.order_fulfillment_id
		WHERE iu.product_variant_id = $1
		ORDER BY iu.created_at DESC, iu.unit_code ASC
	`
	rows, err := r.db.Query(ctx, unitsQuery, item.ProductVariantID)
	if err == nil {
		defer rows.Close()
		physicalUnits := make([]AdminInventoryPhysicalUnit, 0)
		for rows.Next() {
			var uID uuid.UUID
			var uCode, uStatus string
			var uCreatedAt time.Time
			var supplyID *uuid.UUID
			var supplyNumber, supplyStatus *string
			var receivedAt *time.Time
			var allocID *uuid.UUID
			var pickedAt *time.Time
			var orderID *uuid.UUID
			var orderNumber, orderStatus *string
			var fulfillmentID *uuid.UUID
			var fulfillmentStatus *string

			scanErr := rows.Scan(
				&uID, &uCode, &uStatus, &uCreatedAt,
				&supplyID, &supplyNumber, &supplyStatus, &receivedAt,
				&allocID, &pickedAt,
				&orderID, &orderNumber, &orderStatus,
				&fulfillmentID, &fulfillmentStatus,
			)
			if scanErr != nil {
				continue
			}

			availability := "free"
			isStale := false
			var liveAlloc *AdminInventoryAllocationInfo
			var staleAlloc *AdminInventoryAllocationInfo

			if uStatus == "warehouse" {
				if allocID != nil {
					isTerminalOrder := orderStatus != nil && (*orderStatus == "delivered" || *orderStatus == "cancelled" || *orderStatus == "returned" || *orderStatus == "refunded")
					isTerminalFulfill := fulfillmentStatus != nil && (*fulfillmentStatus == "delivered" || *fulfillmentStatus == "cancelled")

					ordNum := formatOrderNum(orderNumber, orderID)
					ordStat := ""
					if orderStatus != nil {
						ordStat = *orderStatus
					}
					var oID uuid.UUID
					if orderID != nil {
						oID = *orderID
					}

					allocInfo := &AdminInventoryAllocationInfo{
						ID:                *allocID,
						OrderID:           oID,
						OrderNumber:       ordNum,
						OrderStatus:       ordStat,
						FulfillmentID:     fulfillmentID,
						FulfillmentStatus: fulfillmentStatus,
						PickedAt:          pickedAt,
					}

					if isTerminalOrder || isTerminalFulfill {
						isStale = true
						availability = "free"
						staleAlloc = allocInfo
					} else {
						isStale = false
						if pickedAt != nil {
							availability = "picked"
						} else {
							availability = "allocated"
						}
						liveAlloc = allocInfo
					}
				} else {
					availability = "free"
				}
			} else {
				switch uStatus {
				case "expected":
					availability = "unavailable_expected"
				case "damaged":
					availability = "unavailable_damaged"
				case "written_off":
					availability = "unavailable_written_off"
				case "shipped":
					availability = "unavailable_shipped"
				default:
					availability = "unavailable"
				}
				if allocID != nil {
					isTerminalOrder := orderStatus != nil && (*orderStatus == "delivered" || *orderStatus == "cancelled" || *orderStatus == "returned" || *orderStatus == "refunded")
					isTerminalFulfill := fulfillmentStatus != nil && (*fulfillmentStatus == "delivered" || *fulfillmentStatus == "cancelled")
					ordNum := formatOrderNum(orderNumber, orderID)
					ordStat := ""
					if orderStatus != nil {
						ordStat = *orderStatus
					}
					var oID uuid.UUID
					if orderID != nil {
						oID = *orderID
					}
					allocInfo := &AdminInventoryAllocationInfo{
						ID:                *allocID,
						OrderID:           oID,
						OrderNumber:       ordNum,
						OrderStatus:       ordStat,
						FulfillmentID:     fulfillmentID,
						FulfillmentStatus: fulfillmentStatus,
						PickedAt:          pickedAt,
					}
					isStale = false
					if isTerminalOrder || isTerminalFulfill {
						staleAlloc = allocInfo
					} else {
						liveAlloc = allocInfo
					}
				}
			}

			var supplyLineage *AdminInventorySupplyLineage
			if supplyID != nil && supplyNumber != nil && supplyStatus != nil {
				supplyLineage = &AdminInventorySupplyLineage{
					SupplyID:     *supplyID,
					SupplyNumber: *supplyNumber,
					SupplyStatus: *supplyStatus,
					ReceivedAt:   receivedAt,
				}
			}

			physicalUnits = append(physicalUnits, AdminInventoryPhysicalUnit{
				ID:                uID,
				UnitCode:          uCode,
				Status:            uStatus,
				CreatedAt:         uCreatedAt,
				Availability:      availability,
				IsStaleAllocation: isStale,
				LiveAllocation:    liveAlloc,
				StaleAllocation:   staleAlloc,
				SupplyLineage:     supplyLineage,
			})
		}
		item.PhysicalUnits = physicalUnits
	}

	return &item, nil
}

func formatOrderNum(oNum *string, oID *uuid.UUID) string {
	if oNum != nil && *oNum != "" {
		return *oNum
	}
	if oID != nil && *oID != uuid.Nil {
		s := (*oID).String()
		if len(s) >= 8 {
			return "ORD-" + strings.ToUpper(s[:8])
		}
		return "ORD-" + strings.ToUpper(s)
	}
	return "—"
}

func formatRussianOrderStatus(status string) string {
	switch status {
	case "pending":
		return "Создан"
	case "paid":
		return "Оплачен"
	case "assembling":
		return "В сборке"
	case "assembled":
		return "Собран"
	case "packed":
		return "Упакован"
	case "shipped":
		return "В доставке"
	case "delivered":
		return "Доставлен"
	case "cancelled":
		return "Отменён"
	case "returned":
		return "Возвращён"
	case "refunded":
		return "Возврат средств"
	default:
		if status != "" {
			return status
		}
		return "Неизвестно"
	}
}

func formatReturnNum(returnID uuid.UUID) string {
	s := returnID.String()
	if len(s) >= 8 {
		return "RET-" + strings.ToUpper(s[:8])
	}
	return "RET-" + strings.ToUpper(s)
}

func formatRussianReturnReason(reason string) string {
	switch reason {
	case "damaged":
		return "Брак / Дефект"
	case "size":
		return "Не подошёл размер"
	case "wrong_item":
		return "Ошибочный товар"
	case "quality":
		return "Качество товара"
	case "not_needed":
		return "Товар больше не нужен"
	default:
		if reason != "" {
			return reason
		}
		return "Причина не указана"
	}
}

// businessCausalOrder defines logical business progression:
// prerequisite events have lower values, consequential events have higher values.
// When two events share an identical timestamp, the prerequisite must appear before the consequence.
func businessCausalOrder(t string) int {
	switch t {
	case "inbound_created":
		return 10
	case "received":
		return 20
	case "allocation_created":
		return 30
	case "allocation_released":
		return 35
	case "picked":
		return 40
	case "packed":
		return 50
	case "shipped":
		return 60
	case "delivered":
		return 70
	case "return_requested":
		return 80
	case "return_approved":
		return 90
	case "return_receiving_started":
		return 100
	case "return_unit_scanned":
		return 110
	case "return_received", "return_damaged":
		return 120
	case "reconciliation_stale_allocation_released":
		return 36
	case "reconciliation_replacement_allocated":
		return 37
	case "reconciliation_missing_written_off":
		return 125
	default:
		return 999
	}
}

func (r *Repository) GetAdminInventoryUnitTraceability(ctx context.Context, unitCode string) (*AdminInventoryUnitTraceability, error) {
	cleanCode := strings.TrimSpace(unitCode)
	if cleanCode == "" {
		return nil, ErrInventoryUnitNotFound
	}

	unitQuery := `
		SELECT
			iu.id, iu.unit_code, iu.status, iu.created_at,
			pv.id as variant_id, pv.sku, pv.barcode, pv.size, pv.color,
			p.id as product_id, p.title as product_title, p.source,
			s.id as seller_id, s.brand_name as seller_name,
			ss.id as origin_supply_id, ss.supply_number, ss.status as supply_status,
			scan.created_at as received_at, staff.name as staff_name
		FROM inventory_units iu
		JOIN product_variants pv ON pv.id = iu.product_variant_id
		JOIN products p ON p.id = pv.product_id
		JOIN sellers s ON s.id = p.seller_id
		LEFT JOIN seller_supplies ss ON ss.id = iu.origin_supply_id
		LEFT JOIN LATERAL (
			SELECT srs.created_at, srs.staff_id
			FROM supply_receiving_scans srs
			WHERE srs.inventory_unit_id = iu.id AND srs.voided_at IS NULL
			ORDER BY srs.created_at DESC
			LIMIT 1
		) scan ON true
		LEFT JOIN users staff ON staff.id = scan.staff_id
		WHERE LOWER(TRIM(iu.unit_code)) = LOWER($1)
		LIMIT 1
	`

	var (
		uID            uuid.UUID
		uCode          string
		uStatus        string
		uCreatedAt     time.Time
		vID            uuid.UUID
		sku            *string
		barcode        *string
		size           *string
		color          *string
		pID            uuid.UUID
		pTitle         string
		pSource        string
		sID            uuid.UUID
		sBrandName     *string
		originSupplyID *uuid.UUID
		supplyNumber   *string
		supplyStatus   *string
		receivedAt     *time.Time
		staffName      *string
	)

	err := r.db.QueryRow(ctx, unitQuery, cleanCode).Scan(
		&uID, &uCode, &uStatus, &uCreatedAt,
		&vID, &sku, &barcode, &size, &color,
		&pID, &pTitle, &pSource,
		&sID, &sBrandName,
		&originSupplyID, &supplyNumber, &supplyStatus,
		&receivedAt, &staffName,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrInventoryUnitNotFound
		}
		return nil, fmt.Errorf("failed to get unit traceability: %w", err)
	}

	skuVal, barcodeVal, sizeVal, colorVal := "", "", "", ""
	if sku != nil {
		skuVal = *sku
	}
	if barcode != nil {
		barcodeVal = *barcode
	}
	if size != nil {
		sizeVal = *size
	}
	if color != nil {
		colorVal = *color
	}
	sellerNameVal := sID.String()
	if sBrandName != nil && *sBrandName != "" {
		sellerNameVal = *sBrandName
	}

	variantParts := []string{}
	if sizeVal != "" {
		variantParts = append(variantParts, sizeVal)
	}
	if colorVal != "" {
		variantParts = append(variantParts, colorVal)
	}
	variantName := strings.Join(variantParts, " · ")

	// 2. Allocations
	allocQuery := `
		SELECT
			oia.id as allocation_id, oia.created_at as allocated_at, oia.picked_at, oia.released_at, oia.release_reason,
			o.id as order_id, o.order_number, o.status as order_status, o.created_at as order_created_at, o.customer_name,
			f.id as fulfillment_id, f.status as fulfillment_status, f.packed_at,
			sh.shipped_at, sh.delivered_at
		FROM order_item_allocations oia
		JOIN order_items oi ON oi.id = oia.order_item_id
		JOIN orders o ON o.id = oi.order_id
		LEFT JOIN order_fulfillments f ON f.id = oi.order_fulfillment_id
		LEFT JOIN shipments sh ON sh.fulfillment_id = f.id
		WHERE oia.inventory_unit_id = $1
		ORDER BY oia.created_at ASC
	`
	rows, err := r.db.Query(ctx, allocQuery, uID)
	if err != nil {
		return nil, fmt.Errorf("failed to query unit allocations: %w", err)
	}
	defer rows.Close()

	type allocRow struct {
		allocationID      uuid.UUID
		allocatedAt       time.Time
		pickedAt          *time.Time
		releasedAt        *time.Time
		releaseReason     *string
		orderID           uuid.UUID
		orderNumber       *string
		orderStatus       string
		orderCreatedAt    time.Time
		customerName      *string
		fulfillmentID     *uuid.UUID
		fulfillmentStatus *string
		packedAt          *time.Time
		shippedAt         *time.Time
		deliveredAt       *time.Time
	}
	var allocRows []allocRow
	for rows.Next() {
		var ar allocRow
		if err := rows.Scan(
			&ar.allocationID, &ar.allocatedAt, &ar.pickedAt, &ar.releasedAt, &ar.releaseReason,
			&ar.orderID, &ar.orderNumber, &ar.orderStatus, &ar.orderCreatedAt, &ar.customerName,
			&ar.fulfillmentID, &ar.fulfillmentStatus, &ar.packedAt,
			&ar.shippedAt, &ar.deliveredAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan alloc row: %w", err)
		}
		allocRows = append(allocRows, ar)
	}

	// 3. Returns
	returnQuery := `
		SELECT
			riu.id as return_unit_id, riu.scanned_at, riu.updated_at as riu_updated_at, riu.disposition, riu.inspected_condition,
			ri.reason as item_reason, ri.condition as item_condition,
			r.id as return_id, r.status as return_status, r.created_at as return_created_at,
			r.approved_at, r.receiving_started_at, r.completed_at, r.reason as return_reason,
			u.name as customer_name
		FROM return_item_units riu
		JOIN order_item_allocations oia ON oia.id = riu.order_item_allocation_id
		JOIN return_items ri ON ri.id = riu.return_item_id
		JOIN returns r ON r.id = ri.return_id
		LEFT JOIN users u ON u.id = r.user_id
		WHERE oia.inventory_unit_id = $1
		ORDER BY r.created_at ASC
	`
	retRows, err := r.db.Query(ctx, returnQuery, uID)
	if err != nil {
		return nil, fmt.Errorf("failed to query unit returns: %w", err)
	}
	defer retRows.Close()

	type returnRow struct {
		returnUnitID       uuid.UUID
		scannedAt          *time.Time
		riuUpdatedAt       *time.Time
		disposition        *string
		inspectedCondition *string
		itemReason         *string
		itemCondition      *string
		returnID           uuid.UUID
		returnStatus       string
		returnCreatedAt    time.Time
		approvedAt         *time.Time
		receivingStartedAt *time.Time
		completedAt        *time.Time
		returnReason       string
		customerName       *string
	}
	var retList []returnRow
	for retRows.Next() {
		var rr returnRow
		if err := retRows.Scan(
			&rr.returnUnitID, &rr.scannedAt, &rr.riuUpdatedAt, &rr.disposition, &rr.inspectedCondition,
			&rr.itemReason, &rr.itemCondition,
			&rr.returnID, &rr.returnStatus, &rr.returnCreatedAt,
			&rr.approvedAt, &rr.receivingStartedAt, &rr.completedAt, &rr.returnReason,
			&rr.customerName,
		); err != nil {
			return nil, fmt.Errorf("failed to scan return row: %w", err)
		}
		retList = append(retList, rr)
	}

	// 4. Current State & Context
	var (
		liveAlloc         *AdminInventoryAllocationInfo
		staleAlloc        *AdminInventoryAllocationInfo
		isStale           bool
		availability      string
		healthIssue       string
		hasPartialHistory bool
	)

	// Check allocations for current state
	for _, ar := range allocRows {
		ordNum := formatOrderNum(ar.orderNumber, &ar.orderID)
		allocInfo := &AdminInventoryAllocationInfo{
			ID:                ar.allocationID,
			OrderID:           ar.orderID,
			OrderNumber:       ordNum,
			OrderStatus:       ar.orderStatus,
			FulfillmentID:     ar.fulfillmentID,
			FulfillmentStatus: ar.fulfillmentStatus,
			PickedAt:          ar.pickedAt,
		}

		if ar.releasedAt == nil {
			isTerminal := ar.orderStatus == "delivered" || ar.orderStatus == "cancelled" || ar.orderStatus == "returned" || ar.orderStatus == "refunded"
			if uStatus == "warehouse" {
				if isTerminal {
					isStale = true
					staleAlloc = allocInfo
					healthIssue = "stale_active_allocation"
				} else {
					liveAlloc = allocInfo
				}
			} else {
				if isTerminal {
					staleAlloc = allocInfo
				} else {
					liveAlloc = allocInfo
				}
			}
		}
	}

	if uStatus == "warehouse" {
		if liveAlloc != nil {
			if liveAlloc.PickedAt != nil {
				availability = "picked"
			} else {
				availability = "allocated"
			}
		} else {
			availability = "free"
		}
	} else {
		switch uStatus {
		case "expected":
			availability = "unavailable_expected"
		case "damaged":
			availability = "unavailable_damaged"
		case "written_off":
			availability = "unavailable_written_off"
		case "shipped":
			availability = "unavailable_shipped"
		default:
			availability = "unavailable"
		}
	}

	// 5. Construct Timeline Events
	var timeline []AdminInventoryUnitTimelineEvent

	// Inbound events
	if originSupplyID != nil && supplyNumber != nil {
		timeline = append(timeline, AdminInventoryUnitTimelineEvent{
			ID:              fmt.Sprintf("inbound-%s", uID.String()),
			Type:            "inbound_created",
			Category:        "physical",
			EventName:       "Ожидается поступление",
			Description:     fmt.Sprintf("Создана в партии поставки %s", *supplyNumber),
			Timestamp:       uCreatedAt,
			SourceEntity:    "seller_supplies",
			ReferenceNumber: *supplyNumber,
			ReferenceID:     originSupplyID,
			Link:            fmt.Sprintf("/supply-receiving?id=%s", originSupplyID.String()),
		})
	}
	if receivedAt != nil && originSupplyID != nil && supplyNumber != nil {
		actName := ""
		if staffName != nil {
			actName = *staffName
		}
		timeline = append(timeline, AdminInventoryUnitTimelineEvent{
			ID:              fmt.Sprintf("received-%s", uID.String()),
			Type:            "received",
			Category:        "physical",
			EventName:       "Принята на склад",
			Description:     fmt.Sprintf("Принята по поставке %s в процессе приёмки", *supplyNumber),
			Timestamp:       *receivedAt,
			SourceEntity:    "seller_supplies",
			ReferenceNumber: *supplyNumber,
			ReferenceID:     originSupplyID,
			ActorRole:       "staff",
			ActorName:       actName,
			Link:            fmt.Sprintf("/supply-receiving?id=%s", originSupplyID.String()),
		})
	}

	// Allocation & order lifecycle events
	for _, ar := range allocRows {
		ordNum := formatOrderNum(ar.orderNumber, &ar.orderID)
		orderIDCopy := ar.orderID
		timeline = append(timeline, AdminInventoryUnitTimelineEvent{
			ID:              fmt.Sprintf("alloc-%s", ar.allocationID.String()),
			Type:            "allocation_created",
			Category:        "commitment",
			EventName:       "Назначена заказу",
			Description:     fmt.Sprintf("Зарезервирована и назначена под заказ %s (%s)", ordNum, formatRussianOrderStatus(ar.orderStatus)),
			Timestamp:       ar.allocatedAt,
			SourceEntity:    "orders",
			ReferenceNumber: ordNum,
			ReferenceID:     &orderIDCopy,
			ActorRole:       "system",
			Link:            fmt.Sprintf("/orders/%s", ar.orderID.String()),
		})

		if ar.pickedAt != nil {
			timeline = append(timeline, AdminInventoryUnitTimelineEvent{
				ID:              fmt.Sprintf("picked-%s", ar.allocationID.String()),
				Type:            "picked",
				Category:        "operation",
				EventName:       "Собрана на складе",
				Description:     fmt.Sprintf("Единица отобрана сборщиком для заказа %s", ordNum),
				Timestamp:       *ar.pickedAt,
				SourceEntity:    "order_fulfillments",
				ReferenceNumber: ordNum,
				ReferenceID:     &orderIDCopy,
				ActorRole:       "staff",
				Link:            fmt.Sprintf("/orders/%s", ar.orderID.String()),
			})
		}

		if ar.packedAt != nil {
			timeline = append(timeline, AdminInventoryUnitTimelineEvent{
				ID:              fmt.Sprintf("packed-%s", ar.allocationID.String()),
				Type:            "packed",
				Category:        "physical",
				EventName:       "Упакована",
				Description:     fmt.Sprintf("Упакована в отправление заказа %s", ordNum),
				Timestamp:       *ar.packedAt,
				SourceEntity:    "order_fulfillments",
				ReferenceNumber: ordNum,
				ReferenceID:     &orderIDCopy,
				ActorRole:       "staff",
				Link:            fmt.Sprintf("/orders/%s", ar.orderID.String()),
			})
		}

		if ar.shippedAt != nil {
			timeline = append(timeline, AdminInventoryUnitTimelineEvent{
				ID:              fmt.Sprintf("shipped-%s", ar.allocationID.String()),
				Type:            "shipped",
				Category:        "physical",
				EventName:       "Отгружена со склада",
				Description:     fmt.Sprintf("Передана в доставку по заказу %s", ordNum),
				Timestamp:       *ar.shippedAt,
				SourceEntity:    "shipments",
				ReferenceNumber: ordNum,
				ReferenceID:     &orderIDCopy,
				ActorRole:       "system",
				Link:            fmt.Sprintf("/orders/%s", ar.orderID.String()),
			})
		}

		if ar.deliveredAt != nil {
			custName := ""
			if ar.customerName != nil {
				custName = *ar.customerName
			}
			timeline = append(timeline, AdminInventoryUnitTimelineEvent{
				ID:              fmt.Sprintf("delivered-%s", ar.allocationID.String()),
				Type:            "delivered",
				Category:        "order_lifecycle",
				EventName:       "Доставлена покупателю",
				Description:     fmt.Sprintf("Заказ %s успешно доставлен покупателю", ordNum),
				Timestamp:       *ar.deliveredAt,
				SourceEntity:    "orders",
				ReferenceNumber: ordNum,
				ReferenceID:     &orderIDCopy,
				ActorRole:       "customer",
				ActorName:       custName,
				Link:            fmt.Sprintf("/orders/%s", ar.orderID.String()),
			})
		}

		if ar.releasedAt != nil {
			reasonDesc := "Резерв единицы освобождён"
			if ar.releaseReason != nil && *ar.releaseReason != "" {
				switch *ar.releaseReason {
				case "inventory_reconciliation":
					reasonDesc = "Резерв освобождён в ходе инвентаризации"
				case "order_cancelled":
					reasonDesc = "Резерв освобождён: заказ отменён"
				default:
					reasonDesc = fmt.Sprintf("Резерв освобождён: %s", *ar.releaseReason)
				}
			}
			timeline = append(timeline, AdminInventoryUnitTimelineEvent{
				ID:              fmt.Sprintf("rel-%s", ar.allocationID.String()),
				Type:            "allocation_released",
				Category:        "commitment",
				EventName:       "Назначение снято",
				Description:     reasonDesc,
				Timestamp:       *ar.releasedAt,
				SourceEntity:    "order_item_allocations",
				ReferenceNumber: ordNum,
				ReferenceID:     &orderIDCopy,
				ActorRole:       "system",
				Link:            fmt.Sprintf("/orders/%s", ar.orderID.String()),
			})
		}
	}

	// Return events
	for _, rr := range retList {
		retNum := formatReturnNum(rr.returnID)
		retIDCopy := rr.returnID
		custName := ""
		if rr.customerName != nil {
			custName = *rr.customerName
		}
		desc := fmt.Sprintf("Покупатель оформил возврат %s", retNum)
		if rr.returnReason != "" {
			desc += fmt.Sprintf(" (причина: %s)", formatRussianReturnReason(rr.returnReason))
		}

		timeline = append(timeline, AdminInventoryUnitTimelineEvent{
			ID:              fmt.Sprintf("ret-req-%s", rr.returnID.String()),
			Type:            "return_requested",
			Category:        "order_lifecycle",
			EventName:       "Возврат создан",
			Description:     desc,
			Timestamp:       rr.returnCreatedAt,
			SourceEntity:    "returns",
			ReferenceNumber: retNum,
			ReferenceID:     &retIDCopy,
			ActorRole:       "customer",
			ActorName:       custName,
			Link:            fmt.Sprintf("/returns?id=%s", rr.returnID.String()),
		})

		if rr.approvedAt != nil {
			timeline = append(timeline, AdminInventoryUnitTimelineEvent{
				ID:              fmt.Sprintf("ret-appr-%s", rr.returnID.String()),
				Type:            "return_approved",
				Category:        "order_lifecycle",
				EventName:       "Возврат одобрен",
				Description:     fmt.Sprintf("Возврат %s одобрен к приёмке на складе", retNum),
				Timestamp:       *rr.approvedAt,
				SourceEntity:    "returns",
				ReferenceNumber: retNum,
				ReferenceID:     &retIDCopy,
				ActorRole:       "staff",
				Link:            fmt.Sprintf("/returns?id=%s", rr.returnID.String()),
			})
		}

		if rr.receivingStartedAt != nil {
			timeline = append(timeline, AdminInventoryUnitTimelineEvent{
				ID:              fmt.Sprintf("ret-rec-start-%s", rr.returnID.String()),
				Type:            "return_receiving_started",
				Category:        "operation",
				EventName:       "Приёмка возврата начата",
				Description:     fmt.Sprintf("Склад начал приёмку возврата %s", retNum),
				Timestamp:       *rr.receivingStartedAt,
				SourceEntity:    "returns",
				ReferenceNumber: retNum,
				ReferenceID:     &retIDCopy,
				ActorRole:       "staff",
				Link:            fmt.Sprintf("/returns?id=%s", rr.returnID.String()),
			})
		}

		if rr.scannedAt != nil {
			timeline = append(timeline, AdminInventoryUnitTimelineEvent{
				ID:              fmt.Sprintf("ret-scanned-%s", rr.returnUnitID.String()),
				Type:            "return_unit_scanned",
				Category:        "operation",
				EventName:       "Единица отсканирована при приёмке возврата",
				Description:     fmt.Sprintf("Физическая единица отсканирована на складе при приёмке возврата %s", retNum),
				Timestamp:       *rr.scannedAt,
				SourceEntity:    "return_item_units",
				ReferenceNumber: retNum,
				ReferenceID:     &retIDCopy,
				ActorRole:       "staff",
				Link:            fmt.Sprintf("/returns?id=%s", rr.returnID.String()),
			})
		}

		disp := ""
		if rr.disposition != nil && *rr.disposition != "" {
			disp = *rr.disposition
		}
		if disp != "" {
			var finTime time.Time
			var sourceEnt string
			if rr.completedAt != nil {
				finTime = *rr.completedAt
				sourceEnt = "returns"
			} else if rr.riuUpdatedAt != nil {
				finTime = *rr.riuUpdatedAt
				sourceEnt = "return_item_units"
			} else if rr.scannedAt != nil {
				finTime = *rr.scannedAt
				sourceEnt = "return_item_units"
			}

			if !finTime.IsZero() {
				var evtType, evtName, dText string
				if disp == "restock" {
					evtType = "return_received"
					evtName = "Возвращена на склад"
					dText = fmt.Sprintf("Возврат %s: единица проверена и возвращена в свободный остаток (ресток)", retNum)
				} else {
					evtType = "return_damaged"
					evtName = "Признана браком"
					dText = fmt.Sprintf("Возврат %s: зафиксирован дефект (%s)", retNum, disp)
				}
				timeline = append(timeline, AdminInventoryUnitTimelineEvent{
					ID:              fmt.Sprintf("ret-finalized-%s", rr.returnUnitID.String()),
					Type:            evtType,
					Category:        "physical",
					EventName:       evtName,
					Description:     dText,
					Timestamp:       finTime,
					SourceEntity:    sourceEnt,
					ReferenceNumber: retNum,
					ReferenceID:     &retIDCopy,
					ActorRole:       "staff",
					Link:            fmt.Sprintf("/returns?id=%s", rr.returnID.String()),
				})
			}
		}
	}

	// 4. Reconciliation resolutions
	reconQuery := `
		SELECT
			irr.id, irr.session_id, irr.inventory_unit_id, irr.case_type, irr.action_id,
			irr.performed_by, irr.performed_at, irr.related_allocation_id, irr.replacement_inventory_unit_id,
			irr.note, COALESCE(u.name, '') as staff_name,
			COALESCE(o.order_number, '') as order_number
		FROM inventory_reconciliation_resolutions irr
		LEFT JOIN users u ON u.id = irr.performed_by
		LEFT JOIN order_item_allocations oia ON oia.id = irr.related_allocation_id
		LEFT JOIN order_items oi ON oi.id = oia.order_item_id
		LEFT JOIN orders o ON o.id = oi.order_id
		WHERE irr.inventory_unit_id = $1 OR irr.replacement_inventory_unit_id = $1
		ORDER BY irr.performed_at ASC
	`
	reconRows, err := r.db.Query(ctx, reconQuery, uID)
	if err != nil {
		return nil, fmt.Errorf("failed to query unit reconciliation resolutions: %w", err)
	}
	defer reconRows.Close()

	type reconRow struct {
		id                         uuid.UUID
		sessionID                  uuid.UUID
		inventoryUnitID            uuid.UUID
		caseType                   string
		actionID                   string
		performedBy                uuid.UUID
		performedAt                time.Time
		relatedAllocationID        *uuid.UUID
		replacementInventoryUnitID *uuid.UUID
		note                       *string
		staffName                  string
		orderNumber                string
	}
	var reconList []reconRow
	for reconRows.Next() {
		var rr reconRow
		if err := reconRows.Scan(
			&rr.id, &rr.sessionID, &rr.inventoryUnitID, &rr.caseType, &rr.actionID,
			&rr.performedBy, &rr.performedAt, &rr.relatedAllocationID, &rr.replacementInventoryUnitID,
			&rr.note, &rr.staffName, &rr.orderNumber,
		); err != nil {
			return nil, fmt.Errorf("failed to scan recon row: %w", err)
		}
		reconList = append(reconList, rr)
	}

	for _, rr := range reconList {
		sessionIDCopy := rr.sessionID
		refNum := fmt.Sprintf("Сверка %s", rr.sessionID.String()[:8])
		link := fmt.Sprintf("/inventory/reconciliation/%s", rr.sessionID.String())

		if rr.inventoryUnitID == uID {
			if rr.actionID == "close_stale_allocation" {
				desc := "Старое зависшее назначение освобождено по результатам инвентаризации"
				if rr.orderNumber != "" {
					desc = fmt.Sprintf("Старое зависшее назначение заказа %s освобождено по результатам инвентаризации", rr.orderNumber)
				}
				if rr.note != nil && *rr.note != "" {
					desc += fmt.Sprintf(" (%s)", *rr.note)
				}
				timeline = append(timeline, AdminInventoryUnitTimelineEvent{
					ID:              fmt.Sprintf("recon-stale-%s", rr.id.String()),
					Type:            "reconciliation_stale_allocation_released",
					Category:        "commitment",
					EventName:       "Старое назначение освобождено",
					Description:     desc,
					Timestamp:       rr.performedAt,
					SourceEntity:    "inventory_reconciliation_resolutions",
					ReferenceNumber: refNum,
					ReferenceID:     &sessionIDCopy,
					ActorRole:       "staff",
					ActorName:       rr.staffName,
					Link:            link,
				})
			} else if rr.actionID == "confirm_missing" {
				desc := "Единица признана отсутствующей и списана по результатам инвентаризации"
				if rr.note != nil && *rr.note != "" {
					desc += fmt.Sprintf(" (%s)", *rr.note)
				}
				timeline = append(timeline, AdminInventoryUnitTimelineEvent{
					ID:              fmt.Sprintf("recon-missing-%s", rr.id.String()),
					Type:            "reconciliation_missing_written_off",
					Category:        "physical",
					EventName:       "Списана по результатам инвентаризации",
					Description:     desc,
					Timestamp:       rr.performedAt,
					SourceEntity:    "inventory_reconciliation_resolutions",
					ReferenceNumber: refNum,
					ReferenceID:     &sessionIDCopy,
					ActorRole:       "staff",
					ActorName:       rr.staffName,
					Link:            link,
				})
			}
		}

		if rr.replacementInventoryUnitID != nil && *rr.replacementInventoryUnitID == uID {
			desc := "Единица назначена заказу взамен отсутствующей по результатам инвентаризации"
			if rr.note != nil && *rr.note != "" {
				desc += fmt.Sprintf(" (%s)", *rr.note)
			}
			timeline = append(timeline, AdminInventoryUnitTimelineEvent{
				ID:              fmt.Sprintf("recon-repl-%s", rr.id.String()),
				Type:            "reconciliation_replacement_allocated",
				Category:        "commitment",
				EventName:       "Назначена заказу после инвентаризации",
				Description:     desc,
				Timestamp:       rr.performedAt,
				SourceEntity:    "inventory_reconciliation_resolutions",
				ReferenceNumber: refNum,
				ReferenceID:     &sessionIDCopy,
				ActorRole:       "staff",
				ActorName:       rr.staffName,
				Link:            link,
			})
		}
	}

	// Sort timeline newest-first (descending timestamp).
	// When timestamps are equal, preserve business causal order (prerequisite before consequence):
	sort.SliceStable(timeline, func(i, j int) bool {
		if timeline[i].Timestamp.Equal(timeline[j].Timestamp) {
			return businessCausalOrder(timeline[i].Type) < businessCausalOrder(timeline[j].Type)
		}
		return timeline[i].Timestamp.After(timeline[j].Timestamp)
	})

	// Detect partial history
	if originSupplyID == nil || receivedAt == nil {
		hasPartialHistory = true
	}

	var origin *AdminInventorySupplyLineage
	if originSupplyID != nil && supplyNumber != nil && supplyStatus != nil {
		origin = &AdminInventorySupplyLineage{
			SupplyID:     *originSupplyID,
			SupplyNumber: *supplyNumber,
			SupplyStatus: *supplyStatus,
			ReceivedAt:   receivedAt,
		}
	}

	return &AdminInventoryUnitTraceability{
		Identity: AdminInventoryUnitIdentity{
			ID:           uID,
			UnitCode:     uCode,
			VariantID:    vID,
			ProductID:    pID,
			ProductTitle: pTitle,
			VariantName:  variantName,
			SKU:          skuVal,
			Barcode:      barcodeVal,
			Size:         sizeVal,
			Color:        colorVal,
			SellerID:     sID,
			SellerName:   sellerNameVal,
			Source:       pSource,
		},
		CurrentState: AdminInventoryUnitCurrentState{
			Status:            uStatus,
			Availability:      availability,
			Location:          "Не ведётся",
			IsStaleAllocation: isStale,
			HealthIssue:       healthIssue,
		},
		Origin: origin,
		CurrentContext: AdminInventoryUnitContext{
			LiveAllocation:  liveAlloc,
			StaleAllocation: staleAlloc,
		},
		Timeline:          timeline,
		HasPartialHistory: hasPartialHistory,
	}, nil
}

func (r *Repository) GetAdminInventoryItemRichByVariantID(ctx context.Context, id uuid.UUID) (*AdminInventoryItem, error) {
	query := `
		SELECT
			i.id, i.product_id, i.product_variant_id,
			p.title, p.slug, p.main_image_url,
			pv.size, pv.color, pv.sku, pv.seller_sku, pv.barcode,
			i.seller_id, su.brand_name,
			CASE WHEN i.seller_id::text = '` + common.PlatformSellerIDStr + `' THEN 'auction_direct_sale' ELSE 'seller' END,
			i.total_stock, i.reserved_stock, i.created_at, i.updated_at,
			COALESCE(u.physical_warehouse, 0),
			COALESCE(u.physical_allocated, 0),
			COALESCE(u.physical_stale_allocated, 0),
			COALESCE(u.physical_picked, 0),
			COALESCE(u.physical_expected, 0),
			COALESCE(u.physical_damaged, 0),
			COALESCE(u.physical_written_off, 0),
			COALESCE(u.physical_shipped, 0)
		FROM inventory_items i
		JOIN products p ON i.product_id = p.id
		JOIN product_variants pv ON i.product_variant_id = pv.id
		LEFT JOIN sellers su ON i.seller_id = su.id
		LEFT JOIN LATERAL (
			SELECT
				COUNT(*) FILTER (WHERE iu.status = 'warehouse') AS physical_warehouse,
				COUNT(*) FILTER (WHERE iu.status = 'warehouse' AND oia.id IS NOT NULL
					AND COALESCE(o.status, '') NOT IN ('delivered', 'cancelled', 'returned', 'refunded')
					AND COALESCE(f.status, '') NOT IN ('delivered', 'cancelled')) AS physical_allocated,
				COUNT(*) FILTER (WHERE iu.status = 'warehouse' AND oia.id IS NOT NULL
					AND (COALESCE(o.status, '') IN ('delivered', 'cancelled', 'returned', 'refunded')
					     OR COALESCE(f.status, '') IN ('delivered', 'cancelled'))) AS physical_stale_allocated,
				COUNT(*) FILTER (WHERE iu.status = 'warehouse' AND oia.id IS NOT NULL AND oia.picked_at IS NOT NULL
					AND COALESCE(o.status, '') NOT IN ('delivered', 'cancelled', 'returned', 'refunded')
					AND COALESCE(f.status, '') NOT IN ('delivered', 'cancelled')) AS physical_picked,
				COUNT(*) FILTER (WHERE iu.status = 'expected') AS physical_expected,
				COUNT(*) FILTER (WHERE iu.status = 'damaged') AS physical_damaged,
				COUNT(*) FILTER (WHERE iu.status = 'written_off') AS physical_written_off,
				COUNT(*) FILTER (WHERE iu.status = 'shipped') AS physical_shipped
			FROM inventory_units iu
			LEFT JOIN order_item_allocations oia
				ON oia.inventory_unit_id = iu.id
				AND oia.released_at IS NULL
			LEFT JOIN order_items oi ON oi.id = oia.order_item_id
			LEFT JOIN orders o ON o.id = oi.order_id
			LEFT JOIN order_fulfillments f ON f.id = oi.order_fulfillment_id
			WHERE iu.product_variant_id = i.product_variant_id
		) u ON true
		WHERE i.product_variant_id = $1
	`
	var item AdminInventoryItem
	var pSlug string
	var pMainImg, size, color, sku, sellerSku, barcode *string
	var brandName *string
	var cAt, uAt time.Time
	var pWh, pAlloc, pStaleAlloc, pPicked, pExp, pDam, pWoff, pShip int

	err := r.db.QueryRow(ctx, query, id).Scan(
		&item.ID, &item.ProductID, &item.ProductVariantID,
		&item.ProductTitle, &pSlug, &pMainImg,
		&size, &color, &sku, &sellerSku, &barcode,
		&item.SellerID, &brandName,
		&item.Source,
		&item.TotalStock, &item.ReservedStock, &cAt, &uAt,
		&pWh, &pAlloc, &pStaleAlloc, &pPicked, &pExp, &pDam, &pWoff, &pShip,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrInventoryItemNotFound
		}
		return nil, err
	}

	aggTotal := item.TotalStock
	aggReserved := item.ReservedStock
	aggAvailable := aggTotal - aggReserved
	if aggAvailable < 0 {
		aggAvailable = 0
	}
	item.AvailableStock = aggAvailable

	agg := AggregateStock{
		Total:     aggTotal,
		Reserved:  aggReserved,
		Available: aggAvailable,
	}

	physFree := pWh - pAlloc
	if physFree < 0 {
		physFree = 0
	}
	phys := PhysicalStock{
		Warehouse:      pWh,
		Allocated:      pAlloc,
		Picked:         pPicked,
		Free:           physFree,
		Expected:       pExp,
		Damaged:        pDam,
		WrittenOff:     pWoff,
		Shipped:        pShip,
		StaleAllocated: pStaleAlloc,
	}

	legOnHand := aggTotal - pWh
	legReserved := aggReserved - pAlloc
	legAvailable := legOnHand - legReserved
	leg := LegacyStock{
		OnHand:    legOnHand,
		Reserved:  legReserved,
		Available: legAvailable,
	}

	mode, health := EvaluateInventoryHealth(agg, phys, leg)
	item.Aggregate = agg
	item.Physical = phys
	item.Legacy = leg
	item.AccountingMode = mode
	item.Health = health

	item.CreatedAt = cAt.Format(time.RFC3339)
	item.UpdatedAt = uAt.Format(time.RFC3339)

	variantParts := []string{}
	if sku != nil && *sku != "" {
		variantParts = append(variantParts, *sku)
	}
	if size != nil && *size != "" {
		variantParts = append(variantParts, *size)
	}
	if color != nil && *color != "" {
		variantParts = append(variantParts, *color)
	}

	item.VariantLabel = ""
	for idx, part := range variantParts {
		if idx > 0 {
			item.VariantLabel += " / "
		}
		item.VariantLabel += part
	}

	item.Product = ProductInfo{
		ID:           item.ProductID,
		Title:        item.ProductTitle,
		Slug:         pSlug,
		MainImageURL: pMainImg,
	}

	skuVal := ""
	if sku != nil {
		skuVal = *sku
	}
	sellerSkuVal := ""
	if sellerSku != nil {
		sellerSkuVal = *sellerSku
	}
	barcodeVal := ""
	if barcode != nil {
		barcodeVal = *barcode
	}
	sizeVal := ""
	if size != nil {
		sizeVal = *size
	}
	colorVal := ""
	if color != nil {
		colorVal = *color
	}

	item.Variant = VariantInfo{
		ID:        item.ProductVariantID,
		SKU:       skuVal,
		SellerSKU: sellerSkuVal,
		Barcode:   barcodeVal,
		Size:      sizeVal,
		Color:     colorVal,
		Label:     item.VariantLabel,
	}

	sellerNameVal := item.SellerID.String()
	if brandName != nil && *brandName != "" {
		sellerNameVal = *brandName
	}
	item.Seller = SellerInfo{
		ID:   item.SellerID,
		Name: sellerNameVal,
	}
	item.SellerName = sellerNameVal

	unitsQuery := `
		SELECT
			iu.id,
			iu.unit_code,
			iu.status,
			iu.created_at,
			ss.id AS supply_id,
			ss.supply_number,
			ss.status AS supply_status,
			COALESCE(sc.created_at, srs.completed_at, srs.created_at) AS received_at,
			oia.id AS alloc_id,
			oia.picked_at,
			o.id AS order_id,
			o.order_number,
			o.status AS order_status,
			f.id AS fulfillment_id,
			f.status AS fulfillment_status
		FROM inventory_units iu
		LEFT JOIN seller_supplies ss ON ss.id = iu.origin_supply_id
		LEFT JOIN supply_receiving_sessions srs ON srs.id = iu.receiving_session_id
		LEFT JOIN LATERAL (
			SELECT created_at FROM supply_receiving_scans WHERE inventory_unit_id = iu.id ORDER BY created_at DESC LIMIT 1
		) sc ON true
		LEFT JOIN order_item_allocations oia ON oia.inventory_unit_id = iu.id AND oia.released_at IS NULL
		LEFT JOIN order_items oi ON oi.id = oia.order_item_id
		LEFT JOIN orders o ON o.id = oi.order_id
		LEFT JOIN order_fulfillments f ON f.id = oi.order_fulfillment_id
		WHERE iu.product_variant_id = $1
		ORDER BY iu.created_at DESC, iu.unit_code ASC
	`
	rows, err := r.db.Query(ctx, unitsQuery, item.ProductVariantID)
	if err == nil {
		defer rows.Close()
		physicalUnits := make([]AdminInventoryPhysicalUnit, 0)
		for rows.Next() {
			var uID uuid.UUID
			var uCode, uStatus string
			var uCreatedAt time.Time
			var supplyID *uuid.UUID
			var supplyNumber, supplyStatus *string
			var receivedAt *time.Time
			var allocID *uuid.UUID
			var pickedAt *time.Time
			var orderID *uuid.UUID
			var orderNumber, orderStatus *string
			var fulfillmentID *uuid.UUID
			var fulfillmentStatus *string

			scanErr := rows.Scan(
				&uID, &uCode, &uStatus, &uCreatedAt,
				&supplyID, &supplyNumber, &supplyStatus, &receivedAt,
				&allocID, &pickedAt,
				&orderID, &orderNumber, &orderStatus,
				&fulfillmentID, &fulfillmentStatus,
			)
			if scanErr != nil {
				continue
			}

			availability := "free"
			isStale := false
			var liveAlloc *AdminInventoryAllocationInfo
			var staleAlloc *AdminInventoryAllocationInfo

			if uStatus == "warehouse" {
				if allocID != nil {
					isTerminalOrder := orderStatus != nil && (*orderStatus == "delivered" || *orderStatus == "cancelled" || *orderStatus == "returned" || *orderStatus == "refunded")
					isTerminalFulfill := fulfillmentStatus != nil && (*fulfillmentStatus == "delivered" || *fulfillmentStatus == "cancelled")

					ordNum := formatOrderNum(orderNumber, orderID)
					ordStat := ""
					if orderStatus != nil {
						ordStat = *orderStatus
					}
					var oID uuid.UUID
					if orderID != nil {
						oID = *orderID
					}

					allocInfo := &AdminInventoryAllocationInfo{
						ID:                *allocID,
						OrderID:           oID,
						OrderNumber:       ordNum,
						OrderStatus:       ordStat,
						FulfillmentID:     fulfillmentID,
						FulfillmentStatus: fulfillmentStatus,
						PickedAt:          pickedAt,
					}

					if isTerminalOrder || isTerminalFulfill {
						isStale = true
						availability = "free"
						staleAlloc = allocInfo
					} else {
						isStale = false
						if pickedAt != nil {
							availability = "picked"
						} else {
							availability = "allocated"
						}
						liveAlloc = allocInfo
					}
				} else {
					availability = "free"
				}
			} else {
				switch uStatus {
				case "expected":
					availability = "unavailable_expected"
				case "damaged":
					availability = "unavailable_damaged"
				case "written_off":
					availability = "unavailable_written_off"
				case "shipped":
					availability = "unavailable_shipped"
				default:
					availability = "unavailable"
				}
				if allocID != nil {
					isTerminalOrder := orderStatus != nil && (*orderStatus == "delivered" || *orderStatus == "cancelled" || *orderStatus == "returned" || *orderStatus == "refunded")
					isTerminalFulfill := fulfillmentStatus != nil && (*fulfillmentStatus == "delivered" || *fulfillmentStatus == "cancelled")
					ordNum := formatOrderNum(orderNumber, orderID)
					ordStat := ""
					if orderStatus != nil {
						ordStat = *orderStatus
					}
					var oID uuid.UUID
					if orderID != nil {
						oID = *orderID
					}
					allocInfo := &AdminInventoryAllocationInfo{
						ID:                *allocID,
						OrderID:           oID,
						OrderNumber:       ordNum,
						OrderStatus:       ordStat,
						FulfillmentID:     fulfillmentID,
						FulfillmentStatus: fulfillmentStatus,
						PickedAt:          pickedAt,
					}
					isStale = false
					if isTerminalOrder || isTerminalFulfill {
						staleAlloc = allocInfo
					} else {
						liveAlloc = allocInfo
					}
				}
			}

			var supplyLineage *AdminInventorySupplyLineage
			if supplyID != nil && supplyNumber != nil && supplyStatus != nil {
				supplyLineage = &AdminInventorySupplyLineage{
					SupplyID:     *supplyID,
					SupplyNumber: *supplyNumber,
					SupplyStatus: *supplyStatus,
					ReceivedAt:   receivedAt,
				}
			}

			physicalUnits = append(physicalUnits, AdminInventoryPhysicalUnit{
				ID:                uID,
				UnitCode:          uCode,
				Status:            uStatus,
				CreatedAt:         uCreatedAt,
				Availability:      availability,
				IsStaleAllocation: isStale,
				LiveAllocation:    liveAlloc,
				StaleAllocation:   staleAlloc,
				SupplyLineage:     supplyLineage,
			})
		}
		item.PhysicalUnits = physicalUnits
	}

	return &item, nil
}

func (r *Repository) DuplicateGetAdminInventoryItemRichByVariantID(ctx context.Context, id uuid.UUID) (*AdminInventoryItem, error) {
	query := `
		SELECT
			i.id, i.product_id, i.product_variant_id,
			p.title, p.slug, p.main_image_url,
			pv.size, pv.color, pv.sku, pv.seller_sku, pv.barcode,
			i.seller_id, su.brand_name,
			CASE WHEN i.seller_id::text = '` + common.PlatformSellerIDStr + `' THEN 'auction_direct_sale' ELSE 'seller' END,
			i.total_stock, i.reserved_stock, i.created_at, i.updated_at,
			COALESCE(u.physical_warehouse, 0),
			COALESCE(u.physical_allocated, 0),
			COALESCE(u.physical_stale_allocated, 0),
			COALESCE(u.physical_picked, 0),
			COALESCE(u.physical_expected, 0),
			COALESCE(u.physical_damaged, 0),
			COALESCE(u.physical_written_off, 0),
			COALESCE(u.physical_shipped, 0)
		FROM inventory_items i
		JOIN products p ON i.product_id = p.id
		JOIN product_variants pv ON i.product_variant_id = pv.id
		LEFT JOIN sellers su ON i.seller_id = su.id
		LEFT JOIN LATERAL (
			SELECT
				COUNT(*) FILTER (WHERE iu.status = 'warehouse') AS physical_warehouse,
				COUNT(*) FILTER (WHERE iu.status = 'warehouse' AND oia.id IS NOT NULL
					AND COALESCE(o.status, '') NOT IN ('delivered', 'cancelled', 'returned', 'refunded')
					AND COALESCE(f.status, '') NOT IN ('delivered', 'cancelled')) AS physical_allocated,
				COUNT(*) FILTER (WHERE iu.status = 'warehouse' AND oia.id IS NOT NULL
					AND (COALESCE(o.status, '') IN ('delivered', 'cancelled', 'returned', 'refunded')
					     OR COALESCE(f.status, '') IN ('delivered', 'cancelled'))) AS physical_stale_allocated,
				COUNT(*) FILTER (WHERE iu.status = 'warehouse' AND oia.id IS NOT NULL AND oia.picked_at IS NOT NULL
					AND COALESCE(o.status, '') NOT IN ('delivered', 'cancelled', 'returned', 'refunded')
					AND COALESCE(f.status, '') NOT IN ('delivered', 'cancelled')) AS physical_picked,
				COUNT(*) FILTER (WHERE iu.status = 'expected') AS physical_expected,
				COUNT(*) FILTER (WHERE iu.status = 'damaged') AS physical_damaged,
				COUNT(*) FILTER (WHERE iu.status = 'written_off') AS physical_written_off,
				COUNT(*) FILTER (WHERE iu.status = 'shipped') AS physical_shipped
			FROM inventory_units iu
			LEFT JOIN order_item_allocations oia
				ON oia.inventory_unit_id = iu.id
				AND oia.released_at IS NULL
			LEFT JOIN order_items oi ON oi.id = oia.order_item_id
			LEFT JOIN orders o ON o.id = oi.order_id
			LEFT JOIN order_fulfillments f ON f.id = oi.order_fulfillment_id
			WHERE iu.product_variant_id = i.product_variant_id
		) u ON true
		WHERE i.product_variant_id = $1
	`
	var item AdminInventoryItem
	var pSlug string
	var pMainImg, size, color, sku, sellerSku, barcode *string
	var brandName *string
	var cAt, uAt time.Time
	var pWh, pAlloc, pStaleAlloc, pPicked, pExp, pDam, pWoff, pShip int

	err := r.db.QueryRow(ctx, query, id).Scan(
		&item.ID, &item.ProductID, &item.ProductVariantID,
		&item.ProductTitle, &pSlug, &pMainImg,
		&size, &color, &sku, &sellerSku, &barcode,
		&item.SellerID, &brandName,
		&item.Source,
		&item.TotalStock, &item.ReservedStock, &cAt, &uAt,
		&pWh, &pAlloc, &pStaleAlloc, &pPicked, &pExp, &pDam, &pWoff, &pShip,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrInventoryItemNotFound
		}
		return nil, err
	}

	aggTotal := item.TotalStock
	aggReserved := item.ReservedStock
	aggAvailable := aggTotal - aggReserved
	if aggAvailable < 0 {
		aggAvailable = 0
	}
	item.AvailableStock = aggAvailable

	agg := AggregateStock{
		Total:     aggTotal,
		Reserved:  aggReserved,
		Available: aggAvailable,
	}

	physFree := pWh - pAlloc
	if physFree < 0 {
		physFree = 0
	}
	phys := PhysicalStock{
		Warehouse:      pWh,
		Allocated:      pAlloc,
		Picked:         pPicked,
		Free:           physFree,
		Expected:       pExp,
		Damaged:        pDam,
		WrittenOff:     pWoff,
		Shipped:        pShip,
		StaleAllocated: pStaleAlloc,
	}

	legOnHand := aggTotal - pWh
	legReserved := aggReserved - pAlloc
	legAvailable := legOnHand - legReserved
	leg := LegacyStock{
		OnHand:    legOnHand,
		Reserved:  legReserved,
		Available: legAvailable,
	}

	mode, health := EvaluateInventoryHealth(agg, phys, leg)
	item.Aggregate = agg
	item.Physical = phys
	item.Legacy = leg
	item.AccountingMode = mode
	item.Health = health

	item.CreatedAt = cAt.Format(time.RFC3339)
	item.UpdatedAt = uAt.Format(time.RFC3339)

	variantParts := []string{}
	if sku != nil && *sku != "" {
		variantParts = append(variantParts, *sku)
	}
	if size != nil && *size != "" {
		variantParts = append(variantParts, *size)
	}
	if color != nil && *color != "" {
		variantParts = append(variantParts, *color)
	}

	item.VariantLabel = ""
	for idx, part := range variantParts {
		if idx > 0 {
			item.VariantLabel += " / "
		}
		item.VariantLabel += part
	}

	item.Product = ProductInfo{
		ID:           item.ProductID,
		Title:        item.ProductTitle,
		Slug:         pSlug,
		MainImageURL: pMainImg,
	}

	skuVal := ""
	if sku != nil {
		skuVal = *sku
	}
	sellerSkuVal := ""
	if sellerSku != nil {
		sellerSkuVal = *sellerSku
	}
	barcodeVal := ""
	if barcode != nil {
		barcodeVal = *barcode
	}
	sizeVal := ""
	if size != nil {
		sizeVal = *size
	}
	colorVal := ""
	if color != nil {
		colorVal = *color
	}

	item.Variant = VariantInfo{
		ID:        item.ProductVariantID,
		SKU:       skuVal,
		SellerSKU: sellerSkuVal,
		Barcode:   barcodeVal,
		Size:      sizeVal,
		Color:     colorVal,
		Label:     item.VariantLabel,
	}

	sellerNameVal := item.SellerID.String()
	if brandName != nil && *brandName != "" {
		sellerNameVal = *brandName
	}
	item.Seller = SellerInfo{
		ID:   item.SellerID,
		Name: sellerNameVal,
	}
	item.SellerName = sellerNameVal

	unitsQuery := `
		SELECT
			iu.id,
			iu.unit_code,
			iu.status,
			iu.created_at,
			ss.id AS supply_id,
			ss.supply_number,
			ss.status AS supply_status,
			COALESCE(sc.created_at, srs.completed_at, srs.created_at) AS received_at,
			oia.id AS alloc_id,
			oia.picked_at,
			o.id AS order_id,
			o.order_number,
			o.status AS order_status,
			f.id AS fulfillment_id,
			f.status AS fulfillment_status
		FROM inventory_units iu
		LEFT JOIN seller_supplies ss ON ss.id = iu.origin_supply_id
		LEFT JOIN supply_receiving_sessions srs ON srs.id = iu.receiving_session_id
		LEFT JOIN LATERAL (
			SELECT created_at FROM supply_receiving_scans WHERE inventory_unit_id = iu.id ORDER BY created_at DESC LIMIT 1
		) sc ON true
		LEFT JOIN order_item_allocations oia ON oia.inventory_unit_id = iu.id AND oia.released_at IS NULL
		LEFT JOIN order_items oi ON oi.id = oia.order_item_id
		LEFT JOIN orders o ON o.id = oi.order_id
		LEFT JOIN order_fulfillments f ON f.id = oi.order_fulfillment_id
		WHERE iu.product_variant_id = $1
		ORDER BY iu.created_at DESC, iu.unit_code ASC
	`
	rows, err := r.db.Query(ctx, unitsQuery, item.ProductVariantID)
	if err == nil {
		defer rows.Close()
		physicalUnits := make([]AdminInventoryPhysicalUnit, 0)
		for rows.Next() {
			var uID uuid.UUID
			var uCode, uStatus string
			var uCreatedAt time.Time
			var supplyID *uuid.UUID
			var supplyNumber, supplyStatus *string
			var receivedAt *time.Time
			var allocID *uuid.UUID
			var pickedAt *time.Time
			var orderID *uuid.UUID
			var orderNumber, orderStatus *string
			var fulfillmentID *uuid.UUID
			var fulfillmentStatus *string

			scanErr := rows.Scan(
				&uID, &uCode, &uStatus, &uCreatedAt,
				&supplyID, &supplyNumber, &supplyStatus, &receivedAt,
				&allocID, &pickedAt,
				&orderID, &orderNumber, &orderStatus,
				&fulfillmentID, &fulfillmentStatus,
			)
			if scanErr != nil {
				continue
			}

			availability := "free"
			isStale := false
			var liveAlloc *AdminInventoryAllocationInfo
			var staleAlloc *AdminInventoryAllocationInfo

			if uStatus == "warehouse" {
				if allocID != nil {
					isTerminalOrder := orderStatus != nil && (*orderStatus == "delivered" || *orderStatus == "cancelled" || *orderStatus == "returned" || *orderStatus == "refunded")
					isTerminalFulfill := fulfillmentStatus != nil && (*fulfillmentStatus == "delivered" || *fulfillmentStatus == "cancelled")

					ordNum := formatOrderNum(orderNumber, orderID)
					ordStat := ""
					if orderStatus != nil {
						ordStat = *orderStatus
					}
					var oID uuid.UUID
					if orderID != nil {
						oID = *orderID
					}

					allocInfo := &AdminInventoryAllocationInfo{
						ID:                *allocID,
						OrderID:           oID,
						OrderNumber:       ordNum,
						OrderStatus:       ordStat,
						FulfillmentID:     fulfillmentID,
						FulfillmentStatus: fulfillmentStatus,
						PickedAt:          pickedAt,
					}

					if isTerminalOrder || isTerminalFulfill {
						isStale = true
						availability = "free"
						staleAlloc = allocInfo
					} else {
						isStale = false
						if pickedAt != nil {
							availability = "picked"
						} else {
							availability = "allocated"
						}
						liveAlloc = allocInfo
					}
				} else {
					availability = "free"
				}
			} else {
				switch uStatus {
				case "expected":
					availability = "unavailable_expected"
				case "damaged":
					availability = "unavailable_damaged"
				case "written_off":
					availability = "unavailable_written_off"
				case "shipped":
					availability = "unavailable_shipped"
				default:
					availability = "unavailable"
				}
				if allocID != nil {
					isTerminalOrder := orderStatus != nil && (*orderStatus == "delivered" || *orderStatus == "cancelled" || *orderStatus == "returned" || *orderStatus == "refunded")
					isTerminalFulfill := fulfillmentStatus != nil && (*fulfillmentStatus == "delivered" || *fulfillmentStatus == "cancelled")
					ordNum := formatOrderNum(orderNumber, orderID)
					ordStat := ""
					if orderStatus != nil {
						ordStat = *orderStatus
					}
					var oID uuid.UUID
					if orderID != nil {
						oID = *orderID
					}
					allocInfo := &AdminInventoryAllocationInfo{
						ID:                *allocID,
						OrderID:           oID,
						OrderNumber:       ordNum,
						OrderStatus:       ordStat,
						FulfillmentID:     fulfillmentID,
						FulfillmentStatus: fulfillmentStatus,
						PickedAt:          pickedAt,
					}
					isStale = false
					if isTerminalOrder || isTerminalFulfill {
						staleAlloc = allocInfo
					} else {
						liveAlloc = allocInfo
					}
				}
			}

			var supplyLineage *AdminInventorySupplyLineage
			if supplyID != nil && supplyNumber != nil && supplyStatus != nil {
				supplyLineage = &AdminInventorySupplyLineage{
					SupplyID:     *supplyID,
					SupplyNumber: *supplyNumber,
					SupplyStatus: *supplyStatus,
					ReceivedAt:   receivedAt,
				}
			}

			physicalUnits = append(physicalUnits, AdminInventoryPhysicalUnit{
				ID:                uID,
				UnitCode:          uCode,
				Status:            uStatus,
				CreatedAt:         uCreatedAt,
				Availability:      availability,
				IsStaleAllocation: isStale,
				LiveAllocation:    liveAlloc,
				StaleAllocation:   staleAlloc,
				SupplyLineage:     supplyLineage,
			})
		}
		item.PhysicalUnits = physicalUnits
	}

	return &item, nil
}
