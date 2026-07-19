from __future__ import annotations

import threading
import time
from dataclasses import dataclass
from pathlib import Path

from memento.derived.index import DerivedIndex


@dataclass(frozen=True, slots=True)
class SemanticEmbeddingRefreshWorkerState:
    running: bool
    pending: bool
    last_error: str | None


class SemanticEmbeddingRefreshWorker:
    def __init__(self, derived_index: DerivedIndex) -> None:
        self._derived_index = derived_index
        self._condition = threading.Condition()
        self._pending_request: tuple[Path, str] | None = None
        self._running = False
        self._closed = False
        self._last_error: str | None = None
        self._thread = threading.Thread(
            target=self._run,
            name="memento-semantic-refresh",
            daemon=True,
        )
        self._thread.start()

    @property
    def running(self) -> bool:
        with self._condition:
            return self._running

    @property
    def pending(self) -> bool:
        with self._condition:
            return self._pending_request is not None

    @property
    def last_error(self) -> str | None:
        with self._condition:
            return self._last_error

    def state(self) -> SemanticEmbeddingRefreshWorkerState:
        with self._condition:
            return SemanticEmbeddingRefreshWorkerState(
                running=self._running,
                pending=self._pending_request is not None,
                last_error=self._last_error,
            )

    def enqueue(self, bundle_root: Path, repo_revision: str) -> bool:
        with self._condition:
            if self._closed:
                return False
            self._pending_request = (bundle_root, repo_revision)
            self._condition.notify_all()
            return True

    def wait_idle(self, *, timeout_seconds: float) -> bool:
        deadline = time.monotonic() + timeout_seconds
        with self._condition:
            while self._running or self._pending_request is not None:
                remaining = deadline - time.monotonic()
                if remaining <= 0:
                    return False
                self._condition.wait(timeout=remaining)
            return True

    def close(self) -> None:
        with self._condition:
            if self._closed:
                return
            self._closed = True
            self._pending_request = None
            self._condition.notify_all()
        self._thread.join()

    def _run(self) -> None:
        while True:
            with self._condition:
                while self._pending_request is None and not self._closed:
                    self._condition.wait()
                if self._closed:
                    self._condition.notify_all()
                    return
                request = self._pending_request
                self._pending_request = None
                self._running = True
            assert request is not None
            try:
                bundle_root, repo_revision = request
                self._derived_index.refresh_embeddings(bundle_root, repo_revision=repo_revision)
            except Exception as exc:
                with self._condition:
                    self._last_error = str(exc)
            else:
                with self._condition:
                    self._last_error = None
            finally:
                with self._condition:
                    self._running = False
                    self._condition.notify_all()
