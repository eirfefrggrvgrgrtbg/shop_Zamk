package returns_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/orders"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/payments"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/returns"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/testutil"
)

// BANK.REFUND.0: migration 000120 applied as an UPGRADE over historical refunds, on this test's own disposable
// database. The database comes migrated; 000120.down returns it to the pre-000120 schema, historical rows are
// written in every status the code knows, then exactly 000120.up is applied.

func readMigration(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "migrations", name))
	require.NoError(t, err)
	return string(b)
}

type historicalRefund struct {
	id       uuid.UUID
	status   string
	returnID *uuid.UUID
}

func TestMigration000120_UpgradeOverHistoricalRefunds(t *testing.T) {
	ctx := context.Background()
	client, err := postgres.NewClient(ctx, testutil.DisposableDatabaseURL(t))
	require.NoError(t, err)
	t.Cleanup(func() { client.Close() })
	db := client.Pool
	testutil.MustBeDisposableDatabase(t, db)

	// 1. Back to the schema before 000120.
	_, err = db.Exec(ctx, readMigration(t, "000120_refund_bank_submission.down.sql"))
	require.NoError(t, err)
	var leftovers int
	require.NoError(t, db.QueryRow(ctx, `
		SELECT (SELECT count(*) FROM information_schema.columns WHERE table_name = 'refunds'
		          AND column_name IN ('submit_outcome', 'idempotency_key', 'submit_attempts', 'submitted_at', 'last_checked_at',
		                              'provider_status', 'failure_code', 'review_reason', 'retry_of_refund_id'))
		     + (SELECT count(*) FROM pg_trigger WHERE tgrelid = 'refunds'::regclass AND NOT tgisinternal)
		     + (SELECT count(*) FROM pg_proc WHERE proname = 'refunds_bank_attempt_immutable')
		     + (SELECT count(*) FROM pg_indexes WHERE tablename = 'refunds'
		          AND indexname IN ('refunds_idempotency_key_uniq', 'refunds_one_bank_attempt_per_return', 'refunds_retry_of_uniq'))
	`).Scan(&leftovers))
	require.Equal(t, 0, leftovers, "the down migration leaves the pre-000120 refunds schema")

	// 2. Historical data, in every refund status the code reads.
	userID, sellerID, orderID, fulfillmentID, paymentID := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	exec := func(q string, args ...any) {
		t.Helper()
		_, err := db.Exec(ctx, q, args...)
		require.NoError(t, err)
	}
	exec(`INSERT INTO users (id, name, phone, email, password_hash) VALUES ($1, 'Customer', '+79991110000', $2, 'hash')`, userID, "legacy_"+userID.String()+"@test.com")
	exec(`INSERT INTO sellers (id, brand_name, slug, contact_email, status) VALUES ($1, 'Legacy Seller', $2, 'l@test.com', 'active')`, sellerID, "legacy_"+sellerID.String())
	exec(`INSERT INTO orders (id, user_id, order_number, status, total_price_cents, currency, delivery_address, delivery_method_name, delivery_price_cents, customer_name, customer_email, customer_phone, created_at, updated_at)
		VALUES ($1, $2, $3, 'delivered', 50000, 'RUB', 'Addr', 'Courier', 0, 'Legacy', 'l@example.com', '+79990000000', now(), now())`, orderID, userID, "ORD-L-"+orderID.String()[:8])
	exec(`INSERT INTO order_fulfillments (id, order_id, seller_id, status) VALUES ($1, $2, $3, 'delivered')`, fulfillmentID, orderID, sellerID)
	exec(`INSERT INTO payments (id, order_id, provider, provider_payment_id, status, amount_cents, currency, idempotency_key, created_at, updated_at)
		VALUES ($1, $2, 'tbank', $3, 'succeeded', 50000, 'RUB', $4, now(), now())`, paymentID, orderID, "LEGACY-"+paymentID.String()[:8], "IDEM-"+uuid.New().String())

	newReturn := func(status string) *uuid.UUID {
		id := uuid.New()
		exec(`INSERT INTO returns (id, order_id, fulfillment_id, user_id, status, reason, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, 'defective', now(), now())`, id, orderID, fulfillmentID, userID, status)
		return &id
	}
	past := time.Now().Add(-72 * time.Hour).Truncate(time.Microsecond)
	hist := []historicalRefund{
		{uuid.New(), "pending", newReturn("item_received")}, // an active reservation of a return
		{uuid.New(), "pending", nil},                        // payment-only reservation
		{uuid.New(), "succeeded", newReturn("refunded")},
		{uuid.New(), "succeeded", nil}, // succeeded without processed_at, as found in the shared data
		{uuid.New(), "failed", newReturn("item_received")},
		{uuid.New(), "processing", nil},
		{uuid.New(), "completed", nil},
	}
	for i, h := range hist {
		var processedAt, failedAt *time.Time
		if h.status == "succeeded" && h.returnID != nil || h.status == "completed" {
			processedAt = &past
		}
		if h.status == "failed" {
			failedAt = &past
		}
		exec(`INSERT INTO refunds (id, return_id, payment_id, order_id, status, amount_cents, currency, provider, provider_refund_id, reason, created_at, updated_at, processed_at, failed_at)
			VALUES ($1, $2, $3, $4, $5, $6, 'RUB', $7, $8, 'legacy', $9, $9, $10, $11)`,
			h.id, h.returnID, paymentID, orderID, h.status, int64(1000*(i+1)), "tbank", fmt.Sprintf("OLD-%d", i), past, processedAt, failedAt)
	}
	// The seller deduction already written for the historical success.
	exec(`INSERT INTO seller_ledger_entries (id, seller_id, order_id, type, amount_cents, currency, metadata, created_at)
		VALUES ($1, $2, $3, 'adjustment', -3000, 'RUB', '{"reason":"return_deduction"}', now())`, uuid.New(), sellerID, orderID)

	fingerprint := func() (string, string) {
		var refundsFP, ledgerFP string
		require.NoError(t, db.QueryRow(ctx, `
			SELECT md5(string_agg(row(id, return_id, payment_id, order_id, status, amount_cents, currency, provider,
			                          provider_refund_id, reason, created_at, updated_at, processed_at, failed_at)::text, ',' ORDER BY id))
			FROM refunds WHERE order_id = $1`, orderID).Scan(&refundsFP))
		require.NoError(t, db.QueryRow(ctx, `
			SELECT md5(string_agg(row(id, seller_id, order_id, type, amount_cents, payout_batch_id, metadata)::text, ',' ORDER BY id))
			FROM seller_ledger_entries WHERE seller_id = $1`, sellerID).Scan(&ledgerFP))
		return refundsFP, ledgerFP
	}
	refundsBefore, ledgerBefore := fingerprint()

	// 3. Exactly 000120.up.
	_, err = db.Exec(ctx, readMigration(t, "000120_refund_bank_submission.up.sql"))
	require.NoError(t, err, "000120 applies over historical refunds of every status")

	// 4. Amounts, statuses, links, ids and the ledger are untouched; every historical row is legacy, not an attempt.
	refundsAfter, ledgerAfter := fingerprint()
	assert.Equal(t, refundsBefore, refundsAfter)
	assert.Equal(t, ledgerBefore, ledgerAfter)
	var nonLegacy int
	require.NoError(t, db.QueryRow(ctx, `
		SELECT count(*) FROM refunds WHERE order_id = $1 AND NOT (submit_outcome = 'legacy' AND idempotency_key IS NULL
		  AND submit_attempts = 0 AND submitted_at IS NULL AND review_reason IS NULL AND retry_of_refund_id IS NULL)
	`, orderID).Scan(&nonLegacy))
	assert.Equal(t, 0, nonLegacy)

	mustFail := func(name, q string, args ...any) {
		t.Helper()
		_, err := db.Exec(ctx, q, args...)
		assert.Error(t, err, name)
	}
	pendingRet, succeededRet, failedRet := hist[0], hist[2], hist[4]

	// 5. No false bank proof on history; history is not rewritten; legacy is never (re)created.
	mustFail("legacy_success_marked_confirmed", `UPDATE refunds SET submit_outcome = 'confirmed' WHERE id = $1`, succeededRet.id)
	mustFail("legacy_success_marked_confirmed_with_key", `UPDATE refunds SET submit_outcome = 'confirmed', idempotency_key = 'k-x' WHERE id = $1`, succeededRet.id)
	mustFail("legacy_failure_marked_rejected", `UPDATE refunds SET submit_outcome = 'rejected', idempotency_key = 'k-y' WHERE id = $1`, failedRet.id)
	mustFail("legacy_pending_sent", `UPDATE refunds SET submit_outcome = 'sending', idempotency_key = 'k-z' WHERE id = $1`, pendingRet.id)
	mustFail("legacy_success_back_to_pending", `UPDATE refunds SET status = 'pending' WHERE id = $1`, succeededRet.id)
	mustFail("legacy_success_amount_rewritten", `UPDATE refunds SET amount_cents = 1 WHERE id = $1`, succeededRet.id)
	mustFail("legacy_failure_revived", `UPDATE refunds SET status = 'pending' WHERE id = $1`, failedRet.id)
	mustFail("insert_legacy", `INSERT INTO refunds (id, order_id, status, amount_cents, submit_outcome) VALUES ($1, $2, 'pending', 1, 'legacy')`, uuid.New(), orderID)
	mustFail("bad_outcome", `UPDATE refunds SET submit_outcome = 'whatever' WHERE id = $1`, pendingRet.id)

	historyAfterUp := historyFingerprint(t, db, hist)

	// 6. New rows default to not_sent; a bank attempt next to history works and leaves history alone.
	newID := uuid.New()
	exec(`INSERT INTO refunds (id, return_id, payment_id, order_id, status, amount_cents, currency) VALUES ($1, $2, $3, $4, 'pending', 500, 'RUB')`,
		newID, failedRet.returnID, paymentID, orderID)
	var outcome string
	require.NoError(t, db.QueryRow(ctx, `SELECT submit_outcome FROM refunds WHERE id = $1`, newID).Scan(&outcome))
	assert.Equal(t, "not_sent", outcome)
	mustFail("not_sent_straight_to_confirmed", `UPDATE refunds SET status = 'succeeded', submit_outcome = 'confirmed', idempotency_key = 'k-1' WHERE id = $1`, newID)
	exec(`UPDATE refunds SET submit_outcome = 'sending', idempotency_key = 'k-new' WHERE id = $1`, newID)
	exec(`INSERT INTO refunds (id, return_id, order_id, status, amount_cents, idempotency_key) VALUES ($1, $2, $3, 'pending', 1, 'k-2')`,
		uuid.New(), failedRet.returnID, orderID)
	mustFail("second_unresolved_attempt_for_return", `UPDATE refunds SET submit_outcome = 'sending' WHERE idempotency_key = 'k-2'`)
	mustFail("duplicate_key", `INSERT INTO refunds (id, order_id, status, amount_cents, idempotency_key) VALUES ($1, $2, 'pending', 1, 'k-new')`, uuid.New(), orderID)
	mustFail("sending_without_pending", `UPDATE refunds SET status = 'succeeded' WHERE id = $1`, newID)
	mustFail("key_change", `UPDATE refunds SET idempotency_key = 'k-other' WHERE id = $1`, newID)
	exec(`UPDATE refunds SET submit_outcome = 'unknown' WHERE id = $1`, newID)
	mustFail("unknown_back_to_sending", `UPDATE refunds SET submit_outcome = 'sending' WHERE id = $1`, newID)
	exec(`UPDATE refunds SET status = 'succeeded', submit_outcome = 'confirmed' WHERE id = $1`, newID)
	mustFail("confirmed_is_final", `UPDATE refunds SET status = 'failed', submit_outcome = 'rejected' WHERE id = $1`, newID)
	exec(`UPDATE refunds SET retry_of_refund_id = $2 WHERE id = $1`, newID, failedRet.id)
	assert.Equal(t, historyAfterUp, historyFingerprint(t, db, hist), "bank attempts next to history leave it untouched")
	assert.Equal(t, ledgerBefore, ledgerFingerprint(t, db, sellerID))

	// 7. The service never sends history, and keeps the pre-bank compatibility for it.
	fake := payments.NewFakeRefundProvider()
	svc := returns.NewService(returns.NewRepository(db), orders.NewRepository(db), nil, client, nil, nil, 14, nil, nil, nil, nil).
		WithRefundProvider(fake)
	_, err = svc.SubmitRefund(ctx, pendingRet.id)
	assert.ErrorIs(t, err, returns.ErrRefundLegacyRecord)
	_, err = svc.SubmitRefund(ctx, succeededRet.id)
	assert.ErrorIs(t, err, returns.ErrRefundLegacyRecord)
	got, err := svc.ReconcileRefund(ctx, succeededRet.id)
	require.NoError(t, err)
	assert.Equal(t, "succeeded", got.Status)
	assert.Equal(t, 0, fake.Operations("LEGACY-"+paymentID.String()[:8]), "no historical refund reaches a provider")
	// A historical reservation keeps settling through the canonical, pre-bank path (as the simulator did).
	require.NoError(t, client.RunInTx(ctx, func(tx pgx.Tx) error {
		return svc.ProcessRefundSuccessTx(ctx, tx, hist[1].id, time.Now())
	}))
	require.NoError(t, db.QueryRow(ctx, `SELECT status || '/' || submit_outcome FROM refunds WHERE id = $1`, hist[1].id).Scan(&outcome))
	assert.Equal(t, "succeeded/legacy", outcome, "settled locally, with no claim of a bank confirmation")
	assert.Equal(t, ledgerBefore, ledgerFingerprint(t, db, sellerID))
}

// historyFingerprint hashes the historical rows only (their original columns plus the bank columns).
func historyFingerprint(t *testing.T, db testutil.DBQueryRow, hist []historicalRefund) string {
	t.Helper()
	ids := make([]uuid.UUID, len(hist))
	for i, h := range hist {
		ids[i] = h.id
	}
	var fp string
	require.NoError(t, db.QueryRow(context.Background(), `
		SELECT md5(string_agg(row(id, return_id, payment_id, order_id, status, amount_cents, processed_at, failed_at,
		                          submit_outcome, idempotency_key, retry_of_refund_id)::text, ',' ORDER BY id))
		FROM refunds WHERE id = ANY($1)`, ids).Scan(&fp))
	return fp
}

func ledgerFingerprint(t *testing.T, db testutil.DBQueryRow, sellerID uuid.UUID) string {
	t.Helper()
	var fp string
	require.NoError(t, db.QueryRow(context.Background(), `
		SELECT md5(string_agg(row(id, seller_id, order_id, type, amount_cents, payout_batch_id, metadata)::text, ',' ORDER BY id))
		FROM seller_ledger_entries WHERE seller_id = $1`, sellerID).Scan(&fp))
	return fp
}
