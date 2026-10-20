-- WARNING: dropping the flag after orders with a frozen rate exist makes accrual of their
-- not yet delivered fulfillments fall back to the rate in force at delivery.
ALTER TABLE order_fulfillments
    DROP COLUMN IF EXISTS commission_frozen;
