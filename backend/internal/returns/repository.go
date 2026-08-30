package returns

import (
	"context"
	"errors"
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

func (r *Repository) CreateReturnTx(ctx context.Context, tx pgx.Tx, ret *Return, items []ReturnItem) error {
	query := `
		INSERT INTO returns (id, order_id, fulfillment_id, user_id, status, reason, comment)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING created_at, updated_at
	`
	err := tx.QueryRow(ctx, query, ret.ID, ret.OrderID, ret.FulfillmentID, ret.UserID, ret.Status, ret.Reason, ret.Comment).Scan(&ret.CreatedAt, &ret.UpdatedAt)
	if err != nil {
		return err
	}

	for i := range items {
		itemQuery := `
			INSERT INTO return_items (id, return_id, order_item_id, quantity, reason, condition, restock, accepted_quantity, damaged_quantity, rejected_quantity)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
			RETURNING created_at
		`
		err = tx.QueryRow(ctx, itemQuery, items[i].ID, items[i].ReturnID, items[i].OrderItemID, items[i].Quantity, items[i].Reason, items[i].Condition, items[i].Restock, items[i].AcceptedQuantity, items[i].DamagedQuantity, items[i].RejectedQuantity).Scan(&items[i].CreatedAt)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) UpdateReturnTx(ctx context.Context, tx pgx.Tx, ret *Return) error {
	query := `
		UPDATE returns
		SET status = $1, admin_comment = $2, updated_at = now(), approved_at = $3, rejected_at = $4, completed_at = $5, receiving_started_at = $7
		WHERE id = $6
		RETURNING updated_at
	`
	return tx.QueryRow(ctx, query, ret.Status, ret.AdminComment, ret.ApprovedAt, ret.RejectedAt, ret.CompletedAt, ret.ID, ret.ReceivingStartedAt).Scan(&ret.UpdatedAt)
}

func (r *Repository) UpdateReturnItemRestockTx(ctx context.Context, tx pgx.Tx, itemID uuid.UUID, restock bool) error {
	query := `UPDATE return_items SET restock = $1 WHERE id = $2`
	_, err := tx.Exec(ctx, query, restock, itemID)
	return err
}

func (r *Repository) GetReturn(ctx context.Context, id uuid.UUID) (*Return, []ReturnItem, error) {
	query := `
		SELECT id, order_id, fulfillment_id, user_id, status, reason, comment, admin_comment, created_at, updated_at, approved_at, rejected_at, completed_at, receiving_started_at
		FROM returns WHERE id = $1
	`
	var ret Return
	err := r.db.QueryRow(ctx, query, id).Scan(
		&ret.ID, &ret.OrderID, &ret.FulfillmentID, &ret.UserID, &ret.Status, &ret.Reason, &ret.Comment, &ret.AdminComment,
		&ret.CreatedAt, &ret.UpdatedAt, &ret.ApprovedAt, &ret.RejectedAt, &ret.CompletedAt, &ret.ReceivingStartedAt,
	)
	if err != nil {
		fmt.Printf("GetReturn %v err: %v\n", id, err)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, ErrReturnNotFound
		}
		return nil, nil, err
	}

	itemsQuery := `
		SELECT id, return_id, order_item_id, quantity, reason, condition, restock, accepted_quantity, damaged_quantity, rejected_quantity, created_at
		FROM return_items WHERE return_id = $1
	`
	rows, err := r.db.Query(ctx, itemsQuery, id)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	var items []ReturnItem
	for rows.Next() {
		var item ReturnItem
		if err := rows.Scan(&item.ID, &item.ReturnID, &item.OrderItemID, &item.Quantity, &item.Reason, &item.Condition, &item.Restock, &item.AcceptedQuantity, &item.DamagedQuantity, &item.RejectedQuantity, &item.CreatedAt); err != nil {
			return nil, nil, err
		}
		items = append(items, item)
	}
	if items == nil {
		items = make([]ReturnItem, 0)
	}

	return &ret, items, nil
}

func (r *Repository) GetReturnTx(ctx context.Context, tx pgx.Tx, id uuid.UUID) (*Return, error) {
	query := `
		SELECT id, order_id, fulfillment_id, user_id, status, reason, comment, admin_comment, created_at, updated_at, approved_at, rejected_at, completed_at, receiving_started_at
		FROM returns WHERE id = $1 FOR UPDATE
	`
	var ret Return
	err := tx.QueryRow(ctx, query, id).Scan(
		&ret.ID, &ret.OrderID, &ret.FulfillmentID, &ret.UserID, &ret.Status, &ret.Reason,
		&ret.Comment, &ret.AdminComment, &ret.CreatedAt, &ret.UpdatedAt,
		&ret.ApprovedAt, &ret.RejectedAt, &ret.CompletedAt, &ret.ReceivingStartedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrReturnNotFound
		}
		return nil, err
	}
	return &ret, nil
}

func (r *Repository) ListReturnsByCustomer(ctx context.Context, userID uuid.UUID, limit, offset int) ([]Return, error) {
	query := `
		SELECT id, order_id, fulfillment_id, user_id, status, reason, comment, admin_comment, created_at, updated_at, approved_at, rejected_at, completed_at, receiving_started_at
		FROM returns WHERE user_id = $1 ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := r.db.Query(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []Return
	for rows.Next() {
		var ret Return
		if err := rows.Scan(&ret.ID, &ret.OrderID, &ret.FulfillmentID, &ret.UserID, &ret.Status, &ret.Reason, &ret.Comment, &ret.AdminComment, &ret.CreatedAt, &ret.UpdatedAt, &ret.ApprovedAt, &ret.RejectedAt, &ret.CompletedAt, &ret.ReceivingStartedAt); err != nil {
			return nil, err
		}
		list = append(list, ret)
	}
	if list == nil {
		list = make([]Return, 0)
	}
	return list, nil
}

func (r *Repository) ListAllReturns(ctx context.Context, limit, offset int) ([]Return, error) {
	query := `
		SELECT id, order_id, fulfillment_id, user_id, status, reason, comment, admin_comment, created_at, updated_at, approved_at, rejected_at, completed_at, receiving_started_at
		FROM returns ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`
	rows, err := r.db.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []Return
	for rows.Next() {
		var ret Return
		if err := rows.Scan(&ret.ID, &ret.OrderID, &ret.FulfillmentID, &ret.UserID, &ret.Status, &ret.Reason, &ret.Comment, &ret.AdminComment, &ret.CreatedAt, &ret.UpdatedAt, &ret.ApprovedAt, &ret.RejectedAt, &ret.CompletedAt, &ret.ReceivingStartedAt); err != nil {
			return nil, err
		}
		list = append(list, ret)
	}
	if list == nil {
		list = make([]Return, 0)
	}
	return list, nil
}

func (r *Repository) GetSellerReturnItems(ctx context.Context, sellerID uuid.UUID, limit, offset int) ([]SellerReturnItem, error) {
	query := `
		SELECT
			ri.id, ri.return_id, r.order_id, o.order_number, ri.order_item_id,
			r.status, ri.quantity, ri.reason, ri.condition,
			oi.title, oi.variant_size, oi.variant_color, oi.sku, oi.image_url, oi.price_cents, (oi.price_cents * ri.quantity),
			ri.restock, r.admin_comment,
			(SELECT amount_cents FROM seller_ledger_entries sle WHERE sle.order_item_id = oi.id AND sle.type = 'adjustment' AND sle.metadata->>'return_id' = r.id::text LIMIT 1),
			(SELECT CASE
				WHEN sle.metadata->>'reason' = 'return_post_payout' THEN 'debt'
				WHEN sle.available_at IS NULL THEN 'debt'
				WHEN sle.available_at > now() THEN 'frozen'
				ELSE 'available' END
			FROM seller_ledger_entries sle WHERE sle.order_item_id = oi.id AND sle.type = 'adjustment' AND sle.metadata->>'return_id' = r.id::text LIMIT 1),
			r.created_at, r.updated_at
		FROM return_items ri
		JOIN returns r ON r.id = ri.return_id
		JOIN order_items oi ON oi.id = ri.order_item_id
		JOIN orders o ON o.id = r.order_id
		WHERE oi.seller_id = $1
		ORDER BY r.created_at DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := r.db.Query(ctx, query, sellerID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []SellerReturnItem
	for rows.Next() {
		var item SellerReturnItem
		if err := rows.Scan(&item.ReturnItemID, &item.ReturnID, &item.OrderID, &item.OrderNumber, &item.OrderItemID, &item.Status, &item.Quantity, &item.Reason, &item.Condition, &item.ProductTitle, &item.VariantSize, &item.VariantColor, &item.SKU, &item.ImageURL, &item.PriceCents, &item.SubtotalPriceCents, &item.Restock, &item.AdminComment, &item.FinancialAdjustmentCents, &item.FinancialImpactType, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		list = append(list, item)
	}
	if list == nil {
		list = make([]SellerReturnItem, 0)
	}
	return list, nil
}

func (r *Repository) GetSellerReturnItemsForReturn(ctx context.Context, sellerID, returnID uuid.UUID) ([]SellerReturnItem, error) {
	query := `
		SELECT
			ri.id, ri.return_id, r.order_id, o.order_number, ri.order_item_id,
			r.status, ri.quantity, ri.reason, ri.condition,
			oi.title, oi.variant_size, oi.variant_color, oi.sku, oi.image_url, oi.price_cents, (oi.price_cents * ri.quantity),
			ri.restock, r.admin_comment,
			(SELECT amount_cents FROM seller_ledger_entries sle WHERE sle.order_item_id = oi.id AND sle.type = 'adjustment' AND sle.metadata->>'return_id' = r.id::text LIMIT 1),
			(SELECT CASE
				WHEN sle.metadata->>'reason' = 'return_post_payout' THEN 'debt'
				WHEN sle.available_at IS NULL THEN 'debt'
				WHEN sle.available_at > now() THEN 'frozen'
				ELSE 'available' END
			FROM seller_ledger_entries sle WHERE sle.order_item_id = oi.id AND sle.type = 'adjustment' AND sle.metadata->>'return_id' = r.id::text LIMIT 1),
			r.created_at, r.updated_at
		FROM return_items ri
		JOIN returns r ON r.id = ri.return_id
		JOIN order_items oi ON oi.id = ri.order_item_id
		JOIN orders o ON o.id = r.order_id
		WHERE oi.seller_id = $1 AND r.id = $2
	`
	rows, err := r.db.Query(ctx, query, sellerID, returnID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []SellerReturnItem
	for rows.Next() {
		var item SellerReturnItem
		if err := rows.Scan(&item.ReturnItemID, &item.ReturnID, &item.OrderID, &item.OrderNumber, &item.OrderItemID, &item.Status, &item.Quantity, &item.Reason, &item.Condition, &item.ProductTitle, &item.VariantSize, &item.VariantColor, &item.SKU, &item.ImageURL, &item.PriceCents, &item.SubtotalPriceCents, &item.Restock, &item.AdminComment, &item.FinancialAdjustmentCents, &item.FinancialImpactType, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		list = append(list, item)
	}
	if list == nil {
		list = make([]SellerReturnItem, 0)
	}
	return list, nil
}


func (r *Repository) GetTotalRefundedAmountForOrder(ctx context.Context, orderID uuid.UUID) (int64, error) {
	query := `
		SELECT COALESCE(SUM(amount_cents), 0)
		FROM refunds
		WHERE order_id = $1 AND status IN ('pending', 'processing', 'succeeded')
	`
	var total int64
	err := r.db.QueryRow(ctx, query, orderID).Scan(&total)
	return total, err
}

func (r *Repository) GetRefund(ctx context.Context, id uuid.UUID) (*Refund, error) {
	query := `
		SELECT id, return_id, payment_id, order_id, status, amount_cents, currency, provider, provider_refund_id, reason, created_at, updated_at, processed_at, failed_at
		FROM refunds WHERE id = $1
	`
	var ref Refund
	err := r.db.QueryRow(ctx, query, id).Scan(
		&ref.ID, &ref.ReturnID, &ref.PaymentID, &ref.OrderID, &ref.Status, &ref.AmountCents, &ref.Currency,
		&ref.Provider, &ref.ProviderRefundID, &ref.Reason, &ref.CreatedAt, &ref.UpdatedAt, &ref.ProcessedAt, &ref.FailedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrRefundNotFound
		}
		return nil, err
	}
	return &ref, nil
}

func (r *Repository) ListAllRefunds(ctx context.Context, limit, offset int) ([]Refund, error) {
	query := `
		SELECT id, return_id, payment_id, order_id, status, amount_cents, currency, provider, provider_refund_id, reason, created_at, updated_at, processed_at, failed_at
		FROM refunds ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`
	rows, err := r.db.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []Refund
	for rows.Next() {
		var ref Refund
		if err := rows.Scan(&ref.ID, &ref.ReturnID, &ref.PaymentID, &ref.OrderID, &ref.Status, &ref.AmountCents, &ref.Currency, &ref.Provider, &ref.ProviderRefundID, &ref.Reason, &ref.CreatedAt, &ref.UpdatedAt, &ref.ProcessedAt, &ref.FailedAt); err != nil {
			return nil, err
		}
		list = append(list, ref)
	}
	if list == nil {
		list = make([]Refund, 0)
	}
	return list, nil
}

func (r *Repository) GetTotalReturnedQuantityForOrderItem(ctx context.Context, orderItemID uuid.UUID) (int, error) {
	query := `
		SELECT COALESCE(SUM(ri.quantity), 0)
		FROM return_items ri
		JOIN returns r ON r.id = ri.return_id
		WHERE ri.order_item_id = $1 AND r.status NOT IN ('rejected', 'cancelled')
	`
	var total int
	err := r.db.QueryRow(ctx, query, orderItemID).Scan(&total)
	return total, err
}

func (r *Repository) GetReturnReceivingState(ctx context.Context, returnID uuid.UUID) (*AdminReturnReceivingState, error) {
	queryReturn := `
		SELECT r.id, r.order_id, r.fulfillment_id, r.user_id, r.status, r.reason, r.comment, r.admin_comment, r.created_at, r.updated_at, r.approved_at, r.rejected_at, r.completed_at, r.receiving_started_at, o.order_number
		FROM returns r
		JOIN orders o ON o.id = r.order_id
		WHERE r.id = $1
	`
	var ret Return
	var orderNumber *string
	err := r.db.QueryRow(ctx, queryReturn, returnID).Scan(&ret.ID, &ret.OrderID, &ret.FulfillmentID, &ret.UserID, &ret.Status, &ret.Reason, &ret.Comment, &ret.AdminComment, &ret.CreatedAt, &ret.UpdatedAt, &ret.ApprovedAt, &ret.RejectedAt, &ret.CompletedAt, &ret.ReceivingStartedAt, &orderNumber)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrReturnNotFound
		}
		return nil, err
	}

	queryItems := `
		SELECT id, return_id, order_item_id, quantity, reason, condition, restock, accepted_quantity, damaged_quantity, rejected_quantity, created_at
		FROM return_items WHERE return_id = $1
		ORDER BY created_at ASC
	`
	rows, err := r.db.Query(ctx, queryItems, returnID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var state AdminReturnReceivingState
	state.Return = ret
	state.OrderNumber = orderNumber
	state.Items = make([]AdminReturnReceivingItem, 0)

	var returnItems []ReturnItem
	for rows.Next() {
		var item ReturnItem
		if err := rows.Scan(&item.ID, &item.ReturnID, &item.OrderItemID, &item.Quantity, &item.Reason, &item.Condition, &item.Restock, &item.AcceptedQuantity, &item.DamagedQuantity, &item.RejectedQuantity, &item.CreatedAt); err != nil {
			return nil, err
		}
		returnItems = append(returnItems, item)
	}

	for _, item := range returnItems {
		// 1. Fetch outbound allocations
		queryOutbound := `
			SELECT oia.id, iu.unit_code, oia.picked_at, oia.released_at, iu.status
			FROM order_item_allocations oia
			JOIN inventory_units iu ON iu.id = oia.inventory_unit_id
			WHERE oia.order_item_id = $1
			ORDER BY oia.created_at ASC
		`
		outRows, err := r.db.Query(ctx, queryOutbound, item.OrderItemID)
		if err != nil {
			return nil, err
		}
		var outboundAllocs []OutboundAllocationDetail
		for outRows.Next() {
			var d OutboundAllocationDetail
			if err := outRows.Scan(&d.AllocationID, &d.UnitCode, &d.PickedAt, &d.ReleasedAt, &d.UnitStatus); err != nil {
				outRows.Close()
				return nil, err
			}
			outboundAllocs = append(outboundAllocs, d)
		}
		outRows.Close()
		if outboundAllocs == nil {
			outboundAllocs = make([]OutboundAllocationDetail, 0)
		}

		// 2. Fetch scanned units
		queryUnits := `
			SELECT riu.id, riu.return_item_id, riu.order_item_allocation_id, iu.unit_code, riu.scanned_at, riu.inspected_condition, riu.disposition, riu.created_at, riu.updated_at
			FROM return_item_units riu
			JOIN order_item_allocations oia ON oia.id = riu.order_item_allocation_id
			JOIN inventory_units iu ON iu.id = oia.inventory_unit_id
			WHERE riu.return_item_id = $1
			ORDER BY riu.created_at ASC
		`
		unitRows, err := r.db.Query(ctx, queryUnits, item.ID)
		if err != nil {
			return nil, err
		}
		var scannedUnits []ScannedUnitDetail
		for unitRows.Next() {
			var u ScannedUnitDetail
			if err := unitRows.Scan(&u.ID, &u.ReturnItemID, &u.OrderItemAllocationID, &u.UnitCode, &u.ScannedAt, &u.InspectedCondition, &u.Disposition, &u.CreatedAt, &u.UpdatedAt); err != nil {
				unitRows.Close()
				return nil, err
			}
			scannedUnits = append(scannedUnits, u)
		}
		unitRows.Close()
		if scannedUnits == nil {
			scannedUnits = make([]ScannedUnitDetail, 0)
		}

		allocMode := "serialized"
		if len(outboundAllocs) == 0 {
			allocMode = "legacy"
			state.LegacyRequested += item.Quantity
		} else {
			state.SerializedRequested += item.Quantity
			state.SerializedScanned += len(scannedUnits)
		}

		scannedQty := len(scannedUnits)
		remainingQty := item.Quantity - scannedQty

		state.TotalRequested += item.Quantity
		state.TotalScanned += scannedQty
		state.TotalRemaining += remainingQty

		state.Items = append(state.Items, AdminReturnReceivingItem{
			ReturnItem:          item,
			AllocationMode:      allocMode,
			OutboundAllocations: outboundAllocs,
			ScannedUnits:        scannedUnits,
			RequestedQuantity:   item.Quantity,
			ScannedQuantity:     scannedQty,
			RemainingQuantity:   remainingQty,
		})
	}

	return &state, nil
}

type AllocationLookupResult struct {
	OrderItemAllocationID uuid.UUID
	OrderItemID           uuid.UUID
	FulfillmentID         uuid.UUID
	OrderID               uuid.UUID
	UnitStatus            string
	PickedAt              *time.Time
	ReleasedAt            *time.Time
}

func (r *Repository) GetAllocationByZMUCode(ctx context.Context, code string) (*AllocationLookupResult, error) {
	query := `
		SELECT oia.id, oia.order_item_id, oi.order_fulfillment_id, oi.order_id, iu.status, oia.picked_at, oia.released_at
		FROM inventory_units iu
		JOIN order_item_allocations oia ON oia.inventory_unit_id = iu.id
		JOIN order_items oi ON oi.id = oia.order_item_id
		WHERE iu.unit_code = $1
		ORDER BY oia.created_at DESC
		LIMIT 1
	`
	var res AllocationLookupResult
	var fulfillmentID *uuid.UUID
	err := r.db.QueryRow(ctx, query, code).Scan(&res.OrderItemAllocationID, &res.OrderItemID, &fulfillmentID, &res.OrderID, &res.UnitStatus, &res.PickedAt, &res.ReleasedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("zmu not found or not allocated")
		}
		return nil, err
	}
	if fulfillmentID == nil {
		return nil, errors.New("zmu allocation has no fulfillment")
	}
	res.FulfillmentID = *fulfillmentID
	return &res, nil
}

func (r *Repository) GetReturnItemUnitByAllocationID(ctx context.Context, allocationID uuid.UUID) (*ReturnItemUnit, error) {
	query := `
		SELECT id, return_item_id, order_item_allocation_id, scanned_at, inspected_condition, disposition, created_at, updated_at
		FROM return_item_units WHERE order_item_allocation_id = $1
	`
	var u ReturnItemUnit
	err := r.db.QueryRow(ctx, query, allocationID).Scan(&u.ID, &u.ReturnItemID, &u.OrderItemAllocationID, &u.ScannedAt, &u.InspectedCondition, &u.Disposition, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &u, nil
}

func (r *Repository) CreateReturnItemUnitTx(ctx context.Context, tx pgx.Tx, unit *ReturnItemUnit) error {
	query := `
		INSERT INTO return_item_units (id, return_item_id, order_item_allocation_id, scanned_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	if unit.ID == uuid.Nil {
		unit.ID = uuid.New()
	}
	now := time.Now()
	if unit.CreatedAt.IsZero() {
		unit.CreatedAt = now
	}
	unit.UpdatedAt = now

	_, err := tx.Exec(ctx, query, unit.ID, unit.ReturnItemID, unit.OrderItemAllocationID, unit.ScannedAt, unit.CreatedAt, unit.UpdatedAt)
	return err
}

func (r *Repository) GetReturnItemsForUpdateTx(ctx context.Context, tx pgx.Tx, returnID uuid.UUID) ([]ReturnItem, error) {
	query := `
		SELECT id, return_id, order_item_id, quantity, reason, condition, restock, accepted_quantity, damaged_quantity, rejected_quantity, created_at
		FROM return_items WHERE return_id = $1 FOR UPDATE
	`
	rows, err := tx.Query(ctx, query, returnID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []ReturnItem
	for rows.Next() {
		var item ReturnItem
		if err := rows.Scan(&item.ID, &item.ReturnID, &item.OrderItemID, &item.Quantity, &item.Reason, &item.Condition, &item.Restock, &item.AcceptedQuantity, &item.DamagedQuantity, &item.RejectedQuantity, &item.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, item)
	}
	if list == nil {
		list = make([]ReturnItem, 0)
	}
	return list, nil
}

func (r *Repository) GetScannedUnitCountForReturnItemTx(ctx context.Context, tx pgx.Tx, returnItemID uuid.UUID) (int, error) {
	query := `SELECT COUNT(*) FROM return_item_units WHERE return_item_id = $1`
	var count int
	err := tx.QueryRow(ctx, query, returnItemID).Scan(&count)
	return count, err
}
