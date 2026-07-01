CREATE INDEX IF NOT EXISTS user_sessions_refresh_token_hash_idx ON user_sessions(refresh_token_hash);

CREATE INDEX IF NOT EXISTS products_status_created_at_idx ON products(status, created_at DESC);
CREATE INDEX IF NOT EXISTS products_seller_id_status_idx ON products(seller_id, status);
CREATE INDEX IF NOT EXISTS products_status_published_at_idx ON products(status, published_at DESC);
CREATE INDEX IF NOT EXISTS products_status_submitted_at_idx ON products(status, submitted_at ASC);

CREATE INDEX IF NOT EXISTS stock_movements_inventory_item_id_created_at_idx ON stock_movements(inventory_item_id, created_at DESC);
CREATE INDEX IF NOT EXISTS stock_movements_product_variant_id_created_at_idx ON stock_movements(product_variant_id, created_at DESC);
CREATE INDEX IF NOT EXISTS reservations_user_id_status_idx ON reservations(user_id, status);
CREATE INDEX IF NOT EXISTS reservations_status_expires_at_idx ON reservations(status, expires_at);
CREATE INDEX IF NOT EXISTS reservations_order_id_idx ON reservations(order_id);

CREATE INDEX IF NOT EXISTS orders_user_id_idx ON orders(user_id);
CREATE INDEX IF NOT EXISTS orders_status_idx ON orders(status);
CREATE INDEX IF NOT EXISTS orders_created_at_idx ON orders(created_at DESC);
CREATE INDEX IF NOT EXISTS orders_user_id_created_at_idx ON orders(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS orders_status_created_at_idx ON orders(status, created_at);
CREATE INDEX IF NOT EXISTS order_items_order_id_idx ON order_items(order_id);
CREATE INDEX IF NOT EXISTS order_items_seller_id_idx ON order_items(seller_id);
CREATE INDEX IF NOT EXISTS order_items_product_id_idx ON order_items(product_id);
CREATE INDEX IF NOT EXISTS order_items_seller_id_order_id_idx ON order_items(seller_id, order_id);
CREATE INDEX IF NOT EXISTS order_reservations_order_id_idx ON order_reservations(order_id);
CREATE INDEX IF NOT EXISTS order_status_history_order_id_created_at_idx ON order_status_history(order_id, created_at DESC);

CREATE INDEX IF NOT EXISTS payments_order_id_idx ON payments(order_id);
CREATE INDEX IF NOT EXISTS payments_status_idx ON payments(status);
CREATE INDEX IF NOT EXISTS payments_provider_payment_id_idx ON payments(provider_payment_id);
CREATE INDEX IF NOT EXISTS payments_provider_provider_payment_id_idx ON payments(provider, provider_payment_id);
CREATE INDEX IF NOT EXISTS payment_events_payment_id_idx ON payment_events(payment_id);
CREATE INDEX IF NOT EXISTS payment_events_provider_payment_id_idx ON payment_events(provider_payment_id);

CREATE INDEX IF NOT EXISTS shipments_order_id_idx ON shipments(order_id);
CREATE INDEX IF NOT EXISTS shipments_status_idx ON shipments(status);
CREATE INDEX IF NOT EXISTS shipments_created_at_idx ON shipments(created_at DESC);
CREATE INDEX IF NOT EXISTS shipment_events_shipment_id_created_at_idx ON shipment_events(shipment_id, created_at DESC);

CREATE INDEX IF NOT EXISTS returns_order_id_idx ON returns(order_id);
CREATE INDEX IF NOT EXISTS returns_user_id_idx ON returns(user_id);
CREATE INDEX IF NOT EXISTS returns_status_idx ON returns(status);
CREATE INDEX IF NOT EXISTS returns_created_at_idx ON returns(created_at DESC);
CREATE INDEX IF NOT EXISTS returns_user_id_created_at_idx ON returns(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS return_items_return_id_idx ON return_items(return_id);
CREATE INDEX IF NOT EXISTS return_items_order_item_id_idx ON return_items(order_item_id);
CREATE INDEX IF NOT EXISTS refunds_order_id_idx ON refunds(order_id);
CREATE INDEX IF NOT EXISTS refunds_return_id_idx ON refunds(return_id);
CREATE INDEX IF NOT EXISTS refunds_status_idx ON refunds(status);
CREATE INDEX IF NOT EXISTS refunds_created_at_idx ON refunds(created_at DESC);

CREATE INDEX IF NOT EXISTS seller_balance_ledger_seller_id_created_at_idx ON seller_balance_ledger(seller_id, created_at DESC);
CREATE INDEX IF NOT EXISTS seller_balance_ledger_seller_id_type_idx ON seller_balance_ledger(seller_id, type);
CREATE INDEX IF NOT EXISTS seller_balance_ledger_order_item_id_type_idx ON seller_balance_ledger(order_item_id, type);
CREATE INDEX IF NOT EXISTS seller_balance_ledger_type_available_at_idx ON seller_balance_ledger(type, available_at);
CREATE INDEX IF NOT EXISTS payouts_status_idx ON payouts(status);
CREATE INDEX IF NOT EXISTS payouts_created_at_idx ON payouts(created_at DESC);
CREATE INDEX IF NOT EXISTS payouts_seller_id_created_at_idx ON payouts(seller_id, created_at DESC);

CREATE INDEX IF NOT EXISTS product_reviews_product_id_status_idx ON product_reviews(product_id, status);
CREATE INDEX IF NOT EXISTS product_reviews_user_id_idx ON product_reviews(user_id);
CREATE INDEX IF NOT EXISTS product_reviews_seller_id_idx ON product_reviews(seller_id);
CREATE INDEX IF NOT EXISTS product_reviews_status_idx ON product_reviews(status);
CREATE INDEX IF NOT EXISTS product_reviews_created_at_idx ON product_reviews(created_at DESC);
CREATE INDEX IF NOT EXISTS product_review_moderation_logs_review_id_created_at_idx ON product_review_moderation_logs(review_id, created_at DESC);
