package orders

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Seller order queue filters. The value set is closed: the predicate is built from this whitelist, never from input.
const (
	SellerOrderFilterAll        = "all"
	SellerOrderFilterNew        = "new"
	SellerOrderFilterProcessing = "processing"
	SellerOrderFilterShipped    = "shipped"
	SellerOrderFilterDelivered  = "delivered"
	SellerOrderFilterReturns    = "returns"
)

var ErrInvalidSellerOrderFilter = errors.New("invalid seller order filter")

func sellerOrderFilterPredicate(filter string) (string, error) {
	switch filter {
	case "", SellerOrderFilterAll:
		return "TRUE", nil
	case SellerOrderFilterNew:
		return "COALESCE(f.status, o.status) IN ('created', 'awaiting_payment', 'paid')", nil
	case SellerOrderFilterProcessing:
		return "COALESCE(f.status, o.status) IN ('assembling', 'packed', 'accepted', 'discrepancy')", nil
	case SellerOrderFilterShipped:
		return "COALESCE(f.status, o.status) = 'shipped'", nil
	case SellerOrderFilterDelivered:
		return "COALESCE(f.status, o.status) = 'delivered'", nil
	case SellerOrderFilterReturns:
		return "(rt.refunded_units + rt.open_units) > 0", nil
	}
	return "", ErrInvalidSellerOrderFilter
}

// SellerFulfillmentStage maps the internal fulfillment/order status to the seller-facing stage. Warehouse-internal
// receiving states (accepted, discrepancy) are not exposed: for the seller they are "in processing by ZAMK".
func SellerFulfillmentStage(status string) string {
	switch status {
	case "created", "awaiting_payment":
		return "awaiting_payment"
	case "paid":
		return "paid"
	case "assembling", "packed", "accepted", "discrepancy":
		return "processing"
	case "shipped", "delivered", "cancelled":
		return status
	case "returned", "refunded":
		return "delivered"
	}
	return "processing"
}

// sellerDeliveryStatus is the stage name shown to the seller: the internal receiving states collapse into "assembling".
func sellerDeliveryStatus(status string) string {
	switch status {
	case "created":
		return "awaiting_payment"
	case "accepted", "discrepancy":
		return "assembling"
	case "returned", "refunded":
		return "delivered"
	}
	return status
}

func sellerNextStepOwner(stage string) string {
	switch stage {
	case "awaiting_payment":
		return "buyer"
	case "paid", "processing", "shipped":
		return "zamk"
	}
	return "none"
}

const sellerOrdersFrom = `
	FROM orders o
	JOIN order_items oi ON oi.order_id = o.id AND oi.seller_id = $1
	LEFT JOIN order_fulfillments f ON f.order_id = o.id AND f.seller_id = $1
	LEFT JOIN LATERAL (
		SELECT
			COALESCE(SUM(ri.quantity) FILTER (WHERE r.status IN ('refunded', 'completed')), 0) AS refunded_units,
			COALESCE(SUM(ri.quantity) FILTER (WHERE r.status IN ('requested', 'approved', 'receiving', 'item_received')), 0) AS open_units
		FROM return_items ri
		JOIN returns r ON r.id = ri.return_id
		JOIN order_items oi2 ON oi2.id = ri.order_item_id
		WHERE oi2.order_id = o.id AND oi2.seller_id = $1
	) rt ON TRUE`

const sellerOrderSelect = `
	SELECT
		o.id, o.order_number, o.created_at, o.status,
		COALESCE(f.status, o.status),
		COUNT(oi.id), COALESCE(SUM(oi.quantity), 0), COALESCE(SUM(oi.subtotal_price_cents), 0),
		rt.refunded_units, rt.open_units`

const sellerOrderGroupBy = ` GROUP BY o.id, o.order_number, o.created_at, o.status, f.status, rt.refunded_units, rt.open_units`

// ListSellerOrders returns one page of the seller's own order lines grouped per order, plus the total number of
// orders matching the filter. Only the seller's lines are read; other sellers' lines of the same order never are.
func (r *Repository) ListSellerOrders(ctx context.Context, sellerID uuid.UUID, filter string, limit, offset int) ([]SellerOrder, int, error) {
	pred, err := sellerOrderFilterPredicate(filter)
	if err != nil {
		return nil, 0, err
	}

	var total int
	countQ := `SELECT COUNT(DISTINCT o.id)` + sellerOrdersFrom + ` WHERE ` + pred
	if err := r.db.QueryRow(ctx, countQ, sellerID).Scan(&total); err != nil {
		return nil, 0, err
	}

	q := sellerOrderSelect + sellerOrdersFrom + ` WHERE ` + pred + sellerOrderGroupBy +
		` ORDER BY o.created_at DESC, o.id DESC LIMIT $2 OFFSET $3`
	rows, err := r.db.Query(ctx, q, sellerID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	orders, err := scanSellerOrders(rows)
	if err != nil {
		return nil, 0, err
	}
	if err := r.attachSellerOrderItems(ctx, sellerID, orders); err != nil {
		return nil, 0, err
	}
	return orders, total, nil
}

func (r *Repository) GetSellerOrder(ctx context.Context, sellerID, orderID uuid.UUID) (*SellerOrder, error) {
	q := sellerOrderSelect + sellerOrdersFrom + ` WHERE o.id = $2` + sellerOrderGroupBy
	rows, err := r.db.Query(ctx, q, sellerID, orderID)
	if err != nil {
		return nil, err
	}
	orders, err := scanSellerOrders(rows)
	if err != nil {
		return nil, err
	}
	if len(orders) == 0 {
		return nil, ErrOrderNotFound
	}
	if err := r.attachSellerOrderItems(ctx, sellerID, orders); err != nil {
		return nil, err
	}
	return &orders[0], nil
}

func scanSellerOrders(rows pgx.Rows) ([]SellerOrder, error) {
	defer rows.Close()
	orders := make([]SellerOrder, 0)
	for rows.Next() {
		var o SellerOrder
		var orderStatus, rawStage string
		var refunded, open int
		if err := rows.Scan(&o.ID, &o.OrderNumber, &o.CreatedAt, &orderStatus, &rawStage,
			&o.SellerItemCount, &o.SellerUnits, &o.SellerGrossAmount, &refunded, &open); err != nil {
			return nil, err
		}
		o.CommercialStatus = orderStatus
		o.DeliveryStatus = sellerDeliveryStatus(rawStage)
		o.FulfillmentStage = SellerFulfillmentStage(rawStage)
		o.NextStepOwner = sellerNextStepOwner(o.FulfillmentStage)
		o.HasReturns = refunded+open > 0
		o.Items = make([]SellerOrderItem, 0)
		orders = append(orders, o)
	}
	return orders, rows.Err()
}

func (r *Repository) attachSellerOrderItems(ctx context.Context, sellerID uuid.UUID, orders []SellerOrder) error {
	if len(orders) == 0 {
		return nil
	}
	ids := make([]uuid.UUID, len(orders))
	idx := make(map[uuid.UUID]int, len(orders))
	for i, o := range orders {
		ids[i] = o.ID
		idx[o.ID] = i
	}
	rows, err := r.db.Query(ctx, fmt.Sprintf(`
		SELECT oi.id, oi.order_id, oi.product_id, oi.product_variant_id, oi.seller_id, oi.title, oi.product_slug,
			oi.variant_size, oi.variant_color, oi.sku, oi.image_url, oi.price_cents, oi.quantity,
			oi.subtotal_price_cents, oi.created_at,
			COALESCE(f.status, o.status),
			COALESCE(SUM(ri.quantity) FILTER (WHERE r.status = 'requested'), 0),
			COALESCE(SUM(ri.quantity) FILTER (WHERE r.status IN ('approved', 'receiving', 'item_received')), 0),
			COALESCE(SUM(ri.quantity) FILTER (WHERE r.status IN ('refunded', 'completed')), 0)
		FROM order_items oi
		JOIN orders o ON o.id = oi.order_id
		LEFT JOIN order_fulfillments f ON f.order_id = o.id AND f.seller_id = oi.seller_id
		LEFT JOIN return_items ri ON ri.order_item_id = oi.id
		LEFT JOIN returns r ON r.id = ri.return_id
		WHERE oi.seller_id = $1 AND oi.order_id = ANY($2)
		GROUP BY oi.id, o.status, f.status
		ORDER BY oi.created_at ASC, oi.id ASC`), sellerID, ids)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var it SellerOrderItem
		var rawStage string
		if err := rows.Scan(&it.ID, &it.OrderID, &it.ProductID, &it.ProductVariantID, &it.SellerID, &it.Title,
			&it.ProductSlug, &it.VariantSize, &it.VariantColor, &it.Sku, &it.ImageURL, &it.PriceCents, &it.Quantity,
			&it.SubtotalPriceCents, &it.CreatedAt, &rawStage,
			&it.ReturnRequestedUnits, &it.ReturnInProgressUnits, &it.ReturnRefundedUnits); err != nil {
			return err
		}
		it.FulfillmentStage = SellerFulfillmentStage(rawStage)
		i := idx[it.OrderID]
		orders[i].Items = append(orders[i].Items, it)
	}
	return rows.Err()
}
