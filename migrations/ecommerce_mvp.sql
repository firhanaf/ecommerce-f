-- ============================================================
-- Ecommerce MVP — Schema DDL
-- PostgreSQL 16
-- Run: psql -U postgres -d ecommerce_db -f migrations/ecommerce_mvp.sql
-- ============================================================

-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- ─── Users ───────────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS users (
    id             UUID         PRIMARY KEY DEFAULT uuid_generate_v4(),
    name           VARCHAR(100) NOT NULL,
    email          VARCHAR(255) NOT NULL UNIQUE,
    password_hash  TEXT         NOT NULL,
    role           VARCHAR(20)  NOT NULL DEFAULT 'buyer'
                     CHECK (role IN ('buyer', 'admin')),
    phone          VARCHAR(20)  UNIQUE,
    avatar_url     TEXT,
    is_active      BOOLEAN      NOT NULL DEFAULT true,
    phone_verified BOOLEAN      NOT NULL DEFAULT false,
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
CREATE INDEX IF NOT EXISTS idx_users_role  ON users(role);

-- ─── OTP Tokens ──────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS otp_tokens (
    id         UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id    UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    code       VARCHAR(6)  NOT NULL,
    type       VARCHAR(30) NOT NULL CHECK (type IN ('phone_verification', 'reset_password')),
    expires_at TIMESTAMPTZ NOT NULL,
    used_at    TIMESTAMPTZ,
    attempts   INT         NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_otp_tokens_user_type ON otp_tokens(user_id, type);
CREATE INDEX IF NOT EXISTS idx_otp_tokens_expires_at ON otp_tokens(expires_at);

-- ─── Addresses ───────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS addresses (
    id             UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id        UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    recipient_name VARCHAR(100) NOT NULL,
    phone          VARCHAR(20)  NOT NULL,
    street         TEXT         NOT NULL,
    city           VARCHAR(100) NOT NULL,
    province       VARCHAR(100) NOT NULL,
    postal_code    VARCHAR(10)  NOT NULL,
    is_default     BOOLEAN      NOT NULL DEFAULT false,
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_addresses_user_id ON addresses(user_id);

-- ─── Categories ──────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS categories (
    id         UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    name       VARCHAR(100) NOT NULL,
    slug       VARCHAR(120) NOT NULL UNIQUE,
    parent_id  UUID        REFERENCES categories(id) ON DELETE SET NULL,
    is_active  BOOLEAN     NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ─── Products ─────────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS products (
    id          UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    name        VARCHAR(255) NOT NULL,
    slug        VARCHAR(300) NOT NULL UNIQUE,
    description TEXT,
    category_id UUID        REFERENCES categories(id) ON DELETE SET NULL,
    is_active   BOOLEAN     NOT NULL DEFAULT true,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_products_slug        ON products(slug);
CREATE INDEX IF NOT EXISTS idx_products_category_id ON products(category_id);
CREATE INDEX IF NOT EXISTS idx_products_is_active   ON products(is_active);
CREATE INDEX IF NOT EXISTS idx_products_name_search ON products USING GIN (to_tsvector('simple', name));

-- ─── Product Variants ────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS product_variants (
    id         UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    product_id UUID        NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    name       VARCHAR(100) NOT NULL,
    sku        VARCHAR(100),
    price      BIGINT      NOT NULL CHECK (price > 0),
    stock      INT         NOT NULL DEFAULT 0 CHECK (stock >= 0),
    is_active  BOOLEAN     NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_variants_product_id ON product_variants(product_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_variants_sku ON product_variants(sku) WHERE sku IS NOT NULL AND sku != '';

-- ─── Product Images ───────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS product_images (
    id         UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    product_id UUID        NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    url        TEXT        NOT NULL,
    is_primary BOOLEAN     NOT NULL DEFAULT false,
    sort_order INT         NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_product_images_product_id ON product_images(product_id);

-- ─── Carts ────────────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS carts (
    id         UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id    UUID        NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ─── Cart Items ───────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS cart_items (
    id         UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    cart_id    UUID        NOT NULL REFERENCES carts(id) ON DELETE CASCADE,
    variant_id UUID        NOT NULL REFERENCES product_variants(id) ON DELETE CASCADE,
    quantity   INT         NOT NULL DEFAULT 1 CHECK (quantity > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (cart_id, variant_id)
);

CREATE INDEX IF NOT EXISTS idx_cart_items_cart_id ON cart_items(cart_id);

-- ─── Orders ───────────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS orders (
    id               UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id          UUID        NOT NULL REFERENCES users(id),
    address_id       UUID        NOT NULL REFERENCES addresses(id),
    snapshot_address JSONB       NOT NULL DEFAULT '{}',
    status           VARCHAR(30) NOT NULL DEFAULT 'pending_payment'
                        CHECK (status IN (
                            'pending_payment', 'paid', 'processing',
                            'shipped', 'delivered', 'completed',
                            'cancelled', 'refunded'
                        )),
    subtotal         BIGINT      NOT NULL CHECK (subtotal >= 0),
    shipping_cost    BIGINT      NOT NULL DEFAULT 0 CHECK (shipping_cost >= 0),
    total            BIGINT      NOT NULL CHECK (total >= 0),
    courier          VARCHAR(50),
    courier_service  VARCHAR(50),
    notes            TEXT,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_orders_user_id    ON orders(user_id);
CREATE INDEX IF NOT EXISTS idx_orders_status     ON orders(status);
CREATE INDEX IF NOT EXISTS idx_orders_created_at ON orders(created_at DESC);

-- ─── Order Items ─────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS order_items (
    id            UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    order_id      UUID        NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    variant_id    UUID        REFERENCES product_variants(id) ON DELETE SET NULL,
    product_name  VARCHAR(255) NOT NULL,
    variant_name  VARCHAR(100) NOT NULL,
    product_image TEXT,
    quantity      INT         NOT NULL CHECK (quantity > 0),
    unit_price    BIGINT      NOT NULL CHECK (unit_price > 0),
    subtotal      BIGINT      NOT NULL CHECK (subtotal > 0),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_order_items_order_id ON order_items(order_id);

-- ─── Payments ────────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS payments (
    id             UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    order_id       UUID        NOT NULL UNIQUE REFERENCES orders(id),
    provider       VARCHAR(50) NOT NULL DEFAULT 'midtrans',
    payment_method VARCHAR(50),
    status         VARCHAR(20) NOT NULL DEFAULT 'pending'
                     CHECK (status IN ('pending', 'settlement', 'cancel', 'expire', 'refund', 'failed')),
    transaction_id VARCHAR(255),
    amount         NUMERIC(15, 2) NOT NULL,
    raw_response   JSONB       DEFAULT '{}',
    paid_at        TIMESTAMPTZ,
    expired_at     TIMESTAMPTZ,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_payments_order_id       ON payments(order_id);
CREATE INDEX IF NOT EXISTS idx_payments_transaction_id ON payments(transaction_id);
CREATE INDEX IF NOT EXISTS idx_payments_status         ON payments(status);

-- ─── Shipments ───────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS shipments (
    id              UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    order_id        UUID        NOT NULL UNIQUE REFERENCES orders(id),
    courier         VARCHAR(50) NOT NULL,
    courier_service VARCHAR(50),
    tracking_number VARCHAR(100),
    status          VARCHAR(20) NOT NULL DEFAULT 'pending'
                      CHECK (status IN ('pending', 'picked_up', 'in_transit', 'delivered', 'returned')),
    shipped_at      TIMESTAMPTZ,
    delivered_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_shipments_order_id ON shipments(order_id);

-- ─── Audit Logs ──────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS audit_logs (
    id          UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    actor_id    UUID        REFERENCES users(id) ON DELETE SET NULL,
    actor_role  VARCHAR(20),
    action      VARCHAR(20) NOT NULL
                  CHECK (action IN ('CREATE', 'UPDATE', 'DELETE', 'LOGIN', 'LOGOUT', 'EXPORT')),
    entity_type VARCHAR(50) NOT NULL,
    entity_id   UUID,
    old_data    JSONB       DEFAULT '{}',
    new_data    JSONB       DEFAULT '{}',
    ip_address  VARCHAR(50),
    user_agent  TEXT,
    notes       TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_audit_logs_actor_id    ON audit_logs(actor_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_entity      ON audit_logs(entity_type, entity_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_created_at  ON audit_logs(created_at DESC);

-- ─── Seed: default admin user ────────────────────────────────────────────────
-- Password: Admin@12345 (bcrypt hash)
-- GANTI hash ini sebelum deploy ke production!

INSERT INTO users (id, name, email, password_hash, role, is_active, phone_verified)
VALUES (
    uuid_generate_v4(),
    'Admin',
    'admin@ecommerce.local',
    '$2a$10$92IXUNpkjO0rOQ5byMi.Ye4oKoEa3Ro9llC/.og/at2.uheWG/igi', -- password: "password"
    'admin',
    true,
    true  -- admin tidak perlu verifikasi WA
) ON CONFLICT (email) DO NOTHING;
