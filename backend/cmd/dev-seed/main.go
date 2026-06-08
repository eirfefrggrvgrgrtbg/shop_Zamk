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
		productID, err := upsertProduct(ctx, tx, sellerID, categoryID, brandID)
		if err != nil {
			return err
		}
		variantID, err := upsertVariant(ctx, tx, productID)
		if err != nil {
			return err
		}
		if err := upsertProductImage(ctx, tx, productID); err != nil {
			return err
		}
		if err := upsertInventory(ctx, tx, productID, variantID, sellerID, adminID); err != nil {
			return err
		}

		logger.Info("local dev seed ready", "adminUserId", adminID, "sellerUserId", sellerUserID, "customerUserId", customerID, "sellerId", sellerID, "productId", productID)
		return nil
	}); err != nil {
		logger.Error("failed to seed local dev data", "error", err)
		os.Exit(1)
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
		ON CONFLICT (email) DO UPDATE SET
			name = EXCLUDED.name,
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

func upsertProduct(ctx context.Context, tx postgres.DBTX, sellerID, categoryID, brandID uuid.UUID) (uuid.UUID, error) {
	var productID uuid.UUID
	err := tx.QueryRow(ctx, `
		INSERT INTO products (
			id, seller_id, category_id, brand_id, title, slug, description, status, gender, color, material,
			care_instructions, price_cents, old_price_cents, currency, main_image_url, created_at, updated_at,
			submitted_at, approved_at, published_at, moderation_comment
		)
		VALUES (
			$1, $2, $3, $4, 'Dev Wool Coat', 'dev-wool-coat',
			'Minimal published product seeded for local manual testing.', 'published', 'unisex', 'Graphite',
			'Wool blend', 'Dry clean only', 1299000, 1599000, 'RUB', $5, now(), now(), now(), now(), now(),
			'Seeded for local development'
		)
		ON CONFLICT (slug) DO UPDATE SET
			seller_id = EXCLUDED.seller_id,
			category_id = EXCLUDED.category_id,
			brand_id = EXCLUDED.brand_id,
			title = EXCLUDED.title,
			description = EXCLUDED.description,
			status = 'published',
			gender = EXCLUDED.gender,
			color = EXCLUDED.color,
			material = EXCLUDED.material,
			care_instructions = EXCLUDED.care_instructions,
			price_cents = EXCLUDED.price_cents,
			old_price_cents = EXCLUDED.old_price_cents,
			main_image_url = EXCLUDED.main_image_url,
			approved_at = COALESCE(products.approved_at, now()),
			published_at = COALESCE(products.published_at, now()),
			moderation_comment = EXCLUDED.moderation_comment,
			updated_at = now()
		RETURNING id
	`, seedIDs.Product, sellerID, categoryID, brandID, "https://placehold.co/900x1200?text=Dev+Product").Scan(&productID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("upsert product: %w", err)
	}
	return productID, nil
}

func upsertVariant(ctx context.Context, tx postgres.DBTX, productID uuid.UUID) (uuid.UUID, error) {
	var variantID uuid.UUID
	err := tx.QueryRow(ctx, `
		INSERT INTO product_variants (id, product_id, sku, size, color, barcode, price_cents, is_active, created_at, updated_at)
		VALUES ($1, $2, 'DEV-COAT-M-GRAPHITE', 'M', 'Graphite', '000000000001', 1299000, true, now(), now())
		ON CONFLICT (id) DO UPDATE SET
			product_id = EXCLUDED.product_id,
			sku = EXCLUDED.sku,
			size = EXCLUDED.size,
			color = EXCLUDED.color,
			barcode = EXCLUDED.barcode,
			price_cents = EXCLUDED.price_cents,
			is_active = true,
			updated_at = now()
		RETURNING id
	`, seedIDs.Variant, productID).Scan(&variantID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("upsert variant: %w", err)
	}
	return variantID, nil
}

func upsertProductImage(ctx context.Context, tx postgres.DBTX, productID uuid.UUID) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO product_images (id, product_id, image_url, alt_text, sort_order, created_at)
		VALUES ($1, $2, $3, 'Dev Wool Coat placeholder image', 0, now())
		ON CONFLICT (id) DO UPDATE SET
			product_id = EXCLUDED.product_id,
			image_url = EXCLUDED.image_url,
			alt_text = EXCLUDED.alt_text,
			sort_order = EXCLUDED.sort_order
	`, seedIDs.Image, productID, "https://placehold.co/900x1200?text=Dev+Product")
	if err != nil {
		return fmt.Errorf("upsert product image: %w", err)
	}
	return nil
}

func upsertInventory(ctx context.Context, tx postgres.DBTX, productID, variantID, sellerID, actorUserID uuid.UUID) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 25, 0, now(), now())
		ON CONFLICT (product_variant_id) DO UPDATE SET
			product_id = EXCLUDED.product_id,
			seller_id = EXCLUDED.seller_id,
			total_stock = GREATEST(inventory_items.total_stock, 25),
			reserved_stock = LEAST(inventory_items.reserved_stock, GREATEST(inventory_items.total_stock, 25)),
			updated_at = now()
	`, seedIDs.Inventory, productID, variantID, sellerID)
	if err != nil {
		return fmt.Errorf("upsert inventory item: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO stock_movements (id, inventory_item_id, product_id, product_variant_id, seller_id, type, quantity, reason, actor_user_id, created_at)
		VALUES ($1, $2, $3, $4, $5, 'receipt', 25, 'Local dev seed stock', $6, now())
		ON CONFLICT (id) DO NOTHING
	`, seedIDs.Movement, seedIDs.Inventory, productID, variantID, sellerID, actorUserID)
	if err != nil {
		return fmt.Errorf("upsert stock movement: %w", err)
	}
	return nil
}
