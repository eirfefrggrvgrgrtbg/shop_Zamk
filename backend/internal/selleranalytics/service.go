package selleranalytics

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/google/uuid"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) getPreviousPeriod(from, to time.Time) (time.Time, time.Time) {
	duration := to.Sub(from)
	prevTo := from
	prevFrom := from.Add(-duration)
	return prevFrom, prevTo
}

func calcChangePercent(current, previous float64) (*float64, string) {
	if previous == 0 && current == 0 {
		val := 0.0
		return &val, "unchanged"
	}
	if previous == 0 && current > 0 {
		return nil, "new"
	}
	if previous == 0 && current < 0 {
		return nil, "negative"
	}
	change := ((current - previous) / math.Abs(previous)) * 100
	if change > 0 {
		return &change, "positive"
	} else if change < 0 {
		return &change, "negative"
	}
	val := 0.0
	return &val, "unchanged"
}

func (s *Service) GetOverview(ctx context.Context, sellerID uuid.UUID, from, to time.Time, timezone string) (OverviewResponse, error) {
	prevFrom, prevTo := s.getPreviousPeriod(from, to)

	currSummary, err := s.repo.GetLedgerSummary(ctx, sellerID, from, to)
	if err != nil { return OverviewResponse{}, err }
	
	currPaidOrders, err := s.repo.GetSellerPaidOrders(ctx, sellerID, from, to)
	if err != nil { return OverviewResponse{}, err }

	currDeliveredOrders, err := s.repo.GetSellerDeliveredOrders(ctx, sellerID, from, to)
	if err != nil { return OverviewResponse{}, err }
	
	currUnits, err := s.repo.GetUnitsSold(ctx, sellerID, from, to)
	if err != nil { return OverviewResponse{}, err }
	
	currReturnedUnits, err := s.repo.GetReturnedUnits(ctx, sellerID, from, to)
	if err != nil { return OverviewResponse{}, err }

	prevSummary, err := s.repo.GetLedgerSummary(ctx, sellerID, prevFrom, prevTo)
	if err != nil { return OverviewResponse{}, err }
	
	prevPaidOrders, err := s.repo.GetSellerPaidOrders(ctx, sellerID, prevFrom, prevTo)
	if err != nil { return OverviewResponse{}, err }

	prevDeliveredOrders, err := s.repo.GetSellerDeliveredOrders(ctx, sellerID, prevFrom, prevTo)
	if err != nil { return OverviewResponse{}, err }
	
	prevUnits, err := s.repo.GetUnitsSold(ctx, sellerID, prevFrom, prevTo)
	if err != nil { return OverviewResponse{}, err }
	
	prevReturnedUnits, err := s.repo.GetReturnedUnits(ctx, sellerID, prevFrom, prevTo)
	if err != nil { return OverviewResponse{}, err }

	timeseries, err := s.repo.GetOverviewTimeseries(ctx, sellerID, from, to, timezone)
	if err != nil { return OverviewResponse{}, err }

	currAOV := int64(0)
	if currDeliveredOrders > 0 {
		currAOV = currSummary.GrossSalesCents / int64(currDeliveredOrders)
	}
	prevAOV := int64(0)
	if prevDeliveredOrders > 0 {
		prevAOV = prevSummary.GrossSalesCents / int64(prevDeliveredOrders)
	}
	aovChange, aovState := calcChangePercent(float64(currAOV), float64(prevAOV))

	currNet := currSummary.SellerEarningCents + currSummary.ReturnDeductionsCents
	prevNet := prevSummary.SellerEarningCents + prevSummary.ReturnDeductionsCents
	netChange, netState := calcChangePercent(float64(currNet), float64(prevNet))

	grossChange, grossState := calcChangePercent(float64(currSummary.GrossSalesCents), float64(prevSummary.GrossSalesCents))
	ordersChange, ordersState := calcChangePercent(float64(currPaidOrders), float64(prevPaidOrders))
	unitsChange, unitsState := calcChangePercent(float64(currUnits), float64(prevUnits))

	currReturnRate := 0.0
	if currUnits > 0 {
		currReturnRate = float64(currReturnedUnits) / float64(currUnits) * 100
	}
	prevReturnRate := 0.0
	if prevUnits > 0 {
		prevReturnRate = float64(prevReturnedUnits) / float64(prevUnits) * 100
	}

	tsDTO := make([]TimeseriesBucketDTO, len(timeseries))
	for i, r := range timeseries {
		tsDTO[i] = TimeseriesBucketDTO{
			Date:                      r.Date.Format("2006-01-02"),
			GrossSalesCents:           r.GrossSalesCents,
			OrdersCount:               r.OrdersCount,
			UnitsSold:                 r.UnitsSold,
			CommissionCents:           r.CommissionCents,
			SellerEarningCents:        r.SellerEarningCents,
			ReturnDeductionsCents:     r.ReturnDeductionsCents,
			NetCommercialEarningCents: r.NetCommercialEarningCents,
			ReturnedUnits:             r.ReturnedUnits,
		}
	}

	// Generate Insights for Top 5 Products
	asOfNow := time.Now().UTC()
	allProducts, _ := s.repo.GetProductsPerformance(ctx, sellerID, from, to, asOfNow)
	allPrevProducts, _ := s.repo.GetProductsPerformance(ctx, sellerID, prevFrom, prevTo, asOfNow)
	prevProductMap := make(map[uuid.UUID]ProductPerformance)
	for _, p := range allPrevProducts {
		prevProductMap[p.ProductID] = p
	}

	// Sort allProducts by GrossSalesCents desc manually (simple bubble/selection sort since count is small or use slice sort)
	// Actually we'll just extract top 5
	topCount := 5
	if len(allProducts) < 5 {
		topCount = len(allProducts)
	}
	// Sort descending
	for i := 0; i < len(allProducts); i++ {
		for j := i + 1; j < len(allProducts); j++ {
			if allProducts[i].GrossSalesCents < allProducts[j].GrossSalesCents {
				allProducts[i], allProducts[j] = allProducts[j], allProducts[i]
			}
		}
	}

	var insights []InsightDTO
	days := to.Sub(from).Hours() / 24
	if days <= 0 { days = 1 }

	for i := 0; i < topCount; i++ {
		prod := allProducts[i]
		prevProd := prevProductMap[prod.ProductID]
		variants, _ := s.repo.GetVariantsPerformance(ctx, sellerID, prod.ProductID, from, to, asOfNow)
		fmt.Printf("Product %s has %d variants\n", prod.ProductID, len(variants))
		for _, v := range variants {
			fmt.Printf("Variant %s: Available=%d, Sold=%d\n", v.VariantID, v.AvailableStock, v.UnitsSold)
		}
		prodInsights := s.generateInsights(prod.ProductID, prod, prevProd, variants, days, currReturnRate)
		insights = append(insights, prodInsights...)
	}

	hasHist, err := s.repo.CheckHasHistoricalSales(ctx, sellerID)
	if err != nil { return OverviewResponse{}, err }

	res := OverviewResponse{
		Period: PeriodDTO{From: from.Format(time.RFC3339), To: to.Format(time.RFC3339), Timezone: timezone},
		HasHistoricalSales: hasHist,
		GrossSales: MetricCentsDTO{CurrentCents: currSummary.GrossSalesCents, PreviousCents: prevSummary.GrossSalesCents, ChangePercent: grossChange, ComparisonState: grossState},
		Orders: MetricCountDTO{Current: currPaidOrders, Previous: prevPaidOrders, ChangePercent: ordersChange, ComparisonState: ordersState},
		DeliveredOrders: &MetricCountSimpleDTO{Current: currDeliveredOrders, Previous: prevDeliveredOrders},
		UnitsSold: MetricCountDTO{Current: currUnits, Previous: prevUnits, ChangePercent: unitsChange, ComparisonState: unitsState},
		AverageOrderValue: MetricCentsDTO{CurrentCents: currAOV, PreviousCents: prevAOV, ChangePercent: aovChange, ComparisonState: aovState},
		Commission: MetricCentsSimpleDTO{CurrentCents: currSummary.CommissionCents, PreviousCents: prevSummary.CommissionCents},
		SellerEarningBeforeReturns: MetricCentsSimpleDTO{CurrentCents: currSummary.SellerEarningCents, PreviousCents: prevSummary.SellerEarningCents},
		ReturnDeductions: MetricCentsSimpleDTO{CurrentCents: currSummary.ReturnDeductionsCents, PreviousCents: prevSummary.ReturnDeductionsCents},
		OtherAdjustments: MetricCentsSimpleDTO{CurrentCents: currSummary.OtherAdjustmentsCents, PreviousCents: prevSummary.OtherAdjustmentsCents},
		NetCommercialEarning: MetricCentsDTO{CurrentCents: currNet, PreviousCents: prevNet, ChangePercent: netChange, ComparisonState: netState},
		ReturnedUnits: MetricCountSimpleDTO{Current: currReturnedUnits, Previous: prevReturnedUnits},
		ReturnRate: MetricPercentDTO{CurrentPercent: currReturnRate, PreviousPercent: prevReturnRate},
		Timeseries: tsDTO,
		Insights: insights,
	}
	if res.Insights == nil {
		res.Insights = make([]InsightDTO, 0)
	}
	return res, nil
}

func (s *Service) GetProducts(ctx context.Context, sellerID uuid.UUID, from, to time.Time, limit, offset int, sortBy, order string, asOfOpt ...time.Time) (ProductsResponse, error) {
	asOf := time.Now().UTC()
	if len(asOfOpt) > 0 && !asOfOpt[0].IsZero() {
		asOf = asOfOpt[0]
	}
	curr, err := s.repo.GetProductsPerformance(ctx, sellerID, from, to, asOf)
	if err != nil { return ProductsResponse{}, err }

	prevFrom, prevTo := s.getPreviousPeriod(from, to)
	prev, err := s.repo.GetProductsPerformance(ctx, sellerID, prevFrom, prevTo, asOf)
	if err != nil { return ProductsResponse{}, err }

	prevMap := make(map[uuid.UUID]ProductPerformance)
	for _, p := range prev {
		prevMap[p.ProductID] = p
	}

	items := make([]ProductRow, len(curr))
	for i, c := range curr {
		if c.CohortReturnedUnits > c.UnitsSold || c.HasItemAnomaly {
			return ProductsResponse{}, ErrCohortDataAnomaly
		}
		p := prevMap[c.ProductID]
		grossChange, grossState := calcChangePercent(float64(c.GrossSalesCents), float64(p.GrossSalesCents))

		var returnRate *float64
		if c.UnitsSold > 0 {
			rate := float64(c.CohortReturnedUnits) / float64(c.UnitsSold) * 100
			returnRate = &rate
		}

		items[i] = ProductRow{
			ProductID:               c.ProductID.String(),
			Title:                   c.Title,
			GrossSalesCents:         c.GrossSalesCents,
			OrdersCount:             c.OrdersCount,
			UnitsSold:               c.UnitsSold,
			ReturnedUnits:           c.ReturnedUnits,
			ReturnRatePercent:       returnRate,
			AvailableStock:          c.AvailableStock,
			PreviousGrossSalesCents: p.GrossSalesCents,
			GrossSalesChangePercent: grossChange,
			ComparisonState:         grossState,
			CohortDeliveredUnits:    c.UnitsSold,
			CohortReturnedUnits:     c.CohortReturnedUnits,
		}
	}

	// Sort items with deterministic tie-breaking for pagination stability
	sort.SliceStable(items, func(i, j int) bool {
		var cmp int
		switch sortBy {
		case "gross_sales", "grossSalesCents":
			if items[i].GrossSalesCents < items[j].GrossSalesCents {
				cmp = -1
			} else if items[i].GrossSalesCents > items[j].GrossSalesCents {
				cmp = 1
			}
		case "orders", "ordersCount":
			if items[i].OrdersCount < items[j].OrdersCount {
				cmp = -1
			} else if items[i].OrdersCount > items[j].OrdersCount {
				cmp = 1
			}
		case "units", "unitsSold":
			if items[i].UnitsSold < items[j].UnitsSold {
				cmp = -1
			} else if items[i].UnitsSold > items[j].UnitsSold {
				cmp = 1
			}
		case "returns", "returnRatePercent":
			// If one is nil and the other is not, nil is ALWAYS sorted last regardless of order
			if items[i].ReturnRatePercent == nil && items[j].ReturnRatePercent != nil {
				return false
			}
			if items[i].ReturnRatePercent != nil && items[j].ReturnRatePercent == nil {
				return true
			}
			if items[i].ReturnRatePercent != nil && items[j].ReturnRatePercent != nil {
				if *items[i].ReturnRatePercent != *items[j].ReturnRatePercent {
					if order == "asc" {
						return *items[i].ReturnRatePercent < *items[j].ReturnRatePercent
					}
					return *items[i].ReturnRatePercent > *items[j].ReturnRatePercent
				}
			}
			// Both have equal return rates (or both are nil):
			// Secondary tie-breaker: unitsSold DESC
			if items[i].UnitsSold != items[j].UnitsSold {
				return items[i].UnitsSold > items[j].UnitsSold
			}
			// Final stable tie-breaker: ProductID ASC
			return items[i].ProductID < items[j].ProductID
		case "stock", "availableStock":
			if items[i].AvailableStock < items[j].AvailableStock {
				cmp = -1
			} else if items[i].AvailableStock > items[j].AvailableStock {
				cmp = 1
			}
		default:
			if items[i].GrossSalesCents < items[j].GrossSalesCents {
				cmp = -1
			} else if items[i].GrossSalesCents > items[j].GrossSalesCents {
				cmp = 1
			}
		}

		if cmp != 0 {
			if order == "asc" {
				return cmp < 0
			}
			return cmp > 0
		}
		// Deterministic tie-breaking by ProductID
		return items[i].ProductID < items[j].ProductID
	})

	totalCount := len(items)

	// Apply pagination
	items = paginateProducts(items, limit, offset)

	res := ProductsResponse{
		Items: items,
		TotalCount: totalCount,
	}
	return res, nil
}

func (s *Service) GetProductDetail(ctx context.Context, sellerID, productID uuid.UUID, from, to time.Time, timezone string, asOfOpt ...time.Time) (ProductDetailResponse, error) {
	asOf := time.Now().UTC()
	if len(asOfOpt) > 0 && !asOfOpt[0].IsZero() {
		asOf = asOfOpt[0]
	}
	owned, title, err := s.repo.GetProductOwnership(ctx, sellerID, productID)
	if err != nil {
		return ProductDetailResponse{}, err
	}
	if !owned {
		return ProductDetailResponse{}, ErrProductNotFound
	}

	currP, err := s.repo.GetProductsPerformance(ctx, sellerID, from, to, asOf)
	if err != nil { return ProductDetailResponse{}, err }
	
	var prod ProductPerformance
	for _, p := range currP {
		if p.ProductID == productID {
			prod = p
			break
		}
	}
	if title != "" {
		prod.Title = title
	}

	prevFrom, prevTo := s.getPreviousPeriod(from, to)
	prevP, err := s.repo.GetProductsPerformance(ctx, sellerID, prevFrom, prevTo, asOf)
	if err != nil { return ProductDetailResponse{}, err }
	var prevProd ProductPerformance
	for _, p := range prevP {
		if p.ProductID == productID {
			prevProd = p
			break
		}
	}

	variants, err := s.repo.GetVariantsPerformance(ctx, sellerID, productID, from, to, asOf)
	if err != nil { return ProductDetailResponse{}, err }

	days := to.Sub(from).Hours() / 24
	if days <= 0 { days = 1 }

	variantRows := make([]VariantRow, len(variants))
	for i, v := range variants {
		if v.CohortReturnedUnits > v.UnitsSold || v.HasItemAnomaly {
			return ProductDetailResponse{}, ErrCohortDataAnomaly
		}
		var retRate *float64
		if v.UnitsSold > 0 {
			rate := float64(v.CohortReturnedUnits) / float64(v.UnitsSold) * 100
			retRate = &rate
		}
		
		vel := float64(v.UnitsSold) / days
		
		var dos *float64
		covState := ""
		if v.AvailableStock == 0 {
			covState = "out_of_stock"
			val := 0.0
			dos = &val
		} else if vel == 0 {
			covState = "no_sales"
		} else {
			val := float64(v.AvailableStock) / vel
			dos = &val
		}

		variantRows[i] = VariantRow{
			VariantID:            v.VariantID.String(),
			SKU:                  v.SKU,
			DisplayName:          v.DisplayName,
			UnitsSold:            v.UnitsSold,
			GrossSalesCents:      v.GrossSalesCents,
			ReturnedUnits:        v.ReturnedUnits,
			ReturnRatePercent:    retRate,
			AvailableStock:       v.AvailableStock,
			SalesVelocity:        vel,
			DaysOfStock:          dos,
			StockCoverageState:   covState,
			CohortDeliveredUnits: v.UnitsSold,
			CohortReturnedUnits:  v.CohortReturnedUnits,
		}
	}

	if prod.CohortReturnedUnits > prod.UnitsSold || prod.HasItemAnomaly {
		return ProductDetailResponse{}, ErrCohortDataAnomaly
	}

	grossChg, grossState := calcChangePercent(float64(prod.GrossSalesCents), float64(prevProd.GrossSalesCents))
	unitsChg, unitsState := calcChangePercent(float64(prod.UnitsSold), float64(prevProd.UnitsSold))
	ordersChg, ordersState := calcChangePercent(float64(prod.OrdersCount), float64(prevProd.OrdersCount))
	
	var prodRetRate *float64
	if prod.UnitsSold > 0 {
		rate := float64(prod.CohortReturnedUnits) / float64(prod.UnitsSold) * 100
		prodRetRate = &rate
	}
	var prevProdRetRate *float64
	if prevProd.UnitsSold > 0 {
		rate := float64(prevProd.CohortReturnedUnits) / float64(prevProd.UnitsSold) * 100
		prevProdRetRate = &rate
	}

	// Calculate overall Seller return rate for Insights
	currUnits, _ := s.repo.GetUnitsSold(ctx, sellerID, from, to)
	currReturnedUnits, _ := s.repo.GetReturnedUnits(ctx, sellerID, from, to)
	sellerReturnRate := 0.0
	if currUnits > 0 { sellerReturnRate = float64(currReturnedUnits) / float64(currUnits) * 100 }

	insights := s.generateInsights(productID, prod, prevProd, variants, days, sellerReturnRate)

	availStock := prod.AvailableStock
	if availStock == 0 {
		for _, v := range variants {
			availStock += v.AvailableStock
		}
	}

	productTimeseries, err := s.repo.GetProductTimeseries(ctx, sellerID, productID, from, to, timezone)
	if err != nil {
		return ProductDetailResponse{}, err
	}

	tsDTO := make([]TimeseriesBucketDTO, len(productTimeseries))
	for i, r := range productTimeseries {
		tsDTO[i] = TimeseriesBucketDTO{
			Date:            r.Date.Format("2006-01-02"),
			GrossSalesCents: r.GrossSalesCents,
			OrdersCount:     r.OrdersCount,
			UnitsSold:       r.UnitsSold,
			ReturnedUnits:   r.ReturnedUnits,
		}
	}

	return ProductDetailResponse{
		ProductID:             productID.String(),
		Title:                 prod.Title,
		GrossSales:            MetricCentsDTO{CurrentCents: prod.GrossSalesCents, PreviousCents: prevProd.GrossSalesCents, ChangePercent: grossChg, ComparisonState: grossState},
		UnitsSold:             MetricCountDTO{Current: prod.UnitsSold, Previous: prevProd.UnitsSold, ChangePercent: unitsChg, ComparisonState: unitsState},
		Orders:                MetricCountDTO{Current: prod.OrdersCount, Previous: prevProd.OrdersCount, ChangePercent: ordersChg, ComparisonState: ordersState},
		ReturnedUnits:         MetricCountSimpleDTO{Current: prod.ReturnedUnits, Previous: prevProd.ReturnedUnits},
		ReturnRate:            MetricPercentNullableDTO{CurrentPercent: prodRetRate, PreviousPercent: prevProdRetRate},
		CurrentAvailableStock: availStock,
		Timeseries:            tsDTO,
		Variants:              variantRows,
		Insights:              insights,
		CohortDeliveredUnits:  prod.UnitsSold,
		CohortReturnedUnits:   prod.CohortReturnedUnits,
	}, nil
}

func (s *Service) generateInsights(productID uuid.UUID, prod, prevProd ProductPerformance, variants []VariantPerformance, days, sellerReturnRate float64) []InsightDTO {
	var insights []InsightDTO

	for _, v := range variants {
		vel := float64(v.UnitsSold) / days
		dos := float64(0)
		if vel > 0 {
			dos = float64(v.AvailableStock) / vel
		}

		if vel > 0 && dos > 0 && dos <= 7 {
			velRef := vel
			dosRef := dos
			availRef := v.AvailableStock
			vid := v.VariantID.String()
			insights = append(insights, InsightDTO{
				Type: "low_stock", Severity: "high", ProductID: productID.String(), VariantID: &vid, MessageCode: "variant_low_stock",
				Evidence: InsightEvidence{Available: &availRef, SalesVelocity: &velRef, DaysOfStock: &dosRef},
			})
		}
		if v.AvailableStock == 0 && v.UnitsSold > 0 {
			availRef := v.AvailableStock
			unitsRef := v.UnitsSold
			vid := v.VariantID.String()
			insights = append(insights, InsightDTO{
				Type: "out_of_stock", Severity: "high", ProductID: productID.String(), VariantID: &vid, MessageCode: "variant_out_of_stock",
				Evidence: InsightEvidence{Available: &availRef, UnitsSold: &unitsRef},
			})
		}
		if v.AvailableStock > 0 && v.UnitsSold == 0 {
			availRef := v.AvailableStock
			unitsRef := v.UnitsSold
			vid := v.VariantID.String()
			insights = append(insights, InsightDTO{
				Type: "no_sales", Severity: "low", ProductID: productID.String(), VariantID: &vid, MessageCode: "variant_no_sales",
				Evidence: InsightEvidence{Available: &availRef, UnitsSold: &unitsRef},
			})
		}
	}

	prodRetRate := 0.0
	if prod.UnitsSold > 0 {
		prodRetRate = float64(prod.CohortReturnedUnits) / float64(prod.UnitsSold) * 100
	}
	if prod.UnitsSold >= 10 && prod.CohortReturnedUnits >= 2 && prodRetRate > sellerReturnRate && prodRetRate >= 10.0 {
		u := prod.UnitsSold
		ru := prod.CohortReturnedUnits
		rr := prodRetRate
		insights = append(insights, InsightDTO{
			Type: "high_return_rate", Severity: "medium", ProductID: productID.String(), MessageCode: "product_high_returns",
			Evidence: InsightEvidence{UnitsSold: &u, ReturnedUnits: &ru, ReturnRatePercent: &rr},
		})
	}

	if prevProd.GrossSalesCents > 100000 { // 1000 RUB min base
		chg := ((float64(prod.GrossSalesCents) - float64(prevProd.GrossSalesCents)) / float64(prevProd.GrossSalesCents)) * 100
		g := prod.GrossSalesCents
		pg := prevProd.GrossSalesCents
		c := chg
		if chg > 50 {
			insights = append(insights, InsightDTO{
				Type: "growing", Severity: "low", ProductID: productID.String(), MessageCode: "product_growing",
				Evidence: InsightEvidence{GrossSalesCents: &g, PreviousGrossSalesCents: &pg, ChangePercent: &c},
			})
		} else if chg < -50 {
			insights = append(insights, InsightDTO{
				Type: "falling", Severity: "medium", ProductID: productID.String(), MessageCode: "product_falling",
				Evidence: InsightEvidence{GrossSalesCents: &g, PreviousGrossSalesCents: &pg, ChangePercent: &c},
			})
		}
	}

	return insights
}

func (s *Service) GetInventory(ctx context.Context, sellerID uuid.UUID, from, to time.Time) (InventoryResponse, error) {
	inv, err := s.repo.GetInventoryPerformance(ctx, sellerID, from, to)
	if err != nil { return InventoryResponse{}, err }

	days := to.Sub(from).Hours() / 24
	if days <= 0 { days = 1 }

	items := make([]InventoryRow, len(inv))
	for i, in := range inv {
		vel := float64(in.UnitsSold) / days
		
		var dos *float64
		covState := ""
		isForecastLow := false

		if in.Available == 0 {
			covState = "out_of_stock"
		} else if in.UnitsSold == 0 {
			covState = "no_sales"
		} else {
			val := float64(in.Available) / vel
			dos = &val
			if *dos <= 7.0 {
				isForecastLow = true
			}
		}

		isPhysLow := in.Available > 0 && in.Available <= 10
		hasNoSales := in.UnitsSold == 0

		items[i] = InventoryRow{
			ProductID:             in.ProductID.String(),
			VariantID:             in.VariantID.String(),
			ProductTitle:          in.ProductTitle,
			SKU:                   in.SKU,
			DisplayName:           in.DisplayName,
			Available:             in.Available,
			OnHand:                in.OnHand,
			Reserved:              in.Reserved,
			Inbound:               in.Inbound,
			UnitsSold:             in.UnitsSold,
			SalesVelocity:         vel,
			DaysOfStock:           dos,
			StockCoverageState:    covState,
			IsPhysicalLowStock:    isPhysLow,
			HasNoSalesInPeriod:    hasNoSales,
			IsForecastLowCoverage: isForecastLow,
		}
	}
	return InventoryResponse{Items: items}, nil
}

// paginateProducts returns the [offset, offset+limit) window of items. Negative
// arguments are treated as zero, and the bounds are computed without overflowing
// offset+limit, so an out-of-range window yields an empty page instead of panicking.
func paginateProducts(items []ProductRow, limit, offset int) []ProductRow {
	if limit < 0 { limit = 0 }
	if offset < 0 { offset = 0 }
	if offset >= len(items) {
		return items[:0]
	}
	end := len(items)
	if limit < end-offset {
		end = offset + limit
	}
	return items[offset:end]
}
