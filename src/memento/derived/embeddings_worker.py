from __future__ import annotations

import sqlite3
import threading
import time
from collections import deque
from collections.abc import Callable
from dataclasses import dataclass
from pathlib import Path

from memento.activity import ActivityClock
from memento.cpu_usage import CpuUsageSampler
from memento.derived.index import DerivedIndex, DerivedIndexUnavailableError


@dataclass(frozen=True, slots=True)
class ProgressiveEmbeddingPolicy:
    enabled: bool = False
    startup_delay_seconds: float = 120.0
    interactive_idle_seconds: float = 15.0
    delay_seconds: float = 30.0
    cpu_busy_limit_percent: float = 75.0


@dataclass(frozen=True, slots=True)
class SemanticEmbeddingRefreshWorkerState:
    running: bool
    pending: bool
    last_error: str | None
    pause_reason: str | None = None
    current_path: str | None = None
    completed: int = 0
    alive: bool = False


class SemanticEmbeddingRefreshWorker:
    def __init__(
        self,
        derived_index: DerivedIndex,
        *,
        policy: ProgressiveEmbeddingPolicy | None = None,
        activity: ActivityClock | None = None,
        cpu_usage: Callable[[], float | None] | None = None,
        monotonic: Callable[[], float] = time.monotonic,
    ) -> None:
        self._derived_index = derived_index
        self._policy = policy or ProgressiveEmbeddingPolicy()
        self._activity = activity or ActivityClock(monotonic)
        self._cpu_usage = cpu_usage or CpuUsageSampler(monotonic=monotonic).sample
        self._monotonic = monotonic
        self._condition = threading.Condition()
        self._bundle_root: Path | None = None
        self._repo_revision: str | None = None
        self._priority_paths: deque[str] = deque()
        self._full_requested = False
        self._full_generation = 0
        self._persisted_pending = False
        self._running = False
        self._closed = False
        self._last_error: str | None = None
        self._pause_reason: str | None = "startup" if self._policy.enabled else None
        self._current_path: str | None = None
        self._completed = 0
        self._started_at = monotonic()
        self._last_completed_at: float | None = None
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
            return self._has_pending_locked()

    @property
    def last_error(self) -> str | None:
        with self._condition:
            return self._last_error

    def state(self) -> SemanticEmbeddingRefreshWorkerState:
        with self._condition:
            return SemanticEmbeddingRefreshWorkerState(
                alive=self._thread.is_alive() and not self._closed,
                running=self._running,
                pending=self._has_pending_locked(),
                last_error=(
                    self._last_error
                    or (
                        "embedding worker stopped unexpectedly"
                        if not self._closed and not self._thread.is_alive()
                        else None
                    )
                ),
                pause_reason=self._pause_reason,
                current_path=self._current_path,
                completed=self._completed,
            )

    def enqueue(
        self,
        bundle_root: Path,
        repo_revision: str,
        *,
        paths: tuple[str, ...] | None = None,
    ) -> bool:
        with self._condition:
            if self._closed or not self._thread.is_alive():
                return False
            self._bundle_root = bundle_root
            self._repo_revision = repo_revision
            self._persisted_pending = self._policy.enabled
            if paths is None:
                self._full_requested = True
                self._full_generation += 1
            else:
                existing = set(self._priority_paths)
                for path in paths:
                    if path not in existing:
                        self._priority_paths.append(path)
                        existing.add(path)
            self._condition.notify_all()
            return True

    def wait_idle(self, *, timeout_seconds: float) -> bool:
        deadline = time.monotonic() + timeout_seconds
        with self._condition:
            while self._running or self._has_pending_locked():
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
            self._priority_paths.clear()
            self._full_requested = False
            self._persisted_pending = False
            self._condition.notify_all()
        self._thread.join()

    def _run(self) -> None:
        try:
            self._run_loop()
        finally:
            with self._condition:
                self._running = False
                self._current_path = None
                if not self._closed:
                    self._last_error = self._last_error or "embedding worker stopped unexpectedly"
                    self._pause_reason = "worker-stopped"
                self._condition.notify_all()

    def _run_loop(self) -> None:
        while True:
            request: tuple[Path, str, str | None, int] | None = None
            attempted = False
            try:
                with self._condition:
                    if self._closed:
                        return
                # SQLite polling must not hold the condition used by status/enqueue/close.
                request = self._next_request()
                with self._condition:
                    if self._closed:
                        return
                    if request is None:
                        self._pause_reason = None
                        self._condition.wait(timeout=1.0)
                        continue
                    pause = self._pause_reason_for_work_locked()
                    if pause is not None:
                        self._pause_reason = pause[0]
                        self._condition.wait(timeout=min(1.0, max(0.01, pause[1])))
                        continue
                    bundle_root, repo_revision, path, generation = request
                    self._pause_reason = None
                    self._running = True
                    self._current_path = path
                attempted = True
                if path is None:
                    self._derived_index.refresh_embeddings(bundle_root, repo_revision=repo_revision)
                else:
                    self._derived_index.refresh_embedding_paths(
                        bundle_root, repo_revision=repo_revision, paths=(path,)
                    )
            except Exception as exc:
                transient = _is_transient_database_error(exc)
                with self._condition:
                    self._last_error = f"{type(exc).__name__}: {exc}"[:500]
                    self._running = False
                    self._current_path = None
                    self._pause_reason = "database-busy" if transient else "error"
                    # Preserve selected/full work on contention. Permanent work failures
                    # are consumed as before; failed polling leaves all requests intact.
                    if attempted and request is not None and not transient:
                        self._finish_request_locked(request)
                    self._condition.notify_all()
                    if self._closed:
                        return
                    # Bounded retry rate; close/enqueue wake this wait immediately.
                    self._condition.wait(timeout=1.0)
            else:
                with self._condition:
                    self._last_error = None
                    self._completed += 1
                    self._last_completed_at = self._monotonic()
                    self._finish_request_locked(request)
                    self._running = False
                    self._current_path = None
                    self._condition.notify_all()

    def _finish_request_locked(self, request: tuple[Path, str, str | None, int]) -> None:
        _root, _revision, path, generation = request
        if path is None and generation == self._full_generation:
            self._full_requested = False
        elif path is not None and self._priority_paths and self._priority_paths[0] == path:
            self._priority_paths.popleft()

    def _next_request(self) -> tuple[Path, str, str | None, int] | None:
        with self._condition:
            if self._bundle_root is None or self._repo_revision is None:
                return None
            root, revision = self._bundle_root, self._repo_revision
            generation = self._full_generation
            if self._priority_paths:
                return root, revision, self._priority_paths[0], generation
            if not self._policy.enabled:
                return (root, revision, None, generation) if self._full_requested else None
        pending = self._derived_index.pending_embedding_paths(limit=1)
        with self._condition:
            if (
                root != self._bundle_root
                or revision != self._repo_revision
                or generation != self._full_generation
            ):
                return None
            self._persisted_pending = bool(pending)
            if self._priority_paths:
                return root, revision, self._priority_paths[0], generation
            if pending:
                return root, revision, pending[0], generation
            self._full_requested = False
            self._condition.notify_all()
            return None

    def _pause_reason_for_work_locked(self) -> tuple[str, float] | None:
        if not self._policy.enabled:
            return None
        now = self._monotonic()
        startup_remaining = self._policy.startup_delay_seconds - (now - self._started_at)
        if startup_remaining > 0:
            return "startup", startup_remaining
        idle_remaining = self._policy.interactive_idle_seconds - self._activity.idle_seconds()
        if idle_remaining > 0:
            return "interactive", idle_remaining
        cpu_busy = self._cpu_usage()
        if cpu_busy is None:
            return "cpu-sampling", 1.0
        if cpu_busy > self._policy.cpu_busy_limit_percent:
            return "cpu", 1.0
        if self._last_completed_at is not None:
            delay_remaining = self._policy.delay_seconds - (now - self._last_completed_at)
            if delay_remaining > 0:
                return "pacing", delay_remaining
        return None

    def _has_pending_locked(self) -> bool:
        return bool(self._priority_paths or self._full_requested or self._persisted_pending)


def _is_transient_database_error(exc: Exception) -> bool:
    if isinstance(exc, DerivedIndexUnavailableError):
        return True
    if not isinstance(exc, sqlite3.OperationalError):
        return False
    code = getattr(exc, "sqlite_errorcode", 0)
    return (code & 0xFF) in {sqlite3.SQLITE_BUSY, sqlite3.SQLITE_LOCKED} or any(
        marker in str(exc).casefold()
        for marker in ("database is locked", "database is busy", "database table is locked")
    )
