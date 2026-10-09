"""Shared data shapes of the invoice worker.

These dataclasses are the contract between the queue consumer, the invoice
builder, the persistence layer and the outbox. Other slices import exactly
these names and read exactly these fields.
"""

from __future__ import annotations

from dataclasses import dataclass, field


@dataclass
class QueueMessage:
    """One work item as published by the API to ``invoices:queue``."""

    order_id: int
    order_number: str


@dataclass
class InvoiceLine:
    """One invoice position, all amounts in whole cents.

    ``kind`` is "labor" or "part": it labels the line in the invoice's
    "Position" column.
    """

    description: str
    quantity: float
    unit_price_cents: int
    total_cents: int
    kind: str = ""


@dataclass
class InvoiceDraft:
    """A calculated invoice before it is persisted."""

    order_id: int
    order_number: str
    lines: list[InvoiceLine] = field(default_factory=list)
    labor_cents: int = 0
    parts_cents: int = 0
    net_cents: int = 0
    vat_cents: int = 0
    gross_cents: int = 0
    invoice_number: str = ""


@dataclass
class SavedInvoice:
    """A persisted invoice, as stored in PostgreSQL."""

    id: int
    invoice_number: str
    order_id: int
    order_number: str
    gross_cents: int
    net_cents: int
