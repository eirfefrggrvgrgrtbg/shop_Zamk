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
	var returns7d, prevReturns7d int
	err := r.db.QueryRow(ctx, `
		SELECT 
			COUNT(*),
			COUNT(*) FILTER (WHERE created_at >= CURRENT_DATE),
			COUNT(*) FILTER (WHERE status IN ('pending_payment', 'created', 'pending')),
			COUNT(*) FILTER (WHERE status = 'paid'),
			COUNT(*) FILTER (WHERE status = 'in_fulfillment'),
			COUNT(*) FILTER (WHERE status IN ('shipped', 'delivered')),
			COUNT(*) FILTER (WHERE status IN ('cancelled', 'refunded')),
			COUNT(*) FILTER (WHERE status IN ('cancelled', 'refunded') AND created_at >= CURRENT_DATE - INTERVAL '7 days'),
			COUNT(*) FILTER (WHERE status IN ('cancelled', 'refunded') AND created_at >= CURRENT_DATE - INTERVAL '14 days' AND created_at < CURRENT_DATE - INTERVAL '7 days')
		FROM orders
	`).Scan(&totalOrders, &ordersToday, &newOrPending, &paid, &inFulfillment, &shippedDelivered, &cancelledRefunded, &returns7d, &prevReturns7d)
	if err != nil { return nil, err }

	summary.Overview.TotalOrders = totalOrders
	summary.Overview.OrdersToday = ordersToday
	summary.Overview.Returns7d = returns7d
	summary.Overview.PreviousReturns7d = prevReturns7d
	
	summary.Orders = OrdersMetrics{
		NewOrPending:        newOrPending,
		Paid:                paid,
		InFulfillment:       inFulfillment,
		ShippedOrDelivered:  shippedDelivered,
		CancelledOrRefunded: cancelledRefunded,
	}

	// Query Revenue and Averages
	var revToday, rev7d, paidOrdersSum, prevRev7d, aov7d, prevAov7d, avgRev20d int64
	var avgOrders20d float64
	err = r.db.QueryRow(ctx, `
		SELECT 
			COALESCE(SUM(total_price_cents) FILTER (WHERE created_at >= CURRENT_DATE AND status NOT IN ('cancelled', 'refunded')), 0)::bigint,
			COALESCE(SUM(total_price_cents) FILTER (WHERE created_at >= CURRENT_DATE - INTERVAL '7 days' AND status NOT IN ('cancelled', 'refunded')), 0)::bigint,
			COALESCE(SUM(total_price_cents) FILTER (WHERE status = 'paid'), 0)::bigint,
			COALESCE(SUM(total_price_cents) FILTER (WHERE created_at >= CURRENT_DATE - INTERVAL '14 days' AND created_at < CURRENT_DATE - INTERVAL '7 days' AND status NOT IN ('cancelled', 'refunded')), 0)::bigint,
			COALESCE((COUNT(*) FILTER (WHERE created_at >= CURRENT_DATE - INTERVAL '20 days' AND created_at < CURRENT_DATE))::float / 20.0, 0),
			COALESCE((SUM(total_price_cents) FILTER (WHERE created_at >= CURRENT_DATE - INTERVAL '20 days' AND created_at < CURRENT_DATE AND status NOT IN ('cancelled', 'refunded'))) / 20, 0)::bigint,
			COALESCE(SUM(total_price_cents) FILTER (WHERE status = 'paid' AND created_at >= CURRENT_DATE - INTERVAL '7 days') / NULLIF(COUNT(*) FILTER (WHERE status = 'paid' AND created_at >= CURRENT_DATE - INTERVAL '7 days'), 0), 0)::bigint,
			COALESCE(SUM(total_price_cents) FILTER (WHERE status = 'paid' AND created_at >= CURRENT_DATE - INTERVAL '14 days' AND created_at < CURRENT_DATE - INTERVAL '7 days') / NULLIF(COUNT(*) FILTER (WHERE status = 'paid' AND created_at >= CURRENT_DATE - INTERVAL '14 days' AND created_at < CURRENT_DATE - INTERVAL '7 days'), 0), 0)::bigint
		FROM orders
	`).Scan(&revToday, &rev7d, &paidOrdersSum, &prevRev7d, &avgOrders20d, &avgRev20d, &aov7d, &prevAov7d)
	if err != nil { return nil, err }
	summary.Overview.RevenueTodayCents = revToday
	summary.Overview.Revenue7dCents = rev7d
	summary.Overview.PreviousRevenue7dCents = prevRev7d
	summary.Overview.AverageDailyOrders20d = avgOrders20d
	summary.Overview.AverageDailyRevenue20dCents = avgRev20d
	summary.Overview.AverageOrderValue7dCents = aov7d
	summary.Overview.PreviousAverageOrderValue7dCents = prevAov7d
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
			COUNT(*) FILTER (WHERE (total_stock - reserved_stock) > 0 AND (total_stock - reserved_stock) <= 5),
			COALESCE(SUM(reserved_stock), 0),
			COUNT(*) FILTER (WHERE (total_stock - reserved_stock) = 0)
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
	var pendingPayouts, paidPayouts int64
	err = r.db.QueryRow(ctx, `
		SELECT 
			COALESCE(SUM(amount_cents) FILTER (WHERE status = 'requested'), 0)::bigint,
			COALESCE(SUM(amount_cents) FILTER (WHERE status IN ('paid', 'completed')), 0)::bigint
		FROM payouts
	`).Scan(&pendingPayouts, &paidPayouts)
	if err == nil {
		summary.Payments.PendingPayoutsCents = pendingPayouts
		summary.Payments.PaidPayoutsCents = paidPayouts
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
