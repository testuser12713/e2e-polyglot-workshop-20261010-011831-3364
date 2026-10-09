"""Entry point of the invoice worker (``python -m worker``).

The worker configures logging, opens the PostgreSQL pool, connects to Valkey,
announces that it listens on ``invoices:queue`` and then consumes messages one
at a time: build an invoice, save it, enqueue the customer notification and
acknowledge the message. A failing message is caught and logged in the same
uniform error shape the API returns, and never with customer data.
"""

from __future__ import annotations

import logging
import signal
import time

from psycopg_pool import PoolTimeout

from . import invoice, outbox, store
from .config import load_config
from .db import close_pool, get_pool, open_pool
from .errors import ErrorRecord, WorkerError
from .models import QueueMessage
from .queue import QUEUE_KEY, QueueConsumer

logger = logging.getLogger("worker")


def process_message(msg: QueueMessage) -> None:
    """Run the full pipeline for one message: build, save, notify."""
    draft = invoice.build_invoice(msg)
    saved = store.save_invoice(draft)
    outbox.enqueue_notification(saved)


def log_failure(exc: Exception, msg: QueueMessage | None = None) -> None:
    """Log one failure in the uniform error shape, without customer data."""
    if isinstance(exc, WorkerError):
        record = exc.record()
    else:
        record = ErrorRecord(code="internal_error", message="unexpected worker failure")
    context = f" (order_id={msg.order_id})" if msg is not None else ""
    logger.error("worker error%s: %s", context, record.as_json())


def run() -> None:
    """Boot the worker and consume ``invoices:queue`` until interrupted."""
    logging.basicConfig(
        level=logging.INFO,
        format="%(asctime)s %(levelname)s %(name)s %(message)s",
    )
    config = load_config()
    open_pool(config.database_url)
    try:
        get_pool().wait(timeout=10)
    except PoolTimeout as exc:
        close_pool()
        raise WorkerError(
            "internal_error",
            "database is not reachable (DATABASE_URL)",
        ) from exc

    consumer = QueueConsumer(config.valkey_url)
    consumer.connect()
    logger.info("worker listening on %s", QUEUE_KEY)

    stop = False

    def request_stop(signum: int, frame: object) -> None:
        nonlocal stop
        stop = True

    for sig in (signal.SIGINT, signal.SIGTERM):
        signal.signal(sig, request_stop)

    try:
        while not stop:
            try:
                msg = consumer.pop(timeout=5)
            except WorkerError as exc:
                log_failure(exc)
                continue
            except Exception:
                log_failure(WorkerError("internal_error", "queue is not reachable (VALKEY_URL)"))
                time.sleep(1)
                continue
            if msg is None:
                continue
            try:
                process_message(msg)
                consumer.ack(msg)
                logger.info("processed order %s", msg.order_number)
            except WorkerError as exc:
                log_failure(exc, msg)
            except Exception:
                log_failure(WorkerError("internal_error", "failed to process message"), msg)
    finally:
        consumer.close()
        close_pool()
    logger.info("worker stopped")


def main() -> None:
    try:
        run()
    except WorkerError as exc:
        log_failure(exc)
        raise SystemExit(1) from None
    except Exception:
        log_failure(WorkerError("internal_error", "worker failed to start"))
        raise SystemExit(1) from None
