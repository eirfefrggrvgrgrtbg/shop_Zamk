-- Migration 000098: Harden promotion allocations and first order concurrency
-- Up:
-- 1. Relax order_item_promotions divisibility constraints to allow cent-exact fixed discount allocation across quantities > 1
-- 2. Add is_first_order and unique partial indexes to promo_code_usages for race-free first-paid-order enforcement

ALTER TABLE order_item_promotions DROP CONSTRAINT IF EXISTS chk_oip_total_seller_discount;
ALTER TABLE order_item_promotions DROP CONSTRAINT IF EXISTS chk_oip_total_customer_paid;
ALTER TABLE order_item_promotions DROP CONSTRAINT IF EXISTS chk_oip_total_comm_base;
ALTER TABLE order_item_promotions DROP CONSTRAINT IF EXISTS chk_oip_line_totals;

ALTER TABLE order_item_promotions ADD CONSTRAINT chk_oip_line_totals CHECK (
    total_customer_paid_cents = (base_unit_price_cents * quantity) - total_seller_discount_cents - total_zamk_subsidy_cents AND
    total_commission_base_cents = (base_unit_price_cents * quantity) - total_seller_discount_cents AND
    total_seller_discount_cents <= (base_unit_price_cents * quantity) AND
    total_zamk_subsidy_cents <= (base_unit_price_cents * quantity) - total_seller_discount_cents
);

ALTER TABLE promo_code_usages ADD COLUMN IF NOT EXISTS is_first_order BOOLEAN NOT NULL DEFAULT false;

DROP INDEX IF EXISTS uq_first_order_reserved_per_customer;
DROP INDEX IF EXISTS uq_first_order_consumed_per_customer;

CREATE UNIQUE INDEX IF NOT EXISTS uq_first_order_active_per_customer
ON promo_code_usages (user_id)
WHERE is_first_order = true AND status IN ('reserved', 'consumed');

CREATE TABLE IF NOT EXISTS customer_first_payment_claims (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    order_id UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE payments ADD COLUMN IF NOT EXISTS init_outcome TEXT NOT NULL DEFAULT 'successful';
ALTER TABLE payments ADD COLUMN IF NOT EXISTS reconciliation_attempted_at TIMESTAMPTZ;

WITH duplicates AS (
    SELECT id, ROW_NUMBER() OVER (PARTITION BY provider, provider_payment_id ORDER BY created_at DESC) as rn
    FROM payments
    WHERE provider_payment_id IS NOT NULL AND provider_payment_id != ''
)
UPDATE payments
SET provider_payment_id = provider_payment_id || '-' || substr(id::text, 1, 8)
WHERE id IN (SELECT id FROM duplicates WHERE rn > 1);

CREATE UNIQUE INDEX IF NOT EXISTS uq_payments_provider_payment_id
ON payments(provider, provider_payment_id)
WHERE provider_payment_id IS NOT NULL AND provider_payment_id != '';
