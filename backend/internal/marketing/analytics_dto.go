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

type SourceAnalyticsRequest struct {
	From   time.Time `json:"from" query:"from"`
	To     time.Time `json:"to" query:"to"`
	Search *string   `json:"search" query:"search"`
	Sort   *string   `json:"sort" query:"sort"`
}

type SourceKind string

const (
	SourceKindDirect       SourceKind = "direct"
	SourceKindUnattributed SourceKind = "unattributed"
	SourceKindNamed        SourceKind = "named"
)

type SourcePerformanceRow struct {
	SourceKey            *string    `json:"sourceKey"`
	SourceKind           SourceKind `json:"sourceKind"`
	Source               string     `json:"source,omitempty"`
	Visits               int        `json:"visits"`
	PaidOrders           int        `json:"paidOrders"`
	SoldUnits            int        `json:"soldUnits"`
	RevenueCents         int64      `json:"revenueCents"`
	ConversionRate       float64    `json:"conversionRate"`
	ConversionRateBps    int        `json:"conversionRateBps,omitempty"`
	NewCustomers         int        `json:"newCustomers"`
	RepeatCustomers      int        `json:"repeatCustomers"`
	ReturnsCount         int        `json:"returnsCount"`
	PreviousVisits       int        `json:"previousVisits"`
	VisitsChangePct      *float64   `json:"visitsChangePct"`
	PreviousRevenueCents int64      `json:"previousRevenueCents"`
	RevenueChangePct     *float64   `json:"revenueChangePct"`
}

type AnalyticsSourcesResponse struct {
	Sources []SourcePerformanceRow `json:"sources"`
}

type SourceAnalyticsResponse struct {
	Coverage ProductAnalyticsCoverage `json:"coverage"`
	Sources  []SourcePerformanceRow   `json:"sources"`
}

type SourceDetailRequest struct {
	From time.Time `json:"from" query:"from"`
	To   time.Time `json:"to" query:"to"`
}

type SourceDetailResponse struct {
	Source       SourcePerformanceRow     `json:"source"`
	Coverage     ProductAnalyticsCoverage `json:"coverage"`
	Trend        []TrendDataPoint         `json:"trend"`
	TopProducts  []ProductAnalyticsRow    `json:"topProducts"`
	TopDesigners []DesignerAnalyticsRow   `json:"topDesigners"`
	Campaigns    []CampaignPerformanceRow `json:"campaigns"`
}

const (
	SourceSortVisits        = "visits"
	SourceSortRevenue       = "revenue"
	SourceSortOrders        = "orders"
	SourceSortConversion    = "conversion"
	SourceSortRevenueGrowth = "revenue_growth"
	SourceSortRevenueDrop   = "revenue_drop"
)

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

type DesignerAnalyticsRequest struct {
	From       time.Time `json:"from" query:"from"`
	To         time.Time `json:"to" query:"to"`
	CategoryID *string   `json:"categoryId" query:"categoryId"`
	Search     *string   `json:"search" query:"search"`
	Sort       *string   `json:"sort" query:"sort"`
}

type DesignerAnalyticsRow struct {
	DesignerID           string  `json:"designerId"`
	DesignerName         string  `json:"designerName"`
	ProductsCount        int     `json:"productsCount"`
	PrimaryImage         *string `json:"primaryImage"`
	Views                int     `json:"views"`
	Favorites            int     `json:"favorites"`
	AddToCart            int     `json:"addToCart"`
	Purchases            int     `json:"purchases"`
	SoldUnits            int     `json:"soldUnits"`
	RevenueCents         int64   `json:"revenueCents"`
	ConversionRate       float64 `json:"conversionRate"`
	Returns              int     `json:"returns"`
	PreviousRevenueCents int64   `json:"previousRevenueCents"`
	RevenueChangePct     *float64 `json:"revenueChangePct"`
}

const (
	DesignerSortRevenue         = "revenue"
	DesignerSortSales           = "sales"
	DesignerSortViews           = "views"
	DesignerSortFavorites       = "favorites"
	DesignerSortConversion      = "conversion"
	DesignerSortRevenueGrowth   = "revenue_growth"
	DesignerSortRevenueDrop     = "revenue_drop"
	DesignerSortHighViewsLowSales = "high_views_low_sales"
)

type DesignerAnalyticsResponse struct {
	Coverage ProductAnalyticsCoverage `json:"coverage"`
	Designers []DesignerAnalyticsRow  `json:"designers"`
}
