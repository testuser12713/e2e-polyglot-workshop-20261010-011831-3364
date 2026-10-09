"""Environment configuration of the invoice worker.

Configuration is read lazily: importing this module never touches the
environment. A process that cannot be configured fails with a message that
names the missing variable (see RUN.json). Secrets never have a literal value
in the repository.
"""

from __future__ import annotations

import os
from dataclasses import dataclass

from .errors import WorkerError

DEFAULT_HOURLY_RATE_CENTS = 8900


@dataclass(frozen=True)
class Config:
    """The configuration the worker needs to boot."""

    database_url: str
    valkey_url: str
    hourly_rate_cents: int


def _required(name: str) -> str:
    value = os.environ.get(name)
    if not value:
        raise WorkerError(
            "internal_error",
            f"missing required environment variable {name} (see RUN.json)",
        )
    return value


def load_config() -> Config:
    """Read and validate the worker configuration from the environment."""
    rate_raw = os.environ.get("HOURLY_RATE_CENTS", str(DEFAULT_HOURLY_RATE_CENTS))
    try:
        hourly_rate_cents = int(rate_raw)
    except ValueError as exc:
        raise WorkerError(
            "internal_error",
            "HOURLY_RATE_CENTS must be an integer number of cents",
        ) from exc
    if hourly_rate_cents < 0:
        raise WorkerError("internal_error", "HOURLY_RATE_CENTS must not be negative")
    return Config(
        database_url=_required("DATABASE_URL"),
        valkey_url=_required("VALKEY_URL"),
        hourly_rate_cents=hourly_rate_cents,
    )
