BEGIN;

CREATE TABLE return_messages (
    id UUID PRIMARY KEY,
    return_id UUID NOT NULL REFERENCES returns(id) ON DELETE RESTRICT,
    sender_user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    sender_role VARCHAR(50) NOT NULL CHECK (sender_role IN ('customer', 'admin')),
    message_type VARCHAR(50) NOT NULL CHECK (message_type IN ('message', 'info_request')),
    body TEXT NOT NULL CHECK (length(trim(body)) > 0),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_return_messages_return_id_created_at_id ON return_messages(return_id, created_at, id);

ALTER TABLE returns DROP CONSTRAINT IF EXISTS valid_return_status;
ALTER TABLE returns ADD CONSTRAINT valid_return_status CHECK (status IN ('requested', 'needs_info', 'approved', 'receiving', 'item_received', 'refunded', 'completed', 'rejected', 'cancelled'));

COMMIT;
