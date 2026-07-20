from __future__ import annotations

import sqlite3
import threading
import time
from collections.abc import Generator
from datetime import UTC, datetime
from pathlib import Path

import pytest

from memento.authz import EffectivePolicy
from memento.control.db import connect_control_db, migrate_control_db
from memento.control.operations import OperationRequest
from memento.derived.index import DerivedIndex, DerivedIndexCorruptionError, SearchFreshness
from memento.repository.frontmatter import serialize_concept
from memento.repository.git import GitRepositoryPaths, bootstrap_repository, get_main_revision
from memento.repository.schema import ConceptDocument, ConceptFrontmatter, ConceptStatus
from memento.repository.transactions import TransactionManager, TransactionRequest


@pytest.fixture()
def control_connection(tmp_path: Path) -> Generator[sqlite3.Connection, None, None]:
    connection = connect_control_db(tmp_path / "control.sqlite")
    migrate_control_db(connection)
    yield connection
    connection.close()


@pytest.fixture()
def repo_paths(tmp_path: Path) -> GitRepositoryPaths:
    seed = tmp_path / "seed"
    write_concept(
        seed / "instances" / "smith.md",
        concept_id="smith-id",
        concept_type="instance",
        title="Smith",
        description="Primary instance for ranked visible search.",
        tags=("alpha", "visible"),
        body="# Smith\n\nSee [Piclaw](/projects/piclaw.md) and [Missing](/projects/missing.md).\n",
    )
    write_concept(
        seed / "projects" / "piclaw.md",
        concept_id="piclaw-id",
        concept_type="project",
        title="Piclaw",
        description="Shared project.",
        tags=("project",),
        body="# Piclaw\n\nSee [Smith](/instances/smith.md).\n",
    )
    write_concept(
        seed / "projects" / "orphan.md",
        concept_id="orphan-id",
        concept_type="project",
        title="Orphan",
        description="Disconnected concept.",
        tags=("orphan",),
        body="# Orphan\n",
    )
    write_concept(
        seed / "secret" / "ghost.md",
        concept_id="ghost-id",
        concept_type="project",
        title="Ghost",
        description="Invisible ghost keeps visible terms hidden from other readers.",
        tags=("hidden",),
        body="# Ghost\n\nvisible visible visible hidden\n",
    )
    paths = GitRepositoryPaths(
        bare_dir=tmp_path / "repo.git",
        current_dir=tmp_path / "current",
        worktrees_dir=tmp_path / "worktrees",
    )
    bootstrap_repository(paths, seed)
    return paths


@pytest.fixture()
def policy() -> EffectivePolicy:
    return EffectivePolicy(
        principal="smith",
        roles=("reader",),
        read_prefixes=("/instances/", "/projects/"),
        write_prefixes=("/instances/", "/projects/"),
    )


@pytest.fixture()
def hidden_policy() -> EffectivePolicy:
    return EffectivePolicy(
        principal="admin",
        roles=("reader",),
        read_prefixes=("/instances/", "/projects/", "/secret/"),
        write_prefixes=("/instances/", "/projects/", "/secret/"),
    )


@pytest.fixture()
def derived_index(tmp_path: Path, repo_paths: GitRepositoryPaths) -> DerivedIndex:
    index = DerivedIndex(tmp_path / "derived.sqlite")
    index.rebuild(repo_paths.current_dir, repo_revision=get_main_revision(repo_paths))
    return index


def test_status_snapshot_counts_only_authorized_concepts(
    derived_index: DerivedIndex,
    policy: EffectivePolicy,
    hidden_policy: EffectivePolicy,
) -> None:
    visible = derived_index.status_snapshot(policy)
    all_concepts = derived_index.status_snapshot(hidden_policy)
    assert visible.state == all_concepts.state
    assert visible.visible_concepts == 3
    assert all_concepts.visible_concepts == 4


def test_concurrent_first_open_initializes_schema_once(tmp_path: Path) -> None:
    index = DerivedIndex(tmp_path / "concurrent.sqlite")
    states: list[str] = []
    errors: list[BaseException] = []

    def read_state() -> None:
        try:
            states.append(index.get_state().status)
        except BaseException as exc:  # pragma: no cover - surfaced by assertion
            errors.append(exc)

    threads = [threading.Thread(target=read_state) for _ in range(8)]
    for thread in threads:
        thread.start()
    for thread in threads:
        thread.join()
    assert errors == []
    assert states == ["ready"] * 8


def test_full_rebuild_search_filters_and_hidden_namespace_do_not_leak_ranking(
    derived_index: DerivedIndex,
    repo_paths: GitRepositoryPaths,
    policy: EffectivePolicy,
    hidden_policy: EffectivePolicy,
) -> None:
    visible = derived_index.search(policy=policy, query="visible", limit=10)
    visible_paths = [result.path for result in visible.results]
    assert "/instances/smith.md" in visible_paths
    assert "/secret/ghost.md" not in visible_paths

    typed = derived_index.search(policy=policy, query="Piclaw", concept_type="project")
    assert [result.path for result in typed.results] == ["/projects/piclaw.md"]

    tagged = derived_index.search(policy=policy, query="concept", tags=("orphan",))
    assert [result.path for result in tagged.results] == ["/projects/orphan.md"]

    prefixed = derived_index.search(policy=policy, query="visible", path_prefix="/instances/")
    assert [result.path for result in prefixed.results] == ["/instances/smith.md"]

    hidden = derived_index.search(policy=hidden_policy, query="visible", limit=10)
    assert "/secret/ghost.md" in [result.path for result in hidden.results]
    assert [result.path for result in visible.results][0] == "/instances/smith.md"
    assert visible.next_cursor is None
    assert visible.repo_revision == get_main_revision(repo_paths)
    assert visible.index_revision == get_main_revision(repo_paths)


def test_graph_metrics_and_bounded_neighborhood(
    derived_index: DerivedIndex,
    policy: EffectivePolicy,
) -> None:
    graph = derived_index.graph(policy=policy, concept_id="smith-id", depth=2)
    assert [edge.path for edge in graph.outbound] == ["/projects/piclaw.md"]
    assert [edge.path for edge in graph.inbound] == ["/projects/piclaw.md"]
    assert graph.broken_targets == ("/projects/missing.md",)

    smith_metrics = derived_index.metrics("smith-id")
    assert smith_metrics.inbound_degree == 1
    assert smith_metrics.outbound_degree == 2
    assert smith_metrics.broken_link_count == 1
    assert smith_metrics.orphan_flag is False

    orphan_metrics = derived_index.metrics("orphan-id")
    assert orphan_metrics.orphan_flag is True


def test_incremental_update_ignores_non_markdown_artifacts(
    tmp_path: Path, derived_index: DerivedIndex
) -> None:
    artifact = tmp_path / "skills" / ".versions" / "demo" / "1.0.0.zip"
    artifact.parent.mkdir(parents=True)
    artifact.write_bytes(b"PK-not-a-concept")

    derived_index.rebuild(tmp_path, repo_revision="rev-empty")
    derived_index.update_paths(
        tmp_path,
        repo_revision="rev-artifact",
        changed_paths=("/skills/.versions/demo/1.0.0.zip", "/.gitattributes"),
    )

    state = derived_index.get_state()
    assert state.repo_revision == "rev-artifact"
    assert state.index_revision == "rev-artifact"


def test_incremental_update_matches_clean_rebuild_and_supports_delete_then_rebuild(
    tmp_path: Path,
    repo_paths: GitRepositoryPaths,
    derived_index: DerivedIndex,
) -> None:
    target = repo_paths.current_dir / "projects" / "piclaw.md"
    write_concept(
        target,
        concept_id="piclaw-id",
        concept_type="project",
        title="Piclaw",
        description="Updated project.",
        tags=("project", "updated"),
        body="# Piclaw\n\nSee [Smith](/instances/smith.md).\n\nUpdated body.\n",
    )
    orphan = repo_paths.current_dir / "projects" / "orphan.md"
    orphan.unlink()
    write_concept(
        repo_paths.current_dir / "projects" / "new.md",
        concept_id="new-id",
        concept_type="project",
        title="New",
        description="Brand new concept.",
        tags=("new",),
        body="# New\n",
    )

    derived_index.update_paths(
        repo_paths.current_dir,
        repo_revision="rev-2",
        changed_paths=("/projects/piclaw.md", "/projects/orphan.md", "/projects/new.md"),
    )
    parity = derived_index.parity_check(repo_paths.current_dir, repo_revision="rev-2")
    assert parity.matches is True

    db_path = derived_index.db_path
    db_path.unlink()
    derived_index.rebuild(repo_paths.current_dir, repo_revision="rev-2")
    rebuilt = derived_index.search(policy=policy_fixture(), query="Updated", limit=10)
    assert [result.path for result in rebuilt.results] == ["/projects/piclaw.md"]


def test_strict_freshness_waits_until_index_matches_repo_revision(
    derived_index: DerivedIndex,
) -> None:
    derived_index.set_repo_revision("rev-fresh")
    results: list[str] = []

    def delayed_update() -> None:
        time.sleep(0.1)
        derived_index.update_paths(Path("."), repo_revision="rev-fresh", changed_paths=())
        results.append("done")

    thread = threading.Thread(target=delayed_update)
    thread.start()
    state = derived_index.wait_for_freshness(timeout_seconds=1.0)
    thread.join()

    assert state.index_revision == "rev-fresh"
    assert results == ["done"]


def test_fts_syntax_error_returns_validation_error_without_quarantine(
    tmp_path: Path,
    repo_paths: GitRepositoryPaths,
    policy: EffectivePolicy,
) -> None:
    index = DerivedIndex(tmp_path / "syntax.sqlite")
    index.rebuild(repo_paths.current_dir, repo_revision=get_main_revision(repo_paths))

    with pytest.raises(ValueError, match="invalid FTS query"):
        index.search(policy=policy, query='"unterminated', query_syntax="fts5")

    plain = index.search(policy=policy, query="visible---", query_syntax="plain")
    assert [result.path for result in plain.results] == ["/instances/smith.md"]
    with pytest.raises(ValueError, match="unsupported query_syntax"):
        index.search(policy=policy, query="visible", query_syntax="unknown")
    with pytest.raises(ValueError, match="at least one word"):
        index.search(policy=policy, query="---", query_syntax="plain")

    state = index.get_state()
    assert state.status == "ready"
    assert state.quarantine_path is None
    assert [result.path for result in index.search(policy=policy, query="visible").results] == [
        "/instances/smith.md"
    ]


def test_corruption_is_quarantined_and_rebuild_recovers(
    tmp_path: Path,
    repo_paths: GitRepositoryPaths,
    policy: EffectivePolicy,
) -> None:
    index = DerivedIndex(tmp_path / "corrupt.sqlite")
    index.rebuild(repo_paths.current_dir, repo_revision=get_main_revision(repo_paths))
    for extra in (
        index.db_path,
        index.db_path.with_name(index.db_path.name + "-wal"),
        index.db_path.with_name(index.db_path.name + "-shm"),
    ):
        if extra.exists():
            extra.unlink()
    index.db_path.write_bytes(b"not sqlite")

    with pytest.raises(DerivedIndexCorruptionError):
        index.search(policy=policy, query="visible")

    quarantines = sorted(index.db_path.parent.glob("corrupt.quarantine-*.sqlite"))
    assert quarantines

    index.rebuild(repo_paths.current_dir, repo_revision=get_main_revision(repo_paths))
    assert [result.path for result in index.search(policy=policy, query="visible").results] == [
        "/instances/smith.md"
    ]


def test_transaction_apply_updates_derived_index_before_success(
    tmp_path: Path,
    control_connection: sqlite3.Connection,
    repo_paths: GitRepositoryPaths,
    policy: EffectivePolicy,
) -> None:
    derived_index = DerivedIndex(tmp_path / "tx-derived.sqlite")
    derived_index.rebuild(repo_paths.current_dir, repo_revision=get_main_revision(repo_paths))

    def apply_update(
        materialized_root: Path, repo_revision: str, changed_paths: tuple[str, ...]
    ) -> None:
        if changed_paths:
            derived_index.update_paths(
                materialized_root, repo_revision=repo_revision, changed_paths=changed_paths
            )
        else:
            derived_index.rebuild(materialized_root, repo_revision=repo_revision)

    manager = TransactionManager(control_connection, repo_paths, derived_update=apply_update)
    result = manager.apply(
        TransactionRequest(
            operation=OperationRequest(
                op_id="op-derived",
                principal="smith",
                idempotency_key="idem-derived",
                tool_name="memory_patch",
                request_json='{"path":"/projects/piclaw.md"}',
            ),
            expected_revision=get_main_revision(repo_paths),
            commit_message="memory: update piclaw",
            author_name="Rui Carmo",
            author_email="rui.carmo@gmail.com",
        ),
        lambda worktree: _update_project(worktree),
    )

    page = derived_index.search(
        policy=policy,
        query="transaction visible",
        freshness=SearchFreshness.STRICT,
        timeout_seconds=0.5,
    )
    assert page.index_revision == result.result_revision
    assert [item.path for item in page.results] == ["/projects/piclaw.md"]


def _update_project(worktree: Path) -> tuple[str, ...]:
    write_concept(
        worktree / "projects" / "piclaw.md",
        concept_id="piclaw-id",
        concept_type="project",
        title="Piclaw",
        description="Transaction visible update.",
        tags=("project", "updated"),
        body="# Piclaw\n\ntransaction visible\n",
    )
    return ("/projects/piclaw.md",)


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
            created_at=datetime(2026, 7, 17, 12, 0, tzinfo=UTC),
            updated_at=datetime(2026, 7, 17, 12, 0, tzinfo=UTC),
            updated_by="rui/tests",
        ),
        body=body,
    )
    path.write_text(serialize_concept(document), encoding="utf-8")


def policy_fixture() -> EffectivePolicy:
    return EffectivePolicy(
        principal="smith",
        roles=("reader",),
        read_prefixes=("/instances/", "/projects/"),
        write_prefixes=("/instances/", "/projects/"),
    )
