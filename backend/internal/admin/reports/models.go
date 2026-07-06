package reports

type AdminReportSummary struct {
	TotalOrders       int    `json:"totalOrders"`
	TotalRevenueCents int64  `json:"totalRevenueCents"`
	TotalSellers      int    `json:"totalSellers"`
	ActiveSellers     int    `json:"activeSellers"`
	TotalProducts     int    `json:"totalProducts"`
	PendingProducts   int    `json:"pendingProducts"`
	RejectedProducts  int    `json:"rejectedProducts"`
	PublishedProducts int    `json:"publishedProducts"`
	PendingPayouts    int64  `json:"pendingPayouts"` // In cents based on dashboard summary
	PaidPayouts       int64  `json:"paidPayouts"`    // Can be inferred or left 0 if not tracked natively in summary
	LowStockItems     int    `json:"lowStockItems"`
	OpenComplaints    int    `json:"openComplaints"` // Left 0 if no complaints module
	Currency          string `json:"currency"`
}
