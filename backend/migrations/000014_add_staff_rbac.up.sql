-- 1. Staff roles
CREATE TABLE IF NOT EXISTS staff_roles (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code        TEXT UNIQUE NOT NULL,
    name        TEXT NOT NULL,
    description TEXT,
    is_system   BOOLEAN NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 2. Staff role permissions
CREATE TABLE IF NOT EXISTS staff_role_permissions (
    role_id     UUID NOT NULL REFERENCES staff_roles(id) ON DELETE CASCADE,
    permission  TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (role_id, permission)
);

-- 3. Staff members
CREATE TABLE IF NOT EXISTS staff_members (
    user_id        UUID PRIMARY KEY REFERENCES users(id) ON DELETE RESTRICT,
    staff_role_id  UUID NOT NULL REFERENCES staff_roles(id) ON DELETE RESTRICT,
    status         TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'blocked', 'archived')),
    created_by     UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 4. Audit logs
CREATE TABLE IF NOT EXISTS audit_logs (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    actor_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    actor_email   TEXT,
    actor_role    TEXT,
    permission    TEXT,
    action        TEXT NOT NULL,
    entity_type   TEXT,
    entity_id     UUID,
    request_id    TEXT,
    ip            TEXT,
    user_agent    TEXT,
    metadata      JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes
CREATE INDEX IF NOT EXISTS idx_staff_roles_code ON staff_roles(code);
CREATE INDEX IF NOT EXISTS idx_staff_members_role ON staff_members(staff_role_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_created ON audit_logs(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_logs_actor ON audit_logs(actor_user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_logs_entity ON audit_logs(entity_type, entity_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_logs_action ON audit_logs(action, created_at DESC);

-- Seed system roles
INSERT INTO staff_roles (code, name, description, is_system) VALUES
    ('owner',              'Владелец',             'Полный доступ',                          true),
    ('co_owner',           'Совладелец',           'Почти полный доступ',                    true),
    ('admin',              'Администратор',        'Операционный доступ',                    true),
    ('moderator',          'Модератор',            'Модерация товаров и отзывов',             true),
    ('support',            'Поддержка',            'Работа с тикетами и заказами',           true),
    ('finance',            'Финансы',              'Платежи, выплаты, возмещения',           true),
    ('content_manager',    'Контент-менеджер',     'Категории, бренды, витрина',             true),
    ('warehouse_operator', 'Оператор склада',      'Остатки, приёмка, отгрузки',             true)
ON CONFLICT (code) DO NOTHING;

-- owner: ALL permissions
INSERT INTO staff_role_permissions (role_id, permission)
SELECT r.id, p.permission
FROM staff_roles r
CROSS JOIN (VALUES
    ('staff.read'), ('staff.create'), ('staff.update'), ('staff.block'),
    ('roles.read'), ('roles.manage'),
    ('sellers.read'), ('sellers.create_access'), ('sellers.update_status'), ('sellers.warn'), ('sellers.message'), ('sellers.verify'),
    ('products.read'), ('products.moderate'), ('products.approve'), ('products.reject'), ('products.publish'), ('products.hide'), ('products.block'),
    ('categories.read'), ('categories.create'), ('categories.update'), ('categories.delete'),
    ('brands.read'), ('brands.create'), ('brands.update'), ('brands.delete'),
    ('inventory.read'), ('inventory.receipt'), ('inventory.adjust'), ('inventory.write_off'), ('inventory.movements.read'),
    ('orders.read'), ('orders.update_status'),
    ('payments.read'),
    ('shipments.read'), ('shipments.create'), ('shipments.update_status'),
    ('returns.read'), ('returns.update_status'),
    ('refunds.read'), ('refunds.create'),
    ('payouts.read'), ('payouts.approve'), ('payouts.reject'), ('payouts.mark_paid'),
    ('reviews.read'), ('reviews.approve'), ('reviews.reject'), ('reviews.hide'), ('reviews.block'),
    ('complaints.read'), ('complaints.resolve'),
    ('support.read'), ('support.respond'), ('support.close'),
    ('analytics.read'), ('exports.excel'),
    ('settings.read'), ('settings.manage'),
    ('audit.read'),
    ('storefront.manage'),
    ('commission.manage')
) AS p(permission)
WHERE r.code = 'owner'
ON CONFLICT DO NOTHING;

-- co_owner: ALL except commission.manage, settings.manage
INSERT INTO staff_role_permissions (role_id, permission)
SELECT r.id, p.permission
FROM staff_roles r
CROSS JOIN (VALUES
    ('staff.read'), ('staff.create'), ('staff.update'), ('staff.block'),
    ('roles.read'), ('roles.manage'),
    ('sellers.read'), ('sellers.create_access'), ('sellers.update_status'), ('sellers.warn'), ('sellers.message'), ('sellers.verify'),
    ('products.read'), ('products.moderate'), ('products.approve'), ('products.reject'), ('products.publish'), ('products.hide'), ('products.block'),
    ('categories.read'), ('categories.create'), ('categories.update'), ('categories.delete'),
    ('brands.read'), ('brands.create'), ('brands.update'), ('brands.delete'),
    ('inventory.read'), ('inventory.receipt'), ('inventory.adjust'), ('inventory.write_off'), ('inventory.movements.read'),
    ('orders.read'), ('orders.update_status'),
    ('payments.read'),
    ('shipments.read'), ('shipments.create'), ('shipments.update_status'),
    ('returns.read'), ('returns.update_status'),
    ('refunds.read'), ('refunds.create'),
    ('payouts.read'), ('payouts.approve'), ('payouts.reject'), ('payouts.mark_paid'),
    ('reviews.read'), ('reviews.approve'), ('reviews.reject'), ('reviews.hide'), ('reviews.block'),
    ('complaints.read'), ('complaints.resolve'),
    ('support.read'), ('support.respond'), ('support.close'),
    ('analytics.read'), ('exports.excel'),
    ('settings.read'),
    ('audit.read'),
    ('storefront.manage')
) AS p(permission)
WHERE r.code = 'co_owner'
ON CONFLICT DO NOTHING;

-- admin: operational access
INSERT INTO staff_role_permissions (role_id, permission)
SELECT r.id, p.permission
FROM staff_roles r
CROSS JOIN (VALUES
    ('sellers.read'), ('sellers.create_access'), ('sellers.update_status'), ('sellers.warn'), ('sellers.message'), ('sellers.verify'),
    ('products.read'), ('products.moderate'), ('products.approve'), ('products.reject'), ('products.publish'), ('products.hide'), ('products.block'),
    ('categories.read'), ('categories.create'), ('categories.update'), ('categories.delete'),
    ('brands.read'), ('brands.create'), ('brands.update'), ('brands.delete'),
    ('inventory.read'), ('inventory.receipt'), ('inventory.adjust'), ('inventory.write_off'), ('inventory.movements.read'),
    ('orders.read'), ('orders.update_status'),
    ('payments.read'),
    ('shipments.read'), ('shipments.create'), ('shipments.update_status'),
    ('returns.read'), ('returns.update_status'),
    ('refunds.read'), ('refunds.create'),
    ('payouts.read'),
    ('reviews.read'), ('reviews.approve'), ('reviews.reject'), ('reviews.hide'), ('reviews.block'),
    ('complaints.read'), ('complaints.resolve'),
    ('support.read'), ('support.respond'), ('support.close'),
    ('analytics.read'), ('exports.excel'),
    ('settings.read'),
    ('audit.read'),
    ('roles.read')
) AS p(permission)
WHERE r.code = 'admin'
ON CONFLICT DO NOTHING;

-- moderator
INSERT INTO staff_role_permissions (role_id, permission)
SELECT r.id, p.permission
FROM staff_roles r
CROSS JOIN (VALUES
    ('products.read'), ('products.moderate'), ('products.approve'), ('products.reject'), ('products.publish'), ('products.hide'), ('products.block'),
    ('reviews.read'), ('reviews.approve'), ('reviews.reject'), ('reviews.hide'), ('reviews.block'),
    ('complaints.read'), ('complaints.resolve'),
    ('sellers.read')
) AS p(permission)
WHERE r.code = 'moderator'
ON CONFLICT DO NOTHING;

-- support
INSERT INTO staff_role_permissions (role_id, permission)
SELECT r.id, p.permission
FROM staff_roles r
CROSS JOIN (VALUES
    ('orders.read'),
    ('returns.read'), ('returns.update_status'),
    ('complaints.read'), ('complaints.resolve'),
    ('support.read'), ('support.respond'), ('support.close'),
    ('sellers.read'),
    ('products.read')
) AS p(permission)
WHERE r.code = 'support'
ON CONFLICT DO NOTHING;

-- finance
INSERT INTO staff_role_permissions (role_id, permission)
SELECT r.id, p.permission
FROM staff_roles r
CROSS JOIN (VALUES
    ('payments.read'),
    ('refunds.read'), ('refunds.create'),
    ('payouts.read'), ('payouts.approve'), ('payouts.reject'), ('payouts.mark_paid'),
    ('orders.read'),
    ('analytics.read'),
    ('exports.excel')
) AS p(permission)
WHERE r.code = 'finance'
ON CONFLICT DO NOTHING;

-- content_manager
INSERT INTO staff_role_permissions (role_id, permission)
SELECT r.id, p.permission
FROM staff_roles r
CROSS JOIN (VALUES
    ('categories.read'), ('categories.create'), ('categories.update'), ('categories.delete'),
    ('brands.read'), ('brands.create'), ('brands.update'), ('brands.delete'),
    ('storefront.manage'),
    ('products.read'),
    ('analytics.read')
) AS p(permission)
WHERE r.code = 'content_manager'
ON CONFLICT DO NOTHING;

-- warehouse_operator
INSERT INTO staff_role_permissions (role_id, permission)
SELECT r.id, p.permission
FROM staff_roles r
CROSS JOIN (VALUES
    ('inventory.read'), ('inventory.receipt'), ('inventory.adjust'), ('inventory.write_off'), ('inventory.movements.read'),
    ('shipments.read'), ('shipments.create'), ('shipments.update_status'),
    ('orders.read')
) AS p(permission)
WHERE r.code = 'warehouse_operator'
ON CONFLICT DO NOTHING;
