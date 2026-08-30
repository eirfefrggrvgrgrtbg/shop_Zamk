BEGIN;

CREATE TABLE return_staged_message_attachments (
    id UUID PRIMARY KEY,
    return_id UUID NOT NULL REFERENCES returns(id) ON DELETE CASCADE,
    uploader_user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    storage_key TEXT NOT NULL,
    content_type TEXT NOT NULL,
    size_bytes BIGINT NOT NULL,
    original_filename TEXT,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE TABLE return_message_attachments (
    id UUID PRIMARY KEY,
    message_id UUID NOT NULL REFERENCES return_messages(id) ON DELETE CASCADE,
    storage_key TEXT NOT NULL,
    content_type TEXT NOT NULL,
    size_bytes BIGINT NOT NULL,
    original_filename TEXT,
    sort_order INT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_return_message_attachments_message_id_sort_order ON return_message_attachments(message_id, sort_order);
CREATE INDEX idx_return_staged_message_attachments_return_id ON return_staged_message_attachments(return_id);

COMMIT;
