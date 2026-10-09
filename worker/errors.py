"""Uniform error handling for the invoice worker.

Every failure the worker reports uses the same body the API returns:
``{"error": {"code": "...", "message": "..."}}``. The message never contains
customer data, SQL fragments, stack traces or file paths.
"""

from __future__ import annotations

import json
from dataclasses import dataclass

# The shared code -> HTTP status mapping. The worker carries the same code so a
# failure reads identically to the API's error body.
ERROR_STATUS: dict[str, int] = {
    "validation_error": 400,
    "unauthorized": 401,
    "not_found": 404,
    "invalid_transition": 409,
    "rate_limited": 429,
    "not_implemented": 501,
    "internal_error": 500,
}

DEFAULT_ERROR_CODE = "internal_error"


@dataclass(frozen=True)
class ErrorRecord:
    """One error line in the shape of the API's error body."""

    code: str
    message: str

    def as_body(self) -> dict[str, dict[str, str]]:
        return {"error": {"code": self.code, "message": self.message}}

    def as_json(self) -> str:
        return json.dumps(self.as_body())


class WorkerError(Exception):
    """A worker failure carrying a uniform code and a safe message."""

    def __init__(self, code: str, message: str) -> None:
        super().__init__(message)
        self.code = code
        self.message = message

    @property
    def status(self) -> int:
        return ERROR_STATUS.get(self.code, ERROR_STATUS[DEFAULT_ERROR_CODE])

    def record(self) -> ErrorRecord:
        return ErrorRecord(code=self.code, message=self.message)

    def as_body(self) -> dict[str, dict[str, str]]:
        return self.record().as_body()

    def as_json(self) -> str:
        return self.record().as_json()
