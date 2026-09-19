"""Bounded, serial POSIX pipe transport for an optional idle-capped child."""

from __future__ import annotations

import os
import selectors
import signal
import struct
import subprocess
import threading
import time
from collections.abc import Callable, Iterator
from contextlib import contextmanager, suppress

from memento.semantic import SemanticSearchError

_MAX_REQUESTS = 256
_POLL_SECONDS = 0.05
_STDERR_LIMIT = 8192


class FramedWorker:
    """Own one child; callers hold session() through response validation/fallback."""

    def __init__(self, idle_seconds: float) -> None:
        if os.name != "posix":
            raise ValueError("warm embedding workers require POSIX pipes")
        self._idle_seconds = idle_seconds
        self._lock = threading.Lock()
        self._closed = threading.Event()
        self._process: subprocess.Popen[bytes] | None = None
        self._command: list[str] = []
        self._environment: dict[str, str] = {}
        self._timer: threading.Timer | None = None
        self._generation = 0
        self._expiry_token = 0
        self._requests = 0
        self._starts = 0
        self._completed = 0
        self._stderr = bytearray()
        self._status: dict[str, object] = {}
        self._publish()

    @property
    def status(self) -> dict[str, object]:
        # Replaced as a whole under the lock; readers never wait for inference.
        return dict(self._status)

    def _publish(self, *, active: bool = False, idle_deadline: float | None = None) -> None:
        self._status = {
            "pid": self._process.pid if self._process else None,
            "generation": self._generation,
            "starts": self._starts,
            "completed_requests": self._completed,
            "active": active,
            "idle_deadline_monotonic": idle_deadline,
            "closed": self._closed.is_set(),
        }

    def _check(self, deadline: float, cancelled: Callable[[], bool] | None) -> float:
        if self._closed.is_set():
            raise SemanticSearchError("embedding client closed")
        if cancelled is not None and cancelled():
            raise SemanticSearchError("embedding cancelled")
        remaining = deadline - time.monotonic()
        if remaining <= 0:
            raise SemanticSearchError("embedding worker deadline exceeded")
        return remaining

    @contextmanager
    def session(self, deadline: float, cancelled: Callable[[], bool] | None) -> Iterator[None]:
        while True:
            remaining = self._check(deadline, cancelled)
            if self._lock.acquire(timeout=min(_POLL_SECONDS, remaining)):
                break
        try:
            self._check(deadline, cancelled)
            self._cancel_expiry()
            self._publish(active=True)
            try:
                yield
                self._completed += 1
                self._schedule_expiry()
            except BaseException:
                self.discard()
                raise
        finally:
            self._lock.release()

    def _cancel_expiry(self) -> None:
        self._expiry_token += 1
        if self._timer is not None:
            self._timer.cancel()
            self._timer = None

    def _schedule_expiry(self) -> None:
        if self._process is None or self._closed.is_set():
            self.discard()
            return
        token, generation = self._expiry_token, self._generation
        self._timer = threading.Timer(self._idle_seconds, self._expire, args=(token, generation))
        self._timer.daemon = True
        self._publish(idle_deadline=time.monotonic() + self._idle_seconds)
        self._timer.start()

    def _expire(self, token: int, generation: int) -> None:
        with self._lock:
            if token == self._expiry_token and generation == self._generation:
                self.discard()

    def discard(self) -> None:
        """Kill and reap the current generation. Requires the session lock."""
        self._cancel_expiry()
        process = self._process
        if process is None:
            self._publish()
            return
        # Kill the process group too: a wrapper must not leave descendants or pipes.
        with suppress(ProcessLookupError):
            os.killpg(process.pid, signal.SIGKILL)
        try:
            process.wait(timeout=1.0)
        except subprocess.TimeoutExpired as exc:
            # Keep ownership so close()/the next request can try reaping again.
            raise SemanticSearchError("embedding worker could not be reaped") from exc
        finally:
            # Even a failed reap must never allow reuse of closed pipes.
            self._command = []
            self._environment = {}
            for stream in (process.stdin, process.stdout, process.stderr):
                if stream is not None:
                    stream.close()
        self._process = None
        self._command = []
        self._stderr.clear()
        self._publish()

    def close(self) -> None:
        self._closed.set()
        # Active I/O checks the event every 50ms, then has a 1s reap allowance.
        if not self._lock.acquire(timeout=2.0):
            raise SemanticSearchError("embedding worker close timed out")
        try:
            self.discard()
        finally:
            self._lock.release()

    def exchange(
        self,
        command: list[str],
        environment: dict[str, str],
        wire: bytes,
        *,
        deadline: float,
        cancelled: Callable[[], bool] | None,
        max_response: int,
    ) -> bytes:
        """Exchange one frame within session(); never trust its declared length."""
        self._check(deadline, cancelled)
        process = self._process
        if process is not None and (
            process.poll() is not None
            or self._command != command
            or self._environment != environment
            or self._requests >= _MAX_REQUESTS
        ):
            self.discard()
            process = None
        try:
            self._check(deadline, cancelled)
            if process is None:
                process = subprocess.Popen(
                    command,
                    stdin=subprocess.PIPE,
                    stdout=subprocess.PIPE,
                    stderr=subprocess.PIPE,
                    bufsize=0,
                    env=environment,
                    start_new_session=True,
                )
                self._process = process
                self._command = list(command)
                self._environment = dict(environment)
                self._generation += 1
                self._starts += 1
                self._requests = 0
            assert process.stdin is not None
            assert process.stdout is not None
            assert process.stderr is not None
            self._publish(active=True)
            result = self._transfer(
                process.stdin.fileno(),
                process.stdout.fileno(),
                process.stderr.fileno(),
                wire,
                deadline,
                cancelled,
                max_response,
            )
        except OSError as exc:
            raise SemanticSearchError("embedding worker pipe failed") from exc
        self._requests += 1
        return result

    def _transfer(
        self,
        stdin: int,
        stdout: int,
        stderr: int,
        wire: bytes,
        deadline: float,
        cancelled: Callable[[], bool] | None,
        max_response: int,
    ) -> bytes:
        response = bytearray()
        expected: int | None = None
        sent = 0
        with selectors.DefaultSelector() as selector:
            for stream, event, name in (
                (stdin, selectors.EVENT_WRITE, "stdin"),
                (stdout, selectors.EVENT_READ, "stdout"),
                (stderr, selectors.EVENT_READ, "stderr"),
            ):
                os.set_blocking(stream, False)
                selector.register(stream, event, name)
            while True:
                remaining = self._check(deadline, cancelled)
                for key, _mask in selector.select(min(_POLL_SECONDS, remaining)):
                    try:
                        if key.data == "stdin":
                            sent += os.write(key.fd, wire[sent : sent + 65536])
                            if sent == len(wire):
                                selector.unregister(stdin)
                            continue
                        # One look-ahead byte detects excess data without buffering
                        # a full extra 64 KiB chunk beyond the declared frame.
                        limit = 65536
                        if key.data == "stdout":
                            limit = (
                                4 - len(response)
                                if expected is None
                                else expected - len(response) + 1
                            )
                        chunk = os.read(key.fd, min(65536, limit))
                    except BlockingIOError:
                        continue
                    if key.data == "stderr":
                        if not chunk:
                            selector.unregister(stderr)
                        else:
                            self._stderr.extend(chunk)
                            del self._stderr[:-_STDERR_LIMIT]
                        continue
                    if not chunk:
                        raise SemanticSearchError("embedding worker returned a truncated frame")
                    response.extend(chunk)
                    if expected is None and len(response) >= 4:
                        size = struct.unpack_from("<I", response)[0]
                        if not 4 <= size <= max_response:
                            raise SemanticSearchError(
                                "embedding worker response exceeds frame limit"
                            )
                        expected = size + 4
                    if expected is not None and len(response) >= expected:
                        if len(response) != expected or sent != len(wire):
                            raise SemanticSearchError("embedding worker returned an invalid frame")
                        self._check(deadline, cancelled)
                        return bytes(response)
