"""PostgreSQL access for the invoice worker.

The worker keeps exactly one psycopg connection pool per process. The pool is
created and opened by the entry point; every module that needs a connection
reads it through :func:`get_pool`.
"""

from __future__ import annotations

from psycopg_pool import ConnectionPool

from .errors import WorkerError

_pool: ConnectionPool | None = None


def open_pool(database_url: str) -> ConnectionPool:
    """Create (once) and open the process-wide connection pool."""
    global _pool
    if _pool is None:
        _pool = ConnectionPool(
            conninfo=database_url,
            min_size=1,
            max_size=4,
            open=False,
        )
        _pool.open()
    return _pool


def get_pool() -> ConnectionPool:
    """Return the open connection pool, or fail if the worker is not started."""
    if _pool is None:
        raise WorkerError("internal_error", "database pool is not initialized")
    return _pool


def close_pool() -> None:
    """Close and forget the pool (idempotent)."""
    global _pool
    if _pool is not None:
        _pool.close()
        _pool = None
