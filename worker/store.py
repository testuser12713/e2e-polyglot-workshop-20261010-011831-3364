"""Invoice persistence.

Skeleton stub: the body is filled in by the invoice-calculation ticket. The
signature and the return shape are already final so ``worker.main`` can wire
the whole pipeline. Every SQL statement added here must be parametrized.
"""

from __future__ import annotations

from .models import InvoiceDraft, SavedInvoice


def save_invoice(draft: InvoiceDraft) -> SavedInvoice:
    """Persist an invoice draft and return the saved record (zero until implemented)."""
    return SavedInvoice(
        id=0,
        invoice_number="",
        order_id=0,
        order_number="",
        gross_cents=0,
        net_cents=0,
    )
