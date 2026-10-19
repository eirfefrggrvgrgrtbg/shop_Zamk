package selleranalytics

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrProductNotFound   = errors.New("product not found")
	ErrCohortDataAnomaly = errors.New("cohort return quantity exceeds delivered quantity")
)

type TimePeriod struct {
	From time.Time
	To   time.Time
}

// LedgerSummary aggregates all relevant ledger entries in a period
type LedgerSummary struct {
	GrossSalesCents        int64
	CommissionCents        int64
	SellerEarningCents     int64
	ReturnDeductionsCents  int64
	OtherAdjustmentsCents  int64
}

// TimeseriesRow represents a single day's aggregated metrics
type TimeseriesRow struct {
	Date                      time.Time
	GrossSalesCents           int64
	OrdersCount               int
	UnitsSold                 int
	CommissionCents           int64
	SellerEarningCents        int64
	ReturnDeductionsCents     int64
	NetCommercialEarningCents int64
	ReturnedUnits             int
}

// ProductPerformance represents metrics for a single product across a period
type ProductPerformance struct {
	ProductID           uuid.UUID
	Title               string
	GrossSalesCents     int64
	OrdersCount         int
	UnitsSold           int
	CohortReturnedUnits int
	ReturnedUnits       int
	AvailableStock      int
	HasItemAnomaly      bool
}

// VariantPerformance represents metrics for a single variant across a period
type VariantPerformance struct {
	VariantID           uuid.UUID
	ProductID           uuid.UUID
	SKU                 string
	DisplayName         string
	UnitsSold           int
	GrossSalesCents     int64
	CohortReturnedUnits int
	ReturnedUnits       int
	AvailableStock      int
	HasItemAnomaly      bool
}

// InventoryPerformance represents stock/inbound details for a product/variant
type InventoryPerformance struct {
	ProductID    uuid.UUID
	VariantID    uuid.UUID
	ProductTitle string
	SKU          string
	DisplayName  string
	Available    int
	OnHand       int
	Reserved     int
	Inbound      int
	UnitsSold    int
}
