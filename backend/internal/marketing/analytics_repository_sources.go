package marketing

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func (r *AnalyticsRepository) GetSourcesAnalytics(ctx context.Context, req SourceAnalyticsRequest) (*SourceAnalyticsResponse, error) {
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

	requestedSort := SourceSortVisits
	if req.Sort != nil && *req.Sort != "" {
		requestedSort = *req.Sort
	}

	if requestedSort == SourceSortConversion && coverage.Views.Status != CoverageAvailable {
		return nil, ErrConversionCoverageIncomplete
	}

	var sortClause string
	switch requestedSort {
	case SourceSortVisits:
		sortClause = "visits DESC, revenue_cents DESC, a.source ASC"
	case SourceSortRevenue:
		sortClause = "revenue_cents DESC, visits DESC, a.source ASC"
	case SourceSortOrders:
		sortClause = "purchases DESC, revenue_cents DESC, a.source ASC"
	case SourceSortConversion:
		sortClause = "(CAST(COALESCE(o.purchases, 0) AS FLOAT) / NULLIF(COALESCE(b.visits, 0), 0)) DESC NULLS LAST, revenue_cents DESC, a.source ASC"
	case SourceSortRevenueGrowth:
		sortClause = "revenue_change_pct DESC NULLS LAST, revenue_cents DESC, a.source ASC"
	case SourceSortRevenueDrop:
		sortClause = "revenue_change_pct ASC NULLS LAST, revenue_cents DESC, a.source ASC"
	default:
		return nil, ErrInvalidSort
	}

	duration := req.To.Sub(req.From)
	prevFrom := req.From.Add(-duration)
	prevTo := req.From

	var args []interface{}
	args = append(args, req.From, req.To, prevFrom, prevTo)
	argID := 5

	searchFilter := "1=1"
	if req.Search != nil && *req.Search != "" {
		searchFilter = fmt.Sprintf("a.source ILIKE $%d", argID)
		args = append(args, "%"+*req.Search+"%")
		argID++
	}

	query := fmt.Sprintf(`
		WITH sessions_data AS (
			SELECT COALESCE(source, '') AS source,
				COUNT(id) AS visits
			FROM analytics_sessions s
			WHERE started_at >= $1 AND started_at < $2
			GROUP BY COALESCE(source, '')
		),
		prev_sessions_data AS (
			SELECT COALESCE(source, '') AS source,
				COUNT(id) AS previous_visits
			FROM analytics_sessions s
			WHERE started_at >= $3 AND started_at < $4
			GROUP BY COALESCE(source, '')
		),
		distinct_paid_orders AS (
			SELECT o.id AS order_id, o.user_id, COALESCE(oa.source, '') AS source
			FROM orders o
			JOIN payments p ON p.order_id = o.id
			LEFT JOIN order_attributions oa ON oa.order_id = o.id
			WHERE p.status = 'succeeded'
			  AND o.status != 'cancelled'
			  AND p.paid_at >= $1 AND p.paid_at < $2
			GROUP BY o.id, o.user_id, COALESCE(oa.source, '')
		),
		first_orders AS (
			SELECT user_id, MIN(created_at) as first_order_at
			FROM orders
			WHERE status != 'cancelled'
			GROUP BY user_id
		),
		orders_data AS (
			SELECT dpo.source,
				COUNT(DISTINCT dpo.order_id) AS purchases,
				SUM(oi.quantity) AS sold_units,
				SUM(COALESCE(oip.total_customer_paid_cents, oi.price_cents * oi.quantity)) AS revenue_cents,
				COUNT(DISTINCT CASE WHEN dpo.user_id IS NOT NULL AND fo.first_order_at >= $1 AND fo.first_order_at < $2 THEN dpo.order_id END) AS new_customers,
				COUNT(DISTINCT CASE WHEN dpo.user_id IS NOT NULL AND fo.first_order_at < $1 THEN dpo.order_id END) AS repeat_customers
			FROM distinct_paid_orders dpo
			JOIN order_items oi ON oi.order_id = dpo.order_id
			LEFT JOIN first_orders fo ON fo.user_id = dpo.user_id
			LEFT JOIN order_item_promotions oip ON oip.order_item_id = oi.id
			GROUP BY dpo.source
		),
		returns_data AS (
			SELECT COALESCE(oa.source, '') AS source,
				COUNT(DISTINCT r.id) AS returns_count
			FROM return_items ri
			JOIN returns r ON ri.return_id = r.id
			JOIN order_items oi ON ri.order_item_id = oi.id
			JOIN orders o ON oi.order_id = o.id
			LEFT JOIN order_attributions oa ON oa.order_id = o.id
			WHERE r.status = 'completed'
			  AND COALESCE(r.completed_at, r.updated_at) >= $1
			  AND COALESCE(r.completed_at, r.updated_at) < $2
			GROUP BY COALESCE(oa.source, '')
		),
		prev_distinct_paid_orders AS (
			SELECT o.id AS order_id, COALESCE(oa.source, '') AS source
			FROM orders o
			JOIN payments p ON p.order_id = o.id
			LEFT JOIN order_attributions oa ON oa.order_id = o.id
			WHERE p.status = 'succeeded'
			  AND o.status != 'cancelled'
			  AND p.paid_at >= $3 AND p.paid_at < $4
			GROUP BY o.id, COALESCE(oa.source, '')
		),
		prev_orders_data AS (
			SELECT pdpo.source,
				SUM(COALESCE(oip.total_customer_paid_cents, oi.price_cents * oi.quantity)) AS previous_revenue_cents
			FROM prev_distinct_paid_orders pdpo
			JOIN order_items oi ON oi.order_id = pdpo.order_id
			LEFT JOIN order_item_promotions oip ON oip.order_item_id = oi.id
			GROUP BY pdpo.source
		),
		all_sources AS (
			SELECT source FROM sessions_data
			UNION
			SELECT source FROM orders_data
			UNION
			SELECT source FROM returns_data
		)
		SELECT
			a.source,
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
		FROM all_sources a
		LEFT JOIN sessions_data b ON a.source = b.source
		LEFT JOIN orders_data o ON a.source = o.source
		LEFT JOIN returns_data r ON a.source = r.source
		LEFT JOIN prev_sessions_data pb ON a.source = pb.source
		LEFT JOIN prev_orders_data po ON a.source = po.source
		WHERE %s
		ORDER BY %s LIMIT 100
	`, searchFilter, sortClause)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sources []SourcePerformanceRow
	for rows.Next() {
		var row SourcePerformanceRow
		var rawSource string
		err := rows.Scan(
			&rawSource, &row.Visits, &row.PaidOrders, &row.SoldUnits, &row.RevenueCents,
			&row.NewCustomers, &row.RepeatCustomers, &row.ReturnsCount,
			&row.PreviousVisits, &row.PreviousRevenueCents,
			&row.VisitsChangePct, &row.RevenueChangePct,
		)
		if err != nil {
			return nil, err
		}

		if rawSource == "" {
			row.SourceKey = nil
			row.SourceKind = SourceKindUnattributed
			row.Source = ""
		} else if strings.EqualFold(rawSource, "direct") {
			directKey := "direct"
			row.SourceKey = &directKey
			row.SourceKind = SourceKindDirect
			row.Source = "direct"
		} else {
			key := rawSource
			row.SourceKey = &key
			row.SourceKind = SourceKindNamed
			row.Source = rawSource
		}

		if row.Visits > 0 {
			row.ConversionRate = float64(row.PaidOrders) / float64(row.Visits)
		}
		sources = append(sources, row)
	}

	if sources == nil {
		sources = []SourcePerformanceRow{}
	}

	return &SourceAnalyticsResponse{
		Coverage: coverage,
		Sources:  sources,
	}, nil
}
