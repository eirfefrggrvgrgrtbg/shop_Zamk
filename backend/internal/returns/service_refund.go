package returns

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/payments"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/payouts"
)

type calculatedRefundItem struct {
	OrderItemID        uuid.UUID
	ProductTitle       string
	Mode               string // "serialized" | "legacy"
	RequestedQuantity  int
	RefundableQuantity int
	UnitPriceCents     int64
	RefundCents        int64
}

type calculatedRefundBreakdown struct {
	Items               []calculatedRefundItem
	ProductsRefundCents int64
	DeliveryRefundCents int64
	TotalRefundCents    int64
}

type orderItemInfo struct {
	Title      string
	PriceCents int64
	Quantity   int
	// PaidTotalCents is what the customer actually paid for the whole line: the
	// order_item_promotions snapshot when promoted, otherwise price_cents * quantity.
	PaidTotalCents int64
}

// refundQuerier is satisfied by both the pool and a transaction.
type refundQuerier interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

// refundableQuantity returns how many units of a return item are refundable.
//
// Canonical Allocation Invariant:
// Q = order_items.quantity
// A = len(outbound_allocations)
// A == Q => serialized
// A == 0 => legacy
// 0 < A < Q => INVALID / FAIL CLOSED
// A > Q => INVALID / FAIL CLOSED
func refundableQuantity(item AdminReturnReceivingItem, orderQty int) (string, int, error) {
	allocCount := len(item.OutboundAllocations)
	if allocCount == orderQty && orderQty > 0 {
		// For serialized items: return_item_units.disposition is authoritative.
		// disposition = restock => refundable
		// disposition = damaged => refundable
		// disposition = reject => NOT refundable
		// unreceived / no bound ReturnItemUnit => NOT refundable
		// NOTE: return_items aggregate accepted/damaged values are NOT used.
		refundableQty := 0
		for _, u := range item.ScannedUnits {
			if u.Disposition != nil {
				disp := *u.Disposition
				if disp == "restock" || disp == "damaged" {
					refundableQty++
				}
			}
		}
		return "serialized", refundableQty, nil
	}
	if allocCount == 0 {
		// For legacy items (no outbound allocations):
		// return_items.accepted_quantity + return_items.damaged_quantity is authoritative.
		// rejected_quantity and notReceived are NOT refundable.
		return "legacy", item.AcceptedQuantity + item.DamagedQuantity, nil
	}
	// Corrupted partial or excess allocation: fail-closed with typed error
	return "", 0, ErrRefundAllocationInvariant
}

// cumulativeRefundCents returns the refund for n more units of a line whose customer paid
// total is paidTotal over quantity q, after prior units were already refunded. Allocating
// floor(paidTotal * units / q) cumulatively keeps the sum over all units exactly equal to
// paidTotal, so repeated partial refunds never exceed what was paid for the line.
func cumulativeRefundCents(paidTotal int64, q, prior, n int) int64 {
	if q <= 0 || n <= 0 || paidTotal <= 0 {
		return 0
	}
	if prior > q {
		prior = q
	}
	end := prior + n
	if end > q {
		end = q
	}
	return paidTotal*int64(end)/int64(q) - paidTotal*int64(prior)/int64(q)
}

// calculateRefundBreakdown prices the refundable units of a return from what the customer
// actually paid. priorRefundedQty holds units of each order item already covered by other
// returns' active refunds; it may be nil when only quantities are needed.
func calculateRefundBreakdown(state *AdminReturnReceivingState, orderItemsMap map[uuid.UUID]orderItemInfo, priorRefundedQty map[uuid.UUID]int) (calculatedRefundBreakdown, error) {
	var breakdown calculatedRefundBreakdown
	breakdown.Items = make([]calculatedRefundItem, 0, len(state.Items))

	for _, item := range state.Items {
		oi, ok := orderItemsMap[item.ReturnItem.OrderItemID]
		if !ok {
			return breakdown, fmt.Errorf("order item %s not found in order", item.ReturnItem.OrderItemID)
		}

		mode, refundableQty, err := refundableQuantity(item, oi.Quantity)
		if err != nil {
			return breakdown, err
		}

		itemRefundCents := cumulativeRefundCents(oi.PaidTotalCents, oi.Quantity, priorRefundedQty[item.ReturnItem.OrderItemID], refundableQty)
		unitPrice := int64(0)
		if oi.Quantity > 0 {
			unitPrice = oi.PaidTotalCents / int64(oi.Quantity)
		}

		breakdown.Items = append(breakdown.Items, calculatedRefundItem{
			OrderItemID:        item.ReturnItem.OrderItemID,
			ProductTitle:       oi.Title,
			Mode:               mode,
			RequestedQuantity:  item.RequestedQuantity,
			RefundableQuantity: refundableQty,
			UnitPriceCents:     unitPrice,
			RefundCents:        itemRefundCents,
		})

		breakdown.ProductsRefundCents += itemRefundCents
	}

	breakdown.DeliveryRefundCents = 0 // M5.4A policy: delivery fee refund is 0
	breakdown.TotalRefundCents = breakdown.ProductsRefundCents + breakdown.DeliveryRefundCents
	return breakdown, nil
}

// priorRefundedQuantities returns, per order item, the units already covered by active
// (pending, processing or succeeded) refunds of other returns of the same order.
func (s *Service) priorRefundedQuantities(ctx context.Context, db refundQuerier, orderID, excludeReturnID uuid.UUID, orderItemsMap map[uuid.UUID]orderItemInfo) (map[uuid.UUID]int, error) {
	returnIDs, err := s.repo.getActivelyRefundedReturnIDs(ctx, db, orderID, excludeReturnID)
	if err != nil {
		return nil, err
	}
	prior := make(map[uuid.UUID]int)
	for _, rid := range returnIDs {
		state, err := s.repo.getReturnReceivingState(ctx, db, rid)
		if err != nil {
			return nil, err
		}
		for _, item := range state.Items {
			oi, ok := orderItemsMap[item.ReturnItem.OrderItemID]
			if !ok {
				return nil, fmt.Errorf("order item %s not found in order", item.ReturnItem.OrderItemID)
			}
			_, qty, err := refundableQuantity(item, oi.Quantity)
			if err != nil {
				return nil, err
			}
			prior[item.ReturnItem.OrderItemID] += qty
		}
	}
	return prior, nil
}

func (s *Service) CalculateRefundQuote(ctx context.Context, returnID uuid.UUID) (*ReturnRefundQuote, error) {
	state, err := s.repo.GetReturnReceivingState(ctx, returnID)
	if err != nil {
		return nil, err
	}

	orderItemMap, err := s.repo.getRefundOrderItems(ctx, s.repo.db, state.Return.OrderID)
	if err != nil {
		return nil, err
	}

	succeededRefunded, pendingRefund, err := s.repo.GetRefundSumsForOrder(ctx, state.Return.OrderID)
	if err != nil {
		return nil, err
	}

	priorQty, breakdownErr := s.priorRefundedQuantities(ctx, s.repo.db, state.Return.OrderID, returnID, orderItemMap)
	var breakdown calculatedRefundBreakdown
	if breakdownErr == nil {
		breakdown, breakdownErr = calculateRefundBreakdown(state, orderItemMap, priorQty)
	}

	var quoteItems []ReturnRefundQuoteItem
	for _, it := range breakdown.Items {
		quoteItems = append(quoteItems, ReturnRefundQuoteItem{
			OrderItemID:        it.OrderItemID,
			ProductTitle:       it.ProductTitle,
			Mode:               it.Mode,
			RequestedQuantity:  it.RequestedQuantity,
			RefundableQuantity: it.RefundableQuantity,
			UnitPriceCents:     it.UnitPriceCents,
			RefundCents:        it.RefundCents,
		})
	}

	remainingRefundable := breakdown.TotalRefundCents - succeededRefunded - pendingRefund
	if remainingRefundable < 0 {
		remainingRefundable = 0
	}

	var blockingReason *string
	canRefund := true
	var latestRefundStatus *string
	var latestRefundProcessedAt *time.Time

	// Check if an active (pending or succeeded) refund already exists for this return
	existingRefund, err := s.repo.GetRefundByReturnID(ctx, returnID)
	if err == nil && existingRefund != nil {
		statusStr := existingRefund.Status
		latestRefundStatus = &statusStr
		latestRefundProcessedAt = existingRefund.ProcessedAt

		if existingRefund.Status == "pending" {
			canRefund = false
			reason := "Возврат средств уже зарезервирован и ожидает обработки"
			blockingReason = &reason
		} else if existingRefund.Status == "succeeded" || existingRefund.Status == "completed" {
			canRefund = false
			reason := "Возврат средств уже выполнен"
			blockingReason = &reason
		}
		// If existingRefund.Status == "failed", quote remains eligible for retry
	}

	retStatus := state.Return.Status
	if canRefund {
		if retStatus == "requested" || retStatus == "needs_info" || retStatus == "approved" || retStatus == "receiving" {
			canRefund = false
			reason := "Возврат средств доступен только после приёмки товара на складе."
			blockingReason = &reason
		} else if retStatus == "rejected" {
			canRefund = false
			reason := "Возврат отклонен: возврат средств невозможен"
			blockingReason = &reason
		} else if retStatus == "cancelled" {
			canRefund = false
			reason := "Возврат средств недоступен для отмененных возвратов"
			blockingReason = &reason
		} else if retStatus == "refunded" || retStatus == "completed" {
			canRefund = false
			reason := "Возврат средств уже выполнен"
			blockingReason = &reason
		} else if retStatus == "item_received" {
			// Check allocation invariant error
			if errors.Is(breakdownErr, ErrRefundAllocationInvariant) {
				canRefund = false
				reason := "Несогласованное состояние резервирования: количество единиц не соответствует заказу"
				blockingReason = &reason
			} else if breakdownErr != nil {
				canRefund = false
				reason := fmt.Sprintf("Ошибка расчета возврата: %v", breakdownErr)
				blockingReason = &reason
			} else if breakdown.TotalRefundCents <= 0 {
				canRefund = false
				reason := "Нет принятых товаров, подлежащих возврату средств"
				blockingReason = &reason
			} else {
				// Check funding payment
				if s.payments != nil {
					_, payErr := s.payments.GetSucceededPaymentIDForOrder(ctx, state.Return.OrderID)
					if payErr != nil {
						canRefund = false
						if errors.Is(payErr, payments.ErrPaymentNotFound) || errors.Is(payErr, ErrPaymentNotFound) {
							reason := "Не найдена успешная оплата по заказу"
							blockingReason = &reason
						} else if errors.Is(payErr, payments.ErrAmbiguousFundingPayment) || errors.Is(payErr, ErrAmbiguousFundingPayment) {
							reason := "Неоднозначная оплата: обнаружено несколько успешных платежей по заказу"
							blockingReason = &reason
						} else {
							reason := fmt.Sprintf("Ошибка проверки оплаты: %v", payErr)
							blockingReason = &reason
						}
					}
				}
			}
		} else {
			canRefund = false
			reason := "Недопустимый статус возврата"
			blockingReason = &reason
		}
	}

	return &ReturnRefundQuote{
		ReturnID:                 state.Return.ID,
		OrderNumber:              state.OrderNumber,
		Currency:                 "RUB",
		Items:                    quoteItems,
		ProductsRefundCents:      breakdown.ProductsRefundCents,
		DeliveryRefundCents:      breakdown.DeliveryRefundCents,
		TotalRefundCents:         breakdown.TotalRefundCents,
		AlreadyRefundedCents:     succeededRefunded,
		SucceededRefundedCents:   succeededRefunded,
		PendingRefundCents:       pendingRefund,
		RemainingRefundableCents: remainingRefundable,
		CanRefund:                canRefund,
		BlockingReason:           blockingReason,
		LatestRefundStatus:       latestRefundStatus,
		LatestRefundProcessedAt:  latestRefundProcessedAt,
	}, nil
}

func (s *Service) CreateRefund(ctx context.Context, adminID, returnID uuid.UUID, req CreateRefundRequest) (*Refund, error) {
	var finalRefund *Refund

	err := s.db.RunInTx(ctx, func(tx pgx.Tx) error {
		// 1. Lock return row for update to guarantee strict concurrency protection
		ret, err := s.repo.GetReturnTx(ctx, tx, returnID)
		if err != nil {
			return err
		}

		// 2. Status gate & idempotency check
		if ret.Status == "refunded" {
			existingRef, err := s.repo.GetRefundByReturnIDTx(ctx, tx, returnID)
			if err == nil && existingRef != nil {
				finalRefund = existingRef
				return nil
			}
			return ErrReturnAlreadyRefunded
		}

		if ret.Status == "completed" {
			return ErrReturnAlreadyRefunded
		}

		if ret.Status == "requested" || ret.Status == "needs_info" || ret.Status == "approved" || ret.Status == "receiving" {
			return ErrReturnNotReceived
		}

		if ret.Status == "rejected" || ret.Status == "cancelled" {
			return ErrReturnRejected
		}

		if ret.Status != "item_received" {
			return ErrInvalidStatusTransition
		}

		// 3. Load receiving state and historical order items within tx
		state, err := s.repo.GetReturnReceivingStateTx(ctx, tx, returnID)
		if err != nil {
			return err
		}

		// Serialize refunds of different returns of the same order on the funding payment
		// row, so prior refunded units are read only after concurrent refunds commit.
		if _, err := tx.Exec(ctx, `SELECT id FROM payments WHERE order_id = $1 AND status = 'succeeded' FOR UPDATE`, ret.OrderID); err != nil {
			return err
		}

		orderItemMap, err := s.repo.getRefundOrderItems(ctx, tx, ret.OrderID)
		if err != nil {
			return err
		}
		priorQty, err := s.priorRefundedQuantities(ctx, tx, ret.OrderID, ret.ID, orderItemMap)
		if err != nil {
			return err
		}

		breakdown, err := calculateRefundBreakdown(state, orderItemMap, priorQty)
		if err != nil {
			return err
		}
		if breakdown.TotalRefundCents <= 0 {
			return ErrRefundNoEligibleItems
		}

		// Check if an identical pending or succeeded refund already exists for this return (idempotency)
		existingRef, err := s.repo.GetRefundByReturnIDTx(ctx, tx, returnID)
		if err == nil && existingRef != nil && existingRef.AmountCents == breakdown.TotalRefundCents {
			if existingRef.Status == "pending" || existingRef.Status == "succeeded" {
				finalRefund = existingRef
				return nil
			}
		}

		// 4. Find funding payment (fails closed on 0 or >1 payments)
		paymentID, err := s.payments.GetSucceededPaymentIDForOrder(ctx, ret.OrderID)
		if err != nil {
			return err
		}

		// 5. Reserve financial refund in payments module within tx
		// NOTE: ReserveRefundTx creates refunds row with status = 'pending', processed_at = NULL.
		// Money is NOT yet considered refunded; this is an authorized reservation.
		reason := ""
		if req.Reason != nil {
			reason = *req.Reason
		}

		if err := s.payments.ReserveRefundTx(ctx, tx, paymentID, breakdown.TotalRefundCents, reason, &ret.ID); err != nil {
			return err
		}

		// 6. Return remains in 'item_received' until financial refund is durably confirmed (succeeded).
		// (Return does NOT falsely become 'refunded' and seller is NOT debited prematurely while pending).

		// 7. Retrieve the created pending refund record within tx
		createdRef, err := s.repo.GetRefundByReturnIDTx(ctx, tx, returnID)
		if err == nil && createdRef != nil {
			finalRefund = createdRef
		} else {
			finalRefund = &Refund{
				ID:          uuid.New(),
				ReturnID:    &ret.ID,
				OrderID:     ret.OrderID,
				PaymentID:   &paymentID,
				Status:      "pending",
				AmountCents: breakdown.TotalRefundCents,
				Currency:    "RUB",
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return finalRefund, nil
}

// ProcessRefundSuccessTx marks a refund as succeeded with its durable processed_at timestamp,
// transitions the linked Return to 'refunded', and applies the seller finance deduction.
func (s *Service) ProcessRefundSuccessTx(ctx context.Context, tx pgx.Tx, refundID uuid.UUID, processedAt time.Time) error {
	var ref Refund
	queryRef := `
		SELECT id, return_id, payment_id, order_id, status, amount_cents, currency, processed_at, created_at, updated_at
		FROM refunds
		WHERE id = $1
		FOR UPDATE
	`
	err := tx.QueryRow(ctx, queryRef, refundID).Scan(
		&ref.ID, &ref.ReturnID, &ref.PaymentID, &ref.OrderID, &ref.Status, &ref.AmountCents, &ref.Currency, &ref.ProcessedAt, &ref.CreatedAt, &ref.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrRefundNotFound
		}
		return err
	}

	if ref.Status == "succeeded" {
		return nil // idempotent success
	}
	if ref.Status == "failed" {
		return nil // terminal state: failed refund cannot become succeeded
	}

	// Update refund to succeeded with processed_at
	_, err = tx.Exec(ctx, `
		UPDATE refunds
		SET status = 'succeeded', processed_at = $1, updated_at = now()
		WHERE id = $2
	`, processedAt, refundID)
	if err != nil {
		return err
	}

	// If bound to a return, transition Return to 'refunded' and apply seller finance deduction
	if ref.ReturnID != nil {
		returnID := *ref.ReturnID
		ret, err := s.repo.GetReturnTx(ctx, tx, returnID)
		if err != nil {
			return err
		}

		ret.Status = "refunded"
		if err := s.repo.UpdateReturnTx(ctx, tx, ret); err != nil {
			return err
		}

		// Calculate seller deduction from actual refundable quantities
		state, err := s.repo.GetReturnReceivingStateTx(ctx, tx, returnID)
		if err != nil {
			return err
		}
		orderItemMap, err := s.repo.getRefundOrderItems(ctx, tx, ret.OrderID)
		if err != nil {
			return err
		}

		// Only refundable quantities are used here; the seller deduction is priced from the ledger.
		breakdown, err := calculateRefundBreakdown(state, orderItemMap, nil)
		if err != nil {
			return err
		}

		var deductionItems []payouts.ReturnItemDeduction
		for _, it := range breakdown.Items {
			if it.RefundableQuantity > 0 {
				deductionItems = append(deductionItems, payouts.ReturnItemDeduction{
					OrderItemID: it.OrderItemID,
					Quantity:    it.RefundableQuantity,
				})
			}
		}

		if s.payouts != nil && len(deductionItems) > 0 {
			if err := s.payouts.ProcessReturnDeduction(ctx, tx, ret.ID, ret.OrderID, deductionItems); err != nil {
				return err
			}

			// Trigger A: Reconcile compensation for all distinct affected order_items in deterministic order
			distinctOrderItemIDs := make(map[uuid.UUID]struct{})
			for _, it := range deductionItems {
				distinctOrderItemIDs[it.OrderItemID] = struct{}{}
			}
			sortedOrderItemIDs := make([]uuid.UUID, 0, len(distinctOrderItemIDs))
			for id := range distinctOrderItemIDs {
				sortedOrderItemIDs = append(sortedOrderItemIDs, id)
			}
			sort.Slice(sortedOrderItemIDs, func(i, j int) bool {
				return sortedOrderItemIDs[i].String() < sortedOrderItemIDs[j].String()
			})

			for _, orderItemID := range sortedOrderItemIDs {
				if err := s.payouts.ReconcileReturnCompensationTx(ctx, tx, orderItemID); err != nil {
					return err
				}
			}
		}

		// PAYOUT.SAFETY.1: the deduction above is authoritative; the temporary hold for this return is released
		// (hold + release cancel out exactly, so the refund is counted once).
		if s.payouts != nil {
			if err := s.payouts.ReleaseReturnHoldsTx(ctx, tx, returnID); err != nil {
				return err
			}
		}
	}

	return nil
}

// ProcessRefundFailureTx marks a refund as failed; linked Return remains in item_received and no seller deduction occurs.
func (s *Service) ProcessRefundFailureTx(ctx context.Context, tx pgx.Tx, refundID uuid.UUID) error {
	var ref Refund
	queryRef := `
		SELECT id, return_id, payment_id, order_id, status, amount_cents, currency, processed_at, created_at, updated_at
		FROM refunds
		WHERE id = $1
		FOR UPDATE
	`
	err := tx.QueryRow(ctx, queryRef, refundID).Scan(
		&ref.ID, &ref.ReturnID, &ref.PaymentID, &ref.OrderID, &ref.Status, &ref.AmountCents, &ref.Currency, &ref.ProcessedAt, &ref.CreatedAt, &ref.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrRefundNotFound
		}
		return err
	}

	if ref.Status == "failed" {
		return nil // idempotent failure
	}
	if ref.Status == "succeeded" {
		return nil // terminal state: succeeded refund cannot become failed
	}

	_, err = tx.Exec(ctx, `
		UPDATE refunds
		SET status = 'failed', updated_at = now()
		WHERE id = $1
	`, refundID)
	if err != nil {
		return err
	}

	return nil
}

func (s *Service) ProcessRefundSuccess(ctx context.Context, refundID uuid.UUID, processedAt time.Time) error {
	return s.db.RunInTx(ctx, func(tx pgx.Tx) error {
		return s.ProcessRefundSuccessTx(ctx, tx, refundID, processedAt)
	})
}

func (s *Service) ProcessRefundFailure(ctx context.Context, refundID uuid.UUID) error {
	return s.db.RunInTx(ctx, func(tx pgx.Tx) error {
		return s.ProcessRefundFailureTx(ctx, tx, refundID)
	})
}

// SimulateRefundSuccess resolves the exact single pending refund for a return and completes it via canonical ProcessRefundSuccessTx.
func (s *Service) SimulateRefundSuccess(ctx context.Context, returnID uuid.UUID) (*Refund, error) {
	var completedRefund *Refund
	err := s.db.RunInTx(ctx, func(tx pgx.Tx) error {
		pendingList, err := s.repo.GetPendingRefundsByReturnIDTx(ctx, tx, returnID)
		if err != nil {
			return err
		}
		if len(pendingList) == 0 {
			return ErrNoPendingRefund
		}
		if len(pendingList) > 1 {
			return ErrMultiplePendingRefunds
		}
		target := pendingList[0]

		procTime := time.Now().Truncate(time.Microsecond)
		if err := s.ProcessRefundSuccessTx(ctx, tx, target.ID, procTime); err != nil {
			return err
		}

		updated, err := s.repo.GetRefundTx(ctx, tx, target.ID)
		if err != nil {
			return err
		}
		completedRefund = updated
		return nil
	})
	if err != nil {
		return nil, err
	}
	return completedRefund, nil
}

// SimulateRefundFailure resolves the exact single pending refund for a return and fails it via canonical ProcessRefundFailureTx.
func (s *Service) SimulateRefundFailure(ctx context.Context, returnID uuid.UUID) (*Refund, error) {
	var completedRefund *Refund
	err := s.db.RunInTx(ctx, func(tx pgx.Tx) error {
		pendingList, err := s.repo.GetPendingRefundsByReturnIDTx(ctx, tx, returnID)
		if err != nil {
			return err
		}
		if len(pendingList) == 0 {
			return ErrNoPendingRefund
		}
		if len(pendingList) > 1 {
			return ErrMultiplePendingRefunds
		}
		target := pendingList[0]

		if err := s.ProcessRefundFailureTx(ctx, tx, target.ID); err != nil {
			return err
		}

		updated, err := s.repo.GetRefundTx(ctx, tx, target.ID)
		if err != nil {
			return err
		}
		completedRefund = updated
		return nil
	})
	if err != nil {
		return nil, err
	}
	return completedRefund, nil
}
