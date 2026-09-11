package router_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/notifications"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/products"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/sellers"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/users"
)

type paymentFixture struct {
	status   string
	paidAt   *time.Time
	failedAt *time.Time
}

func setupForecastTestEnvironment(t *testing.T) (
	context.Context,
	*postgres.Client,
	*products.Service,
	*notifications.Service,
	func(),
) {
	ctx := context.Background()

	pgClient, err := postgres.NewClient(ctx, testDBURL)
	require.NoError(t, err)

	var dbName string
	err = pgClient.Pool.QueryRow(ctx, "SELECT current_database()").Scan(&dbName)
	require.NoError(t, err)
	require.Equal(t, "zamk_test", dbName, "tests must strictly run against zamk_test")

	userRepo := users.NewRepository(pgClient.Pool)
	notifsRepo := notifications.NewRepository(pgClient)
	notifsService := notifications.NewService(notifsRepo, userRepo, nil)

	sellersRepo := sellers.NewRepository(pgClient.Pool)
	productsRepo := products.NewRepository(pgClient.Pool)
	productsService := products.NewService(productsRepo, sellersRepo, pgClient, nil, notifsService)

	cleanup := func() {
		pgClient.Close()
	}

	return ctx, pgClient, productsService, notifsService, cleanup
}

func TestNTF3_StockForecastAlerts(t *testing.T) {
	ctx, pgClient, productsService, _, cleanup := setupForecastTestEnvironment(t)
	defer cleanup()

	var (
		createdUserIDs     []uuid.UUID
		createdSellerIDs   []uuid.UUID
		createdCategoryIDs []uuid.UUID
		createdProductIDs  []uuid.UUID
		createdVariantIDs  []uuid.UUID
		createdOrderIDs    []uuid.UUID
	)

	t.Cleanup(func() {
		var dbName string
		if err := pgClient.Pool.QueryRow(ctx, "SELECT current_database()").Scan(&dbName); err == nil && dbName == "zamk_test" {
			if len(createdOrderIDs) > 0 {
				_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM payments WHERE order_id = ANY($1)", createdOrderIDs)
				_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM order_items WHERE order_id = ANY($1)", createdOrderIDs)
				_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM order_fulfillments WHERE order_id = ANY($1)", createdOrderIDs)
				_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM orders WHERE id = ANY($1)", createdOrderIDs)
			}
			if len(createdSellerIDs) > 0 {
				_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM notifications WHERE recipient_seller_id = ANY($1)", createdSellerIDs)
				_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM seller_users WHERE seller_id = ANY($1)", createdSellerIDs)
			}
			if len(createdVariantIDs) > 0 {
				_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM inventory_items WHERE product_variant_id = ANY($1)", createdVariantIDs)
				_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM product_variants WHERE id = ANY($1)", createdVariantIDs)
			}
			if len(createdProductIDs) > 0 {
				_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM products WHERE id = ANY($1)", createdProductIDs)
			}
			if len(createdSellerIDs) > 0 {
				_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM sellers WHERE id = ANY($1)", createdSellerIDs)
			}
			if len(createdCategoryIDs) > 0 {
				_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM categories WHERE id = ANY($1)", createdCategoryIDs)
			}
			if len(createdUserIDs) > 0 {
				_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM users WHERE id = ANY($1)", createdUserIDs)
			}
		}
	})

	// 1. Create Base Seller
	sellerOwnerID := uuid.New()
	createdUserIDs = append(createdUserIDs, sellerOwnerID)
	_, err := pgClient.Pool.Exec(ctx, `
		INSERT INTO users (id, email, phone, name, password_hash, role, status, created_at, updated_at)
		VALUES ($1, $2, $3, 'Forecast Owner', 'hash', 'seller', 'active', now(), now())
	`, sellerOwnerID, "owner-"+sellerOwnerID.String()[:8]+"@test.com", "+7995"+sellerOwnerID.String()[:7])
	require.NoError(t, err)

	sellerID := uuid.New()
	createdSellerIDs = append(createdSellerIDs, sellerID)
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO sellers (id, brand_name, slug, contact_email, status, created_at, updated_at)
		VALUES ($1, 'Forecast Brand', $2, $3, 'active', now(), now())
	`, sellerID, "fc-seller-"+sellerID.String()[:8], "fc-"+sellerID.String()[:8]+"@test.com")
	require.NoError(t, err)

	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO seller_users (id, seller_id, user_id, role, created_at)
		VALUES ($1, $2, $3, 'owner', now())
	`, uuid.New(), sellerID, sellerOwnerID)
	require.NoError(t, err)

	// Customer
	customerID := uuid.New()
	createdUserIDs = append(createdUserIDs, customerID)
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO users (id, email, phone, name, password_hash, role, status, created_at, updated_at)
		VALUES ($1, $2, $3, 'Forecast Customer', 'hash', 'customer', 'active', now(), now())
	`, customerID, "cust-"+customerID.String()[:8]+"@test.com", "+7994"+customerID.String()[:7])
	require.NoError(t, err)

	// Category
	catID := uuid.New()
	createdCategoryIDs = append(createdCategoryIDs, catID)
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO categories (id, name, slug, is_active, created_at, updated_at)
		VALUES ($1, 'Forecast Category', $2, true, now(), now())
	`, catID, "fc-cat-"+catID.String()[:8])
	require.NoError(t, err)

	createProductAndVariant := func(title string, publishedAt, variantCreatedAt time.Time, color, size string) (uuid.UUID, uuid.UUID) {
		pID := uuid.New()
		vID := uuid.New()
		createdProductIDs = append(createdProductIDs, pID)
		createdVariantIDs = append(createdVariantIDs, vID)

		_, err := pgClient.Pool.Exec(ctx, `
			INSERT INTO products (
				id, seller_id, category_id, title, slug, price_cents, currency, status,
				published_at, created_at, updated_at
			) VALUES ($1, $2, $3, $4, $5, 250000, 'RUB', 'published', $6, $6, $6)
		`, pID, sellerID, catID, title, "slug-"+pID.String()[:8], publishedAt)
		require.NoError(t, err)

		_, err = pgClient.Pool.Exec(ctx, `
			INSERT INTO product_variants (
				id, product_id, sku, seller_sku, color, size, price_cents, is_active, created_at, updated_at
			) VALUES ($1, $2, $3, $3, $4, $5, 250000, true, $6, $6)
		`, vID, pID, "SKU-"+vID.String()[:8], color, size, variantCreatedAt)
		require.NoError(t, err)

		return pID, vID
	}

	setVariantStock := func(pID, vID uuid.UUID, totalStock, reservedStock int) {
		_, err := pgClient.Pool.Exec(ctx, `
			INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, now(), now())
			ON CONFLICT (product_variant_id) DO UPDATE SET total_stock = EXCLUDED.total_stock, reserved_stock = EXCLUDED.reserved_stock
		`, uuid.New(), pID, vID, sellerID, totalStock, reservedStock)
		require.NoError(t, err)
	}

	createOrderWithPayments := func(pID, vID uuid.UUID, qty int, orderTime time.Time, cancelledAt *time.Time, payments []paymentFixture) uuid.UUID {
		orderID := uuid.New()
		fulfillmentID := uuid.New()
		createdOrderIDs = append(createdOrderIDs, orderID)

		_, err := pgClient.Pool.Exec(ctx, `
			INSERT INTO orders (
				id, user_id, status, total_price_cents, currency, customer_name, customer_phone, customer_email, delivery_address, created_at, updated_at, cancelled_at
			) VALUES ($1, $2, 'paid', 250000, 'RUB', 'Buyer', '+79991112233', 'b@t.com', 'Moscow', $3, $3, $4)
		`, orderID, customerID, orderTime, cancelledAt)
		require.NoError(t, err)

		_, err = pgClient.Pool.Exec(ctx, `
			INSERT INTO order_fulfillments (
				id, order_id, seller_id, status, subtotal_cents, commission_bps, seller_amount_cents, created_at, updated_at
			) VALUES ($1, $2, $3, 'paid', 250000, 1500, 212500, $4, $4)
		`, fulfillmentID, orderID, sellerID, orderTime)
		require.NoError(t, err)

		_, err = pgClient.Pool.Exec(ctx, `
			INSERT INTO order_items (
				id, order_id, order_fulfillment_id, product_id, product_variant_id, seller_id, title, product_slug, price_cents, quantity, subtotal_price_cents, created_at
			) VALUES ($1, $2, $3, $4, $5, $6, 'Item', 'slug', 250000, $7, $7 * 250000, $8)
		`, uuid.New(), orderID, fulfillmentID, pID, vID, sellerID, qty, orderTime)
		require.NoError(t, err)

		for i, p := range payments {
			pID := uuid.New()
			_, err = pgClient.Pool.Exec(ctx, `
				INSERT INTO payments (
					id, order_id, provider, status, amount_cents, currency, idempotency_key, payment_number, payment_method, integration_mode, created_at, updated_at, paid_at, failed_at
				) VALUES ($1, $2, 'tbank', $3, 250000, 'RUB', $4, $5, 'card', 'mock', now(), now(), $6, $7)
			`, pID, orderID, p.status, fmt.Sprintf("idemp-%s-%d", orderID.String()[:8], i), fmt.Sprintf("PAY-%s-%d", orderID.String()[:8], i), p.paidAt, p.failedAt)
			require.NoError(t, err)
		}

		return orderID
	}

	getActiveAlert := func(pID uuid.UUID) *notifications.Notification {
		dedupeKey := products.StockForecastDedupeKey(pID)
		var n notifications.Notification
		var rawMeta []byte
		err := pgClient.Pool.QueryRow(ctx, `
			SELECT id, recipient_seller_id, recipient_kind, type, title, body, kind, severity, status, dedupe_key, action_url, metadata, read_at, resolved_at, created_at
			FROM notifications
			WHERE recipient_seller_id = $1 AND dedupe_key = $2 AND kind = 'alert' AND status = 'active'
		`, sellerID, dedupeKey).Scan(
			&n.ID, &n.RecipientSellerID, &n.RecipientKind, &n.Type, &n.Title, &n.Body, &n.Kind, &n.Severity, &n.Status, &n.DedupeKey, &n.ActionURL, &rawMeta, &n.ReadAt, &n.ResolvedAt, &n.CreatedAt,
		)
		if err != nil {
			return nil
		}
		_ = json.Unmarshal(rawMeta, &n.Metadata)
		return &n
	}

	now := time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)

	// -------------------------------------------------------------
	// 19.A: Payment Cardinality: Failed attempt + Successful attempt
	// -------------------------------------------------------------
	t.Run("A. Payment Cardinality - Failed attempt followed by success counts item qty exactly once", func(t *testing.T) {
		pID, vID := createProductAndVariant("Кардиналити Товар", now.AddDate(0, 0, -30), now.AddDate(0, 0, -30), "Черный", "M")
		setVariantStock(pID, vID, 2, 0) // free = 2

		orderTime := now.AddDate(0, 0, -10)
		failedTime := now.AddDate(0, 0, -10)
		paidTime := now.AddDate(0, 0, -10)

		// 1 order with 2 payments: 1 failed, 1 succeeded (quantity = 6 units)
		// 6 units in 30 days -> DSV = 0.2. Free = 2 -> Cover = 10 -> Warning!
		createOrderWithPayments(pID, vID, 6, orderTime, nil, []paymentFixture{
			{status: "failed", failedAt: &failedTime},
			{status: "succeeded", paidAt: &paidTime},
		})

		err := productsService.ReconcileStockForecastForProduct(ctx, pID)
		require.NoError(t, err)

		alert := getActiveAlert(pID)
		require.NotNil(t, alert, "Expected warning alert")
		assert.Equal(t, notifications.SeverityWarning, alert.Severity)
		assert.Equal(t, notifications.TypeStockForecastRisk, alert.Type)
		assert.Equal(t, "/supplies/new", *alert.ActionURL)

		// Verify metadata
		variantsMeta, ok := alert.Metadata["variants"].([]interface{})
		require.True(t, ok)
		require.Len(t, variantsMeta, 1)
		v0 := variantsMeta[0].(map[string]interface{})
		assert.Equal(t, float64(6), v0["paidDemandUnits"])
		assert.InDelta(t, 10.0, v0["daysOfCover"].(float64), 0.1)
	})

	// -------------------------------------------------------------
	// 19.B: Duplicate Success Safety
	// -------------------------------------------------------------
	t.Run("B. Duplicate Success Safety - Multiple successful payments still count exactly once using FIRST paid_at", func(t *testing.T) {
		pID, vID := createProductAndVariant("Дубликат Товар", now.AddDate(0, 0, -30), now.AddDate(0, 0, -30), "Красный", "L")
		setVariantStock(pID, vID, 2, 0) // free = 2

		// 1 order with TWO successful payment rows (15d ago and 5d ago), qty = 6
		orderTime := now.AddDate(0, 0, -15)
		paid1 := now.AddDate(0, 0, -15)
		paid2 := now.AddDate(0, 0, -5)

		createOrderWithPayments(pID, vID, 6, orderTime, nil, []paymentFixture{
			{status: "succeeded", paidAt: &paid1},
			{status: "succeeded", paidAt: &paid2},
		})

		err := productsService.ReconcileStockForecastForProduct(ctx, pID)
		require.NoError(t, err)

		alert := getActiveAlert(pID)
		require.NotNil(t, alert)
		variantsMeta := alert.Metadata["variants"].([]interface{})
		v0 := variantsMeta[0].(map[string]interface{})
		assert.Equal(t, float64(6), v0["paidDemandUnits"], "Item quantity must be counted exactly once despite 2 successful payments")

		// Second sub-test: first success outside 30d (40d ago), duplicate success inside 30d (5d ago)
		// -> Order must NOT re-enter current forecast window!
		pID2, vID2 := createProductAndVariant("Старый Успех Товар", now.AddDate(0, 0, -60), now.AddDate(0, 0, -60), "Зеленый", "S")
		setVariantStock(pID2, vID2, 2, 0)

		orderTimeOld := now.AddDate(0, 0, -40)
		paidOld := now.AddDate(0, 0, -40)
		paidNew := now.AddDate(0, 0, -5)

		createOrderWithPayments(pID2, vID2, 6, orderTimeOld, nil, []paymentFixture{
			{status: "succeeded", paidAt: &paidOld},
			{status: "succeeded", paidAt: &paidNew},
		})

		err = productsService.ReconcileStockForecastForProduct(ctx, pID2)
		require.NoError(t, err)

		alertOld := getActiveAlert(pID2)
		assert.Nil(t, alertOld, "Order whose first success was outside 30d must not re-enter forecast window due to duplicate payment")
	})

	// -------------------------------------------------------------
	// 19.C: Dead Variant: 0 paid units in 30d, free=0 -> no alert
	// -------------------------------------------------------------
	t.Run("C. Dead Variant - 0 paid units in 30d, free=0 does not alert", func(t *testing.T) {
		pID, vID := createProductAndVariant("Мертвый Товар", now.AddDate(0, 0, -30), now.AddDate(0, 0, -30), "Серый", "XL")
		setVariantStock(pID, vID, 0, 0) // free = 0

		err := productsService.ReconcileStockForecastForProduct(ctx, pID)
		require.NoError(t, err)

		alert := getActiveAlert(pID)
		assert.Nil(t, alert, "Dead variant with 0 sales must not create forecast alert even with free=0")
	})

	// -------------------------------------------------------------
	// 19.D & 19.E: Cold Start
	// -------------------------------------------------------------
	t.Run("D & E. Cold Start - 1 sale in 2 days does not alert; 2 sales in 2 days creates critical alert", func(t *testing.T) {
		// D: 1 sale
		pID, vID := createProductAndVariant("Cold Start 1", now.AddDate(0, 0, -2), now.AddDate(0, 0, -2), "Белый", "S")
		setVariantStock(pID, vID, 2, 0)
		paidTime := now.AddDate(0, 0, -1)
		createOrderWithPayments(pID, vID, 1, paidTime, nil, []paymentFixture{{status: "succeeded", paidAt: &paidTime}})

		err := productsService.ReconcileStockForecastForProduct(ctx, pID)
		require.NoError(t, err)
		assert.Nil(t, getActiveAlert(pID), "1 sale in 2 days does not meet minimum demand evidence")

		// E: 2 sales
		pID2, vID2 := createProductAndVariant("Cold Start 2", now.AddDate(0, 0, -2), now.AddDate(0, 0, -2), "Белый", "M")
		setVariantStock(pID2, vID2, 2, 0) // free = 2
		paidTime2 := now.AddDate(0, 0, -1)
		createOrderWithPayments(pID2, vID2, 2, paidTime2, nil, []paymentFixture{{status: "succeeded", paidAt: &paidTime2}})

		err = productsService.ReconcileStockForecastForProduct(ctx, pID2)
		require.NoError(t, err)
		alert := getActiveAlert(pID2)
		require.NotNil(t, alert, "2 sales in 2 days meets evidence and triggers critical alert (cover=2d)")
		assert.Equal(t, notifications.SeverityCritical, alert.Severity)
		assert.Equal(t, "Товар может закончиться в ближайшие дни", alert.Title)
	})

	// -------------------------------------------------------------
	// 19.F: Slow Seller (2 units / 30d, free=1 -> cover=15 -> no new alert)
	// -------------------------------------------------------------
	t.Run("F. Slow Seller - 2 units / 30 observed days, free=1, cover=15 does not create new alert", func(t *testing.T) {
		pID, vID := createProductAndVariant("Slow Seller Product", now.AddDate(0, 0, -30), now.AddDate(0, 0, -30), "Хаки", "L")
		setVariantStock(pID, vID, 1, 0) // free = 1
		paidTime := now.AddDate(0, 0, -10)
		createOrderWithPayments(pID, vID, 2, paidTime, nil, []paymentFixture{{status: "succeeded", paidAt: &paidTime}})

		err := productsService.ReconcileStockForecastForProduct(ctx, pID)
		require.NoError(t, err)
		assert.Nil(t, getActiveAlert(pID), "Cover=15 days (>14) must not create new alert")
	})

	// -------------------------------------------------------------
	// 19.G: Medium Seller (6 units / 30d, free=2 -> cover=10 -> warning)
	// -------------------------------------------------------------
	t.Run("G. Medium Seller - 6 units / 30d, free=2, cover=10 creates warning alert", func(t *testing.T) {
		pID, vID := createProductAndVariant("Medium Seller Product", now.AddDate(0, 0, -30), now.AddDate(0, 0, -30), "Оранжевый", "M")
		setVariantStock(pID, vID, 2, 0) // free = 2
		paidTime := now.AddDate(0, 0, -10)
		createOrderWithPayments(pID, vID, 6, paidTime, nil, []paymentFixture{{status: "succeeded", paidAt: &paidTime}})

		err := productsService.ReconcileStockForecastForProduct(ctx, pID)
		require.NoError(t, err)
		alert := getActiveAlert(pID)
		require.NotNil(t, alert)
		assert.Equal(t, notifications.SeverityWarning, alert.Severity)
		assert.Equal(t, "Запас товара скоро закончится", alert.Title)
	})

	// -------------------------------------------------------------
	// 19.H: Fast Seller (20 units / 30d, free=5 -> cover=7.5 -> warning)
	// -------------------------------------------------------------
	t.Run("H. Fast Seller - 20 units / 30d, free=5, cover=7.5 creates warning alert", func(t *testing.T) {
		pID, vID := createProductAndVariant("Fast Seller Product", now.AddDate(0, 0, -30), now.AddDate(0, 0, -30), "Фиолетовый", "S")
		setVariantStock(pID, vID, 5, 0) // free = 5
		paidTime := now.AddDate(0, 0, -10)
		createOrderWithPayments(pID, vID, 20, paidTime, nil, []paymentFixture{{status: "succeeded", paidAt: &paidTime}})

		err := productsService.ReconcileStockForecastForProduct(ctx, pID)
		require.NoError(t, err)
		alert := getActiveAlert(pID)
		require.NotNil(t, alert)
		assert.Equal(t, notifications.SeverityWarning, alert.Severity)
	})

	// -------------------------------------------------------------
	// 19.I: Critical (positive velocity with cover <= 7 -> critical)
	// -------------------------------------------------------------
	t.Run("I. Critical - Positive velocity with cover <= 7 creates critical alert", func(t *testing.T) {
		pID, vID := createProductAndVariant("Critical Seller Product", now.AddDate(0, 0, -30), now.AddDate(0, 0, -30), "Бордовый", "XL")
		setVariantStock(pID, vID, 2, 0) // free = 2
		paidTime := now.AddDate(0, 0, -10)
		createOrderWithPayments(pID, vID, 10, paidTime, nil, []paymentFixture{{status: "succeeded", paidAt: &paidTime}}) // DSV = 10/30 = 0.333, Cover = 2/0.333 = 6 days <= 7

		err := productsService.ReconcileStockForecastForProduct(ctx, pID)
		require.NoError(t, err)
		alert := getActiveAlert(pID)
		require.NotNil(t, alert)
		assert.Equal(t, notifications.SeverityCritical, alert.Severity)
		assert.Equal(t, "Товар может закончиться в ближайшие дни", alert.Title)
	})

	// -------------------------------------------------------------
	// 19.J: Hysteresis (13 -> 16 -> 18+)
	// -------------------------------------------------------------
	t.Run("J. Hysteresis - Active warning stays active at cover=16 and resolves at cover>=18", func(t *testing.T) {
		pID, vID := createProductAndVariant("Гистерезис Товар", now.AddDate(0, 0, -30), now.AddDate(0, 0, -30), "Синий", "L")
		// 30 sales in 30 days -> DSV = 1.0 unit/day
		orderTime := now.AddDate(0, 0, -10)
		paidTime := now.AddDate(0, 0, -10)
		createOrderWithPayments(pID, vID, 30, orderTime, nil, []paymentFixture{{status: "succeeded", paidAt: &paidTime}})

		// Free = 13 -> cover = 13 <= 14 -> triggers warning alert
		setVariantStock(pID, vID, 13, 0)
		err := productsService.ReconcileStockForecastForProduct(ctx, pID)
		require.NoError(t, err)

		alert1 := getActiveAlert(pID)
		require.NotNil(t, alert1)
		assert.Equal(t, notifications.SeverityWarning, alert1.Severity)
		firstAlertID := alert1.ID

		// Re-run with Free = 16 -> cover = 16 (between 14 and 18).
		// Because it was already participating in active alert, it must STAY active on the SAME alert ID!
		setVariantStock(pID, vID, 16, 0)
		err = productsService.ReconcileStockForecastForProduct(ctx, pID)
		require.NoError(t, err)

		alert2 := getActiveAlert(pID)
		require.NotNil(t, alert2, "Alert must remain active at cover=16 due to hysteresis (<18)")
		assert.Equal(t, firstAlertID, alert2.ID, "Must be the exact same alert ID (no flapping)")

		// Re-run with Free = 19 -> cover = 19 (>= 18) -> must resolve!
		setVariantStock(pID, vID, 19, 0)
		err = productsService.ReconcileStockForecastForProduct(ctx, pID)
		require.NoError(t, err)

		alert3 := getActiveAlert(pID)
		assert.Nil(t, alert3, "Alert must resolve when cover reaches >= 18")
	})

	// -------------------------------------------------------------
	// 19.K: Severity Update (warning -> critical on same alert ID)
	// -------------------------------------------------------------
	t.Run("K. Severity Update - Warning alert updates to critical on same alert ID when cover <= 7", func(t *testing.T) {
		pID, vID := createProductAndVariant("Обновление Серьезности", now.AddDate(0, 0, -30), now.AddDate(0, 0, -30), "Желтый", "S")
		paidTime := now.AddDate(0, 0, -10)
		createOrderWithPayments(pID, vID, 30, paidTime, nil, []paymentFixture{{status: "succeeded", paidAt: &paidTime}}) // 1/day

		// Free = 10 -> Warning
		setVariantStock(pID, vID, 10, 0)
		err := productsService.ReconcileStockForecastForProduct(ctx, pID)
		require.NoError(t, err)

		alert1 := getActiveAlert(pID)
		require.NotNil(t, alert1)
		assert.Equal(t, notifications.SeverityWarning, alert1.Severity)
		alertID := alert1.ID

		// Free drops to 5 -> cover = 5 <= 7 -> Critical
		setVariantStock(pID, vID, 5, 0)
		err = productsService.ReconcileStockForecastForProduct(ctx, pID)
		require.NoError(t, err)

		alert2 := getActiveAlert(pID)
		require.NotNil(t, alert2)
		assert.Equal(t, alertID, alert2.ID, "Same alert ID must be updated")
		assert.Equal(t, notifications.SeverityCritical, alert2.Severity, "Severity must update to critical")
		assert.Equal(t, "Товар может закончиться в ближайшие дни", alert2.Title)
	})

	// -------------------------------------------------------------
	// 19.L & 19.M: Multi-Variant Product & Partial Recovery
	// -------------------------------------------------------------
	t.Run("L & M. Multi-Variant Product & Partial Recovery", func(t *testing.T) {
		pID, vID1 := createProductAndVariant("Многовариантный Товар", now.AddDate(0, 0, -30), now.AddDate(0, 0, -30), "Красный", "L")
		// Add second variant to same product
		vID2 := uuid.New()
		createdVariantIDs = append(createdVariantIDs, vID2)
		_, err := pgClient.Pool.Exec(ctx, `
			INSERT INTO product_variants (id, product_id, sku, seller_sku, color, size, price_cents, is_active, created_at, updated_at)
			VALUES ($1, $2, $3, $3, 'Белый', 'M', 250000, true, $4, $4)
		`, vID2, pID, "SKU-"+vID2.String()[:8], now.AddDate(0, 0, -30))
		require.NoError(t, err)

		// Both sell 30 units in 30 days -> DSV = 1.0/day
		paidTime := now.AddDate(0, 0, -10)
		createOrderWithPayments(pID, vID1, 30, paidTime, nil, []paymentFixture{{status: "succeeded", paidAt: &paidTime}})
		createOrderWithPayments(pID, vID2, 30, paidTime, nil, []paymentFixture{{status: "succeeded", paidAt: &paidTime}})

		// Var1: Free = 4 -> Critical (cover = 4)
		// Var2: Free = 9 -> Warning (cover = 9)
		setVariantStock(pID, vID1, 4, 0)
		setVariantStock(pID, vID2, 9, 0)

		err = productsService.ReconcileStockForecastForProduct(ctx, pID)
		require.NoError(t, err)

		alert := getActiveAlert(pID)
		require.NotNil(t, alert, "Expected active product alert")
		assert.Equal(t, notifications.SeverityCritical, alert.Severity, "Product alert severity must equal worst variant severity")
		assert.Equal(t, float64(2), alert.Metadata["variantCount"])
		assert.Contains(t, alert.Body, "Красный / L — примерно на 4 дня")
		assert.Contains(t, alert.Body, "Белый / M — примерно на 9 дней")

		// 19.M: Partial recovery: Var1 receives stock, free becomes 20 (>=18)
		// Var2 remains at free = 9
		setVariantStock(pID, vID1, 20, 0)
		err = productsService.ReconcileStockForecastForProduct(ctx, pID)
		require.NoError(t, err)

		alert2 := getActiveAlert(pID)
		require.NotNil(t, alert2, "Alert must remain active while Var2 is at risk")
		assert.Equal(t, alert.ID, alert2.ID, "Same alert ID must remain active")
		assert.Equal(t, notifications.SeverityWarning, alert2.Severity, "Severity should downgrade to warning since Var1 recovered")
		assert.Equal(t, float64(1), alert2.Metadata["variantCount"], "Var1 must be removed from metadata")

		// Second variant recovers: Free = 20 (>=18)
		setVariantStock(pID, vID2, 20, 0)
		err = productsService.ReconcileStockForecastForProduct(ctx, pID)
		require.NoError(t, err)

		alert3 := getActiveAlert(pID)
		assert.Nil(t, alert3, "Product alert must resolve when all variants recover")
	})

	// -------------------------------------------------------------
	// 19.N: Recurrence
	// -------------------------------------------------------------
	t.Run("N. Recurrence - New alert ID is generated when a resolved alert re-crosses threshold", func(t *testing.T) {
		pID, vID := createProductAndVariant("Повторяющийся Товар", now.AddDate(0, 0, -30), now.AddDate(0, 0, -30), "Черный", "S")
		paidTime := now.AddDate(0, 0, -10)
		createOrderWithPayments(pID, vID, 30, paidTime, nil, []paymentFixture{{status: "succeeded", paidAt: &paidTime}})

		// 1. Trigger alert
		setVariantStock(pID, vID, 5, 0)
		err := productsService.ReconcileStockForecastForProduct(ctx, pID)
		require.NoError(t, err)
		alert1 := getActiveAlert(pID)
		require.NotNil(t, alert1)
		firstID := alert1.ID

		// 2. Resolve alert via stock increase
		setVariantStock(pID, vID, 25, 0)
		err = productsService.ReconcileStockForecastForProduct(ctx, pID)
		require.NoError(t, err)
		assert.Nil(t, getActiveAlert(pID))

		// 3. Stock drops again to 5 -> new alert ID!
		setVariantStock(pID, vID, 5, 0)
		err = productsService.ReconcileStockForecastForProduct(ctx, pID)
		require.NoError(t, err)
		alert2 := getActiveAlert(pID)
		require.NotNil(t, alert2)
		assert.NotEqual(t, firstID, alert2.ID, "Recurring alert must have a NEW notification ID")
		assert.Equal(t, *alert1.DedupeKey, *alert2.DedupeKey, "Dedupe key must be identical")
	})

	// -------------------------------------------------------------
	// 19.O: Ineligibility
	// -------------------------------------------------------------
	t.Run("O. Ineligibility - Resolves when product or variant becomes ineligible", func(t *testing.T) {
		pID, vID := createProductAndVariant("Неподходящий Товар", now.AddDate(0, 0, -30), now.AddDate(0, 0, -30), "Розовый", "M")
		paidTime := now.AddDate(0, 0, -10)
		createOrderWithPayments(pID, vID, 30, paidTime, nil, []paymentFixture{{status: "succeeded", paidAt: &paidTime}})
		setVariantStock(pID, vID, 5, 0)

		err := productsService.ReconcileStockForecastForProduct(ctx, pID)
		require.NoError(t, err)
		require.NotNil(t, getActiveAlert(pID))

		// Subcase 1: Variant becomes inactive -> resolves alert
		_, err = pgClient.Pool.Exec(ctx, "UPDATE product_variants SET is_active = false WHERE id = $1", vID)
		require.NoError(t, err)

		err = productsService.ReconcileStockForecastForProduct(ctx, pID)
		require.NoError(t, err)
		assert.Nil(t, getActiveAlert(pID), "Inactive variant must cause alert to resolve")

		// Reactivate variant -> alert re-appears
		_, err = pgClient.Pool.Exec(ctx, "UPDATE product_variants SET is_active = true WHERE id = $1", vID)
		require.NoError(t, err)
		err = productsService.ReconcileStockForecastForProduct(ctx, pID)
		require.NoError(t, err)
		require.NotNil(t, getActiveAlert(pID))

		// Subcase 2: Product un-published -> resolves alert
		_, err = pgClient.Pool.Exec(ctx, "UPDATE products SET status = 'draft' WHERE id = $1", pID)
		require.NoError(t, err)

		err = productsService.ReconcileStockForecastForProduct(ctx, pID)
		require.NoError(t, err)
		assert.Nil(t, getActiveAlert(pID), "Draft product must cause alert to resolve")
	})
}
