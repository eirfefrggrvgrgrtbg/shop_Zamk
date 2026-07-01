package payments

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

func (r *Repository) CreatePayment(ctx context.Context, p *Payment) error {
	query := `
		INSERT INTO payments (id, order_id, provider, provider_payment_id, status, amount_cents, currency, payment_url, idempotency_key)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING created_at, updated_at
	`
	err := r.db.QueryRow(ctx, query, p.ID, p.OrderID, p.Provider, p.ProviderPaymentID, p.Status, p.AmountCents, p.Currency, p.PaymentURL, p.IdempotencyKey).Scan(&p.CreatedAt, &p.UpdatedAt)
	return err
}

func (r *Repository) GetActivePaymentForOrder(ctx context.Context, orderID uuid.UUID) (*Payment, error) {
	query := `
		SELECT id, order_id, provider, provider_payment_id, status, amount_cents, currency, payment_url, idempotency_key, created_at, updated_at, paid_at, failed_at, cancelled_at
		FROM payments
		WHERE order_id = $1 AND status IN ('created', 'pending')
		ORDER BY created_at DESC LIMIT 1
	`
	var p Payment
	err := r.db.QueryRow(ctx, query, orderID).Scan(
		&p.ID, &p.OrderID, &p.Provider, &p.ProviderPaymentID, &p.Status, &p.AmountCents, &p.Currency, &p.PaymentURL, &p.IdempotencyKey, &p.CreatedAt, &p.UpdatedAt, &p.PaidAt, &p.FailedAt, &p.CancelledAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrPaymentNotFound
		}
		return nil, err
	}
	return &p, nil
}

func (r *Repository) GetPaymentByProviderIDForUpdate(ctx context.Context, tx pgx.Tx, provider string, providerPaymentID string) (*Payment, error) {
	query := `
		SELECT id, order_id, provider, provider_payment_id, status, amount_cents, currency, payment_url, idempotency_key, created_at, updated_at, paid_at, failed_at, cancelled_at
		FROM payments
		WHERE provider = $1 AND provider_payment_id = $2
		FOR UPDATE
	`
	var p Payment
	err := tx.QueryRow(ctx, query, provider, providerPaymentID).Scan(
		&p.ID, &p.OrderID, &p.Provider, &p.ProviderPaymentID, &p.Status, &p.AmountCents, &p.Currency, &p.PaymentURL, &p.IdempotencyKey, &p.CreatedAt, &p.UpdatedAt, &p.PaidAt, &p.FailedAt, &p.CancelledAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrPaymentNotFound
		}
		return nil, err
	}
	return &p, nil
}

func (r *Repository) UpdatePaymentStatusTx(ctx context.Context, tx pgx.Tx, p *Payment) error {
	query := `
		UPDATE payments 
		SET status = $1, updated_at = now(), paid_at = $2, failed_at = $3, cancelled_at = $4
		WHERE id = $5
	`
	_, err := tx.Exec(ctx, query, p.Status, p.PaidAt, p.FailedAt, p.CancelledAt, p.ID)
	return err
}

func (r *Repository) CreatePaymentEventTx(ctx context.Context, tx pgx.Tx, e *PaymentEvent) error {
	query := `
		INSERT INTO payment_events (id, payment_id, provider, provider_payment_id, event_type, raw_payload, signature_valid, processed_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING created_at
	`
	return tx.QueryRow(ctx, query, e.ID, e.PaymentID, e.Provider, e.ProviderPaymentID, e.EventType, e.RawPayload, e.SignatureValid, e.ProcessedAt).Scan(&e.CreatedAt)
}

func (r *Repository) ListAdminPayments(ctx context.Context, limit, offset int) ([]Payment, error) {
	query := `
		SELECT id, order_id, provider, provider_payment_id, status, amount_cents, currency, payment_url, idempotency_key, created_at, updated_at, paid_at, failed_at, cancelled_at
		FROM payments ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`
	rows, err := r.db.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var payments []Payment
	for rows.Next() {
		var p Payment
		if err := rows.Scan(&p.ID, &p.OrderID, &p.Provider, &p.ProviderPaymentID, &p.Status, &p.AmountCents, &p.Currency, &p.PaymentURL, &p.IdempotencyKey, &p.CreatedAt, &p.UpdatedAt, &p.PaidAt, &p.FailedAt, &p.CancelledAt); err != nil {
			return nil, err
		}
		payments = append(payments, p)
	}
	if payments == nil {
		payments = make([]Payment, 0)
	}
	return payments, nil
}

func (r *Repository) GetAdminPayment(ctx context.Context, id uuid.UUID) (*Payment, error) {
	query := `
		SELECT id, order_id, provider, provider_payment_id, status, amount_cents, currency, payment_url, idempotency_key, created_at, updated_at, paid_at, failed_at, cancelled_at
		FROM payments WHERE id = $1
	`
	var p Payment
	err := r.db.QueryRow(ctx, query, id).Scan(
		&p.ID, &p.OrderID, &p.Provider, &p.ProviderPaymentID, &p.Status, &p.AmountCents, &p.Currency, &p.PaymentURL, &p.IdempotencyKey, &p.CreatedAt, &p.UpdatedAt, &p.PaidAt, &p.FailedAt, &p.CancelledAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrPaymentNotFound
		}
		return nil, err
	}
	return &p, nil
}
