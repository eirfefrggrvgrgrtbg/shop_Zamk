package products

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// Reasons why an active variant must stay active when the seller removes its color/size cell.
// Deactivation is a soft flag (is_active = false): the row, its SKU, stock records and order history are kept.
// It is refused only while the variant still takes part in physical or commercial operations.
const (
	VariantBlockStock      = "stock"
	VariantBlockReserved   = "reserved"
	VariantBlockOpenOrder  = "open_order"
	VariantBlockOpenReturn = "open_return"
	VariantBlockOpenSupply = "open_supply"
)

var variantBlockMessages = map[string]string{
	VariantBlockStock:      "на складе ZAMK есть остатки",
	VariantBlockReserved:   "есть зарезервированные единицы",
	VariantBlockOpenOrder:  "есть незавершённые заказы",
	VariantBlockOpenReturn: "есть незавершённые возвраты",
	VariantBlockOpenSupply: "вариант есть в незавершённой поставке",
}

type VariantDeactivationBlockedError struct {
	VariantID uuid.UUID
	Label     string
	Reason    string
}

func (e *VariantDeactivationBlockedError) Error() string {
	return fmt.Sprintf("variant %s cannot be deactivated: %s", e.VariantID, e.Reason)
}

// SellerMessage is the seller-facing explanation (no technical ids).
func (e *VariantDeactivationBlockedError) SellerMessage() string {
	label := e.Label
	if label == "" {
		label = "вариант"
	}
	msg, ok := variantBlockMessages[e.Reason]
	if !ok {
		msg = "вариант участвует в складских операциях"
	}
	return fmt.Sprintf("Нельзя убрать «%s»: %s. Вариант сохранён без изменений.", label, msg)
}

func variantLabel(v ProductVariant) string {
	var parts []string
	if v.ColorName != nil && strings.TrimSpace(*v.ColorName) != "" {
		parts = append(parts, strings.TrimSpace(*v.ColorName))
	} else if v.Color != nil && strings.TrimSpace(*v.Color) != "" {
		parts = append(parts, strings.TrimSpace(*v.Color))
	}
	if v.Size != nil && strings.TrimSpace(*v.Size) != "" {
		parts = append(parts, strings.TrimSpace(*v.Size))
	}
	return strings.Join(parts, " / ")
}

// variantDeactivationBlockReason returns "" when the variant may be soft-deactivated. The inventory row is locked
// so a concurrent reservation cannot slip in between the check and the deactivation in the same transaction.
func (r *Repository) variantDeactivationBlockReason(ctx context.Context, variantID uuid.UUID) (string, error) {
	var total, reserved int
	err := r.db.QueryRow(ctx, `
		SELECT COALESCE(SUM(total_stock), 0), COALESCE(SUM(reserved_stock), 0)
		FROM (SELECT total_stock, reserved_stock FROM inventory_items WHERE product_variant_id = $1 FOR UPDATE) ii
	`, variantID).Scan(&total, &reserved)
	if err != nil {
		return "", fmt.Errorf("failed to check variant stock: %w", err)
	}
	if reserved > 0 {
		return VariantBlockReserved, nil
	}
	if total > 0 {
		return VariantBlockStock, nil
	}

	var activeReservation, openOrder, openReturn, openSupply bool
	err = r.db.QueryRow(ctx, `
		SELECT
			EXISTS (SELECT 1 FROM reservations WHERE product_variant_id = $1 AND status = 'active'),
			EXISTS (
				SELECT 1 FROM order_items oi JOIN orders o ON o.id = oi.order_id
				WHERE oi.product_variant_id = $1
				  AND o.status IN ('created', 'awaiting_payment', 'paid', 'assembling', 'packed', 'shipped')
			),
			EXISTS (
				SELECT 1 FROM return_items ri
				JOIN returns rt ON rt.id = ri.return_id
				JOIN order_items oi ON oi.id = ri.order_item_id
				WHERE oi.product_variant_id = $1
				  AND rt.status NOT IN ('completed', 'rejected', 'cancelled')
			),
			(
				EXISTS (
					SELECT 1 FROM seller_supply_items si JOIN seller_supplies s ON s.id = si.supply_id
					WHERE si.variant_id = $1
					  AND s.status NOT IN ('completed', 'completed_with_discrepancies', 'cancelled')
				) OR
				EXISTS (SELECT 1 FROM inventory_units WHERE product_variant_id = $1 AND status IN ('expected', 'warehouse'))
			)
	`, variantID).Scan(&activeReservation, &openOrder, &openReturn, &openSupply)
	if err != nil {
		return "", fmt.Errorf("failed to check variant operations: %w", err)
	}
	switch {
	case activeReservation:
		return VariantBlockReserved, nil
	case openOrder:
		return VariantBlockOpenOrder, nil
	case openReturn:
		return VariantBlockOpenReturn, nil
	case openSupply:
		return VariantBlockOpenSupply, nil
	}
	return "", nil
}
