"""Test bootstrap for the invoice worker.

The office test runner executes ``PYTHONPATH=. pytest`` from this directory,
which puts ``worker/`` itself on ``sys.path``. That would make the standard
library ``queue`` module resolve to ``worker/queue.py`` (the Valkey consumer)
and break every dependency that imports it. The installed ``worker`` package
does not need that path entry, so drop it before any test module is imported.
"""

from __future__ import annotations

import os
import sys

_HERE = os.path.dirname(os.path.abspath(__file__))
sys.path[:] = [entry for entry in sys.path if os.path.abspath(entry or os.getcwd()) != _HERE]

# Keep the repository root importable so ``import worker`` works from a source
# checkout too, not only from the installed package.
_ROOT = os.path.dirname(_HERE)
if _ROOT not in sys.path:
    sys.path.insert(0, _ROOT)
