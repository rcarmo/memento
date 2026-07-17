from __future__ import annotations

import sqlite3
from collections.abc import Generator
from datetime import datetime, timedelta, timezone
from pathlib import Path
from typing import Any, cast

import pytest

from memento.config import (
    AuthorizationConfig,
    NamespacePolicy,
    Principal,
    RepositoryConfig,
    ServiceConfig,
)
from memento.control.db import connect_control_db, migrate_control_db
from memento.control.proposals import ProposalStatus, update_proposal_status
from memento.derived.index import DerivedIndex
from memento.repository.frontmatter import serialize_concept
from memento.repository.git import GitRepositoryPaths, bootstrap_repository, get_main_revision
from memento.repository.schema import ConceptDocument, ConceptFrontmatter, ConceptStatus
from memento.repository.transactions import TransactionManager
from memento.service import MemoryService, ServiceContext, ServiceDependencies


def success_data(result: object) -> dict[str, Any]:
    payload = cast(Any, result)
    assert payload.status == "success"
    return cast(dict[str, Any], payload.data)


@pytest.fixture()
def service_config(tmp_path: Path) -> ServiceConfig:
    return ServiceConfig(
        schema_version=1,
        repository=RepositoryConfig(root_path=str(tmp_path / "state")),
        authorization=AuthorizationConfig(
            principals={
                "smith": NamespacePolicy(
                    roles=("reader", "proposer", "curator"),
                    read_prefixes=("/instances/", "/projects/"),
                    write_prefixes=("/instances/", "/projects/"),
                ),
                "flint": NamespacePolicy(
                    roles=("reader", "proposer"),
                    read_prefixes=("/instances/", "/projects/"),
                    write_prefixes=("/projects/",),
                ),
                "ghost": NamespacePolicy(
                    roles=("reader",),
                    read_prefixes=("/secret/",),
                    write_prefixes=(),
                ),
            }
        ),
    )


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
        description="Visible instance.",
        tags=("visible",),
        body="# Smith\n\nSee [Piclaw](/projects/piclaw.md).\n",
    )
    write_concept(
        seed / "projects" / "piclaw.md",
        concept_id="piclaw-id",
        concept_type="project",
        title="Piclaw",
        description="Visible project.",
        tags=("shared",),
        body="# Piclaw\n\nSee [Smith](/instances/smith.md).\n",
    )
    write_concept(
        seed / "secret" / "ghost.md",
        concept_id="ghost-id",
        concept_type="project",
        title="Ghost",
        description="Hidden project.",
        tags=("hidden",),
        body="# Ghost\n",
    )
    paths = GitRepositoryPaths(
        bare_dir=tmp_path / "repo.git",
        current_dir=tmp_path / "current",
        worktrees_dir=tmp_path / "worktrees",
    )
    bootstrap_repository(paths, seed)
    return paths


@pytest.fixture()
def service(
    tmp_path: Path,
    service_config: ServiceConfig,
    control_connection: sqlite3.Connection,
    repo_paths: GitRepositoryPaths,
) -> MemoryService:
    derived_index = DerivedIndex(tmp_path / "derived.sqlite")
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
    return MemoryService(
        ServiceDependencies(
            config=service_config,
            repo_paths=repo_paths,
            control_connection=control_connection,
            derived_index=derived_index,
            transaction_manager=manager,
        )
    )


@pytest.fixture()
def smith() -> ServiceContext:
    return ServiceContext(Principal(name="smith", roles=("reader", "proposer", "curator")))


@pytest.fixture()
def flint() -> ServiceContext:
    return ServiceContext(Principal(name="flint", roles=("reader", "proposer")))


@pytest.fixture()
def ghost() -> ServiceContext:
    return ServiceContext(Principal(name="ghost", roles=("reader",)))


def test_auth_visibility_and_standard_envelopes(
    service: MemoryService, flint: ServiceContext, ghost: ServiceContext
) -> None:
    search = service.memory_search(flint, query="Ghost")
    assert search.status == "success"
    assert success_data(search)["results"] == []
    assert search.repo_revision == search.index_revision

    read_hidden = service.memory_read(flint, id_or_path="/secret/ghost.md")
    assert read_hidden.status == "error"
    assert read_hidden.error_class == "forbidden"

    hidden_visible = service.memory_read(ghost, id_or_path="/secret/ghost.md")
    assert hidden_visible.status == "success"
    assert success_data(hidden_visible)["frontmatter"]["title"] == "Ghost"


def test_proposal_lifecycle_self_approval_stale_apply_and_idempotency(
    service: MemoryService,
    control_connection: sqlite3.Connection,
    repo_paths: GitRepositoryPaths,
    smith: ServiceContext,
    flint: ServiceContext,
) -> None:
    base_revision = get_main_revision(repo_paths)
    proposed = service.memory_propose(
        flint,
        intent="Update Piclaw",
        base_revision=base_revision,
        changes=[
            {
                "kind": "patch",
                "path": "/projects/piclaw.md",
                "body": "# Piclaw\n\nUpdated by proposal.\n",
            }
        ],
        rationale="Need fresher summary.",
    )
    assert proposed.status == "success"
    proposed_data = success_data(proposed)
    proposal_id = proposed_data["proposal"]["proposal_id"]
    assert "Updated by proposal" in proposed_data["proposal"]["diff"]

    self_approve = service.memory_proposal_review(
        flint, proposal_id=proposal_id, decision="approve"
    )
    assert self_approve.status == "error"
    assert self_approve.error_class == "forbidden"

    approved = service.memory_proposal_review(
        smith, proposal_id=proposal_id, decision="approve", comment="ok"
    )
    assert approved.status == "success"
    assert success_data(approved)["proposal"]["status"] == "approved"

    applied = service.memory_proposal_apply(
        smith,
        proposal_id=proposal_id,
        expected_revision=base_revision,
        idempotency_key="apply-proposal-1",
    )
    assert applied.status == "success"
    applied_data = success_data(applied)
    assert applied_data["proposal"]["status"] == "applied"
    assert applied_data["replayed"] is False

    replay = service.memory_proposal_apply(
        smith,
        proposal_id=proposal_id,
        expected_revision=base_revision,
        idempotency_key="apply-proposal-1",
    )
    assert replay.status == "error"
    assert replay.error_class == "conflict"

    stale_proposal = service.memory_propose(
        flint,
        intent="Stale patch",
        base_revision=base_revision,
        changes=[{"kind": "patch", "path": "/projects/piclaw.md", "title": "Piclaw stale"}],
    )
    stale_id = success_data(stale_proposal)["proposal"]["proposal_id"]
    stale_status = service.memory_proposal_get(flint, proposal_id=stale_id)
    assert stale_status.status == "success"
    assert success_data(stale_status)["proposal"]["status"] == "stale"

    mismatched = service.memory_create(
        smith,
        path="/projects/new.md",
        concept_type="project",
        title="New",
        body="# New\n",
        expected_revision=get_main_revision(repo_paths),
        idempotency_key="create-1",
    )
    assert mismatched.status == "success"
    idempotency_conflict = service.memory_create(
        smith,
        path="/projects/other.md",
        concept_type="project",
        title="Other",
        body="# Other\n",
        expected_revision=get_main_revision(repo_paths),
        idempotency_key="create-1",
    )
    assert idempotency_conflict.status == "error"
    assert idempotency_conflict.error_class == "idempotency_conflict"


def test_direct_rename_rewrites_inbound_links_atomically(
    service: MemoryService,
    repo_paths: GitRepositoryPaths,
    smith: ServiceContext,
) -> None:
    revision = get_main_revision(repo_paths)
    renamed = service.memory_rename(
        smith,
        path="/projects/piclaw.md",
        new_path="/projects/shared-piclaw.md",
        expected_revision=revision,
        idempotency_key="rename-1",
    )
    assert renamed.status == "success"
    renamed_data = success_data(renamed)
    assert "/instances/smith.md" in renamed_data["changed_paths"]
    assert "/projects/shared-piclaw.md" in renamed_data["changed_paths"]
    updated = (repo_paths.current_dir / "instances" / "smith.md").read_text(encoding="utf-8")
    assert "/projects/shared-piclaw.md" in updated
    assert "/projects/piclaw.md" not in updated


def test_proposal_list_visibility_and_expiry(
    service: MemoryService,
    control_connection: sqlite3.Connection,
    repo_paths: GitRepositoryPaths,
    smith: ServiceContext,
    flint: ServiceContext,
) -> None:
    proposal = service.memory_propose(
        flint,
        intent="Visible only to author or curator",
        base_revision=get_main_revision(repo_paths),
        changes=[{"kind": "patch", "path": "/projects/piclaw.md", "description": "desc"}],
    )
    proposal_id = success_data(proposal)["proposal"]["proposal_id"]
    expired = update_proposal_status(
        control_connection,
        proposal_id,
        status=ProposalStatus.SUBMITTED,
    )
    control_connection.execute(
        "UPDATE proposals SET expires_at = ? WHERE proposal_id = ?",
        (
            (datetime.now(tz=timezone.utc) - timedelta(days=1))
            .replace(microsecond=0)
            .isoformat()
            .replace("+00:00", "Z"),
            proposal_id,
        ),
    )
    control_connection.commit()
    assert expired.proposal_id == proposal_id

    author_visible = service.memory_proposal_list(flint)
    author_visible_data = success_data(author_visible)
    assert len(author_visible_data["proposals"]) == 1
    assert author_visible_data["proposals"][0]["status"] == "expired"

    curator_visible = service.memory_proposal_list(smith)
    assert len(success_data(curator_visible)["proposals"]) == 1


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
