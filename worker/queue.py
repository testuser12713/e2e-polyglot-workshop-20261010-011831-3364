"""Valkey queue consumer for the invoice worker.

The queue is the fixed Valkey list ``invoices:queue``. The API pushes a JSON
payload ``{"order_id": int, "order_number": str}``. The worker blocking-pops
from the left, parses the payload with ``json`` and validates it against
:class:`~worker.models.QueueMessage` before anything else touches it.
"""

from __future__ import annotations

import json
from typing import Any

import valkey

from .errors import WorkerError
from .models import QueueMessage

QUEUE_KEY = "invoices:queue"


def parse_message(raw: str | bytes) -> QueueMessage:
    """Parse and validate one raw queue payload.

    Raises :class:`WorkerError` with code ``validation_error`` for anything
    that is not a well-formed message. The error never echoes the payload, so
    no customer data can leak into a log line.
    """
    if isinstance(raw, bytes):
        raw = raw.decode("utf-8", errors="replace")
    try:
        payload: Any = json.loads(raw)
    except (TypeError, ValueError) as exc:
        raise WorkerError("validation_error", "queue message is not valid JSON") from exc
    if not isinstance(payload, dict):
        raise WorkerError("validation_error", "queue message must be a JSON object")

    order_id = payload.get("order_id")
    order_number = payload.get("order_number")
    if isinstance(order_id, bool) or not isinstance(order_id, int):
        raise WorkerError("validation_error", "queue message field 'order_id' must be an integer")
    if not isinstance(order_number, str) or not order_number:
        raise WorkerError(
            "validation_error",
            "queue message field 'order_number' must be a non-empty string",
        )
    return QueueMessage(order_id=order_id, order_number=order_number)


class QueueConsumer:
    """A blocking consumer of the ``invoices:queue`` list."""

    def __init__(self, valkey_url: str, client: valkey.Valkey | None = None) -> None:
        self._url = valkey_url
        self._client = client

    @property
    def client(self) -> valkey.Valkey:
        if self._client is None:
            raise WorkerError("internal_error", "queue consumer is not connected")
        return self._client

    def connect(self) -> None:
        """Open the Valkey connection (idempotent)."""
        if self._client is None:
            self._client = valkey.Valkey.from_url(self._url, decode_responses=True)

    def pop(self, timeout: int = 5) -> QueueMessage | None:
        """Blocking-pop one message and parse it.

        Waits up to ``timeout`` seconds for a message and returns ``None`` when
        the wait elapsed, so the caller can react to a shutdown request between
        polls. A signal delivered while the read is blocked makes the socket
        report a timeout too; that is the same "nothing arrived" outcome.
        """
        try:
            item = self.client.blpop([QUEUE_KEY], timeout=timeout)
        except (TimeoutError, valkey.exceptions.TimeoutError):
            return None
        if item is None:
            return None
        _, raw = item
        return parse_message(raw)

    def ack(self, message: QueueMessage) -> None:
        """Acknowledge a consumed message.

        ``invoices:queue`` is a plain list and the blocking pop already removed
        the message atomically, so acknowledging is a no-op. The method exists
        so the orchestration reads in its final form and a future at-least-once
        upgrade (processing list + ``LREM``) has a seam.
        """
        return None

    def close(self) -> None:
        if self._client is not None:
            self._client.close()
            self._client = None
