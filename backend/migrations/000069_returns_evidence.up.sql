CREATE TABLE return_item_evidences (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    customer_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    return_item_id UUID REFERENCES return_items(id) ON DELETE CASCADE,
    storage_key VARCHAR NOT NULL,
    content_type VARCHAR NOT NULL,
    sort_order INT NOT NULL DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT timezone('utc', now())
);

CREATE INDEX idx_return_item_evidences_return_item_id ON return_item_evidences(return_item_id);
CREATE INDEX idx_return_item_evidences_customer_id ON return_item_evidences(customer_id);
