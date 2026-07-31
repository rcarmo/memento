from __future__ import annotations

import time
from collections.abc import Callable
from pathlib import Path


class CpuUsageSampler:
    """Sample aggregate CPU busy time from /proc/stat over a monotonic window."""

    def __init__(
        self,
        *,
        window_seconds: float = 15.0,
        stat_path: Path = Path("/proc/stat"),
        monotonic: Callable[[], float] = time.monotonic,
    ) -> None:
        self._window_seconds = window_seconds
        self._stat_path = stat_path
        self._monotonic = monotonic
        self._previous: tuple[float, int, int] | None = None

    def sample(self) -> float | None:
        now = self._monotonic()
        busy, total = self._read()
        previous = self._previous
        if previous is None:
            self._previous = (now, busy, total)
            return None
        previous_at, previous_busy, previous_total = previous
        if now - previous_at < self._window_seconds:
            return None
        self._previous = (now, busy, total)
        total_delta = total - previous_total
        busy_delta = busy - previous_busy
        if total_delta <= 0 or busy_delta < 0:
            return None
        return min(100.0, max(0.0, busy_delta * 100.0 / total_delta))

    def _read(self) -> tuple[int, int]:
        line = self._stat_path.read_text(encoding="utf-8").splitlines()[0]
        parts = line.split()
        if not parts or parts[0] != "cpu" or len(parts) < 5:
            raise ValueError("invalid aggregate CPU counters")
        counters = [int(value) for value in parts[1:]]
        idle = counters[3]
        iowait = counters[4] if len(counters) > 4 else 0
        total = sum(counters)
        busy = total - idle - iowait
        return busy, total
