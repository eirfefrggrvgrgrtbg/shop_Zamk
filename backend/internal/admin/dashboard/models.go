package dashboard

type DashboardSummary struct {
	Overview  OverviewMetrics  `json:"overview"`
	Orders    OrdersMetrics    `json:"orders"`
	Sellers   SellersMetrics   `json:"sellers"`
	Products  ProductsMetrics  `json:"products"`
	Auctions  AuctionsMetrics  `json:"auctions"`
	Inventory InventoryMetrics `json:"inventory"`
	Payments  PaymentsMetrics  `json:"payments"`
	Attention []AttentionItem  `json:"attention"`
}

type OverviewMetrics struct {
	TotalOrders       int   `json:"totalOrders"`
	OrdersToday       int   `json:"ordersToday"`
	RevenueTodayCents int64 `json:"revenueTodayCents"`
	Revenue7dCents    int64 `json:"revenue7dCents"`
	PendingModeration int   `json:"pendingModeration"`
	ActiveSellers     int   `json:"activeSellers"`
	ActiveProducts    int   `json:"activeProducts"`
	LowStockCount     int   `json:"lowStockCount"`

	AverageDailyOrders20d            float64 `json:"averageDailyOrders20d"`
	AverageDailyRevenue20dCents      int64   `json:"averageDailyRevenue20dCents"`
	PreviousRevenue7dCents           int64   `json:"previousRevenue7dCents"`
	AverageOrderValue7dCents         int64   `json:"averageOrderValue7dCents"`
	PreviousAverageOrderValue7dCents int64   `json:"previousAverageOrderValue7dCents"`
	Returns7d                        int     `json:"returns7d"`
	PreviousReturns7d                int     `json:"previousReturns7d"`
}

type OrdersMetrics struct {
	NewOrPending        int `json:"newOrPending"`
	Paid                int `json:"paid"`
	InFulfillment       int `json:"inFulfillment"`
	ShippedOrDelivered  int `json:"shippedOrDelivered"`
	CancelledOrRefunded int `json:"cancelledOrRefunded"`
	RequiresPicking     int `json:"requiresPicking"`
}

type SellersMetrics struct {
	Active            int `json:"active"`
	WaitingModeration int `json:"waitingModeration"`
	Blocked           int `json:"blocked"`
}

type ProductsMetrics struct {
	Published         int `json:"published"`
	PendingModeration int `json:"pendingModeration"`
	RejectedOrBlocked int `json:"rejectedOrBlocked"`
	OutOfStock        int `json:"outOfStock"`
}

type AuctionsMetrics struct {
	Active             int `json:"active"`
	AwaitingPayment    int `json:"awaitingPayment"`
	UnpaidManualReview int `json:"unpaidManualReview"`
	DirectSaleItems    int `json:"directSaleItems"`
}

type InventoryMetrics struct {
	LowStockVariants int `json:"lowStockVariants"`
	ReservedStock    int `json:"reservedStock"`
	OutOfStockCount  int `json:"outOfStockCount"`
}

type PaymentsMetrics struct {
	PaidOrdersSumCents  int64 `json:"paidOrdersSumCents"`
	PendingPayoutsCents int64 `json:"pendingPayoutsCents"`
	PaidPayoutsCents    int64 `json:"paidPayoutsCents"`
	FailedPaymentsCount int   `json:"failedPaymentsCount"`
}

type AttentionItem struct {
	Title    string `json:"title"`
	Count    int    `json:"count"`
	Severity string `json:"severity"` // info, warning, danger
	Link     string `json:"link,omitempty"`
}
