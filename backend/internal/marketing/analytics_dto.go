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
