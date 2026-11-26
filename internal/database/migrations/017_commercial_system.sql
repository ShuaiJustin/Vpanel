-- Commercial System Migration
-- Creates tables for plans, orders, balance, coupons, invites, commissions, and invoices

-- Commercial Plans table
CREATE TABLE IF NOT EXISTS commercial_plans (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name VARCHAR(128) NOT NULL,
    description TEXT,
    traffic_limit BIGINT DEFAULT 0,
    duration INTEGER NOT NULL,
    price BIGINT NOT NULL,
    plan_type VARCHAR(32) NOT NULL DEFAULT 'monthly',
    reset_cycle VARCHAR(32) DEFAULT 'monthly',
    ip_limit INTEGER DEFAULT 0,
    sort_order INTEGER DEFAULT 0,
    is_active BOOLEAN DEFAULT TRUE,
    is_recommended BOOLEAN DEFAULT FALSE,
    group_id INTEGER,
    payment_methods TEXT,
    features TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_commercial_plans_active ON commercial_plans(is_active, sort_order);
CREATE INDEX IF NOT EXISTS idx_commercial_plans_group ON commercial_plans(group_id);

-- Plan Groups table
CREATE TABLE IF NOT EXISTS plan_groups (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name VARCHAR(64) NOT NULL,
    sort_order INTEGER DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Orders table
CREATE TABLE IF NOT EXISTS orders (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    order_no VARCHAR(64) NOT NULL UNIQUE,
    user_id INTEGER NOT NULL,
    plan_id INTEGER NOT NULL,
    coupon_id INTEGER,
    original_amount BIGINT NOT NULL,
    discount_amount BIGINT DEFAULT 0,
    balance_used BIGINT DEFAULT 0,
    pay_amount BIGINT NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'pending',
    payment_method VARCHAR(32),
    payment_no VARCHAR(128),
    paid_at DATETIME,
    expired_at DATETIME NOT NULL,
    notes TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id),
    FOREIGN KEY (plan_id) REFERENCES commercial_plans(id),
    FOREIGN KEY (coupon_id) REFERENCES coupons(id)
);

CREATE INDEX IF NOT EXISTS idx_orders_user ON orders(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_orders_status ON orders(status, expired_at);
CREATE INDEX IF NOT EXISTS idx_orders_order_no ON orders(order_no);
CREATE INDEX IF NOT EXISTS idx_orders_payment_no ON orders(payment_no);

-- Balance transactions table
CREATE TABLE IF NOT EXISTS balance_transactions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    type VARCHAR(32) NOT NULL,
    amount BIGINT NOT NULL,
    balance BIGINT NOT NULL,
    order_id INTEGER,
    description VARCHAR(256),
    operator VARCHAR(64),
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id),
    FOREIGN KEY (order_id) REFERENCES orders(id)
);

CREATE INDEX IF NOT EXISTS idx_balance_tx_user ON balance_transactions(user_id, created_at DESC);

-- Coupons table
CREATE TABLE IF NOT EXISTS coupons (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    code VARCHAR(32) NOT NULL UNIQUE,
    name VARCHAR(128) NOT NULL,
    type VARCHAR(16) NOT NULL,
    value BIGINT NOT NULL,
    min_order_amount BIGINT DEFAULT 0,
    max_discount BIGINT DEFAULT 0,
    total_limit INTEGER DEFAULT 0,
    per_user_limit INTEGER DEFAULT 1,
    used_count INTEGER DEFAULT 0,
    plan_ids TEXT,
    start_at DATETIME NOT NULL,
    expire_at DATETIME NOT NULL,
    is_active BOOLEAN DEFAULT TRUE,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_coupons_code ON coupons(code);
CREATE INDEX IF NOT EXISTS idx_coupons_active ON coupons(is_active, start_at, expire_at);

-- Coupon usage table
CREATE TABLE IF NOT EXISTS coupon_usages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    coupon_id INTEGER NOT NULL,
    user_id INTEGER NOT NULL,
    order_id INTEGER NOT NULL,
    discount BIGINT NOT NULL,
    used_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (coupon_id) REFERENCES coupons(id),
    FOREIGN KEY (user_id) REFERENCES users(id),
    FOREIGN KEY (order_id) REFERENCES orders(id)
);

CREATE INDEX IF NOT EXISTS idx_coupon_usage ON coupon_usages(coupon_id, user_id);

-- Commercial Invite codes table
CREATE TABLE IF NOT EXISTS commercial_invite_codes (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL UNIQUE,
    code VARCHAR(16) NOT NULL UNIQUE,
    invite_count INTEGER DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id)
);

CREATE INDEX IF NOT EXISTS idx_commercial_invite_codes_code ON commercial_invite_codes(code);

-- Referrals table
CREATE TABLE IF NOT EXISTS referrals (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    inviter_id INTEGER NOT NULL,
    invitee_id INTEGER NOT NULL UNIQUE,
    invite_code VARCHAR(16) NOT NULL,
    status VARCHAR(32) DEFAULT 'registered',
    converted_at DATETIME,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (inviter_id) REFERENCES users(id),
    FOREIGN KEY (invitee_id) REFERENCES users(id)
);

CREATE INDEX IF NOT EXISTS idx_referrals_inviter ON referrals(inviter_id);

-- Commissions table
CREATE TABLE IF NOT EXISTS commissions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    from_user_id INTEGER NOT NULL,
    order_id INTEGER NOT NULL,
    amount BIGINT NOT NULL,
    rate REAL NOT NULL,
    level INTEGER DEFAULT 1,
    status VARCHAR(32) DEFAULT 'pending',
    confirm_at DATETIME,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id),
    FOREIGN KEY (from_user_id) REFERENCES users(id),
    FOREIGN KEY (order_id) REFERENCES orders(id)
);

CREATE INDEX IF NOT EXISTS idx_commissions_user ON commissions(user_id, status);

-- Invoices table
CREATE TABLE IF NOT EXISTS invoices (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    invoice_no VARCHAR(64) NOT NULL UNIQUE,
    order_id INTEGER NOT NULL,
    user_id INTEGER NOT NULL,
    amount BIGINT NOT NULL,
    content TEXT NOT NULL,
    pdf_path VARCHAR(256),
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (order_id) REFERENCES orders(id),
    FOREIGN KEY (user_id) REFERENCES users(id)
);

CREATE INDEX IF NOT EXISTS idx_invoices_user ON invoices(user_id, created_at DESC);

-- Add balance and auto_renewal fields to users table
ALTER TABLE users ADD COLUMN balance BIGINT DEFAULT 0;
ALTER TABLE users ADD COLUMN auto_renewal BOOLEAN DEFAULT FALSE;
