package payouts

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/orders"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (s *Service) ReconcileReturnCompensationTx(ctx context.Context, tx pgx.Tx, orderItemID uuid.UUID) error {
	// 1. acquire deterministic lock for order_item;
	earningEntry, err := s.repo.GetSellerEarningEntryTx(ctx, tx, orderItemID)
	if err != nil {
		return fmt.Errorf("failed to get earning entry: %w", err)
	}
	if earningEntry == nil || earningEntry.AmountCents <= 0 {
		return nil // No earning to compensate
	}

	orderItem, err := s.orders.GetOrderItems(ctx, *earningEntry.OrderID)
	if err != nil {
		return fmt.Errorf("failed to get order items: %w", err)
	}
	var targetOrderItem *orders.OrderItem
	for _, oi := range orderItem {
		if oi.ID == orderItemID {
			oiCopy := oi
			targetOrderItem = &oiCopy
			break
		}
	}
	if targetOrderItem == nil || targetOrderItem.Quantity <= 0 {
		return nil
	}

	// 2. calculate E/Q;
	E := earningEntry.AmountCents
	Q := int64(targetOrderItem.Quantity)

	// 3. calculate financially reversed quantity R;
	_, R, err := s.repo.GetPriorReturnDeductionsTx(ctx, tx, orderItemID, uuid.Nil)
	if err != nil {
		return fmt.Errorf("failed to get prior deductions: %w", err)
	}

	// 4. calculate compensable quantity C via per-return intersection;
	rawC, err := s.repo.GetCompensableQuantityTx(ctx, tx, orderItemID)
	if err != nil {
		return fmt.Errorf("failed to get compensable quantity: %w", err)
	}

	C := rawC
	if C > R {
		C = R
	}
	if C < 0 {
		C = 0
	}
	if C > Q {
		C = Q
	}

	// 5. calculate prior net compensation;
	netPrior, err := s.repo.GetNetPriorCompensationTx(ctx, tx, orderItemID)
	if err != nil {
		return fmt.Errorf("failed to get prior compensation: %w", err)
	}

	// 6. calculate target/delta;
	target := (E * C) / Q
	delta := target - netPrior

	if delta == 0 {
		return nil // NO ledger write
	}

	// 7. insert at most one adjustment;
	reason := "return_compensation"
	if delta < 0 {
		reason = "return_compensation_correction"
	}

	meta := map[string]interface{}{
		"reason":                          reason,
		"target_compensation_cents":       target,
		"previous_net_compensation_cents": netPrior,
		"compensable_quantity":            C,
		"reversed_quantity":               R,
		"original_quantity":               Q,
	}
	metaBytes, _ := json.Marshal(meta)

	// 16. AVAILABLE_AT rule
	var availableAt *time.Time
	if earningEntry.PayoutBatchID != nil {
		availableAt = nil // already paid
	} else if earningEntry.AvailableAt != nil && earningEntry.AvailableAt.After(time.Now()) {
		availableAt = earningEntry.AvailableAt // still frozen
	} else {
		availableAt = nil // already available
	}

	currency := earningEntry.Currency
	if currency == "" {
		currency = "RUB"
	}

	err = s.repo.CreateLedgerEntryTx(ctx, tx, &SellerLedgerEntry{
		ID:          uuid.New(),
		SellerID:    earningEntry.SellerID,
		OrderID:     earningEntry.OrderID,
		OrderItemID: &orderItemID,
		Type:        "adjustment",
		AmountCents: delta,
		Currency:    currency,
		AvailableAt: availableAt,
		Metadata:    metaBytes,
		CreatedAt:   time.Now(),
	})
	if err != nil {
		return fmt.Errorf("failed to insert compensation: %w", err)
	}

	return nil
}
