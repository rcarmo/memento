from __future__ import annotations

import json
import os
import stat
import subprocess
import sys
import time
from collections.abc import Callable, Iterator
from concurrent.futures import Future, ThreadPoolExecutor
from contextlib import suppress
from dataclasses import dataclass
from pathlib import Path
from textwrap import dedent
from typing import Literal, Protocol, TypeVar

import pytest

import memento.framed_worker as framed_worker
from memento.semantic import SemanticSearchError
from memento.subprocess_embeddings import SubprocessEmbeddingClient

T = TypeVar("T")


@dataclass(frozen=True, slots=True)
class WorkerHarness:
    worker: Path
    model: Path
    log: Path
    markers: Path


@pytest.fixture
def worker_harness(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> WorkerHarness:
    worker = tmp_path / "fake_worker.py"
    model = tmp_path / "model.gte"
    log = tmp_path / "worker.jsonl"
    markers = tmp_path / "markers"
    markers.mkdir()
    _write_fake_worker(worker)
    model.write_bytes(b"model")
    monkeypatch.setenv("MEMENTO_FAKE_LOG", str(log))
    monkeypatch.setenv("MEMENTO_FAKE_MARKERS", str(markers))
    return WorkerHarness(worker=worker, model=model, log=log, markers=markers)


class ClientFactory(Protocol):
    def __call__(
        self,
        *,
        max_batch: int = 8,
        max_input_chars: int = 2_000_000,
        timeout_seconds: float = 0.7,
        idle_seconds: float = 0.4,
        backend: Literal["cpu", "vulkan", "auto"] = "cpu",
        vulkan_device: str | None = None,
    ) -> SubprocessEmbeddingClient: ...


@pytest.fixture
def client_factory(
    worker_harness: WorkerHarness,
) -> Iterator[ClientFactory]:
    clients: list[SubprocessEmbeddingClient] = []

    def factory(
        *,
        max_batch: int = 8,
        max_input_chars: int = 2_000_000,
        timeout_seconds: float = 0.7,
        idle_seconds: float = 0.4,
        backend: Literal["cpu", "vulkan", "auto"] = "cpu",
        vulkan_device: str | None = None,
    ) -> SubprocessEmbeddingClient:
        client = SubprocessEmbeddingClient(
            worker_harness.worker,
            worker_harness.model,
            dimensions=2,
            max_batch=max_batch,
            max_input_chars=max_input_chars,
            timeout_seconds=timeout_seconds,
            idle_seconds=idle_seconds,
            backend=backend,
            vulkan_device="fake0"
            if backend == "vulkan" and vulkan_device is None
            else vulkan_device,
        )
        clients.append(client)
        return client

    try:
        yield factory
    finally:
        for client in reversed(clients):
            with suppress(Exception):
                client.close()


def _write_fake_worker(path: Path) -> None:
    body = dedent(
        f"""\
        #!{sys.executable}
        import json
        import os
        import struct
        import sys
        import time
        from pathlib import Path

        LOG = Path(os.environ["MEMENTO_FAKE_LOG"])
        MARKERS = Path(os.environ["MEMENTO_FAKE_MARKERS"])
        MARKERS.mkdir(parents=True, exist_ok=True)
        BACKEND = sys.argv[2] if len(sys.argv) > 2 else "cpu"
        DEVICE = sys.argv[3] if len(sys.argv) > 3 else None
        READ_CHUNK = max(1, int(os.environ.get("MEMENTO_FAKE_READ_CHUNK", "1048576")))
        WRITE_CHUNK = max(0, int(os.environ.get("MEMENTO_FAKE_WRITE_CHUNK", "0")))


        def log(event, **fields):
            with LOG.open("a", encoding="utf-8") as handle:
                payload = {{"event": event, "pid": os.getpid(), "backend": BACKEND, **fields}}
                handle.write(json.dumps(payload, separators=(",", ":")) + "\\n")
                handle.flush()


        def marker(name):
            return MARKERS / name


        def wait_marker(name):
            path = marker(name)
            while not path.exists():
                time.sleep(0.01)


        def read_exact(count):
            parts = []
            remaining = count
            while remaining:
                chunk = sys.stdin.buffer.read(min(remaining, READ_CHUNK))
                if not chunk:
                    raise EOFError("truncated request")
                parts.append(chunk)
                remaining -= len(chunk)
            return b"".join(parts)


        def parse_control(text):
            if text.startswith("CTRL|"):
                _, mode, token, payload = text.split("|", 3)
                return mode, token, payload
            return "", "", text


        def vector_for_text(text):
            data = text.encode("utf-8")
            total = sum(data)
            return float((total % 97) + 1), float(((total + len(data) * 7) % 89) + 1)


        def write_frame(header, payload):
            header_bytes = json.dumps(header, separators=(",", ":")).encode("utf-8")
            frame = struct.pack("<II", 4 + len(header_bytes) + len(payload), len(header_bytes))
            frame += header_bytes + payload
            if WRITE_CHUNK:
                for index in range(0, len(frame), WRITE_CHUNK):
                    sys.stdout.buffer.write(frame[index : index + WRITE_CHUNK])
                    sys.stdout.buffer.flush()
                    time.sleep(0.001)
            else:
                sys.stdout.buffer.write(frame)
                sys.stdout.buffer.flush()


        log("start", argv=sys.argv[1:], device=DEVICE)
        if os.environ.get("MEMENTO_FAKE_NONREADER") == "1":
            marker("nonreader.started").write_text("", encoding="utf-8")
            log("nonreader")
            while True:
                time.sleep(1.0)

        while True:
            prefix = sys.stdin.buffer.read(4)
            if not prefix:
                log("eof")
                break
            if len(prefix) < 4:
                log("bad_prefix", size=len(prefix))
                raise SystemExit(2)
            (size,) = struct.unpack("<I", prefix)
            body = read_exact(size)
            request = json.loads(body.decode("utf-8"))
            texts = request["texts"]
            if BACKEND == "cpu":
                time.sleep(float(os.environ.get("MEMENTO_FAKE_CPU_DELAY", "0")))
            mode, token, payload = parse_control(texts[0] if texts else "")
            if token:
                marker(f"{{token}}.started").write_text("", encoding="utf-8")
            log(
                "request",
                id=request.get("id"),
                method=request.get("method"),
                mode=mode,
                token=token,
                count=len(texts),
            )

            if os.environ.get("MEMENTO_FAKE_STDERR_FLOOD") == "1":
                for _ in range(64):
                    sys.stderr.buffer.write(b"x" * 4096)
                    sys.stderr.flush()

            if os.environ.get("MEMENTO_FAKE_AUTO_FAIL") == "1" and BACKEND == "auto":
                time.sleep(float(os.environ.get("MEMENTO_FAKE_AUTO_DELAY", "0")))
                payload_bytes = struct.pack("<2f", *vector_for_text(payload))
                write_frame(
                    {{
                        "id": request.get("id"),
                        "ok": True,
                        "method": request.get("method"),
                        "dimensions": 2,
                        "count": len(texts),
                        "payload_len": len(payload_bytes),
                        "backend": {{"selected": "bogus", "requested": "auto", "fallback_reason": "gpu unavailable"}},
                    }},
                    payload_bytes,
                )
                continue

            vectors = []
            for index, text in enumerate(texts):
                if index == 0 and mode:
                    vectors.extend(vector_for_text(payload))
                else:
                    vectors.extend(vector_for_text(text))
            backend_selected = BACKEND if BACKEND == "vulkan" else "cpu"
            backend_info = {{
                "selected": backend_selected,
                "requested": BACKEND,
                "fallback_reason": None,
            }}
            if os.environ.get("MEMENTO_FAKE_VULKAN_FORCE_CPU") == "1" and BACKEND == "vulkan":
                backend_info["selected"] = "cpu"
                backend_info["fallback_reason"] = "no vulkan support"
            elif mode == "backend-list" and BACKEND == "auto":
                backend_info["selected"] = ["cpu"]
            elif mode == "backend-dict" and BACKEND == "auto":
                backend_info["selected"] = {{"name": "cpu"}}

            response_id = request.get("id")
            if mode == "hold":
                wait_marker(f"{{token}}.release")
            elif mode == "hang":
                while True:
                    time.sleep(1.0)
            elif mode == "crash":
                os._exit(17)
            elif mode == "malformed":
                sys.stdout.buffer.write(struct.pack("<II", 5, 1) + b"{{")
                sys.stdout.buffer.flush()
                continue
            elif mode == "truncated":
                payload_bytes = struct.pack("<2f", *vector_for_text(payload))
                header_bytes = json.dumps(
                    {{
                        "id": request.get("id"),
                        "ok": True,
                        "method": request.get("method"),
                        "dimensions": 2,
                        "count": len(texts),
                        "payload_len": len(payload_bytes),
                        "backend": backend_info,
                    }},
                    separators=(",", ":"),
                ).encode("utf-8")
                frame = struct.pack("<II", 4 + len(header_bytes) + len(payload_bytes), len(header_bytes))
                frame += header_bytes + payload_bytes
                sys.stdout.buffer.write(frame[:6])
                sys.stdout.buffer.flush()
                raise SystemExit(0)
            elif mode == "wrong-id":
                response_id = f"{{request.get('id')}}-wrong"

            if mode == "oversized":
                sys.stdout.buffer.write(struct.pack("<I", 1_000_000))
                sys.stdout.buffer.flush()
                time.sleep(60.0)
                continue

            payload_bytes = struct.pack(f"<{{len(vectors)}}f", *vectors)
            if mode == "nonfinite":
                payload_bytes = struct.pack("<2f", float("nan"), 1.0)

            write_frame(
                {{
                    "id": response_id,
                    "ok": True,
                    "method": request.get("method"),
                    "dimensions": 2,
                    "count": len(texts),
                    "payload_len": len(payload_bytes),
                    "backend": backend_info,
                }},
                payload_bytes,
            )
        """
    )
    path.write_text(body, encoding="utf-8")
    path.chmod(path.stat().st_mode | stat.S_IXUSR)


def test_environment_change_recycles_worker(
    client_factory: ClientFactory, monkeypatch: pytest.MonkeyPatch
) -> None:
    client = client_factory(idle_seconds=5.0)
    client.embed("before")
    first = _status_int(client.worker_status, "pid")
    monkeypatch.setenv("MEMENTO_TEST_WARM_ENV", "changed")
    client.embed("after")
    assert _status_int(client.worker_status, "pid") != first
    assert not _pid_exists(first)


def test_failed_reap_is_retried_not_reused(
    client_factory: ClientFactory, monkeypatch: pytest.MonkeyPatch
) -> None:
    client = client_factory(idle_seconds=5.0)
    client.embed("before")
    pid = _status_int(client.worker_status, "pid")
    real_wait = subprocess.Popen.wait
    failures = 0

    def wait_once(process: subprocess.Popen[bytes], timeout: float | None = None) -> int:
        nonlocal failures
        if process.pid == pid and failures == 0:
            failures += 1
            raise subprocess.TimeoutExpired("fake", timeout or 0.0)
        return real_wait(process, timeout=timeout)

    monkeypatch.setattr(subprocess.Popen, "wait", wait_once)
    with pytest.raises(SemanticSearchError, match="reaped"):
        client.close()
    client.close()
    assert client.worker_status["pid"] is None
    assert not _pid_exists(pid)


def test_fallback_shares_original_deadline(
    client_factory: ClientFactory, worker_harness: WorkerHarness, monkeypatch: pytest.MonkeyPatch
) -> None:
    monkeypatch.setenv("MEMENTO_FAKE_AUTO_FAIL", "1")
    monkeypatch.setenv("MEMENTO_FAKE_AUTO_DELAY", "0.35")
    monkeypatch.setenv("MEMENTO_FAKE_CPU_DELAY", "0.35")
    client = client_factory(backend="auto", timeout_seconds=0.55, idle_seconds=5.0)
    started = time.monotonic()
    with pytest.raises(SemanticSearchError, match="deadline"):
        client.embed("test")
    assert time.monotonic() - started < 1.3
    assert [event["backend"] for event in _events(worker_harness, "start")] == ["auto", "cpu"]
    assert client.worker_status["pid"] is None


def test_queued_deadline_does_not_discard_active_worker(
    client_factory: ClientFactory, worker_harness: WorkerHarness, monkeypatch: pytest.MonkeyPatch
) -> None:
    client = client_factory(timeout_seconds=2.0, idle_seconds=5.0)
    token = "queue-deadline"
    with ThreadPoolExecutor(max_workers=1) as pool:
        active = pool.submit(client.embed, _control("hold", token, "active"))
        _wait_for(lambda: _marker(worker_harness, f"{token}.started").exists())
        # The active call already captured its own deadline.
        monkeypatch.setattr(client, "_timeout_seconds", 0.1)
        with pytest.raises(SemanticSearchError, match="deadline"):
            client.embed("queued")
        _marker(worker_harness, f"{token}.release").write_text("go")
        assert _wait_result(active) == _vector_for_text("active")
    assert client.worker_status["starts"] == 1


def _vector_for_text(text: str) -> tuple[float, float]:
    data = text.encode("utf-8")
    total = sum(data)
    return float((total % 97) + 1), float(((total + len(data) * 7) % 89) + 1)


def _control(mode: str, token: str, payload: str = "payload") -> str:
    return f"CTRL|{mode}|{token}|{payload}"


def _wait_for(predicate: Callable[[], bool], *, timeout: float = 2.0) -> None:
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        if predicate():
            return
        time.sleep(0.01)
    raise AssertionError("condition not met before timeout")


def _wait_result(future: Future[T], *, timeout: float = 2.0) -> T:
    return future.result(timeout=timeout)


def _marker(paths: WorkerHarness, name: str) -> Path:
    return paths.markers / name


def _events(paths: WorkerHarness, event: str | None = None) -> list[dict[str, object]]:
    if not paths.log.exists():
        return []
    entries = [
        json.loads(line)
        for line in paths.log.read_text(encoding="utf-8").splitlines()
        if line.strip()
    ]
    if event is None:
        return entries
    return [entry for entry in entries if entry["event"] == event]


def _status_int(status: dict[str, object], key: str) -> int:
    value = status[key]
    assert type(value) is int
    return value


def _pid_exists(pid: int) -> bool:
    try:
        os.kill(pid, 0)
    except ProcessLookupError:
        return False
    return True


def test_warm_client_reuses_same_process_and_updates_counters(
    worker_harness: WorkerHarness,
    client_factory: ClientFactory,
) -> None:
    client = client_factory()

    first = client.embed("alpha")
    status1 = client.worker_status
    second = client.embed("beta")
    status2 = client.worker_status

    assert first == _vector_for_text("alpha")
    assert second == _vector_for_text("beta")
    assert status1["pid"] == status2["pid"]
    assert status1["generation"] == 1
    assert status2["generation"] == 1
    assert status1["starts"] == 1
    assert status2["starts"] == 1
    assert status1["completed_requests"] == 1
    assert status2["completed_requests"] == 2
    assert status2["active"] is False
    assert status2["idle_deadline_monotonic"] is not None
    assert len(_events(worker_harness, "start")) == 1


def test_warm_client_expires_after_idle_and_restarts(
    worker_harness: WorkerHarness,
    client_factory: ClientFactory,
) -> None:
    client = client_factory(idle_seconds=0.3)

    client.embed("idle")
    status1 = client.worker_status
    pid1 = _status_int(status1, "pid")
    _wait_for(lambda: client.worker_status["pid"] is None, timeout=1.5)
    _wait_for(lambda: not _pid_exists(pid1), timeout=1.5)

    assert client.embed("restart") == _vector_for_text("restart")
    status2 = client.worker_status
    assert status2["generation"] == 2
    assert status2["starts"] == 2
    assert status2["completed_requests"] == 2
    assert status2["pid"] is not None


def test_warm_client_close_is_idempotent_and_blocks_future_use(
    client_factory: ClientFactory,
) -> None:
    client = client_factory()
    assert client.embed("before-close") == _vector_for_text("before-close")

    client.close()
    client.close()

    assert client.worker_status["closed"] is True
    assert client.worker_status["pid"] is None
    with pytest.raises(SemanticSearchError, match="closed"):
        client.embed("after-close")


def test_warm_client_serialises_concurrent_calls_and_returns_unique_vectors(
    worker_harness: WorkerHarness,
    client_factory: ClientFactory,
) -> None:
    client = client_factory(timeout_seconds=1.0)

    with ThreadPoolExecutor(max_workers=2) as executor:
        first_future = executor.submit(client.embed, _control("hold", "serial-first", "alpha"))
        _wait_for(lambda: _marker(worker_harness, "serial-first.started").exists())
        second_future = executor.submit(client.embed, "beta")
        _wait_for(lambda: len(_events(worker_harness, "request")) == 1)
        assert not second_future.done()
        _marker(worker_harness, "serial-first.release").write_text("", encoding="utf-8")
        first = _wait_result(first_future)
        second = _wait_result(second_future)

    requests = _events(worker_harness, "request")
    assert len(requests) == 2
    assert requests[0]["pid"] == requests[1]["pid"]
    assert first == _vector_for_text("alpha")
    assert second == _vector_for_text("beta")
    assert first != second


def test_warm_client_active_request_can_outlive_idle_window(
    worker_harness: WorkerHarness,
    client_factory: ClientFactory,
) -> None:
    client = client_factory(idle_seconds=0.3, timeout_seconds=1.0)

    with ThreadPoolExecutor(max_workers=1) as executor:
        future = executor.submit(client.embed, _control("hold", "active-idle", "alpha"))
        _wait_for(lambda: _marker(worker_harness, "active-idle.started").exists())
        pid = _status_int(client.worker_status, "pid")
        time.sleep(0.35)
        assert client.worker_status["pid"] == pid
        assert client.worker_status["active"] is True
        _marker(worker_harness, "active-idle.release").write_text("", encoding="utf-8")
        assert _wait_result(future) == _vector_for_text("alpha")

    assert client.embed("beta") == _vector_for_text("beta")
    status = client.worker_status
    assert status["pid"] == pid
    assert status["starts"] == 1
    assert status["completed_requests"] == 2


def test_warm_client_cancels_queued_call_without_killing_active_request(
    worker_harness: WorkerHarness,
    client_factory: ClientFactory,
) -> None:
    client = client_factory(timeout_seconds=1.0)
    queued_cancel = False

    def cancelled() -> bool:
        return queued_cancel

    with ThreadPoolExecutor(max_workers=2) as executor:
        active = executor.submit(client.embed, _control("hold", "queued-cancel", "alpha"))
        _wait_for(lambda: _marker(worker_harness, "queued-cancel.started").exists())
        pid = _status_int(client.worker_status, "pid")
        queued = executor.submit(lambda: client.embed("beta", cancelled=cancelled))
        _wait_for(lambda: len(_events(worker_harness, "request")) == 1)
        queued_cancel = True
        with pytest.raises(SemanticSearchError, match="cancelled"):
            _wait_result(queued)
        assert client.worker_status["pid"] == pid
        _marker(worker_harness, "queued-cancel.release").write_text("", encoding="utf-8")
        assert _wait_result(active) == _vector_for_text("alpha")

    assert client.embed("gamma") == _vector_for_text("gamma")
    requests = _events(worker_harness, "request")
    assert [entry["backend"] for entry in requests] == ["cpu", "cpu"]
    assert client.worker_status["starts"] == 1


def test_warm_client_cancels_active_call_and_recovers(
    worker_harness: WorkerHarness,
    client_factory: ClientFactory,
) -> None:
    client = client_factory(timeout_seconds=1.0)
    cancelled = False

    def is_cancelled() -> bool:
        return cancelled

    with ThreadPoolExecutor(max_workers=1) as executor:
        future = executor.submit(
            lambda: client.embed(_control("hold", "active-cancel", "alpha"), cancelled=is_cancelled)
        )
        _wait_for(lambda: _marker(worker_harness, "active-cancel.started").exists())
        pid = _status_int(client.worker_status, "pid")
        cancelled = True
        with pytest.raises(SemanticSearchError, match="cancelled"):
            _wait_result(future)

    assert client.worker_status["pid"] is None
    _wait_for(lambda: not _pid_exists(pid), timeout=1.5)
    assert client.embed("beta") == _vector_for_text("beta")
    assert client.worker_status["starts"] == 2


def test_warm_client_close_during_active_request_aborts_and_reaps_worker(
    worker_harness: WorkerHarness,
    client_factory: ClientFactory,
) -> None:
    client = client_factory(timeout_seconds=1.0)

    with ThreadPoolExecutor(max_workers=1) as executor:
        future = executor.submit(client.embed, _control("hold", "close-active", "alpha"))
        _wait_for(lambda: _marker(worker_harness, "close-active.started").exists())
        pid = _status_int(client.worker_status, "pid")
        client.close()
        with pytest.raises(SemanticSearchError, match="closed"):
            _wait_result(future)

    assert client.worker_status["closed"] is True
    assert client.worker_status["pid"] is None
    _wait_for(lambda: not _pid_exists(pid), timeout=1.5)


@pytest.mark.parametrize("mode", ["backend-list", "backend-dict"])
def test_warm_client_auto_backend_falls_back_when_selected_type_is_malformed(
    worker_harness: WorkerHarness,
    client_factory: ClientFactory,
    mode: str,
) -> None:
    client = client_factory(backend="auto", timeout_seconds=1.0)

    assert client.embed(_control(mode, f"bad-selected-{mode}", "alpha")) == _vector_for_text(
        "alpha"
    )

    starts = _events(worker_harness, "start")
    requests = _events(worker_harness, "request")
    assert [entry["backend"] for entry in starts] == ["auto", "cpu"]
    assert [entry["backend"] for entry in requests] == ["auto", "cpu"]
    assert client.last_backend == {
        "selected": "cpu",
        "requested": "auto",
        "fallback_reason": "embedding worker reported an invalid backend",
    }


def test_warm_client_missing_worker_executable_fails_cleanly(tmp_path: Path) -> None:
    model = tmp_path / "model.gte"
    model.write_bytes(b"model")
    client = SubprocessEmbeddingClient(
        tmp_path / "missing-worker.py",
        model,
        dimensions=2,
        max_batch=8,
        max_input_chars=1024,
        timeout_seconds=0.5,
        idle_seconds=0.4,
    )

    try:
        with pytest.raises(SemanticSearchError, match="pipe failed"):
            client.embed("alpha")
    finally:
        client.close()

    assert client.worker_status["pid"] is None


@pytest.mark.parametrize(
    ("mode", "match"),
    [
        ("crash", "truncated frame"),
        ("malformed", "invalid JSON"),
        ("truncated", "truncated frame"),
        ("wrong-id", "ID or method mismatch"),
        ("oversized", "exceeds frame limit"),
        ("nonfinite", "nonfinite"),
    ],
)
def test_warm_client_bad_responses_drop_process_and_recover(
    worker_harness: WorkerHarness,
    client_factory: ClientFactory,
    mode: str,
    match: str,
) -> None:
    client = client_factory(timeout_seconds=1.0)
    assert client.embed("warmup") == _vector_for_text("warmup")
    pid = _status_int(client.worker_status, "pid")

    with pytest.raises(SemanticSearchError, match=match):
        client.embed(_control(mode, f"bad-{mode}", "broken"))

    assert client.worker_status["pid"] is None
    _wait_for(lambda: not _pid_exists(pid), timeout=1.5)
    assert client.embed("recover") == _vector_for_text("recover")
    assert client.worker_status["starts"] == 2


def test_warm_client_drains_stderr_without_deadlock(
    worker_harness: WorkerHarness,
    client_factory: ClientFactory,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    monkeypatch.setenv("MEMENTO_FAKE_STDERR_FLOOD", "1")
    client = client_factory(timeout_seconds=1.0)

    assert client.embed("stderr") == _vector_for_text("stderr")
    assert len(_events(worker_harness, "request")) == 1
    assert client.worker_status["completed_requests"] == 1


def test_warm_client_handles_partial_pipe_reads_and_writes(
    client_factory: ClientFactory,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    monkeypatch.setenv("MEMENTO_FAKE_READ_CHUNK", "7")
    monkeypatch.setenv("MEMENTO_FAKE_WRITE_CHUNK", "3")
    text = "x" * 200_000
    client = client_factory(max_input_chars=len(text) + 1, timeout_seconds=1.0)

    assert client.embed(text) == _vector_for_text(text)


@pytest.mark.parametrize("scenario", ["hang", "nonreader"])
def test_warm_client_times_out_on_hung_worker_and_recovers(
    worker_harness: WorkerHarness,
    client_factory: ClientFactory,
    monkeypatch: pytest.MonkeyPatch,
    scenario: str,
) -> None:
    client = client_factory(timeout_seconds=0.4, max_input_chars=2_000_000)
    text = _control("hang", "hung-reader", "alpha")
    if scenario == "nonreader":
        monkeypatch.setenv("MEMENTO_FAKE_NONREADER", "1")
        text = "y" * 1_000_000

    with pytest.raises(SemanticSearchError, match="deadline exceeded"):
        client.embed(text)

    assert client.worker_status["pid"] is None
    monkeypatch.delenv("MEMENTO_FAKE_NONREADER", raising=False)
    assert client.embed("recover") == _vector_for_text("recover")
    assert client.worker_status["starts"] == 2
    if scenario == "nonreader":
        assert _marker(worker_harness, "nonreader.started").exists()


def test_warm_client_auto_backend_falls_back_to_cpu_and_remembers_failure(
    worker_harness: WorkerHarness,
    client_factory: ClientFactory,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    monkeypatch.setenv("MEMENTO_FAKE_AUTO_FAIL", "1")
    client = client_factory(backend="auto", timeout_seconds=1.0)

    assert client.embed("alpha") == _vector_for_text("alpha")
    assert client.embed("beta") == _vector_for_text("beta")

    starts = _events(worker_harness, "start")
    requests = _events(worker_harness, "request")
    assert [entry["backend"] for entry in starts] == ["auto", "cpu"]
    assert [entry["backend"] for entry in requests] == ["auto", "cpu", "cpu"]
    assert client.worker_status["starts"] == 2
    assert client.last_backend == {
        "selected": "cpu",
        "requested": "auto",
        "fallback_reason": "embedding worker reported an invalid backend",
    }


def test_warm_client_explicit_vulkan_failure_does_not_fall_back(
    worker_harness: WorkerHarness,
    client_factory: ClientFactory,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    monkeypatch.setenv("MEMENTO_FAKE_VULKAN_FORCE_CPU", "1")
    client = client_factory(backend="vulkan", timeout_seconds=1.0)

    with pytest.raises(SemanticSearchError, match="did not use Vulkan"):
        client.embed("alpha")

    starts = _events(worker_harness, "start")
    assert [entry["backend"] for entry in starts] == ["vulkan"]
    assert client.last_backend is None


def test_warm_client_restarts_after_request_count_cap(
    client_factory: ClientFactory,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    monkeypatch.setattr(framed_worker, "_MAX_REQUESTS", 2)
    client = client_factory(timeout_seconds=1.0)

    assert client.embed("one") == _vector_for_text("one")
    pid1 = _status_int(client.worker_status, "pid")
    assert client.embed("two") == _vector_for_text("two")
    assert client.embed("three") == _vector_for_text("three")
    status = client.worker_status

    assert status["starts"] == 2
    assert status["generation"] == 2
    assert status["completed_requests"] == 3
    assert status["pid"] is not None
    _wait_for(lambda: not _pid_exists(pid1), timeout=1.5)


def test_default_configuration_remains_cold(
    worker_harness: WorkerHarness,
    client_factory: ClientFactory,
) -> None:
    client = client_factory(idle_seconds=0.0)

    assert client.worker_status == {"reuse": False}
    assert client.embed("alpha") == _vector_for_text("alpha")
    assert client.embed("beta") == _vector_for_text("beta")
    starts = _events(worker_harness, "start")
    assert len(starts) == 2
