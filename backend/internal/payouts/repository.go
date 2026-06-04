package payouts

import (
	"context"
	"errors"
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

func (r *Repository) GetSellerBalances(ctx context.Context, sellerID uuid.UUID) (*BalanceResponse, error) {
	query := `
		SELECT 
			type,
			SUM(amount_cents) as total
		FROM seller_balance_ledger
		WHERE seller_id = $1
		GROUP BY type
	`
	rows, err := r.db.Query(ctx, query, sellerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pending, available, requested, paid int64
	// availableBalanceCents = SUM(sale_available) + SUM(manual_adjustment) + SUM(refund_deduction) + SUM(payout_requested) + SUM(payout_rejected)
	// pendingBalanceCents = SUM(sale_pending)
	var saleAvailable, manualAdj, refundDeduction, payoutReq, payoutRej int64

	for rows.Next() {
		var ltype string
		var total int64
		if err := rows.Scan(&ltype, &total); err != nil {
			return nil, err
		}
		switch ltype {
		case "sale_pending":
			pending += total
		case "sale_available":
			saleAvailable += total
		case "refund_deduction":
			refundDeduction += total
		case "manual_adjustment":
			manualAdj += total
		case "payout_requested":
			payoutReq += total
			requested -= total // it's negative in ledger, so we negate to show positive absolute value
		case "payout_rejected":
			payoutRej += total
		case "payout_cancelled":
			payoutRej += total
		case "payout_paid":
			paid += total // wait, payout_paid might be 0 amount audit marker or positive if we tracked it differently. If 0, it doesn't affect.
		}
	}

	available = saleAvailable + manualAdj + refundDeduction + payoutReq + payoutRej

	return &BalanceResponse{
		PendingBalanceCents:   pending,
		AvailableBalanceCents: available,
		RequestedPayoutsCents: requested,
		PaidPayoutsCents:      paid,
		Currency:              "RUB",
	}, nil
}

func (r *Repository) InsertLedgerEntryTx(ctx context.Context, tx pgx.Tx, entry *SellerBalanceLedger) error {
	query := `
		INSERT INTO seller_balance_ledger (id, seller_id, order_id, order_item_id, return_id, refund_id, payout_id, type, amount_cents, currency, available_at, comment)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING created_at
	`
	return tx.QueryRow(ctx, query, entry.ID, entry.SellerID, entry.OrderID, entry.OrderItemID, entry.ReturnID, entry.RefundID, entry.PayoutID, entry.Type, entry.AmountCents, entry.Currency, entry.AvailableAt, entry.Comment).Scan(&entry.CreatedAt)
}

func (r *Repository) HasSalePendingForOrderItem(ctx context.Context, orderItemID uuid.UUID) (bool, error) {
	query := `SELECT 1 FROM seller_balance_ledger WHERE order_item_id = $1 AND type = 'sale_pending' LIMIT 1`
	var tmp int
	err := r.db.QueryRow(ctx, query, orderItemID).Scan(&tmp)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (r *Repository) GetLedgerEntriesByType(ctx context.Context, tx pgx.Tx, ltype string, limit int, availableBefore time.Time) ([]SellerBalanceLedger, error) {
	query := `
		SELECT id, seller_id, order_id, order_item_id, return_id, refund_id, payout_id, type, amount_cents, currency, available_at, created_at, comment
		FROM seller_balance_ledger
		WHERE type = $1 AND available_at <= $2
		LIMIT $3
		FOR UPDATE SKIP LOCKED
	`
	rows, err := tx.Query(ctx, query, ltype, availableBefore, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []SellerBalanceLedger
	for rows.Next() {
		var entry SellerBalanceLedger
		if err := rows.Scan(&entry.ID, &entry.SellerID, &entry.OrderID, &entry.OrderItemID, &entry.ReturnID, &entry.RefundID, &entry.PayoutID, &entry.Type, &entry.AmountCents, &entry.Currency, &entry.AvailableAt, &entry.CreatedAt, &entry.Comment); err != nil {
			return nil, err
		}
		list = append(list, entry)
	}
	if list == nil {
		list = make([]SellerBalanceLedger, 0)
	}
	return list, nil
}

func (r *Repository) HasSaleAvailableForOrderItem(ctx context.Context, orderItemID uuid.UUID) (bool, error) {
	query := `SELECT 1 FROM seller_balance_ledger WHERE order_item_id = $1 AND type = 'sale_available' LIMIT 1`
	var tmp int
	err := r.db.QueryRow(ctx, query, orderItemID).Scan(&tmp)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (r *Repository) CreatePayoutTx(ctx context.Context, tx pgx.Tx, payout *Payout) error {
	query := `
		INSERT INTO payouts (id, seller_id, status, amount_cents, currency, comment)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING requested_at, updated_at, created_at
	`
	return tx.QueryRow(ctx, query, payout.ID, payout.SellerID, payout.Status, payout.AmountCents, payout.Currency, payout.Comment).Scan(&payout.RequestedAt, &payout.UpdatedAt, &payout.CreatedAt)
}

func (r *Repository) GetPayout(ctx context.Context, id uuid.UUID) (*Payout, error) {
	query := `
		SELECT id, seller_id, status, amount_cents, currency, requested_at, approved_at, rejected_at, paid_at, admin_user_id, comment, created_at, updated_at
		FROM payouts WHERE id = $1
	`
	var p Payout
	err := r.db.QueryRow(ctx, query, id).Scan(
		&p.ID, &p.SellerID, &p.Status, &p.AmountCents, &p.Currency, &p.RequestedAt, &p.ApprovedAt, &p.RejectedAt, &p.PaidAt, &p.AdminUserID, &p.Comment, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrPayoutNotFound
		}
		return nil, err
	}
	return &p, nil
}

func (r *Repository) UpdatePayoutTx(ctx context.Context, tx pgx.Tx, payout *Payout) error {
	query := `
		UPDATE payouts
		SET status = $1, approved_at = $2, rejected_at = $3, paid_at = $4, admin_user_id = $5, comment = $6, updated_at = now()
		WHERE id = $7
		RETURNING updated_at
	`
	return tx.QueryRow(ctx, query, payout.Status, payout.ApprovedAt, payout.RejectedAt, payout.PaidAt, payout.AdminUserID, payout.Comment, payout.ID).Scan(&payout.UpdatedAt)
}

func (r *Repository) ListSellerPayouts(ctx context.Context, sellerID uuid.UUID) ([]Payout, error) {
	query := `
		SELECT id, seller_id, status, amount_cents, currency, requested_at, approved_at, rejected_at, paid_at, admin_user_id, comment, created_at, updated_at
		FROM payouts WHERE seller_id = $1 ORDER BY created_at DESC
	`
	rows, err := r.db.Query(ctx, query, sellerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []Payout
	for rows.Next() {
		var p Payout
		if err := rows.Scan(&p.ID, &p.SellerID, &p.Status, &p.AmountCents, &p.Currency, &p.RequestedAt, &p.ApprovedAt, &p.RejectedAt, &p.PaidAt, &p.AdminUserID, &p.Comment, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		list = append(list, p)
	}
	if list == nil {
		list = make([]Payout, 0)
	}
	return list, nil
}

func (r *Repository) ListAllPayouts(ctx context.Context) ([]Payout, error) {
	query := `
		SELECT id, seller_id, status, amount_cents, currency, requested_at, approved_at, rejected_at, paid_at, admin_user_id, comment, created_at, updated_at
		FROM payouts ORDER BY created_at DESC
	`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []Payout
	for rows.Next() {
		var p Payout
		if err := rows.Scan(&p.ID, &p.SellerID, &p.Status, &p.AmountCents, &p.Currency, &p.RequestedAt, &p.ApprovedAt, &p.RejectedAt, &p.PaidAt, &p.AdminUserID, &p.Comment, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		list = append(list, p)
	}
	if list == nil {
		list = make([]Payout, 0)
	}
	return list, nil
}

// GetTotalPaidByPayout calculates total paid amount (just in case we sum payout_paid)
func (r *Repository) GetTotalPaidPayouts(ctx context.Context, sellerID uuid.UUID) (int64, error) {
	query := `SELECT COALESCE(SUM(amount_cents), 0) FROM payouts WHERE seller_id = $1 AND status = 'paid'`
	var total int64
	err := r.db.QueryRow(ctx, query, sellerID).Scan(&total)
	return total, err
}
