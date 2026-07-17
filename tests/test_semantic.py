from __future__ import annotations

import hashlib
import math
import sqlite3
import struct
import subprocess
import sys
from collections.abc import Callable, Sequence
from datetime import datetime, timezone
from pathlib import Path

import pytest

from memento.authz import EffectivePolicy
from memento.config import (
    AuthorizationConfig,
    IntelligentTiersConfig,
    NamespacePolicy,
    Principal,
    RepositoryConfig,
    SemanticSearchConfig,
    ServiceConfig,
)
from memento.control.db import connect_control_db, migrate_control_db
from memento.derived.index import DerivedIndex, SearchMode
from memento.ffi import (
    FfiCancelledError,
    FfiClosedError,
    FfiFiniteError,
    FfiModelError,
    RustFfiLibrary,
)
from memento.repository.frontmatter import serialize_concept
from memento.repository.git import GitRepositoryPaths, bootstrap_repository, get_main_revision
from memento.repository.schema import ConceptDocument, ConceptFrontmatter, ConceptStatus
from memento.repository.transactions import TransactionManager, TransactionRequest
from memento.semantic import (
    EmbeddingClient,
    EmbeddingModelInfo,
    SemanticSearchError,
    cosine_similarity,
    pack_f32le,
)
from memento.service import MemoryService, ServiceContext, ServiceDependencies


class FakeEmbedder(EmbeddingClient):
    def __init__(
        self,
        *,
        model_id: str = "fake-384",
        revision: str = "rev-1",
        fail_on: str | None = None,
    ) -> None:
        self._info = EmbeddingModelInfo(
            model_id=model_id,
            dimensions=384,
            revision=revision,
            max_batch=32,
            max_input_chars=4096,
        )
        self._fail_on = fail_on.casefold() if fail_on is not None else None

    def model_info(self) -> EmbeddingModelInfo:
        return self._info

    def embed(self, text: str, *, cancelled: Callable[[], bool] | None = None) -> tuple[float, ...]:
        if cancelled is not None and cancelled():
            raise SemanticSearchError("cancelled")
        lowered = text.casefold()
        if self._fail_on is not None and self._fail_on in lowered:
            raise SemanticSearchError(f"synthetic embed failure for {self._fail_on}")
        digest = hashlib.sha256(text.encode("utf-8")).digest()
        values = [0.0] * self._info.dimensions
        for index in range(self._info.dimensions):
            byte = digest[index % len(digest)]
            values[index] = ((byte + 1) / 255.0) * (1.0 if index % 2 == 0 else -1.0)
        values[0] += lowered.count("red") * 10.0
        values[1] += lowered.count("blue") * 10.0
        values[2] += lowered.count("green") * 10.0
        values[3] += lowered.count("visible") * 3.0
        values[4] += lowered.count("common") * 2.0
        values[5] += lowered.count("ghost") * 4.0
        return tuple(values)

    def embed_batch(
        self, texts: Sequence[str], *, cancelled: Callable[[], bool] | None = None
    ) -> tuple[tuple[float, ...], ...]:
        if cancelled is not None and cancelled():
            raise SemanticSearchError("cancelled")
        return tuple(self.embed(text, cancelled=cancelled) for text in texts)


@pytest.fixture()
def semantic_repo_paths(tmp_path: Path) -> GitRepositoryPaths:
    seed = tmp_path / "seed"
    write_concept(
        seed / "projects" / "alpha.md",
        concept_id="alpha-id",
        concept_type="project",
        title="Alpha",
        description="Visible alpha",
        tags=("shared",),
        body="# Alpha\n\ncommon common common common common blue visible\n",
    )
    write_concept(
        seed / "projects" / "beta.md",
        concept_id="beta-id",
        concept_type="project",
        title="Beta",
        description="Visible beta",
        tags=("shared",),
        body="# Beta\n\ncommon common common common red visible\n",
    )
    write_concept(
        seed / "projects" / "gamma.md",
        concept_id="gamma-id",
        concept_type="project",
        title="Gamma",
        description="Visible gamma",
        tags=("shared",),
        body="# Gamma\n\ncommon common common red green visible\n",
    )
    write_concept(
        seed / "secret" / "ghost.md",
        concept_id="ghost-id",
        concept_type="project",
        title="Ghost",
        description="Hidden best semantic match",
        tags=("hidden",),
        body="# Ghost\n\nred red red red red ghost visible\n",
    )
    paths = GitRepositoryPaths(
        bare_dir=tmp_path / "repo.git",
        current_dir=tmp_path / "current",
        worktrees_dir=tmp_path / "worktrees",
    )
    bootstrap_repository(paths, seed)
    return paths


@pytest.fixture()
def visible_policy() -> EffectivePolicy:
    return EffectivePolicy(
        principal="smith",
        roles=("reader",),
        read_prefixes=("/projects/",),
        write_prefixes=("/projects/",),
    )


@pytest.fixture()
def hidden_policy() -> EffectivePolicy:
    return EffectivePolicy(
        principal="ghost",
        roles=("reader",),
        read_prefixes=("/projects/", "/secret/"),
        write_prefixes=("/projects/", "/secret/"),
    )


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


def test_fake_embedder_full_rebuild_incremental_update_delete_and_model_invalidation(
    tmp_path: Path,
    semantic_repo_paths: GitRepositoryPaths,
) -> None:
    index = DerivedIndex(
        tmp_path / "semantic.sqlite",
        semantic_config=semantic_config(),
        embedding_client=FakeEmbedder(revision="rev-a"),
    )
    revision = get_main_revision(semantic_repo_paths)
    index.rebuild(semantic_repo_paths.current_dir, repo_revision=revision)

    with sqlite3.connect(index.db_path) as connection:
        row = connection.execute(
            "SELECT COUNT(*), MIN(dimensions), MAX(dimensions) FROM concept_embeddings"
        ).fetchone()
        assert row == (4, 384, 384)

    alpha_path = semantic_repo_paths.current_dir / "projects" / "alpha.md"
    write_concept(
        alpha_path,
        concept_id="alpha-id",
        concept_type="project",
        title="Alpha",
        description="Visible alpha updated",
        tags=("shared", "updated"),
        body="# Alpha\n\ncommon common common green visible\n",
    )
    gamma_path = semantic_repo_paths.current_dir / "projects" / "gamma.md"
    gamma_path.unlink()
    index.update_paths(
        semantic_repo_paths.current_dir,
        repo_revision="rev-2",
        changed_paths=("/projects/alpha.md", "/projects/gamma.md"),
    )

    with sqlite3.connect(index.db_path) as connection:
        row = connection.execute(
            "SELECT status, embedding_revision FROM concept_embeddings WHERE concept_id = ?",
            ("alpha-id",),
        ).fetchone()
        assert row == ("ready", "rev-2")
        deleted = connection.execute(
            "SELECT COUNT(*) FROM concept_embeddings WHERE concept_id = ?",
            ("gamma-id",),
        ).fetchone()
        assert deleted == (0,)

    invalidated = DerivedIndex(
        index.db_path,
        semantic_config=semantic_config(model_id="fake-384-v2"),
        embedding_client=FakeEmbedder(model_id="fake-384-v2", revision="rev-b"),
    )
    invalidated.update_paths(
        semantic_repo_paths.current_dir,
        repo_revision="rev-3",
        changed_paths=("/projects/alpha.md",),
    )
    status = invalidated.semantic_status()
    assert status.enabled is True
    assert status.ready is False
    assert status.embedding_revision == "rev-3"
    assert any("semantic_embeddings_degraded" in warning for warning in status.warnings)

    with sqlite3.connect(index.db_path) as connection:
        rows = connection.execute(
            "SELECT concept_id, status, model_id, model_revision FROM concept_embeddings ORDER BY concept_id"
        ).fetchall()
    assert rows == [
        ("alpha-id", "ready", "fake-384-v2", "rev-b"),
        ("beta-id", "stale", "fake-384-v2", "rev-b"),
        ("ghost-id", "stale", "fake-384-v2", "rev-b"),
    ]


def test_semantic_search_disabled_or_unavailable_falls_back_to_lexical(
    tmp_path: Path,
    semantic_repo_paths: GitRepositoryPaths,
    visible_policy: EffectivePolicy,
) -> None:
    revision = get_main_revision(semantic_repo_paths)
    lexical_only = DerivedIndex(tmp_path / "lexical.sqlite")
    lexical_only.rebuild(semantic_repo_paths.current_dir, repo_revision=revision)
    baseline = lexical_only.search(
        policy=visible_policy, query="common", search_mode=SearchMode.LEXICAL
    )

    unavailable = DerivedIndex(
        tmp_path / "semantic-unavailable.sqlite",
        semantic_config=semantic_config(),
        embedding_client=None,
    )
    unavailable.rebuild(semantic_repo_paths.current_dir, repo_revision=revision)
    semantic_page = unavailable.search(
        policy=visible_policy,
        query="common",
        search_mode=SearchMode.SEMANTIC,
    )
    hybrid_page = unavailable.search(
        policy=visible_policy,
        query="common",
        search_mode=SearchMode.HYBRID,
    )

    assert [item.path for item in semantic_page.results] == [item.path for item in baseline.results]
    assert [item.path for item in hybrid_page.results] == [item.path for item in baseline.results]
    assert semantic_page.warnings and "semantic_search_unavailable" in semantic_page.warnings[0]
    assert hybrid_page.warnings and "semantic_search_unavailable" in hybrid_page.warnings[0]


def test_hidden_best_match_cannot_change_visible_semantic_or_hybrid_scores(
    tmp_path: Path,
    semantic_repo_paths: GitRepositoryPaths,
    visible_policy: EffectivePolicy,
    hidden_policy: EffectivePolicy,
) -> None:
    index = DerivedIndex(
        tmp_path / "semantic-hidden.sqlite",
        semantic_config=semantic_config(),
        embedding_client=FakeEmbedder(),
    )
    index.rebuild(
        semantic_repo_paths.current_dir, repo_revision=get_main_revision(semantic_repo_paths)
    )

    visible_semantic = index.search(
        policy=visible_policy,
        query="red visible",
        search_mode=SearchMode.SEMANTIC,
        limit=10,
    )
    visible_hybrid = index.search(
        policy=visible_policy,
        query="common red visible",
        search_mode=SearchMode.HYBRID,
        limit=10,
    )

    ghost_path = semantic_repo_paths.current_dir / "secret" / "ghost.md"
    write_concept(
        ghost_path,
        concept_id="ghost-id",
        concept_type="project",
        title="Ghost",
        description="Even better hidden semantic match",
        tags=("hidden",),
        body="# Ghost\n\nred red red red red red red ghost visible\n",
    )
    index.update_paths(
        semantic_repo_paths.current_dir,
        repo_revision="rev-hidden-2",
        changed_paths=("/secret/ghost.md",),
    )

    visible_semantic_after = index.search(
        policy=visible_policy,
        query="red visible",
        search_mode=SearchMode.SEMANTIC,
        limit=10,
    )
    visible_hybrid_after = index.search(
        policy=visible_policy,
        query="common red visible",
        search_mode=SearchMode.HYBRID,
        limit=10,
    )
    hidden_semantic = index.search(
        policy=hidden_policy,
        query="red visible",
        search_mode=SearchMode.SEMANTIC,
        limit=10,
    )

    assert [(item.path, round(item.score, 6)) for item in visible_semantic.results] == [
        (item.path, round(item.score, 6)) for item in visible_semantic_after.results
    ]
    assert [(item.path, round(item.score, 6)) for item in visible_hybrid.results] == [
        (item.path, round(item.score, 6)) for item in visible_hybrid_after.results
    ]
    assert "/secret/ghost.md" in [item.path for item in hidden_semantic.results]
    assert visible_semantic_after.results[0].path != "/secret/ghost.md"


def test_lexical_default_unchanged_and_hybrid_rrf_is_deterministic(
    tmp_path: Path,
    semantic_repo_paths: GitRepositoryPaths,
    visible_policy: EffectivePolicy,
) -> None:
    index = DerivedIndex(
        tmp_path / "rrf.sqlite",
        semantic_config=semantic_config(),
        embedding_client=FakeEmbedder(),
    )
    index.rebuild(
        semantic_repo_paths.current_dir, repo_revision=get_main_revision(semantic_repo_paths)
    )

    default_page = index.search(policy=visible_policy, query="common", limit=10)
    lexical_page = index.search(
        policy=visible_policy,
        query="common",
        limit=10,
        search_mode=SearchMode.LEXICAL,
    )
    assert [item.path for item in default_page.results] == [
        item.path for item in lexical_page.results
    ]
    assert [round(item.score, 6) for item in default_page.results] == [
        round(item.score, 6) for item in lexical_page.results
    ]

    hybrid_runs = [
        index.search(
            policy=visible_policy,
            query="common red visible",
            search_mode=SearchMode.HYBRID,
            limit=10,
        )
        for _ in range(3)
    ]
    expected = [item.path for item in hybrid_runs[0].results]
    assert expected[0] == "/projects/beta.md"
    assert expected == [item.path for item in hybrid_runs[1].results]
    assert expected == [item.path for item in hybrid_runs[2].results]
    assert [round(item.score, 9) for item in hybrid_runs[0].results] == [
        round(item.score, 9) for item in hybrid_runs[1].results
    ]


def test_memory_search_api_search_mode_and_status_warnings(
    tmp_path: Path,
    semantic_repo_paths: GitRepositoryPaths,
) -> None:
    control = connect_control_db(tmp_path / "control.sqlite")
    migrate_control_db(control)
    config = ServiceConfig(
        schema_version=2,
        repository=RepositoryConfig(root_path=str(tmp_path / "state")),
        authorization=AuthorizationConfig(
            principals={
                "smith": NamespacePolicy(
                    roles=("reader",),
                    token_env="MEMENTO_TOKEN_SMITH",
                    read_prefixes=("/projects/",),
                    write_prefixes=("/projects/",),
                )
            }
        ),
        intelligent_tiers=IntelligentTiersConfig(
            semantic_search=semantic_config(
                ffi_library_path="/tmp/libmemento_ffi.so",
                sqlite_extension_path="/definitely/missing/libmemento_sqlite_vector.so",
                default_search_mode="lexical",
            )
        ),
    )
    derived_index = DerivedIndex(
        tmp_path / "service.sqlite",
        semantic_config=config.intelligent_tiers.semantic_search,
        embedding_client=None,
    )
    derived_index.rebuild(
        semantic_repo_paths.current_dir, repo_revision=get_main_revision(semantic_repo_paths)
    )

    def apply_update(
        materialized_root: Path, repo_revision: str, changed_paths: tuple[str, ...]
    ) -> None:
        if changed_paths:
            derived_index.update_paths(
                materialized_root,
                repo_revision=repo_revision,
                changed_paths=changed_paths,
            )
        else:
            derived_index.rebuild(materialized_root, repo_revision=repo_revision)

    service = MemoryService(
        ServiceDependencies(
            config=config,
            repo_paths=semantic_repo_paths,
            control_connection=control,
            derived_index=derived_index,
            transaction_manager=TransactionManager(
                control, semantic_repo_paths, derived_update=apply_update
            ),
        )
    )
    context = ServiceContext(Principal(name="smith", roles=("reader",)))

    status = service.memory_status(context)
    assert status.status == "success"
    assert status.warnings
    assert any("sqlite_vector_extension_unavailable" in warning for warning in status.warnings)
    status_data = status.data
    assert status_data is not None
    assert status_data["features"]["semantic_search"] is True
    assert status_data["readiness"]["semantic_search"]["ready"] is False

    default_search = service.memory_search(context, query="common")
    assert default_search.status == "success"
    assert default_search.data is not None
    assert default_search.data["search_mode"] == "lexical"

    fallback_search = service.memory_search(context, query="common", search_mode="semantic")
    assert fallback_search.status == "success"
    assert fallback_search.warnings
    assert "semantic_search_unavailable" in fallback_search.warnings[0]

    invalid_search = service.memory_search(context, query="common", search_mode="bogus")
    assert invalid_search.status == "error"
    assert invalid_search.error_class == "validation_error"

    control.close()


def test_transaction_succeeds_when_embedding_update_degrades_but_lexical_advances(
    tmp_path: Path,
    semantic_repo_paths: GitRepositoryPaths,
) -> None:
    control = connect_control_db(tmp_path / "control.sqlite")
    migrate_control_db(control)
    derived_index = DerivedIndex(
        tmp_path / "degraded.sqlite",
        semantic_config=semantic_config(),
        embedding_client=FakeEmbedder(fail_on="breaksemantic"),
    )
    base_revision = get_main_revision(semantic_repo_paths)
    derived_index.rebuild(semantic_repo_paths.current_dir, repo_revision=base_revision)

    def apply_update(
        materialized_root: Path, repo_revision: str, changed_paths: tuple[str, ...]
    ) -> None:
        if changed_paths:
            derived_index.update_paths(
                materialized_root,
                repo_revision=repo_revision,
                changed_paths=changed_paths,
            )
        else:
            derived_index.rebuild(materialized_root, repo_revision=repo_revision)

    manager = TransactionManager(control, semantic_repo_paths, derived_update=apply_update)
    result = manager.apply(
        TransactionRequest(
            operation=__import__(
                "memento.control.operations", fromlist=["OperationRequest"]
            ).OperationRequest(
                op_id="semantic-degraded-op",
                principal="smith",
                idempotency_key="semantic-degraded-op",
                tool_name="memory_patch",
                request_json='{"path":"/projects/beta.md"}',
            ),
            expected_revision=base_revision,
            commit_message="memory: semantic degraded update",
            author_name="Rui Carmo",
            author_email="rui.carmo@gmail.com",
        ),
        lambda worktree: _patch_break_semantic(worktree),
    )

    lexical = derived_index.search(
        policy=EffectivePolicy(
            principal="smith",
            roles=("reader",),
            read_prefixes=("/projects/",),
            write_prefixes=("/projects/",),
        ),
        query="breaksemantic",
        search_mode=SearchMode.LEXICAL,
        limit=10,
    )
    assert result.result_revision == lexical.index_revision
    assert [item.path for item in lexical.results] == ["/projects/beta.md"]

    semantic_status = derived_index.semantic_status()
    assert semantic_status.ready is False
    assert any("semantic_embeddings_degraded" in warning for warning in semantic_status.warnings)
    with sqlite3.connect(derived_index.db_path) as connection:
        row = connection.execute(
            "SELECT status, error_message FROM concept_embeddings WHERE concept_id = ?",
            ("beta-id",),
        ).fetchone()
    assert row is not None
    assert row[0] == "degraded"
    assert "breaksemantic" in row[1]
    control.close()


@pytest.mark.parametrize(
    ("field_index", "value", "message"),
    (
        (0, 103, "vocab_size must include reserved token id 103"),
        (3, 0, "num_heads must be positive and divide hidden_size"),
        (5, 1, "max_seq_len must be at least 2"),
    ),
)
def test_ctypes_wrapper_rejects_malformed_model_headers(
    tmp_path: Path,
    field_index: int,
    value: int,
    message: str,
) -> None:
    ffi_library_path = build_rust_cdylib("memento-ffi", "libmemento_ffi")
    library = RustFfiLibrary(ffi_library_path)
    model_path = tmp_path / f"malformed-{field_index}.gte"
    model_path.write_bytes(synthetic_model_bytes(header_overrides={field_index: value}))

    with pytest.raises(FfiModelError, match=message):
        library.load_model(model_path)


def test_ctypes_wrapper_abi_lifecycle_info_embed_batch_cancel_and_errors(tmp_path: Path) -> None:
    ffi_library_path = build_rust_cdylib("memento-ffi", "libmemento_ffi")
    library = RustFfiLibrary(ffi_library_path)
    model_path = tmp_path / "synthetic.gte"
    model_path.write_bytes(synthetic_model_bytes())

    assert library.path == ffi_library_path
    assert library.vector_cosine((1.0, 0.0), (0.5, 0.0)) == pytest.approx(1.0)

    with library.new_cancel_token() as token:
        assert token.pointer is not None
        token.cancel()

    with library.load_model(model_path) as model:
        info = model.info()
        model_info = model.model_info()
        assert info.abi_version == 1
        assert info.dimensions == 4
        assert model_info.dimensions == 4
        assert model_info.model_id == "synthetic.gte"
        assert model_info.revision == hashlib.sha256(model_path.read_bytes()).hexdigest()

        hello = model.embed("hello")
        world = model.embed("world")
        batch = model.embed_batch(("hello", "world"))
        assert len(hello) == 4
        assert len(world) == 4
        assert batch == (hello, world)
        assert math.isclose(cosine_similarity(hello, batch[0]), 1.0, rel_tol=1e-6)

        with pytest.raises(FfiCancelledError):
            model.embed("hello", cancelled=lambda: True)

    closed_model = library.load_model(model_path)
    closed_model.close()
    with pytest.raises(FfiClosedError):
        closed_model.info()
    with pytest.raises(FfiClosedError):
        closed_model.embed("hello")
    with pytest.raises(FfiClosedError):
        closed_model.embed_batch(("hello",))

    other_path = tmp_path / "synthetic-copy.gte"
    other_path.write_bytes(synthetic_model_bytes(body_seed=7.0))
    with library.load_model(other_path) as other_model:
        assert other_model.model_info().model_id == "synthetic-copy.gte"
        assert (
            other_model.model_info().revision != hashlib.sha256(model_path.read_bytes()).hexdigest()
        )

    with pytest.raises(FfiFiniteError):
        library.vector_cosine((1.0, float("nan")), (1.0, 0.0))

    model_path.unlink()


def test_model_info_revision_differs_for_same_basename_with_different_contents(
    tmp_path: Path,
) -> None:
    ffi_library_path = build_rust_cdylib("memento-ffi", "libmemento_ffi")
    library = RustFfiLibrary(ffi_library_path)
    left_dir = tmp_path / "left"
    right_dir = tmp_path / "right"
    left_dir.mkdir()
    right_dir.mkdir()
    left_path = left_dir / "synthetic.gte"
    right_path = right_dir / "synthetic.gte"
    left_path.write_bytes(synthetic_model_bytes())
    right_path.write_bytes(synthetic_model_bytes(body_seed=9.0))

    with library.load_model(left_path) as left_model, library.load_model(right_path) as right_model:
        left_info = left_model.model_info()
        right_info = right_model.model_info()
        assert left_info.model_id == right_info.model_id == "synthetic.gte"
        assert left_info.revision != right_info.revision


def test_python_ctypes_wrapper_surfaces_vector_errors_and_sqlite_vector_matches_python_and_rust(
    tmp_path: Path,
) -> None:
    ffi_library_path = build_rust_cdylib("memento-ffi", "libmemento_ffi")
    sqlite_extension_path = build_rust_cdylib("memento-sqlite-vector", "libmemento_sqlite_vector")
    library = RustFfiLibrary(ffi_library_path)

    left = (1.0, 2.0, 3.0, 4.0)
    right = (4.0, 3.0, 2.0, 1.0)
    left_blob = pack_f32le(left)
    right_blob = pack_f32le(right)
    expected = cosine_similarity(left, right)
    assert library.vector_cosine(left, right) == pytest.approx(expected, rel=1e-6)

    connection = sqlite3.connect(tmp_path / "vector.sqlite")
    extension_target = str(sqlite_extension_path)
    if extension_target.endswith(".so"):
        extension_target = extension_target[:-3]
    connection.enable_load_extension(True)
    connection.load_extension(extension_target)
    connection.enable_load_extension(False)

    row = connection.execute(
        "SELECT vector_is_valid(?), vector_dimensions(?), vector_cosine(?, ?)",
        (left_blob, left_blob, left_blob, right_blob),
    ).fetchone()
    assert row is not None
    assert row[0] == 1
    assert row[1] == 4
    assert row[2] == pytest.approx(expected, rel=1e-6)

    malformed = b"abc"
    nan_blob = pack_f32le((1.0, float("nan")))
    with pytest.raises(sqlite3.OperationalError):
        connection.execute("SELECT vector_cosine(?, ?)", (malformed, right_blob)).fetchone()
    with pytest.raises(sqlite3.OperationalError):
        connection.execute("SELECT vector_cosine(?, ?)", (nan_blob, right_blob)).fetchone()
    with pytest.raises(sqlite3.OperationalError):
        connection.execute(
            "SELECT vector_cosine(?, ?)", (left_blob, pack_f32le((1.0, 2.0)))
        ).fetchone()

    connection.close()


def test_failed_sqlite_extension_load_disables_further_extension_loading(tmp_path: Path) -> None:
    index = DerivedIndex(
        tmp_path / "derived.sqlite",
        semantic_config=SemanticSearchConfig(
            enabled=True, sqlite_extension_path="/missing/extension"
        ),
    )
    connection = index._connect()
    try:
        with pytest.raises(sqlite3.OperationalError, match="not authorized"):
            connection.execute("SELECT load_extension('/missing/extension')").fetchone()
    finally:
        connection.close()


# Synthetic model fixture ported from the Rust FFI tests.
def synthetic_model_bytes(
    *, header_overrides: dict[int, int] | None = None, body_seed: float = 1.0
) -> bytes:
    vocab = [
        "[PAD]",
        "[unused1]",
        "[unused2]",
        "[unused3]",
        "[unused4]",
        "[unused5]",
        "[unused6]",
        "[unused7]",
        "[unused8]",
        "[unused9]",
        "[unused10]",
        "[unused11]",
        "[unused12]",
        "[unused13]",
        "[unused14]",
        "[unused15]",
        "[unused16]",
        "[unused17]",
        "[unused18]",
        "[unused19]",
        "[unused20]",
        "[unused21]",
        "[unused22]",
        "[unused23]",
        "[unused24]",
        "[unused25]",
        "[unused26]",
        "[unused27]",
        "[unused28]",
        "[unused29]",
        "[unused30]",
        "[unused31]",
        "[unused32]",
        "[unused33]",
        "[unused34]",
        "[unused35]",
        "[unused36]",
        "[unused37]",
        "[unused38]",
        "[unused39]",
        "[unused40]",
        "[unused41]",
        "[unused42]",
        "[unused43]",
        "[unused44]",
        "[unused45]",
        "[unused46]",
        "[unused47]",
        "[unused48]",
        "[unused49]",
        "[unused50]",
        "[unused51]",
        "[unused52]",
        "[unused53]",
        "[unused54]",
        "[unused55]",
        "[unused56]",
        "[unused57]",
        "[unused58]",
        "[unused59]",
        "[unused60]",
        "[unused61]",
        "[unused62]",
        "[unused63]",
        "[unused64]",
        "[unused65]",
        "[unused66]",
        "[unused67]",
        "[unused68]",
        "[unused69]",
        "[unused70]",
        "[unused71]",
        "[unused72]",
        "[unused73]",
        "[unused74]",
        "[unused75]",
        "[unused76]",
        "[unused77]",
        "[unused78]",
        "[unused79]",
        "[unused80]",
        "[unused81]",
        "[unused82]",
        "[unused83]",
        "[unused84]",
        "[unused85]",
        "[unused86]",
        "[unused87]",
        "[unused88]",
        "[unused89]",
        "[unused90]",
        "[unused91]",
        "[unused92]",
        "[unused93]",
        "[unused94]",
        "[unused95]",
        "[unused96]",
        "[unused97]",
        "[unused98]",
        "[unused99]",
        "[UNK]",
        "[CLS]",
        "[SEP]",
        "[MASK]",
        "hello",
        "world",
        ",",
        "!",
    ]
    hidden_size = 4
    max_seq_len = 8
    header = [len(vocab), hidden_size, 0, 1, hidden_size, max_seq_len]
    for index, override in (header_overrides or {}).items():
        header[index] = override
    out = bytearray(b"GTE1")
    out.extend(struct.pack("<6I", *header))
    for token in vocab:
        encoded = token.encode("utf-8")
        out.extend(struct.pack("<H", len(encoded)))
        out.extend(encoded)
    for token_id in range(len(vocab)):
        base = body_seed + float(token_id)
        out.extend(struct.pack("<4f", base, 0.0, 0.0, 0.0))
    out.extend(
        struct.pack(f"<{max_seq_len * hidden_size}f", *([0.0] * (max_seq_len * hidden_size)))
    )
    out.extend(struct.pack(f"<{2 * hidden_size}f", *([0.0] * (2 * hidden_size))))
    out.extend(struct.pack(f"<{hidden_size}f", *([1.0] * hidden_size)))
    out.extend(struct.pack(f"<{hidden_size}f", *([0.0] * hidden_size)))
    out.extend(
        struct.pack(f"<{hidden_size * hidden_size}f", *([0.0] * (hidden_size * hidden_size)))
    )
    out.extend(struct.pack(f"<{hidden_size}f", *([0.0] * hidden_size)))
    return bytes(out)


def build_rust_cdylib(package: str, stem: str) -> Path:
    project_root = Path(__file__).resolve().parents[1]
    rust_dir = project_root / "rust"
    target_dir = rust_dir / "target" / "debug"
    suffix = ".dylib" if sys.platform == "darwin" else ".dll" if sys.platform == "win32" else ".so"
    library_path = target_dir / f"{stem}{suffix}"
    subprocess.run(
        ["cargo", "build", "-p", package],
        cwd=rust_dir,
        check=True,
    )
    return library_path


def _patch_break_semantic(worktree: Path) -> tuple[str, ...]:
    write_concept(
        worktree / "projects" / "beta.md",
        concept_id="beta-id",
        concept_type="project",
        title="Beta",
        description="Visible beta",
        tags=("shared",),
        body="# Beta\n\nbreaksemantic common red visible\n",
    )
    return ("/projects/beta.md",)


def write_concept(
    path: Path,
    *,
    concept_id: str,
    concept_type: str,
    title: str,
    description: str,
    tags: tuple[str, ...],
    body: str,
) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    document = ConceptDocument(
        frontmatter=ConceptFrontmatter(
            schema_version=1,
            id=concept_id,
            type=concept_type,
            title=title,
            description=description,
            tags=tags,
            aliases=(),
            source_refs=(),
            supersedes=(),
            status=ConceptStatus.ACTIVE,
            created_at=datetime(2026, 7, 17, 12, 0, tzinfo=timezone.utc),
            updated_at=datetime(2026, 7, 17, 12, 0, tzinfo=timezone.utc),
            updated_by="rui/tests",
        ),
        body=body,
    )
    path.write_text(serialize_concept(document), encoding="utf-8")
