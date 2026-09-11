package products

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/notifications"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ForecastReconciliationAdvisoryLockKey is the canonical PostgreSQL transaction-level
// advisory lock key for synchronizing stock forecast reconciliations.
// Value corresponds to 64-bit ASCII "ZAMKSFRC" (0x5a414d4b53465243).
const ForecastReconciliationAdvisoryLockKey int64 = 0x5a414d4b53465243

// StockForecastDedupeKey returns the canonical dedupe key for a product's stock forecast alert.
func StockForecastDedupeKey(productID uuid.UUID) string {
	return fmt.Sprintf("stock:forecast:product:%s", productID.String())
}

// VariantForecastInput holds raw variant and sales data needed for forecast computation.
type VariantForecastInput struct {
	VariantID         uuid.UUID
	ProductID         uuid.UUID
	SellerID          uuid.UUID
	ProductTitle      string
	ProductStatus     string
	SellerStatus      string
	VariantIsActive   bool
	VariantCreatedAt  time.Time
	SellerSKU         string
	Color             string
	Size              string
	FreeSellableStock int
	ObservationStart  time.Time
	PaidDemandUnits   int
}

// VariantForecastMetrics holds computed velocity, days of cover, and risk classification.
type VariantForecastMetrics struct {
	VariantID           uuid.UUID `json:"variantId"`
	SellerSKU           string    `json:"sellerSku"`
	Color               string    `json:"color"`
	Size                string    `json:"size"`
	FreeSellableStock   int       `json:"freeSellableStock"`
	PaidDemandUnits     int       `json:"paidDemandUnits"`
	ObservedDays        int       `json:"observedDays"`
	DailySalesVelocity  float64   `json:"dailySalesVelocity"`
	DaysOfCover         float64   `json:"daysOfCover"`
	Severity            string    `json:"severity"`
	IsAtRisk            bool      `json:"-"`
	EligibleForForecast bool      `json:"-"`
}

// ComputeVariantForecast computes observed days, daily sales velocity, days of cover,
// and applies the forecast risk rules and hysteresis.
func ComputeVariantForecast(input VariantForecastInput, now time.Time, wasInActiveAlert bool) VariantForecastMetrics {
	// Calendar elapsed days since observation start:
	calendarDays := int(now.UTC().Truncate(24*time.Hour).Sub(input.ObservationStart.UTC().Truncate(24*time.Hour)).Hours() / 24)
	observedDays := calendarDays
	if observedDays < 1 {
		observedDays = 1
	}
	if observedDays > 30 {
		observedDays = 30
	}

	// Demand evidence rule:
	// eligibleForForecast = paidDemandUnits >= 2 OR observedDays >= 7
	// However: paidDemandUnits = 0 -> DSV = 0 -> NO forecast alert.
	eligibleForForecast := (input.PaidDemandUnits >= 2 || observedDays >= 7)

	var dsv float64
	var daysOfCover float64
	var hasCover bool

	if input.PaidDemandUnits > 0 {
		dsv = float64(input.PaidDemandUnits) / float64(observedDays)
		daysOfCover = float64(input.FreeSellableStock) / dsv
		hasCover = true
	}

	severity, isAtRisk := ClassifyVariantRisk(
		eligibleForForecast,
		input.PaidDemandUnits,
		daysOfCover,
		hasCover,
		wasInActiveAlert,
	)

	return VariantForecastMetrics{
		VariantID:           input.VariantID,
		SellerSKU:           input.SellerSKU,
		Color:               input.Color,
		Size:                input.Size,
		FreeSellableStock:   input.FreeSellableStock,
		PaidDemandUnits:     input.PaidDemandUnits,
		ObservedDays:        observedDays,
		DailySalesVelocity:  dsv,
		DaysOfCover:         daysOfCover,
		Severity:            severity,
		IsAtRisk:            isAtRisk,
		EligibleForForecast: eligibleForForecast,
	}
}

// ClassifyVariantRisk classifies risk level and handles hysteresis.
func ClassifyVariantRisk(
	eligibleForForecast bool,
	paidDemandUnits int,
	daysOfCover float64,
	hasCover bool,
	wasInActiveAlert bool,
) (string, bool) {
	if !eligibleForForecast || !hasCover || paidDemandUnits <= 0 {
		return "", false
	}

	if wasInActiveAlert {
		// Hysteresis threshold: < 18 days to remain at risk
		if daysOfCover >= 18.0 {
			return "", false
		}
	} else {
		// New entry threshold: <= 14 days
		if daysOfCover > 14.0 {
			return "", false
		}
	}

	if daysOfCover <= 7.0 {
		return notifications.SeverityCritical, true
	}
	return notifications.SeverityWarning, true
}

func formatVariantLabel(color, size, sku string) string {
	c := strings.TrimSpace(color)
	s := strings.TrimSpace(size)
	if c != "" && s != "" {
		return fmt.Sprintf("%s / %s", c, s)
	}
	if s != "" {
		return s
	}
	if c != "" {
		return c
	}
	k := strings.TrimSpace(sku)
	if k != "" {
		return k
	}
	return "Основной"
}

func formatDaysRussian(days int) string {
	if days < 0 {
		days = 0
	}
	mod100 := days % 100
	mod10 := days % 10
	if mod100 >= 11 && mod100 <= 19 {
		return fmt.Sprintf("%d дней", days)
	}
	switch mod10 {
	case 1:
		return fmt.Sprintf("%d день", days)
	case 2, 3, 4:
		return fmt.Sprintf("%d дня", days)
	default:
		return fmt.Sprintf("%d дней", days)
	}
}

// BuildProductForecastCopy generates seller-facing title and body summarizing at-risk variants.
func BuildProductForecastCopy(atRiskVariants []VariantForecastMetrics, severity string) (string, string) {
	var title string
	if severity == notifications.SeverityCritical {
		title = "Товар может закончиться в ближайшие дни"
	} else {
		title = "Запас товара скоро закончится"
	}

	parts := make([]string, 0, 4)
	showCount := len(atRiskVariants)
	if showCount > 3 {
		showCount = 3
	}
	for i := 0; i < showCount; i++ {
		v := atRiskVariants[i]
		days := int(math.Round(v.DaysOfCover))
		label := formatVariantLabel(v.Color, v.Size, v.SellerSKU)
		parts = append(parts, fmt.Sprintf("%s — примерно на %s", label, formatDaysRussian(days)))
	}
	if len(atRiskVariants) > 3 {
		parts = append(parts, fmt.Sprintf("Ещё: %d", len(atRiskVariants)-3))
	}
	body := strings.Join(parts, "; ") + "."
	return title, body
}

func roundFloat(val float64, precision int) float64 {
	pow := math.Pow(10, float64(precision))
	return math.Round(val*pow) / pow
}

type activeForecastAlertInfo struct {
	id        uuid.UUID
	sellerID  uuid.UUID
	dedupeKey string
	variants  map[uuid.UUID]bool
}

// ReconcileStockForecastAlerts reconciles stock forecast alerts for all published products in a new transaction.
func (s *Service) ReconcileStockForecastAlerts(ctx context.Context) error {
	if s.dbPool == nil {
		return nil
	}
	return s.dbPool.RunInTx(ctx, func(tx pgx.Tx) error {
		return s.ReconcileStockForecastAlertsTx(ctx, tx, time.Now().UTC())
	})
}

// ReconcileStockForecastForProduct reconciles stock forecast alerts for a single product in a new transaction.
func (s *Service) ReconcileStockForecastForProduct(ctx context.Context, productID uuid.UUID) error {
	if s.dbPool == nil {
		return nil
	}
	return s.dbPool.RunInTx(ctx, func(tx pgx.Tx) error {
		return s.ReconcileStockForecastForProductTx(ctx, tx, productID, time.Now().UTC())
	})
}

// ReconcileStockForecastAlertsTx reconciles stock forecast alerts for all published products.
func (s *Service) ReconcileStockForecastAlertsTx(ctx context.Context, tx pgx.Tx, now time.Time) error {
	return s.reconcileStockForecastInternal(ctx, tx, uuid.Nil, now)
}

// ReconcileStockForecastForProductTx reconciles stock forecast alert for a specific product.
func (s *Service) ReconcileStockForecastForProductTx(ctx context.Context, tx pgx.Tx, productID uuid.UUID, now time.Time) error {
	return s.reconcileStockForecastInternal(ctx, tx, productID, now)
}

func (s *Service) reconcileStockForecastInternal(ctx context.Context, tx pgx.Tx, targetProductID uuid.UUID, now time.Time) error {
	// Step 0: Acquire transaction-scoped advisory lock to serialize forecast reconciliation.
	// Both full and product-scoped reconciliations acquire this same lock so that
	// concurrent executions never race on snapshot persistence, active alerts, or stale cleanup.
	// The lock releases automatically when tx commits or rolls back.
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, ForecastReconciliationAdvisoryLockKey); err != nil {
		return fmt.Errorf("failed to acquire forecast reconciliation advisory lock: %w", err)
	}

	// Step 1: Load active forecast alerts
	activeAlertsQuery := `
		SELECT id, recipient_seller_id, dedupe_key, metadata
		FROM notifications
		WHERE kind = 'alert' AND status = 'active' AND dedupe_key LIKE 'stock:forecast:product:%'
	`
	var activeArgs []interface{}
	if targetProductID != uuid.Nil {
		activeAlertsQuery += ` AND dedupe_key = $1`
		activeArgs = append(activeArgs, StockForecastDedupeKey(targetProductID))
	}

	activeRows, err := tx.Query(ctx, activeAlertsQuery, activeArgs...)
	if err != nil {
		return fmt.Errorf("failed to query active forecast alerts: %w", err)
	}
	defer activeRows.Close()

	activeAlerts := make(map[uuid.UUID]*activeForecastAlertInfo)
	for activeRows.Next() {
		var a activeForecastAlertInfo
		var rawMeta []byte
		if err := activeRows.Scan(&a.id, &a.sellerID, &a.dedupeKey, &rawMeta); err != nil {
			return fmt.Errorf("failed to scan active forecast alert: %w", err)
		}
		a.variants = make(map[uuid.UUID]bool)
		var metaMap map[string]interface{}
		if err := json.Unmarshal(rawMeta, &metaMap); err == nil {
			if vList, ok := metaMap["variants"].([]interface{}); ok {
				for _, rawItem := range vList {
					if itemMap, ok := rawItem.(map[string]interface{}); ok {
						if vIDStr, ok := itemMap["variantId"].(string); ok {
							if vID, err := uuid.Parse(vIDStr); err == nil {
								a.variants[vID] = true
							}
						}
					}
				}
			}
		}

		pIDStr := strings.TrimPrefix(a.dedupeKey, "stock:forecast:product:")
		if pID, err := uuid.Parse(pIDStr); err == nil {
			activeAlerts[pID] = &a
		}
	}
	if err := activeRows.Err(); err != nil {
		return fmt.Errorf("error reading active forecast alerts: %w", err)
	}

	// Step 2: Query eligible variants and 30-day paid demand
	variantsQuery := `
		WITH order_first_paid AS (
			SELECT
				p.order_id,
				MIN(p.paid_at) AS first_paid_at
			FROM payments p
			WHERE p.status IN ('succeeded', 'paid')
			  AND p.paid_at IS NOT NULL
			GROUP BY p.order_id
		),
		valid_orders AS (
			SELECT
				o.id AS order_id,
				ofp.first_paid_at
			FROM orders o
			JOIN order_first_paid ofp ON o.id = ofp.order_id
			WHERE o.cancelled_at IS NULL
			  AND ofp.first_paid_at >= $1::timestamptz - INTERVAL '30 days'
		),
		recent_order_items AS (
			SELECT
				oi.product_variant_id,
				oi.quantity,
				vo.first_paid_at
			FROM order_items oi
			JOIN valid_orders vo ON oi.order_id = vo.order_id
		)
		SELECT
			pv.id AS variant_id,
			pv.product_id,
			p.seller_id,
			p.title AS product_title,
			p.status AS product_status,
			p.published_at,
			s.status AS seller_status,
			pv.is_active AS variant_is_active,
			pv.created_at AS variant_created_at,
			COALESCE(pv.seller_sku, pv.sku, '') AS seller_sku,
			COALESCE(c.name_ru, pv.color, '') AS color,
			COALESCE(sv.value, pv.size, '') AS size,
			GREATEST(0, COALESCE(ii.total_stock, 0) - COALESCE(ii.reserved_stock, 0)) AS free_sellable_stock,
			GREATEST(p.published_at, pv.created_at) AS observation_start,
			COALESCE(SUM(
				CASE
					WHEN roi.first_paid_at >= GREATEST($1::timestamptz - INTERVAL '30 days', GREATEST(p.published_at, pv.created_at))
					THEN roi.quantity
					ELSE 0
				END
			), 0) AS paid_demand_units
		FROM product_variants pv
		JOIN products p ON pv.product_id = p.id
		JOIN sellers s ON p.seller_id = s.id
		LEFT JOIN size_values sv ON pv.size_value_id = sv.id
		LEFT JOIN colors c ON pv.color_id = c.id
		LEFT JOIN inventory_items ii ON pv.id = ii.product_variant_id
		LEFT JOIN recent_order_items roi ON pv.id = roi.product_variant_id
		WHERE s.status = 'active'
		  AND p.status = 'published'
		  AND p.published_at IS NOT NULL
		  AND pv.is_active = true
	`

	variantArgs := []interface{}{now}
	if targetProductID != uuid.Nil {
		variantsQuery += ` AND p.id = $2`
		variantArgs = append(variantArgs, targetProductID)
	}

	variantsQuery += `
		GROUP BY pv.id, pv.product_id, p.seller_id, p.title, p.status, p.published_at, s.status,
		         pv.is_active, pv.created_at, pv.seller_sku, pv.sku, c.name_ru, pv.color, sv.value, pv.size,
		         ii.total_stock, ii.reserved_stock
		ORDER BY pv.product_id, pv.id
	`

	rows, err := tx.Query(ctx, variantsQuery, variantArgs...)
	if err != nil {
		return fmt.Errorf("failed to query eligible variants for forecast: %w", err)
	}
	defer rows.Close()

	type productGroup struct {
		sellerID     uuid.UUID
		productTitle string
		variants     []VariantForecastInput
	}
	grouped := make(map[uuid.UUID]*productGroup)
	var productOrder []uuid.UUID

	for rows.Next() {
		var input VariantForecastInput
		var publishedAt *time.Time
		if err := rows.Scan(
			&input.VariantID,
			&input.ProductID,
			&input.SellerID,
			&input.ProductTitle,
			&input.ProductStatus,
			&publishedAt,
			&input.SellerStatus,
			&input.VariantIsActive,
			&input.VariantCreatedAt,
			&input.SellerSKU,
			&input.Color,
			&input.Size,
			&input.FreeSellableStock,
			&input.ObservationStart,
			&input.PaidDemandUnits,
		); err != nil {
			return fmt.Errorf("failed to scan variant forecast input: %w", err)
		}

		g, exists := grouped[input.ProductID]
		if !exists {
			g = &productGroup{
				sellerID:     input.SellerID,
				productTitle: input.ProductTitle,
				variants:     nil,
			}
			grouped[input.ProductID] = g
			productOrder = append(productOrder, input.ProductID)
		}
		g.variants = append(g.variants, input)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("error reading variant forecast inputs: %w", err)
	}

	handledProducts := make(map[uuid.UUID]bool)
	var evaluatedVariantIDs []uuid.UUID

	// Step 3: Compute forecasts and reconcile per product
	for _, pID := range productOrder {
		g := grouped[pID]
		handledProducts[pID] = true
		dedupeKey := StockForecastDedupeKey(pID)
		activeAlert := activeAlerts[pID]

		var atRiskVariants []VariantForecastMetrics
		for _, v := range g.variants {
			evaluatedVariantIDs = append(evaluatedVariantIDs, v.VariantID)
			wasInActive := false
			if activeAlert != nil && activeAlert.variants != nil {
				wasInActive = activeAlert.variants[v.VariantID]
			}

			metrics := ComputeVariantForecast(v, now, wasInActive)

			// Determine persistent snapshot state
			var state string
			var cover *float64
			if !metrics.EligibleForForecast {
				state = "insufficient_data"
			} else if metrics.PaidDemandUnits == 0 {
				state = "no_sales"
			} else if !metrics.IsAtRisk {
				state = "healthy"
				c := metrics.DaysOfCover
				cover = &c
			} else if metrics.Severity == notifications.SeverityWarning {
				state = "warning"
				c := metrics.DaysOfCover
				cover = &c
			} else if metrics.Severity == notifications.SeverityCritical {
				state = "critical"
				c := metrics.DaysOfCover
				cover = &c
			}

			upsertQuery := `
				INSERT INTO variant_stock_forecasts (product_variant_id, state, days_of_cover, calculated_at)
				VALUES ($1, $2, $3, $4)
				ON CONFLICT (product_variant_id) DO UPDATE SET
					state = EXCLUDED.state,
					days_of_cover = EXCLUDED.days_of_cover,
					calculated_at = EXCLUDED.calculated_at
			`
			if _, err := tx.Exec(ctx, upsertQuery, metrics.VariantID, state, cover, now); err != nil {
				return fmt.Errorf("failed to upsert variant forecast snapshot: %w", err)
			}

			if metrics.IsAtRisk {
				atRiskVariants = append(atRiskVariants, metrics)
			}
		}

		if len(atRiskVariants) == 0 {
			// No remaining at-risk variants: resolve active alert if exists
			if activeAlert != nil && s.notifs != nil {
				if err := s.notifs.ResolveActiveSellerAlertTx(ctx, tx, g.sellerID, dedupeKey); err != nil {
					return fmt.Errorf("failed to resolve active forecast alert for product %s: %w", pID, err)
				}
			}
			continue
		}

		// Sort variants by daysOfCover ASC, then VariantID ASC for determinism
		sort.Slice(atRiskVariants, func(i, j int) bool {
			if atRiskVariants[i].DaysOfCover != atRiskVariants[j].DaysOfCover {
				return atRiskVariants[i].DaysOfCover < atRiskVariants[j].DaysOfCover
			}
			return atRiskVariants[i].VariantID.String() < atRiskVariants[j].VariantID.String()
		})

		worstDaysOfCover := atRiskVariants[0].DaysOfCover
		productSeverity := notifications.SeverityWarning
		for _, v := range atRiskVariants {
			if v.Severity == notifications.SeverityCritical {
				productSeverity = notifications.SeverityCritical
				break
			}
		}

		title, body := BuildProductForecastCopy(atRiskVariants, productSeverity)
		actionURL := "/supplies/new"

		variantsMeta := make([]map[string]interface{}, 0, len(atRiskVariants))
		for _, v := range atRiskVariants {
			variantsMeta = append(variantsMeta, map[string]interface{}{
				"variantId":          v.VariantID.String(),
				"sellerSku":          v.SellerSKU,
				"color":              v.Color,
				"size":               v.Size,
				"freeSellableStock":  v.FreeSellableStock,
				"paidDemandUnits":    v.PaidDemandUnits,
				"observedDays":       v.ObservedDays,
				"dailySalesVelocity": roundFloat(v.DailySalesVelocity, 4),
				"daysOfCover":        roundFloat(v.DaysOfCover, 2),
				"severity":           v.Severity,
			})
		}

		metadata := map[string]interface{}{
			"productId":        pID.String(),
			"productTitle":     g.productTitle,
			"worstDaysOfCover": roundFloat(worstDaysOfCover, 2),
			"variantCount":     len(atRiskVariants),
			"variants":         variantsMeta,
		}

		alertNotif := notifications.Notification{
			RecipientKind:     notifications.RecipientKindSeller,
			RecipientSellerID: &g.sellerID,
			Kind:              notifications.KindAlert,
			Severity:          productSeverity,
			Type:              notifications.TypeStockForecastRisk,
			Title:             title,
			Body:              body,
			ActionURL:         &actionURL,
			DedupeKey:         &dedupeKey,
			EntityType:        "product",
			EntityID:          pID,
			Metadata:          metadata,
		}

		if s.notifs != nil {
			if _, err := s.notifs.CreateOrUpdateActiveSellerAlertTx(ctx, tx, alertNotif); err != nil {
				return fmt.Errorf("failed to upsert forecast alert for product %s: %w", pID, err)
			}
		}
	}

	// Step 4: Resolve orphaned active alerts for products no longer eligible
	if targetProductID == uuid.Nil {
		for pID, alert := range activeAlerts {
			if !handledProducts[pID] && s.notifs != nil {
				if err := s.notifs.ResolveActiveSellerAlertTx(ctx, tx, alert.sellerID, alert.dedupeKey); err != nil {
					return fmt.Errorf("failed to resolve orphaned forecast alert for product %s: %w", pID, err)
				}
			}
		}
	} else {
		if !handledProducts[targetProductID] {
			if alert, exists := activeAlerts[targetProductID]; exists && s.notifs != nil {
				if err := s.notifs.ResolveActiveSellerAlertTx(ctx, tx, alert.sellerID, alert.dedupeKey); err != nil {
					return fmt.Errorf("failed to resolve orphaned forecast alert for product %s: %w", targetProductID, err)
				}
			}
		}
	}

	// Step 5: Clean up stale snapshots
	if targetProductID == uuid.Nil {
		// Full reconciliation: delete snapshots not evaluated in this full run
		if len(evaluatedVariantIDs) > 0 {
			if _, err := tx.Exec(ctx, `DELETE FROM variant_stock_forecasts WHERE NOT (product_variant_id = ANY($1))`, evaluatedVariantIDs); err != nil {
				return fmt.Errorf("failed to clean up stale variant forecasts: %w", err)
			}
		} else {
			if _, err := tx.Exec(ctx, `DELETE FROM variant_stock_forecasts`); err != nil {
				return fmt.Errorf("failed to clean up stale variant forecasts: %w", err)
			}
		}
	} else {
		// Product-scoped reconciliation: NEVER delete other products' snapshots!
		// Delete only snapshots for variants belonging to targetProductID that were not evaluated in this run
		if len(evaluatedVariantIDs) > 0 {
			cleanupQuery := `
				DELETE FROM variant_stock_forecasts
				WHERE product_variant_id IN (
					SELECT id FROM product_variants WHERE product_id = $1
				)
				AND NOT (product_variant_id = ANY($2))
			`
			if _, err := tx.Exec(ctx, cleanupQuery, targetProductID, evaluatedVariantIDs); err != nil {
				return fmt.Errorf("failed to clean up stale variant forecasts for product %s: %w", targetProductID, err)
			}
		} else {
			cleanupQuery := `
				DELETE FROM variant_stock_forecasts
				WHERE product_variant_id IN (
					SELECT id FROM product_variants WHERE product_id = $1
				)
			`
			if _, err := tx.Exec(ctx, cleanupQuery, targetProductID); err != nil {
				return fmt.Errorf("failed to clean up stale variant forecasts for product %s: %w", targetProductID, err)
			}
		}
	}

	return nil
}
