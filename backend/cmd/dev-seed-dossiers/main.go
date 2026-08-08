package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/auth"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/config"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
)

const demoDomain = "@demo.zamk.local"

// Deterministic UUIDs for demo sellers
var demoSellerIDs = []uuid.UUID{
	uuid.MustParse("d1111111-1111-4111-8111-111111111111"), // 1 Leader
	uuid.MustParse("d2222222-2222-4222-8222-222222222222"), // 2 Stable
	uuid.MustParse("d3333333-3333-4333-8333-333333333333"), // 3 Attention
	uuid.MustParse("d4444444-4444-4444-8444-444444444444"), // 4 Recovery
	uuid.MustParse("d5555555-5555-4555-8555-555555555555"), // 5 Stock Risk
	uuid.MustParse("d6666666-6666-4666-8666-666666666666"), // 6 Returns
	uuid.MustParse("d7777777-7777-4777-8777-777777777777"), // 7 Inactive
	uuid.MustParse("d8888888-8888-4888-8888-888888888888"), // 8 Finance
	uuid.MustParse("d9999999-9999-4999-8999-999999999999"), // 9 Edge 1 New
	uuid.MustParse("daaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"), // 10 Edge 2 Setup
}

func main() {
	resetFlag := flag.Bool("reset", false, "Reset and delete all demo sellers data")
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		logger.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	if strings.EqualFold(cfg.App.Env, "production") {
		logger.Error("dev seed-dossiers refused to run in production")
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	pgClient, err := postgres.NewClient(ctx, cfg.Postgres.DSN)
	if err != nil {
		logger.Error("failed to connect to postgres", "error", err)
		os.Exit(1)
	}
	defer pgClient.Close()

	if *resetFlag {
		logger.Info("resetting demo sellers data...")
		if err := resetDemoData(ctx, pgClient); err != nil {
			logger.Error("failed to reset demo data", "error", err)
			os.Exit(1)
		}
		logger.Info("successfully reset demo sellers data!")
		return
	}

	logger.Info("seeding demo sellers data...")
	if err := seedDemoData(ctx, pgClient); err != nil {
		logger.Error("failed to seed demo data", "error", err)
		os.Exit(1)
	}
	logger.Info("successfully seeded 10 demo sellers with dossiers!")
}

func resetDemoData(ctx context.Context, client *postgres.Client) error {
	return client.RunInTx(ctx, func(tx pgx.Tx) error {
		demoPattern := "%" + demoDomain

		steps := []struct {
			name  string
			query string
		}{
			{"ledger", `DELETE FROM seller_balance_ledger WHERE seller_id IN (SELECT id FROM sellers WHERE contact_email LIKE $1)`},
			{"payouts", `DELETE FROM payouts WHERE seller_id IN (SELECT id FROM sellers WHERE contact_email LIKE $1)`},
			{"plans", `DELETE FROM seller_improvement_plans WHERE seller_id IN (SELECT id FROM sellers WHERE contact_email LIKE $1)`},
			{"violations", `DELETE FROM seller_violations WHERE seller_id IN (SELECT id FROM sellers WHERE contact_email LIKE $1)`},
			{"warnings", `DELETE FROM seller_warnings WHERE seller_id IN (SELECT id FROM sellers WHERE contact_email LIKE $1)`},
			{"refunds", `DELETE FROM refunds WHERE order_id IN (SELECT id FROM orders WHERE customer_email LIKE $1 OR user_id IN (SELECT id FROM users WHERE email LIKE $1))`},
			{"return_items", `DELETE FROM return_items WHERE return_id IN (SELECT id FROM returns WHERE user_id IN (SELECT id FROM users WHERE email LIKE $1))`},
			{"returns", `DELETE FROM returns WHERE user_id IN (SELECT id FROM users WHERE email LIKE $1)`},
			{"reviews", `DELETE FROM product_reviews WHERE user_id IN (SELECT id FROM users WHERE email LIKE $1) OR seller_id IN (SELECT id FROM sellers WHERE contact_email LIKE $1)`},
			{"shipment_events", `DELETE FROM shipment_events WHERE shipment_id IN (SELECT id FROM shipments WHERE order_id IN (SELECT id FROM orders WHERE customer_email LIKE $1 OR user_id IN (SELECT id FROM users WHERE email LIKE $1)))`},
			{"shipments", `DELETE FROM shipments WHERE order_id IN (SELECT id FROM orders WHERE customer_email LIKE $1 OR user_id IN (SELECT id FROM users WHERE email LIKE $1))`},
			{"payments", `DELETE FROM payments WHERE order_id IN (SELECT id FROM orders WHERE customer_email LIKE $1 OR user_id IN (SELECT id FROM users WHERE email LIKE $1))`},
			{"order_items", `DELETE FROM order_items WHERE seller_id IN (SELECT id FROM sellers WHERE contact_email LIKE $1)`},
			{"order_fulfillments", `DELETE FROM order_fulfillments WHERE seller_id IN (SELECT id FROM sellers WHERE contact_email LIKE $1)`},
			{"orders", `DELETE FROM orders WHERE customer_email LIKE $1 OR user_id IN (SELECT id FROM users WHERE email LIKE $1)`},
			{"inventory_items", `DELETE FROM inventory_items WHERE seller_id IN (SELECT id FROM sellers WHERE contact_email LIKE $1)`},
			{"product_moderation_logs", `DELETE FROM product_moderation_logs WHERE product_id IN (SELECT id FROM products WHERE seller_id IN (SELECT id FROM sellers WHERE contact_email LIKE $1))`},
			{"product_variants", `DELETE FROM product_variants WHERE product_id IN (SELECT id FROM products WHERE seller_id IN (SELECT id FROM sellers WHERE contact_email LIKE $1))`},
			{"products", `DELETE FROM products WHERE seller_id IN (SELECT id FROM sellers WHERE contact_email LIKE $1)`},
			{"seller_users", `DELETE FROM seller_users WHERE seller_id IN (SELECT id FROM sellers WHERE contact_email LIKE $1)`},
			{"sellers", `DELETE FROM sellers WHERE contact_email LIKE $1`},
			{"users", `DELETE FROM users WHERE email LIKE $1`},
		}

		for _, s := range steps {
			if _, err := tx.Exec(ctx, s.query, demoPattern); err != nil {
				slog.Error("reset step failed", "step", s.name, "error", err)
			}
		}
		return nil
	})
}

func seedDemoData(ctx context.Context, client *postgres.Client) error {
	return client.RunInTx(ctx, func(tx pgx.Tx) error {
		// Ensure shared category and brand exist
		categoryID := uuid.MustParse("c1111111-1111-4111-8111-111111111111")
		brandID := uuid.MustParse("b1111111-1111-4111-8111-111111111111")

		_, _ = tx.Exec(ctx, `
			INSERT INTO categories (id, name, slug, created_at, updated_at)
			VALUES ($1, 'Электроника и Гаджеты', 'electronics', NOW(), NOW())
			ON CONFLICT (id) DO NOTHING
		`, categoryID)

		_, _ = tx.Exec(ctx, `
			INSERT INTO brands (id, name, slug, created_at, updated_at)
			VALUES ($1, 'ZAMK Certified', 'zamk-certified', NOW(), NOW())
			ON CONFLICT (id) DO NOTHING
		`, brandID)

		now := time.Now()

		// Create shared customer user
		customerUser := upsertUserHelper(ctx, tx, "33333333-3333-4333-8333-333333333333", "Иван Покупатель", "customer@demo.zamk.local")

		// -------------------------------------------------------------
		// SELLER 1 — ZAMK Demo Leader (Leader)
		// -------------------------------------------------------------
		s1User := upsertUserHelper(ctx, tx, "11111111-1111-4111-8111-111111111101", "Алексей Смирнов", "demo-seller-01@demo.zamk.local")
		s1 := upsertSellerHelper(ctx, tx, demoSellerIDs[0], "Алексей Смирнов", "demo-seller-01@demo.zamk.local", "ZAMK Demo Leader", "zamk-demo-leader", "active", s1User, now.Add(-180*24*time.Hour))
		seedSellerOperations(ctx, tx, s1, customerUser, categoryID, brandID, "Флагманский смарт-замок ZAMK Pro", 60, 85000000, 4.9, 45, 0, 0, false, "high", now)

		// -------------------------------------------------------------
		// SELLER 2 — ZAMK Demo Stable (Stable)
		// -------------------------------------------------------------
		s2User := upsertUserHelper(ctx, tx, "11111111-1111-4111-8111-111111111102", "Елена Васильева", "demo-seller-02@demo.zamk.local")
		s2 := upsertSellerHelper(ctx, tx, demoSellerIDs[1], "Елена Васильева", "demo-seller-02@demo.zamk.local", "ZAMK Demo Stable", "zamk-demo-stable", "active", s2User, now.Add(-120*24*time.Hour))
		seedSellerOperations(ctx, tx, s2, customerUser, categoryID, brandID, "Умная дверная ручка ZAMK Touch", 35, 42000000, 4.6, 18, 0, 0, false, "stable", now)

		// -------------------------------------------------------------
		// SELLER 3 — ZAMK Demo Attention (Needs Attention)
		// -------------------------------------------------------------
		s3User := upsertUserHelper(ctx, tx, "11111111-1111-4111-8111-111111111103", "Игорь Соколов", "demo-seller-03@demo.zamk.local")
		s3 := upsertSellerHelper(ctx, tx, demoSellerIDs[2], "Игорь Соколов", "demo-seller-03@demo.zamk.local", "ZAMK Demo Attention", "zamk-demo-attention", "active", s3User, now.Add(-90*24*time.Hour))
		seedSellerOperations(ctx, tx, s3, customerUser, categoryID, brandID, "Биометрический накладной замок", 25, 28000000, 4.1, 12, 1, 0, true, "needs_attention", now)
		addWarning(ctx, tx, s3, "assembly_delay", "Задержка сборки заказов", "Зафиксировано 3 случая превышения нормативного времени сборки", "high")
		addImprovementPlan(ctx, tx, s3, "active", "Высокий процент задержек сборки заказов", "Оптимизация работы склада и упаковки", now.Add(14*24*time.Hour))

		// -------------------------------------------------------------
		// SELLER 4 — ZAMK Demo Recovery (Low Performance)
		// -------------------------------------------------------------
		s4User := upsertUserHelper(ctx, tx, "11111111-1111-4111-8111-111111111104", "Дмитрий Орлов", "demo-seller-04@demo.zamk.local")
		s4 := upsertSellerHelper(ctx, tx, demoSellerIDs[3], "Дмитрий Орлов", "demo-seller-04@demo.zamk.local", "ZAMK Demo Recovery", "zamk-demo-recovery", "active", s4User, now.Add(-60*24*time.Hour))
		seedSellerOperations(ctx, tx, s4, customerUser, categoryID, brandID, "Кодовая панель доступа ZAMK Keypad", 15, 12000000, 3.4, 8, 2, 1, true, "low", now)
		addViolation(ctx, tx, s4, "wrong_item_shipped", "Пересортица при отправке", "Отправлена неподходящая модель замка покупателю", "high")
		addImprovementPlan(ctx, tx, s4, "active", "Ошибки комплектации и жалобы клиентов", "Внедрение сканирования штрихкодов перед отгрузкой", now.Add(7*24*time.Hour))

		// -------------------------------------------------------------
		// SELLER 5 — ZAMK Demo Stock Risk (Stock Out & Low Stock)
		// -------------------------------------------------------------
		s5User := upsertUserHelper(ctx, tx, "11111111-1111-4111-8111-111111111105", "Ольга Морозова", "demo-seller-05@demo.zamk.local")
		s5 := upsertSellerHelper(ctx, tx, demoSellerIDs[4], "Ольга Морозова", "demo-seller-05@demo.zamk.local", "ZAMK Demo Stock Risk", "zamk-demo-stock-risk", "active", s5User, now.Add(-100*24*time.Hour))
		seedSellerStockRisk(ctx, tx, s5, categoryID, brandID, now)

		// -------------------------------------------------------------
		// SELLER 6 — ZAMK Demo Returns (High Returns)
		// -------------------------------------------------------------
		s6User := upsertUserHelper(ctx, tx, "11111111-1111-4111-8111-111111111106", "Максим Романов", "demo-seller-06@demo.zamk.local")
		s6 := upsertSellerHelper(ctx, tx, demoSellerIDs[5], "Максим Романов", "demo-seller-06@demo.zamk.local", "ZAMK Demo Returns", "zamk-demo-returns", "active", s6User, now.Add(-110*24*time.Hour))
		seedSellerReturns(ctx, tx, s6, categoryID, brandID, now)

		// -------------------------------------------------------------
		// SELLER 7 — ZAMK Demo Inactive (No Recent Activity)
		// -------------------------------------------------------------
		s7User := upsertUserHelper(ctx, tx, "11111111-1111-4111-8111-111111111107", "Артем Петров", "demo-seller-07@demo.zamk.local")
		s7 := upsertSellerHelper(ctx, tx, demoSellerIDs[6], "Артем Петров", "demo-seller-07@demo.zamk.local", "ZAMK Demo Inactive", "zamk-demo-inactive", "active", s7User, now.Add(-200*24*time.Hour))
		seedSellerInactive(ctx, tx, s7, categoryID, brandID, now)

		// -------------------------------------------------------------
		// SELLER 8 — ZAMK Demo Finance (Payouts & Balance)
		// -------------------------------------------------------------
		s8User := upsertUserHelper(ctx, tx, "11111111-1111-4111-8111-111111111108", "Сергей Фёдоров", "demo-seller-08@demo.zamk.local")
		s8 := upsertSellerHelper(ctx, tx, demoSellerIDs[7], "Сергей Фёдоров", "demo-seller-08@demo.zamk.local", "ZAMK Demo Finance", "zamk-demo-finance", "active", s8User, now.Add(-150*24*time.Hour))
		seedSellerFinance(ctx, tx, s8, categoryID, brandID, now)

		// -------------------------------------------------------------
		// EDGE CASE 1 — New Seller (Invited, no store)
		// -------------------------------------------------------------
		sEdge1User := upsertUserHelper(ctx, tx, "11111111-1111-4111-8111-111111111109", "Новый Продавец", "demo-seller-edge-01@demo.zamk.local")
		_, _ = tx.Exec(ctx, `
			INSERT INTO sellers (id, brand_name, slug, description, contact_email, status, created_at, updated_at)
			VALUES ($1, 'Без названия', 'demo-edge-01', 'Продавец без настроенного магазина', 'demo-seller-edge-01@demo.zamk.local', 'pending', NOW(), NOW())
			ON CONFLICT (id) DO UPDATE SET status = 'pending'
		`, demoSellerIDs[8])
		_, _ = tx.Exec(ctx, `
			INSERT INTO seller_users (id, seller_id, user_id, role, created_at)
			VALUES ($1, $2, $3, 'owner', NOW())
			ON CONFLICT (seller_id, user_id) DO NOTHING
		`, uuid.New(), demoSellerIDs[8], sEdge1User)

		// -------------------------------------------------------------
		// EDGE CASE 2 — Store Setup (Pending Review)
		// -------------------------------------------------------------
		sEdge2User := upsertUserHelper(ctx, tx, "11111111-1111-4111-8111-111111111110", "Магазин На Настройке", "demo-seller-edge-02@demo.zamk.local")
		_, _ = tx.Exec(ctx, `
			INSERT INTO sellers (id, brand_name, slug, description, contact_email, status, created_at, updated_at)
			VALUES ($1, 'ZAMK Demo Setup', 'zamk-demo-setup', 'Настройка каталога', 'demo-seller-edge-02@demo.zamk.local', 'pending', NOW(), NOW())
			ON CONFLICT (id) DO UPDATE SET status = 'pending'
		`, demoSellerIDs[9])
		_, _ = tx.Exec(ctx, `
			INSERT INTO seller_users (id, seller_id, user_id, role, created_at)
			VALUES ($1, $2, $3, 'owner', NOW())
			ON CONFLICT (seller_id, user_id) DO NOTHING
		`, uuid.New(), demoSellerIDs[9], sEdge2User)

		// -------------------------------------------------------------
		// MODERATION DEMO PRODUCTS (8 Scenarios)
		// -------------------------------------------------------------
		adminUserID := uuid.MustParse("00000000-0000-4000-8000-000000000001")
		// Clean up known leftover development/e2e test fixture products safely
		cleanupQueries := []string{
			`DELETE FROM order_reservations WHERE reservation_id IN (SELECT id FROM reservations WHERE inventory_item_id IN (SELECT id FROM inventory_items WHERE product_id IN (SELECT id FROM products WHERE title ILIKE 'Test%' OR title ILIKE 'Product A%' OR title ILIKE 'Upload Test%')))`,
			`DELETE FROM reservations WHERE inventory_item_id IN (SELECT id FROM inventory_items WHERE product_id IN (SELECT id FROM products WHERE title ILIKE 'Test%' OR title ILIKE 'Product A%' OR title ILIKE 'Upload Test%'))`,
			`DELETE FROM order_items WHERE product_id IN (SELECT id FROM products WHERE title ILIKE 'Test%' OR title ILIKE 'Product A%' OR title ILIKE 'Upload Test%')`,
			`DELETE FROM cart_items WHERE product_id IN (SELECT id FROM products WHERE title ILIKE 'Test%' OR title ILIKE 'Product A%' OR title ILIKE 'Upload Test%')`,
			`DELETE FROM customer_favorites WHERE product_id IN (SELECT id FROM products WHERE title ILIKE 'Test%' OR title ILIKE 'Product A%' OR title ILIKE 'Upload Test%')`,
			`DELETE FROM product_reviews WHERE product_id IN (SELECT id FROM products WHERE title ILIKE 'Test%' OR title ILIKE 'Product A%' OR title ILIKE 'Upload Test%')`,
			`DELETE FROM inventory_items WHERE product_id IN (SELECT id FROM products WHERE title ILIKE 'Test%' OR title ILIKE 'Product A%' OR title ILIKE 'Upload Test%')`,
			`DELETE FROM product_moderation_logs WHERE product_id IN (SELECT id FROM products WHERE title ILIKE 'Test%' OR title ILIKE 'Product A%' OR title ILIKE 'Upload Test%')`,
			`DELETE FROM product_variants WHERE product_id IN (SELECT id FROM products WHERE title ILIKE 'Test%' OR title ILIKE 'Product A%' OR title ILIKE 'Upload Test%')`,
			`DELETE FROM products WHERE title ILIKE 'Test%' OR title ILIKE 'Product A%' OR title ILIKE 'Upload Test%'`,
		}
		for _, q := range cleanupQueries {
			if _, err := tx.Exec(ctx, q); err != nil {
				slog.Warn("cleanup query info", "query", q, "error", err)
			}
		}

		if err := seedModerationProducts(ctx, tx, demoSellerIDs, categoryID, brandID, adminUserID, now); err != nil {
			slog.Error("seedModerationProducts failed", "error", err)
			return err
		}

		return nil
	})
}

func upsertUserHelper(ctx context.Context, tx pgx.Tx, uidStr, name, email string) uuid.UUID {
	uid := uuid.MustParse(uidStr)
	hash, _ := auth.HashPassword("DemoPass123!")
	var existingID uuid.UUID
	err := tx.QueryRow(ctx, `SELECT id FROM users WHERE email = $1 OR id = $2 LIMIT 1`, email, uid).Scan(&existingID)
	if err == nil {
		_, _ = tx.Exec(ctx, `UPDATE users SET name = $1, email = $2, password_hash = $3 WHERE id = $4`, name, email, hash, existingID)
		return existingID
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO users (id, name, email, password_hash, role, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 'seller', 'active', NOW(), NOW())
		ON CONFLICT (id) DO UPDATE SET email = EXCLUDED.email, name = EXCLUDED.name
	`, uid, name, email, hash)
	if err != nil {
		slog.Error("upsertUserHelper error", "error", err, "email", email)
	}
	return uid
}

func upsertSellerHelper(ctx context.Context, tx pgx.Tx, sid uuid.UUID, ownerName, ownerEmail, brandName, slug, status string, userID uuid.UUID, createdAt time.Time) uuid.UUID {
	var existingID uuid.UUID
	err := tx.QueryRow(ctx, `SELECT id FROM sellers WHERE id = $1 OR slug = $2`, sid, slug).Scan(&existingID)
	if err == nil {
		_, _ = tx.Exec(ctx, `UPDATE sellers SET brand_name = $1, slug = $2, status = $3, contact_email = $4 WHERE id = $5`, brandName, slug, status, ownerEmail, existingID)
		sid = existingID
	} else {
		_, err = tx.Exec(ctx, `
			INSERT INTO sellers (id, brand_name, slug, description, contact_email, contact_phone, status, created_at, updated_at)
			VALUES ($1, $2, $3, 'Официальный магазин инновационных замков ZAMK', $4, '+7 (999) 000-00-00', $5, $6, NOW())
			ON CONFLICT (id) DO UPDATE SET status = EXCLUDED.status, brand_name = EXCLUDED.brand_name
		`, sid, brandName, slug, ownerEmail, status, createdAt)
		if err != nil {
			slog.Error("upsertSellerHelper error", "error", err, "sellerID", sid)
		}
	}

	suID := uuid.NewSHA1(sid, []byte("seller_user_owner"))
	_, err = tx.Exec(ctx, `
		INSERT INTO seller_users (id, seller_id, user_id, role, created_at)
		VALUES ($1, $2, $3, 'owner', NOW())
		ON CONFLICT (seller_id, user_id) DO NOTHING
	`, suID, sid, userID)
	if err != nil {
		slog.Error("upsertSellerUser error", "error", err, "sellerID", sid, "userID", userID)
	}

	return sid
}

func seedSellerOperations(ctx context.Context, tx pgx.Tx, sellerID uuid.UUID, customerID uuid.UUID, categoryID, brandID uuid.UUID, productTitle string, ordersCount int, grossSales int, rating float64, reviewsCount int, warningsCount, violationsCount int, hasPlan bool, perfCategory string, now time.Time) {
	productID := uuid.NewSHA1(sellerID, []byte("prod_ops"))
	variantID := uuid.NewSHA1(sellerID, []byte("var_ops"))
	var lastOrderID uuid.UUID
	var lastOrderItemID uuid.UUID
	productSlug := fmt.Sprintf("prod-%s", productID.String()[:8])
	sku := fmt.Sprintf("SKU-%s", variantID.String()[:8])
	priceAmount := grossSales / ordersCount

	_, err := tx.Exec(ctx, `
		INSERT INTO products (id, seller_id, category_id, brand_id, title, slug, description, status, price_cents, currency, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, 'Высокотехнологичный замок премиум-класса', 'published', $7, 'RUB', NOW() - INTERVAL '60 days', NOW())
		ON CONFLICT (id) DO UPDATE SET title = $5, price_cents = $7
	`, productID, sellerID, categoryID, brandID, productTitle, productSlug, priceAmount)
	if err != nil {
		slog.Error("insert products error", "error", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO product_variants (id, product_id, sku, price_cents, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, true, NOW() - INTERVAL '60 days', NOW())
		ON CONFLICT (id) DO NOTHING
	`, variantID, productID, sku, priceAmount)
	if err != nil {
		slog.Error("insert product_variants error", "error", err)
	}

	invID := uuid.NewSHA1(sellerID, []byte("inv_ops"))
	_, err = tx.Exec(ctx, `
		INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 150, 2, NOW() - INTERVAL '60 days', NOW())
		ON CONFLICT (product_variant_id) DO UPDATE SET total_stock = 150
	`, invID, productID, variantID, sellerID)
	if err != nil {
		slog.Error("insert inventory_items error", "error", err)
	}

	for i := 0; i < ordersCount; i++ {
		orderID := uuid.NewSHA1(sellerID, []byte(fmt.Sprintf("order_%d", i)))

		var orderDate time.Time
		if i < ordersCount/5 {
			orderDate = now.Add(-time.Duration(i+1) * 12 * time.Hour)
		} else if i < (ordersCount * 5 / 10) {
			orderDate = now.Add(-time.Duration(i) * 24 * time.Hour)
		} else if i < (ordersCount * 8 / 10) {
			orderDate = now.Add(-time.Duration(i) * 36 * time.Hour)
		} else {
			orderDate = now.Add(-time.Duration(i) * 60 * time.Hour)
		}

		status := "delivered"
		if i == 0 {
			status = "assembling"
		}

		_, err = tx.Exec(ctx, `
			INSERT INTO orders (id, user_id, status, total_price_cents, currency, customer_name, customer_phone, customer_email, delivery_address, created_at, updated_at)
			VALUES ($1, $2, $3, $4, 'RUB', 'Иван Покупатель', '+7 (900) 123-45-67', 'customer@demo.zamk.local', 'г. Москва, ул. Тверская 12, кв. 45', $5, $5)
			ON CONFLICT (id) DO NOTHING
		`, orderID, customerID, status, priceAmount, orderDate)
		if err != nil {
			slog.Error("insert orders error", "error", err)
		}

		fulID := uuid.NewSHA1(sellerID, []byte(fmt.Sprintf("fulfillment_%d", i)))
		fulStatus := "delivered"
		if status == "assembling" {
			fulStatus = "assembling"
		}

		_, err = tx.Exec(ctx, `
			INSERT INTO order_fulfillments (id, order_id, seller_id, status, subtotal_cents, commission_bps, seller_amount_cents, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, 900, $6, $7, $7)
			ON CONFLICT (id) DO NOTHING
		`, fulID, orderID, sellerID, fulStatus, priceAmount, int(float64(priceAmount)*0.91), orderDate)
		orderItemID := uuid.NewSHA1(sellerID, []byte(fmt.Sprintf("order_item_%d", i)))
		_, err = tx.Exec(ctx, `
			INSERT INTO order_items (id, order_id, order_fulfillment_id, product_id, product_variant_id, seller_id, title, product_slug, price_cents, quantity, subtotal_price_cents, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 1, $9, $10)
			ON CONFLICT (id) DO NOTHING
		`, orderItemID, orderID, fulID, productID, variantID, sellerID, productTitle, productSlug, priceAmount, orderDate)
		if err != nil {
			slog.Error("insert order_items error", "error", err)
		}

		lastOrderID = orderID
		lastOrderItemID = orderItemID
	}

	// Add reviews
	for r := 0; r < reviewsCount; r++ {
		if lastOrderID == uuid.Nil || lastOrderItemID == uuid.Nil {
			break
		}
		revRating := int(rating)
		if r%3 == 0 && revRating > 1 {
			revRating--
		}
		revID := uuid.NewSHA1(sellerID, []byte(fmt.Sprintf("review_%d", r)))
		_, err = tx.Exec(ctx, `
			INSERT INTO product_reviews (id, product_id, product_variant_id, order_id, order_item_id, user_id, seller_id, rating, comment, status, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'Отличный смарт-замок ZAMK, очень доволен качеством!', 'published', NOW() - INTERVAL '10 days', NOW())
			ON CONFLICT (order_item_id) DO NOTHING
		`, revID, productID, variantID, lastOrderID, lastOrderItemID, customerID, sellerID, revRating)
		if err != nil {
			slog.Error("insert product_reviews error", "error", err)
		}
	}
}

func seedSellerStockRisk(ctx context.Context, tx pgx.Tx, sellerID uuid.UUID, categoryID, brandID uuid.UUID, now time.Time) {
	p1ID := uuid.NewSHA1(sellerID, []byte("prod_zero"))
	v1ID := uuid.NewSHA1(sellerID, []byte("var_zero"))
	_, _ = tx.Exec(ctx, `INSERT INTO products (id, seller_id, category_id, brand_id, title, slug, status, price_cents, created_at, updated_at) VALUES ($1, $2, $3, $4, 'Замок ZAMK Zero (Out of Stock)', 'zamk-zero', 'published', 1500000, NOW(), NOW()) ON CONFLICT (id) DO NOTHING`, p1ID, sellerID, categoryID, brandID)
	_, _ = tx.Exec(ctx, `INSERT INTO product_variants (id, product_id, sku, price_cents, is_active, created_at, updated_at) VALUES ($1, $2, 'SKU-ZERO', 1500000, true, NOW(), NOW()) ON CONFLICT (id) DO NOTHING`, v1ID, p1ID)
	_, _ = tx.Exec(ctx, `INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock, created_at, updated_at) VALUES ($1, $2, $3, $4, 0, 0, NOW(), NOW()) ON CONFLICT (product_variant_id) DO UPDATE SET total_stock = 0`, uuid.NewSHA1(sellerID, []byte("inv_zero")), p1ID, v1ID, sellerID)

	p2ID := uuid.NewSHA1(sellerID, []byte("prod_low"))
	v2ID := uuid.NewSHA1(sellerID, []byte("var_low"))
	_, _ = tx.Exec(ctx, `INSERT INTO products (id, seller_id, category_id, brand_id, title, slug, status, price_cents, created_at, updated_at) VALUES ($1, $2, $3, $4, 'Замок ZAMK Low (Low Stock)', 'zamk-low', 'published', 1800000, NOW(), NOW()) ON CONFLICT (id) DO NOTHING`, p2ID, sellerID, categoryID, brandID)
	_, _ = tx.Exec(ctx, `INSERT INTO product_variants (id, product_id, sku, price_cents, is_active, created_at, updated_at) VALUES ($1, $2, 'SKU-LOW', 1800000, true, NOW(), NOW()) ON CONFLICT (id) DO NOTHING`, v2ID, p2ID)
	_, _ = tx.Exec(ctx, `INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock, created_at, updated_at) VALUES ($1, $2, $3, $4, 2, 0, NOW(), NOW()) ON CONFLICT (product_variant_id) DO UPDATE SET total_stock = 2`, uuid.NewSHA1(sellerID, []byte("inv_low")), p2ID, v2ID, sellerID)
}

func seedSellerReturns(ctx context.Context, tx pgx.Tx, sellerID uuid.UUID, categoryID, brandID uuid.UUID, now time.Time) {
	pID := uuid.NewSHA1(sellerID, []byte("prod_ret"))
	vID := uuid.NewSHA1(sellerID, []byte("var_ret"))
	_, _ = tx.Exec(ctx, `INSERT INTO products (id, seller_id, category_id, brand_id, title, slug, status, price_cents, created_at, updated_at) VALUES ($1, $2, $3, $4, 'Замок ZAMK Return Special', 'zamk-return', 'published', 1200000, NOW(), NOW()) ON CONFLICT (id) DO NOTHING`, pID, sellerID, categoryID, brandID)
	_, _ = tx.Exec(ctx, `INSERT INTO product_variants (id, product_id, sku, price_cents, is_active, created_at, updated_at) VALUES ($1, $2, 'SKU-RET', 1200000, true, NOW(), NOW()) ON CONFLICT (id) DO NOTHING`, vID, pID)
	_, _ = tx.Exec(ctx, `INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock, created_at, updated_at) VALUES ($1, $2, $3, $4, 50, 0, NOW(), NOW()) ON CONFLICT (product_variant_id) DO UPDATE SET total_stock = 50`, uuid.NewSHA1(sellerID, []byte("inv_ret")), pID, vID, sellerID)

	for i := 0; i < 6; i++ {
		oID := uuid.NewSHA1(sellerID, []byte(fmt.Sprintf("order_ret_%d", i)))
		custID := uuid.MustParse("33333333-3333-4333-8333-333333333333")
		_, err := tx.Exec(ctx, `INSERT INTO orders (id, user_id, status, total_price_cents, currency, customer_name, customer_phone, customer_email, delivery_address, created_at, updated_at) VALUES ($1, $2, 'cancelled', 1200000, 'RUB', 'Иван Покупатель', '+7 (900) 123-45-67', 'customer@demo.zamk.local', 'г. Москва', $3, $3) ON CONFLICT (id) DO NOTHING`, oID, custID, now.Add(-time.Duration(i+1)*24*time.Hour))
		if err != nil {
			slog.Error("seedSellerReturns insert order error", "error", err)
		}

		retID := uuid.NewSHA1(sellerID, []byte(fmt.Sprintf("return_%d", i)))
		reason := "Несоответствие описанию"
		if i%2 == 0 {
			reason = "Брак товара"
		}
		_, err = tx.Exec(ctx, `INSERT INTO returns (id, order_id, user_id, status, reason, created_at, updated_at) VALUES ($1, $2, $3, 'approved', $4, $5, $5) ON CONFLICT (id) DO NOTHING`, retID, oID, custID, reason, now.Add(-time.Duration(i+1)*24*time.Hour))
		if err != nil {
			slog.Error("seedSellerReturns insert returns error", "error", err)
		}
	}
}

func seedSellerInactive(ctx context.Context, tx pgx.Tx, sellerID uuid.UUID, categoryID, brandID uuid.UUID, now time.Time) {
	pID := uuid.NewSHA1(sellerID, []byte("prod_old"))
	vID := uuid.NewSHA1(sellerID, []byte("var_old"))
	oldDate := now.Add(-120 * 24 * time.Hour)
	_, _ = tx.Exec(ctx, `INSERT INTO products (id, seller_id, category_id, brand_id, title, slug, status, price_cents, created_at, updated_at) VALUES ($1, $2, $3, $4, 'Замок ZAMK Inactive Model', 'zamk-inactive', 'published', 1000000, $5, $5) ON CONFLICT (id) DO NOTHING`, pID, sellerID, categoryID, brandID, oldDate)
	_, _ = tx.Exec(ctx, `INSERT INTO product_variants (id, product_id, sku, price_cents, is_active, created_at, updated_at) VALUES ($1, $2, 'SKU-OLD', 1000000, true, $3, $3) ON CONFLICT (id) DO NOTHING`, vID, pID, oldDate)
	_, _ = tx.Exec(ctx, `INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock, created_at, updated_at) VALUES ($1, $2, $3, $4, 10, 0, $5, $5) ON CONFLICT (product_variant_id) DO UPDATE SET total_stock = 10`, uuid.NewSHA1(sellerID, []byte("inv_old")), pID, vID, sellerID, oldDate)
}

func seedSellerFinance(ctx context.Context, tx pgx.Tx, sellerID uuid.UUID, categoryID, brandID uuid.UUID, now time.Time) {
	p1ID := uuid.NewSHA1(sellerID, []byte("payout_1"))
	_, _ = tx.Exec(ctx, `INSERT INTO payouts (id, seller_id, status, amount_cents, currency, created_at, updated_at) VALUES ($1, $2, 'paid', 45000000, 'RUB', NOW() - INTERVAL '15 days', NOW() - INTERVAL '14 days') ON CONFLICT (id) DO NOTHING`, p1ID, sellerID)

	p2ID := uuid.NewSHA1(sellerID, []byte("payout_2"))
	_, _ = tx.Exec(ctx, `INSERT INTO payouts (id, seller_id, status, amount_cents, currency, created_at, updated_at) VALUES ($1, $2, 'requested', 15000000, 'RUB', NOW() - INTERVAL '2 days', NOW() - INTERVAL '2 days') ON CONFLICT (id) DO NOTHING`, p2ID, sellerID)

	_, _ = tx.Exec(ctx, `INSERT INTO seller_balance_ledger (id, seller_id, payout_id, type, amount_cents, currency, comment, created_at) VALUES ($1, $2, $3, 'payout_paid', 45000000, 'RUB', 'Выплата на расчётный счет', NOW() - INTERVAL '14 days') ON CONFLICT (id) DO NOTHING`, uuid.NewSHA1(sellerID, []byte("ledger_1")), sellerID, p1ID)
	_, _ = tx.Exec(ctx, `INSERT INTO seller_balance_ledger (id, seller_id, type, amount_cents, currency, comment, created_at) VALUES ($1, $2, 'sale_available', 15000000, 'RUB', 'Оплата за выполненный заказ', NOW() - INTERVAL '2 days') ON CONFLICT (id) DO NOTHING`, uuid.NewSHA1(sellerID, []byte("ledger_2")), sellerID)
}

func addWarning(ctx context.Context, tx pgx.Tx, sellerID uuid.UUID, wType, title, message, severity string) {
	wID := uuid.NewSHA1(sellerID, []byte(fmt.Sprintf("warning_%s_%s", wType, title)))
	_, _ = tx.Exec(ctx, `
		INSERT INTO seller_warnings (id, seller_id, type, title, message, severity, status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, 'active', NOW()) ON CONFLICT (id) DO NOTHING
	`, wID, sellerID, wType, title, message, severity)
}

func addViolation(ctx context.Context, tx pgx.Tx, sellerID uuid.UUID, vType, title, description, severity string) {
	vID := uuid.NewSHA1(sellerID, []byte(fmt.Sprintf("violation_%s_%s", vType, title)))
	_, _ = tx.Exec(ctx, `
		INSERT INTO seller_violations (id, seller_id, type, title, description, severity, status, counts_for_penalty, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, 'active', true, NOW()) ON CONFLICT (id) DO NOTHING
	`, vID, sellerID, vType, title, description, severity)
}

func addImprovementPlan(ctx context.Context, tx pgx.Tx, sellerID uuid.UUID, status, reason, actionDesc string, deadline time.Time) {
	planID := uuid.NewSHA1(sellerID, []byte(fmt.Sprintf("plan_%s_%s", status, reason)))
	actionsJSON := fmt.Sprintf(`[{"id":"a1","title":"%s","status":"pending"}]`, actionDesc)
	_, _ = tx.Exec(ctx, `
		INSERT INTO seller_improvement_plans (id, seller_id, status, reason, actions, internal_comment, deadline, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5::jsonb, 'Назначен регулярный мониторинг администратора', $6, NOW(), NOW()) ON CONFLICT (id) DO NOTHING
	`, planID, sellerID, status, reason, actionsJSON, deadline)
}

func seedModerationProducts(ctx context.Context, tx pgx.Tx, sellerIDs []uuid.UUID, categoryID, brandID uuid.UUID, adminUserID uuid.UUID, now time.Time) error {
	_ = upsertUserHelper(ctx, tx, adminUserID.String(), "Главный Модератор", "admin-moderator@demo.zamk.local")
	_, _ = tx.Exec(ctx, `UPDATE users SET role = 'admin' WHERE id = $1`, adminUserID)
	_, _ = tx.Exec(ctx, `
		INSERT INTO staff_members (user_id, staff_role_id, status)
		SELECT $1, id, 'active' FROM staff_roles WHERE code = 'owner'
		ON CONFLICT (user_id) DO UPDATE SET staff_role_id = EXCLUDED.staff_role_id
	`, adminUserID)

	exec := func(sql string, args ...any) error {
		if _, err := tx.Exec(ctx, sql, args...); err != nil {
			slog.Error("seedModerationProducts query failed", "sql", sql, "error", err)
			return err
		}
		return nil
	}

	// Product 1: Fully correct product (Ready for approval)
	p1ID := uuid.MustParse("22222222-2222-4222-8222-222222222201")
	p1Submitted := now.Add(-4 * time.Hour)
		if err := exec(`
		INSERT INTO products (id, seller_id, category_id, brand_id, title, slug, description, status, price_cents, currency, main_image_url, created_at, updated_at, submitted_at, gender, color, material, care_instructions)
		VALUES ($1, $2, $3, $4, 'Умный биометрический замок ZAMK Pro Max', 'zamk-pro-max-mod', 'Полнофункциональный электронный замок с сканером отпечатков пальцев, сенсорной клавиатурой и поддержкой Wi-Fi.', 'pending_moderation', 1499000, 'RUB', 'https://images.unsplash.com/photo-1558002038-1055907df827?auto=format&fit=crop&w=800&q=80', $5, $5, $5, 'unisex', 'Черный матовый', 'Закаленная сталь', 'Протирать сухой тканью')
		ON CONFLICT (id) DO UPDATE SET status = 'pending_moderation', submitted_at = $5
	`, p1ID, sellerIDs[0], categoryID, brandID, p1Submitted); err != nil { return err }
	v1ID := uuid.MustParse("33333333-3333-4333-8333-333333333301")
	v2ID := uuid.MustParse("33333333-3333-4333-8333-333333333302")
	if err := exec(`INSERT INTO product_variants (id, product_id, sku, size, color, price_cents, is_active, created_at, updated_at) VALUES ($1, $2, 'ZAMK-PRO-BLK', 'Standard', 'Черный', 1499000, true, NOW(), NOW()) ON CONFLICT (id) DO NOTHING`, v1ID, p1ID); err != nil { return err }
	if err := exec(`INSERT INTO product_variants (id, product_id, sku, size, color, price_cents, is_active, created_at, updated_at) VALUES ($1, $2, 'ZAMK-PRO-SLV', 'Standard', 'Серебристый', 1599000, true, NOW(), NOW()) ON CONFLICT (id) DO NOTHING`, v2ID, p1ID); err != nil { return err }
	if err := exec(`INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock, created_at, updated_at) VALUES ($1, $2, $3, $4, 25, 0, NOW(), NOW()) ON CONFLICT (product_variant_id) DO UPDATE SET total_stock = 25`, uuid.New(), p1ID, v1ID, sellerIDs[0]); err != nil { return err }
	if err := exec(`INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock, created_at, updated_at) VALUES ($1, $2, $3, $4, 10, 0, NOW(), NOW()) ON CONFLICT (product_variant_id) DO UPDATE SET total_stock = 10`, uuid.New(), p1ID, v2ID, sellerIDs[0]); err != nil { return err }

	// Product 2: Missing main image
	p2ID := uuid.MustParse("22222222-2222-4222-8222-222222222202")
	p2Submitted := now.Add(-18 * time.Hour)
	if err := exec(`
		INSERT INTO products (id, seller_id, category_id, brand_id, title, slug, description, status, price_cents, currency, main_image_url, created_at, updated_at, submitted_at)
		VALUES ($1, $2, $3, $4, 'Кодовая панель ZAMK Touch Slim', 'zamk-touch-slim-mod', 'Компактная сенсорная панель управления для офисных дверей.', 'pending_moderation', 850000, 'RUB', NULL, $5, $5, $5)
		ON CONFLICT (id) DO UPDATE SET status = 'pending_moderation', submitted_at = $5
	`, p2ID, sellerIDs[1], categoryID, brandID, p2Submitted); err != nil { return err }

	// Product 3: Missing description
	p3ID := uuid.MustParse("22222222-2222-4222-8222-222222222203")
	p3Submitted := now.Add(-30 * time.Hour)
	if err := exec(`
		INSERT INTO products (id, seller_id, category_id, brand_id, title, slug, description, status, price_cents, currency, main_image_url, created_at, updated_at, submitted_at)
		VALUES ($1, $2, $3, $4, 'Дверная ручка ZAMK BioHandle V2', 'zamk-biohandle-v2-mod', NULL, 'pending_moderation', 1200000, 'RUB', 'https://images.unsplash.com/photo-1558002038-1055907df827?auto=format&fit=crop&w=800&q=80', $5, $5, $5)
		ON CONFLICT (id) DO UPDATE SET status = 'pending_moderation', submitted_at = $5
	`, p3ID, sellerIDs[2], categoryID, brandID, p3Submitted); err != nil { return err }

	// Product 4: Missing required characteristics / brand
	p4ID := uuid.MustParse("22222222-2222-4222-8222-222222222204")
	p4Submitted := now.Add(-52 * time.Hour)
	if err := exec(`
		INSERT INTO products (id, seller_id, category_id, brand_id, title, slug, description, status, price_cents, currency, main_image_url, created_at, updated_at, submitted_at)
		VALUES ($1, $2, $3, NULL, 'Замок накладной без указания бренда', 'zamk-no-brand-mod', 'Простой накладной замок для подсобных помещений.', 'pending_moderation', 450000, 'RUB', 'https://images.unsplash.com/photo-1558002038-1055907df827?auto=format&fit=crop&w=800&q=80', $4, $4, $4)
		ON CONFLICT (id) DO UPDATE SET status = 'pending_moderation', submitted_at = $4
	`, p4ID, sellerIDs[3], categoryID, p4Submitted); err != nil { return err }

	// Product 5: Variant without price / 0 price
	p5ID := uuid.MustParse("22222222-2222-4222-8222-222222222205")
	p5Submitted := now.Add(-12 * time.Hour)
	if err := exec(`
		INSERT INTO products (id, seller_id, category_id, brand_id, title, slug, description, status, price_cents, currency, main_image_url, created_at, updated_at, submitted_at)
		VALUES ($1, $2, $3, $4, 'Умный защёлочный замок ZAMK Latch-01', 'zamk-latch-01-mod', 'Замок защёлка с поддержкой карт доступа RFID.', 'pending_moderation', 990000, 'RUB', 'https://images.unsplash.com/photo-1558002038-1055907df827?auto=format&fit=crop&w=800&q=80', $5, $5, $5)
		ON CONFLICT (id) DO UPDATE SET status = 'pending_moderation', submitted_at = $5
	`, p5ID, sellerIDs[4], categoryID, brandID, p5Submitted); err != nil { return err }
	v5ID1 := uuid.MustParse("33333333-3333-4333-8333-333333333305")
	v5ID2 := uuid.MustParse("33333333-3333-4333-8333-333333333306")
	if err := exec(`INSERT INTO product_variants (id, product_id, sku, size, color, price_cents, is_active, created_at, updated_at) VALUES ($1, $2, 'ZAMK-LATCH-STD', 'Standard', 'Хром', 990000, true, NOW(), NOW()) ON CONFLICT (id) DO NOTHING`, v5ID1, p5ID); err != nil { return err }
	if err := exec(`INSERT INTO product_variants (id, product_id, sku, size, color, price_cents, is_active, created_at, updated_at) VALUES ($1, $2, 'ZAMK-LATCH-NOPRICE', 'Pro', 'Золото', 0, true, NOW(), NOW()) ON CONFLICT (id) DO NOTHING`, v5ID2, p5ID); err != nil { return err }

	// Product 6: Duplicate SKU
	p6ID := uuid.MustParse("22222222-2222-4222-8222-222222222206")
	p6Submitted := now.Add(-8 * time.Hour)
	if err := exec(`
		INSERT INTO products (id, seller_id, category_id, brand_id, title, slug, description, status, price_cents, currency, main_image_url, created_at, updated_at, submitted_at)
		VALUES ($1, $2, $3, $4, 'Контроллер доступа ZAMK Relay Unit', 'zamk-relay-unit-mod', 'Модуль управления автоматическими замками.', 'pending_moderation', 620000, 'RUB', 'https://images.unsplash.com/photo-1558002038-1055907df827?auto=format&fit=crop&w=800&q=80', $5, $5, $5)
		ON CONFLICT (id) DO UPDATE SET status = 'pending_moderation', submitted_at = $5
	`, p6ID, sellerIDs[5], categoryID, brandID, p6Submitted); err != nil { return err }
	v6ID1 := uuid.MustParse("33333333-3333-4333-8333-333333333307")
	v6ID2 := uuid.MustParse("33333333-3333-4333-8333-333333333308")
	if err := exec(`INSERT INTO product_variants (id, product_id, sku, size, color, price_cents, is_active, created_at, updated_at) VALUES ($1, $2, 'SKU-ZAMK-RELAY-001', 'Single', 'Белый', 620000, true, NOW(), NOW()) ON CONFLICT (id) DO NOTHING`, v6ID1, p6ID); err != nil { return err }
	if err := exec(`INSERT INTO product_variants (id, product_id, sku, size, color, price_cents, is_active, created_at, updated_at) VALUES ($1, $2, 'SKU-ZAMK-RELAY-001', 'Dual', 'Белый', 620000, true, NOW(), NOW()) ON CONFLICT (id) DO NOTHING`, v6ID2, p6ID); err != nil { return err }

	// Product 7: Multiple warnings product (Short title, low stock)
	p7ID := uuid.MustParse("22222222-2222-4222-8222-222222222207")
	p7Submitted := now.Add(-2 * time.Hour)
	if err := exec(`
		INSERT INTO products (id, seller_id, category_id, brand_id, title, slug, description, status, price_cents, currency, main_image_url, created_at, updated_at, submitted_at)
		VALUES ($1, $2, $3, $4, 'Замок', 'zamk-short-title-mod', 'Качественный дверной замок.', 'pending_moderation', 1150000, 'RUB', 'https://images.unsplash.com/photo-1558002038-1055907df827?auto=format&fit=crop&w=800&q=80', $5, $5, $5)
		ON CONFLICT (id) DO UPDATE SET status = 'pending_moderation', submitted_at = $5
	`, p7ID, sellerIDs[6], categoryID, brandID, p7Submitted); err != nil { return err }
	v7ID := uuid.MustParse("33333333-3333-4333-8333-333333333309")
	if err := exec(`INSERT INTO product_variants (id, product_id, sku, size, color, price_cents, is_active, created_at, updated_at) VALUES ($1, $2, 'SKU-SHORT-01', 'Universal', 'Черный', 1150000, true, NOW(), NOW()) ON CONFLICT (id) DO NOTHING`, v7ID, p7ID); err != nil { return err }
	if err := exec(`INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock, created_at, updated_at) VALUES ($1, $2, $3, $4, 0, 0, NOW(), NOW()) ON CONFLICT (product_variant_id) DO UPDATE SET total_stock = 0`, uuid.New(), p7ID, v7ID, sellerIDs[6]); err != nil { return err }

	// Product 8: Previously rejected and resubmitted product
	p8ID := uuid.MustParse("22222222-2222-4222-8222-222222222208")
	p8Submitted := now.Add(-1 * time.Hour)
	commentResubmitted := "Исправлены фотографии"
	if err := exec(`
		INSERT INTO products (id, seller_id, category_id, brand_id, title, slug, description, status, price_cents, currency, main_image_url, created_at, updated_at, submitted_at, moderation_comment)
		VALUES ($1, $2, $3, $4, 'Премиальный электронный замок ZAMK Titanium Key', 'zamk-titanium-key-mod', 'Титановый замок высокой прочности для премиальных объектов и загородных домов.', 'pending_moderation', 4500000, 'RUB', 'https://images.unsplash.com/photo-1558002038-1055907df827?auto=format&fit=crop&w=800&q=80', $5, $5, $5, $6)
		ON CONFLICT (id) DO UPDATE SET status = 'pending_moderation', submitted_at = $5
	`, p8ID, sellerIDs[0], categoryID, brandID, p8Submitted, commentResubmitted); err != nil { return err }
	v8ID := uuid.MustParse("33333333-3333-4333-8333-333333333310")
	if err := exec(`INSERT INTO product_variants (id, product_id, sku, size, color, price_cents, is_active, created_at, updated_at) VALUES ($1, $2, 'SKU-TITANIUM-01', 'Premium', 'Титан', 4500000, true, NOW(), NOW()) ON CONFLICT (id) DO NOTHING`, v8ID, p8ID); err != nil { return err }
	if err := exec(`INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock, created_at, updated_at) VALUES ($1, $2, $3, $4, 15, 0, NOW(), NOW()) ON CONFLICT (product_variant_id) DO UPDATE SET total_stock = 15`, uuid.New(), p8ID, v8ID, sellerIDs[0]); err != nil { return err }

	// Add moderation history for Product 8 with deterministic UUIDs
	fromDraft := "draft"
	toPending := "pending_moderation"
	toRejected := "rejected"
	comment1 := "Нечеткое изображение главного вида. Пожалуйста, загрузите высококачественное фото."
	comment2 := "Исправлены фотографии по замечаниям модератора."

	log1ID := uuid.MustParse("44444444-4444-4444-8444-444444444401")
	log2ID := uuid.MustParse("44444444-4444-4444-8444-444444444402")
	log3ID := uuid.MustParse("44444444-4444-4444-8444-444444444403")

	if err := exec(`
		INSERT INTO product_moderation_logs (id, product_id, admin_user_id, from_status, to_status, comment, created_at)
		VALUES ($1, $2, NULL, $3, $4, NULL, $5) ON CONFLICT (id) DO NOTHING
	`, log1ID, p8ID, fromDraft, toPending, now.Add(-5*24*time.Hour)); err != nil { return err }
	if err := exec(`
		INSERT INTO product_moderation_logs (id, product_id, admin_user_id, from_status, to_status, comment, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7) ON CONFLICT (id) DO NOTHING
	`, log2ID, p8ID, adminUserID, toPending, toRejected, comment1, now.Add(-3*24*time.Hour)); err != nil { return err }
	if err := exec(`
		INSERT INTO product_moderation_logs (id, product_id, admin_user_id, from_status, to_status, comment, created_at)
		VALUES ($1, $2, NULL, $3, $4, $5, $6) ON CONFLICT (id) DO NOTHING
	`, log3ID, p8ID, toRejected, toPending, comment2, now.Add(-1*time.Hour)); err != nil { return err }

	return nil
}
