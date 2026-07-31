from __future__ import annotations

import hashlib
import json
import sqlite3
import threading
import time
from collections.abc import Callable, Sequence
from datetime import UTC, datetime
from pathlib import Path
from typing import cast

import pytest

from memento.activity import ActivityClock
from memento.app import build_runtime
from memento.config import SemanticSearchConfig
from memento.derived.embeddings_worker import (
    ProgressiveEmbeddingPolicy,
    SemanticEmbeddingRefreshWorker,
)
from memento.derived.index import DerivedIndex
from memento.repository.frontmatter import serialize_concept
from memento.repository.schema import ConceptDocument, ConceptFrontmatter, ConceptStatus
from memento.semantic import EmbeddingClient, EmbeddingModelInfo, SemanticSearchError


class RecordingFakeEmbedder(EmbeddingClient):
    def __init__(
        self,
        *,
        model_id: str = "fake-384",
        revision: str = "model-rev-1",
        delay_seconds: float = 0.0,
        gate: threading.Event | None = None,
    ) -> None:
        self._info = EmbeddingModelInfo(
            model_id=model_id,
            dimensions=384,
            revision=revision,
            max_batch=32,
            max_input_chars=4096,
        )
        self._delay_seconds = delay_seconds
        self._gate = gate
        self.batch_sizes: list[int] = []
        self.batch_started = threading.Event()
        self.closed = False
        self.closed_during_batch = False

    def model_info(self) -> EmbeddingModelInfo:
        return self._info

    def embed(self, text: str, *, cancelled: Callable[[], bool] | None = None) -> tuple[float, ...]:
        if cancelled is not None and cancelled():
            raise SemanticSearchError("cancelled")
        return self._embed_text(text)

    def embed_batch(
        self, texts: Sequence[str], *, cancelled: Callable[[], bool] | None = None
    ) -> tuple[tuple[float, ...], ...]:
        if cancelled is not None and cancelled():
            raise SemanticSearchError("cancelled")
        self.batch_started.set()
        self.batch_sizes.append(len(texts))
        if self._gate is not None and not self._gate.wait(timeout=5.0):
            raise SemanticSearchError("gate timeout")
        if self._delay_seconds > 0:
            time.sleep(self._delay_seconds)
        if self.closed:
            self.closed_during_batch = True
        return tuple(self._embed_text(text) for text in texts)

    def close(self) -> None:
        self.closed = True

    def _embed_text(self, text: str) -> tuple[float, ...]:
        digest = hashlib.sha256(text.encode("utf-8")).digest()
        values = [0.0] * self._info.dimensions
        lowered = text.casefold()
        for index in range(self._info.dimensions):
            byte = digest[index % len(digest)]
            values[index] = ((byte + 1) / 255.0) * (1.0 if index % 2 == 0 else -1.0)
        values[0] += lowered.count("alpha") * 10.0
        values[1] += lowered.count("beta") * 10.0
        values[2] += lowered.count("gamma") * 10.0
        return tuple(values)


class FakeRefreshIndex:
    def __init__(self) -> None:
        self.calls: list[str] = []
        self.path_calls: list[tuple[str, ...]] = []
        self.pending_paths: list[str] = []
        self.started = threading.Event()
        self.release = threading.Event()

    def refresh_embeddings(self, bundle_root: Path, *, repo_revision: str) -> None:
        del bundle_root
        self.calls.append(repo_revision)
        self.started.set()
        if repo_revision == "rev-1":
            assert self.release.wait(timeout=5.0)

    def refresh_embedding_paths(
        self, bundle_root: Path, *, repo_revision: str, paths: tuple[str, ...]
    ) -> None:
        del bundle_root, repo_revision
        self.path_calls.append(paths)
        for path in paths:
            if path in self.pending_paths:
                self.pending_paths.remove(path)

    def pending_embedding_paths(self, *, limit: int = 1) -> tuple[str, ...]:
        return tuple(self.pending_paths[:limit])


@pytest.fixture()
def semantic_bundle(tmp_path: Path) -> Path:
    bundle = tmp_path / "bundle"
    write_concept(
        bundle / "projects" / "alpha.md",
        concept_id="alpha-id",
        title="Alpha",
        body="# Alpha\n\nalpha visible\n",
    )
    write_concept(
        bundle / "projects" / "beta.md",
        concept_id="beta-id",
        title="Beta",
        body="# Beta\n\nbeta visible\n",
    )
    return bundle


@pytest.fixture()
def runtime_config_path(tmp_path: Path) -> tuple[Path, Path]:
    root = tmp_path / "state"
    seed = tmp_path / "seed"
    write_concept(
        seed / "projects" / "alpha.md",
        concept_id="alpha-id",
        title="Alpha",
        body="# Alpha\n\nalpha visible\n",
    )
    write_concept(
        seed / "projects" / "beta.md",
        concept_id="beta-id",
        title="Beta",
        body="# Beta\n\nbeta visible\n",
    )
    config_path = tmp_path / "config.json"
    config_path.write_text(
        json.dumps(
            {
                "schema_version": 2,
                "repository": {"root_path": str(root), "bundle_root": "/"},
                "authorization": {
                    "principals": {
                        "smith": {
                            "roles": ["reader"],
                            "token_env": "MEMENTO_TOKEN_SMITH",
                            "read_prefixes": ["/projects/"],
                            "write_prefixes": ["/projects/"],
                        }
                    }
                },
                "intelligent_tiers": {
                    "semantic_search": {
                        "enabled": True,
                        "worker_mode": "subprocess",
                        "worker_path": "/tmp/memento-embed",
                        "model_path": "/tmp/fake-model.gte",
                        "model_id": "fake-384",
                        "dimensions": 384,
                        "max_batch_size": 2,
                    }
                },
            }
        ),
        encoding="utf-8",
    )
    return config_path, seed


def semantic_config(**overrides: object) -> SemanticSearchConfig:
    payload = {
        "enabled": True,
        "model_path": "/tmp/fake-model.gte",
        "ffi_library_path": "/tmp/libmemento_ffi.so",
        "model_id": "fake-384",
        "dimensions": 384,
        "max_batch_size": 2,
        "max_candidates": 50,
        "default_search_mode": "lexical",
    }
    payload.update(overrides)
    return SemanticSearchConfig.model_validate(payload)


def test_deferred_semantic_refresh_lags_and_catches_up(
    semantic_bundle: Path, tmp_path: Path
) -> None:
    embedder = RecordingFakeEmbedder()
    index = DerivedIndex(
        tmp_path / "derived.sqlite",
        semantic_config=semantic_config(),
        embedding_client=embedder,
        defer_embeddings=True,
    )

    index.rebuild(semantic_bundle, repo_revision="rev-1")
    assert index.get_state().index_revision == "rev-1"
    assert embedder.batch_sizes == []
    status = index.semantic_status()
    assert status.ready is False
    assert status.embedding_revision is None

    index.refresh_embeddings(semantic_bundle, repo_revision="rev-1")
    status = index.semantic_status()
    assert status.ready is True
    assert status.embedding_revision == "rev-1"
    assert embedder.batch_sizes == [2]

    write_concept(
        semantic_bundle / "projects" / "alpha.md",
        concept_id="alpha-id",
        title="Alpha",
        body="# Alpha\n\nalpha updated visible\n",
    )
    index.update_paths(
        semantic_bundle,
        repo_revision="rev-2",
        changed_paths=("/projects/alpha.md",),
    )
    assert index.get_state().index_revision == "rev-2"
    assert embedder.batch_sizes == [2]

    lagging = index.semantic_status()
    assert lagging.ready is False
    assert lagging.embedding_revision == "rev-1"
    with sqlite3.connect(index.db_path) as connection:
        row = connection.execute(
            "SELECT COUNT(*) FROM concept_embeddings WHERE concept_id = ?",
            ("alpha-id",),
        ).fetchone()
    assert row == (0,)

    index.refresh_embeddings(semantic_bundle, repo_revision="rev-2")
    caught_up = index.semantic_status()
    assert caught_up.ready is True
    assert caught_up.embedding_revision == "rev-2"
    assert embedder.batch_sizes == [2, 1]


def test_refresh_embeddings_batches_full_bundle(semantic_bundle: Path, tmp_path: Path) -> None:
    write_concept(
        semantic_bundle / "projects" / "gamma.md",
        concept_id="gamma-id",
        title="Gamma",
        body="# Gamma\n\ngamma visible\n",
    )
    write_concept(
        semantic_bundle / "projects" / "delta.md",
        concept_id="delta-id",
        title="Delta",
        body="# Delta\n\nvisible\n",
    )
    write_concept(
        semantic_bundle / "projects" / "epsilon.md",
        concept_id="epsilon-id",
        title="Epsilon",
        body="# Epsilon\n\nvisible\n",
    )
    embedder = RecordingFakeEmbedder()
    index = DerivedIndex(
        tmp_path / "batched.sqlite",
        semantic_config=semantic_config(max_batch_size=2),
        embedding_client=embedder,
        defer_embeddings=True,
    )

    index.rebuild(semantic_bundle, repo_revision="rev-batch")
    index.refresh_embeddings(semantic_bundle, repo_revision="rev-batch")

    assert embedder.batch_sizes == [2, 2, 1]
    assert index.semantic_status().ready is True


def test_rebuild_preserves_ready_embeddings_and_marks_changed_content_stale(
    semantic_bundle: Path, tmp_path: Path
) -> None:
    embedder = RecordingFakeEmbedder()
    index = DerivedIndex(
        tmp_path / "preserved.sqlite",
        semantic_config=semantic_config(),
        embedding_client=embedder,
        defer_embeddings=True,
    )
    index.rebuild(semantic_bundle, repo_revision="rev-1")
    index.refresh_embeddings(semantic_bundle, repo_revision="rev-1")
    assert embedder.batch_sizes == [2]

    index.rebuild(semantic_bundle, repo_revision="rev-2")
    assert index.semantic_status().ready is True
    assert index.semantic_status().embedding_revision == "rev-2"
    assert index.pending_embedding_paths() == ()
    assert embedder.batch_sizes == [2]

    write_concept(
        semantic_bundle / "projects" / "alpha.md",
        concept_id="alpha-id",
        title="Alpha",
        body="# Alpha\n\nchanged after rebuild\n",
    )
    index.rebuild(semantic_bundle, repo_revision="rev-3")
    assert index.pending_embedding_paths(limit=10) == ("/projects/alpha.md",)
    assert index.semantic_status().embedding_revision == "partial"


def test_model_revision_change_marks_persisted_embeddings_stale(
    semantic_bundle: Path, tmp_path: Path
) -> None:
    first = DerivedIndex(
        tmp_path / "model-change.sqlite",
        semantic_config=semantic_config(),
        embedding_client=RecordingFakeEmbedder(revision="model-1"),
        defer_embeddings=True,
    )
    first.rebuild(semantic_bundle, repo_revision="rev-1")
    first.refresh_embeddings(semantic_bundle, repo_revision="rev-1")
    assert first.pending_embedding_paths(limit=10) == ()

    reopened = DerivedIndex(
        first.db_path,
        semantic_config=semantic_config(),
        embedding_client=RecordingFakeEmbedder(revision="model-2"),
        defer_embeddings=True,
    )
    assert reopened.mark_model_stale() == 2
    assert reopened.pending_embedding_paths(limit=10) == (
        "/projects/alpha.md",
        "/projects/beta.md",
    )


def test_pending_paths_skip_degraded_until_manual_or_content_change(
    semantic_bundle: Path, tmp_path: Path
) -> None:
    index = DerivedIndex(
        tmp_path / "degraded.sqlite",
        semantic_config=semantic_config(),
        embedding_client=RecordingFakeEmbedder(),
        defer_embeddings=True,
    )
    index.rebuild(semantic_bundle, repo_revision="rev-1")
    with sqlite3.connect(index.db_path) as connection:
        connection.execute(
            """
            INSERT INTO concept_embeddings(
                concept_id,path,embedding_text_hash,model_id,dimensions,embedding_revision,
                status,model_revision,embedding_blob,embedding_norm,updated_at,error_message
            ) VALUES('alpha-id','/projects/alpha.md','hash','fake-384',384,'rev-1',
                'degraded','model-rev-1',NULL,NULL,'now','failed')
            """
        )
        connection.commit()
    assert index.pending_embedding_paths(limit=10) == ("/projects/beta.md",)


def test_progressive_worker_resumes_pending_state_and_prioritizes_manual_paths(
    tmp_path: Path,
) -> None:
    fake_index = FakeRefreshIndex()
    fake_index.pending_paths = ["/a.md", "/b.md"]
    worker = SemanticEmbeddingRefreshWorker(
        cast(DerivedIndex, fake_index),
        policy=ProgressiveEmbeddingPolicy(
            enabled=True,
            startup_delay_seconds=0,
            interactive_idle_seconds=0,
            delay_seconds=0,
            load_average_limit=99,
        ),
        load_average=lambda: 0,
    )
    try:
        assert worker.enqueue(tmp_path, "rev-1", paths=("/manual.md",)) is True
        assert wait_for(lambda: len(fake_index.path_calls) == 3, timeout_seconds=2.0)
        assert fake_index.path_calls == [("/manual.md",), ("/a.md",), ("/b.md",)]
        assert worker.state().completed == 3
    finally:
        worker.close()


def test_progressive_worker_pauses_for_activity_load_and_pacing(tmp_path: Path) -> None:
    fake_index = FakeRefreshIndex()
    fake_index.pending_paths = ["/a.md"]
    now = [0.0]
    load = [2.0]
    activity = ActivityClock(lambda: now[0])
    worker = SemanticEmbeddingRefreshWorker(
        cast(DerivedIndex, fake_index),
        policy=ProgressiveEmbeddingPolicy(
            enabled=True,
            startup_delay_seconds=10,
            interactive_idle_seconds=5,
            delay_seconds=30,
            load_average_limit=1.5,
        ),
        activity=activity,
        load_average=lambda: load[0],
        cpu_count=lambda: 1,
        monotonic=lambda: now[0],
    )
    try:
        worker.enqueue(tmp_path, "rev-1")
        assert wait_for(lambda: worker.state().pause_reason == "startup", timeout_seconds=1.0)
        now[0] = 11
        activity.touch()
        assert wait_for(lambda: worker.state().pause_reason == "interactive", timeout_seconds=1.5)
        now[0] = 17
        assert wait_for(lambda: worker.state().pause_reason == "load", timeout_seconds=1.5)
        load[0] = 0
        assert wait_for(lambda: fake_index.path_calls == [("/a.md",)], timeout_seconds=1.5)
        fake_index.pending_paths = ["/b.md"]
        worker.enqueue(tmp_path, "rev-1")
        assert wait_for(lambda: worker.state().pause_reason == "pacing", timeout_seconds=1.5)
        now[0] = 48
        assert wait_for(lambda: fake_index.path_calls[-1] == ("/b.md",), timeout_seconds=1.5)
    finally:
        worker.close()


def test_progressive_load_limit_is_normalized_by_cpu_count(tmp_path: Path) -> None:
    fake_index = FakeRefreshIndex()
    fake_index.pending_paths = ["/a.md"]
    worker = SemanticEmbeddingRefreshWorker(
        cast(DerivedIndex, fake_index),
        policy=ProgressiveEmbeddingPolicy(
            enabled=True,
            startup_delay_seconds=0,
            interactive_idle_seconds=0,
            delay_seconds=0,
            load_average_limit=1.5,
        ),
        load_average=lambda: 5.0,
        cpu_count=lambda: 4,
    )
    try:
        worker.enqueue(tmp_path, "rev-1")
        assert wait_for(lambda: fake_index.path_calls == [("/a.md",)], timeout_seconds=1.5)
    finally:
        worker.close()


def test_embedding_refresh_worker_coalesces_latest_revision_and_close(tmp_path: Path) -> None:
    fake_index = FakeRefreshIndex()
    worker = SemanticEmbeddingRefreshWorker(cast(DerivedIndex, fake_index))
    try:
        assert worker.enqueue(tmp_path, "rev-1") is True
        assert fake_index.started.wait(timeout=1.0)
        assert worker.running is True

        assert worker.enqueue(tmp_path, "rev-2") is True
        assert worker.enqueue(tmp_path, "rev-3") is True
        fake_index.release.set()

        assert worker.wait_idle(timeout_seconds=2.0) is True
        assert fake_index.calls == ["rev-1", "rev-3"]
        assert worker.state().pending is False
        assert worker.state().last_error is None

        worker.close()
        assert worker.enqueue(tmp_path, "rev-4") is False
        assert fake_index.calls == ["rev-1", "rev-3"]
    finally:
        worker.close()


def test_runtime_subprocess_worker_reports_lag_then_catches_up_and_closes(
    runtime_config_path: tuple[Path, Path],
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    config_path, seed = runtime_config_path
    gate = threading.Event()
    embedder = RecordingFakeEmbedder(gate=gate)

    monkeypatch.setattr("memento.app.SubprocessEmbeddingClient", lambda *args, **kwargs: embedder)
    runtime = build_runtime(config_path, bootstrap_seed=seed)
    try:
        assert runtime.embedding_refresh_worker is not None
        assert embedder.batch_started.wait(timeout=1.0)

        status = runtime.status_snapshot()
        assert status["index_revision"] == status["repo_revision"]
        assert status["semantic_search"]["ready"] is False

        release = threading.Thread(target=_release_gate, args=(gate, 0.05), daemon=True)
        release.start()
        assert wait_for(
            lambda: runtime.status_snapshot()["semantic_search"]["ready"] is True,
            timeout_seconds=2.0,
        )
        runtime.close()
        release.join(timeout=1.0)
        assert embedder.closed is True
        assert embedder.closed_during_batch is False
    finally:
        if not runtime.closed:
            gate.set()
            runtime.close()


def write_concept(path: Path, *, concept_id: str, title: str, body: str) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(
        serialize_concept(
            ConceptDocument(
                frontmatter=ConceptFrontmatter(
                    schema_version=1,
                    id=concept_id,
                    type="project",
                    title=title,
                    description=title,
                    tags=("shared",),
                    aliases=(),
                    source_refs=(),
                    supersedes=(),
                    status=ConceptStatus.ACTIVE,
                    created_at=datetime(2026, 7, 17, tzinfo=UTC),
                    updated_at=datetime(2026, 7, 17, tzinfo=UTC),
                    updated_by="tests",
                ),
                body=body,
            )
        ),
        encoding="utf-8",
    )


def wait_for(predicate: Callable[[], bool], *, timeout_seconds: float) -> bool:
    deadline = time.monotonic() + timeout_seconds
    while time.monotonic() < deadline:
        if predicate():
            return True
        time.sleep(0.01)
    return predicate()


def _release_gate(gate: threading.Event, delay_seconds: float) -> None:
    time.sleep(delay_seconds)
    gate.set()
