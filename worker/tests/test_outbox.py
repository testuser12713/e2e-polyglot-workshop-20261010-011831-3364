"""Tests for the customer notification outbox.

These run against the real PostgreSQL the office provides (``DATABASE_URL``,
the same variable the worker itself is configured with). The test builds the
tables it uses in its own schema, seeds one customer, vehicle, order and
invoice, and then checks what the ticket promises: a row with the right
recipient and gross amount, no duplicate on redelivery, and no customer
personal data in any log line (AC-07, AC-24). No e-mail is ever sent.
"""

from __future__ import annotations

import logging
import os
import smtplib
import uuid
from dataclasses import dataclass

import psycopg
import pytest

from worker.models import SavedInvoice
from worker.outbox import enqueue_notification

DATABASE_URL = os.environ.get("DATABASE_URL")

pytestmark = pytest.mark.skipif(
    not DATABASE_URL,
    reason="DATABASE_URL is not set — skipping database-backed test",
)

SCHEMA = "test_outbox"

DDL = (
    f"DROP SCHEMA IF EXISTS {SCHEMA} CASCADE",
    f"CREATE SCHEMA {SCHEMA}",
    f"SET search_path TO {SCHEMA}",
    """
    CREATE TABLE customers (
        id         BIGSERIAL PRIMARY KEY,
        name       TEXT NOT NULL,
        email      TEXT NOT NULL,
        phone      TEXT NOT NULL,
        created_at TIMESTAMPTZ NOT NULL DEFAULT now()
    )
    """,
    """
    CREATE TABLE vehicles (
        id            BIGSERIAL PRIMARY KEY,
        license_plate TEXT NOT NULL UNIQUE,
        brand         TEXT NOT NULL,
        model         TEXT NOT NULL,
        mileage       INTEGER NOT NULL DEFAULT 0,
        created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
    )
    """,
    """
    CREATE TABLE orders (
        id                  BIGSERIAL PRIMARY KEY,
        order_number        TEXT NOT NULL UNIQUE,
        customer_id         BIGINT NOT NULL REFERENCES customers (id),
        vehicle_id          BIGINT NOT NULL REFERENCES vehicles (id),
        status              TEXT NOT NULL DEFAULT 'angefragt',
        requested_date      DATE NOT NULL,
        problem_description TEXT NOT NULL,
        created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
    )
    """,
    """
    CREATE TABLE invoices (
        id             BIGSERIAL PRIMARY KEY,
        invoice_number TEXT NOT NULL UNIQUE,
        order_id       BIGINT NOT NULL UNIQUE REFERENCES orders (id),
        issued_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
        labor_cents    INTEGER NOT NULL DEFAULT 0,
        parts_cents    INTEGER NOT NULL DEFAULT 0,
        net_cents      INTEGER NOT NULL DEFAULT 0,
        vat_cents      INTEGER NOT NULL DEFAULT 0,
        gross_cents    INTEGER NOT NULL DEFAULT 0
    )
    """,
    """
    CREATE TABLE outbox (
        id              BIGSERIAL PRIMARY KEY,
        order_id        BIGINT NOT NULL REFERENCES orders (id),
        invoice_id      BIGINT NOT NULL REFERENCES invoices (id),
        recipient_email TEXT NOT NULL,
        subject         TEXT NOT NULL,
        body            TEXT NOT NULL,
        created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
        sent_at         TIMESTAMPTZ
    )
    """,
)


@dataclass
class Seeded:
    """One isolated order/invoice fixture plus the identity values to assert."""

    conn: psycopg.Connection
    saved: SavedInvoice
    email: str
    plate: str
    name: str


def _german_euros(cents: int) -> str:
    return f"{cents / 100:,.2f}".replace(",", "\x00").replace(".", ",").replace("\x00", ".")


@pytest.fixture()
def seeded() -> Seeded:
    assert DATABASE_URL is not None
    conn = psycopg.connect(DATABASE_URL, autocommit=True)
    try:
        with conn.cursor() as cur:
            for statement in DDL:
                cur.execute(statement)

            unique = uuid.uuid4().hex
            email = f"kunde-{unique}@example.com"
            name = f"Max Mustermann {unique}"
            plate = f"B-{unique[:8].upper()}"

            cur.execute(
                "INSERT INTO customers (name, email, phone) VALUES (%s, %s, %s) RETURNING id",
                (name, email, "+49 30 123456"),
            )
            customer_id = cur.fetchone()[0]

            cur.execute(
                "INSERT INTO vehicles (license_plate, brand, model, mileage) "
                "VALUES (%s, %s, %s, %s) RETURNING id",
                (plate, "VW", "Golf", 120000),
            )
            vehicle_id = cur.fetchone()[0]

            order_number = f"2026-{unique[:6].upper()}"
            cur.execute(
                "INSERT INTO orders "
                "(order_number, customer_id, vehicle_id, status, requested_date, problem_description) "
                "VALUES (%s, %s, %s, %s, CURRENT_DATE, %s) RETURNING id",
                (order_number, customer_id, vehicle_id, "fertig", "Bremsen quietschen"),
            )
            order_id = cur.fetchone()[0]

            gross_cents = 123456
            net_cents = 103744
            invoice_number = f"RE-{unique[:6].upper()}"
            cur.execute(
                "INSERT INTO invoices "
                "(invoice_number, order_id, labor_cents, parts_cents, net_cents, vat_cents, gross_cents) "
                "VALUES (%s, %s, %s, %s, %s, %s, %s) RETURNING id",
                (invoice_number, order_id, 8900, 0, net_cents, 19656, gross_cents),
            )
            invoice_id = cur.fetchone()[0]

        saved = SavedInvoice(
            id=invoice_id,
            invoice_number=invoice_number,
            order_id=order_id,
            order_number=order_number,
            gross_cents=gross_cents,
            net_cents=net_cents,
        )
        yield Seeded(conn=conn, saved=saved, email=email, plate=plate, name=name)
    finally:
        try:
            with conn.cursor() as cur:
                cur.execute(f"DROP SCHEMA IF EXISTS {SCHEMA} CASCADE")
        finally:
            conn.close()


def _outbox_rows(conn: psycopg.Connection, order_id: int) -> list[tuple]:
    return conn.execute(
        "SELECT id, order_id, invoice_id, recipient_email, subject, body "
        "FROM outbox WHERE order_id = %s",
        (order_id,),
    ).fetchall()


def test_outbox_row_has_recipient_amount_and_invoice_number(seeded: Seeded) -> None:
    enqueue_notification(seeded.conn, seeded.saved)

    rows = _outbox_rows(seeded.conn, seeded.saved.order_id)
    assert len(rows) == 1
    _, order_id, invoice_id, recipient, subject, body = rows[0]
    assert order_id == seeded.saved.order_id
    assert invoice_id == seeded.saved.id
    assert recipient == seeded.email
    assert seeded.saved.invoice_number in subject
    assert seeded.saved.invoice_number in body
    assert _german_euros(seeded.saved.gross_cents) in body


def test_redelivery_does_not_create_a_second_row(seeded: Seeded) -> None:
    enqueue_notification(seeded.conn, seeded.saved)
    enqueue_notification(seeded.conn, seeded.saved)

    assert len(_outbox_rows(seeded.conn, seeded.saved.order_id)) == 1


def test_processed_message_logs_identify_by_id_and_order_number_only(
    seeded: Seeded, caplog: pytest.LogCaptureFixture
) -> None:
    caplog.set_level(logging.INFO, logger="worker.outbox")

    enqueue_notification(seeded.conn, seeded.saved)
    enqueue_notification(seeded.conn, seeded.saved)

    messages = "\n".join(record.getMessage() for record in caplog.records)
    assert seeded.saved.order_number in messages
    row_id = _outbox_rows(seeded.conn, seeded.saved.order_id)[0][0]
    assert str(row_id) in messages

    assert seeded.email not in messages
    assert seeded.plate not in messages
    assert seeded.name not in messages


def test_no_plain_smtp_client_is_used(seeded: Seeded, monkeypatch: pytest.MonkeyPatch) -> None:
    def _forbidden(*args: object, **kwargs: object) -> None:
        raise AssertionError("the outbox must not send a real e-mail")

    monkeypatch.setattr(smtplib, "SMTP", _forbidden)
    monkeypatch.setattr(smtplib, "SMTP_SSL", _forbidden)

    enqueue_notification(seeded.conn, seeded.saved)
    assert len(_outbox_rows(seeded.conn, seeded.saved.order_id)) == 1
