package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/config"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	ctx := context.Background()
	pg, err := postgres.NewClient(ctx, cfg.Postgres.DSN)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer pg.Close()

	sqlScript := `
	TRUNCATE fulfillment_receiving_scans, fulfillment_receiving_items, fulfillment_receiving_sessions CASCADE;
	DELETE FROM shipments WHERE fulfillment_id IN ('77777777-7777-4777-8777-333333333333', '77777777-7777-4777-8777-444444444444');

	-- Active Seller for fixtures
	INSERT INTO sellers (id, brand_name, slug, status, created_at, updated_at)
	VALUES (
		'11111111-1111-4111-8111-000000000001',
		'Fixture Active Seller',
		'fixture-active-seller',
		'active',
		NOW(),
		NOW()
	) ON CONFLICT (id) DO UPDATE SET status = 'active';

	-- Inactive Seller for fixtures
	INSERT INTO sellers (id, brand_name, slug, status, created_at, updated_at)
	VALUES (
		'33333333-3333-4333-8333-000000000003',
		'Fixture Suspended Seller',
		'fixture-suspended-seller',
		'blocked',
		NOW(),
		NOW()
	) ON CONFLICT (id) DO UPDATE SET status = 'blocked';

	-- 1. Published Visible
	INSERT INTO products (id, seller_id, title, slug, description, price_cents, currency, status, created_at, updated_at)
	VALUES (
		'11111111-1111-4111-8111-111111111111',
		'11111111-1111-4111-8111-000000000001',
		'Fixture Published Visible Product',
		'fixture-published-visible',
		'Full description for acceptance-published-visible',
		10000,
		'RUB',
		'published',
		NOW(),
		NOW()
	) ON CONFLICT (id) DO UPDATE SET status = 'published', price_cents = 10000, seller_id = '11111111-1111-4111-8111-000000000001';

	INSERT INTO product_variants (id, product_id, sku, barcode, price_cents, is_active, created_at, updated_at)
	VALUES ('11111111-1111-4111-8111-222222222222', '11111111-1111-4111-8111-111111111111', 'STAGE6A-SKU-01', '4601234567890', 10000, true, NOW(), NOW())
	ON CONFLICT (id) DO UPDATE SET is_active = true, price_cents = 10000, sku = 'STAGE6A-SKU-01', barcode = '4601234567890';

	INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock, created_at, updated_at)
	VALUES ('11111111-1111-4111-8111-333333333333', '11111111-1111-4111-8111-111111111111', '11111111-1111-4111-8111-222222222222', '11111111-1111-4111-8111-000000000001', 50, 0, NOW(), NOW())
	ON CONFLICT (product_variant_id) DO UPDATE SET total_stock = 50, reserved_stock = 0;

	-- 2. Hidden Eligible
	INSERT INTO products (id, seller_id, title, slug, description, price_cents, currency, status, created_at, updated_at)
	VALUES (
		'22222222-2222-4222-8222-222222222222',
		'11111111-1111-4111-8111-000000000001',
		'Fixture Hidden Eligible Product',
		'fixture-hidden-eligible',
		'Full description for acceptance-hidden-eligible',
		15000,
		'RUB',
		'hidden',
		NOW(),
		NOW()
	) ON CONFLICT (id) DO UPDATE SET status = 'hidden', price_cents = 15000, seller_id = '11111111-1111-4111-8111-000000000001';

	INSERT INTO product_variants (id, product_id, sku, price_cents, is_active, created_at, updated_at)
	VALUES ('22222222-2222-4222-8222-333333333333', '22222222-2222-4222-8222-222222222222', 'FIX-HID-01', 15000, true, NOW(), NOW())
	ON CONFLICT (id) DO UPDATE SET is_active = true, price_cents = 15000;

	INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock, created_at, updated_at)
	VALUES ('22222222-2222-4222-8222-444444444444', '22222222-2222-4222-8222-222222222222', '22222222-2222-4222-8222-333333333333', '11111111-1111-4111-8111-000000000001', 25, 0, NOW(), NOW())
	ON CONFLICT (product_variant_id) DO UPDATE SET total_stock = 25, reserved_stock = 0;

	-- 3. Hidden Seller Inactive
	INSERT INTO products (id, seller_id, title, slug, description, price_cents, currency, status, created_at, updated_at)
	VALUES (
		'33333333-3333-4333-8333-333333333333',
		'33333333-3333-4333-8333-000000000003',
		'Fixture Hidden Inactive Seller Product',
		'fixture-hidden-inactive-seller',
		'Full description for acceptance-hidden-seller-inactive',
		20000,
		'RUB',
		'hidden',
		NOW(),
		NOW()
	) ON CONFLICT (id) DO UPDATE SET status = 'hidden', price_cents = 20000, seller_id = '33333333-3333-4333-8333-000000000003';

	INSERT INTO product_variants (id, product_id, sku, price_cents, is_active, created_at, updated_at)
	VALUES ('33333333-3333-4333-8333-444444444444', '33333333-3333-4333-8333-333333333333', 'FIX-INACT-01', 20000, true, NOW(), NOW())
	ON CONFLICT (id) DO UPDATE SET is_active = true, price_cents = 20000;

	INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock, created_at, updated_at)
	VALUES ('33333333-3333-4333-8333-555555555555', '33333333-3333-4333-8333-333333333333', '33333333-3333-4333-8333-444444444444', '33333333-3333-4333-8333-000000000003', 10, 0, NOW(), NOW())
	ON CONFLICT (product_variant_id) DO UPDATE SET total_stock = 10, reserved_stock = 0;

	-- 4. Hidden No Inventory
	INSERT INTO products (id, seller_id, title, slug, description, price_cents, currency, status, created_at, updated_at)
	VALUES (
		'44444444-4444-4444-8444-444444444444',
		'11111111-1111-4111-8111-000000000001',
		'Fixture Hidden No Inventory Product',
		'fixture-hidden-no-inventory',
		'Full description for acceptance-hidden-no-inventory',
		30000,
		'RUB',
		'hidden',
		NOW(),
		NOW()
	) ON CONFLICT (id) DO UPDATE SET status = 'hidden', price_cents = 30000, seller_id = '11111111-1111-4111-8111-000000000001';

	INSERT INTO product_variants (id, product_id, sku, price_cents, is_active, created_at, updated_at)
	VALUES ('44444444-4444-4444-8444-555555555555', '44444444-4444-4444-8444-444444444444', 'FIX-NOINV-01', 30000, true, NOW(), NOW())
	ON CONFLICT (id) DO UPDATE SET is_active = true, price_cents = 30000;

	DELETE FROM inventory_items WHERE product_variant_id = '44444444-4444-4444-8444-555555555555';

	-- 5. Pending Moderation
	INSERT INTO products (id, seller_id, title, slug, description, price_cents, currency, status, created_at, updated_at)
	VALUES (
		'55555555-5555-4555-8555-555555555555',
		'11111111-1111-4111-8111-000000000001',
		'Fixture Pending Moderation Product',
		'fixture-pending-moderation',
		'Full description for acceptance-pending-moderation',
		25000,
		'RUB',
		'pending_moderation',
		NOW(),
		NOW()
	) ON CONFLICT (id) DO UPDATE SET status = 'pending_moderation', price_cents = 25000, seller_id = '11111111-1111-4111-8111-000000000001';

	INSERT INTO product_variants (id, product_id, sku, price_cents, is_active, created_at, updated_at)
	VALUES ('55555555-5555-4555-8555-666666666666', '55555555-5555-4555-8555-555555555555', 'FIX-PEND-01', 25000, true, NOW(), NOW())
	ON CONFLICT (id) DO UPDATE SET is_active = true, price_cents = 25000;

	INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock, created_at, updated_at)
	VALUES ('55555555-5555-4555-8555-777777777777', '55555555-5555-4555-8555-555555555555', '55555555-5555-4555-8555-666666666666', '11111111-1111-4111-8111-000000000001', 15, 0, NOW(), NOW())
	ON CONFLICT (product_variant_id) DO UPDATE SET total_stock = 15, reserved_stock = 0;
	DELETE FROM product_moderation_logs WHERE product_id IN (
		'11111111-1111-4111-8111-111111111111',
		'22222222-2222-4222-8222-222222222222',
		'33333333-3333-4333-8333-333333333333',
		'44444444-4444-4444-8444-444444444444',
		'55555555-5555-4555-8555-555555555555'
	);

	UPDATE products
	SET assigned_admin_user_id = NULL, review_started_at = NULL
	WHERE id IN (
		'11111111-1111-4111-8111-111111111111',
		'22222222-2222-4222-8222-222222222222',
		'33333333-3333-4333-8333-333333333333',
		'44444444-4444-4444-8444-444444444444',
		'55555555-5555-4555-8555-555555555555'
	);

	-- Stage 6A Deterministic Orders & Fulfillments Fixtures
	-- Customer user for orders
	INSERT INTO users (id, email, password_hash, role, name, created_at, updated_at)
	VALUES ('99999999-9999-4999-8999-999999999999', 'customer@zamk.local', '$2a$10$wT0...', 'customer', 'Иван Покупатель', NOW(), NOW())
	ON CONFLICT (id) DO NOTHING;

	-- Clean old test shipments & fulfillments & orders for fixtures
	DELETE FROM shipments WHERE order_id IN ('66666666-6666-4666-8666-111111111111','66666666-6666-4666-8666-222222222222','66666666-6666-4666-8666-333333333333','66666666-6666-4666-8666-444444444444','66666666-6666-4666-8666-555555555555','66666666-6666-4666-8666-666666666666','66666666-6666-4666-8666-777777777777','66666666-6666-4666-8666-888888888888');
	DELETE FROM order_items WHERE order_id IN ('66666666-6666-4666-8666-111111111111','66666666-6666-4666-8666-222222222222','66666666-6666-4666-8666-333333333333','66666666-6666-4666-8666-444444444444','66666666-6666-4666-8666-555555555555','66666666-6666-4666-8666-666666666666','66666666-6666-4666-8666-777777777777','66666666-6666-4666-8666-888888888888');
	DELETE FROM order_fulfillments WHERE order_id IN ('66666666-6666-4666-8666-111111111111','66666666-6666-4666-8666-222222222222','66666666-6666-4666-8666-333333333333','66666666-6666-4666-8666-444444444444','66666666-6666-4666-8666-555555555555','66666666-6666-4666-8666-666666666666','66666666-6666-4666-8666-777777777777','66666666-6666-4666-8666-888888888888');
	DELETE FROM orders WHERE id IN ('66666666-6666-4666-8666-111111111111','66666666-6666-4666-8666-222222222222','66666666-6666-4666-8666-333333333333','66666666-6666-4666-8666-444444444444','66666666-6666-4666-8666-555555555555','66666666-6666-4666-8666-666666666666','66666666-6666-4666-8666-777777777777','66666666-6666-4666-8666-888888888888');

	-- Fixture 1: 1 Seller, assembling
	INSERT INTO orders (id, user_id, status, total_price_cents, currency, customer_name, customer_phone, customer_email, delivery_address, order_number, created_at, updated_at)
	VALUES ('66666666-6666-4666-8666-111111111111', '99999999-9999-4999-8999-999999999999', 'paid', 10500, 'RUB', 'Иван Тестов', '+79991112233', 'ivan@demo.zamk.local', 'Москва, Тверская 1', 'ZMK-000101', NOW(), NOW());

	INSERT INTO order_fulfillments (id, order_id, seller_id, status, subtotal_cents, commission_bps, seller_amount_cents, created_at, updated_at)
	VALUES ('77777777-7777-4777-8777-111111111111', '66666666-6666-4666-8666-111111111111', '11111111-1111-4111-8111-000000000001', 'assembling', 10000, 1500, 8500, NOW(), NOW());

	INSERT INTO order_items (id, order_id, order_fulfillment_id, product_id, product_variant_id, seller_id, title, product_slug, sku, price_cents, quantity, subtotal_price_cents, created_at)
	VALUES ('88888888-8888-4888-8888-111111111111', '66666666-6666-4666-8666-111111111111', '77777777-7777-4777-8777-111111111111', '11111111-1111-4111-8111-111111111111', '11111111-1111-4111-8111-222222222222', '11111111-1111-4111-8111-000000000001', 'Fixture Published Visible Product', 'fixture-published-visible', 'STAGE6A-SKU-01', 10000, 1, 10000, NOW());

	-- Fixture 3: Packed fulfillment ready for receiving
	INSERT INTO orders (id, user_id, status, total_price_cents, currency, customer_name, customer_phone, customer_email, delivery_address, order_number, created_at, updated_at)
	VALUES ('66666666-6666-4666-8666-333333333333', '99999999-9999-4999-8999-999999999999', 'paid', 15000, 'RUB', 'Анна Смирнова', '+79993334455', 'anna@demo.zamk.local', 'Санкт-Петербург, Невский 10', 'ZMK-000103', NOW(), NOW());

	INSERT INTO order_fulfillments (id, order_id, seller_id, status, subtotal_cents, commission_bps, seller_amount_cents, receiving_code, receiving_qr_token, packed_at, created_at, updated_at)
	VALUES ('77777777-7777-4777-8777-333333333333', '66666666-6666-4666-8666-333333333333', '11111111-1111-4111-8111-000000000001', 'packed', 15000, 1500, 12750, 'FUL-2026-REC001', 'REC001TOKEN', NOW(), NOW(), NOW());

	INSERT INTO order_items (id, order_id, order_fulfillment_id, product_id, product_variant_id, seller_id, title, product_slug, sku, price_cents, quantity, subtotal_price_cents, created_at)
	VALUES ('88888888-8888-4888-8888-333333333333', '66666666-6666-4666-8666-333333333333', '77777777-7777-4777-8777-333333333333', '11111111-1111-4111-8111-111111111111', '11111111-1111-4111-8111-222222222222', '11111111-1111-4111-8111-000000000001', 'Fixture Published Visible Product', 'fixture-published-visible', 'STAGE6A-SKU-01', 15000, 1, 15000, NOW());

	-- Fixture 4: Fulfillment ready for discrepancy test
	INSERT INTO orders (id, user_id, status, total_price_cents, currency, customer_name, customer_phone, customer_email, delivery_address, order_number, created_at, updated_at)
	VALUES ('66666666-6666-4666-8666-444444444444', '99999999-9999-4999-8999-999999999999', 'paid', 30000, 'RUB', 'Павел Кузнецов', '+79994445566', 'pavel@demo.zamk.local', 'Казань, Баумана 5', 'ZMK-000104', NOW(), NOW());

	INSERT INTO order_fulfillments (id, order_id, seller_id, status, subtotal_cents, commission_bps, seller_amount_cents, receiving_code, receiving_qr_token, packed_at, created_at, updated_at)
	VALUES ('77777777-7777-4777-8777-444444444444', '66666666-6666-4666-8666-444444444444', '11111111-1111-4111-8111-000000000001', 'packed', 30000, 1500, 25500, 'FUL-2026-DISC01', 'DISC01TOKEN', NOW(), NOW(), NOW());

	INSERT INTO order_items (id, order_id, order_fulfillment_id, product_id, product_variant_id, seller_id, title, product_slug, sku, price_cents, quantity, subtotal_price_cents, created_at)
	VALUES ('88888888-8888-4888-8888-444444444444', '66666666-6666-4666-8666-444444444444', '77777777-7777-4777-8777-444444444444', '11111111-1111-4111-8111-111111111111', '11111111-1111-4111-8111-222222222222', '11111111-1111-4111-8111-000000000001', 'Fixture Published Visible Product', 'fixture-published-visible', 'STAGE6A-SKU-01', 15000, 2, 30000, NOW());

	-- Fixture 5: Accepted fulfillment without shipment (problem state)
	INSERT INTO orders (id, user_id, status, total_price_cents, currency, customer_name, customer_phone, customer_email, delivery_address, order_number, created_at, updated_at)
	VALUES ('66666666-6666-4666-8666-555555555555', '99999999-9999-4999-8999-999999999999', 'paid', 20000, 'RUB', 'Ольга Васильева', '+79995556677', 'olga@demo.zamk.local', 'Новосибирск, Ленина 12', 'ZMK-000105', NOW(), NOW());

	INSERT INTO order_fulfillments (id, order_id, seller_id, status, subtotal_cents, commission_bps, seller_amount_cents, receiving_code, receiving_qr_token, accepted_at, created_at, updated_at)
	VALUES ('77777777-7777-4777-8777-555555555555', '66666666-6666-4666-8666-555555555555', '11111111-1111-4111-8111-000000000001', 'accepted', 20000, 1500, 17000, 'FUL-2026-NOSHIP01', 'NOSHIP01TOKEN', NOW(), NOW(), NOW());

	INSERT INTO order_items (id, order_id, order_fulfillment_id, product_id, product_variant_id, seller_id, title, product_slug, sku, price_cents, quantity, subtotal_price_cents, created_at)
	VALUES ('88888888-8888-4888-8888-555555555555', '66666666-6666-4666-8666-555555555555', '77777777-7777-4777-8777-555555555555', '11111111-1111-4111-8111-111111111111', '11111111-1111-4111-8111-222222222222', '11111111-1111-4111-8111-000000000001', 'Fixture Published Visible Product', 'fixture-published-visible', 'STAGE6A-SKU-01', 20000, 1, 20000, NOW());

	-- Fixture 7: Cancelled order
	INSERT INTO orders (id, user_id, status, total_price_cents, currency, customer_name, customer_phone, customer_email, delivery_address, order_number, created_at, updated_at)
	VALUES ('66666666-6666-4666-8666-777777777777', '99999999-9999-4999-8999-999999999999', 'cancelled', 12000, 'RUB', 'Елена Соколова', '+79997778899', 'elena@demo.zamk.local', 'Екатеринбург, Мира 20', 'ZMK-000107', NOW(), NOW());

	INSERT INTO order_fulfillments (id, order_id, seller_id, status, subtotal_cents, commission_bps, seller_amount_cents, created_at, updated_at)
	VALUES ('77777777-7777-4777-8777-777777777777', '66666666-6666-4666-8666-777777777777', '11111111-1111-4111-8111-000000000001', 'cancelled', 12000, 1500, 10200, NOW(), NOW());

	-- Fixture 8: Multi-seller order (2 sellers, 2 items, 2 fulfillments)
	INSERT INTO orders (id, user_id, status, total_price_cents, currency, customer_name, customer_phone, customer_email, delivery_address, order_number, created_at, updated_at)
	VALUES ('66666666-6666-4666-8666-888888888888', '99999999-9999-4999-8999-999999999999', 'paid', 25000, 'RUB', 'Дмитрий Иванов', '+79998887766', 'dmitry@demo.zamk.local', 'Самара, Лесная 3', 'ZMK-000108', NOW(), NOW());

	-- Seller A (Active Seller), assembling
	INSERT INTO order_fulfillments (id, order_id, seller_id, status, subtotal_cents, commission_bps, seller_amount_cents, created_at, updated_at)
	VALUES ('77777777-7777-4777-8777-888888888881', '66666666-6666-4666-8666-888888888888', '11111111-1111-4111-8111-000000000001', 'assembling', 10000, 1500, 8500, NOW(), NOW());

	INSERT INTO order_items (id, order_id, order_fulfillment_id, product_id, product_variant_id, seller_id, title, product_slug, sku, price_cents, quantity, subtotal_price_cents, created_at)
	VALUES ('88888888-8888-4888-8888-888888888881', '66666666-6666-4666-8666-888888888888', '77777777-7777-4777-8777-888888888881', '11111111-1111-4111-8111-111111111111', '11111111-1111-4111-8111-222222222222', '11111111-1111-4111-8111-000000000001', 'Fixture Published Visible Product', 'fixture-published-visible', 'STAGE6A-SKU-01', 10000, 1, 10000, NOW());

	-- Seller B (Suspended Seller - just to use the other one), packed with receivingCode
	INSERT INTO order_fulfillments (id, order_id, seller_id, status, subtotal_cents, commission_bps, seller_amount_cents, receiving_code, receiving_qr_token, packed_at, created_at, updated_at)
	VALUES ('77777777-7777-4777-8777-888888888882', '66666666-6666-4666-8666-888888888888', '33333333-3333-4333-8333-000000000003', 'packed', 15000, 1500, 12750, 'FUL-2026-MULTI01', 'MULTI01TOKEN', NOW(), NOW(), NOW());

	INSERT INTO order_items (id, order_id, order_fulfillment_id, product_id, product_variant_id, seller_id, title, product_slug, sku, price_cents, quantity, subtotal_price_cents, created_at)
	VALUES ('88888888-8888-4888-8888-888888888882', '66666666-6666-4666-8666-888888888888', '77777777-7777-4777-8777-888888888882', '22222222-2222-4222-8222-222222222222', '22222222-2222-4222-8222-333333333333', '33333333-3333-4333-8333-000000000003', 'Fixture Hidden Eligible Product', 'fixture-hidden-eligible', 'FIX-HID-01', 7500, 2, 15000, NOW());
	`

	_, err = pg.Pool.Exec(ctx, sqlScript)
	if err != nil {
		log.Fatalf("Failed to seed fixture data: %v", err)
	}

	fmt.Println("Successfully seeded Stage 6A deterministic fixtures!")
	os.Exit(0)
}
