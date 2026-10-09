"""Customer notification outbox.

Skeleton stub: the body is filled in by the customer-notification ticket. The
signature is already final so ``worker.main`` can wire the whole pipeline. No
real e-mail is ever sent; a row is written to the ``outbox`` table.
"""

from __future__ import annotations

from .models import SavedInvoice


def enqueue_notification(saved: SavedInvoice) -> None:
    """Enqueue a customer notification for a saved invoice (no-op until implemented)."""
    return None
