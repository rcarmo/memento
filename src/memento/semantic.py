from __future__ import annotations

import hashlib
import math
import struct
from collections.abc import Callable, Iterable, Sequence
from dataclasses import dataclass
from typing import Protocol


class SemanticSearchError(RuntimeError):
    """Raised when semantic search cannot be completed safely."""


class SemanticConfigError(SemanticSearchError):
    """Raised when semantic search configuration is invalid."""


@dataclass(frozen=True, slots=True)
class EmbeddingModelInfo:
    model_id: str
    dimensions: int
    revision: str
    max_batch: int
    max_input_chars: int


class EmbeddingClient(Protocol):
    def model_info(self) -> EmbeddingModelInfo: ...

    def embed(
        self, text: str, *, cancelled: Callable[[], bool] | None = None
    ) -> tuple[float, ...]: ...

    def embed_batch(
        self, texts: Sequence[str], *, cancelled: Callable[[], bool] | None = None
    ) -> tuple[tuple[float, ...], ...]: ...


@dataclass(frozen=True, slots=True)
class ValidatedEmbedding:
    values: tuple[float, ...]
    norm: float


@dataclass(frozen=True, slots=True)
class SemanticWarning:
    code: str
    message: str


def embedding_text(*, title: str, description: str | None, body: str) -> str:
    parts = [title.strip()]
    if description:
        parts.append(description.strip())
    body_text = body.strip()
    if body_text:
        parts.append(body_text)
    return "\n\n".join(part for part in parts if part)


def embedding_content_hash(text: str) -> str:
    return hashlib.sha256(text.encode("utf-8")).hexdigest()


def validate_embedding(values: Iterable[float], *, dimensions: int) -> ValidatedEmbedding:
    materialized = tuple(float(value) for value in values)
    if len(materialized) != dimensions:
        raise SemanticSearchError(
            f"embedding dimension mismatch: got {len(materialized)}, expected {dimensions}"
        )
    norm_sq = 0.0
    for index, value in enumerate(materialized):
        if not math.isfinite(value):
            raise SemanticSearchError(f"embedding contains non-finite value at index {index}")
        norm_sq += value * value
    norm = math.sqrt(norm_sq)
    if not math.isfinite(norm) or norm <= 0.0:
        raise SemanticSearchError("embedding has zero or invalid norm")
    return ValidatedEmbedding(values=materialized, norm=norm)


def pack_f32le(values: Sequence[float]) -> bytes:
    if not values:
        return b""
    return struct.pack(f"<{len(values)}f", *values)


def unpack_f32le(blob: bytes) -> tuple[float, ...]:
    if not blob:
        return ()
    if len(blob) % 4 != 0:
        raise SemanticSearchError(f"invalid embedding blob length: {len(blob)}")
    return tuple(item[0] for item in struct.iter_unpack("<f", blob))


def cosine_similarity(left: Sequence[float], right: Sequence[float]) -> float:
    if len(left) != len(right):
        raise SemanticSearchError(f"vector dimension mismatch: left={len(left)} right={len(right)}")
    dot = 0.0
    left_norm_sq = 0.0
    right_norm_sq = 0.0
    for left_value, right_value in zip(left, right, strict=True):
        if not math.isfinite(left_value) or not math.isfinite(right_value):
            raise SemanticSearchError("vector contains non-finite value")
        dot += left_value * right_value
        left_norm_sq += left_value * left_value
        right_norm_sq += right_value * right_value
    if left_norm_sq <= 0.0 or right_norm_sq <= 0.0:
        raise SemanticSearchError("vector has zero norm")
    return dot / math.sqrt(left_norm_sq * right_norm_sq)
