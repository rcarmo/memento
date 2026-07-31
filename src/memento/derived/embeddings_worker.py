from __future__ import annotations

import threading
import time
from collections import deque
from collections.abc import Callable
from dataclasses import dataclass
from pathlib import Path

from memento.activity import ActivityClock
from memento.cpu_usage import CpuUsageSampler
from memento.derived.index import DerivedIndex


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
                running=self._running,
                pending=self._has_pending_locked(),
                last_error=self._last_error,
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
            if self._closed:
                return False
            self._bundle_root = bundle_root
            self._repo_revision = repo_revision
            if paths is None:
                self._full_requested = True
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
            while self._running or self._priority_paths or self._full_requested:
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
            self._condition.notify_all()
        self._thread.join()

    def _run(self) -> None:
        while True:
            with self._condition:
                if self._closed:
                    self._condition.notify_all()
                    return
                request = self._next_request_locked()
                if request is None:
                    self._condition.wait(timeout=1.0 if self._policy.enabled else None)
                    continue
                pause = self._pause_reason_for_work_locked()
                if pause is not None:
                    self._pause_reason = pause[0]
                    self._condition.wait(timeout=min(1.0, max(0.01, pause[1])))
                    continue
                bundle_root, repo_revision, path = request
                self._pause_reason = None
                self._running = True
                self._current_path = path
            try:
                if path is None:
                    self._derived_index.refresh_embeddings(bundle_root, repo_revision=repo_revision)
                else:
                    self._derived_index.refresh_embedding_paths(
                        bundle_root, repo_revision=repo_revision, paths=(path,)
                    )
            except Exception as exc:
                with self._condition:
                    self._last_error = str(exc)
                    if (
                        path is not None
                        and self._priority_paths
                        and self._priority_paths[0] == path
                    ):
                        self._priority_paths.popleft()
            else:
                with self._condition:
                    self._last_error = None
                    self._completed += 1
                    self._last_completed_at = self._monotonic()
                    if (
                        path is not None
                        and self._priority_paths
                        and self._priority_paths[0] == path
                    ):
                        self._priority_paths.popleft()
            finally:
                with self._condition:
                    self._running = False
                    self._current_path = None
                    self._condition.notify_all()

    def _next_request_locked(self) -> tuple[Path, str, str | None] | None:
        if self._bundle_root is None or self._repo_revision is None:
            return None
        if not self._policy.enabled:
            if self._priority_paths:
                return self._bundle_root, self._repo_revision, self._priority_paths[0]
            if self._full_requested:
                self._full_requested = False
                return self._bundle_root, self._repo_revision, None
            return None
        if self._priority_paths:
            return self._bundle_root, self._repo_revision, self._priority_paths[0]
        pending = self._derived_index.pending_embedding_paths(limit=1)
        if pending:
            return self._bundle_root, self._repo_revision, pending[0]
        self._full_requested = False
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
        if self._priority_paths or self._full_requested:
            return True
        if self._policy.enabled and self._bundle_root is not None:
            return bool(self._derived_index.pending_embedding_paths(limit=1))
        return False
