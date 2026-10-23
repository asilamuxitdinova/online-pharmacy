-- Every timestamp is unix seconds (UTC). Money is integer UZS.

CREATE TABLE IF NOT EXISTS users (
    id             TEXT PRIMARY KEY,
    email          TEXT NOT NULL UNIQUE,
    phone          TEXT UNIQUE,
    name           TEXT NOT NULL,
    password_hash  TEXT NOT NULL,
    role           TEXT NOT NULL CHECK (role IN ('customer', 'staff', 'admin')),
    tin            TEXT,                       -- taxpayer number, staff-only
    disabled_at    INTEGER,
    failed_logins  INTEGER NOT NULL DEFAULT 0,
    locked_until   INTEGER,
    created_at     INTEGER NOT NULL
);

-- One row per sign-in. Access tokens carry its id, so ending the row ends
-- every token issued for it on the very next request.
CREATE TABLE IF NOT EXISTS sessions (
    id            TEXT PRIMARY KEY,
    user_id       TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind          TEXT NOT NULL,              -- password, otp, oauth, cookie, register
    cookie_hash   TEXT UNIQUE,                -- only for kind = cookie
    created_at    INTEGER NOT NULL,
    expires_at    INTEGER NOT NULL,
    ended_at      INTEGER,
    ended_reason  TEXT
);

CREATE TABLE IF NOT EXISTS refresh_tokens (
    token_hash  TEXT PRIMARY KEY,
    session_id  TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    created_at  INTEGER NOT NULL,
    expires_at  INTEGER NOT NULL,
    used_at     INTEGER
);

CREATE TABLE IF NOT EXISTS otp_codes (
    id           TEXT PRIMARY KEY,
    destination  TEXT NOT NULL,
    code         TEXT NOT NULL,
    attempts     INTEGER NOT NULL DEFAULT 0,
    expires_at   INTEGER NOT NULL,
    used_at      INTEGER,
    created_at   INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS api_keys (
    id          TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    prefix      TEXT NOT NULL,
    key_hash    TEXT NOT NULL UNIQUE,
    created_at  INTEGER NOT NULL,
    revoked_at  INTEGER
);

CREATE TABLE IF NOT EXISTS oauth_codes (
    code_hash       TEXT PRIMARY KEY,
    client_id       TEXT NOT NULL,
    user_id         TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    redirect_uri    TEXT NOT NULL,
    code_challenge  TEXT NOT NULL,
    expires_at      INTEGER NOT NULL,
    used_at         INTEGER
);

CREATE TABLE IF NOT EXISTS categories (
    id       TEXT PRIMARY KEY,
    name_en  TEXT NOT NULL,
    name_ru  TEXT NOT NULL,
    name_uz  TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS products (
    id                     TEXT PRIMARY KEY,
    sku                    TEXT NOT NULL UNIQUE,
    category_id            TEXT NOT NULL REFERENCES categories(id),
    name_en                TEXT NOT NULL,
    name_ru                TEXT NOT NULL,
    name_uz                TEXT NOT NULL,
    manufacturer           TEXT NOT NULL,
    form                   TEXT NOT NULL,
    strength               TEXT NOT NULL,
    pack_size              INTEGER NOT NULL,
    price                  INTEGER NOT NULL,
    stock                  INTEGER NOT NULL,
    requires_prescription  INTEGER NOT NULL,
    search                 TEXT NOT NULL,      -- lower-cased names in every language
    created_at             INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS products_category ON products(category_id);

CREATE TABLE IF NOT EXISTS addresses (
    id          TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    label       TEXT NOT NULL,
    line1       TEXT NOT NULL,
    city        TEXT NOT NULL,
    phone       TEXT NOT NULL,
    created_at  INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS prescriptions (
    id           TEXT PRIMARY KEY,
    user_id      TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    doctor_name  TEXT NOT NULL,
    clinic       TEXT NOT NULL,
    issued_on    TEXT NOT NULL,
    diagnosis    TEXT NOT NULL,
    status       TEXT NOT NULL CHECK (status IN ('pending_review', 'approved', 'rejected')),
    review_note  TEXT,
    reviewed_by  TEXT,
    created_at   INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS orders (
    id               TEXT PRIMARY KEY,
    user_id          TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status           TEXT NOT NULL,
    delivery         TEXT NOT NULL,
    address_id       TEXT,
    prescription_id  TEXT,
    note             TEXT NOT NULL DEFAULT '',
    subtotal         INTEGER NOT NULL,
    delivery_fee     INTEGER NOT NULL,
    total            INTEGER NOT NULL,
    refunded         INTEGER NOT NULL DEFAULT 0,
    created_at       INTEGER NOT NULL,
    updated_at       INTEGER NOT NULL,
    paid_at          INTEGER
);
CREATE INDEX IF NOT EXISTS orders_user ON orders(user_id, created_at);

CREATE TABLE IF NOT EXISTS order_items (
    order_id    TEXT NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    product_id  TEXT NOT NULL REFERENCES products(id),
    quantity    INTEGER NOT NULL,
    unit_price  INTEGER NOT NULL,
    PRIMARY KEY (order_id, product_id)
);

CREATE TABLE IF NOT EXISTS cards (
    id          TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    brand       TEXT NOT NULL,
    last4       TEXT NOT NULL,
    exp_month   INTEGER NOT NULL,
    exp_year    INTEGER NOT NULL,
    outcome     TEXT NOT NULL,                 -- what the test processor will answer
    created_at  INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS payments (
    id          TEXT PRIMARY KEY,
    order_id    TEXT NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    amount      INTEGER NOT NULL,
    brand       TEXT NOT NULL,
    last4       TEXT NOT NULL,
    status      TEXT NOT NULL,
    created_at  INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS refunds (
    id           TEXT PRIMARY KEY,
    order_id     TEXT NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    amount       INTEGER NOT NULL,
    reason       TEXT NOT NULL,
    status       TEXT NOT NULL CHECK (status IN ('approved', 'pending_review', 'rejected')),
    decided_by   TEXT,
    created_at   INTEGER NOT NULL,
    decided_at   INTEGER
);

CREATE TABLE IF NOT EXISTS idempotency_keys (
    user_id       TEXT NOT NULL,
    key           TEXT NOT NULL,
    route         TEXT NOT NULL,
    request_hash  TEXT NOT NULL,
    status_code   INTEGER,                     -- NULL while the first request is running
    response      BLOB,
    created_at    INTEGER NOT NULL,
    PRIMARY KEY (user_id, key)
);

-- A job's state is derived from the clock, so it survives restarts and needs
-- no worker: progress = elapsed / duration.
CREATE TABLE IF NOT EXISTS jobs (
    id            TEXT PRIMARY KEY,
    user_id       TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind          TEXT NOT NULL,
    input         TEXT NOT NULL,
    duration_ms   INTEGER NOT NULL,
    fail_reason   TEXT,
    created_at_ms INTEGER NOT NULL
);

-- Everything that would otherwise go to email, SMS or a webhook endpoint.
CREATE TABLE IF NOT EXISTS inbox (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    kind        TEXT NOT NULL,                 -- otp, webhook_received, webhook_sent, notice
    recipient   TEXT NOT NULL,
    subject     TEXT NOT NULL,
    body        TEXT NOT NULL,
    meta        TEXT NOT NULL DEFAULT '{}',
    created_at  INTEGER NOT NULL
);
