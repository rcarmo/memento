from __future__ import annotations

from collections.abc import Callable
from dataclasses import dataclass, field


class CheckpointError(RuntimeError):
    """Raised when an injected checkpoint aborts an operation."""


CheckpointCallback = Callable[[str], None]


@dataclass(slots=True)
class CheckpointHook:
    """Deterministic test hook for transaction checkpoints."""

    callback: CheckpointCallback | None = None
    seen: list[str] = field(default_factory=list)

    def hit(self, name: str) -> None:
        self.seen.append(name)
        if self.callback is not None:
            self.callback(name)


@dataclass(frozen=True, slots=True)
class FailAtCheckpoint:
    name: str

    def __call__(self, checkpoint: str) -> None:
        if checkpoint == self.name:
            raise CheckpointError(f"checkpoint triggered: {checkpoint}")
