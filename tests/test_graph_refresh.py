from __future__ import annotations

from dataclasses import dataclass, field
from pathlib import Path

import pytest

from memento.config import GraphExplorerConfig
from memento.graph_debug.refresh import GraphEmbeddingRefreshCoordinator
from memento.graph_debug.snapshot import GraphSnapshotError


@dataclass
class _Revisions:
    repository: str = "rev"


@dataclass
class _Metrics:
    memory_count: int = 3


@dataclass
class _Overview:
    revisions: _Revisions = field(default_factory=_Revisions)
    metrics: _Metrics = field(default_factory=_Metrics)


class _Snapshot:
    def overview(self) -> _Overview:
        return _Overview()

    def paths_for_ids(self, ids: tuple[str, ...]) -> tuple[str, ...]:
        known = {"a": "/projects/a.md", "b": "/projects/b.md"}
        return tuple(known[item] for item in ids if item in known)


@dataclass
class _WorkerState:
    running: bool = False
    pending: bool = True
    last_error: str | None = None


class _Worker:
    def __init__(self) -> None:
        self.calls: list[tuple[Path, str, tuple[str, ...] | None]] = []

    def enqueue(self, root: Path, revision: str, *, paths: tuple[str, ...] | None = None) -> bool:
        self.calls.append((root, revision, paths))
        return True

    def state(self) -> _WorkerState:
        return _WorkerState()


def coordinator(tmp_path: Path, worker: _Worker | None = None) -> GraphEmbeddingRefreshCoordinator:
    return GraphEmbeddingRefreshCoordinator(
        GraphExplorerConfig(enabled=True, refresh_max_paths=2),
        repository_root=tmp_path,
        snapshot_service=_Snapshot(),  # type: ignore[arg-type]
        worker=worker,  # type: ignore[arg-type]
    )


def test_selected_visible_and_confirmed_full_refresh(tmp_path: Path) -> None:
    worker = _Worker()
    refresh = coordinator(tmp_path, worker)
    selected = refresh.enqueue(scope="selected", concept_ids=("a",))
    assert selected.available and selected.queued_paths == 1
    assert worker.calls[-1] == (tmp_path, "rev", ("/projects/a.md",))
    refresh.enqueue(scope="visible", concept_ids=("b", "a", "a"))
    visible_paths = worker.calls[-1][2]
    assert visible_paths is not None
    assert tuple(visible_paths) == ("/projects/a.md", "/projects/b.md")
    with pytest.raises(GraphSnapshotError, match="confirmation"):
        refresh.enqueue(scope="full")
    refresh.enqueue(scope="full", confirm_full=True)
    full_root, full_revision, full_paths = worker.calls[-1]
    assert (full_root, full_revision) == (tmp_path, "rev")
    assert full_paths is None


def test_refresh_rejects_unknown_bounds_and_unavailable_worker(tmp_path: Path) -> None:
    worker = _Worker()
    refresh = coordinator(tmp_path, worker)
    with pytest.raises(GraphSnapshotError, match="unknown"):
        refresh.enqueue(scope="selected", concept_ids=("missing",))
    with pytest.raises(GraphSnapshotError, match="path limit"):
        refresh.enqueue(scope="visible", concept_ids=("a", "b", "c"))
    with pytest.raises(GraphSnapshotError, match="unsupported"):
        refresh.enqueue(scope="other")
    unavailable = coordinator(tmp_path)
    assert unavailable.state().available is False
    with pytest.raises(GraphSnapshotError, match="unavailable"):
        unavailable.enqueue(scope="selected", concept_ids=("a",))
