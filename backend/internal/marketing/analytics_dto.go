package marketing

import "time"

type AnalyticsOverviewRequest struct {
	From time.Time `json:"from" query:"from"`
	To   time.Time `json:"to" query:"to"`
}

type MetricsSnapshot struct {
	Visits              int   `json:"visits"`
	PaidOrders          int   `json:"paidOrders"`
	SoldUnits           int   `json:"soldUnits"`
	RevenueCents        int64 `json:"revenueCents"`
	AovCents            int64 `json:"aovCents"`
	ConversionRateBps   int   `json:"conversionRateBps"`
	NewCustomers        int   `json:"newCustomers"`
	RepeatCustomers     int   `json:"repeatCustomers"`
	ReturnsCount        int   `json:"returnsCount"`
	ReturnedUnits       int   `json:"returnedUnits"`
	ReturnedAmountCents int64 `json:"returnedAmountCents"`
}

type AnalyticsOverviewResponse struct {
	Current  MetricsSnapshot `json:"current"`
	Previous MetricsSnapshot `json:"previous"`
}

type SourcePerformanceRow struct {
	Source            string `json:"source"`
	Visits            int    `json:"visits"`
	PaidOrders        int    `json:"paidOrders"`
	RevenueCents      int64  `json:"revenueCents"`
	ConversionRateBps int    `json:"conversionRateBps"`
}

type AnalyticsSourcesResponse struct {
	Sources []SourcePerformanceRow `json:"sources"`
}

type CampaignPerformanceRow struct {
	Status             CampaignStatus `json:"status,omitempty"`
	PlannedBudgetCents *int64         `json:"plannedBudgetCents,omitempty"`
	CampaignID         *string        `json:"campaignId"`
	Name               string         `json:"name"`
	Visits             int            `json:"visits"`
	PaidOrders         int            `json:"paidOrders"`
	RevenueCents       int64          `json:"revenueCents"`
	ConversionRateBps  int            `json:"conversionRateBps"`
}

type AnalyticsCampaignsResponse struct {
	Campaigns []CampaignPerformanceRow `json:"campaigns"`
}

type TrendDataPoint struct {
	Date         string `json:"date"`
	RevenueCents int64  `json:"revenueCents"`
	PaidOrders   int    `json:"paidOrders"`
}

type AnalyticsTrendResponse struct {
	Trend []TrendDataPoint `json:"trend"`
}

type ProductAnalyticsRequest struct {
	From       time.Time `json:"from" query:"from"`
	To         time.Time `json:"to" query:"to"`
	CategoryID *string   `json:"categoryId" query:"categoryId"`
	DesignerID *string   `json:"designerId" query:"designerId"`
	Search     *string   `json:"search" query:"search"`
	Sort       *string   `json:"sort" query:"sort"`
	Direction  *string   `json:"direction" query:"direction"`
}

type MetricCoverageStatus string

const (
	CoverageAvailable   MetricCoverageStatus = "available"
	CoveragePartial     MetricCoverageStatus = "partial"
	CoverageUnavailable MetricCoverageStatus = "unavailable"
)

type MetricCoverage struct {
	Status      MetricCoverageStatus `json:"status"`
	TrackedFrom *time.Time           `json:"trackedFrom,omitempty"`
}

type ProductAnalyticsCoverage struct {
	Views     MetricCoverage `json:"views"`
	Favorites MetricCoverage `json:"favorites"`
	AddToCart MetricCoverage `json:"addToCart"`
}

const (
	ProductSortRevenue           = "revenue"
	ProductSortSales             = "sales"
	ProductSortViews             = "views"
	ProductSortFavorites         = "favorites"
	ProductSortConversion        = "conversion"
	ProductSortHighViewsLowSales = "high_views_low_sales"
)

type ProductAnalyticsRow struct {
	ProductID      string  `json:"productId"`
	ProductName    string  `json:"productName"`
	PrimaryImage   *string `json:"primaryImage"`
	DesignerName   *string `json:"designerName"`
	CategoryName   *string `json:"categoryName"`
	Views          int     `json:"views"`
	Favorites      int     `json:"favorites"`
	AddToCart      int     `json:"addToCart"`
	Purchases      int     `json:"purchases"`
	SoldUnits      int     `json:"soldUnits"`
	RevenueCents   int64   `json:"revenueCents"`
	ConversionRate float64 `json:"conversionRate"` // decimal (purchases / views), optional formatting on frontend
	Returns        int     `json:"returns"`
	ReturnedUnits  int     `json:"returnedUnits"`
}

type ProductAnalyticsResponse struct {
	Coverage ProductAnalyticsCoverage `json:"coverage"`
	Products []ProductAnalyticsRow    `json:"products"`
}
