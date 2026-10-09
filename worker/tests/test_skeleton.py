"""Skeleton tests for the invoice worker.

These cover what the skeleton itself delivers: parsing and validating queue
messages, the uniform error shape, and consuming from a real Valkey when one is
configured. The invoice, persistence and outbox bodies are stubs here and are
tested by their own tickets — this file asserts only the wiring.
"""

from __future__ import annotations

import json
import os

import pytest

from worker import invoice, outbox, store
from worker.config import load_config
from worker.errors import WorkerError
from worker.models import QueueMessage
from worker.queue import QUEUE_KEY, QueueConsumer, parse_message

WELL_FORMED = {"order_id": 42, "order_number": "2026-0001"}


def test_well_formed_message_is_parsed_into_queue_message() -> None:
    msg = parse_message(json.dumps(WELL_FORMED))
    assert msg == QueueMessage(order_id=42, order_number="2026-0001")


def test_bytes_payload_is_parsed_too() -> None:
    msg = parse_message(b'{"order_id": 7, "order_number": "A-7"}')
    assert msg.order_id == 7
    assert msg.order_number == "A-7"


@pytest.mark.parametrize(
    "raw",
    [
        "not json",
        "{",
        "[]",
        '"a string"',
        "null",
        "{}",
        '{"order_id": "42", "order_number": "A-1"}',
        '{"order_id": 42}',
        '{"order_number": "A-1"}',
        '{"order_id": true, "order_number": "A-1"}',
        '{"order_id": 42, "order_number": ""}',
    ],
)
def test_malformed_payload_is_rejected_with_uniform_error(raw: str) -> None:
    with pytest.raises(WorkerError) as exc_info:
        parse_message(raw)
    error = exc_info.value
    assert error.code == "validation_error"
    assert error.status == 400
    assert error.as_body() == {"error": {"code": "validation_error", "message": error.message}}


def test_error_body_matches_the_shared_api_shape() -> None:
    body = None
    try:
        parse_message("{")
    except WorkerError as exc:
        body = exc.as_body()
    assert body is not None
    assert set(body) == {"error"}
    assert set(body["error"]) == {"code", "message"}
    assert isinstance(body["error"]["code"], str)
    assert isinstance(body["error"]["message"], str)


def test_malformed_message_does_not_leak_the_payload() -> None:
    secret = "AC-1234"
    with pytest.raises(WorkerError) as exc_info:
        parse_message(json.dumps({"order_id": secret, "order_number": secret}))
    assert secret not in exc_info.value.message


def test_config_names_the_missing_variable(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.delenv("DATABASE_URL", raising=False)
    monkeypatch.delenv("VALKEY_URL", raising=False)
    with pytest.raises(WorkerError) as exc_info:
        load_config()
    assert "DATABASE_URL" in exc_info.value.message


def test_config_reads_hourly_rate(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv("DATABASE_URL", "postgresql://example/db")
    monkeypatch.setenv("VALKEY_URL", "redis://example:6379/0")
    monkeypatch.setenv("HOURLY_RATE_CENTS", "12000")
    config = load_config()
    assert config.hourly_rate_cents == 12000


def test_pipeline_modules_are_wired() -> None:
    assert callable(invoice.build_invoice)
    assert callable(store.save_invoice)
    assert callable(outbox.enqueue_notification)


@pytest.mark.skipif(not os.environ.get("VALKEY_URL"), reason="VALKEY_URL is not set")
def test_listener_consumes_a_message_from_real_valkey() -> None:
    consumer = QueueConsumer(os.environ["VALKEY_URL"])
    consumer.connect()
    try:
        consumer.client.delete(QUEUE_KEY)
        consumer.client.rpush(QUEUE_KEY, json.dumps({"order_id": 99, "order_number": "2026-0099"}))
        message = consumer.pop(timeout=5)
        assert message == QueueMessage(order_id=99, order_number="2026-0099")
        assert consumer.client.llen(QUEUE_KEY) == 0
        consumer.ack(message)
    finally:
        consumer.close()
