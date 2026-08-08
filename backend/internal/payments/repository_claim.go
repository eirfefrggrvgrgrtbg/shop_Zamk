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
		INSERT INTO payments (id, order_id, provider, status, amount_cents, currency, idempotency_key, payment_method, integration_mode)
		VALUES ($1, $2, $3, 'created', $4, $5, $6, $7, $8)
		ON CONFLICT (order_id) WHERE status IN ('created', 'pending') DO NOTHING
		RETURNING payment_number, created_at, updated_at
	`
	err := r.db.QueryRow(ctx, query, p.ID, p.OrderID, p.Provider, p.AmountCents, p.Currency, p.IdempotencyKey, p.PaymentMethod, p.IntegrationMode).Scan(&p.PaymentNumber, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrPaymentClaimConflict
	}
	return err
}

func (r *Repository) UpdatePaymentWithProviderData(ctx context.Context, id uuid.UUID, providerPaymentID, paymentURL string) error {
	query := `
		UPDATE payments
		SET provider_payment_id = $2, payment_url = $3, status = 'pending', updated_at = NOW()
		WHERE id = $1
	`
	_, err := r.db.Exec(ctx, query, id, providerPaymentID, paymentURL)
	return err
}

func (r *Repository) MarkPaymentFailed(ctx context.Context, id uuid.UUID) error {
	query := `
		UPDATE payments
		SET status = 'failed', failed_at = NOW(), updated_at = NOW()
		WHERE id = $1
	`
	_, err := r.db.Exec(ctx, query, id)
	return err
}
