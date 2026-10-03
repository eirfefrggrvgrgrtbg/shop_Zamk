CREATE TABLE IF NOT EXISTS support_categories (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    requester_scope TEXT NOT NULL CHECK (requester_scope IN ('CUSTOMER', 'SELLER', 'BOTH')),
    active BOOLEAN NOT NULL DEFAULT true,
    sort_order INT NOT NULL DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_support_categories_scope ON support_categories (requester_scope, active, sort_order);

CREATE TABLE IF NOT EXISTS support_conversations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    requester_type TEXT NOT NULL CHECK (requester_type IN ('CUSTOMER', 'SELLER')),
    requester_user_id UUID REFERENCES users(id),
    requester_seller_id UUID REFERENCES sellers(id),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT chk_requester_identity CHECK (
        (requester_type = 'CUSTOMER' AND requester_user_id IS NOT NULL AND requester_seller_id IS NULL) OR
        (requester_type = 'SELLER' AND requester_seller_id IS NOT NULL AND requester_user_id IS NULL)
    )
);

ALTER TABLE support_conversations DROP COLUMN IF EXISTS requester_unread_count;

CREATE UNIQUE INDEX IF NOT EXISTS idx_support_conv_customer ON support_conversations (requester_user_id) WHERE requester_type = 'CUSTOMER';
CREATE UNIQUE INDEX IF NOT EXISTS idx_support_conv_seller ON support_conversations (requester_seller_id) WHERE requester_type = 'SELLER';

CREATE TABLE IF NOT EXISTS support_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    conversation_id UUID NOT NULL REFERENCES support_conversations(id) ON DELETE CASCADE,
    status TEXT NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE', 'COMPLETED')),
    priority TEXT NOT NULL DEFAULT 'NORMAL' CHECK (priority IN ('NORMAL', 'HIGH', 'URGENT')),
    category_id UUID REFERENCES support_categories(id),
    assigned_to UUID REFERENCES users(id),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMP WITH TIME ZONE,
    CONSTRAINT chk_support_session_completed_consistency CHECK (
        (status = 'ACTIVE' AND completed_at IS NULL) OR
        (status = 'COMPLETED' AND completed_at IS NOT NULL)
    )
);

ALTER TABLE support_sessions ADD COLUMN IF NOT EXISTS category_id UUID REFERENCES support_categories(id);
ALTER TABLE support_sessions DROP COLUMN IF EXISTS category;
ALTER TABLE support_sessions DROP CONSTRAINT IF EXISTS support_sessions_priority_check;
ALTER TABLE support_sessions ADD CONSTRAINT support_sessions_priority_check CHECK (priority IN ('NORMAL', 'HIGH', 'URGENT'));

CREATE UNIQUE INDEX IF NOT EXISTS idx_support_sessions_active_one_per_conv ON support_sessions (conversation_id) WHERE status = 'ACTIVE';
CREATE INDEX IF NOT EXISTS idx_support_sessions_conv_created ON support_sessions (conversation_id, created_at);
CREATE INDEX IF NOT EXISTS idx_support_sessions_status ON support_sessions (status);

CREATE TABLE IF NOT EXISTS support_messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id UUID NOT NULL REFERENCES support_sessions(id) ON DELETE CASCADE,
    sender_type TEXT NOT NULL CHECK (sender_type IN ('CUSTOMER', 'SELLER', 'STAFF')),
    sender_user_id UUID NOT NULL REFERENCES users(id),
    text_content TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

ALTER TABLE support_messages DROP COLUMN IF EXISTS conversation_id;
ALTER TABLE support_messages DROP CONSTRAINT IF EXISTS support_messages_sender_type_check;
ALTER TABLE support_messages ADD CONSTRAINT support_messages_sender_type_check CHECK (sender_type IN ('CUSTOMER', 'SELLER', 'STAFF'));

CREATE INDEX IF NOT EXISTS idx_support_messages_session_created ON support_messages (session_id, created_at);

CREATE TABLE IF NOT EXISTS support_internal_notes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id UUID NOT NULL REFERENCES support_sessions(id) ON DELETE CASCADE,
    author_user_id UUID NOT NULL REFERENCES users(id),
    text_content TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

ALTER TABLE support_internal_notes DROP COLUMN IF EXISTS conversation_id;

CREATE INDEX IF NOT EXISTS idx_support_notes_session_created ON support_internal_notes (session_id, created_at);

CREATE TABLE IF NOT EXISTS support_context_links (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    message_id UUID NOT NULL REFERENCES support_messages(id) ON DELETE CASCADE,
    context_type TEXT NOT NULL CHECK (context_type IN ('ORDER', 'RETURN', 'PRODUCT')),
    context_id UUID NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

ALTER TABLE support_context_links DROP COLUMN IF EXISTS conversation_id;
ALTER TABLE support_context_links DROP COLUMN IF EXISTS session_id;

CREATE INDEX IF NOT EXISTS idx_support_context_message ON support_context_links (message_id);

CREATE TABLE IF NOT EXISTS support_staged_attachments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    conversation_id UUID NOT NULL REFERENCES support_conversations(id) ON DELETE CASCADE,
    uploader_user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    storage_key TEXT NOT NULL,
    content_type TEXT NOT NULL,
    size_bytes BIGINT NOT NULL,
    original_filename TEXT,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_support_staged_conv_uploader ON support_staged_attachments (conversation_id, uploader_user_id);

CREATE TABLE IF NOT EXISTS support_attachments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    message_id UUID NOT NULL REFERENCES support_messages(id) ON DELETE CASCADE,
    storage_key TEXT NOT NULL,
    content_type TEXT NOT NULL,
    size_bytes BIGINT NOT NULL,
    original_filename TEXT,
    sort_order INT NOT NULL DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

ALTER TABLE support_attachments ADD COLUMN IF NOT EXISTS sort_order INT NOT NULL DEFAULT 0;
ALTER TABLE support_attachments ADD COLUMN IF NOT EXISTS original_filename TEXT;
ALTER TABLE support_attachments DROP COLUMN IF EXISTS file_name;

CREATE INDEX IF NOT EXISTS idx_support_attachments_message ON support_attachments (message_id, sort_order);

CREATE TABLE IF NOT EXISTS support_staff_reads (
    staff_user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    conversation_id UUID NOT NULL REFERENCES support_conversations(id) ON DELETE CASCADE,
    last_read_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (staff_user_id, conversation_id)
);

CREATE TABLE IF NOT EXISTS support_requester_reads (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    conversation_id UUID NOT NULL REFERENCES support_conversations(id) ON DELETE CASCADE,
    last_read_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, conversation_id)
);
