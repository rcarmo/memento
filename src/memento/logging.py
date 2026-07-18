from __future__ import annotations

import json
import sys
from dataclasses import dataclass, field
from datetime import UTC, datetime
from typing import Any, TextIO

_DEFAULT_REDACT_KEYS = frozenset(
    {
        "authorization",
        "token",
        "bearer_token",
        "password",
        "secret",
        "secrets",
        "api_key",
    }
)


@dataclass(slots=True)
class JsonLogger:
    service: str = "memento"
    stream: TextIO = field(default_factory=lambda: sys.stderr)
    redact_keys: frozenset[str] = field(default_factory=lambda: _DEFAULT_REDACT_KEYS)

    def log(self, level: str, event: str, **fields: Any) -> None:
        payload = {
            "ts": datetime.now(tz=UTC).replace(microsecond=0).isoformat(),
            "service": self.service,
            "level": level.lower(),
            "event": event,
            **_redact_mapping(fields, self.redact_keys),
        }
        self.stream.write(json.dumps(payload, sort_keys=True) + "\n")
        self.stream.flush()

    def debug(self, event: str, **fields: Any) -> None:
        self.log("debug", event, **fields)

    def info(self, event: str, **fields: Any) -> None:
        self.log("info", event, **fields)

    def warning(self, event: str, **fields: Any) -> None:
        self.log("warning", event, **fields)

    def error(self, event: str, **fields: Any) -> None:
        self.log("error", event, **fields)


def _redact_mapping(value: dict[str, Any], redact_keys: frozenset[str]) -> dict[str, Any]:
    return {key: _redact_value(key, item, redact_keys) for key, item in value.items()}


def _redact_value(key: str, value: Any, redact_keys: frozenset[str]) -> Any:
    if key.lower() in redact_keys:
        return "<redacted>"
    if isinstance(value, dict):
        return _redact_mapping(value, redact_keys)
    if isinstance(value, list):
        return [_redact_value(key, item, redact_keys) for item in value]
    if isinstance(value, tuple):
        return tuple(_redact_value(key, item, redact_keys) for item in value)
    return value
