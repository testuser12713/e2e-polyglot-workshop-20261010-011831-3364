"""Tests for the invoice calculation (``worker.invoice``).

They run against the real PostgreSQL named by ``DATABASE_URL`` and are skipped
cleanly when it is not set. Each test sets up the order rows it needs and
removes only those rows again.
"""

from __future__ import annotations

import os
from uuid import uuid4

import psycopg
import pytest

from worker import db, invoice
from worker.models import QueueMessage

DATABASE_URL = os.environ.get("DATABASE_URL")

pytestmark = pytest.mark.skipif(
    not DATABASE_URL,
    reason="DATABASE_URL is not set; the invoice tests need the real PostgreSQL",
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
    """Open the process pool against the real database, or skip."""
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
    """Create an order with items and clean it up afterwards."""
    pool = _pool()
    created: list[tuple[int, int, int]] = []

    def _make(items: list[tuple[str, str, float | None, int | None, int | None]]):
        suffix = uuid4().hex[:12]
        with pool.connection() as conn, conn.transaction():
            customer_id = conn.execute(
                "INSERT INTO customers (name, email, phone) VALUES (%s, %s, %s) RETURNING id",
                ("Testkunde", "test@example.invalid", "0000"),
            ).fetchone()[0]
            vehicle_id = conn.execute(
                "INSERT INTO vehicles (license_plate, brand, model, mileage) "
                "VALUES (%s, %s, %s, %s) RETURNING id",
                (f"T-{suffix}", "VW", "Golf", 0),
            ).fetchone()[0]
            order_number = f"TEST-{suffix}"
            order_id = conn.execute(
                "INSERT INTO orders (order_number, customer_id, vehicle_id, "
                "requested_date, problem_description) VALUES (%s, %s, %s, %s, %s) RETURNING id",
                (order_number, customer_id, vehicle_id, "2026-01-02", "Test"),
            ).fetchone()[0]
            for kind, description, hours, quantity, unit_price in items:
                conn.execute(
                    "INSERT INTO order_items (order_id, kind, description, hours, "
                    "quantity, unit_price_cents) VALUES (%s, %s, %s, %s, %s, %s)",
                    (order_id, kind, description, hours, quantity, unit_price),
                )
        created.append((order_id, customer_id, vehicle_id))
        return order_id, order_number

    yield _make

    with pool.connection() as conn, conn.transaction():
        for order_id, customer_id, vehicle_id in created:
            conn.execute("DELETE FROM order_items WHERE order_id = %s", (order_id,))
            conn.execute("DELETE FROM orders WHERE id = %s", (order_id,))
            conn.execute("DELETE FROM vehicles WHERE id = %s", (vehicle_id,))
            conn.execute("DELETE FROM customers WHERE id = %s", (customer_id,))
    db.close_pool()


def test_labor_and_parts_are_summed_in_whole_cents(order_factory, monkeypatch) -> None:
    monkeypatch.setenv("HOURLY_RATE_CENTS", "8900")
    order_id, order_number = order_factory(
        [
            ("labor", "Arbeitszeit", 2.50, None, None),
            ("part", "Ölfilter", None, 3, 999),
        ]
    )

    draft = invoice.build_invoice(QueueMessage(order_id=order_id, order_number=order_number))

    assert draft.labor_cents == 22250  # 2.50 h * 8900 ct
    assert draft.parts_cents == 2997  # 3 * 999 ct
    assert draft.net_cents == 25247
    assert draft.vat_cents == 4797  # 25247 * 0.19 = 4796.93 -> 4797
    assert draft.gross_cents == 30044
    assert draft.invoice_number == f"RE-{order_number}"

    labor_line = next(line for line in draft.lines if line.description == "Arbeitszeit")
    assert labor_line.quantity == pytest.approx(2.50)
    assert labor_line.unit_price_cents == 8900
    assert labor_line.total_cents == 22250

    part_line = next(line for line in draft.lines if line.description == "Ölfilter")
    assert part_line.quantity == pytest.approx(3.0)
    assert part_line.unit_price_cents == 999
    assert part_line.total_cents == 2997


def test_vat_is_nineteen_percent_of_the_net(order_factory, monkeypatch) -> None:
    monkeypatch.setenv("HOURLY_RATE_CENTS", "10000")
    order_id, order_number = order_factory([("labor", "Arbeitszeit", 1.00, None, None)])

    draft = invoice.build_invoice(QueueMessage(order_id=order_id, order_number=order_number))

    assert draft.net_cents == 10000
    assert draft.vat_cents == 1900
    assert draft.gross_cents == 11900
    assert draft.net_cents + draft.vat_cents == draft.gross_cents


def test_labor_rounds_to_whole_cents(order_factory, monkeypatch) -> None:
    monkeypatch.setenv("HOURLY_RATE_CENTS", "9999")
    order_id, order_number = order_factory([("labor", "Kleinarbeit", 0.01, None, None)])

    draft = invoice.build_invoice(QueueMessage(order_id=order_id, order_number=order_number))

    assert draft.labor_cents == 100  # 9999 * 0.01 = 99.99 ct -> 100 ct
    assert draft.net_cents == 100
    assert draft.vat_cents == 19
    assert draft.gross_cents == 119


def test_labor_rounds_half_up(order_factory, monkeypatch) -> None:
    monkeypatch.setenv("HOURLY_RATE_CENTS", "10")
    order_id, order_number = order_factory([("labor", "Runde", 0.05, None, None)])

    draft = invoice.build_invoice(QueueMessage(order_id=order_id, order_number=order_number))

    assert draft.labor_cents == 1  # 10 * 0.05 = 0.5 ct -> 1 ct


def test_zero_item_order_yields_a_zero_invoice(order_factory, monkeypatch) -> None:
    monkeypatch.setenv("HOURLY_RATE_CENTS", "8900")
    order_id, order_number = order_factory([])

    draft = invoice.build_invoice(QueueMessage(order_id=order_id, order_number=order_number))

    assert draft.lines == []
    assert draft.labor_cents == 0
    assert draft.parts_cents == 0
    assert draft.net_cents == 0
    assert draft.vat_cents == 0
    assert draft.gross_cents == 0
    assert draft.invoice_number == f"RE-{order_number}"
