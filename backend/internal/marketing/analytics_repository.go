package marketing

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AnalyticsRepository struct {
	db *pgxpool.Pool
}

func NewAnalyticsRepository(db *pgxpool.Pool) *AnalyticsRepository {
	return &AnalyticsRepository{db: db}
}

func (r *AnalyticsRepository) GetOverviewMetrics(ctx context.Context, from, to time.Time) (MetricsSnapshot, error) {
	var ms MetricsSnapshot

	// 1. Visits
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(id) FROM analytics_sessions
		WHERE started_at >= $1 AND started_at < $2
	`, from, to).Scan(&ms.Visits)
	if err != nil {
		return ms, err
	}

	// 2. Paid Orders, Revenue, Sold Units
	// An order is counted as a paid order if it has a succeeded payment.
	// Revenue is the actual customer-paid gross amount (payments.amount_cents) for succeeded payments.
	// We group by order_id to guarantee that:
	// - each paid order is counted exactly once (even with multiple payment attempts/retries)
	// - revenue is counted exactly once per order
	// - sold units are counted exactly once per order without cartesian duplication
	// Unpaid orders and cancelled orders are excluded.
	err = r.db.QueryRow(ctx, `
		WITH paid_orders AS (
			SELECT
				o.id AS order_id,
				p.amount_cents AS revenue_cents
			FROM orders o
			JOIN payments p ON p.order_id = o.id
			WHERE p.status = 'succeeded'
			  AND o.status != 'cancelled'
			  AND p.paid_at >= $1 AND p.paid_at < $2
		),
		distinct_paid_orders AS (
			SELECT
				order_id,
				MAX(revenue_cents) AS revenue_cents
			FROM paid_orders
			GROUP BY order_id
		),
		order_units AS (
			SELECT
				oi.order_id,
				COALESCE(SUM(oi.quantity), 0) AS units
			FROM order_items oi
			GROUP BY oi.order_id
		)
		SELECT
			COUNT(dpo.order_id),
			COALESCE(SUM(dpo.revenue_cents), 0),
			COALESCE(SUM(ou.units), 0)
		FROM distinct_paid_orders dpo
		LEFT JOIN order_units ou ON ou.order_id = dpo.order_id
	`, from, to).Scan(&ms.PaidOrders, &ms.RevenueCents, &ms.SoldUnits)
	if err != nil {
		return ms, err
	}

	// 3. New vs Repeat Customers
	err = r.db.QueryRow(ctx, `
		WITH period_users AS (
			SELECT DISTINCT o.user_id
			FROM orders o
			JOIN payments p ON p.order_id = o.id
			WHERE p.status = 'succeeded'
			  AND o.status != 'cancelled'
			  AND p.paid_at >= $1 AND p.paid_at < $2
		)
		SELECT
			COUNT(CASE WHEN EXISTS (
				SELECT 1 FROM orders prev_o
				JOIN payments prev_p ON prev_p.order_id = prev_o.id
				WHERE prev_o.user_id = pu.user_id
				  AND prev_p.status = 'succeeded'
				  AND prev_o.status != 'cancelled'
				  AND prev_p.paid_at < $1
			) THEN 1 END) as repeat_customers,
			COUNT(CASE WHEN NOT EXISTS (
				SELECT 1 FROM orders prev_o
				JOIN payments prev_p ON prev_p.order_id = prev_o.id
				WHERE prev_o.user_id = pu.user_id
				  AND prev_p.status = 'succeeded'
				  AND prev_o.status != 'cancelled'
				  AND prev_p.paid_at < $1
			) THEN 1 END) as new_customers
		FROM period_users pu
	`, from, to).Scan(&ms.RepeatCustomers, &ms.NewCustomers)
	if err != nil {
		return ms, err
	}

	// 4. Returns Count and Units (canonical status 'completed')
	err = r.db.QueryRow(ctx, `
		SELECT
			COUNT(DISTINCT ret.id),
			COALESCE(SUM(ri.accepted_quantity), 0)
		FROM returns ret
		LEFT JOIN return_items ri ON ri.return_id = ret.id
		WHERE ret.status = 'completed'
		  AND COALESCE(ret.completed_at, ret.updated_at) >= $1
		  AND COALESCE(ret.completed_at, ret.updated_at) < $2
	`, from, to).Scan(&ms.ReturnsCount, &ms.ReturnedUnits)
	if err != nil {
		return ms, err
	}

	// 5. Returned Amount (from refunds)
	err = r.db.QueryRow(ctx, `
		SELECT COALESCE(SUM(amount_cents), 0)
		FROM refunds
		WHERE status IN ('succeeded', 'completed')
		  AND COALESCE(processed_at, created_at) >= $1
		  AND COALESCE(processed_at, created_at) < $2
	`, from, to).Scan(&ms.ReturnedAmountCents)
	if err != nil {
		return ms, err
	}

	// Derived metrics
	if ms.Visits > 0 {
		// Basis points (10000 = 100%)
		ms.ConversionRateBps = int(float64(ms.PaidOrders) / float64(ms.Visits) * 10000)
	}
	if ms.PaidOrders > 0 {
		ms.AovCents = ms.RevenueCents / int64(ms.PaidOrders)
	}

	return ms, nil
}

type RawSourceMetrics struct {
	Source       string
	Visits       int
	PaidOrders   int
	RevenueCents int64
}

func (r *AnalyticsRepository) GetSourceMetrics(ctx context.Context, from, to time.Time) ([]RawSourceMetrics, error) {
	visitMap := make(map[string]*RawSourceMetrics)

	// Get Visits per source
	vRows, err := r.db.Query(ctx, `
		SELECT
			CASE
				WHEN LOWER(COALESCE(utm_source, source, '')) = 'direct' THEN 'Direct'
				WHEN LOWER(COALESCE(utm_source, source, '')) IN ('missing', '') OR (utm_source IS NULL AND source IS NULL) THEN 'Unattributed'
				ELSE COALESCE(utm_source, source)
			END AS norm_source,
			COUNT(id)
		FROM analytics_sessions
		WHERE started_at >= $1 AND started_at < $2
		GROUP BY 1
	`, from, to)
	if err != nil {
		return nil, err
	}
	defer vRows.Close()

	for vRows.Next() {
		var src string
		var count int
		if err := vRows.Scan(&src, &count); err != nil {
			return nil, err
		}
		visitMap[src] = &RawSourceMetrics{Source: src, Visits: count}
	}

	// Get Orders & Revenue per source (each paid order counted once)
	oRows, err := r.db.Query(ctx, `
		WITH distinct_paid_orders AS (
			SELECT
				o.id AS order_id,
				MAX(p.amount_cents) AS revenue_cents
			FROM orders o
			JOIN payments p ON p.order_id = o.id
			WHERE p.status = 'succeeded'
			  AND o.status != 'cancelled'
			  AND p.paid_at >= $1 AND p.paid_at < $2
			GROUP BY o.id
		)
		SELECT
			CASE
				WHEN LOWER(COALESCE(oa.utm_source, oa.source, '')) = 'direct' THEN 'Direct'
				WHEN LOWER(COALESCE(oa.utm_source, oa.source, '')) IN ('missing', '') OR oa.order_id IS NULL OR (oa.utm_source IS NULL AND oa.source IS NULL) THEN 'Unattributed'
				ELSE COALESCE(oa.utm_source, oa.source)
			END AS norm_source,
			COUNT(dpo.order_id),
			COALESCE(SUM(dpo.revenue_cents), 0)
		FROM distinct_paid_orders dpo
		LEFT JOIN order_attributions oa ON oa.order_id = dpo.order_id
		GROUP BY 1
	`, from, to)
	if err != nil {
		return nil, err
	}
	defer oRows.Close()

	for oRows.Next() {
		var src string
		var orders int
		var rev int64
		if err := oRows.Scan(&src, &orders, &rev); err != nil {
			return nil, err
		}
		if _, exists := visitMap[src]; !exists {
			visitMap[src] = &RawSourceMetrics{Source: src}
		}
		visitMap[src].PaidOrders = orders
		visitMap[src].RevenueCents = rev
	}

	var result []RawSourceMetrics
	for _, m := range visitMap {
		result = append(result, *m)
	}

	return result, nil
}

type RawCampaignMetrics struct {
	Status             CampaignStatus
	PlannedBudgetCents *int64
	CampaignID         *uuid.UUID
	Name               string
	Visits             int
	PaidOrders         int
	RevenueCents       int64
}

func (r *AnalyticsRepository) GetCampaignMetrics(ctx context.Context, from, to time.Time) ([]RawCampaignMetrics, error) {
	campMap := make(map[string]*RawCampaignMetrics)

	// Fetch mapping of CampaignID to Title so we don't have to join on every group
	cRows, err := r.db.Query(ctx, `SELECT id, title, status, planned_budget_cents FROM marketing_campaigns WHERE purpose = 'advertising'`)
	if err != nil {
		return nil, err
	}
	defer cRows.Close()

	names := make(map[uuid.UUID]RawCampaignMetrics)
	for cRows.Next() {
		var id uuid.UUID
		var metadata RawCampaignMetrics
		if err := cRows.Scan(&id, &metadata.Name, &metadata.Status, &metadata.PlannedBudgetCents); err != nil {
			return nil, err
		}
		names[id] = metadata
	}

	// 1. Get Visits per campaign
	vRows, err := r.db.Query(ctx, `
		SELECT campaign_id, COUNT(id)
		FROM analytics_sessions
		WHERE started_at >= $1 AND started_at < $2
		GROUP BY 1
	`, from, to)
	if err != nil {
		return nil, err
	}
	defer vRows.Close()

	for vRows.Next() {
		var cID *uuid.UUID
		var count int
		if err := vRows.Scan(&cID, &count); err != nil {
			return nil, err
		}
		key := "none"
		name := "Без кампании"
		if cID != nil {
			key = cID.String()
			if n, ok := names[*cID]; ok {
				name = n.Name
			} else {
				continue // Exclude promotion-domain rows, preserving their attribution in storage.
			}
		}

		campMap[key] = &RawCampaignMetrics{CampaignID: cID, Name: name, Visits: count}
	}

	// 2. Get Orders & Revenue per campaign (each paid order counted once)
	oRows, err := r.db.Query(ctx, `
		WITH distinct_paid_orders AS (
			SELECT
				o.id AS order_id,
				MAX(p.amount_cents) AS revenue_cents
			FROM orders o
			JOIN payments p ON p.order_id = o.id
			WHERE p.status = 'succeeded'
			  AND o.status != 'cancelled'
			  AND p.paid_at >= $1 AND p.paid_at < $2
			GROUP BY o.id
		)
		SELECT
			oa.campaign_id,
			COUNT(dpo.order_id),
			COALESCE(SUM(dpo.revenue_cents), 0)
		FROM distinct_paid_orders dpo
		LEFT JOIN order_attributions oa ON oa.order_id = dpo.order_id
		GROUP BY 1
	`, from, to)
	if err != nil {
		return nil, err
	}
	defer oRows.Close()

	for oRows.Next() {
		var cID *uuid.UUID
		var orders int
		var rev int64
		if err := oRows.Scan(&cID, &orders, &rev); err != nil {
			return nil, err
		}
		key := "none"
		name := "Без кампании"
		if cID != nil {
			key = cID.String()
			if n, ok := names[*cID]; ok {
				name = n.Name
			} else {
				continue // Exclude promotion-domain rows, preserving their attribution in storage.
			}
		}

		if _, exists := campMap[key]; !exists {
			campMap[key] = &RawCampaignMetrics{CampaignID: cID, Name: name}
		}
		campMap[key].PaidOrders = orders
		campMap[key].RevenueCents = rev
	}

	var result []RawCampaignMetrics
	for _, m := range campMap {
		if m.CampaignID != nil {
			metadata := names[*m.CampaignID]
			m.Status = metadata.Status
			m.PlannedBudgetCents = metadata.PlannedBudgetCents
		}
		result = append(result, *m)
	}

	return result, nil
}

func (r *AnalyticsRepository) GetTrendMetrics(ctx context.Context, from, to time.Time) ([]TrendDataPoint, error) {
	// Daily buckets using generate_series with a left join to distinct paid orders.
	// Groups orders by o.id to ensure idempotency: retry attempts and duplicate payments
	// for the same order are counted exactly once, identically to GetOverviewMetrics.
	rows, err := r.db.Query(ctx, `
		WITH dates AS (
			SELECT generate_series(
				date_trunc('day', $1::timestamptz AT TIME ZONE 'UTC'),
				date_trunc('day', ($2::timestamptz - interval '1 millisecond') AT TIME ZONE 'UTC'),
				'1 day'::interval
			)::date AS d
		),
		paid_orders AS (
			SELECT
				o.id AS order_id,
				p.amount_cents AS revenue_cents,
				date_trunc('day', p.paid_at AT TIME ZONE 'UTC')::date AS paid_date
			FROM orders o
			JOIN payments p ON p.order_id = o.id
			WHERE p.status = 'succeeded'
			  AND o.status != 'cancelled'
			  AND p.paid_at >= $1 AND p.paid_at < $2
		),
		distinct_paid_orders AS (
			SELECT
				order_id,
				MAX(revenue_cents) AS revenue_cents,
				MIN(paid_date) AS paid_date
			FROM paid_orders
			GROUP BY order_id
		)
		SELECT
			dates.d,
			COUNT(dpo.order_id) AS paid_orders,
			COALESCE(SUM(dpo.revenue_cents), 0) AS revenue_cents
		FROM dates
		LEFT JOIN distinct_paid_orders dpo ON dates.d = dpo.paid_date
		GROUP BY dates.d
		ORDER BY dates.d ASC
	`, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	trend := make([]TrendDataPoint, 0)
	for rows.Next() {
		var date time.Time
		var pt TrendDataPoint
		if err := rows.Scan(&date, &pt.PaidOrders, &pt.RevenueCents); err != nil {
			return nil, err
		}
		pt.Date = date.Format("2006-01-02")
		trend = append(trend, pt)
	}

	return trend, nil
}
