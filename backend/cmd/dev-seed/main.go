package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/auth"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/config"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/staff"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
)

const (
	adminEmail       = "admin@zamk.local"
	adminPassword    = "Admin12345!"
	sellerEmail      = "seller@zamk.local"
	sellerPassword   = "Seller12345!"
	customerEmail    = "customer@zamk.local"
	customerPassword = "Customer12345!"
)

var seedIDs = struct {
	AdminUser     uuid.UUID
	SellerUser    uuid.UUID
	CustomerUser  uuid.UUID
	Seller        uuid.UUID
	SellerUserMap uuid.UUID
	Category      uuid.UUID
	Brand         uuid.UUID
	Product       uuid.UUID
	Variant       uuid.UUID
	Image         uuid.UUID
	Inventory     uuid.UUID
	Movement      uuid.UUID
}{
	AdminUser:     uuid.MustParse("11111111-1111-4111-8111-111111111111"),
	SellerUser:    uuid.MustParse("22222222-2222-4222-8222-222222222222"),
	CustomerUser:  uuid.MustParse("33333333-3333-4333-8333-333333333333"),
	Seller:        uuid.MustParse("44444444-4444-4444-8444-444444444444"),
	SellerUserMap: uuid.MustParse("55555555-5555-4555-8555-555555555555"),
	Category:      uuid.MustParse("66666666-6666-4666-8666-666666666666"),
	Brand:         uuid.MustParse("77777777-7777-4777-8777-777777777777"),
	Product:       uuid.MustParse("88888888-8888-4888-8888-888888888888"),
	Variant:       uuid.MustParse("99999999-9999-4999-8999-999999999999"),
	Image:         uuid.MustParse("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"),
	Inventory:     uuid.MustParse("bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"),
	Movement:      uuid.MustParse("cccccccc-cccc-4ccc-8ccc-cccccccccccc"),
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		logger.Error("failed to load config", "error", err)
		os.Exit(1)
	}
	if strings.EqualFold(cfg.App.Env, "production") {
		logger.Error("dev seed refused to run in production")
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pgClient, err := postgres.NewClient(ctx, cfg.Postgres.DSN)
	if err != nil {
		logger.Error("failed to connect to postgres", "error", err)
		os.Exit(1)
	}
	defer pgClient.Close()

	if err := pgClient.RunInTx(ctx, func(tx pgx.Tx) error {
		adminID, err := upsertUser(ctx, tx, seedIDs.AdminUser, "Local Admin", adminEmail, adminPassword, "admin", false)
		if err != nil {
			return err
		}
		sellerUserID, err := upsertUser(ctx, tx, seedIDs.SellerUser, "Local Seller", sellerEmail, sellerPassword, "seller", false)
		if err != nil {
			return err
		}
		customerID, err := upsertUser(ctx, tx, seedIDs.CustomerUser, "Local Customer", customerEmail, customerPassword, "customer", false)
		if err != nil {
			return err
		}
		sellerID, err := upsertSeller(ctx, tx)
		if err != nil {
			return err
		}
		if err := upsertSellerUser(ctx, tx, sellerID, sellerUserID); err != nil {
			return err
		}
		categoryID, err := upsertCategory(ctx, tx)
		if err != nil {
			return err
		}
		brandID, err := upsertBrand(ctx, tx)
		if err != nil {
			return err
		}
		if err := upsertSellerBrand(ctx, tx, sellerID, brandID); err != nil {
			return err
		}
		if err := seedDevProducts(ctx, tx, sellerID, categoryID, brandID, adminID); err != nil {
			return err
		}

		if err := upsertPaymentFixtures(ctx, tx, customerID); err != nil {
			return err
		}

		logger.Info("local dev seed ready", "adminUserId", adminID, "sellerUserId", sellerUserID, "customerUserId", customerID, "sellerId", sellerID)
		return nil
	}); err != nil {
		logger.Error("failed to seed local dev data", "error", err)
		os.Exit(1)
	}

	// Assign owner staff role to admin@zamk.local
	staffRepo := staff.NewRepository(pgClient.Pool)
	var adminUserID uuid.UUID
	if err2 := pgClient.Pool.QueryRow(ctx, `SELECT id FROM users WHERE email = $1`, adminEmail).Scan(&adminUserID); err2 != nil {
		logger.Warn("could not find admin user for staff seed", "error", err2)
	} else {
		if err2 := staffRepo.EnsureOwnerForSeed(ctx, adminUserID); err2 != nil {
			logger.Warn("could not assign owner role to admin", "error", err2)
		} else {
			logger.Info("admin@zamk.local assigned owner role")
		}
	}

	fmt.Println("Local dev seed complete.")
	fmt.Println("Admin:    admin@zamk.local / Admin12345!")
	fmt.Println("Seller:   seller@zamk.local / Seller12345!")
	fmt.Println("Customer: customer@zamk.local / Customer12345!")
}

func upsertUser(ctx context.Context, tx postgres.DBTX, id uuid.UUID, name, email, password, role string, mustChangePassword bool) (uuid.UUID, error) {
	hash, err := auth.HashPassword(password)
	if err != nil {
		return uuid.Nil, fmt.Errorf("hash password for %s: %w", email, err)
	}

	var userID uuid.UUID
	err = tx.QueryRow(ctx, `
		INSERT INTO users (id, name, email, password_hash, role, status, must_change_password, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, 'active', $6, now(), now())
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name,
			email = EXCLUDED.email,
			password_hash = EXCLUDED.password_hash,
			role = EXCLUDED.role,
			status = 'active',
			must_change_password = EXCLUDED.must_change_password,
			updated_at = now()
		RETURNING id
	`, id, name, email, hash, role, mustChangePassword).Scan(&userID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("upsert user %s: %w", email, err)
	}
	return userID, nil
}

func upsertSeller(ctx context.Context, tx postgres.DBTX) (uuid.UUID, error) {
	description := "Local development seller for manual inspection."
	var sellerID uuid.UUID
	err := tx.QueryRow(ctx, `
		INSERT INTO sellers (id, brand_name, slug, description, contact_email, contact_phone, status, logo_url, created_at, updated_at)
		VALUES ($1, 'ZAMK Dev Seller', 'zamk-dev-seller', $2, $3, '+79990000000', 'active', $4, now(), now())
		ON CONFLICT (slug) DO UPDATE SET
			brand_name = EXCLUDED.brand_name,
			description = EXCLUDED.description,
			contact_email = EXCLUDED.contact_email,
			contact_phone = EXCLUDED.contact_phone,
			status = 'active',
			logo_url = EXCLUDED.logo_url,
			updated_at = now()
		RETURNING id
	`, seedIDs.Seller, description, sellerEmail, "https://placehold.co/160x160?text=ZAMK").Scan(&sellerID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("upsert seller: %w", err)
	}
	return sellerID, nil
}

func upsertSellerUser(ctx context.Context, tx postgres.DBTX, sellerID, userID uuid.UUID) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO seller_users (id, seller_id, user_id, role, created_at)
		VALUES ($1, $2, $3, 'owner', now())
		ON CONFLICT (user_id) DO UPDATE SET
			seller_id = EXCLUDED.seller_id,
			role = EXCLUDED.role
	`, seedIDs.SellerUserMap, sellerID, userID)
	if err != nil {
		return fmt.Errorf("upsert seller user: %w", err)
	}
	return nil
}

func upsertCategory(ctx context.Context, tx postgres.DBTX) (uuid.UUID, error) {
	var categoryID uuid.UUID
	err := tx.QueryRow(ctx, `
		INSERT INTO categories (id, name, slug, description, sort_order, is_active, created_at, updated_at)
		VALUES ($1, 'Dev Category', 'dev-category', 'Local dev category for manual testing.', 10, true, now(), now())
		ON CONFLICT (slug) DO UPDATE SET
			name = EXCLUDED.name,
			description = EXCLUDED.description,
			sort_order = EXCLUDED.sort_order,
			is_active = true,
			updated_at = now()
		RETURNING id
	`, seedIDs.Category).Scan(&categoryID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("upsert category: %w", err)
	}
	return categoryID, nil
}

func upsertBrand(ctx context.Context, tx postgres.DBTX) (uuid.UUID, error) {
	var brandID uuid.UUID
	err := tx.QueryRow(ctx, `
		INSERT INTO brands (id, name, slug, description, logo_url, is_active, created_at, updated_at)
		VALUES ($1, 'Dev Brand', 'dev-brand', 'Local dev brand for manual testing.', $2, true, now(), now())
		ON CONFLICT (slug) DO UPDATE SET
			name = EXCLUDED.name,
			description = EXCLUDED.description,
			logo_url = EXCLUDED.logo_url,
			is_active = true,
			updated_at = now()
		RETURNING id
	`, seedIDs.Brand, "https://placehold.co/160x160?text=DEV").Scan(&brandID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("upsert brand: %w", err)
	}
	return brandID, nil
}

func upsertSellerBrand(ctx context.Context, tx postgres.DBTX, sellerID, brandID uuid.UUID) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO seller_brands (id, seller_id, brand_id, status, is_primary, created_at, updated_at)
		VALUES ($1, $2, $3, 'active', true, now(), now())
		ON CONFLICT (seller_id, brand_id) DO UPDATE SET
			status = 'active',
			is_primary = true,
			updated_at = now()
	`, uuid.New(), sellerID, brandID)
	if err != nil {
		return fmt.Errorf("upsert seller brand: %w", err)
	}
	return nil
}

func seedDevProducts(ctx context.Context, tx postgres.DBTX, sellerID, categoryID, brandID, adminID uuid.UUID) error {
	scenarios := []struct {
		title         string
		slug          string
		status        string
		comment       string
		priceCents    int64
		hasInventory  bool
		size          string
		color         string
	}{
		{"Dev Wool Coat", "dev-wool-coat", "published", "Seeded for local development", 1299000, true, "M", "Graphite"},
		{"Draft Sweater", "draft-sweater", "draft", "", 599000, false, "L", "Navy"},
		{"Pending Jeans", "pending-jeans", "pending_moderation", "", 799000, false, "32/32", "Blue"},
		{"Rejected T-Shirt", "rejected-tshirt", "rejected", "Фотографии низкого качества. Пожалуйста, добавьте студийные фото.", 299000, false, "S", "White"},
		{"Approved Sneakers", "approved-sneakers", "approved", "Отлично, можно везти на склад.", 1499000, false, "42", "Black"},
	}

	baseNS := uuid.MustParse("88888888-8888-4888-8888-888888888888")

	for i, s := range scenarios {
		productID := uuid.NewMD5(baseNS, []byte(s.slug+"-prod"))
		variantID := uuid.NewMD5(baseNS, []byte(s.slug+"-var"))
		imageID := uuid.NewMD5(baseNS, []byte(s.slug+"-img"))
		inventoryID := uuid.NewMD5(baseNS, []byte(s.slug+"-inv"))
		movementID := uuid.NewMD5(baseNS, []byte(s.slug+"-mov"))

		err := tx.QueryRow(ctx, `
			INSERT INTO products (
				id, seller_id, category_id, brand_id, title, slug, description, status, gender, color, material,
				care_instructions, price_cents, old_price_cents, currency, main_image_url, created_at, updated_at,
				submitted_at, approved_at, published_at, moderation_comment
			)
			VALUES (
				$1, $2, $3, $4, $5, $6,
				'Minimal product seeded for local manual testing.', $7, 'unisex', $8, 'Test material',
				'Dry clean only', $9, $10, 'RUB', $11, now(), now(), now(), now(), now(), $12
			)
			ON CONFLICT (slug) DO UPDATE SET
				seller_id = EXCLUDED.seller_id,
				category_id = EXCLUDED.category_id,
				brand_id = EXCLUDED.brand_id,
				title = EXCLUDED.title,
				description = EXCLUDED.description,
				status = EXCLUDED.status,
				gender = EXCLUDED.gender,
				color = EXCLUDED.color,
				material = EXCLUDED.material,
				care_instructions = EXCLUDED.care_instructions,
				price_cents = EXCLUDED.price_cents,
				old_price_cents = EXCLUDED.old_price_cents,
				main_image_url = EXCLUDED.main_image_url,
				moderation_comment = EXCLUDED.moderation_comment,
				updated_at = now()
			RETURNING id
		`, productID, sellerID, categoryID, brandID, s.title, s.slug, s.status, s.color, s.priceCents, s.priceCents+300000, "https://placehold.co/900x1200?text="+strings.ReplaceAll(s.title, " ", "+"), s.comment).Scan(&productID)
		if err != nil {
			return fmt.Errorf("upsert product %s: %w", s.slug, err)
		}

		sku := fmt.Sprintf("DEV-SKU-%d", i)
		_, err = tx.Exec(ctx, `
			INSERT INTO product_variants (id, product_id, sku, size, color, barcode, price_cents, is_active, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, true, now(), now())
			ON CONFLICT (id) DO UPDATE SET
				product_id = EXCLUDED.product_id,
				sku = EXCLUDED.sku,
				size = EXCLUDED.size,
				color = EXCLUDED.color,
				barcode = EXCLUDED.barcode,
				price_cents = EXCLUDED.price_cents,
				is_active = true,
				updated_at = now()
		`, variantID, productID, sku, s.size, s.color, fmt.Sprintf("ZMK-DEV-%04d", i+1), s.priceCents)
		if err != nil {
			return fmt.Errorf("upsert variant %s: %w", s.slug, err)
		}

		_, err = tx.Exec(ctx, `
			INSERT INTO product_images (id, product_id, image_url, alt_text, sort_order, created_at)
			VALUES ($1, $2, $3, 'Dev placeholder image', 0, now())
			ON CONFLICT (id) DO UPDATE SET
				product_id = EXCLUDED.product_id,
				image_url = EXCLUDED.image_url,
				alt_text = EXCLUDED.alt_text,
				sort_order = EXCLUDED.sort_order
		`, imageID, productID, "https://placehold.co/900x1200?text="+strings.ReplaceAll(s.title, " ", "+"))
		if err != nil {
			return fmt.Errorf("upsert product image %s: %w", s.slug, err)
		}

		if s.hasInventory {
			_, err = tx.Exec(ctx, `
				INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock, created_at, updated_at)
				VALUES ($1, $2, $3, $4, 25, 0, now(), now())
				ON CONFLICT (product_variant_id) DO UPDATE SET
					product_id = EXCLUDED.product_id,
					seller_id = EXCLUDED.seller_id,
					total_stock = GREATEST(inventory_items.total_stock, 25),
					reserved_stock = LEAST(inventory_items.reserved_stock, GREATEST(inventory_items.total_stock, 25)),
					updated_at = now()
			`, inventoryID, productID, variantID, sellerID)
			if err != nil {
				return fmt.Errorf("upsert inventory item %s: %w", s.slug, err)
			}

			_, err = tx.Exec(ctx, `
				INSERT INTO stock_movements (id, inventory_item_id, product_id, product_variant_id, seller_id, type, quantity, reason, actor_user_id, created_at)
				VALUES ($1, $2, $3, $4, $5, 'receipt', 25, 'Local dev seed stock', $6, now())
				ON CONFLICT (id) DO NOTHING
			`, movementID, inventoryID, productID, variantID, sellerID, adminID)
			if err != nil {
				return fmt.Errorf("upsert stock movement %s: %w", s.slug, err)
			}
		}
	}

	return nil
}

func upsertPaymentFixtures(ctx context.Context, tx postgres.DBTX, customerID uuid.UUID) error {
	// Predictable namespace for payment fixtures
	baseNS := uuid.MustParse("dddddddd-dddd-4ddd-8ddd-000000000000")

	for i := 1; i <= 15; i++ {
		// Stable UUIDs for this scenario
		orderID := uuid.NewMD5(baseNS, []byte(fmt.Sprintf("order-%d", i)))
		paymentID := uuid.NewMD5(baseNS, []byte(fmt.Sprintf("payment-%d", i)))
		orderNumber := fmt.Sprintf("ORD-DEV-P-%d", i)
		
		oStatus := "awaiting_payment"
		if i%2 == 0 { oStatus = "paid" }
		if i == 5 { oStatus = "awaiting_payment" } // for SUCCEEDED_PAYMENT_ORDER_NOT_PAID
		if i == 9 { oStatus = "paid" } // PAID_ORDER_WITHOUT_SUCCEEDED_PAYMENT
		
		_, err := tx.Exec(ctx, `
			INSERT INTO orders (id, user_id, order_number, status, total_price_cents, currency, delivery_address, delivery_method_name, delivery_price_cents, customer_name, customer_email, customer_phone, created_at, updated_at)
			VALUES ($1, $2, $3, $4, 100000, 'RUB', 'Test Address', 'Delivery', 0, 'Test User', 'test@example.com', '+79000000000', now(), now())
			ON CONFLICT (order_number) DO UPDATE SET status = EXCLUDED.status
		`, orderID, customerID, orderNumber, oStatus)
		if err != nil {
			return err
		}

		pStatus := "pending"
		pAmount := int64(100000)
		createdAt := time.Now()

		switch i {
		case 1: pStatus = "succeeded" // normal paid
		case 2: pStatus = "pending"; createdAt = time.Now().Add(-2 * time.Hour) // STUCK_PENDING
		case 3: pStatus = "failed"
		case 4: pStatus = "cancelled"
		case 5: pStatus = "succeeded" // SUCCEEDED_PAYMENT_ORDER_NOT_PAID
		case 6: pStatus = "succeeded" // partial refund
		case 7: pStatus = "succeeded" // pending refund
		case 8: pStatus = "succeeded" // full refund
		case 9: pStatus = "failed" // PAID_ORDER_WITHOUT_SUCCEEDED_PAYMENT
		case 10: pStatus = "succeeded" // SUCCEEDED_PAYMENT_ORDER_NOT_PAID
		case 11: pStatus = "succeeded"; pAmount = 50000 // AMOUNT_MISMATCH
		case 12: pStatus = "pending"; createdAt = time.Now().Add(-2 * time.Hour) // STUCK_PENDING
		case 13: pStatus = "pending" // INVALID_WEBHOOK_SIGNATURE
		case 14: pStatus = "pending" // UNPROCESSED_WEBHOOK
		case 15: pStatus = "succeeded" // MULTIPLE_SUCCEEDED
		default: pStatus = "pending"
		}

		_, err = tx.Exec(ctx, `
			INSERT INTO payments (id, order_id, provider, provider_payment_id, status, amount_cents, currency, payment_number, idempotency_key, integration_mode, payment_method, created_at) 
			VALUES ($1, $2, 'tbank', $3, $4, $5, 'RUB', $6, $7, 'mock', 'tpay', $8)
			ON CONFLICT (id) DO UPDATE SET status = EXCLUDED.status, amount_cents = EXCLUDED.amount_cents
		`, paymentID, orderID, fmt.Sprintf("T-DEV-%d", i), pStatus, pAmount, fmt.Sprintf("PAY-F-DEV-%d", i), fmt.Sprintf("idem-dev-%d", i), createdAt)
		if err != nil {
			return err
		}

		// Retries & Multiple Succeeded
		if i == 5 {
			_, err = tx.Exec(ctx, `
				INSERT INTO payments (id, order_id, provider, provider_payment_id, status, amount_cents, currency, payment_number, idempotency_key, integration_mode, payment_method, created_at) 
				VALUES ($1, $2, 'tbank', $3, 'failed', 100000, 'RUB', $4, $5, 'mock', 'tpay', now())
				ON CONFLICT (id) DO NOTHING
			`, uuid.NewMD5(baseNS, []byte("retry-5")), orderID, "T-DEV-5-FAIL", "PAY-F-DEV-5-FAIL", "idem-dev-5-fail")
			if err != nil { return err }
		}

		if i == 15 {
			_, err = tx.Exec(ctx, `
				INSERT INTO payments (id, order_id, provider, provider_payment_id, status, amount_cents, currency, payment_number, idempotency_key, integration_mode, payment_method, created_at) 
				VALUES ($1, $2, 'tbank', $3, 'succeeded', 100000, 'RUB', $4, $5, 'mock', 'tpay', now())
				ON CONFLICT (id) DO NOTHING
			`, uuid.NewMD5(baseNS, []byte("mult-15")), orderID, "T-DEV-15-EXTRA", "PAY-F-DEV-15-EXTRA", "idem-dev-15-extra")
			if err != nil { return err }
		}

		// Refunds
		if i == 6 {
			_, err = tx.Exec(ctx, `
				INSERT INTO refunds (id, payment_id, order_id, amount_cents, status, reason, currency, created_at, updated_at) 
				VALUES ($1, $2, $3, 30000, 'succeeded', 'partial return', 'RUB', now(), now())
				ON CONFLICT (id) DO NOTHING
			`, uuid.NewMD5(baseNS, []byte("refund-6")), paymentID, orderID)
			if err != nil { return err }
		}
		if i == 7 {
			_, err = tx.Exec(ctx, `
				INSERT INTO refunds (id, payment_id, order_id, amount_cents, status, reason, currency, created_at, updated_at) 
				VALUES ($1, $2, $3, 100000, 'pending', 'pending return', 'RUB', now(), now())
				ON CONFLICT (id) DO NOTHING
			`, uuid.NewMD5(baseNS, []byte("refund-7")), paymentID, orderID)
			if err != nil { return err }
		}
		if i == 8 {
			_, err = tx.Exec(ctx, `
				INSERT INTO refunds (id, payment_id, order_id, amount_cents, status, reason, currency, created_at, updated_at) 
				VALUES ($1, $2, $3, 100000, 'succeeded', 'full return', 'RUB', now(), now())
				ON CONFLICT (id) DO NOTHING
			`, uuid.NewMD5(baseNS, []byte("refund-8")), paymentID, orderID)
			if err != nil { return err }
		}

		// Payment Events
		if i == 13 {
			_, err = tx.Exec(ctx, `
				INSERT INTO payment_events (id, payment_id, provider, provider_payment_id, event_type, event_key, raw_payload, signature_valid, processed_at) 
				VALUES ($1, $2, 'tbank', 'T-DEV-13', 'AUTHORIZED', 'key-13', '{}', false, now())
				ON CONFLICT (id) DO NOTHING
			`, uuid.NewMD5(baseNS, []byte("evt-13")), paymentID)
			if err != nil { return err }
		}
		if i == 14 {
			_, err = tx.Exec(ctx, `
				INSERT INTO payment_events (id, payment_id, provider, provider_payment_id, event_type, event_key, raw_payload, signature_valid, processed_at) 
				VALUES ($1, $2, 'tbank', 'T-DEV-14', 'AUTHORIZED', 'key-14', '{}', true, NULL)
				ON CONFLICT (id) DO NOTHING
			`, uuid.NewMD5(baseNS, []byte("evt-14")), paymentID)
			if err != nil { return err }
		}
	}
	return nil
}

