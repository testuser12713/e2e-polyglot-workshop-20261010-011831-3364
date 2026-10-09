"""``python -m worker`` execution shim.

When the process happens to be started from inside this directory, that
directory is on ``sys.path`` and its ``queue.py`` (the Valkey consumer) would
shadow the standard-library ``queue`` module for every later import. Drop the
entry before the rest of the package — and its dependencies — are imported.
"""

from __future__ import annotations

import importlib
import os
import sys

_HERE = os.path.dirname(os.path.abspath(__file__))
sys.path[:] = [entry for entry in sys.path if os.path.abspath(entry or os.getcwd()) != _HERE]

if __name__ == "__main__":
    importlib.import_module(".main", __package__).main()
