package selleranalytics

type PeriodDTO struct {
	From     string `json:"from"`
	To       string `json:"to"`
	Timezone string `json:"timezone"`
}

type MetricCentsDTO struct {
	CurrentCents    int64   `json:"currentCents"`
	PreviousCents   int64   `json:"previousCents"`
	ChangePercent   *float64 `json:"changePercent"`
	ComparisonState string  `json:"comparisonState"`
}

type MetricCountDTO struct {
	Current         int     `json:"current"`
	Previous        int     `json:"previous"`
	ChangePercent   *float64 `json:"changePercent"`
	ComparisonState string  `json:"comparisonState"`
}

type MetricCentsSimpleDTO struct {
	CurrentCents  int64 `json:"currentCents"`
	PreviousCents int64 `json:"previousCents"`
}

type MetricCountSimpleDTO struct {
	Current  int `json:"current"`
	Previous int `json:"previous"`
}

type MetricPercentDTO struct {
	CurrentPercent  float64 `json:"currentPercent"`
	PreviousPercent float64 `json:"previousPercent"`
}

type TimeseriesBucketDTO struct {
	Date                      string `json:"date"`
	GrossSalesCents           int64  `json:"grossSalesCents"`
	OrdersCount               int    `json:"ordersCount"`
	UnitsSold                 int    `json:"unitsSold"`
	CommissionCents           int64  `json:"commissionCents"`
	SellerEarningCents        int64  `json:"sellerEarningCents"`
	ReturnDeductionsCents     int64  `json:"returnDeductionsCents"`
	NetCommercialEarningCents int64  `json:"netCommercialEarningCents"`
	ReturnedUnits             int    `json:"returnedUnits"`
}

type OverviewResponse struct {
	Period                     PeriodDTO            `json:"period"`
	HasHistoricalSales         bool                 `json:"hasHistoricalSales"`
	GrossSales                 MetricCentsDTO       `json:"grossSales"`
	Orders                     MetricCountDTO       `json:"orders"`
	UnitsSold                  MetricCountDTO       `json:"unitsSold"`
	AverageOrderValue          MetricCentsDTO       `json:"averageOrderValue"`
	Commission                 MetricCentsSimpleDTO `json:"commission"`
	SellerEarningBeforeReturns MetricCentsSimpleDTO `json:"sellerEarningBeforeReturns"`
	ReturnDeductions           MetricCentsSimpleDTO `json:"returnDeductions"`
	OtherAdjustments           MetricCentsSimpleDTO `json:"otherAdjustments"`
	NetCommercialEarning       MetricCentsDTO       `json:"netCommercialEarning"`
	ReturnedUnits              MetricCountSimpleDTO `json:"returnedUnits"`
	ReturnRate                 MetricPercentDTO     `json:"returnRate"`
	Timeseries                 []TimeseriesBucketDTO `json:"timeseries"`
	Insights                   []InsightDTO          `json:"insights"`
}

type ProductRow struct {
	ProductID               string  `json:"productId"`
	Title                   string  `json:"title"`
	GrossSalesCents         int64   `json:"grossSalesCents"`
	OrdersCount             int     `json:"ordersCount"`
	UnitsSold               int     `json:"unitsSold"`
	ReturnedUnits           int     `json:"returnedUnits"`
	ReturnRatePercent       float64 `json:"returnRatePercent"`
	AvailableStock          int     `json:"availableStock"`
	PreviousGrossSalesCents int64   `json:"previousGrossSalesCents"`
	GrossSalesChangePercent *float64 `json:"grossSalesChangePercent"`
	ComparisonState         string  `json:"comparisonState"`
}

type ProductsResponse struct {
	Items      []ProductRow `json:"items"`
	TotalCount int          `json:"totalCount"`
}

type VariantRow struct {
	VariantID          string   `json:"variantId"`
	SKU                string   `json:"sku"`
	DisplayName        string   `json:"displayName"`
	UnitsSold          int      `json:"unitsSold"`
	GrossSalesCents    int64    `json:"grossSalesCents"`
	ReturnedUnits      int      `json:"returnedUnits"`
	ReturnRatePercent  float64  `json:"returnRatePercent"`
	AvailableStock     int      `json:"availableStock"`
	SalesVelocity      float64  `json:"salesVelocity"`
	DaysOfStock        *float64 `json:"daysOfStock"`
	StockCoverageState string   `json:"stockCoverageState"`
}

type ProductDetailResponse struct {
	ProductID                  string                `json:"productId"`
	Title                      string                `json:"title"`
	GrossSales                 MetricCentsDTO        `json:"grossSales"`
	UnitsSold                  MetricCountDTO        `json:"unitsSold"`
	Orders                     MetricCountDTO        `json:"orders"`
	ReturnedUnits              MetricCountSimpleDTO  `json:"returnedUnits"`
	ReturnRate                 MetricPercentDTO      `json:"returnRate"`
	CurrentAvailableStock      int                   `json:"currentAvailableStock"`
	Timeseries                 []TimeseriesBucketDTO `json:"timeseries"`
	Variants                   []VariantRow          `json:"variants"`
	Insights                   []InsightDTO          `json:"insights"`
}

type InventoryRow struct {
	ProductID          string   `json:"productId"`
	VariantID          string   `json:"variantId"`
	SKU                string   `json:"sku"`
	Available          int      `json:"available"`
	OnHand             int      `json:"onHand"`
	Reserved           int      `json:"reserved"`
	Inbound            int      `json:"inbound"`
	UnitsSold          int      `json:"unitsSold"`
	SalesVelocity      float64  `json:"salesVelocity"`
	DaysOfStock        *float64 `json:"daysOfStock"`
	StockCoverageState string   `json:"stockCoverageState"`
}

type InventoryResponse struct {
	Items []InventoryRow `json:"items"`
}

type InsightEvidence struct {
	Available     *int     `json:"available,omitempty"`
	SalesVelocity *float64 `json:"salesVelocity,omitempty"`
	DaysOfStock   *float64 `json:"daysOfStock,omitempty"`
	
	GrossSalesCents         *int64   `json:"grossSalesCents,omitempty"`
	PreviousGrossSalesCents *int64   `json:"previousGrossSalesCents,omitempty"`
	ChangePercent           *float64 `json:"changePercent,omitempty"`
	
	UnitsSold         *int     `json:"unitsSold,omitempty"`
	ReturnedUnits     *int     `json:"returnedUnits,omitempty"`
	ReturnRatePercent *float64 `json:"returnRatePercent,omitempty"`
}

type InsightDTO struct {
	Type        string          `json:"type"`
	Severity    string          `json:"severity"`
	ProductID   string          `json:"productId"`
	VariantID   *string         `json:"variantId,omitempty"`
	MessageCode string          `json:"messageCode"`
	Evidence    InsightEvidence `json:"evidence"`
}
