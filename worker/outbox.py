"""Customer notification outbox.

The worker never sends a real e-mail: for every saved invoice it writes one row
into the ``outbox`` table. The row carries the order id, the invoice id, the
customer's e-mail address read from the order's customer, a German subject and a
plain-text body naming the invoice number and the gross amount. A redelivered
message must not produce a second letter, so the insert is skipped when a row
for that invoice already exists.

Log lines name the outbox row id and the order number only. The recipient
address, the customer name and the license plate never reach a log line (AC-24),
and every SQL statement is parametrized (AC-19).
"""

from __future__ import annotations

import logging

import psycopg

from .errors import WorkerError
from .models import SavedInvoice

logger = logging.getLogger("worker.outbox")


def _format_euros(cents: int) -> str:
    """Render whole cents as a German euro amount, e.g. 123456 -> ``1234,56``."""
    formatted = f"{cents / 100:,.2f}"
    return formatted.replace(",", "\x00").replace(".", ",").replace("\x00", ".")


def _subject(saved: SavedInvoice) -> str:
    """German subject line for the notification."""
    return f"Ihre Rechnung {saved.invoice_number}"


def _body(saved: SavedInvoice) -> str:
    """Plain-text German body naming the invoice number and the gross amount."""
    amount = _format_euros(saved.gross_cents)
    return (
        "Guten Tag,\n\n"
        f"vielen Dank für Ihren Auftrag. Die Rechnung {saved.invoice_number} "
        f"über {amount} EUR ist ab sofort verfügbar.\n\n"
        "Mit freundlichen Grüßen\n"
        "Ihre Kfz-Werkstatt\n"
    )


def _existing_outbox_id(conn: psycopg.Connection, invoice_id: int) -> int | None:
    """Return the outbox row id already stored for this invoice, if any."""
    row = conn.execute(
        "SELECT id FROM outbox WHERE invoice_id = %s LIMIT 1",
        (invoice_id,),
    ).fetchone()
    return int(row[0]) if row is not None else None


def _recipient_email(conn: psycopg.Connection, order_id: int) -> str | None:
    """Read the customer's e-mail address for an order with parametrized SQL."""
    row = conn.execute(
        "SELECT c.email FROM orders o JOIN customers c ON c.id = o.customer_id WHERE o.id = %s",
        (order_id,),
    ).fetchone()
    return row[0] if row is not None else None


def enqueue_notification(conn: psycopg.Connection, saved: SavedInvoice) -> None:
    """Write one customer notification for ``saved`` into the outbox.

    Idempotent per invoice: a second call for the same invoice inserts nothing.
    No SMTP client is involved; the letter only exists as an outbox row.
    """
    existing = _existing_outbox_id(conn, saved.id)
    if existing is not None:
        logger.info(
            "outbox notification %s already exists for order %s",
            existing,
            saved.order_number,
        )
        return

    recipient = _recipient_email(conn, saved.order_id)
    if recipient is None:
        raise WorkerError(
            "not_found",
            f"no customer found for order {saved.order_number}",
        )

    row = conn.execute(
        "INSERT INTO outbox (order_id, invoice_id, recipient_email, subject, body, created_at) "
        "VALUES (%s, %s, %s, %s, %s, now()) RETURNING id",
        (saved.order_id, saved.id, recipient, _subject(saved), _body(saved)),
    ).fetchone()
    if row is None:
        raise WorkerError("internal_error", "failed to write outbox notification")
    conn.commit()
    logger.info("outbox notification %s queued for order %s", row[0], saved.order_number)
