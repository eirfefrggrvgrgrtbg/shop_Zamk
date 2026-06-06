DROP INDEX IF EXISTS product_review_moderation_logs_review_id_created_at_idx;
DROP INDEX IF EXISTS product_reviews_created_at_idx;
DROP INDEX IF EXISTS product_reviews_status_idx;
DROP INDEX IF EXISTS product_reviews_seller_id_idx;
DROP INDEX IF EXISTS product_reviews_user_id_idx;
DROP INDEX IF EXISTS product_reviews_product_id_status_idx;

DROP INDEX IF EXISTS payouts_seller_id_created_at_idx;
DROP INDEX IF EXISTS payouts_created_at_idx;
DROP INDEX IF EXISTS payouts_status_idx;
DROP INDEX IF EXISTS seller_balance_ledger_type_available_at_idx;
DROP INDEX IF EXISTS seller_balance_ledger_order_item_id_type_idx;
DROP INDEX IF EXISTS seller_balance_ledger_seller_id_type_idx;
DROP INDEX IF EXISTS seller_balance_ledger_seller_id_created_at_idx;

DROP INDEX IF EXISTS refunds_created_at_idx;
DROP INDEX IF EXISTS refunds_status_idx;
DROP INDEX IF EXISTS refunds_return_id_idx;
DROP INDEX IF EXISTS refunds_order_id_idx;
DROP INDEX IF EXISTS return_items_order_item_id_idx;
DROP INDEX IF EXISTS return_items_return_id_idx;
DROP INDEX IF EXISTS returns_user_id_created_at_idx;
DROP INDEX IF EXISTS returns_created_at_idx;
DROP INDEX IF EXISTS returns_status_idx;
DROP INDEX IF EXISTS returns_user_id_idx;
DROP INDEX IF EXISTS returns_order_id_idx;

DROP INDEX IF EXISTS shipment_events_shipment_id_created_at_idx;
DROP INDEX IF EXISTS shipments_created_at_idx;
DROP INDEX IF EXISTS shipments_status_idx;
DROP INDEX IF EXISTS shipments_order_id_idx;

DROP INDEX IF EXISTS payment_events_provider_payment_id_idx;
DROP INDEX IF EXISTS payment_events_payment_id_idx;
DROP INDEX IF EXISTS payments_provider_provider_payment_id_idx;
DROP INDEX IF EXISTS payments_provider_payment_id_idx;
DROP INDEX IF EXISTS payments_status_idx;
DROP INDEX IF EXISTS payments_order_id_idx;

DROP INDEX IF EXISTS order_status_history_order_id_created_at_idx;
DROP INDEX IF EXISTS order_reservations_order_id_idx;
DROP INDEX IF EXISTS order_items_seller_id_order_id_idx;
DROP INDEX IF EXISTS order_items_product_id_idx;
DROP INDEX IF EXISTS order_items_seller_id_idx;
DROP INDEX IF EXISTS order_items_order_id_idx;
DROP INDEX IF EXISTS orders_status_created_at_idx;
DROP INDEX IF EXISTS orders_user_id_created_at_idx;
DROP INDEX IF EXISTS orders_created_at_idx;
DROP INDEX IF EXISTS orders_status_idx;
DROP INDEX IF EXISTS orders_user_id_idx;

DROP INDEX IF EXISTS reservations_order_id_idx;
DROP INDEX IF EXISTS reservations_status_expires_at_idx;
DROP INDEX IF EXISTS reservations_user_id_status_idx;
DROP INDEX IF EXISTS stock_movements_product_variant_id_created_at_idx;
DROP INDEX IF EXISTS stock_movements_inventory_item_id_created_at_idx;

DROP INDEX IF EXISTS products_status_submitted_at_idx;
DROP INDEX IF EXISTS products_status_published_at_idx;
DROP INDEX IF EXISTS products_seller_id_status_idx;
DROP INDEX IF EXISTS products_status_created_at_idx;

DROP INDEX IF EXISTS user_sessions_refresh_token_hash_idx;
