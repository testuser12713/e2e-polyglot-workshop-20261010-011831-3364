-- Schema of the workshop api. Owned by api/internal/db/schema.sql.
-- Every statement is idempotent: the api applies this at startup
-- against an empty PostgreSQL 18 database.

CREATE TABLE IF NOT EXISTS customers (
    id         BIGSERIAL PRIMARY KEY,
    name       TEXT NOT NULL,
    email      TEXT NOT NULL,
    phone      TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS vehicles (
    id            BIGSERIAL PRIMARY KEY,
    license_plate TEXT NOT NULL UNIQUE,
    brand         TEXT NOT NULL,
    model         TEXT NOT NULL,
    mileage       INTEGER NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS orders (
    id                  BIGSERIAL PRIMARY KEY,
    order_number        TEXT NOT NULL UNIQUE,
    customer_id         BIGINT NOT NULL REFERENCES customers (id),
    vehicle_id          BIGINT NOT NULL REFERENCES vehicles (id),
    status              TEXT NOT NULL DEFAULT 'angefragt',
    requested_date      DATE NOT NULL,
    problem_description TEXT NOT NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS order_items (
    id               BIGSERIAL PRIMARY KEY,
    order_id         BIGINT NOT NULL REFERENCES orders (id),
    kind             TEXT NOT NULL,
    description      TEXT NOT NULL,
    hours            NUMERIC(10, 2),
    quantity         INTEGER,
    unit_price_cents INTEGER,
    total_cents      INTEGER NOT NULL DEFAULT 0,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS order_status_events (
    id          BIGSERIAL PRIMARY KEY,
    order_id    BIGINT NOT NULL REFERENCES orders (id),
    from_status TEXT,
    to_status   TEXT NOT NULL,
    changed_by  TEXT NOT NULL,
    changed_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS employees (
    id            BIGSERIAL PRIMARY KEY,
    name          TEXT NOT NULL,
    email         TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS invoices (
    id             BIGSERIAL PRIMARY KEY,
    invoice_number TEXT NOT NULL UNIQUE,
    order_id       BIGINT NOT NULL UNIQUE REFERENCES orders (id),
    issued_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    labor_cents    INTEGER NOT NULL DEFAULT 0,
    parts_cents    INTEGER NOT NULL DEFAULT 0,
    net_cents      INTEGER NOT NULL DEFAULT 0,
    vat_cents      INTEGER NOT NULL DEFAULT 0,
    gross_cents    INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS invoice_lines (
    id               BIGSERIAL PRIMARY KEY,
    invoice_id       BIGINT NOT NULL REFERENCES invoices (id),
    description      TEXT NOT NULL,
    quantity         INTEGER NOT NULL DEFAULT 0,
    unit_price_cents INTEGER NOT NULL DEFAULT 0,
    total_cents      INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS outbox (
    id              BIGSERIAL PRIMARY KEY,
    order_id        BIGINT NOT NULL REFERENCES orders (id),
    invoice_id      BIGINT NOT NULL REFERENCES invoices (id),
    recipient_email TEXT NOT NULL,
    subject         TEXT NOT NULL,
    body            TEXT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    sent_at         TIMESTAMPTZ
);
