"""Tests for invoice persistence (``worker.store``).

They run against the real PostgreSQL named by ``DATABASE_URL`` and are skipped
cleanly when it is not set. Each test creates its own order and removes only the
rows it created.
"""

from __future__ import annotations

import os
from uuid import uuid4

import psycopg
import pytest

from worker import db, store
from worker.models import InvoiceDraft, InvoiceLine

DATABASE_URL = os.environ.get("DATABASE_URL")

pytestmark = pytest.mark.skipif(
    not DATABASE_URL,
    reason="DATABASE_URL is not set; the store tests need the real PostgreSQL",
)

SCHEMA_STATEMENTS = [
    """
    CREATE TABLE IF NOT EXISTS customers (
        id BIGSERIAL PRIMARY KEY,
        name TEXT NOT NULL,
        email TEXT NOT NULL,
        phone TEXT NOT NULL,
        created_at TIMESTAMPTZ NOT NULL DEFAULT now()
    )
    """,
    """
    CREATE TABLE IF NOT EXISTS vehicles (
        id BIGSERIAL PRIMARY KEY,
        license_plate TEXT NOT NULL UNIQUE,
        brand TEXT NOT NULL,
        model TEXT NOT NULL,
        mileage INTEGER NOT NULL DEFAULT 0,
        created_at TIMESTAMPTZ NOT NULL DEFAULT now()
    )
    """,
    """
    CREATE TABLE IF NOT EXISTS orders (
        id BIGSERIAL PRIMARY KEY,
        order_number TEXT NOT NULL UNIQUE,
        customer_id BIGINT NOT NULL REFERENCES customers (id),
        vehicle_id BIGINT NOT NULL REFERENCES vehicles (id),
        status TEXT NOT NULL DEFAULT 'angefragt',
        requested_date DATE NOT NULL,
        problem_description TEXT NOT NULL,
        created_at TIMESTAMPTZ NOT NULL DEFAULT now()
    )
    """,
    """
    CREATE TABLE IF NOT EXISTS order_items (
        id BIGSERIAL PRIMARY KEY,
        order_id BIGINT NOT NULL REFERENCES orders (id),
        kind TEXT NOT NULL,
        description TEXT NOT NULL,
        hours NUMERIC(10, 2),
        quantity INTEGER,
        unit_price_cents INTEGER,
        total_cents INTEGER NOT NULL DEFAULT 0,
        created_at TIMESTAMPTZ NOT NULL DEFAULT now()
    )
    """,
    """
    CREATE TABLE IF NOT EXISTS invoices (
        id BIGSERIAL PRIMARY KEY,
        invoice_number TEXT NOT NULL UNIQUE,
        order_id BIGINT NOT NULL UNIQUE REFERENCES orders (id),
        issued_at TIMESTAMPTZ NOT NULL DEFAULT now(),
        labor_cents INTEGER NOT NULL DEFAULT 0,
        parts_cents INTEGER NOT NULL DEFAULT 0,
        net_cents INTEGER NOT NULL DEFAULT 0,
        vat_cents INTEGER NOT NULL DEFAULT 0,
        gross_cents INTEGER NOT NULL DEFAULT 0
    )
    """,
    """
    CREATE TABLE IF NOT EXISTS invoice_lines (
        id BIGSERIAL PRIMARY KEY,
        invoice_id BIGINT NOT NULL REFERENCES invoices (id),
        description TEXT NOT NULL,
        quantity INTEGER NOT NULL DEFAULT 0,
        unit_price_cents INTEGER NOT NULL DEFAULT 0,
        total_cents INTEGER NOT NULL DEFAULT 0
    )
    """,
]


def _pool():
    try:
        pool = db.open_pool(DATABASE_URL)
        with pool.connection() as conn:
            for statement in SCHEMA_STATEMENTS:
                conn.execute(statement)
    except psycopg.OperationalError as exc:  # pragma: no cover - environment dependent
        pytest.skip(f"DATABASE_URL is not reachable: {exc}")
    return pool


@pytest.fixture()
def order_factory():
    pool = _pool()
    created: list[tuple[int, int, int]] = []

    def _make() -> tuple[int, str]:
        suffix = uuid4().hex[:12]
        with pool.connection() as conn, conn.transaction():
            customer_id = conn.execute(
                "INSERT INTO customers (name, email, phone) VALUES (%s, %s, %s) RETURNING id",
                ("Testkunde", "test@example.invalid", "0000"),
            ).fetchone()[0]
            vehicle_id = conn.execute(
                "INSERT INTO vehicles (license_plate, brand, model, mileage) "
                "VALUES (%s, %s, %s, %s) RETURNING id",
                (f"S-{suffix}", "VW", "Golf", 0),
            ).fetchone()[0]
            order_number = f"STORE-{suffix}"
            order_id = conn.execute(
                "INSERT INTO orders (order_number, customer_id, vehicle_id, "
                "requested_date, problem_description) VALUES (%s, %s, %s, %s, %s) RETURNING id",
                (order_number, customer_id, vehicle_id, "2026-01-02", "Test"),
            ).fetchone()[0]
        created.append((order_id, customer_id, vehicle_id))
        return order_id, order_number

    yield _make

    with pool.connection() as conn, conn.transaction():
        for order_id, customer_id, vehicle_id in created:
            conn.execute(
                "DELETE FROM invoice_lines WHERE invoice_id IN "
                "(SELECT id FROM invoices WHERE order_id = %s)",
                (order_id,),
            )
            conn.execute("DELETE FROM invoices WHERE order_id = %s", (order_id,))
            conn.execute("DELETE FROM order_items WHERE order_id = %s", (order_id,))
            conn.execute("DELETE FROM orders WHERE id = %s", (order_id,))
            conn.execute("DELETE FROM vehicles WHERE id = %s", (vehicle_id,))
            conn.execute("DELETE FROM customers WHERE id = %s", (customer_id,))
    db.close_pool()


def _draft(order_id: int, order_number: str) -> InvoiceDraft:
    return InvoiceDraft(
        order_id=order_id,
        order_number=order_number,
        lines=[
            InvoiceLine("Arbeitszeit", 2.0, 8900, 17800),
            InvoiceLine("Ölfilter", 3.0, 999, 2997),
        ],
        labor_cents=17800,
        parts_cents=2997,
        net_cents=20797,
        vat_cents=3951,
        gross_cents=24748,
        invoice_number=f"RE-{order_number}",
    )


def test_invoice_and_lines_are_persisted(order_factory) -> None:
    order_id, order_number = order_factory()
    draft = _draft(order_id, order_number)

    saved = store.save_invoice(draft)

    assert saved.order_id == order_id
    assert saved.order_number == order_number
    assert saved.invoice_number == f"RE-{order_number}"
    assert saved.net_cents == 20797
    assert saved.gross_cents == 24748
    assert saved.id > 0

    pool = db.get_pool()
    with pool.connection() as conn:
        row = conn.execute(
            "SELECT labor_cents, parts_cents, net_cents, vat_cents, gross_cents "
            "FROM invoices WHERE order_id = %s",
            (order_id,),
        ).fetchone()
        assert row == (17800, 2997, 20797, 3951, 24748)
        lines = conn.execute(
            "SELECT quantity, unit_price_cents, total_cents FROM invoice_lines "
            "WHERE invoice_id = %s ORDER BY id",
            (saved.id,),
        ).fetchall()
    assert lines == [(2, 8900, 17800), (3, 999, 2997)]


def test_saving_twice_leaves_exactly_one_invoice(order_factory) -> None:
    order_id, order_number = order_factory()
    draft = _draft(order_id, order_number)

    first = store.save_invoice(draft)
    second = store.save_invoice(draft)

    assert second == first

    pool = db.get_pool()
    with pool.connection() as conn:
        invoice_count = conn.execute(
            "SELECT count(*) FROM invoices WHERE order_id = %s", (order_id,)
        ).fetchone()[0]
        line_count = conn.execute(
            "SELECT count(*) FROM invoice_lines WHERE invoice_id = %s", (first.id,)
        ).fetchone()[0]
    assert invoice_count == 1
    assert line_count == 2


def test_second_save_keeps_the_existing_invoice(order_factory) -> None:
    order_id, order_number = order_factory()
    first = store.save_invoice(_draft(order_id, order_number))

    changed = _draft(order_id, order_number)
    changed.gross_cents = 999999
    changed.net_cents = 1
    second = store.save_invoice(changed)

    assert second.id == first.id
    assert second.invoice_number == first.invoice_number
    assert second.gross_cents == first.gross_cents

    pool = db.get_pool()
    with pool.connection() as conn:
        count = conn.execute(
            "SELECT count(*) FROM invoices WHERE order_id = %s", (order_id,)
        ).fetchone()[0]
        gross = conn.execute(
            "SELECT gross_cents FROM invoices WHERE order_id = %s", (order_id,)
        ).fetchone()[0]
    assert count == 1
    assert gross == first.gross_cents
