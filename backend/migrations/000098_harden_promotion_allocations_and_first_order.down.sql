-- Migration 000098 Down: Revert line total constraints and first order concurrency indexes

DROP INDEX IF EXISTS uq_payments_provider_payment_id;
ALTER TABLE payments DROP COLUMN IF EXISTS reconciliation_attempted_at;
ALTER TABLE payments DROP COLUMN IF EXISTS init_outcome;
DROP TABLE IF EXISTS customer_first_payment_claims;
DROP INDEX IF EXISTS uq_first_order_active_per_customer;
DROP INDEX IF EXISTS uq_first_order_consumed_per_customer;
DROP INDEX IF EXISTS uq_first_order_reserved_per_customer;
ALTER TABLE promo_code_usages DROP COLUMN IF EXISTS is_first_order;

ALTER TABLE order_item_promotions DROP CONSTRAINT IF EXISTS chk_oip_line_totals;
ALTER TABLE order_item_promotions ADD CONSTRAINT chk_oip_total_seller_discount CHECK (total_seller_discount_cents = seller_discount_unit_cents * quantity);
ALTER TABLE order_item_promotions ADD CONSTRAINT chk_oip_total_customer_paid CHECK (total_customer_paid_cents = customer_paid_unit_price_cents * quantity);
ALTER TABLE order_item_promotions ADD CONSTRAINT chk_oip_total_comm_base CHECK (total_commission_base_cents = commission_base_unit_cents * quantity);
