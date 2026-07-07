package payouts

import (
	"context"
	"errors"
	"strconv"
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

func (r *Repository) ListSellerPayouts(ctx context.Context, sellerID uuid.UUID, limit, offset int) ([]Payout, error) {
	query := `
		SELECT id, seller_id, status, amount_cents, currency, requested_at, approved_at, rejected_at, paid_at, admin_user_id, comment, created_at, updated_at
		FROM payouts WHERE seller_id = $1 ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := r.db.Query(ctx, query, sellerID, limit, offset)
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

func (r *Repository) ListAllPayouts(ctx context.Context, limit, offset int) ([]Payout, error) {
	query := `
		SELECT id, seller_id, status, amount_cents, currency, requested_at, approved_at, rejected_at, paid_at, admin_user_id, comment, created_at, updated_at
		FROM payouts ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`
	rows, err := r.db.Query(ctx, query, limit, offset)
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

func (r *Repository) GetAdminPayoutSummary(ctx context.Context) (*AdminPayoutSummary, error) {
	query := `
		SELECT type, SUM(amount_cents) as total
		FROM seller_balance_ledger
		GROUP BY type
	`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var summary AdminPayoutSummary
	var salePending, saleAvailable, manualAdj, refundDeduction, payoutReq, payoutRej, payoutPaid int64

	for rows.Next() {
		var ltype string
		var total int64
		if err := rows.Scan(&ltype, &total); err != nil {
			return nil, err
		}
		switch ltype {
		case "sale_pending":
			salePending += total
		case "sale_available":
			saleAvailable += total
		case "manual_adjustment":
			manualAdj += total
		case "refund_deduction":
			refundDeduction += total
		case "payout_requested":
			payoutReq += total
			summary.TotalPendingCents -= total // payoutReq is negative, so sub to get pos
		case "payout_rejected":
			payoutRej += total
			summary.TotalRejectedCents += total // payout_rejected is positive
		case "payout_cancelled":
			payoutRej += total
			summary.TotalRejectedCents += total
		case "payout_paid":
			payoutPaid += total
		}
	}

	summary.TotalAvailableCents = saleAvailable + manualAdj + refundDeduction + payoutReq + payoutRej

	// Commission logic: Assuming we want commission from the marketplace orders
	// For third-party orders: The seller net is stored as "sale_pending" (and later "sale_available").
	// To get total marketplace commission, we might need a separate sum from order_fulfillments or calculate it based on something else.
	// The prompt states: "total marketplace commission".
	// Let's compute commission by summing (subtotal - seller_amount) from order_fulfillments for 3rd party sellers.
	commQuery := `
		SELECT COALESCE(SUM(subtotal_cents - seller_amount_cents), 0)
		FROM order_fulfillments
		WHERE seller_id != '00000000-0000-4000-8000-000000000000'
	`
	err = r.db.QueryRow(ctx, commQuery).Scan(&summary.TotalCommissionCents)
	if err != nil {
		return nil, err
	}

	// Wait, payoutPaid in ledger is often 0 or not perfectly mapped if "paid" doesn't create negative.
	// Let's sum from payouts table directly for paid:
	paidQuery := `SELECT COALESCE(SUM(amount_cents), 0) FROM payouts WHERE status = 'paid'`
	err = r.db.QueryRow(ctx, paidQuery).Scan(&summary.TotalPaidCents)
	if err != nil {
		return nil, err
	}

	summary.Currency = "RUB"
	return &summary, nil
}

func (r *Repository) ListAdminSellerBalances(ctx context.Context, limit, offset int) ([]AdminSellerBalance, int, error) {
	// Let's use a subquery to aggregate ledger by seller
	query := `
		WITH agg AS (
			SELECT 
				seller_id,
				SUM(CASE WHEN type = 'sale_pending' THEN amount_cents ELSE 0 END) as pending_cents,
				SUM(CASE WHEN type IN ('sale_available', 'manual_adjustment', 'refund_deduction', 'payout_requested', 'payout_rejected', 'payout_cancelled') THEN amount_cents ELSE 0 END) as available_cents
			FROM seller_balance_ledger
			GROUP BY seller_id
		)
		SELECT 
			a.seller_id,
			s.brand_name,
			a.pending_cents,
			a.available_cents
		FROM agg a
		JOIN sellers s ON s.id = a.seller_id
		ORDER BY a.available_cents DESC
		LIMIT $1 OFFSET $2
	`
	rows, err := r.db.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var list []AdminSellerBalance
	for rows.Next() {
		var b AdminSellerBalance
		if err := rows.Scan(&b.SellerID, &b.SellerName, &b.PendingBalanceCents, &b.AvailableBalanceCents); err != nil {
			return nil, 0, err
		}
		b.Currency = "RUB"
		list = append(list, b)
	}
	if list == nil {
		list = make([]AdminSellerBalance, 0)
	}

	var total int
	err = r.db.QueryRow(ctx, `SELECT count(DISTINCT seller_id) FROM seller_balance_ledger`).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	return list, total, nil
}

func (r *Repository) ListAdminPayoutsFiltered(ctx context.Context, filter PayoutFilter, limit, offset int) ([]Payout, int, error) {
	query := `
		SELECT p.id, p.seller_id, p.status, p.amount_cents, p.currency, p.requested_at, p.approved_at, p.rejected_at, p.paid_at, p.admin_user_id, p.comment, p.created_at, p.updated_at
		FROM payouts p
		LEFT JOIN sellers s ON s.id = p.seller_id
		WHERE 1=1
	`
	var args []interface{}
	argIdx := 1

	if filter.Status != "" {
		query += ` AND p.status = $` + strconv.Itoa(argIdx)
		args = append(args, filter.Status)
		argIdx++
	}

	if filter.SellerID != "" {
		if _, err := uuid.Parse(filter.SellerID); err == nil {
			query += ` AND p.seller_id = $` + strconv.Itoa(argIdx)
			args = append(args, filter.SellerID)
			argIdx++
		}
	}

	if filter.Q != "" {
		query += ` AND (s.brand_name ILIKE $` + strconv.Itoa(argIdx) + ` OR p.id::text ILIKE $` + strconv.Itoa(argIdx) + `)`
		args = append(args, "%"+filter.Q+"%")
		argIdx++
	}

	countQuery := `SELECT count(*) FROM (` + query + `) as c`
	var total int
	if err := r.db.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query += ` ORDER BY p.created_at DESC LIMIT $` + strconv.Itoa(argIdx) + ` OFFSET $` + strconv.Itoa(argIdx+1)
	args = append(args, limit, offset)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var list []Payout
	for rows.Next() {
		var p Payout
		if err := rows.Scan(&p.ID, &p.SellerID, &p.Status, &p.AmountCents, &p.Currency, &p.RequestedAt, &p.ApprovedAt, &p.RejectedAt, &p.PaidAt, &p.AdminUserID, &p.Comment, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, 0, err
		}
		list = append(list, p)
	}
	if list == nil {
		list = make([]Payout, 0)
	}

	return list, total, nil
}
