-- Migration 000099 Down: Revert promotion product scope, targets, and maximum discount cap
DROP TABLE IF EXISTS promo_code_product_targets;

ALTER TABLE promo_codes
    DROP CONSTRAINT IF EXISTS chk_pc_max_discount,
    DROP COLUMN IF EXISTS max_discount_cents,
    DROP CONSTRAINT IF EXISTS chk_pc_product_scope,
    DROP COLUMN IF EXISTS product_scope;
