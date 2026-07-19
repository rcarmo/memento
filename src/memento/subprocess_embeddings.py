from __future__ import annotations

import hashlib
import json
import struct
import subprocess
from collections.abc import Callable, Sequence
from pathlib import Path
from typing import Any

from memento.semantic import EmbeddingClient, EmbeddingModelInfo, SemanticSearchError


class SubprocessEmbeddingClient(EmbeddingClient):
    """Run one framed memento-embed worker per request so model RAM is reclaimed."""

    def __init__(
        self,
        worker_path: Path | str,
        model_path: Path | str,
        *,
        dimensions: int,
        max_batch: int,
        max_input_chars: int,
        timeout_seconds: float = 300.0,
    ) -> None:
        self._worker_path = Path(worker_path)
        self._model_path = Path(model_path)
        self._dimensions = dimensions
        self._max_batch = max_batch
        self._max_input_chars = max_input_chars
        self._timeout_seconds = timeout_seconds
        self._revision = _sha256_file(self._model_path)

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
        request = {
            "method": "embed_batch",
            "id": "batch",
            "texts": list(texts),
        }
        payload = json.dumps(request, separators=(",", ":")).encode("utf-8")
        wire = struct.pack("<I", len(payload)) + payload
        try:
            completed = subprocess.run(
                [str(self._worker_path), str(self._model_path)],
                input=wire,
                capture_output=True,
                timeout=self._timeout_seconds,
                check=False,
            )
        except (OSError, subprocess.TimeoutExpired) as exc:
            raise SemanticSearchError(f"embedding worker failed: {exc}") from exc
        if completed.returncode != 0:
            stderr = completed.stderr.decode("utf-8", errors="replace").strip()
            raise SemanticSearchError(f"embedding worker exited {completed.returncode}: {stderr}")
        header, raw = _decode_response(completed.stdout)
        if not header.get("ok"):
            raise SemanticSearchError(str(header.get("error") or "embedding worker error"))
        dimensions = int(header.get("dimensions") or 0)
        count = int(header.get("count") or 0)
        if dimensions != self._dimensions or count != len(texts):
            raise SemanticSearchError(
                f"embedding worker shape mismatch: {count}x{dimensions}, "
                f"expected {len(texts)}x{self._dimensions}"
            )
        expected_bytes = count * dimensions * 4
        if len(raw) != expected_bytes:
            raise SemanticSearchError(
                f"embedding payload has {len(raw)} bytes; expected {expected_bytes}"
            )
        values = struct.unpack(f"<{count * dimensions}f", raw)
        return tuple(
            tuple(values[index * dimensions : (index + 1) * dimensions]) for index in range(count)
        )


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
    if int(header.get("payload_len") or 0) != len(raw):
        raise SemanticSearchError("embedding worker payload length mismatch")
    return header, raw


def _sha256_file(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for block in iter(lambda: handle.read(1 << 20), b""):
            digest.update(block)
    return digest.hexdigest()
