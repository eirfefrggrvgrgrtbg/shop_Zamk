package selleranalytics

import (
	"context"
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
	change := ((current - previous) / previous) * 100
	return &change, ""
}

func (s *Service) GetOverview(ctx context.Context, sellerID uuid.UUID, from, to time.Time, timezone string) (OverviewResponse, error) {
	prevFrom, prevTo := s.getPreviousPeriod(from, to)

	currSummary, err := s.repo.GetLedgerSummary(ctx, sellerID, from, to)
	if err != nil { return OverviewResponse{}, err }
	
	currOrders, err := s.repo.GetSellerOrders(ctx, sellerID, from, to)
	if err != nil { return OverviewResponse{}, err }
	
	currUnits, err := s.repo.GetUnitsSold(ctx, sellerID, from, to)
	if err != nil { return OverviewResponse{}, err }
	
	currReturnedUnits, err := s.repo.GetReturnedUnits(ctx, sellerID, from, to)
	if err != nil { return OverviewResponse{}, err }

	prevSummary, err := s.repo.GetLedgerSummary(ctx, sellerID, prevFrom, prevTo)
	if err != nil { return OverviewResponse{}, err }
	
	prevOrders, err := s.repo.GetSellerOrders(ctx, sellerID, prevFrom, prevTo)
	if err != nil { return OverviewResponse{}, err }
	
	prevUnits, err := s.repo.GetUnitsSold(ctx, sellerID, prevFrom, prevTo)
	if err != nil { return OverviewResponse{}, err }
	
	prevReturnedUnits, err := s.repo.GetReturnedUnits(ctx, sellerID, prevFrom, prevTo)
	if err != nil { return OverviewResponse{}, err }

	timeseries, err := s.repo.GetOverviewTimeseries(ctx, sellerID, from, to, timezone)
	if err != nil { return OverviewResponse{}, err }

	currAOV := int64(0)
	if currOrders > 0 {
		currAOV = currSummary.GrossSalesCents / int64(currOrders)
	}
	prevAOV := int64(0)
	if prevOrders > 0 {
		prevAOV = prevSummary.GrossSalesCents / int64(prevOrders)
	}
	aovChange, aovState := calcChangePercent(float64(currAOV), float64(prevAOV))

	currNet := currSummary.SellerEarningCents + currSummary.ReturnDeductionsCents + currSummary.OtherAdjustmentsCents
	prevNet := prevSummary.SellerEarningCents + prevSummary.ReturnDeductionsCents + prevSummary.OtherAdjustmentsCents
	netChange, netState := calcChangePercent(float64(currNet), float64(prevNet))

	grossChange, grossState := calcChangePercent(float64(currSummary.GrossSalesCents), float64(prevSummary.GrossSalesCents))
	ordersChange, ordersState := calcChangePercent(float64(currOrders), float64(prevOrders))
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

	res := OverviewResponse{
		Period: PeriodDTO{From: from.Format(time.RFC3339), To: to.Format(time.RFC3339), Timezone: timezone},
		GrossSales: MetricCentsDTO{CurrentCents: currSummary.GrossSalesCents, PreviousCents: prevSummary.GrossSalesCents, ChangePercent: grossChange, ComparisonState: grossState},
		Orders: MetricCountDTO{Current: currOrders, Previous: prevOrders, ChangePercent: ordersChange, ComparisonState: ordersState},
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
	}
	return res, nil
}

func (s *Service) GetProducts(ctx context.Context, sellerID uuid.UUID, from, to time.Time) (ProductsResponse, error) {
	curr, err := s.repo.GetProductsPerformance(ctx, sellerID, from, to)
	if err != nil { return ProductsResponse{}, err }

	prevFrom, prevTo := s.getPreviousPeriod(from, to)
	prev, err := s.repo.GetProductsPerformance(ctx, sellerID, prevFrom, prevTo)
	if err != nil { return ProductsResponse{}, err }

	prevMap := make(map[uuid.UUID]ProductPerformance)
	for _, p := range prev {
		prevMap[p.ProductID] = p
	}

	items := make([]ProductRow, len(curr))
	for i, p := range curr {
		prevP := prevMap[p.ProductID]
		
		returnRate := 0.0
		if p.UnitsSold > 0 {
			returnRate = float64(p.ReturnedUnits) / float64(p.UnitsSold) * 100
		}
		
		chg, state := calcChangePercent(float64(p.GrossSalesCents), float64(prevP.GrossSalesCents))
		
		items[i] = ProductRow{
			ProductID:               p.ProductID.String(),
			Title:                   p.Title,
			GrossSalesCents:         p.GrossSalesCents,
			OrdersCount:             p.OrdersCount,
			UnitsSold:               p.UnitsSold,
			ReturnedUnits:           p.ReturnedUnits,
			ReturnRatePercent:       returnRate,
			AvailableStock:          p.AvailableStock,
			PreviousGrossSalesCents: prevP.GrossSalesCents,
			GrossSalesChangePercent: chg,
			ComparisonState:         state,
		}
	}
	return ProductsResponse{Items: items}, nil
}

func (s *Service) GetProductDetail(ctx context.Context, sellerID, productID uuid.UUID, from, to time.Time, timezone string) (ProductDetailResponse, error) {
	currP, err := s.repo.GetProductsPerformance(ctx, sellerID, from, to)
	if err != nil { return ProductDetailResponse{}, err }
	
	var prod ProductPerformance
	for _, p := range currP {
		if p.ProductID == productID {
			prod = p
			break
		}
	}
	// If product is empty, maybe it has 0 sales in period but we still want to show variants, handled.

	prevFrom, prevTo := s.getPreviousPeriod(from, to)
	prevP, err := s.repo.GetProductsPerformance(ctx, sellerID, prevFrom, prevTo)
	if err != nil { return ProductDetailResponse{}, err }
	var prevProd ProductPerformance
	for _, p := range prevP {
		if p.ProductID == productID {
			prevProd = p
			break
		}
	}

	variants, err := s.repo.GetVariantsPerformance(ctx, sellerID, productID, from, to)
	if err != nil { return ProductDetailResponse{}, err }

	days := to.Sub(from).Hours() / 24
	if days <= 0 { days = 1 }

	variantRows := make([]VariantRow, len(variants))
	for i, v := range variants {
		retRate := 0.0
		if v.UnitsSold > 0 { retRate = float64(v.ReturnedUnits) / float64(v.UnitsSold) * 100 }
		
		vel := float64(v.UnitsSold) / days
		
		var dos *float64
		covState := ""
		if vel == 0 {
			if v.AvailableStock == 0 {
				covState = "out_of_stock"
				val := 0.0
				dos = &val
			} else {
				covState = "no_sales"
			}
		} else {
			val := float64(v.AvailableStock) / vel
			dos = &val
		}

		variantRows[i] = VariantRow{
			VariantID:          v.VariantID.String(),
			SKU:                v.SKU,
			DisplayName:        v.DisplayName,
			UnitsSold:          v.UnitsSold,
			GrossSalesCents:    v.GrossSalesCents,
			ReturnedUnits:      v.ReturnedUnits,
			ReturnRatePercent:  retRate,
			AvailableStock:     v.AvailableStock,
			SalesVelocity:      vel,
			DaysOfStock:        dos,
			StockCoverageState: covState,
		}
	}

	grossChg, grossState := calcChangePercent(float64(prod.GrossSalesCents), float64(prevProd.GrossSalesCents))
	unitsChg, unitsState := calcChangePercent(float64(prod.UnitsSold), float64(prevProd.UnitsSold))
	ordersChg, ordersState := calcChangePercent(float64(prod.OrdersCount), float64(prevProd.OrdersCount))
	
	prodRetRate := 0.0
	if prod.UnitsSold > 0 { prodRetRate = float64(prod.ReturnedUnits) / float64(prod.UnitsSold) * 100 }
	prevProdRetRate := 0.0
	if prevProd.UnitsSold > 0 { prevProdRetRate = float64(prevProd.ReturnedUnits) / float64(prevProd.UnitsSold) * 100 }

	// Calculate overall Seller return rate for Insights
	currUnits, _ := s.repo.GetUnitsSold(ctx, sellerID, from, to)
	currReturnedUnits, _ := s.repo.GetReturnedUnits(ctx, sellerID, from, to)
	sellerReturnRate := 0.0
	if currUnits > 0 { sellerReturnRate = float64(currReturnedUnits) / float64(currUnits) * 100 }

	insights := s.generateInsights(productID, prod, prevProd, variants, days, sellerReturnRate)

	return ProductDetailResponse{
		ProductID: productID.String(),
		Title:     prod.Title,
		GrossSales: MetricCentsDTO{CurrentCents: prod.GrossSalesCents, PreviousCents: prevProd.GrossSalesCents, ChangePercent: grossChg, ComparisonState: grossState},
		UnitsSold: MetricCountDTO{Current: prod.UnitsSold, Previous: prevProd.UnitsSold, ChangePercent: unitsChg, ComparisonState: unitsState},
		Orders: MetricCountDTO{Current: prod.OrdersCount, Previous: prevProd.OrdersCount, ChangePercent: ordersChg, ComparisonState: ordersState},
		ReturnedUnits: MetricCountSimpleDTO{Current: prod.ReturnedUnits, Previous: prevProd.ReturnedUnits},
		ReturnRate: MetricPercentDTO{CurrentPercent: prodRetRate, PreviousPercent: prevProdRetRate},
		CurrentAvailableStock: prod.AvailableStock,
		Timeseries: []TimeseriesBucketDTO{}, // Omitted detail for brevity in this stage
		Variants: variantRows,
		Insights: insights,
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
		prodRetRate = float64(prod.ReturnedUnits) / float64(prod.UnitsSold) * 100
	}
	if prod.UnitsSold >= 10 && prod.ReturnedUnits >= 2 && prodRetRate > sellerReturnRate && prodRetRate >= 10.0 {
		u := prod.UnitsSold
		ru := prod.ReturnedUnits
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
		if vel == 0 {
			if in.Available == 0 {
				covState = "out_of_stock"
				val := 0.0
				dos = &val
			} else {
				covState = "no_sales"
			}
		} else {
			val := float64(in.Available) / vel
			dos = &val
		}

		items[i] = InventoryRow{
			ProductID:          in.ProductID.String(),
			VariantID:          in.VariantID.String(),
			SKU:                in.SKU,
			Available:          in.Available,
			OnHand:             in.OnHand,
			Reserved:           in.Reserved,
			Inbound:            in.Inbound,
			UnitsSold:          in.UnitsSold,
			SalesVelocity:      vel,
			DaysOfStock:        dos,
			StockCoverageState: covState,
		}
	}
	return InventoryResponse{Items: items}, nil
}
