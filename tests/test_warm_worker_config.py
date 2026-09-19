from __future__ import annotations

import json
import math
import sqlite3
from pathlib import Path

import pytest
from pydantic import ValidationError

from memento import app
from memento.config import SemanticSearchConfig


@pytest.mark.parametrize("worker_idle_seconds", [0.0, 3600.0])
def test_worker_idle_seconds_accepts_bounds(worker_idle_seconds: float) -> None:
    config = SemanticSearchConfig(worker_idle_seconds=worker_idle_seconds)
    assert config.worker_idle_seconds == worker_idle_seconds


def test_worker_idle_seconds_defaults_to_zero_without_changing_cpu_backend() -> None:
    config = SemanticSearchConfig()
    assert config.worker_idle_seconds == 0.0
    assert config.backend == "cpu"


@pytest.mark.parametrize(
    "worker_idle_seconds",
    [-0.01, 3600.01, math.nan, math.inf, -math.inf],
)
def test_worker_idle_seconds_rejects_invalid_values(worker_idle_seconds: float) -> None:
    with pytest.raises(ValidationError):
        SemanticSearchConfig(worker_idle_seconds=worker_idle_seconds)


def test_worker_idle_seconds_requires_subprocess_mode_when_positive() -> None:
    assert SemanticSearchConfig(worker_mode="in_process", worker_idle_seconds=0.0).worker_mode == (
        "in_process"
    )
    with pytest.raises(ValidationError, match="subprocess"):
        SemanticSearchConfig(worker_mode="in_process", worker_idle_seconds=1.0)


class _SentinelError(RuntimeError):
    pass


class _FakeLease:
    def __init__(self) -> None:
        self.released = False

    def release(self) -> None:
        self.released = True


def _write_config(tmp_path: Path, *, worker_idle_seconds: float) -> Path:
    config_path = tmp_path / "config.json"
    runtime_root = tmp_path / "runtime"
    config_path.write_text(
        json.dumps(
            {
                "schema_version": 2,
                "repository": {"root_path": str(runtime_root)},
                "authorization": {"principals": {}},
                "intelligent_tiers": {
                    "semantic_search": {
                        "enabled": True,
                        "worker_mode": "subprocess",
                        "worker_path": "/worker",
                        "worker_idle_seconds": worker_idle_seconds,
                        "model_path": "/model.gte",
                    }
                },
            }
        ),
        encoding="utf-8",
    )
    return config_path


def test_build_runtime_passes_worker_idle_seconds_to_subprocess_client(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    seen: dict[str, object] = {}
    lease = _FakeLease()

    def fake_client(*args: object, **kwargs: object) -> object:
        seen["args"] = args
        seen["kwargs"] = kwargs
        raise _SentinelError("stop after wiring")

    monkeypatch.setattr(app, "SubprocessEmbeddingClient", fake_client)
    monkeypatch.setattr(app, "acquire_writer_lease", lambda *args, **kwargs: lease)
    monkeypatch.setattr(app, "bootstrap_repository", lambda *args, **kwargs: None)
    monkeypatch.setattr(
        app, "connect_control_db", lambda *args, **kwargs: sqlite3.connect(":memory:")
    )
    monkeypatch.setattr(app, "migrate_control_db", lambda *args, **kwargs: None)

    config_path = _write_config(tmp_path, worker_idle_seconds=12.5)
    with pytest.raises(_SentinelError, match="wiring"):
        app.build_runtime(config_path)

    assert seen["args"] == ("/worker", "/model.gte")
    kwargs = seen.get("kwargs")
    assert isinstance(kwargs, dict)
    assert kwargs["idle_seconds"] == 12.5
    assert lease.released is True
