"""Exercise advisory writer lease contention using synthetic owner strings."""

from __future__ import annotations

import importlib
import tempfile
from pathlib import Path
from typing import Any


def fixtures() -> list[dict[str, Any]]:
    module: Any = importlib.import_module("memento.repository.lease")
    results = []
    for owner in ["worker-one", "", " \t\n", " café/日本 ", "first\nsecond", "\x1cwriter\x85"]:
        with tempfile.TemporaryDirectory(prefix="memento-lease-oracle-") as directory:
            path = Path(directory) / "nested" / "writer.lock"
            first = module.acquire_writer_lease(path, owner=owner)
            try:
                try:
                    module.acquire_writer_lease(path, owner="second")
                except module.WriterLeaseError as exc:
                    blocked = str(exc)
                results.append(
                    {"owner": owner, "bytes": list(path.read_bytes()), "blocked": blocked}
                )
            finally:
                first.release()
            with module.acquire_writer_lease(path, owner="second"):
                if path.read_text() != "second\n":
                    raise AssertionError("lease release/reacquire failed")
    return results
