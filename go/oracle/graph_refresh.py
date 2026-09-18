"""Synthetic GraphEmbeddingRefreshCoordinator fixtures."""

from __future__ import annotations

from dataclasses import asdict
from pathlib import Path
from typing import Any

from memento.config import GraphExplorerConfig  # type: ignore[import-untyped]
from memento.graph_debug.models import (  # type: ignore[import-untyped]
    GraphMetrics,
    GraphOverview,
    GraphRevisions,
)
from memento.graph_debug.refresh import (  # type: ignore[import-untyped]
    GraphEmbeddingRefreshCoordinator,
)


class Snapshot:
    def overview(self) -> GraphOverview:
        return GraphOverview(
            revisions=GraphRevisions(repository="main", index="main", stale=False),
            metrics=GraphMetrics(
                memory_count=3,
                markdown_bytes=0,
                asset_bytes=0,
                explicit_edges=0,
                broken_edges=0,
                orphan_count=0,
            ),
            layout_seed="main",
        )

    def paths_for_ids(self, ids: tuple[str, ...]) -> tuple[str, ...]:
        paths = {"a": "/a.md", "b": "/b.md"}
        return tuple(paths[item] for item in ids if item in paths)


class Worker:
    def __init__(self, accepted: bool = True) -> None:
        self.accepted = accepted
        self.calls: list[tuple[Any, ...]] = []

    def enqueue(self, root: Path, revision: str, *, paths: tuple[str, ...] | None = None) -> bool:
        self.calls.append((str(root), revision, list(paths) if paths is not None else None))
        return self.accepted

    def state(self) -> Any:
        return type(
            "State",
            (),
            {
                "alive": True,
                "running": True,
                "pending": False,
                "last_error": "boom",
                "pause_reason": "pause",
                "current_path": "/a.md",
                "completed": 2,
            },
        )()


def fixtures() -> dict[str, Any]:
    worker = Worker()
    coordinator = GraphEmbeddingRefreshCoordinator(
        GraphExplorerConfig(refresh_max_paths=2),
        repository_root=Path("/repo"),
        snapshot_service=Snapshot(),
        worker=worker,
    )
    selected = asdict(coordinator.enqueue(scope="selected", concept_ids=("b", "a", "b")))
    full = asdict(coordinator.enqueue(scope="full", confirm_full=True))
    none = asdict(
        GraphEmbeddingRefreshCoordinator(
            GraphExplorerConfig(),
            repository_root=Path("/repo"),
            snapshot_service=Snapshot(),
            worker=None,
        ).state()
    )
    errors: dict[str, str] = {}
    calls = {
        "full_confirmation": lambda: coordinator.enqueue(scope="full"),
        "selected_ids": lambda: coordinator.enqueue(scope="selected"),
        "limit": lambda: coordinator.enqueue(scope="visible", concept_ids=("a", "b", "c")),
        "unknown": lambda: coordinator.enqueue(scope="selected", concept_ids=("missing",)),
        "scope": lambda: coordinator.enqueue(scope="bad"),
    }
    for name, call in calls.items():
        try:
            call()  # type: ignore[no-untyped-call]
        except Exception as exc:
            errors[name] = str(exc)
    return {
        "selected": selected,
        "full": full,
        "none": none,
        "calls": worker.calls,
        "errors": errors,
    }
