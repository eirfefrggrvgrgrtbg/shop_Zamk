package payments

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

func (r *Repository) CreatePayment(ctx context.Context, p *Payment) error {
	query := `
		INSERT INTO payments (id, order_id, provider, provider_payment_id, status, amount_cents, currency, payment_url, idempotency_key, payment_method, integration_mode, init_outcome)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, 'successful')
		RETURNING payment_number, created_at, updated_at
	`
	err := r.db.QueryRow(ctx, query, p.ID, p.OrderID, p.Provider, p.ProviderPaymentID, p.Status, p.AmountCents, p.Currency, p.PaymentURL, p.IdempotencyKey, p.PaymentMethod, p.IntegrationMode).Scan(&p.PaymentNumber, &p.CreatedAt, &p.UpdatedAt)
	return err
}

func (r *Repository) GetActivePaymentForOrder(ctx context.Context, orderID uuid.UUID) (*Payment, error) {
	query := `
		SELECT id, order_id, provider, provider_payment_id, status, amount_cents, currency, payment_url, idempotency_key, payment_number, payment_method, integration_mode, init_outcome, created_at, updated_at, paid_at, failed_at, cancelled_at, reconciliation_attempted_at
		FROM payments
		WHERE order_id = $1 AND status IN ('created', 'pending')
		ORDER BY created_at DESC LIMIT 1
	`
	var p Payment
	err := r.db.QueryRow(ctx, query, orderID).Scan(
		&p.ID, &p.OrderID, &p.Provider, &p.ProviderPaymentID, &p.Status, &p.AmountCents, &p.Currency, &p.PaymentURL, &p.IdempotencyKey, &p.PaymentNumber, &p.PaymentMethod, &p.IntegrationMode, &p.InitOutcome, &p.CreatedAt, &p.UpdatedAt, &p.PaidAt, &p.FailedAt, &p.CancelledAt, &p.ReconciliationAttemptedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrPaymentNotFound
		}
		return nil, err
	}
	return &p, nil
}

func (r *Repository) GetPaymentByID(ctx context.Context, id uuid.UUID) (*Payment, error) {
	query := `
		SELECT id, order_id, provider, provider_payment_id, status, amount_cents, currency, payment_url, idempotency_key, payment_number, payment_method, integration_mode, init_outcome, created_at, updated_at, paid_at, failed_at, cancelled_at, reconciliation_attempted_at
		FROM payments
		WHERE id = $1
	`
	var p Payment
	err := r.db.QueryRow(ctx, query, id).Scan(
		&p.ID, &p.OrderID, &p.Provider, &p.ProviderPaymentID, &p.Status, &p.AmountCents, &p.Currency, &p.PaymentURL, &p.IdempotencyKey, &p.PaymentNumber, &p.PaymentMethod, &p.IntegrationMode, &p.InitOutcome, &p.CreatedAt, &p.UpdatedAt, &p.PaidAt, &p.FailedAt, &p.CancelledAt, &p.ReconciliationAttemptedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrPaymentNotFound
		}
		return nil, err
	}
	return &p, nil
}

func (r *Repository) GetPaymentByIDForUpdateTx(ctx context.Context, tx pgx.Tx, id uuid.UUID) (*Payment, error) {
	query := `
		SELECT id, order_id, provider, provider_payment_id, status, amount_cents, currency, payment_url, idempotency_key, payment_number, payment_method, integration_mode, init_outcome, created_at, updated_at, paid_at, failed_at, cancelled_at, reconciliation_attempted_at
		FROM payments
		WHERE id = $1
		FOR UPDATE
	`
	var p Payment
	err := tx.QueryRow(ctx, query, id).Scan(
		&p.ID, &p.OrderID, &p.Provider, &p.ProviderPaymentID, &p.Status, &p.AmountCents, &p.Currency, &p.PaymentURL, &p.IdempotencyKey, &p.PaymentNumber, &p.PaymentMethod, &p.IntegrationMode, &p.InitOutcome, &p.CreatedAt, &p.UpdatedAt, &p.PaidAt, &p.FailedAt, &p.CancelledAt, &p.ReconciliationAttemptedAt,
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
		SELECT id, order_id, provider, provider_payment_id, status, amount_cents, currency, payment_url, idempotency_key, payment_number, payment_method, integration_mode, init_outcome, created_at, updated_at, paid_at, failed_at, cancelled_at, reconciliation_attempted_at
		FROM payments
		WHERE provider = $1 AND provider_payment_id = $2
		FOR UPDATE
	`
	var p Payment
	err := tx.QueryRow(ctx, query, provider, providerPaymentID).Scan(
		&p.ID, &p.OrderID, &p.Provider, &p.ProviderPaymentID, &p.Status, &p.AmountCents, &p.Currency, &p.PaymentURL, &p.IdempotencyKey, &p.PaymentNumber, &p.PaymentMethod, &p.IntegrationMode, &p.InitOutcome, &p.CreatedAt, &p.UpdatedAt, &p.PaidAt, &p.FailedAt, &p.CancelledAt, &p.ReconciliationAttemptedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrPaymentNotFound
		}
		return nil, err
	}
	return &p, nil
}

func (r *Repository) GetPaymentByProviderOrOrderIDForUpdate(ctx context.Context, tx pgx.Tx, provider string, providerPaymentID, orderIDStr string, amountCents int64) (*Payment, error) {
	if providerPaymentID != "" {
		p, err := r.GetPaymentByProviderIDForUpdate(ctx, tx, provider, providerPaymentID)
		if err == nil {
			if orderIDStr != "" && p.OrderID.String() != orderIDStr {
				return nil, fmt.Errorf("%w: provider payment ID belongs to different order (%s != %s)", ErrPaymentConflict, p.OrderID.String(), orderIDStr)
			}
			if amountCents > 0 && p.AmountCents != amountCents {
				return nil, fmt.Errorf("%w: amount mismatch (%d != %d)", ErrPaymentAmountMismatch, p.AmountCents, amountCents)
			}
			return p, nil
		}
		if !errors.Is(err, ErrPaymentNotFound) {
			return nil, err
		}
	}

	if orderUUID, err := uuid.Parse(orderIDStr); err == nil {
		if providerPaymentID != "" {
			var otherCount int
			err := tx.QueryRow(ctx, `SELECT count(*) FROM payments WHERE provider = $1 AND provider_payment_id = $2`, provider, providerPaymentID).Scan(&otherCount)
			if err != nil {
				return nil, err
			}
			if otherCount > 0 {
				return nil, fmt.Errorf("%w: provider payment ID %s already assigned to another payment", ErrProviderPaymentAlreadyAssigned, providerPaymentID)
			}
		}

		query := `
			SELECT id, order_id, provider, provider_payment_id, status, amount_cents, currency, payment_url, idempotency_key, payment_number, payment_method, integration_mode, init_outcome, created_at, updated_at, paid_at, failed_at, cancelled_at, reconciliation_attempted_at
			FROM payments
			WHERE order_id = $1 AND (provider_payment_id IS NULL OR provider_payment_id = '') AND (status IN ('created', 'pending') OR init_outcome = 'unknown')
			ORDER BY created_at DESC
			FOR UPDATE
		`
		rows, err := tx.Query(ctx, query, orderUUID)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		var candidates []*Payment
		for rows.Next() {
			var p Payment
			if err := rows.Scan(
				&p.ID, &p.OrderID, &p.Provider, &p.ProviderPaymentID, &p.Status, &p.AmountCents, &p.Currency, &p.PaymentURL, &p.IdempotencyKey, &p.PaymentNumber, &p.PaymentMethod, &p.IntegrationMode, &p.InitOutcome, &p.CreatedAt, &p.UpdatedAt, &p.PaidAt, &p.FailedAt, &p.CancelledAt, &p.ReconciliationAttemptedAt,
			); err != nil {
				return nil, err
			}
			if amountCents > 0 && p.AmountCents != amountCents {
				continue
			}
			candidates = append(candidates, &p)
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}

		if len(candidates) == 0 {
			if providerPaymentID != "" {
				p, err := r.GetPaymentByProviderIDForUpdate(ctx, tx, provider, providerPaymentID)
				if err == nil {
					if orderIDStr != "" && p.OrderID.String() != orderIDStr {
						return nil, fmt.Errorf("%w: provider payment ID belongs to different order", ErrPaymentConflict)
					}
					if amountCents > 0 && p.AmountCents != amountCents {
						return nil, fmt.Errorf("%w: amount mismatch", ErrPaymentAmountMismatch)
					}
					return p, nil
				}
			}

			var pSettled Payment
			errSettled := tx.QueryRow(ctx, `
				SELECT id, order_id, provider, provider_payment_id, status, amount_cents, currency, payment_url, idempotency_key, payment_number, payment_method, integration_mode, init_outcome, created_at, updated_at, paid_at, failed_at, cancelled_at, reconciliation_attempted_at
				FROM payments
				WHERE order_id = $1
				ORDER BY created_at DESC
				LIMIT 1
				FOR UPDATE
			`, orderUUID).Scan(
				&pSettled.ID, &pSettled.OrderID, &pSettled.Provider, &pSettled.ProviderPaymentID, &pSettled.Status, &pSettled.AmountCents, &pSettled.Currency, &pSettled.PaymentURL, &pSettled.IdempotencyKey, &pSettled.PaymentNumber, &pSettled.PaymentMethod, &pSettled.IntegrationMode, &pSettled.InitOutcome, &pSettled.CreatedAt, &pSettled.UpdatedAt, &pSettled.PaidAt, &pSettled.FailedAt, &pSettled.CancelledAt, &pSettled.ReconciliationAttemptedAt,
			)
			if errSettled == nil {
				if amountCents > 0 && pSettled.AmountCents != amountCents {
					return nil, fmt.Errorf("%w: amount mismatch", ErrPaymentAmountMismatch)
				}
				if providerPaymentID != "" && pSettled.ProviderPaymentID != nil && *pSettled.ProviderPaymentID != "" && *pSettled.ProviderPaymentID != providerPaymentID {
					return nil, fmt.Errorf("%w: provider payment ID conflict", ErrPaymentConflict)
				}
				return &pSettled, nil
			}

			return nil, ErrPaymentNotFound
		}
		if len(candidates) > 1 {
			return nil, fmt.Errorf("%w: multiple plausible payments exist for order %s", ErrMultipleProviderPaymentsAmbiguous, orderIDStr)
		}
		return candidates[0], nil
	}
	return nil, ErrPaymentNotFound
}

func (r *Repository) IsProviderPaymentIDAttachedToOtherPayment(ctx context.Context, provider, providerPaymentID string, excludePaymentID uuid.UUID) (bool, error) {
	if providerPaymentID == "" {
		return false, nil
	}
	var count int
	err := r.db.QueryRow(ctx, `
		SELECT count(*) FROM payments
		WHERE provider = $1 AND provider_payment_id = $2 AND id != $3
	`, provider, providerPaymentID, excludePaymentID).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *Repository) ClaimReconciliationCandidates(ctx context.Context, batchLimit int, minAge time.Duration) ([]*Payment, error) {
	if batchLimit <= 0 {
		batchLimit = 50
	}
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	query := `
		SELECT id, order_id, provider, provider_payment_id, status, amount_cents, currency, payment_url, idempotency_key, payment_number, payment_method, integration_mode, init_outcome, created_at, updated_at, paid_at, failed_at, cancelled_at, reconciliation_attempted_at
		FROM payments
		WHERE status IN ('created', 'pending') AND init_outcome = 'unknown'
		  AND (reconciliation_attempted_at IS NULL OR reconciliation_attempted_at < $1)
		ORDER BY created_at ASC
		LIMIT $2
		FOR UPDATE SKIP LOCKED
	`
	threshold := time.Now().UTC().Add(-minAge)
	rows, err := tx.Query(ctx, query, threshold, batchLimit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var candidates []*Payment
	var ids []uuid.UUID
	for rows.Next() {
		var p Payment
		if err := rows.Scan(
			&p.ID, &p.OrderID, &p.Provider, &p.ProviderPaymentID, &p.Status, &p.AmountCents, &p.Currency, &p.PaymentURL, &p.IdempotencyKey, &p.PaymentNumber, &p.PaymentMethod, &p.IntegrationMode, &p.InitOutcome, &p.CreatedAt, &p.UpdatedAt, &p.PaidAt, &p.FailedAt, &p.CancelledAt, &p.ReconciliationAttemptedAt,
		); err != nil {
			return nil, err
		}
		candidates = append(candidates, &p)
		ids = append(ids, p.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()

	if len(ids) > 0 {
		_, err = tx.Exec(ctx, `
			UPDATE payments
			SET reconciliation_attempted_at = now(), updated_at = now()
			WHERE id = ANY($1)
		`, ids)
		if err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return candidates, nil
}

func (r *Repository) UpdatePaymentStatusTx(ctx context.Context, tx pgx.Tx, p *Payment) error {
	query := `
		UPDATE payments 
		SET status = $1, updated_at = now(), paid_at = $2, failed_at = $3, cancelled_at = $4, init_outcome = $5
		WHERE id = $6
	`
	_, err := tx.Exec(ctx, query, p.Status, p.PaidAt, p.FailedAt, p.CancelledAt, p.InitOutcome, p.ID)
	return err
}

func (r *Repository) CreatePaymentEventTx(ctx context.Context, tx pgx.Tx, e *PaymentEvent) error {
	query := `
		INSERT INTO payment_events (id, payment_id, provider, provider_payment_id, event_type, event_key, raw_payload, signature_valid, processed_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (provider, event_key) DO NOTHING
		RETURNING created_at
	`
	err := tx.QueryRow(ctx, query, e.ID, e.PaymentID, e.Provider, e.ProviderPaymentID, e.EventType, e.EventKey, e.RawPayload, e.SignatureValid, e.ProcessedAt).Scan(&e.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrPaymentAlreadyProcessed
	}
	return err
}


