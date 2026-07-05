package dashboard

import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) GetSummary(ctx context.Context) (*DashboardSummary, error) {
	summary := &DashboardSummary{}
	
	// Query Orders
	var totalOrders, ordersToday, newOrPending, paid, inFulfillment, shippedDelivered, cancelledRefunded int
	err := r.db.QueryRow(ctx, `
		SELECT 
			COUNT(*),
			COUNT(*) FILTER (WHERE created_at >= CURRENT_DATE),
			COUNT(*) FILTER (WHERE status IN ('pending_payment', 'created', 'pending')),
			COUNT(*) FILTER (WHERE status = 'paid'),
			COUNT(*) FILTER (WHERE status = 'in_fulfillment'),
			COUNT(*) FILTER (WHERE status IN ('shipped', 'delivered')),
			COUNT(*) FILTER (WHERE status IN ('cancelled', 'refunded'))
		FROM orders
	`).Scan(&totalOrders, &ordersToday, &newOrPending, &paid, &inFulfillment, &shippedDelivered, &cancelledRefunded)
	if err != nil { return nil, err }

	summary.Overview.TotalOrders = totalOrders
	summary.Overview.OrdersToday = ordersToday
	summary.Orders = OrdersMetrics{
		NewOrPending:        newOrPending,
		Paid:                paid,
		InFulfillment:       inFulfillment,
		ShippedOrDelivered:  shippedDelivered,
		CancelledOrRefunded: cancelledRefunded,
	}

	// Query Revenue
	var revToday, rev7d, paidOrdersSum int64
	err = r.db.QueryRow(ctx, `
		SELECT 
			COALESCE(SUM(total_price_cents) FILTER (WHERE created_at >= CURRENT_DATE AND status NOT IN ('cancelled', 'refunded')), 0),
			COALESCE(SUM(total_price_cents) FILTER (WHERE created_at >= CURRENT_DATE - INTERVAL '7 days' AND status NOT IN ('cancelled', 'refunded')), 0),
			COALESCE(SUM(total_price_cents) FILTER (WHERE status = 'paid'), 0)
		FROM orders
	`).Scan(&revToday, &rev7d, &paidOrdersSum)
	if err != nil { return nil, err }
	summary.Overview.RevenueTodayCents = revToday
	summary.Overview.Revenue7dCents = rev7d
	summary.Payments.PaidOrdersSumCents = paidOrdersSum

	// Query Sellers
	var activeSellers, waitingModSellers, blockedSellers int
	err = r.db.QueryRow(ctx, `
		SELECT 
			COUNT(*) FILTER (WHERE status = 'active'),
			COUNT(*) FILTER (WHERE status = 'pending'),
			COUNT(*) FILTER (WHERE status = 'blocked')
		FROM sellers
	`).Scan(&activeSellers, &waitingModSellers, &blockedSellers)
	if err != nil { return nil, err }
	summary.Overview.ActiveSellers = activeSellers
	summary.Sellers = SellersMetrics{
		Active:            activeSellers,
		WaitingModeration: waitingModSellers,
		Blocked:           blockedSellers,
	}

	// Query Products
	var pubProducts, modProducts, rejProducts int
	err = r.db.QueryRow(ctx, `
		SELECT 
			COUNT(*) FILTER (WHERE status = 'published'),
			COUNT(*) FILTER (WHERE status = 'pending_moderation'),
			COUNT(*) FILTER (WHERE status IN ('rejected', 'blocked'))
		FROM products
	`).Scan(&pubProducts, &modProducts, &rejProducts)
	if err != nil { return nil, err }
	
	// Inventory
	var lowStock, reserved, oosCount int
	err = r.db.QueryRow(ctx, `
		SELECT 
			COUNT(*) FILTER (WHERE available_quantity > 0 AND available_quantity <= 5),
			COALESCE(SUM(reserved_quantity), 0),
			COUNT(*) FILTER (WHERE available_quantity = 0)
		FROM inventory_items
	`).Scan(&lowStock, &reserved, &oosCount)
	if err != nil { return nil, err }

	summary.Overview.ActiveProducts = pubProducts
	summary.Overview.PendingModeration = modProducts
	summary.Overview.LowStockCount = lowStock
	summary.Products = ProductsMetrics{
		Published:         pubProducts,
		PendingModeration: modProducts,
		RejectedOrBlocked: rejProducts,
		OutOfStock:        oosCount,
	}
	summary.Inventory = InventoryMetrics{
		LowStockVariants: lowStock,
		ReservedStock:    reserved,
		OutOfStockCount:  oosCount,
	}

	// Query Auctions
	var actAuctions, awaitPay, unpaidRev, dirSale int
	err = r.db.QueryRow(ctx, `
		SELECT 
			COUNT(*) FILTER (WHERE status = 'active'),
			COUNT(*) FILTER (WHERE status = 'won_pending_payment'),
			COUNT(*) FILTER (WHERE status = 'unpaid_manual_review'),
			COUNT(*) FILTER (WHERE status = 'moved_to_direct_sale')
		FROM auction_lots
	`).Scan(&actAuctions, &awaitPay, &unpaidRev, &dirSale)
	if err == nil {
		summary.Auctions = AuctionsMetrics{
			Active:             actAuctions,
			AwaitingPayment:    awaitPay,
			UnpaidManualReview: unpaidRev,
			DirectSaleItems:    dirSale,
		}
	}

	// Payouts
	var pendingPayouts int64
	err = r.db.QueryRow(ctx, `
		SELECT COALESCE(SUM(amount_cents), 0)
		FROM payouts
		WHERE status = 'requested'
	`).Scan(&pendingPayouts)
	if err == nil {
		summary.Payments.PendingPayoutsCents = pendingPayouts
	}

	// Payments
	var failedPay int
	err = r.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM payments WHERE status = 'failed'
	`).Scan(&failedPay)
	if err == nil {
		summary.Payments.FailedPaymentsCount = failedPay
	}

	return summary, nil
}
