from __future__ import annotations

import fcntl
import os
from dataclasses import dataclass
from pathlib import Path
from typing import TextIO


class WriterLeaseError(RuntimeError):
    """Raised when the authoritative repository writer lease cannot be acquired."""


@dataclass(slots=True)
class WriterLease:
    path: Path
    owner: str
    _handle: TextIO

    def release(self) -> None:
        fcntl.flock(self._handle, fcntl.LOCK_UN)
        self._handle.close()

    def __enter__(self) -> WriterLease:
        return self

    def __exit__(self, exc_type: object, exc: object, tb: object) -> None:
        self.release()


def acquire_writer_lease(path: Path, *, owner: str) -> WriterLease:
    path.parent.mkdir(parents=True, exist_ok=True)
    handle = path.open("a+", encoding="utf-8")
    try:
        fcntl.flock(handle, fcntl.LOCK_EX | fcntl.LOCK_NB)
    except BlockingIOError as exc:
        handle.seek(0)
        current_owner = handle.read().strip() or "unknown writer"
        handle.close()
        raise WriterLeaseError(f"writer lease already held by {current_owner}") from exc
    handle.seek(0)
    handle.truncate(0)
    handle.write(owner + os.linesep)
    handle.flush()
    return WriterLease(path=path, owner=owner, _handle=handle)
