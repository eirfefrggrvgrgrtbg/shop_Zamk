package testlab

import (
	"time"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/selleranalytics"
)

// Calculator provides helper functions to independently build ExpectedResults
// without reading from the Analytics read-model.
type Calculator struct{}

func NewCalculator() *Calculator {
	return &Calculator{}
}

// calcChangePercent replicates the exact formula for percentage change
func (c *Calculator) calcChangePercent(current, previous int64) (*float64, string) {
	if previous == 0 {
		if current == 0 {
			return nil, "unchanged"
		}
		return nil, "new"
	}
	change := float64(current-previous) / float64(previous) * 100.0
	return &change, ""
}

func (c *Calculator) calcCountChangePercent(current, previous int) (*float64, string) {
	return c.calcChangePercent(int64(current), int64(previous))
}

// BuildBasicSales generates the expected result for a simple 1-order scenario.
func (c *Calculator) BuildBasicSales(
	period selleranalytics.TimePeriod,
	orderDate time.Time,
	priceCents int64,
	quantity int,
	commissionRateBPS int,
	productID, variantID string,
) selleranalytics.OverviewResponse {

	grossCents := priceCents * int64(quantity)
	commCents := (grossCents * int64(commissionRateBPS)) / 10000
	netCents := grossCents - commCents

	// In BASIC_SALES, previous period is empty
	changePerc, compState := c.calcChangePercent(grossCents, 0)
	countChangePerc, countCompState := c.calcCountChangePercent(1, 0)
	unitsChangePerc, unitsCompState := c.calcCountChangePercent(quantity, 0)
	aovChangePerc, aovCompState := c.calcChangePercent(grossCents, 0)
	netChangePerc, netCompState := c.calcChangePercent(netCents, 0)

	// Build timeseries
	dateStr := orderDate.Format("2006-01-02")
	tsBucket := selleranalytics.TimeseriesBucketDTO{
		Date:                      dateStr,
		GrossSalesCents:           grossCents,
		OrdersCount:               1,
		UnitsSold:                 quantity,
		CommissionCents:           commCents,
		SellerEarningCents:        netCents,
		ReturnDeductionsCents:     0,
		NetCommercialEarningCents: netCents,
		ReturnedUnits:             0,
	}

	return selleranalytics.OverviewResponse{
		Period: selleranalytics.PeriodDTO{
			From: period.From.Format(time.RFC3339),
			To:   period.To.Format(time.RFC3339),
		},
		HasHistoricalSales: true,
		GrossSales: selleranalytics.MetricCentsDTO{
			CurrentCents:    grossCents,
			PreviousCents:   0,
			ChangePercent:   changePerc,
			ComparisonState: compState,
		},
		Orders: selleranalytics.MetricCountDTO{
			Current:         1,
			Previous:        0,
			ChangePercent:   countChangePerc,
			ComparisonState: countCompState,
		},
		UnitsSold: selleranalytics.MetricCountDTO{
			Current:         quantity,
			Previous:        0,
			ChangePercent:   unitsChangePerc,
			ComparisonState: unitsCompState,
		},
		AverageOrderValue: selleranalytics.MetricCentsDTO{
			CurrentCents:    grossCents, // 1 order
			PreviousCents:   0,
			ChangePercent:   aovChangePerc,
			ComparisonState: aovCompState,
		},
		Commission: selleranalytics.MetricCentsSimpleDTO{
			CurrentCents:  commCents,
			PreviousCents: 0,
		},
		SellerEarningBeforeReturns: selleranalytics.MetricCentsSimpleDTO{
			CurrentCents:  netCents,
			PreviousCents: 0,
		},
		ReturnDeductions: selleranalytics.MetricCentsSimpleDTO{
			CurrentCents:  0,
			PreviousCents: 0,
		},
		OtherAdjustments: selleranalytics.MetricCentsSimpleDTO{
			CurrentCents:  0,
			PreviousCents: 0,
		},
		NetCommercialEarning: selleranalytics.MetricCentsDTO{
			CurrentCents:    netCents,
			PreviousCents:   0,
			ChangePercent:   netChangePerc,
			ComparisonState: netCompState,
		},
		ReturnedUnits: selleranalytics.MetricCountSimpleDTO{
			Current:  0,
			Previous: 0,
		},
		ReturnRate: selleranalytics.MetricPercentDTO{
			CurrentPercent:  0,
			PreviousPercent: 0,
		},
		Timeseries: []selleranalytics.TimeseriesBucketDTO{tsBucket},
		Insights:   []selleranalytics.InsightDTO{},
	}
}

// BuildNeverSold generates the expected result for an empty seller.
func (c *Calculator) BuildNeverSold(period selleranalytics.TimePeriod) selleranalytics.OverviewResponse {
	return selleranalytics.OverviewResponse{
		Period: selleranalytics.PeriodDTO{
			From: period.From.Format(time.RFC3339),
			To:   period.To.Format(time.RFC3339),
		},
		HasHistoricalSales: false,
		GrossSales: selleranalytics.MetricCentsDTO{
			CurrentCents:    0,
			PreviousCents:   0,
			ComparisonState: "unchanged",
		},
		Orders: selleranalytics.MetricCountDTO{
			Current:         0,
			Previous:        0,
			ComparisonState: "unchanged",
		},
		UnitsSold: selleranalytics.MetricCountDTO{
			Current:         0,
			Previous:        0,
			ComparisonState: "unchanged",
		},
		AverageOrderValue: selleranalytics.MetricCentsDTO{
			CurrentCents:    0,
			PreviousCents:   0,
			ComparisonState: "unchanged",
		},
		NetCommercialEarning: selleranalytics.MetricCentsDTO{
			CurrentCents:    0,
			PreviousCents:   0,
			ComparisonState: "unchanged",
		},
		Timeseries: []selleranalytics.TimeseriesBucketDTO{},
		Insights:   []selleranalytics.InsightDTO{},
	}
}

// BuildZeroCurrentPeriod generates expected results when there are sales, but not in current period.
func (c *Calculator) BuildZeroCurrentPeriod(period selleranalytics.TimePeriod, prevGrossCents int64, prevOrders int, prevUnits int, prevCommCents int64) selleranalytics.OverviewResponse {
	// A drop to 0 from something
	grossChange, _ := c.calcChangePercent(0, prevGrossCents)
	ordersChange, _ := c.calcCountChangePercent(0, prevOrders)
	unitsChange, _ := c.calcCountChangePercent(0, prevUnits)
	aovChange, _ := c.calcChangePercent(0, prevGrossCents/int64(prevOrders))
	prevNet := prevGrossCents - prevCommCents
	netChange, _ := c.calcChangePercent(0, prevNet)

	return selleranalytics.OverviewResponse{
		Period: selleranalytics.PeriodDTO{
			From: period.From.Format(time.RFC3339),
			To:   period.To.Format(time.RFC3339),
		},
		HasHistoricalSales: true,
		GrossSales: selleranalytics.MetricCentsDTO{
			CurrentCents:    0,
			PreviousCents:   prevGrossCents,
			ChangePercent:   grossChange,
		},
		Orders: selleranalytics.MetricCountDTO{
			Current:         0,
			Previous:        prevOrders,
			ChangePercent:   ordersChange,
		},
		UnitsSold: selleranalytics.MetricCountDTO{
			Current:         0,
			Previous:        prevUnits,
			ChangePercent:   unitsChange,
		},
		AverageOrderValue: selleranalytics.MetricCentsDTO{
			CurrentCents:    0,
			PreviousCents:   prevGrossCents / int64(prevOrders),
			ChangePercent:   aovChange,
		},
		Commission: selleranalytics.MetricCentsSimpleDTO{
			CurrentCents:  0,
			PreviousCents: prevCommCents,
		},
		SellerEarningBeforeReturns: selleranalytics.MetricCentsSimpleDTO{
			CurrentCents:  0,
			PreviousCents: prevNet,
		},
		NetCommercialEarning: selleranalytics.MetricCentsDTO{
			CurrentCents:    0,
			PreviousCents:   prevNet,
			ChangePercent:   netChange,
		},
		Timeseries: []selleranalytics.TimeseriesBucketDTO{},
		Insights:   []selleranalytics.InsightDTO{},
	}
}

// BuildInventoryAndInbound handles the case where we just test inventory insights without sales.
func (c *Calculator) BuildInventoryAndInbound(period selleranalytics.TimePeriod) selleranalytics.OverviewResponse {
	// For V1, we may just return zero sales but expect an insight to be present.
	// Since insights calculation involves multiple rules, we return the base zero state,
	// and the orchestrator can append the insight.
	base := c.BuildNeverSold(period)
	// We could append a low_stock or out_of_stock insight here if passed in
	return base
}
