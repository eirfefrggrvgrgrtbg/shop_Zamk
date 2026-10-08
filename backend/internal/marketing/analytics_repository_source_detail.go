package marketing

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

func (r *AnalyticsRepository) GetSourceDetail(ctx context.Context, source string, req SourceDetailRequest) (*SourceDetailResponse, error) {
	// 1. Coverage
	var viewsTrackingStarted, favTrackingStarted, cartTrackingStarted *time.Time
	err := r.db.QueryRow(ctx, `
		SELECT
		  (SELECT MIN(occurred_at) FROM behavioral_events WHERE event_type = 'product_view'),
		  (SELECT MIN(occurred_at) FROM behavioral_events WHERE event_type = 'favorite_added'),
		  (SELECT MIN(occurred_at) FROM behavioral_events WHERE event_type = 'add_to_cart')
	`).Scan(&viewsTrackingStarted, &favTrackingStarted, &cartTrackingStarted)
	if err != nil {
		return nil, err
	}

	coverage := ProductAnalyticsCoverage{
		Views:     r.evaluateCoverage(req.From, req.To, viewsTrackingStarted, r.getVerifiedCompleteFrom("product_view")),
		Favorites: r.evaluateCoverage(req.From, req.To, favTrackingStarted, r.getVerifiedCompleteFrom("favorite_added")),
		AddToCart: r.evaluateCoverage(req.From, req.To, cartTrackingStarted, r.getVerifiedCompleteFrom("add_to_cart")),
	}

	duration := req.To.Sub(req.From)
	prevFrom := req.From.Add(-duration)
	prevTo := req.From

	// 2. Map route parameter to SQL source filters & canonical DTO identity
	var sessionSourceFilterMain, orderSourceFilterMain string
	var argsSourceRow []interface{}
	argsSourceRow = append(argsSourceRow, req.From, req.To, prevFrom, prevTo)

	var sessionSourceFilterSub, orderSourceFilterSub string
	var argsSub []interface{}
	argsSub = append(argsSub, req.From, req.To)

	var sr SourcePerformanceRow

	if source == "_unattributed" {
		sessionSourceFilterMain = "(s.source IS NULL OR s.source = '')"
		orderSourceFilterMain = "(oa.source IS NULL OR oa.source = '')"
		sessionSourceFilterSub = "(s.source IS NULL OR s.source = '')"
		orderSourceFilterSub = "(oa.source IS NULL OR oa.source = '')"
		sr.SourceKey = nil
		sr.SourceKind = SourceKindUnattributed
		sr.Source = ""
	} else if strings.EqualFold(source, "direct") {
		sessionSourceFilterMain = "LOWER(COALESCE(s.source, '')) = 'direct'"
		orderSourceFilterMain = "LOWER(COALESCE(oa.source, '')) = 'direct'"
		sessionSourceFilterSub = "LOWER(COALESCE(s.source, '')) = 'direct'"
		orderSourceFilterSub = "LOWER(COALESCE(oa.source, '')) = 'direct'"
		directKey := "direct"
		sr.SourceKey = &directKey
		sr.SourceKind = SourceKindDirect
		sr.Source = "direct"
	} else {
		sessionSourceFilterMain = "s.source = $5"
		orderSourceFilterMain = "oa.source = $5"
		argsSourceRow = append(argsSourceRow, source)

		sessionSourceFilterSub = "s.source = $3"
		orderSourceFilterSub = "oa.source = $3"
		argsSub = append(argsSub, source)

		key := source
		sr.SourceKey = &key
		sr.SourceKind = SourceKindNamed
		sr.Source = source
	}

	// 3. Fetch the source row metrics
	querySourceRow := fmt.Sprintf(`
		WITH sessions_data AS (
			SELECT
				COUNT(id) AS visits
			FROM analytics_sessions s
			WHERE started_at >= $1 AND started_at < $2
			  AND %s
		),
		prev_sessions_data AS (
			SELECT
				COUNT(id) AS previous_visits
			FROM analytics_sessions s
			WHERE started_at >= $3 AND started_at < $4
			  AND %s
		),
		distinct_paid_orders AS (
			SELECT o.id AS order_id, o.user_id
			FROM orders o
			JOIN payments p ON p.order_id = o.id
			LEFT JOIN order_attributions oa ON oa.order_id = o.id
			WHERE p.status = 'succeeded'
			  AND o.status != 'cancelled'
			  AND p.paid_at >= $1 AND p.paid_at < $2
			  AND %s
			GROUP BY o.id, o.user_id
		),
		first_orders AS (
			SELECT user_id, MIN(created_at) as first_order_at
			FROM orders
			WHERE status != 'cancelled'
			GROUP BY user_id
		),
		orders_data AS (
			SELECT
				COUNT(DISTINCT dpo.order_id) AS purchases,
				SUM(oi.quantity) AS sold_units,
				SUM(COALESCE(oip.total_customer_paid_cents, oi.price_cents * oi.quantity)) AS revenue_cents,
				COUNT(DISTINCT CASE WHEN dpo.user_id IS NOT NULL AND fo.first_order_at >= $1 AND fo.first_order_at < $2 THEN dpo.order_id END) AS new_customers,
				COUNT(DISTINCT CASE WHEN dpo.user_id IS NOT NULL AND fo.first_order_at < $1 THEN dpo.order_id END) AS repeat_customers
			FROM distinct_paid_orders dpo
			JOIN order_items oi ON oi.order_id = dpo.order_id
			LEFT JOIN first_orders fo ON fo.user_id = dpo.user_id
			LEFT JOIN order_item_promotions oip ON oip.order_item_id = oi.id
		),
		returns_data AS (
			SELECT
				COUNT(DISTINCT r.id) AS returns_count
			FROM return_items ri
			JOIN returns r ON ri.return_id = r.id
			JOIN order_items oi ON ri.order_item_id = oi.id
			JOIN orders o ON oi.order_id = o.id
			LEFT JOIN order_attributions oa ON oa.order_id = o.id
			WHERE r.status = 'completed'
			  AND COALESCE(r.completed_at, r.updated_at) >= $1
			  AND COALESCE(r.completed_at, r.updated_at) < $2
			  AND %s
		),
		prev_distinct_paid_orders AS (
			SELECT o.id AS order_id
			FROM orders o
			JOIN payments p ON p.order_id = o.id
			LEFT JOIN order_attributions oa ON oa.order_id = o.id
			WHERE p.status = 'succeeded'
			  AND o.status != 'cancelled'
			  AND p.paid_at >= $3 AND p.paid_at < $4
			  AND %s
			GROUP BY o.id
		),
		prev_orders_data AS (
			SELECT
				SUM(COALESCE(oip.total_customer_paid_cents, oi.price_cents * oi.quantity)) AS previous_revenue_cents
			FROM prev_distinct_paid_orders pdpo
			JOIN order_items oi ON oi.order_id = pdpo.order_id
			LEFT JOIN order_item_promotions oip ON oip.order_item_id = oi.id
		)
		SELECT
			COALESCE(b.visits, 0) AS visits,
			COALESCE(o.purchases, 0) AS purchases,
			COALESCE(o.sold_units, 0) AS sold_units,
			COALESCE(o.revenue_cents, 0) AS revenue_cents,
			COALESCE(o.new_customers, 0) AS new_customers,
			COALESCE(o.repeat_customers, 0) AS repeat_customers,
			COALESCE(r.returns_count, 0) AS returns_count,
			COALESCE(pb.previous_visits, 0) AS previous_visits,
			COALESCE(po.previous_revenue_cents, 0) AS previous_revenue_cents,
			CASE
				WHEN COALESCE(pb.previous_visits, 0) > 0 THEN
					((COALESCE(b.visits, 0) - COALESCE(pb.previous_visits, 0))::float / COALESCE(pb.previous_visits, 0)::float) * 100.0
				ELSE NULL
			END AS visits_change_pct,
			CASE
				WHEN COALESCE(po.previous_revenue_cents, 0) > 0 THEN
					((COALESCE(o.revenue_cents, 0) - COALESCE(po.previous_revenue_cents, 0))::float / COALESCE(po.previous_revenue_cents, 0)::float) * 100.0
				ELSE NULL
			END AS revenue_change_pct
		FROM (SELECT 1) dummy
		LEFT JOIN sessions_data b ON true
		LEFT JOIN orders_data o ON true
		LEFT JOIN returns_data r ON true
		LEFT JOIN prev_sessions_data pb ON true
		LEFT JOIN prev_orders_data po ON true
	`, sessionSourceFilterMain, sessionSourceFilterMain, orderSourceFilterMain, orderSourceFilterMain, orderSourceFilterMain)

	err = r.db.QueryRow(ctx, querySourceRow, argsSourceRow...).Scan(
		&sr.Visits, &sr.PaidOrders, &sr.SoldUnits, &sr.RevenueCents,
		&sr.NewCustomers, &sr.RepeatCustomers, &sr.ReturnsCount,
		&sr.PreviousVisits, &sr.PreviousRevenueCents,
		&sr.VisitsChangePct, &sr.RevenueChangePct,
	)
	if err != nil {
		sr.Visits = 0
	}
	if sr.Visits > 0 {
		sr.ConversionRate = float64(sr.PaidOrders) / float64(sr.Visits)
	}

	// 4. Top Products
	queryProducts := fmt.Sprintf(`
		WITH distinct_paid_orders AS (
			SELECT o.id AS order_id
			FROM orders o
			JOIN payments p ON p.order_id = o.id
			LEFT JOIN order_attributions oa ON oa.order_id = o.id
			WHERE p.status = 'succeeded'
			  AND o.status != 'cancelled'
			  AND p.paid_at >= $1 AND p.paid_at < $2
			  AND %s
			GROUP BY o.id
		),
		orders_data AS (
			SELECT oi.product_id,
				COUNT(DISTINCT oi.order_id) AS purchases,
				SUM(oi.quantity) AS sold_units,
				SUM(COALESCE(oip.total_customer_paid_cents, oi.price_cents * oi.quantity)) AS revenue_cents
			FROM order_items oi
			JOIN distinct_paid_orders dpo ON oi.order_id = dpo.order_id
			LEFT JOIN order_item_promotions oip ON oip.order_item_id = oi.id
			GROUP BY oi.product_id
		),
		behavioral AS (
			SELECT be.product_id,
				COUNT(*) FILTER (WHERE event_type = 'product_view') AS views,
				COUNT(*) FILTER (WHERE event_type = 'favorite_added') AS favorites,
				COUNT(*) FILTER (WHERE event_type = 'add_to_cart') AS add_to_cart
			FROM behavioral_events be
			JOIN analytics_sessions s ON be.session_id = s.id
			WHERE be.occurred_at >= $1 AND be.occurred_at < $2
			  AND be.product_id IS NOT NULL
			  AND %s
			GROUP BY be.product_id
		)
		SELECT p.id, p.title, p.main_image_url, s.brand_name, c.name,
			COALESCE(b.views, 0), COALESCE(b.favorites, 0), COALESCE(b.add_to_cart, 0),
			COALESCE(o.purchases, 0), COALESCE(o.sold_units, 0), COALESCE(o.revenue_cents, 0)
		FROM orders_data o
		JOIN products p ON o.product_id = p.id
		LEFT JOIN sellers s ON p.seller_id = s.id
		LEFT JOIN categories c ON p.category_id = c.id
		LEFT JOIN behavioral b ON p.id = b.product_id
		ORDER BY o.revenue_cents DESC
		LIMIT 5
	`, orderSourceFilterSub, sessionSourceFilterSub)

	rows, err := r.db.Query(ctx, queryProducts, argsSub...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var topProducts []ProductAnalyticsRow
	for rows.Next() {
		var pr ProductAnalyticsRow
		var rawID uuid.UUID
		err := rows.Scan(
			&rawID, &pr.ProductName, &pr.PrimaryImage, &pr.DesignerName, &pr.CategoryName,
			&pr.Views, &pr.Favorites, &pr.AddToCart,
			&pr.Purchases, &pr.SoldUnits, &pr.RevenueCents,
		)
		if err != nil {
			return nil, err
		}
		pr.ProductID = fmt.Sprintf("prod_%x", rawID[:8])
		if pr.DesignerName != nil && *pr.DesignerName == "" {
			pr.DesignerName = nil
		}
		if pr.Views > 0 {
			pr.ConversionRate = float64(pr.Purchases) / float64(pr.Views)
		}
		topProducts = append(topProducts, pr)
	}
	if topProducts == nil {
		topProducts = []ProductAnalyticsRow{}
	}

	// 5. Top Designers
	queryDesigners := fmt.Sprintf(`
		WITH distinct_paid_orders AS (
			SELECT o.id AS order_id
			FROM orders o
			JOIN payments p ON p.order_id = o.id
			LEFT JOIN order_attributions oa ON oa.order_id = o.id
			WHERE p.status = 'succeeded'
			  AND o.status != 'cancelled'
			  AND p.paid_at >= $1 AND p.paid_at < $2
			  AND %s
			GROUP BY o.id
		),
		orders_data AS (
			SELECT p.seller_id,
				COUNT(DISTINCT oi.order_id) AS purchases,
				SUM(oi.quantity) AS sold_units,
				SUM(COALESCE(oip.total_customer_paid_cents, oi.price_cents * oi.quantity)) AS revenue_cents
			FROM order_items oi
			JOIN distinct_paid_orders dpo ON oi.order_id = dpo.order_id
			JOIN products p ON oi.product_id = p.id
			LEFT JOIN order_item_promotions oip ON oip.order_item_id = oi.id
			GROUP BY p.seller_id
		),
		behavioral AS (
			SELECT p.seller_id,
				COUNT(*) FILTER (WHERE event_type = 'product_view') AS views,
				COUNT(*) FILTER (WHERE event_type = 'favorite_added') AS favorites,
				COUNT(*) FILTER (WHERE event_type = 'add_to_cart') AS add_to_cart
			FROM behavioral_events be
			JOIN analytics_sessions s ON be.session_id = s.id
			JOIN products p ON be.product_id = p.id
			WHERE be.occurred_at >= $1 AND be.occurred_at < $2
			  AND be.product_id IS NOT NULL
			  AND %s
			GROUP BY p.seller_id
		)
		SELECT s.id, s.brand_name, s.logo_url,
			COALESCE(b.views, 0), COALESCE(b.favorites, 0), COALESCE(b.add_to_cart, 0),
			COALESCE(o.purchases, 0), COALESCE(o.sold_units, 0), COALESCE(o.revenue_cents, 0)
		FROM orders_data o
		JOIN sellers s ON o.seller_id = s.id
		LEFT JOIN behavioral b ON s.id = b.seller_id
		ORDER BY o.revenue_cents DESC
		LIMIT 5
	`, orderSourceFilterSub, sessionSourceFilterSub)

	drows, err := r.db.Query(ctx, queryDesigners, argsSub...)
	if err != nil {
		return nil, err
	}
	defer drows.Close()

	var topDesigners []DesignerAnalyticsRow
	for drows.Next() {
		var dr DesignerAnalyticsRow
		var rawID uuid.UUID
		err := drows.Scan(
			&rawID, &dr.DesignerName, &dr.PrimaryImage,
			&dr.Views, &dr.Favorites, &dr.AddToCart,
			&dr.Purchases, &dr.SoldUnits, &dr.RevenueCents,
		)
		if err != nil {
			return nil, err
		}
		dr.DesignerID = fmt.Sprintf("dsgn_%x", rawID[:8])
		dr.DesignerName = formatDesignerName(dr.DesignerName)
		if dr.Views > 0 {
			dr.ConversionRate = float64(dr.Purchases) / float64(dr.Views)
		}
		topDesigners = append(topDesigners, dr)
	}
	if topDesigners == nil {
		topDesigners = []DesignerAnalyticsRow{}
	}

	// 6. Campaigns explicitly attributed to this source
	queryCampaigns := fmt.Sprintf(`
		WITH sessions_data AS (
			SELECT campaign_id, COUNT(id) AS visits
			FROM analytics_sessions s
			WHERE started_at >= $1 AND started_at < $2
			  AND %s AND campaign_id IS NOT NULL
			GROUP BY campaign_id
		),
		orders_data AS (
			SELECT oa.campaign_id,
				COUNT(DISTINCT o.id) AS purchases,
				SUM(COALESCE(oip.total_customer_paid_cents, oi.price_cents * oi.quantity)) AS revenue_cents
			FROM orders o
			JOIN payments p ON p.order_id = o.id
			JOIN order_attributions oa ON oa.order_id = o.id
			JOIN order_items oi ON oi.order_id = o.id
			LEFT JOIN order_item_promotions oip ON oip.order_item_id = oi.id
			WHERE p.status = 'succeeded'
			  AND o.status != 'cancelled'
			  AND p.paid_at >= $1 AND p.paid_at < $2
			  AND %s AND oa.campaign_id IS NOT NULL
			GROUP BY oa.campaign_id
		),
		all_camps AS (
			SELECT campaign_id FROM sessions_data UNION SELECT campaign_id FROM orders_data
		)
		SELECT c.id, c.title, c.status, c.planned_budget_cents,
			COALESCE(sd.visits, 0), COALESCE(od.purchases, 0), COALESCE(od.revenue_cents, 0)
		FROM all_camps ac
		JOIN marketing_campaigns c ON ac.campaign_id = c.id
		LEFT JOIN sessions_data sd ON ac.campaign_id = sd.campaign_id
		LEFT JOIN orders_data od ON ac.campaign_id = od.campaign_id
		ORDER BY od.revenue_cents DESC, sd.visits DESC
		LIMIT 50
	`, sessionSourceFilterSub, orderSourceFilterSub)

	crows, err := r.db.Query(ctx, queryCampaigns, argsSub...)
	if err != nil {
		return nil, err
	}
	defer crows.Close()

	var campaigns []CampaignPerformanceRow
	for crows.Next() {
		var cr CampaignPerformanceRow
		var rawID uuid.UUID
		var status string
		err := crows.Scan(
			&rawID, &cr.Name, &status, &cr.PlannedBudgetCents,
			&cr.Visits, &cr.PaidOrders, &cr.RevenueCents,
		)
		if err != nil {
			return nil, err
		}
		cr.Status = CampaignStatus(status)
		strID := rawID.String()
		cr.CampaignID = &strID
		if cr.Visits > 0 {
			cr.ConversionRateBps = int((float64(cr.PaidOrders) / float64(cr.Visits)) * 10000)
		}
		campaigns = append(campaigns, cr)
	}
	if campaigns == nil {
		campaigns = []CampaignPerformanceRow{}
	}

	// 7. Trend
	queryTrend := fmt.Sprintf(`
		WITH dates AS (
			SELECT generate_series(
				date_trunc('day', $1::timestamptz AT TIME ZONE 'UTC'),
				date_trunc('day', ($2::timestamptz - interval '1 millisecond') AT TIME ZONE 'UTC'),
				'1 day'::interval
			)::date AS d
		),
		paid_orders AS (
			SELECT
				date_trunc('day', p.paid_at AT TIME ZONE 'UTC')::date as order_date,
				o.id AS order_id,
				COALESCE(oip.total_customer_paid_cents, oi.price_cents * oi.quantity) AS revenue_cents
			FROM orders o
			JOIN payments p ON p.order_id = o.id
			JOIN order_items oi ON oi.order_id = o.id
			LEFT JOIN order_item_promotions oip ON oip.order_item_id = oi.id
			LEFT JOIN order_attributions oa ON oa.order_id = o.id
			WHERE p.status = 'succeeded'
			  AND o.status != 'cancelled'
			  AND p.paid_at >= $1 AND p.paid_at < $2
			  AND %s
		),
		daily_stats AS (
			SELECT order_date,
				COUNT(DISTINCT order_id) AS paid_orders,
				SUM(revenue_cents) AS revenue_cents
			FROM paid_orders
			GROUP BY order_date
		)
		SELECT
			to_char(d.d, 'YYYY-MM-DD'),
			COALESCE(ds.revenue_cents, 0),
			COALESCE(ds.paid_orders, 0)
		FROM dates d
		LEFT JOIN daily_stats ds ON d.d = ds.order_date
		ORDER BY d.d ASC
	`, orderSourceFilterSub)

	trows, err := r.db.Query(ctx, queryTrend, argsSub...)
	if err != nil {
		return nil, err
	}
	defer trows.Close()

	var trend []TrendDataPoint
	for trows.Next() {
		var tp TrendDataPoint
		err := trows.Scan(&tp.Date, &tp.RevenueCents, &tp.PaidOrders)
		if err != nil {
			return nil, err
		}
		trend = append(trend, tp)
	}
	if trend == nil {
		trend = []TrendDataPoint{}
	}

	return &SourceDetailResponse{
		Source:       sr,
		Coverage:     coverage,
		TopProducts:  topProducts,
		TopDesigners: topDesigners,
		Campaigns:    campaigns,
		Trend:        trend,
	}, nil
}
