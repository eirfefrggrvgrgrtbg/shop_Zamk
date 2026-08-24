package supplies

import (
	"context"
	"errors"
	"fmt"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type Repository struct {
	db                postgres.DBTX
	unitCodeGenerator func() (string, error)
}

func NewRepository(db postgres.DBTX) *Repository {
	return &Repository{
		db:                db,
		unitCodeGenerator: GenerateUnitCode,
	}
}

func (r *Repository) WithTx(tx pgx.Tx) *Repository {
	return &Repository{
		db:                tx,
		unitCodeGenerator: r.unitCodeGenerator,
	}
}

func (r *Repository) SetUnitCodeGeneratorForTest(fn func() (string, error)) {
	r.unitCodeGenerator = fn
}

func (r *Repository) GenerateSupplyNumber(ctx context.Context) (string, error) {
	// Simple sequence-based generation
	query := `SELECT nextval('supply_number_seq')`
	var seq int
	err := r.db.QueryRow(ctx, query).Scan(&seq)
	if err != nil {
		return "", fmt.Errorf("failed to get sequence: %w", err)
	}
	return fmt.Sprintf("SUP-%06d", seq), nil
}

func (r *Repository) CreateSupply(ctx context.Context, supply *Supply) error {
	query := `
		INSERT INTO seller_supplies (
			id, supply_number, seller_id, status, handoff_method, carrier_name, tracking_number, expected_arrival_date,
			qr_token, created_at, shipped_at, arrived_at, receiving_started_at, completed_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
	`
	_, err := r.db.Exec(ctx, query,
		supply.ID, supply.SupplyNumber, supply.SellerID, supply.Status, supply.HandoffMethod, supply.CarrierName,
		supply.TrackingNumber, supply.ExpectedArrivalDate, supply.QRToken, supply.CreatedAt, supply.ShippedAt,
		supply.ArrivedAt, supply.ReceivingStartedAt, supply.CompletedAt, supply.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to insert supply: %w", err)
	}

	for _, item := range supply.Items {
		queryItem := `
			INSERT INTO seller_supply_items (
				id, supply_id, variant_id, expected_quantity, accepted_quantity, damaged_quantity, missing_quantity, extra_quantity, receiving_comment, created_at, updated_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		`
		_, err := r.db.Exec(ctx, queryItem,
			item.ID, item.SupplyID, item.VariantID, item.ExpectedQuantity, item.AcceptedQuantity, item.DamagedQuantity, item.MissingQuantity, item.ExtraQuantity, item.ReceivingComment, item.CreatedAt, item.UpdatedAt,
		)
		if err != nil {
			return fmt.Errorf("failed to insert supply item: %w", err)
		}
	}

	for _, box := range supply.Boxes {
		queryBox := `
			INSERT INTO seller_supply_boxes (id, supply_id, box_number, qr_token, created_at)
			VALUES ($1, $2, $3, $4, $5)
		`
		_, err := r.db.Exec(ctx, queryBox, box.ID, box.SupplyID, box.BoxNumber, box.QRToken, box.CreatedAt)
		if err != nil {
			return fmt.Errorf("failed to insert supply box: %w", err)
		}

		for _, bi := range box.Items {
			queryBi := `
				INSERT INTO seller_supply_box_items (box_id, supply_item_id, quantity)
				VALUES ($1, $2, $3)
			`
			_, err := r.db.Exec(ctx, queryBi, bi.BoxID, bi.SupplyItemID, bi.Quantity)
			if err != nil {
				return fmt.Errorf("failed to insert box item: %w", err)
			}
		}
	}

	for _, unit := range supply.InventoryUnits {
		queryUnit := `
			INSERT INTO inventory_units (
				id, unit_code, product_variant_id, origin_supply_id, origin_supply_item_id, 
				origin_box_id, unit_index, status, created_at, updated_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		`
		
		// Retry loop for extremely unlikely collision
		const maxRetries = 3
		var insertErr error
		for retry := 0; retry < maxRetries; retry++ {
			spName := fmt.Sprintf("sp_unit_%d_%d", unit.UnitIndex, retry)
			_, _ = r.db.Exec(ctx, "SAVEPOINT "+spName)

			_, insertErr = r.db.Exec(ctx, queryUnit,
				unit.ID, unit.UnitCode, unit.ProductVariantID, unit.OriginSupplyID, unit.OriginSupplyItemID,
				unit.OriginBoxID, unit.UnitIndex, unit.Status, unit.CreatedAt, unit.UpdatedAt,
			)
			if insertErr == nil {
				_, _ = r.db.Exec(ctx, "RELEASE SAVEPOINT "+spName)
				break
			}

			var pgErr *pgconn.PgError
			if errors.As(insertErr, &pgErr) && pgErr.Code == "23505" && (pgErr.ConstraintName == "idx_inventory_units_unit_code" || pgErr.ConstraintName == "inventory_units_unit_code_key") {
				_, _ = r.db.Exec(ctx, "ROLLBACK TO SAVEPOINT "+spName)
				genFn := r.unitCodeGenerator
				if genFn == nil {
					genFn = GenerateUnitCode
				}
				newCode, genErr := genFn()
				if genErr != nil {
					return fmt.Errorf("failed to generate unit code on collision retry: %w", genErr)
				}
				unit.UnitCode = newCode
				continue
			}
			_, _ = r.db.Exec(ctx, "ROLLBACK TO SAVEPOINT "+spName)
			break
		}
		if insertErr != nil {
			return fmt.Errorf("failed to insert inventory unit: %w", insertErr)
		}
	}

	return nil
}

func (r *Repository) ListUnitsBySupplyID(ctx context.Context, supplyID uuid.UUID) ([]InventoryUnit, error) {
	query := `
		SELECT id, unit_code, product_variant_id, origin_supply_id, origin_supply_item_id, origin_box_id, unit_index, external_marking_code, status, receiving_session_id, created_at, updated_at
		FROM inventory_units
		WHERE origin_supply_id = $1
		ORDER BY origin_supply_item_id, unit_index ASC
	`
	rows, err := r.db.Query(ctx, query, supplyID)
	if err != nil {
		return nil, fmt.Errorf("failed to list inventory units: %w", err)
	}
	defer rows.Close()

	var units []InventoryUnit
	for rows.Next() {
		var u InventoryUnit
		err := rows.Scan(
			&u.ID, &u.UnitCode, &u.ProductVariantID, &u.OriginSupplyID, &u.OriginSupplyItemID,
			&u.OriginBoxID, &u.UnitIndex, &u.ExternalMarkingCode, &u.Status, &u.ReceivingSessionID,
			&u.CreatedAt, &u.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan inventory unit: %w", err)
		}
		units = append(units, u)
	}
	return units, nil
}

func (r *Repository) UpdateSupplyStatus(ctx context.Context, supplyID uuid.UUID, status string) error {
	query := `UPDATE seller_supplies SET status = $1, updated_at = now() WHERE id = $2`
	res, err := r.db.Exec(ctx, query, status, supplyID)
	if err != nil {
		return fmt.Errorf("failed to update status: %w", err)
	}
	if res.RowsAffected() == 0 {
		return ErrSupplyNotFound
	}
	return nil
}

func (r *Repository) MarkShipped(ctx context.Context, supplyID uuid.UUID) error {
	query := `UPDATE seller_supplies SET status = 'shipped_by_seller', shipped_at = now(), updated_at = now() WHERE id = $1 AND status = 'ready_to_ship'`
	res, err := r.db.Exec(ctx, query, supplyID)
	if err != nil {
		return fmt.Errorf("failed to mark shipped: %w", err)
	}
	if res.RowsAffected() == 0 {
		return ErrInvalidStatus
	}
	return nil
}

func (r *Repository) GetSupplyByID(ctx context.Context, id uuid.UUID) (*Supply, error) {
	query := `
		SELECT id, supply_number, seller_id, status, handoff_method, carrier_name, tracking_number, expected_arrival_date,
			qr_token, created_at, shipped_at, arrived_at, receiving_started_at, completed_at, updated_at
		FROM seller_supplies
		WHERE id = $1
	`
	var s Supply
	err := r.db.QueryRow(ctx, query, id).Scan(
		&s.ID, &s.SupplyNumber, &s.SellerID, &s.Status, &s.HandoffMethod, &s.CarrierName, &s.TrackingNumber, &s.ExpectedArrivalDate,
		&s.QRToken, &s.CreatedAt, &s.ShippedAt, &s.ArrivedAt, &s.ReceivingStartedAt, &s.CompletedAt, &s.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrSupplyNotFound
		}
		return nil, fmt.Errorf("failed to get supply: %w", err)
	}

	// Fetch items
	itemsQuery := `
		SELECT i.id, i.supply_id, i.variant_id, i.expected_quantity, i.accepted_quantity, i.damaged_quantity, i.missing_quantity, i.extra_quantity, i.receiving_comment, i.created_at, i.updated_at,
		COALESCE(v.seller_sku, v.sku, '') as sku,
		v.seller_sku,
		p.title as product_title,
		v.barcode,
		COALESCE(c.name_ru, v.color) as color_name,
		COALESCE(sv.value, v.size) as size_name
		FROM seller_supply_items i
		JOIN product_variants v ON v.id = i.variant_id
		JOIN products p ON p.id = v.product_id
		LEFT JOIN colors c ON c.id = v.color_id
		LEFT JOIN size_values sv ON sv.id = v.size_value_id
		WHERE i.supply_id = $1
		ORDER BY i.created_at ASC
	`
	rows, err := r.db.Query(ctx, itemsQuery, id)
	if err != nil {
		return nil, fmt.Errorf("failed to list supply items: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var i SupplyItem
		err := rows.Scan(
			&i.ID, &i.SupplyID, &i.VariantID, &i.ExpectedQuantity, &i.AcceptedQuantity, &i.DamagedQuantity, &i.MissingQuantity, &i.ExtraQuantity, &i.ReceivingComment, &i.CreatedAt, &i.UpdatedAt,
			&i.SKU, &i.SellerSKU, &i.ProductTitle, &i.Barcode, &i.ColorName, &i.SizeName,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan item: %w", err)
		}
		s.TotalExpectedItems += i.ExpectedQuantity
		s.TotalAcceptedItems += i.AcceptedQuantity
		s.Items = append(s.Items, i)
	}
	s.SKUCount = len(s.Items)

	// Fetch boxes
	boxesQuery := `
		SELECT id, supply_id, box_number, qr_token, created_at
		FROM seller_supply_boxes
		WHERE supply_id = $1
		ORDER BY created_at ASC
	`
	boxRows, err := r.db.Query(ctx, boxesQuery, id)
	if err != nil {
		return nil, fmt.Errorf("failed to query boxes: %w", err)
	}
	defer boxRows.Close()

	boxes := make([]SupplyBox, 0)
	for boxRows.Next() {
		var b SupplyBox
		err := boxRows.Scan(&b.ID, &b.SupplyID, &b.BoxNumber, &b.QRToken, &b.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan box: %w", err)
		}
		boxes = append(boxes, b)
	}

	for idx, box := range boxes {
		bitemsQuery := `
			SELECT box_id, supply_item_id, quantity
			FROM seller_supply_box_items
			WHERE box_id = $1
		`
		biRows, err := r.db.Query(ctx, bitemsQuery, box.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to query box items: %w", err)
		}
		defer biRows.Close()

		for biRows.Next() {
			var bi SupplyBoxItem
			err := biRows.Scan(&bi.BoxID, &bi.SupplyItemID, &bi.Quantity)
			if err != nil {
				return nil, fmt.Errorf("failed to scan box item: %w", err)
			}
			boxes[idx].Items = append(boxes[idx].Items, bi)
		}
	}
	s.Boxes = boxes
	s.TotalExpectedBoxes = len(boxes)

	return &s, nil
}

func (r *Repository) GetSuppliesBySeller(ctx context.Context, sellerID uuid.UUID) ([]Supply, error) {
	query := `
		SELECT s.id, s.supply_number, s.seller_id, s.status, s.handoff_method, s.carrier_name, s.tracking_number, s.expected_arrival_date,
			s.qr_token, s.created_at, s.shipped_at, s.arrived_at, s.receiving_started_at, s.completed_at, s.updated_at,
			COALESCE(SUM(i.expected_quantity), 0)::int AS total_expected_items,
			COALESCE(SUM(i.accepted_quantity), 0)::int AS total_accepted_items,
			COUNT(DISTINCT i.variant_id)::int AS total_sku_count,
			COALESCE((SELECT COUNT(*) FROM seller_supply_boxes b WHERE b.supply_id = s.id), 0)::int AS total_expected_boxes
		FROM seller_supplies s
		LEFT JOIN seller_supply_items i ON i.supply_id = s.id
		WHERE s.seller_id = $1
		GROUP BY s.id
		ORDER BY s.created_at DESC
	`
	rows, err := r.db.Query(ctx, query, sellerID)
	if err != nil {
		return nil, fmt.Errorf("failed to query supplies: %w", err)
	}
	defer rows.Close()

	var supplies []Supply
	for rows.Next() {
		var s Supply
		err := rows.Scan(
			&s.ID, &s.SupplyNumber, &s.SellerID, &s.Status, &s.HandoffMethod, &s.CarrierName, &s.TrackingNumber, &s.ExpectedArrivalDate,
			&s.QRToken, &s.CreatedAt, &s.ShippedAt, &s.ArrivedAt, &s.ReceivingStartedAt, &s.CompletedAt, &s.UpdatedAt,
			&s.TotalExpectedItems, &s.TotalAcceptedItems, &s.SKUCount, &s.TotalExpectedBoxes,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan supply: %w", err)
		}
		supplies = append(supplies, s)
	}

	return supplies, nil
}

func (r *Repository) VerifyVariantsOwnership(ctx context.Context, sellerID uuid.UUID, variantIDs []uuid.UUID) error {
	if len(variantIDs) == 0 {
		return nil
	}
	query := `
		SELECT COUNT(DISTINCT v.id)
		FROM product_variants v
		JOIN products p ON p.id = v.product_id
		WHERE p.seller_id = $1 AND v.id = ANY($2)
	`
	var count int
	err := r.db.QueryRow(ctx, query, sellerID, variantIDs).Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to verify variants: %w", err)
	}
	if count != len(variantIDs) {
		return errors.New("one or more variants do not belong to the seller")
	}
	return nil
}

func (r *Repository) GetSellerIDByUserID(ctx context.Context, userID uuid.UUID) (uuid.UUID, error) {
	query := `
		SELECT seller_id
		FROM seller_users
		WHERE user_id = $1
		LIMIT 1
	`
	var sellerID uuid.UUID
	err := r.db.QueryRow(ctx, query, userID).Scan(&sellerID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, errors.New("seller not found for user")
		}
		return uuid.Nil, fmt.Errorf("failed to get seller ID: %w", err)
	}
	return sellerID, nil
}
