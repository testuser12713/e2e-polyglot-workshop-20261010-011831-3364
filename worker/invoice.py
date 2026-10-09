"""Invoice construction.

Skeleton stub: the body is filled in by the invoice-calculation ticket. The
signature and the return shape are already final so ``worker.main`` can wire
the whole pipeline.
"""

from __future__ import annotations

from .models import InvoiceDraft, QueueMessage


def build_invoice(msg: QueueMessage) -> InvoiceDraft:
    """Build an invoice draft for one order (empty draft until implemented)."""
    return InvoiceDraft(
        order_id=msg.order_id,
        order_number=msg.order_number,
        lines=[],
        labor_cents=0,
        parts_cents=0,
        net_cents=0,
        vat_cents=0,
        gross_cents=0,
        invoice_number="",
    )
