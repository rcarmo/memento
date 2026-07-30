from __future__ import annotations

import threading
import time
from collections.abc import Callable


class ActivityClock:
    """Track the most recent interactive request using a monotonic clock."""

    def __init__(self, now: Callable[[], float] = time.monotonic) -> None:
        self._now = now
        self._lock = threading.Lock()
        self._last_activity = now()

    def touch(self) -> None:
        with self._lock:
            self._last_activity = self._now()

    def idle_seconds(self) -> float:
        with self._lock:
            return max(0.0, self._now() - self._last_activity)
