"""Invoice construction.

The worker turns one queued ``QueueMessage`` into a calculated
:class:`~worker.models.InvoiceDraft`. The order's positions are read from
PostgreSQL with parametrized SQL; labor is billed as hours times the configured
hourly rate, parts as quantity times unit price, and 19 % VAT is added on the
net amount. Every amount stays an integer number of cents.
"""

from __future__ import annotations

import os
from decimal import ROUND_HALF_UP, Decimal

from .config import DEFAULT_HOURLY_RATE_CENTS
from .db import get_pool
from .errors import WorkerError
from .models import InvoiceDraft, InvoiceLine, QueueMessage

VAT_RATE = Decimal("0.19")

_KIND_LABOR = "labor"
_KIND_PART = "part"


def hourly_rate_cents() -> int:
    """Read ``HOURLY_RATE_CENTS`` lazily, with the configured default.

    The value is never read at import time; a process that cannot be
    configured fails with a message naming the variable.
    """
    raw = os.environ.get("HOURLY_RATE_CENTS", str(DEFAULT_HOURLY_RATE_CENTS))
    try:
        rate = int(raw)
    except ValueError as exc:
        raise WorkerError(
            "internal_error",
            "HOURLY_RATE_CENTS must be an integer number of cents",
        ) from exc
    if rate < 0:
        raise WorkerError("internal_error", "HOURLY_RATE_CENTS must not be negative")
    return rate


def _round_cents(value: Decimal) -> int:
    """Round a decimal cent amount to a whole number of cents, half up."""
    return int(value.quantize(Decimal("1"), rounding=ROUND_HALF_UP))


def invoice_number_for(order_number: str) -> str:
    """Derive the deterministic, unique invoice number of an order."""
    return f"RE-{order_number}"


def _load_items(order_id: int) -> list[tuple[object, ...]]:
    """Load the order's positions from PostgreSQL (parametrized)."""
    pool = get_pool()
    with pool.connection() as conn, conn.cursor() as cur:
        cur.execute(
            "SELECT kind, description, hours, quantity, unit_price_cents "
            "FROM order_items WHERE order_id = %s ORDER BY id",
            (order_id,),
        )
        return cur.fetchall()


def build_invoice(msg: QueueMessage) -> InvoiceDraft:
    """Build an invoice draft for one order.

    Labor positions are billed at ``HOURLY_RATE_CENTS``, part positions at
    their ``unit_price_cents``. The net is labor plus parts, VAT is 19 % of the
    net, and every amount is a whole number of cents.
    """
    rate = hourly_rate_cents()
    lines: list[InvoiceLine] = []
    labor_cents = 0
    parts_cents = 0

    for kind, description, hours, quantity, unit_price in _load_items(msg.order_id):
        if kind == _KIND_LABOR:
            hours_value = Decimal(hours) if hours is not None else Decimal(0)
            total = _round_cents(hours_value * rate)
            lines.append(
                InvoiceLine(
                    description=description,
                    quantity=float(hours_value),
                    unit_price_cents=rate,
                    total_cents=total,
                )
            )
            labor_cents += total
        elif kind == _KIND_PART:
            quantity_value = int(quantity or 0)
            price_value = int(unit_price or 0)
            total = quantity_value * price_value
            lines.append(
                InvoiceLine(
                    description=description,
                    quantity=float(quantity_value),
                    unit_price_cents=price_value,
                    total_cents=total,
                )
            )
            parts_cents += total
        else:
            raise WorkerError("internal_error", "unsupported order item kind")

    net_cents = labor_cents + parts_cents
    vat_cents = _round_cents(Decimal(net_cents) * VAT_RATE)
    gross_cents = net_cents + vat_cents

    return InvoiceDraft(
        order_id=msg.order_id,
        order_number=msg.order_number,
        lines=lines,
        labor_cents=labor_cents,
        parts_cents=parts_cents,
        net_cents=net_cents,
        vat_cents=vat_cents,
        gross_cents=gross_cents,
        invoice_number=invoice_number_for(msg.order_number),
    )
