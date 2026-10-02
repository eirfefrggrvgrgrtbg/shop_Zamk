package payments

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var ErrPaymentClaimConflict = errors.New("payment claim conflict")

func (r *Repository) CreatePaymentClaim(ctx context.Context, p *Payment) error {
	query := `
		INSERT INTO payments (id, order_id, provider, status, amount_cents, currency, idempotency_key, payment_method, integration_mode, init_outcome)
		VALUES ($1, $2, $3, 'created', $4, $5, $6, $7, $8, 'pending')
		ON CONFLICT (order_id) WHERE status IN ('created', 'pending') DO NOTHING
		RETURNING payment_number, created_at, updated_at
	`
	err := r.db.QueryRow(ctx, query, p.ID, p.OrderID, p.Provider, p.AmountCents, p.Currency, p.IdempotencyKey, p.PaymentMethod, p.IntegrationMode).Scan(&p.PaymentNumber, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrPaymentClaimConflict
	}
	return err
}

func (r *Repository) CreatePaymentClaimTx(ctx context.Context, tx pgx.Tx, p *Payment) error {
	query := `
		INSERT INTO payments (id, order_id, provider, status, amount_cents, currency, idempotency_key, payment_method, integration_mode, init_outcome)
		VALUES ($1, $2, $3, 'created', $4, $5, $6, $7, $8, 'pending')
		ON CONFLICT (order_id) WHERE status IN ('created', 'pending') DO NOTHING
		RETURNING payment_number, created_at, updated_at
	`
	err := tx.QueryRow(ctx, query, p.ID, p.OrderID, p.Provider, p.AmountCents, p.Currency, p.IdempotencyKey, p.PaymentMethod, p.IntegrationMode).Scan(&p.PaymentNumber, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrPaymentClaimConflict
	}
	return err
}

func (r *Repository) UpdatePaymentWithProviderData(ctx context.Context, id uuid.UUID, providerPaymentID, paymentURL string) error {
	query := `
		UPDATE payments
		SET provider_payment_id = $2, payment_url = $3, status = 'pending', init_outcome = 'successful', updated_at = NOW()
		WHERE id = $1
	`
	_, err := r.db.Exec(ctx, query, id, providerPaymentID, paymentURL)
	return err
}

func (r *Repository) MarkPaymentFailed(ctx context.Context, id uuid.UUID) error {
	query := `
		UPDATE payments
		SET status = 'failed', init_outcome = 'rejected', failed_at = NOW(), updated_at = NOW()
		WHERE id = $1
	`
	_, err := r.db.Exec(ctx, query, id)
	return err
}

func (r *Repository) MarkPaymentFailedTx(ctx context.Context, tx pgx.Tx, id uuid.UUID) error {
	query := `
		UPDATE payments
		SET status = 'failed', init_outcome = 'rejected', failed_at = NOW(), updated_at = NOW()
		WHERE id = $1
	`
	_, err := tx.Exec(ctx, query, id)
	return err
}

func (r *Repository) UpdatePaymentInitOutcome(ctx context.Context, id uuid.UUID, outcome string) error {
	query := `
		UPDATE payments
		SET init_outcome = $2, updated_at = NOW()
		WHERE id = $1
	`
	_, err := r.db.Exec(ctx, query, id, outcome)
	return err
}

// AcquireFirstPaymentClaimTx acquires or revalidates an exclusive customer-level first-payment claim.
// If another order already holds the first-payment claim for this customer, it returns ErrFirstPaymentInProgress.
// If the same order already holds the claim, it idempotently updates the timestamp and succeeds.
func (r *Repository) AcquireFirstPaymentClaimTx(ctx context.Context, tx pgx.Tx, userID, orderID uuid.UUID) error {
	query := `
		INSERT INTO customer_first_payment_claims (user_id, order_id, created_at, updated_at)
		VALUES ($1, $2, now(), now())
		ON CONFLICT (user_id) DO UPDATE
		SET updated_at = now()
		WHERE customer_first_payment_claims.order_id = $2
	`
	tag, err := tx.Exec(ctx, query, userID, orderID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrFirstPaymentInProgress
	}
	return nil
}

// ReleaseFirstPaymentClaimTx releases the first-payment claim for the given customer and order.
func (r *Repository) ReleaseFirstPaymentClaimTx(ctx context.Context, tx pgx.Tx, userID, orderID uuid.UUID) error {
	_, err := tx.Exec(ctx, `DELETE FROM customer_first_payment_claims WHERE user_id = $1 AND order_id = $2`, userID, orderID)
	return err
}

// ReleaseFirstPaymentClaimByOrderID releases any first-payment claim held by the given order ID.
func (r *Repository) ReleaseFirstPaymentClaimByOrderID(ctx context.Context, orderID uuid.UUID) error {
	_, err := r.db.Exec(ctx, `DELETE FROM customer_first_payment_claims WHERE order_id = $1`, orderID)
	return err
}

// ReleaseFirstPaymentClaimByOrderIDTx releases any first-payment claim held by the given order ID.
func (r *Repository) ReleaseFirstPaymentClaimByOrderIDTx(ctx context.Context, tx pgx.Tx, orderID uuid.UUID) error {
	_, err := tx.Exec(ctx, `DELETE FROM customer_first_payment_claims WHERE order_id = $1`, orderID)
	return err
}

// GetFirstPaymentClaimTx returns the order ID holding the customer's first-payment claim, if any.
func (r *Repository) GetFirstPaymentClaimTx(ctx context.Context, tx pgx.Tx, userID uuid.UUID) (*uuid.UUID, error) {
	var orderID uuid.UUID
	err := tx.QueryRow(ctx, `SELECT order_id FROM customer_first_payment_claims WHERE user_id = $1`, userID).Scan(&orderID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &orderID, nil
}
