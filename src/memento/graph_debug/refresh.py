from __future__ import annotations

from dataclasses import asdict, dataclass
from pathlib import Path

from memento.config import GraphExplorerConfig
from memento.derived.embeddings_worker import SemanticEmbeddingRefreshWorker
from memento.graph_debug.snapshot import GraphSnapshotError, GraphSnapshotService


@dataclass(frozen=True, slots=True)
class GraphEmbeddingRefreshState:
    available: bool
    running: bool
    pending: bool
    last_error: str | None
    last_scope: str | None
    queued_paths: int
    repository_revision: str | None


class GraphEmbeddingRefreshCoordinator:
    def __init__(
        self,
        config: GraphExplorerConfig,
        *,
        repository_root: Path,
        snapshot_service: GraphSnapshotService,
        worker: SemanticEmbeddingRefreshWorker | None,
    ) -> None:
        self._config = config
        self._repository_root = repository_root
        self._snapshot_service = snapshot_service
        self._worker = worker
        self._last_scope: str | None = None
        self._queued_paths = 0
        self._repository_revision: str | None = None

    def enqueue(
        self,
        *,
        scope: str,
        concept_ids: tuple[str, ...] = (),
        confirm_full: bool = False,
    ) -> GraphEmbeddingRefreshState:
        if self._worker is None:
            raise GraphSnapshotError("semantic embedding refresh is unavailable")
        overview = self._snapshot_service.overview()
        revision = overview.revisions.repository
        paths: tuple[str, ...] | None
        if scope == "full":
            if not confirm_full:
                raise GraphSnapshotError("full embedding refresh requires confirmation")
            paths = None
            queued = overview.metrics.memory_count
        elif scope in {"selected", "visible"}:
            if not concept_ids:
                raise GraphSnapshotError(f"{scope} embedding refresh requires concept ids")
            unique = tuple(sorted(dict.fromkeys(concept_ids)))
            if len(unique) > self._config.refresh_max_paths:
                raise GraphSnapshotError("embedding refresh exceeds configured path limit")
            paths = tuple(self._snapshot_service.paths_for_ids(unique))
            if len(paths) != len(unique):
                raise GraphSnapshotError("embedding refresh includes unknown memories")
            queued = len(paths)
        else:
            raise GraphSnapshotError("unsupported embedding refresh scope")
        if not self._worker.enqueue(
            self._repository_root,
            revision,
            paths=paths,
        ):
            raise GraphSnapshotError("embedding refresh worker is closed")
        self._last_scope = scope
        self._queued_paths = queued
        self._repository_revision = revision
        return self.state()

    def state(self) -> GraphEmbeddingRefreshState:
        if self._worker is None:
            return GraphEmbeddingRefreshState(
                available=False,
                running=False,
                pending=False,
                last_error=None,
                last_scope=self._last_scope,
                queued_paths=self._queued_paths,
                repository_revision=self._repository_revision,
            )
        state = self._worker.state()
        return GraphEmbeddingRefreshState(
            available=True,
            running=state.running,
            pending=state.pending,
            last_error=state.last_error,
            last_scope=self._last_scope,
            queued_paths=self._queued_paths,
            repository_revision=self._repository_revision,
        )

    def state_dict(self) -> dict[str, object]:
        return asdict(self.state())
