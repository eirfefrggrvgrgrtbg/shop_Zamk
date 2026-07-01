CREATE UNIQUE INDEX IF NOT EXISTS idx_shipments_unique_fulfillment ON shipments(fulfillment_id) WHERE fulfillment_id IS NOT NULL;
