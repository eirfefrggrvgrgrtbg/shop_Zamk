package payouts

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// PAYOUT.SAFETY.1 (Product Owner decision B): the earning of units under an OPEN return is not paid out while the
// return is open; the undisputed earning stays payable.
//
// The ledger keeps one seller_earning row per order item (all its units), so the disputed share cannot be split off
// that row. Instead, when a payout batch captures an earning that has open returns, an append-only negative
// adjustment (reason "return_hold") for exactly the disputed share is linked to the same batch: the batch pays the
// undisputed part only. When the return closes, an equal positive adjustment (reason "return_hold_release") is
// written once, immediately available: after a rejection / cancellation / completion without refund the seller gets
// the held money in the next batch; after a refund the existing return deduction applies and the hold and its
// release cancel out exactly, so the deduction is counted once. No row is rewritten, available_at is not moved.
//
// Lock order (no new deadlock): open returns of the seller (FOR SHARE) -> ledger rows (FOR UPDATE) -> order_items
// (FOR SHARE). Refund confirmation locks returns -> ledger; return creation locks orders -> order_items; status
// changes lock returns only; the release reads ledger rows without locking them.

// OpenReturnStatuses are the return statuses whose units are still disputed (not rejected, cancelled, refunded
// or completed). Accepting the goods (item_received) does not close the dispute: the refund is still pending.
var OpenReturnStatuses = []string{"requested", "needs_info", "approved", "receiving", "item_received"}

// ErrPayoutBatchHasOpenReturns is returned when a batch would confirm money that an open return disputes.
var ErrPayoutBatchHasOpenReturns = errors.New("payout batch contains earnings disputed by an open return: resolve the return (reject, cancel or confirm the refund), then confirm the payout again")

const (
	reasonReturnHold        = "return_hold"
	reasonReturnHoldRelease = "return_hold_release"
)

type ledgerQuerier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type openReturnItem struct {
	returnItemID uuid.UUID
	returnID     uuid.UUID
	quantity     int64
	heldCents    int64 // unreleased hold already written for this return item (0 = none)
	hasHold      bool
}

// openReturnItemsTx lists the open return items of one order item in a stable order, with any unreleased hold.
func openReturnItemsTx(ctx context.Context, q ledgerQuerier, orderItemID uuid.UUID) ([]openReturnItem, error) {
	rows, err := q.Query(ctx, `
		SELECT ri.id, ri.return_id, ri.quantity,
		       h.amount_cents
		FROM return_items ri
		JOIN returns r ON r.id = ri.return_id
		LEFT JOIN LATERAL (
			SELECT -sle.amount_cents AS amount_cents
			FROM seller_ledger_entries sle
			WHERE sle.type = 'adjustment'
			  AND sle.metadata->>'reason' = 'return_hold'
			  AND sle.metadata->>'return_item_id' = ri.id::text
			  AND NOT EXISTS (
				SELECT 1 FROM seller_ledger_entries rel
				WHERE rel.type = 'adjustment'
				  AND rel.metadata->>'reason' = 'return_hold_release'
				  AND rel.metadata->>'hold_entry_id' = sle.id::text
			  )
			LIMIT 1
		) h ON true
		WHERE ri.order_item_id = $1 AND r.status = ANY($2)
		ORDER BY r.created_at, ri.id
	`, orderItemID, OpenReturnStatuses)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []openReturnItem
	for rows.Next() {
		var it openReturnItem
		var held *int64
		if err := rows.Scan(&it.returnItemID, &it.returnID, &it.quantity, &held); err != nil {
			return nil, err
		}
		if held != nil {
			it.hasHold, it.heldCents = true, *held
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

// priorDeductions returns the refunded (already deducted) amount and quantity of an order item.
func priorDeductions(ctx context.Context, q ledgerQuerier, orderItemID uuid.UUID) (int64, int64, error) {
	var amount, qty int64
	err := q.QueryRow(ctx, `
		SELECT COALESCE(SUM(ABS(sle.amount_cents)), 0),
		       COALESCE(SUM(CASE WHEN (sle.metadata->>'quantity') ~ '^[0-9]+$' THEN (sle.metadata->>'quantity')::bigint ELSE 1 END), 0)
		FROM seller_ledger_entries sle
		WHERE sle.order_item_id = $1 AND sle.type = 'adjustment'
		  AND sle.metadata->>'reason' IN ('return_deduction', 'return_post_payout')
	`, orderItemID).Scan(&amount, &qty)
	return amount, qty, err
}

// disputedShares allocates the disputed earning to each open return item with the same cumulative formula the
// refund deduction uses (target(q) = floor(E*q/Q), on top of what was already deducted). Items that already carry a
// hold keep it; the result holds the NEW hold for each item (0 for items already held).
func disputedShares(earning, orderQty, priorAmount, priorQty int64, items []openReturnItem) []int64 {
	out := make([]int64, len(items))
	if earning <= 0 || orderQty <= 0 {
		return out
	}
	cum := priorQty
	prevTarget := priorAmount
	for i, it := range items {
		cum += it.quantity
		if cum > orderQty {
			cum = orderQty
		}
		target := earning * cum / orderQty
		if it.hasHold {
			prevTarget += it.heldCents
			continue
		}
		if share := target - prevTarget; share > 0 {
			out[i] = share
			prevTarget = target
		}
	}
	return out
}

// lockOpenReturnsForSellerTx takes the returns lock first (see lock order above).
func lockOpenReturnsForSellerTx(ctx context.Context, tx pgx.Tx, sellerID uuid.UUID) error {
	_, err := tx.Exec(ctx, `
		SELECT r.id FROM returns r
		JOIN order_fulfillments f ON f.id = r.fulfillment_id
		WHERE f.seller_id = $1 AND r.status = ANY($2)
		ORDER BY r.id
		FOR SHARE OF r
	`, sellerID, OpenReturnStatuses)
	return err
}

// lockOrderItemsTx serializes with return creation (which locks the order items FOR UPDATE).
func lockOrderItemsTx(ctx context.Context, tx pgx.Tx, ids []uuid.UUID) (map[uuid.UUID]int64, error) {
	rows, err := tx.Query(ctx, `SELECT id, quantity FROM order_items WHERE id = ANY($1) ORDER BY id FOR SHARE`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	qty := make(map[uuid.UUID]int64, len(ids))
	for rows.Next() {
		var id uuid.UUID
		var q int64
		if err := rows.Scan(&id, &q); err != nil {
			return nil, err
		}
		qty[id] = q
	}
	return qty, rows.Err()
}

func earningOrderItems(entries []SellerLedgerEntry) []uuid.UUID {
	seen := map[uuid.UUID]bool{}
	var ids []uuid.UUID
	for _, e := range entries {
		if e.Type == "seller_earning" && e.OrderItemID != nil && !seen[*e.OrderItemID] {
			seen[*e.OrderItemID] = true
			ids = append(ids, *e.OrderItemID)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i].String() < ids[j].String() })
	return ids
}

// applyReturnHoldsTx writes the hold adjustments for the earnings a new batch captures and returns their total.
func (s *Service) applyReturnHoldsTx(ctx context.Context, tx pgx.Tx, batchID uuid.UUID, entries []SellerLedgerEntry) (int64, error) {
	ids := earningOrderItems(entries)
	if len(ids) == 0 {
		return 0, nil
	}
	orderQty, err := lockOrderItemsTx(ctx, tx, ids)
	if err != nil {
		return 0, err
	}
	byItem := map[uuid.UUID]SellerLedgerEntry{}
	for _, e := range entries {
		if e.Type == "seller_earning" && e.OrderItemID != nil {
			byItem[*e.OrderItemID] = e
		}
	}
	var total int64
	for _, itemID := range ids {
		items, err := openReturnItemsTx(ctx, tx, itemID)
		if err != nil || len(items) == 0 {
			if err != nil {
				return 0, err
			}
			continue
		}
		earning := byItem[itemID]
		priorAmount, priorQty, err := priorDeductions(ctx, tx, itemID)
		if err != nil {
			return 0, err
		}
		shares := disputedShares(earning.AmountCents, orderQty[itemID], priorAmount, priorQty, items)
		for i, share := range shares {
			if share <= 0 {
				continue
			}
			meta, _ := json.Marshal(map[string]any{
				"reason": reasonReturnHold, "hold_return_id": items[i].returnID.String(),
				"return_item_id": items[i].returnItemID.String(), "quantity": items[i].quantity,
			})
			id := itemID
			bID := batchID
			if err := s.repo.CreateLedgerEntryTx(ctx, tx, &SellerLedgerEntry{
				ID: uuid.New(), SellerID: earning.SellerID, OrderID: earning.OrderID, OrderItemID: &id,
				PayoutBatchID: &bID, Type: "adjustment", AmountCents: -share, Currency: "RUB",
				AvailableAt: earning.AvailableAt, Metadata: meta, CreatedAt: time.Now(),
			}); err != nil {
				return 0, err
			}
			total += share
		}
	}
	return total, nil
}

// batchHasUnheldOpenReturnsTx reports whether the batch would pay disputed money: an open return on one of its
// earnings whose disputed share is NOT covered by a hold (a return opened, or a disputed quantity added, after the
// batch was formed). Returns that already existed when the batch was formed are covered by their holds inside the
// batch, so the undisputed part stays payable. Locks the order items so a return cannot appear meanwhile.
func batchHasUnheldOpenReturnsTx(ctx context.Context, tx pgx.Tx, batchID uuid.UUID) (bool, error) {
	rows, err := tx.Query(ctx, `SELECT order_item_id, amount_cents FROM seller_ledger_entries WHERE payout_batch_id = $1 AND type = 'seller_earning' AND order_item_id IS NOT NULL`, batchID)
	if err != nil {
		return false, err
	}
	var ids []uuid.UUID
	earningOf := map[uuid.UUID]int64{}
	for rows.Next() {
		var id uuid.UUID
		var amount int64
		if err := rows.Scan(&id, &amount); err != nil {
			rows.Close()
			return false, err
		}
		ids = append(ids, id)
		earningOf[id] = amount
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return false, err
	}
	if len(ids) == 0 {
		return false, nil
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i].String() < ids[j].String() })
	orderQty, err := lockOrderItemsTx(ctx, tx, ids)
	if err != nil {
		return false, err
	}
	for _, id := range ids {
		items, err := openReturnItemsTx(ctx, tx, id)
		if err != nil {
			return false, err
		}
		if len(items) == 0 {
			continue
		}
		pa, pq, err := priorDeductions(ctx, tx, id)
		if err != nil {
			return false, err
		}
		// disputedShares returns the share still to hold for items without a hold: any positive share is uncovered.
		for _, share := range disputedShares(earningOf[id], orderQty[id], pa, pq, items) {
			if share > 0 {
				return true, nil
			}
		}
	}
	return false, nil
}

// ReleaseReturnHoldsTx writes, once, the release of every unreleased hold of a return that has just closed
// (rejected, cancelled, completed, or refund confirmed). Callers hold the return row lock, which serializes repeats.
func (s *Service) ReleaseReturnHoldsTx(ctx context.Context, tx pgx.Tx, returnID uuid.UUID) error {
	rows, err := tx.Query(ctx, `
		SELECT sle.id, sle.seller_id, sle.order_id, sle.order_item_id, sle.amount_cents
		FROM seller_ledger_entries sle
		WHERE sle.type = 'adjustment'
		  AND sle.metadata->>'reason' = 'return_hold'
		  AND sle.metadata->>'hold_return_id' = $1
		  AND NOT EXISTS (
			SELECT 1 FROM seller_ledger_entries rel
			WHERE rel.type = 'adjustment' AND rel.metadata->>'reason' = 'return_hold_release'
			  AND rel.metadata->>'hold_entry_id' = sle.id::text
		  )
		ORDER BY sle.id
	`, returnID.String())
	if err != nil {
		return err
	}
	type hold struct {
		id, sellerID    uuid.UUID
		orderID, itemID *uuid.UUID
		amount          int64
	}
	var holds []hold
	for rows.Next() {
		var h hold
		if err := rows.Scan(&h.id, &h.sellerID, &h.orderID, &h.itemID, &h.amount); err != nil {
			rows.Close()
			return err
		}
		holds = append(holds, h)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for _, h := range holds {
		meta, _ := json.Marshal(map[string]any{"reason": reasonReturnHoldRelease, "hold_entry_id": h.id.String(), "hold_return_id": returnID.String()})
		if err := s.repo.CreateLedgerEntryTx(ctx, tx, &SellerLedgerEntry{
			ID: uuid.New(), SellerID: h.sellerID, OrderID: h.orderID, OrderItemID: h.itemID,
			Type: "adjustment", AmountCents: -h.amount, Currency: "RUB", Metadata: meta, CreatedAt: time.Now(),
		}); err != nil {
			return fmt.Errorf("release return hold: %w", err)
		}
	}
	return nil
}

// disputedAvailableCents is the not-yet-held disputed share of AVAILABLE, unbatched earnings (sellerID nil = all
// sellers). Balances subtract it, so money under an open return is never shown as payable.
func disputedAvailableCents(ctx context.Context, q ledgerQuerier, sellerID *uuid.UUID) (int64, error) {
	rows, err := q.Query(ctx, `
		SELECT DISTINCT sle.order_item_id, sle.amount_cents, oi.quantity
		FROM seller_ledger_entries sle
		JOIN order_items oi ON oi.id = sle.order_item_id
		JOIN return_items ri ON ri.order_item_id = sle.order_item_id
		JOIN returns r ON r.id = ri.return_id AND r.status = ANY($2)
		WHERE sle.type = 'seller_earning' AND sle.payout_batch_id IS NULL AND sle.available_at <= now()
		  AND ($1::uuid IS NULL OR sle.seller_id = $1)
	`, sellerID, OpenReturnStatuses)
	if err != nil {
		return 0, err
	}
	type cand struct {
		item     uuid.UUID
		earning  int64
		orderQty int64
	}
	var cands []cand
	for rows.Next() {
		var c cand
		if err := rows.Scan(&c.item, &c.earning, &c.orderQty); err != nil {
			rows.Close()
			return 0, err
		}
		cands = append(cands, c)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}
	var total int64
	for _, c := range cands {
		items, err := openReturnItemsTx(ctx, q, c.item)
		if err != nil {
			return 0, err
		}
		pa, pq, err := priorDeductions(ctx, q, c.item)
		if err != nil {
			return 0, err
		}
		for _, share := range disputedShares(c.earning, c.orderQty, pa, pq, items) {
			total += share
		}
	}
	return total, nil
}

// heldInBatchesCents is the unreleased hold total still inside payout batches (sellerID nil = all sellers).
func heldInBatchesCents(ctx context.Context, q ledgerQuerier, sellerID *uuid.UUID) (int64, error) {
	var total int64
	err := q.QueryRow(ctx, `
		SELECT COALESCE(SUM(-sle.amount_cents), 0)
		FROM seller_ledger_entries sle
		WHERE sle.type = 'adjustment' AND sle.metadata->>'reason' = 'return_hold'
		  AND ($1::uuid IS NULL OR sle.seller_id = $1)
		  AND NOT EXISTS (
			SELECT 1 FROM seller_ledger_entries rel
			WHERE rel.type = 'adjustment' AND rel.metadata->>'reason' = 'return_hold_release'
			  AND rel.metadata->>'hold_entry_id' = sle.id::text
		  )
	`, sellerID).Scan(&total)
	return total, err
}
