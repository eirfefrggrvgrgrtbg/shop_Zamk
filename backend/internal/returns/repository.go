package returns

import (
	"context"
	"errors"

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
		INSERT INTO returns (id, order_id, user_id, status, reason, comment)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING created_at, updated_at
	`
	err := tx.QueryRow(ctx, query, ret.ID, ret.OrderID, ret.UserID, ret.Status, ret.Reason, ret.Comment).Scan(&ret.CreatedAt, &ret.UpdatedAt)
	if err != nil {
		return err
	}

	for i := range items {
		itemQuery := `
			INSERT INTO return_items (id, return_id, order_item_id, quantity, reason, condition, restock)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			RETURNING created_at
		`
		err = tx.QueryRow(ctx, itemQuery, items[i].ID, items[i].ReturnID, items[i].OrderItemID, items[i].Quantity, items[i].Reason, items[i].Condition, items[i].Restock).Scan(&items[i].CreatedAt)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) UpdateReturnTx(ctx context.Context, tx pgx.Tx, ret *Return) error {
	query := `
		UPDATE returns 
		SET status = $1, admin_comment = $2, updated_at = now(), approved_at = $3, rejected_at = $4, completed_at = $5
		WHERE id = $6
		RETURNING updated_at
	`
	return tx.QueryRow(ctx, query, ret.Status, ret.AdminComment, ret.ApprovedAt, ret.RejectedAt, ret.CompletedAt, ret.ID).Scan(&ret.UpdatedAt)
}

func (r *Repository) UpdateReturnItemRestockTx(ctx context.Context, tx pgx.Tx, itemID uuid.UUID, restock bool) error {
	query := `UPDATE return_items SET restock = $1 WHERE id = $2`
	_, err := tx.Exec(ctx, query, restock, itemID)
	return err
}

func (r *Repository) GetReturn(ctx context.Context, id uuid.UUID) (*Return, []ReturnItem, error) {
	query := `
		SELECT id, order_id, user_id, status, reason, comment, admin_comment, created_at, updated_at, approved_at, rejected_at, completed_at
		FROM returns WHERE id = $1
	`
	var ret Return
	err := r.db.QueryRow(ctx, query, id).Scan(
		&ret.ID, &ret.OrderID, &ret.UserID, &ret.Status, &ret.Reason, &ret.Comment, &ret.AdminComment,
		&ret.CreatedAt, &ret.UpdatedAt, &ret.ApprovedAt, &ret.RejectedAt, &ret.CompletedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, ErrReturnNotFound
		}
		return nil, nil, err
	}

	itemsQuery := `
		SELECT id, return_id, order_item_id, quantity, reason, condition, restock, created_at
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
		if err := rows.Scan(&item.ID, &item.ReturnID, &item.OrderItemID, &item.Quantity, &item.Reason, &item.Condition, &item.Restock, &item.CreatedAt); err != nil {
			return nil, nil, err
		}
		items = append(items, item)
	}
	if items == nil {
		items = make([]ReturnItem, 0)
	}

	return &ret, items, nil
}

func (r *Repository) ListReturnsByCustomer(ctx context.Context, userID uuid.UUID, limit, offset int) ([]Return, error) {
	query := `
		SELECT id, order_id, user_id, status, reason, comment, admin_comment, created_at, updated_at, approved_at, rejected_at, completed_at
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
		if err := rows.Scan(&ret.ID, &ret.OrderID, &ret.UserID, &ret.Status, &ret.Reason, &ret.Comment, &ret.AdminComment, &ret.CreatedAt, &ret.UpdatedAt, &ret.ApprovedAt, &ret.RejectedAt, &ret.CompletedAt); err != nil {
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
		SELECT id, order_id, user_id, status, reason, comment, admin_comment, created_at, updated_at, approved_at, rejected_at, completed_at
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
		if err := rows.Scan(&ret.ID, &ret.OrderID, &ret.UserID, &ret.Status, &ret.Reason, &ret.Comment, &ret.AdminComment, &ret.CreatedAt, &ret.UpdatedAt, &ret.ApprovedAt, &ret.RejectedAt, &ret.CompletedAt); err != nil {
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
		SELECT ri.id, ri.return_id, r.order_id, ri.order_item_id, r.status, ri.quantity, ri.reason, ri.condition
		FROM return_items ri
		JOIN returns r ON r.id = ri.return_id
		JOIN order_items oi ON oi.id = ri.order_item_id
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
		if err := rows.Scan(&item.ReturnItemID, &item.ReturnID, &item.OrderID, &item.OrderItemID, &item.Status, &item.Quantity, &item.Reason, &item.Condition); err != nil {
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
		SELECT ri.id, ri.return_id, r.order_id, ri.order_item_id, r.status, ri.quantity, ri.reason, ri.condition
		FROM return_items ri
		JOIN returns r ON r.id = ri.return_id
		JOIN order_items oi ON oi.id = ri.order_item_id
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
		if err := rows.Scan(&item.ReturnItemID, &item.ReturnID, &item.OrderID, &item.OrderItemID, &item.Status, &item.Quantity, &item.Reason, &item.Condition); err != nil {
			return nil, err
		}
		list = append(list, item)
	}
	if list == nil {
		list = make([]SellerReturnItem, 0)
	}
	return list, nil
}

func (r *Repository) CreateRefundTx(ctx context.Context, tx pgx.Tx, ref *Refund) error {
	query := `
		INSERT INTO refunds (id, return_id, payment_id, order_id, status, amount_cents, currency, provider, provider_refund_id, reason)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING created_at, updated_at
	`
	return tx.QueryRow(ctx, query, ref.ID, ref.ReturnID, ref.PaymentID, ref.OrderID, ref.Status, ref.AmountCents, ref.Currency, ref.Provider, ref.ProviderRefundID, ref.Reason).Scan(&ref.CreatedAt, &ref.UpdatedAt)
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
