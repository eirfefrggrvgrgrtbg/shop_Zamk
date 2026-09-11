package router_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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
				_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM variant_stock_forecasts WHERE product_variant_id = ANY($1)", createdVariantIDs)
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

func TestNTF3_ForecastSnapshotPersistence(t *testing.T) {
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
				_, _ = pgClient.Pool.Exec(ctx, "DELETE FROM variant_stock_forecasts WHERE product_variant_id = ANY($1)", createdVariantIDs)
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

	sellerOwnerID := uuid.New()
	createdUserIDs = append(createdUserIDs, sellerOwnerID)
	_, err := pgClient.Pool.Exec(ctx, `
		INSERT INTO users (id, email, phone, name, password_hash, role, status, created_at, updated_at)
		VALUES ($1, $2, $3, 'Forecast Snap Owner', 'hash', 'seller', 'active', now(), now())
	`, sellerOwnerID, "snap-owner-"+sellerOwnerID.String()[:8]+"@test.com", "+7996"+sellerOwnerID.String()[:7])
	require.NoError(t, err)

	sellerID := uuid.New()
	createdSellerIDs = append(createdSellerIDs, sellerID)
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO sellers (id, brand_name, slug, contact_email, status, created_at, updated_at)
		VALUES ($1, 'Forecast Snap Brand', $2, $3, 'active', now(), now())
	`, sellerID, "snap-seller-"+sellerID.String()[:8], "snap-"+sellerID.String()[:8]+"@test.com")
	require.NoError(t, err)

	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO seller_users (id, seller_id, user_id, role, created_at)
		VALUES ($1, $2, $3, 'owner', now())
	`, uuid.New(), sellerID, sellerOwnerID)
	require.NoError(t, err)

	customerID := uuid.New()
	createdUserIDs = append(createdUserIDs, customerID)
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO users (id, email, phone, name, password_hash, role, status, created_at, updated_at)
		VALUES ($1, $2, $3, 'Snap Customer', 'hash', 'customer', 'active', now(), now())
	`, customerID, "snap-cust-"+customerID.String()[:8]+"@test.com", "+7997"+customerID.String()[:7])
	require.NoError(t, err)

	catID := uuid.New()
	createdCategoryIDs = append(createdCategoryIDs, catID)
	_, err = pgClient.Pool.Exec(ctx, `
		INSERT INTO categories (id, name, slug, is_active, created_at, updated_at)
		VALUES ($1, 'Snap Category', $2, true, now(), now())
	`, catID, "snap-cat-"+catID.String()[:8])
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

	createOrderWithPaidUnits := func(pID, vID uuid.UUID, qty int, paidTime time.Time) uuid.UUID {
		orderID := uuid.New()
		fulfillmentID := uuid.New()
		createdOrderIDs = append(createdOrderIDs, orderID)

		_, err := pgClient.Pool.Exec(ctx, `
			INSERT INTO orders (
				id, user_id, status, total_price_cents, currency, customer_name, customer_phone, customer_email, delivery_address, created_at, updated_at
			) VALUES ($1, $2, 'paid', 250000, 'RUB', 'Buyer', '+79991112233', 'b@t.com', 'Moscow', $3, $3)
		`, orderID, customerID, paidTime)
		require.NoError(t, err)

		_, err = pgClient.Pool.Exec(ctx, `
			INSERT INTO order_fulfillments (
				id, order_id, seller_id, status, subtotal_cents, commission_bps, seller_amount_cents, created_at, updated_at
			) VALUES ($1, $2, $3, 'paid', 250000, 1500, 212500, $4, $4)
		`, fulfillmentID, orderID, sellerID, paidTime)
		require.NoError(t, err)

		_, err = pgClient.Pool.Exec(ctx, `
			INSERT INTO order_items (
				id, order_id, order_fulfillment_id, product_id, product_variant_id, seller_id, title, product_slug, price_cents, quantity, subtotal_price_cents, created_at
			) VALUES ($1, $2, $3, $4, $5, $6, 'Item', 'slug', 250000, $7, $7 * 250000, $8)
		`, uuid.New(), orderID, fulfillmentID, pID, vID, sellerID, qty, paidTime)
		require.NoError(t, err)

		pPayID := uuid.New()
		_, err = pgClient.Pool.Exec(ctx, `
			INSERT INTO payments (
				id, order_id, provider, status, amount_cents, currency, idempotency_key, payment_number, payment_method, integration_mode, created_at, updated_at, paid_at
			) VALUES ($1, $2, 'tbank', 'succeeded', 250000, 'RUB', $3, $4, 'card', 'mock', $5, $5, $5)
		`, pPayID, orderID, fmt.Sprintf("idem-%s", pPayID.String()[:8]), fmt.Sprintf("PAY-%s", pPayID.String()[:8]), paidTime)
		require.NoError(t, err)

		return orderID
	}

	type snapshotRow struct {
		VariantID    uuid.UUID
		State        string
		DaysOfCover  *float64
		CalculatedAt time.Time
	}

	getSnapshot := func(vID uuid.UUID) *snapshotRow {
		var row snapshotRow
		err := pgClient.Pool.QueryRow(ctx, `
			SELECT product_variant_id, state, days_of_cover, calculated_at
			FROM variant_stock_forecasts
			WHERE product_variant_id = $1
		`, vID).Scan(&row.VariantID, &row.State, &row.DaysOfCover, &row.CalculatedAt)
		if err != nil {
			return nil
		}
		return &row
	}

	now := time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)

	// -------------------------------------------------------------
	// 1. Snapshot Persistence Integration (A-G)
	// -------------------------------------------------------------
	t.Run("A. Insufficient data variant -> snapshot state = insufficient_data, cover = NULL", func(t *testing.T) {
		pID, vID := createProductAndVariant("Мало данных товар", now.AddDate(0, 0, -2), now.AddDate(0, 0, -2), "Белый", "S")
		setVariantStock(pID, vID, 10, 0)
		createOrderWithPaidUnits(pID, vID, 1, now.AddDate(0, 0, -1)) // paidDemandUnits < 2 and observedDays < 7

		err := productsService.ReconcileStockForecastForProduct(ctx, pID)
		require.NoError(t, err)

		snap := getSnapshot(vID)
		require.NotNil(t, snap)
		assert.Equal(t, "insufficient_data", snap.State)
		assert.Nil(t, snap.DaysOfCover)
	})

	t.Run("B. Zero demand with sufficient observation -> snapshot state = no_sales, cover = NULL", func(t *testing.T) {
		pID, vID := createProductAndVariant("Нет продаж товар", now.AddDate(0, 0, -30), now.AddDate(0, 0, -30), "Черный", "M")
		setVariantStock(pID, vID, 10, 0)
		// 0 orders

		err := productsService.ReconcileStockForecastForProduct(ctx, pID)
		require.NoError(t, err)

		snap := getSnapshot(vID)
		require.NotNil(t, snap)
		assert.Equal(t, "no_sales", snap.State)
		assert.Nil(t, snap.DaysOfCover)
	})

	t.Run("C. Healthy calculated -> snapshot state = healthy, cover persisted", func(t *testing.T) {
		pID, vID := createProductAndVariant("Здоровый товар", now.AddDate(0, 0, -30), now.AddDate(0, 0, -30), "Синий", "L")
		setVariantStock(pID, vID, 50, 0)
		createOrderWithPaidUnits(pID, vID, 3, now.AddDate(0, 0, -10)) // DSV = 0.1, cover = 500

		err := productsService.ReconcileStockForecastForProduct(ctx, pID)
		require.NoError(t, err)

		snap := getSnapshot(vID)
		require.NotNil(t, snap)
		assert.Equal(t, "healthy", snap.State)
		require.NotNil(t, snap.DaysOfCover)
		assert.InDelta(t, 500.0, *snap.DaysOfCover, 0.01)
	})

	t.Run("D. Warning -> snapshot state = warning, cover persisted", func(t *testing.T) {
		pID, vID := createProductAndVariant("Предупреждение товар", now.AddDate(0, 0, -30), now.AddDate(0, 0, -30), "Желтый", "XL")
		setVariantStock(pID, vID, 2, 0)
		createOrderWithPaidUnits(pID, vID, 6, now.AddDate(0, 0, -10)) // DSV = 0.2, cover = 10 (<=14)

		err := productsService.ReconcileStockForecastForProduct(ctx, pID)
		require.NoError(t, err)

		snap := getSnapshot(vID)
		require.NotNil(t, snap)
		assert.Equal(t, "warning", snap.State)
		require.NotNil(t, snap.DaysOfCover)
		assert.InDelta(t, 10.0, *snap.DaysOfCover, 0.01)
	})

	t.Run("E. Critical -> snapshot state = critical, cover persisted", func(t *testing.T) {
		pID, vID := createProductAndVariant("Критический товар", now.AddDate(0, 0, -30), now.AddDate(0, 0, -30), "Красный", "XXL")
		setVariantStock(pID, vID, 2, 0)
		createOrderWithPaidUnits(pID, vID, 10, now.AddDate(0, 0, -10)) // DSV = 10/30, cover = 6 (<=7)

		err := productsService.ReconcileStockForecastForProduct(ctx, pID)
		require.NoError(t, err)

		snap := getSnapshot(vID)
		require.NotNil(t, snap)
		assert.Equal(t, "critical", snap.State)
		require.NotNil(t, snap.DaysOfCover)
		assert.InDelta(t, 6.0, *snap.DaysOfCover, 0.01)
	})

	t.Run("F. Reconciled again -> exactly one row updated, not duplicated", func(t *testing.T) {
		pID, vID := createProductAndVariant("Повторный товар", now.AddDate(0, 0, -30), now.AddDate(0, 0, -30), "Серый", "M")
		setVariantStock(pID, vID, 2, 0)
		createOrderWithPaidUnits(pID, vID, 6, now.AddDate(0, 0, -10))

		err := productsService.ReconcileStockForecastForProduct(ctx, pID)
		require.NoError(t, err)

		var count int
		err = pgClient.Pool.QueryRow(ctx, "SELECT count(*) FROM variant_stock_forecasts WHERE product_variant_id = $1", vID).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 1, count)

		// Reconcile again with new stock
		setVariantStock(pID, vID, 50, 0)
		err = productsService.ReconcileStockForecastForProduct(ctx, pID)
		require.NoError(t, err)

		err = pgClient.Pool.QueryRow(ctx, "SELECT count(*) FROM variant_stock_forecasts WHERE product_variant_id = $1", vID).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 1, count, "Must remain exactly 1 row after update")

		snap := getSnapshot(vID)
		require.NotNil(t, snap)
		assert.Equal(t, "healthy", snap.State)
	})

	t.Run("G. Hysteresis: warning cover 13 -> 16 remains warning; cover >= 18 becomes healthy", func(t *testing.T) {
		pID, vID := createProductAndVariant("Гистерезис товар", now.AddDate(0, 0, -10), now.AddDate(0, 0, -10), "Зеленый", "S")
		// 10 units in 10 days -> DSV = 1.0/day
		createOrderWithPaidUnits(pID, vID, 10, now.AddDate(0, 0, -5))

		// 1. Cover = 10 -> enters warning
		setVariantStock(pID, vID, 10, 0)
		err := productsService.ReconcileStockForecastForProduct(ctx, pID)
		require.NoError(t, err)
		snap := getSnapshot(vID)
		require.NotNil(t, snap)
		assert.Equal(t, "warning", snap.State)

		// 2. Cover becomes 16 (between 14 and 18): under hysteresis, remains warning!
		setVariantStock(pID, vID, 16, 0)
		err = productsService.ReconcileStockForecastForProduct(ctx, pID)
		require.NoError(t, err)
		snap = getSnapshot(vID)
		require.NotNil(t, snap)
		assert.Equal(t, "warning", snap.State, "Cover 16 under hysteresis must remain warning")
		require.NotNil(t, snap.DaysOfCover)
		assert.InDelta(t, 16.0, *snap.DaysOfCover, 0.01)

		// 3. Cover becomes 18 (>= 18): resolves hysteresis -> becomes healthy!
		setVariantStock(pID, vID, 18, 0)
		err = productsService.ReconcileStockForecastForProduct(ctx, pID)
		require.NoError(t, err)
		snap = getSnapshot(vID)
		require.NotNil(t, snap)
		assert.Equal(t, "healthy", snap.State, "Cover 18 must resolve to healthy")
		require.NotNil(t, snap.DaysOfCover)
		assert.InDelta(t, 18.0, *snap.DaysOfCover, 0.01)
	})

	// -------------------------------------------------------------
	// 2. Product-Scoped Reconciliation Safety (Unrelated Products Untouched)
	// -------------------------------------------------------------
	t.Run("H. Product-scoped reconcile never deletes snapshots of other products", func(t *testing.T) {
		pID1, vID1 := createProductAndVariant("Товар 1", now.AddDate(0, 0, -30), now.AddDate(0, 0, -30), "Белый", "S")
		setVariantStock(pID1, vID1, 20, 0)
		createOrderWithPaidUnits(pID1, vID1, 3, now.AddDate(0, 0, -10))

		pID2, vID2 := createProductAndVariant("Товар 2 Несвязанный", now.AddDate(0, 0, -30), now.AddDate(0, 0, -30), "Черный", "M")
		setVariantStock(pID2, vID2, 20, 0)
		createOrderWithPaidUnits(pID2, vID2, 3, now.AddDate(0, 0, -10))

		// Reconcile Product 2 first
		err := productsService.ReconcileStockForecastForProduct(ctx, pID2)
		require.NoError(t, err)
		require.NotNil(t, getSnapshot(vID2), "Product 2 must have a snapshot")

		// Reconcile Product 1
		err = productsService.ReconcileStockForecastForProduct(ctx, pID1)
		require.NoError(t, err)
		require.NotNil(t, getSnapshot(vID1), "Product 1 must have a snapshot")

		// CRITICAL ASSERTION: Product 2 snapshot MUST still exist!
		require.NotNil(t, getSnapshot(vID2), "Product-scoped reconcile of Product 1 must NOT delete Product 2 snapshot!")
	})

	// -------------------------------------------------------------
	// 3. Failure Preserves Last Valid Snapshot
	// -------------------------------------------------------------
	t.Run("I. Failure preserves last valid snapshot on transaction rollback", func(t *testing.T) {
		pID, vID := createProductAndVariant("Откат Товар", now.AddDate(0, 0, -30), now.AddDate(0, 0, -30), "Фиолетовый", "S")
		setVariantStock(pID, vID, 2, 0)
		createOrderWithPaidUnits(pID, vID, 6, now.AddDate(0, 0, -10)) // warning, cover = 10

		// Establish valid existing snapshot
		err := productsService.ReconcileStockForecastForProduct(ctx, pID)
		require.NoError(t, err)
		initialSnap := getSnapshot(vID)
		require.NotNil(t, initialSnap)
		assert.Equal(t, "warning", initialSnap.State)
		initialCalcAt := initialSnap.CalculatedAt

		// Force failure inside a transaction attempting to refresh the snapshot to healthy
		simulatedErr := pgClient.RunInTx(ctx, func(tx pgx.Tx) error {
			// Update stock in transaction
			_, txErr := tx.Exec(ctx, "UPDATE inventory_items SET total_stock = 100 WHERE product_variant_id = $1", vID)
			if txErr != nil {
				return txErr
			}
			if err := productsService.ReconcileStockForecastForProductTx(ctx, tx, pID, now.Add(1*time.Hour)); err != nil {
				return err
			}
			return fmt.Errorf("simulated error forcing rollback")
		})
		require.Error(t, simulatedErr)

		// Assert: previous snapshot remains present and completely unchanged
		afterSnap := getSnapshot(vID)
		require.NotNil(t, afterSnap, "Snapshot must still exist after rollback")
		assert.Equal(t, "warning", afterSnap.State, "State must not have changed to healthy")
		assert.Equal(t, initialCalcAt, afterSnap.CalculatedAt, "CalculatedAt must remain from the original valid run")
	})

	// -------------------------------------------------------------
	// 4. Ineligibility Cleanup & Isolation
	// -------------------------------------------------------------
	t.Run("J. Ineligibility removes snapshot while preserving unrelated product snapshots", func(t *testing.T) {
		pIDTarget, vIDTarget := createProductAndVariant("Целевой Товар", now.AddDate(0, 0, -30), now.AddDate(0, 0, -30), "Оранжевый", "S")
		setVariantStock(pIDTarget, vIDTarget, 20, 0)
		createOrderWithPaidUnits(pIDTarget, vIDTarget, 3, now.AddDate(0, 0, -10))

		pIDOther, vIDOther := createProductAndVariant("Другой Товар", now.AddDate(0, 0, -30), now.AddDate(0, 0, -30), "Коричневый", "M")
		setVariantStock(pIDOther, vIDOther, 20, 0)
		createOrderWithPaidUnits(pIDOther, vIDOther, 3, now.AddDate(0, 0, -10))

		// Establish snapshots for both
		err := productsService.ReconcileStockForecastForProduct(ctx, pIDTarget)
		require.NoError(t, err)
		err = productsService.ReconcileStockForecastForProduct(ctx, pIDOther)
		require.NoError(t, err)
		require.NotNil(t, getSnapshot(vIDTarget))
		require.NotNil(t, getSnapshot(vIDOther))

		// 1. Variant becomes inactive -> its snapshot removed
		_, err = pgClient.Pool.Exec(ctx, "UPDATE product_variants SET is_active = false WHERE id = $1", vIDTarget)
		require.NoError(t, err)
		err = productsService.ReconcileStockForecastForProduct(ctx, pIDTarget)
		require.NoError(t, err)
		assert.Nil(t, getSnapshot(vIDTarget), "Inactive variant snapshot must be deleted")
		assert.NotNil(t, getSnapshot(vIDOther), "Unrelated variant snapshot must remain intact")

		// Reactivate variant -> snapshot restored
		_, err = pgClient.Pool.Exec(ctx, "UPDATE product_variants SET is_active = true WHERE id = $1", vIDTarget)
		require.NoError(t, err)
		err = productsService.ReconcileStockForecastForProduct(ctx, pIDTarget)
		require.NoError(t, err)
		assert.NotNil(t, getSnapshot(vIDTarget))

		// 2. Product becomes draft (non-published) -> snapshot removed
		_, err = pgClient.Pool.Exec(ctx, "UPDATE products SET status = 'draft' WHERE id = $1", pIDTarget)
		require.NoError(t, err)
		err = productsService.ReconcileStockForecastForProduct(ctx, pIDTarget)
		require.NoError(t, err)
		assert.Nil(t, getSnapshot(vIDTarget), "Draft product snapshot must be deleted")
		assert.NotNil(t, getSnapshot(vIDOther), "Unrelated variant snapshot must remain intact")

		// Reactivate product -> snapshot restored
		_, err = pgClient.Pool.Exec(ctx, "UPDATE products SET status = 'published' WHERE id = $1", pIDTarget)
		require.NoError(t, err)
		err = productsService.ReconcileStockForecastForProduct(ctx, pIDTarget)
		require.NoError(t, err)
		assert.NotNil(t, getSnapshot(vIDTarget))

		// 3. Seller becomes inactive -> snapshot removed
		_, err = pgClient.Pool.Exec(ctx, "UPDATE sellers SET status = 'blocked' WHERE id = $1", sellerID)
		require.NoError(t, err)
		err = productsService.ReconcileStockForecastForProduct(ctx, pIDTarget)
		require.NoError(t, err)
		assert.Nil(t, getSnapshot(vIDTarget), "Snapshot for non-active seller must be deleted")
	})
}

func TestNTF3_ForecastReconciliationAdvisoryLock(t *testing.T) {
	ctx, pgClient, productsService, _, cleanup := setupForecastTestEnvironment(t)
	defer cleanup()

	var dbName string
	err := pgClient.Pool.QueryRow(ctx, "SELECT current_database()").Scan(&dbName)
	require.NoError(t, err)
	require.Equal(t, "zamk_test", dbName, "tests must strictly run against zamk_test")

	connA, err := pgClient.Pool.Acquire(ctx)
	require.NoError(t, err)
	defer connA.Release()

	connB, err := pgClient.Pool.Acquire(ctx)
	require.NoError(t, err)
	defer connB.Release()

	// 1. Transaction A acquires the forecast advisory lock through reconciliation
	txA, err := connA.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = txA.Rollback(ctx) }()

	// Calling ReconcileStockForecastAlertsTx on txA acquires the lock
	err = productsService.ReconcileStockForecastAlertsTx(ctx, txA, time.Now().UTC())
	require.NoError(t, err)

	// 2. Transaction B verifies through non-blocking pg_try_advisory_xact_lock that
	// the lock CANNOT be acquired while Transaction A is open
	txB, err := connB.Begin(ctx)
	require.NoError(t, err)

	var canAcquire bool
	err = txB.QueryRow(ctx, `SELECT pg_try_advisory_xact_lock($1)`, products.ForecastReconciliationAdvisoryLockKey).Scan(&canAcquire)
	require.NoError(t, err)
	assert.False(t, canAcquire, "Transaction B must NOT be able to acquire forecast reconciliation advisory lock while Transaction A holds it")
	_ = txB.Rollback(ctx)

	// 3. Rollback Transaction A -> lock is released automatically
	err = txA.Rollback(ctx)
	require.NoError(t, err)

	// 4. Verify Transaction C on connB can now acquire the lock
	txC, err := connB.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = txC.Rollback(ctx) }()

	err = txC.QueryRow(ctx, `SELECT pg_try_advisory_xact_lock($1)`, products.ForecastReconciliationAdvisoryLockKey).Scan(&canAcquire)
	require.NoError(t, err)
	assert.True(t, canAcquire, "Transaction C must be able to acquire the forecast reconciliation advisory lock after Transaction A rolled back")
	_ = txC.Rollback(ctx)
}
