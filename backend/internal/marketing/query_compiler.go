package marketing

import (
	"fmt"
	"strings"
)

type QueryCompiler struct{}

func NewQueryCompiler() *QueryCompiler {
	return &QueryCompiler{}
}

func (c *QueryCompiler) Build(req QueryRequest) (string, []interface{}, []QueryColumn, error) {
	regDimensions := GetRegistryDimensions()
	regMetrics := GetRegistryMetrics()

	numDims := len(req.Dimensions)
	var selectedDimDefs []*DimensionDef
	for _, dKey := range req.Dimensions {
		dDef := findDimension(regDimensions, dKey)
		if dDef == nil {
			return "", nil, nil, fmt.Errorf("invalid dimension: %s", dKey)
		}
		selectedDimDefs = append(selectedDimDefs, dDef)
	}

	needsSessions := false
	needsOrders := false
	needsBehavioral := false
	needsReturns := false

	hasProductOrDesignerOrCategory := false
	for _, dKey := range req.Dimensions {
		if dKey == "product" || dKey == "designer" || dKey == "category" {
			hasProductOrDesignerOrCategory = true
		}
	}

	for _, mKey := range req.Metrics {
		switch mKey {
		case "sessions":
			needsSessions = true
		case "orders", "sold_units", "revenue":
			needsOrders = true
		case "product_views", "favorites", "add_to_cart":
			needsBehavioral = true
		case "returns", "returned_units":
			needsReturns = true
		case "conversion":
			needsOrders = true
			if hasProductOrDesignerOrCategory {
				needsBehavioral = true
			} else {
				needsSessions = true
			}
		}
	}

	var args []interface{}
	args = append(args, req.Period.From, req.Period.To)

	var cteList []string
	cteNameList := []string{}

	// 1. distinct_paid_orders CTE (used by orders_data)
	if needsOrders {
		cteList = append(cteList, `distinct_paid_orders AS (
		SELECT o.id AS order_id, COALESCE(oa.source, '') AS source, oa.campaign_id, p.paid_at, o.user_id
		FROM orders o
		JOIN payments p ON p.order_id = o.id
		LEFT JOIN order_attributions oa ON oa.order_id = o.id
		WHERE p.status = 'succeeded' AND o.status != 'cancelled'
		  AND p.paid_at >= $1 AND p.paid_at < $2
	)`)
	}

	// Helper to build dimension expressions for a given CTE
	buildDimSelects := func(exprExtractor func(d *DimensionDef) string) (string, string) {
		if numDims == 0 {
			return "", ""
		}
		var selects []string
		var groupBys []string
		for i, d := range selectedDimDefs {
			selects = append(selects, fmt.Sprintf("%s AS dim_%d", exprExtractor(d), i))
			groupBys = append(groupBys, fmt.Sprintf("%d", i+1))
		}
		return strings.Join(selects, ", "), "\n\t\tGROUP BY " + strings.Join(groupBys, ", ")
	}

	// 2. sessions_data CTE
	if needsSessions {
		dimSel, groupBy := buildDimSelects(func(d *DimensionDef) string { return d.SessionsExpr })
		if dimSel != "" {
			dimSel += ","
		}
		cteList = append(cteList, fmt.Sprintf(`sessions_data AS (
		SELECT %s
			COUNT(s.id) AS sessions
		FROM analytics_sessions s
		WHERE s.started_at >= $1 AND s.started_at < $2%s
	)`, dimSel, groupBy))
		cteNameList = append(cteNameList, "sessions_data")
	}

	// 3. orders_data CTE
	if needsOrders {
		dimSel, groupBy := buildDimSelects(func(d *DimensionDef) string { return d.OrdersExpr })
		if dimSel != "" {
			dimSel += ","
		}

		needPrOrdJoin := false
		for _, d := range selectedDimDefs {
			if d.Key == "designer" || d.Key == "category" {
				needPrOrdJoin = true
			}
		}
		prOrdJoin := ""
		if needPrOrdJoin {
			prOrdJoin = "\n\t\tLEFT JOIN products pr_ord ON pr_ord.id = oi.product_id"
		}

		cteList = append(cteList, fmt.Sprintf(`orders_data AS (
		SELECT %s
			COUNT(DISTINCT oi.order_id) AS orders,
			COALESCE(SUM(oi.quantity), 0) AS sold_units,
			COALESCE(SUM(COALESCE(oip.total_customer_paid_cents, oi.price_cents * oi.quantity)), 0) AS revenue
		FROM order_items oi
		JOIN distinct_paid_orders dpo ON oi.order_id = dpo.order_id
		LEFT JOIN order_item_promotions oip ON oip.order_item_id = oi.id%s%s
	)`, dimSel, prOrdJoin, groupBy))
		cteNameList = append(cteNameList, "orders_data")
	}

	// 4. behavioral_data CTE
	if needsBehavioral {
		dimSel, groupBy := buildDimSelects(func(d *DimensionDef) string { return d.BehavioralExpr })
		if dimSel != "" {
			dimSel += ","
		}

		needPrBehJoin := false
		for _, d := range selectedDimDefs {
			if d.Key == "designer" || d.Key == "category" {
				needPrBehJoin = true
			}
		}
		prBehJoin := ""
		if needPrBehJoin {
			prBehJoin = "\n\t\tLEFT JOIN products pr_beh ON pr_beh.id = b.product_id"
		}

		behFilter := ""
		if hasProductOrDesignerOrCategory {
			behFilter = " AND b.product_id IS NOT NULL"
		}

		cteList = append(cteList, fmt.Sprintf(`behavioral_data AS (
		SELECT %s
			COUNT(*) FILTER (WHERE b.event_type = 'product_view') AS product_views,
			COUNT(*) FILTER (WHERE b.event_type = 'favorite_added') AS favorites,
			COUNT(*) FILTER (WHERE b.event_type = 'add_to_cart') AS add_to_cart
		FROM behavioral_events b%s
		WHERE b.occurred_at >= $1 AND b.occurred_at < $2%s%s
	)`, dimSel, prBehJoin, behFilter, groupBy))
		cteNameList = append(cteNameList, "behavioral_data")
	}

	// 5. returns_data CTE (canonical status 'completed' and accepted quantity semantics)
	if needsReturns {
		dimSel, groupBy := buildDimSelects(func(d *DimensionDef) string { return d.ReturnsExpr })
		if dimSel != "" {
			dimSel += ","
		}

		needPrRetJoin := false
		needOaJoin := false
		for _, d := range selectedDimDefs {
			if d.Key == "designer" || d.Key == "category" {
				needPrRetJoin = true
			}
			if d.Key == "source" || d.Key == "campaign" {
				needOaJoin = true
			}
		}
		prRetJoin := ""
		if needPrRetJoin {
			prRetJoin = "\n\t\tLEFT JOIN products pr_ret ON pr_ret.id = oi.product_id"
		}
		oaJoin := ""
		if needOaJoin {
			oaJoin = "\n\t\tLEFT JOIN order_attributions oa ON oa.order_id = r.order_id"
		}

		cteList = append(cteList, fmt.Sprintf(`returns_data AS (
		SELECT %s
			COUNT(DISTINCT r.id) AS returns,
			COALESCE(SUM(COALESCE(ri.accepted_quantity, ri.quantity)), 0) AS returned_units
		FROM return_items ri
		JOIN returns r ON ri.return_id = r.id
		JOIN order_items oi ON ri.order_item_id = oi.id%s%s
		WHERE r.status = 'completed'
		  AND COALESCE(r.completed_at, r.updated_at) >= $1
		  AND COALESCE(r.completed_at, r.updated_at) < $2%s
	)`, dimSel, prRetJoin, oaJoin, groupBy))
		cteNameList = append(cteNameList, "returns_data")
	}

	// 6. all_dims CTE
	if numDims == 0 {
		cteList = append(cteList, `all_dims AS (
		SELECT 1 AS dummy
	)`)
	} else {
		var dimCols []string
		for i := 0; i < numDims; i++ {
			dimCols = append(dimCols, fmt.Sprintf("dim_%d", i))
		}
		dimColsStr := strings.Join(dimCols, ", ")

		var unionParts []string
		for _, cteName := range cteNameList {
			unionParts = append(unionParts, fmt.Sprintf("SELECT %s FROM %s", dimColsStr, cteName))
		}
		if len(unionParts) == 0 {
			var emptyCols []string
			for i := 0; i < numDims; i++ {
				emptyCols = append(emptyCols, fmt.Sprintf("'' AS dim_%d", i))
			}
			unionParts = append(unionParts, fmt.Sprintf("SELECT %s", strings.Join(emptyCols, ", ")))
		}
		cteList = append(cteList, fmt.Sprintf(`all_dims AS (
		%s
	)`, strings.Join(unionParts, "\n\t\tUNION\n\t\t")))
	}

	// 7. Final SELECT columns and joins
	var finalSelects []string
	var finalJoins []string
	var columnsOut []QueryColumn

	// Add dimension columns
	for i, d := range selectedDimDefs {
		if d.EntityJoinSQL != "" {
			if d.Key == "campaign" {
				finalJoins = append(finalJoins, fmt.Sprintf(d.EntityJoinSQL, i, i, i))
			} else {
				finalJoins = append(finalJoins, fmt.Sprintf(d.EntityJoinSQL, i, i, i))
			}
		}
		if d.EntitySelectSQL != "" {
			if d.Key == "source" {
				finalSelects = append(finalSelects, fmt.Sprintf(d.EntitySelectSQL, i, i))
			} else if d.EntityJoinSQL != "" {
				finalSelects = append(finalSelects, fmt.Sprintf(d.EntitySelectSQL, i))
			} else {
				finalSelects = append(finalSelects, fmt.Sprintf(d.EntitySelectSQL, i))
			}
		}
		columnsOut = append(columnsOut, d.SafeColumns...)
	}

	// Join metric CTEs to all_dims
	for _, cteName := range cteNameList {
		alias := ""
		switch cteName {
		case "sessions_data":
			alias = "sd"
		case "orders_data":
			alias = "od"
		case "behavioral_data":
			alias = "bd"
		case "returns_data":
			alias = "rd"
		}

		if numDims == 0 {
			finalJoins = append(finalJoins, fmt.Sprintf("LEFT JOIN %s %s ON true", cteName, alias))
		} else {
			var joinConds []string
			for i := 0; i < numDims; i++ {
				joinConds = append(joinConds, fmt.Sprintf("a.dim_%d = %s.dim_%d", i, alias, i))
			}
			finalJoins = append(finalJoins, fmt.Sprintf("LEFT JOIN %s %s ON %s", cteName, alias, strings.Join(joinConds, " AND ")))
		}
	}

	// Add metric selects
	for _, mKey := range req.Metrics {
		mDef := findMetric(regMetrics, mKey)
		if mDef != nil {
			columnsOut = append(columnsOut, mDef.SafeColumn)
		}

		switch mKey {
		case "sessions":
			finalSelects = append(finalSelects, "COALESCE(sd.sessions, 0) AS sessions")
		case "orders":
			finalSelects = append(finalSelects, "COALESCE(od.orders, 0) AS orders")
		case "sold_units":
			finalSelects = append(finalSelects, "COALESCE(od.sold_units, 0) AS sold_units")
		case "revenue":
			finalSelects = append(finalSelects, "COALESCE(od.revenue, 0) AS revenue")
		case "product_views":
			finalSelects = append(finalSelects, "COALESCE(bd.product_views, 0) AS product_views")
		case "favorites":
			finalSelects = append(finalSelects, "COALESCE(bd.favorites, 0) AS favorites")
		case "add_to_cart":
			finalSelects = append(finalSelects, "COALESCE(bd.add_to_cart, 0) AS add_to_cart")
		case "conversion":
			if hasProductOrDesignerOrCategory {
				finalSelects = append(finalSelects, "CASE WHEN COALESCE(bd.product_views, 0) > 0 THEN (COALESCE(od.orders, 0)::float / bd.product_views::float) ELSE NULL END AS conversion")
			} else {
				finalSelects = append(finalSelects, "CASE WHEN COALESCE(sd.sessions, 0) > 0 THEN (COALESCE(od.orders, 0)::float / sd.sessions::float) ELSE NULL END AS conversion")
			}
		case "returns":
			finalSelects = append(finalSelects, "COALESCE(rd.returns, 0) AS returns")
		case "returned_units":
			finalSelects = append(finalSelects, "COALESCE(rd.returned_units, 0) AS returned_units")
		}
	}

	// 8. Filters
	var finalFilters []string
	for _, f := range req.Filters {
		// Find dimension index
		dimIdx := -1
		for idx, d := range req.Dimensions {
			if d == f.Dimension {
				dimIdx = idx
				break
			}
		}

		if dimIdx == -1 {
			continue // Should have been caught by validator
		}

		// Choose target expression for filter
		targetExpr := fmt.Sprintf("a.dim_%d", dimIdx)
		switch f.Dimension {
		case "product":
			targetExpr = fmt.Sprintf("pr%d.title", dimIdx)
		case "designer":
			targetExpr = fmt.Sprintf("s%d.brand_name", dimIdx)
		case "category":
			targetExpr = fmt.Sprintf("cat%d.name", dimIdx)
		case "campaign":
			targetExpr = fmt.Sprintf("mc%d.title", dimIdx)
		}

		paramIdx := len(args) + 1
		switch f.Operator {
		case "eq":
			finalFilters = append(finalFilters, fmt.Sprintf("%s = $%d", targetExpr, paramIdx))
			args = append(args, f.Values[0])
		case "neq":
			finalFilters = append(finalFilters, fmt.Sprintf("%s != $%d", targetExpr, paramIdx))
			args = append(args, f.Values[0])
		case "contains":
			finalFilters = append(finalFilters, fmt.Sprintf("%s ILIKE $%d", targetExpr, paramIdx))
			args = append(args, "%"+f.Values[0]+"%")
		case "in":
			finalFilters = append(finalFilters, fmt.Sprintf("%s = ANY($%d)", targetExpr, paramIdx))
			args = append(args, f.Values)
		}
	}

	filterClause := ""
	if len(finalFilters) > 0 {
		filterClause = "\nWHERE " + strings.Join(finalFilters, " AND ")
	}

	// 9. Sorting
	var orderClauses []string
	for _, s := range req.Sort {
		dirSQL := "ASC"
		if strings.ToLower(s.Direction) == "desc" {
			dirSQL = "DESC"
		}

		// Check if sort field is a metric
		isMetric := false
		for _, m := range req.Metrics {
			if m == s.Field {
				isMetric = true
				break
			}
		}

		if isMetric {
			orderClauses = append(orderClauses, fmt.Sprintf("%s %s NULLS LAST", s.Field, dirSQL))
		} else {
			// Sort field is a dimension
			dimIdx := -1
			for idx, d := range req.Dimensions {
				if d == s.Field {
					dimIdx = idx
					break
				}
			}
			if dimIdx != -1 {
				dimSortExpr := fmt.Sprintf("a.dim_%d", dimIdx)
				switch s.Field {
				case "product":
					dimSortExpr = fmt.Sprintf("pr%d.title", dimIdx)
				case "designer":
					dimSortExpr = fmt.Sprintf("s%d.brand_name", dimIdx)
				case "category":
					dimSortExpr = fmt.Sprintf("cat%d.name", dimIdx)
				case "campaign":
					dimSortExpr = fmt.Sprintf("mc%d.title", dimIdx)
				}
				orderClauses = append(orderClauses, fmt.Sprintf("%s %s NULLS LAST", dimSortExpr, dirSQL))
			}
		}
	}

	// Deterministic tie-break
	if numDims == 1 {
		orderClauses = append(orderClauses, "a.dim_0 ASC")
	} else if numDims == 2 {
		orderClauses = append(orderClauses, "a.dim_0 ASC", "a.dim_1 ASC")
	}

	orderClause := ""
	if len(orderClauses) > 0 {
		orderClause = "\nORDER BY " + strings.Join(orderClauses, ", ")
	}

	// 10. Limit
	limit := 100
	if req.Limit != nil {
		limit = *req.Limit
	}

	query := fmt.Sprintf("WITH %s\nSELECT %s\nFROM all_dims a\n%s%s%s\nLIMIT %d",
		strings.Join(cteList, ",\n"),
		strings.Join(finalSelects, ",\n\t"),
		strings.Join(finalJoins, "\n"),
		filterClause,
		orderClause,
		limit,
	)

	return query, args, columnsOut, nil
}
