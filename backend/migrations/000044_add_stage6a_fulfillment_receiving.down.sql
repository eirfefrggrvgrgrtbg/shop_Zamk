ALTER TABLE orders DROP COLUMN IF EXISTS order_number;

ALTER TABLE order_fulfillments
DROP COLUMN IF EXISTS receiving_code,
DROP COLUMN IF EXISTS receiving_qr_token,
DROP COLUMN IF EXISTS packed_at,
DROP COLUMN IF EXISTS accepted_at,
DROP COLUMN IF EXISTS accepted_by_staff_id,
DROP COLUMN IF EXISTS receiving_result,
DROP COLUMN IF EXISTS discrepancy_reason,
DROP COLUMN IF EXISTS discrepancy_comment,
DROP COLUMN IF EXISTS discrepancy_at;
