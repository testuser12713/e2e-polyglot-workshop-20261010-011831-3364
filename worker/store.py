"""Invoice persistence.

The worker stores one invoice and its invoice lines in a single PostgreSQL
transaction. Idempotency is enforced by the ``UNIQUE`` constraint on
``invoices.order_id``: saving the same order twice returns the invoice that is
already there instead of inserting a second one. Every SQL statement is
parametrized.
"""

from __future__ import annotations

from decimal import ROUND_HALF_UP, Decimal

from psycopg import Connection

from .db import get_pool
from .models import InvoiceDraft, SavedInvoice


def _quantity_to_int(value: float) -> int:
    """Store a line quantity in the integer column, rounded half up."""
    return int(Decimal(str(value)).quantize(Decimal("1"), rounding=ROUND_HALF_UP))


def _existing_invoice(conn: Connection, order_id: int, order_number: str) -> SavedInvoice | None:
    """Return the order's invoice if one already exists (parametrized)."""
    with conn.cursor() as cur:
        cur.execute(
            "SELECT id, invoice_number, order_id, gross_cents, net_cents "
            "FROM invoices WHERE order_id = %s",
            (order_id,),
        )
        row = cur.fetchone()
    if row is None:
        return None
    return SavedInvoice(
        id=row[0],
        invoice_number=row[1],
        order_id=row[2],
        order_number=order_number,
        gross_cents=row[3],
        net_cents=row[4],
    )


def _insert_invoice(conn: Connection, draft: InvoiceDraft) -> tuple[int, str] | None:
    """Insert the invoice and its lines, returning ``(id, number)``.

    Returns ``None`` when a concurrent writer inserted the order's invoice
    first (``ON CONFLICT`` on the ``order_id`` unique constraint).
    """
    with conn.cursor() as cur:
        cur.execute(
            "INSERT INTO invoices (invoice_number, order_id, labor_cents, "
            "parts_cents, net_cents, vat_cents, gross_cents) "
            "VALUES (%s, %s, %s, %s, %s, %s, %s) "
            "ON CONFLICT (order_id) DO NOTHING "
            "RETURNING id, invoice_number",
            (
                draft.invoice_number,
                draft.order_id,
                draft.labor_cents,
                draft.parts_cents,
                draft.net_cents,
                draft.vat_cents,
                draft.gross_cents,
            ),
        )
        inserted = cur.fetchone()
        if inserted is None:
            return None
        invoice_id, invoice_number = inserted
        for line in draft.lines:
            cur.execute(
                "INSERT INTO invoice_lines (invoice_id, kind, description, quantity, "
                "unit_price_cents, total_cents) VALUES (%s, %s, %s, %s, %s, %s)",
                (
                    invoice_id,
                    line.kind,
                    line.description,
                    _quantity_to_int(line.quantity),
                    line.unit_price_cents,
                    line.total_cents,
                ),
            )
    return invoice_id, invoice_number


def save_invoice(draft: InvoiceDraft) -> SavedInvoice:
    """Persist one invoice draft in a single transaction.

    An order that already has an invoice keeps it: the existing record is
    returned unchanged and no second row is written.
    """
    pool = get_pool()
    with pool.connection() as conn, conn.transaction():
        existing = _existing_invoice(conn, draft.order_id, draft.order_number)
        if existing is not None:
            return existing

        inserted = _insert_invoice(conn, draft)
        if inserted is None:
            # A concurrent writer won the race; return the winner's invoice.
            winner = _existing_invoice(conn, draft.order_id, draft.order_number)
            if winner is None:  # pragma: no cover - the unique constraint guarantees a row
                raise RuntimeError("invoice insert conflicted but no invoice was found")
            return winner

        invoice_id, invoice_number = inserted

    return SavedInvoice(
        id=invoice_id,
        invoice_number=invoice_number,
        order_id=draft.order_id,
        order_number=draft.order_number,
        gross_cents=draft.gross_cents,
        net_cents=draft.net_cents,
    )
