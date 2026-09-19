from __future__ import annotations

import hashlib
import json
import math
import os
import struct
import subprocess
import time
import uuid
from collections.abc import Callable, Sequence
from contextlib import nullcontext
from pathlib import Path
from typing import Any, Protocol

from memento.framed_worker import FramedWorker
from memento.semantic import EmbeddingClient, EmbeddingModelInfo, SemanticSearchError


class ProcessRunner(Protocol):
    def __call__(self, command: list[str], **kwargs: Any) -> subprocess.CompletedProcess[bytes]: ...


def _run_process(command: list[str], **kwargs: Any) -> subprocess.CompletedProcess[bytes]:
    return subprocess.run(command, **kwargs)


class SubprocessEmbeddingClient(EmbeddingClient):
    """Use cold workers by default, or opt into serial, idle-capped process reuse."""

    def __init__(
        self,
        worker_path: Path | str,
        model_path: Path | str,
        *,
        dimensions: int,
        max_batch: int,
        max_input_chars: int,
        timeout_seconds: float = 300.0,
        nice: int = 0,
        threads: int = 1,
        backend: str = "cpu",
        vulkan_device: str | None = None,
        idle_seconds: float = 0.0,
        process_runner: ProcessRunner | None = None,
    ) -> None:
        if not math.isfinite(idle_seconds) or not 0 <= idle_seconds <= 3600:
            raise ValueError("idle_seconds must be finite and between 0 and 3600")
        if idle_seconds and process_runner is not None:
            raise ValueError("process_runner is only supported for cold workers")
        self._warm = FramedWorker(idle_seconds) if idle_seconds else None
        self._worker_path = Path(worker_path)
        self._model_path = Path(model_path)
        self._dimensions = dimensions
        self._max_batch = max_batch
        self._max_input_chars = max_input_chars
        self._timeout_seconds = timeout_seconds
        self._nice = nice
        self._threads = threads
        self._process_runner: ProcessRunner = process_runner or _run_process
        if backend not in {"cpu", "vulkan", "auto"}:
            raise ValueError("backend must be cpu, vulkan or auto")
        self._backend = backend
        self._vulkan_device = vulkan_device
        self.last_backend: dict[str, object] | None = None
        self._gpu_disabled_reason: str | None = None
        self._revision = _sha256_file(self._model_path)
        if backend != "cpu":
            # Isolate experimental CPU/GPU vectors from the established CPU index.
            self._revision += ":gte1-fp32-vulkan-v1"

    @property
    def worker_status(self) -> dict[str, object]:
        return self._warm.status if self._warm else {"reuse": False}

    def close(self) -> None:
        if self._warm is not None:
            self._warm.close()

    def model_info(self) -> EmbeddingModelInfo:
        return EmbeddingModelInfo(
            model_id=self._model_path.name,
            dimensions=self._dimensions,
            revision=self._revision,
            max_batch=self._max_batch,
            max_input_chars=self._max_input_chars,
        )

    def embed(self, text: str, *, cancelled: Callable[[], bool] | None = None) -> tuple[float, ...]:
        return self.embed_batch((text,), cancelled=cancelled)[0]

    def embed_batch(
        self, texts: Sequence[str], *, cancelled: Callable[[], bool] | None = None
    ) -> tuple[tuple[float, ...], ...]:
        if not texts:
            return ()
        if len(texts) > self._max_batch:
            raise SemanticSearchError(
                f"embedding batch has {len(texts)} items; maximum is {self._max_batch}"
            )
        if any(len(text) > self._max_input_chars for text in texts):
            raise SemanticSearchError("embedding input exceeds configured character limit")
        if cancelled is not None and cancelled():
            raise SemanticSearchError("embedding cancelled")
        deadline = time.monotonic() + self._timeout_seconds
        request_id = uuid.uuid4().hex if self._warm else "batch"
        request = {"method": "embed_batch", "id": request_id, "texts": list(texts)}
        payload = json.dumps(request, separators=(",", ":")).encode("utf-8")
        if len(payload) > 4 * 1024 * 1024:
            raise SemanticSearchError("embedding request exceeds frame limit")
        wire = struct.pack("<I", len(payload)) + payload
        session = self._warm.session(deadline, cancelled) if self._warm else nullcontext()
        with session:
            header, raw = self._embed_wire(wire, request_id, len(texts), deadline, cancelled)
            backend_info = header.get("backend")
            if isinstance(backend_info, dict):
                self.last_backend = backend_info
                if self._backend == "auto" and backend_info.get("fallback_reason"):
                    self._gpu_disabled_reason = str(backend_info["fallback_reason"])[:300]
                if self._gpu_disabled_reason:
                    self.last_backend = {
                        **backend_info,
                        "requested": self._backend,
                        "fallback_reason": self._gpu_disabled_reason,
                    }
            values = struct.unpack(f"<{len(texts) * self._dimensions}f", raw)
            return tuple(
                tuple(values[i * self._dimensions : (i + 1) * self._dimensions])
                for i in range(len(texts))
            )

    def _embed_wire(
        self,
        wire: bytes,
        request_id: str,
        count: int,
        deadline: float,
        cancelled: Callable[[], bool] | None,
    ) -> tuple[dict[str, Any], bytes]:
        selected = "cpu" if self._gpu_disabled_reason else self._backend
        try:
            return self._call_worker(wire, selected, deadline, count, request_id, cancelled)
        except SemanticSearchError as exc:
            if self._warm is not None:
                self._warm.discard()
            if self._backend != "auto" or selected == "cpu":
                raise
            self._gpu_disabled_reason = str(exc)[:300]
            if time.monotonic() >= deadline or (cancelled is not None and cancelled()):
                raise
            return self._call_worker(wire, "cpu", deadline, count, request_id, cancelled)

    def _call_worker(
        self,
        wire: bytes,
        backend: str,
        deadline: float,
        expected_count: int,
        request_id: str,
        cancelled: Callable[[], bool] | None,
    ) -> tuple[dict[str, Any], bytes]:
        try:
            command = [str(self._worker_path), str(self._model_path)]
            if backend != "cpu":
                command.append(backend)
                if self._vulkan_device is not None:
                    command.append(self._vulkan_device)
            if self._nice > 0:
                command = ["nice", "-n", str(self._nice), *command]
            environment = os.environ.copy()
            for name in (
                "RAYON_NUM_THREADS",
                "OMP_NUM_THREADS",
                "OPENBLAS_NUM_THREADS",
                "MKL_NUM_THREADS",
            ):
                environment[name] = str(self._threads)
            if self._warm is not None:
                output = self._warm.exchange(
                    command,
                    environment,
                    wire,
                    deadline=deadline,
                    cancelled=cancelled,
                    max_response=65536 + expected_count * self._dimensions * 4,
                )
            else:
                completed = self._process_runner(
                    command,
                    input=wire,
                    capture_output=True,
                    timeout=max(0.0, deadline - time.monotonic()),
                    check=False,
                    env=environment,
                )
                if completed.returncode != 0:
                    stderr = completed.stderr.decode("utf-8", errors="replace").strip()
                    raise SemanticSearchError(
                        f"embedding worker exited {completed.returncode}: {stderr}"
                    )
                output = completed.stdout
        except (OSError, subprocess.TimeoutExpired) as exc:
            raise SemanticSearchError(f"embedding worker failed: {exc}") from exc
        header, raw = _decode_response(output)
        if header.get("ok") is not True:
            # Warm errors may contain input text; never retain it in diagnostics.
            message = "embedding worker error" if self._warm else str(header.get("error"))
            raise SemanticSearchError(message)
        if self._warm and (header.get("id") != request_id or header.get("method") != "embed_batch"):
            raise SemanticSearchError("embedding worker response ID or method mismatch")
        backend_info = header.get("backend")
        if self._backend != "cpu" and not isinstance(backend_info, dict):
            raise SemanticSearchError(
                "Vulkan worker omitted backend diagnostics; use the matching binary"
            )
        if isinstance(backend_info, dict):
            selected = backend_info.get("selected")
            if self._backend == "vulkan" and selected != "vulkan":
                raise SemanticSearchError("explicit Vulkan request did not use Vulkan")
            if backend == "cpu" and selected != "cpu":
                raise SemanticSearchError("CPU request did not use CPU")
            if backend == "auto" and selected not in ("cpu", "vulkan"):
                raise SemanticSearchError("embedding worker reported an invalid backend")
        dimensions, count = header.get("dimensions"), header.get("count")
        if (
            type(dimensions) is not int
            or type(count) is not int
            or dimensions != self._dimensions
            or count != expected_count
        ):
            raise SemanticSearchError("embedding worker shape mismatch")
        if len(raw) != expected_count * self._dimensions * 4:
            raise SemanticSearchError("embedding payload length mismatch")
        if not all(math.isfinite(v) for v in struct.unpack(f"<{count * dimensions}f", raw)):
            raise SemanticSearchError("nonfinite embedding worker output")
        return header, raw


def _decode_response(wire: bytes) -> tuple[dict[str, Any], bytes]:
    if len(wire) < 8:
        raise SemanticSearchError("embedding worker returned a truncated frame")
    total_len, header_len = struct.unpack_from("<II", wire)
    if total_len + 4 != len(wire) or header_len > total_len - 4:
        raise SemanticSearchError("embedding worker returned an invalid frame")
    header_start = 8
    header_end = header_start + header_len
    try:
        header = json.loads(wire[header_start:header_end])
    except (json.JSONDecodeError, UnicodeDecodeError) as exc:
        raise SemanticSearchError("embedding worker returned invalid JSON") from exc
    if not isinstance(header, dict):
        raise SemanticSearchError("embedding worker header must be an object")
    raw = wire[header_end:]
    if type(header.get("payload_len")) is not int or header["payload_len"] != len(raw):
        raise SemanticSearchError("embedding worker payload length mismatch")
    return header, raw


def _sha256_file(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for block in iter(lambda: handle.read(1 << 20), b""):
            digest.update(block)
    return digest.hexdigest()
