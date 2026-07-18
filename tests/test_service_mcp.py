from __future__ import annotations

import asyncio
import base64
import io
import json
import sqlite3
import zipfile
from collections.abc import Generator
from datetime import UTC, datetime, timedelta
from pathlib import Path
from typing import Any, Literal, cast

import pytest

from memento.config import (
    AuthorizationConfig,
    MCPConfig,
    MCPExecuteLimitsConfig,
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
from memento.server import MementoMCPServer
from memento.service import MemoryService, ServiceContext, ServiceDependencies


class FakeNeedleRouter:
    def __init__(self, output: str) -> None:
        self.output = output
        self.calls: list[tuple[str, str]] = []
        self.closed = False

    def generate(self, query: str, tools_json: str, **_: Any) -> str:
        self.calls.append((query, tools_json))
        return self.output

    def close(self) -> None:
        self.closed = True


def success_data(result: object) -> dict[str, Any]:
    payload = cast(Any, result)
    assert payload.status == "success"
    return cast(dict[str, Any], payload.data)


@pytest.fixture()
def service_config(tmp_path: Path) -> ServiceConfig:
    return ServiceConfig(
        schema_version=2,
        repository=RepositoryConfig(root_path=str(tmp_path / "state")),
        authorization=AuthorizationConfig(
            principals={
                "smith": NamespacePolicy(
                    roles=("reader", "proposer", "curator"),
                    token_env="MEMENTO_TOKEN_SMITH",
                    read_prefixes=("/instances/", "/projects/", "/skills/"),
                    write_prefixes=("/instances/", "/projects/", "/skills/"),
                ),
                "flint": NamespacePolicy(
                    roles=("reader", "proposer"),
                    token_env="MEMENTO_TOKEN_FLINT",
                    read_prefixes=("/instances/", "/projects/", "/skills/"),
                    write_prefixes=("/projects/", "/skills/"),
                ),
                "ghost": NamespacePolicy(
                    roles=("reader",),
                    token_env="MEMENTO_TOKEN_GHOST",
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
    assert replay.status == "success"
    replay_data = success_data(replay)
    assert replay_data["replayed"] is True
    assert replay_data["proposal"]["status"] == "applied"

    same_key_different_payload = service.memory_proposal_apply(
        smith,
        proposal_id=proposal_id,
        expected_revision="different-revision",
        idempotency_key="apply-proposal-1",
    )
    assert same_key_different_payload.status == "error"
    assert same_key_different_payload.error_class == "idempotency_conflict"

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


def _server_for(
    service: MemoryService, config: ServiceConfig, *, needle_router: FakeNeedleRouter | None = None
) -> MementoMCPServer:
    tokens = {"smith-token": Principal(name="smith", roles=("curator", "proposer", "reader"))}
    variant_service = MemoryService(
        ServiceDependencies(
            config=config,
            repo_paths=service._deps.repo_paths,
            control_connection=service._deps.control_connection,
            derived_index=service._deps.derived_index,
            transaction_manager=service._deps.transaction_manager,
            model_client=service._deps.model_client,
            needle_router=needle_router or service._deps.needle_router,
        )
    )
    return MementoMCPServer(variant_service, bearer_tokens=tokens)


def test_tool_discovery_surfaces_and_catalog_resources(
    service: MemoryService,
    service_config: ServiceConfig,
    smith: ServiceContext,
) -> None:
    expected_counts: tuple[
        tuple[Literal["compact", "standard", "read_only", "curator", "admin"], int], ...
    ] = (
        ("compact", 5),
        ("standard", 26),
        ("read_only", 10),
        ("curator", 16),
        ("admin", 27),
    )
    for surface, count in expected_counts:
        server = _server_for(
            service,
            service_config.model_copy(update={"mcp": MCPConfig(tool_surface=surface)}),
        )
        tools = server.discover_tools()["tools"]
        assert len(tools) == count
    server = _server_for(service, service_config)
    catalog = json.loads(asyncio.run(server.resource_catalog())["text"])
    assert "operations" in catalog
    assert [item["operation"] for item in catalog["operations"]] == [
        "help",
        "status",
        "search",
        "read",
        "execute",
    ]
    assert any(item["operation"] == "proposal_apply" for item in catalog["execute_only_operations"])
    operation = json.loads(asyncio.run(server.resource_template_catalog("propose"))["text"])
    assert operation["tool"] == "memory_propose"
    assert operation["direct_tool_available"] is False
    assert operation["available_via_execute"] is True
    changes_schema = operation["input_schema"]["properties"]["changes"]
    assert changes_schema["type"] == "array"
    assert "anyOf" in changes_schema["items"]
    help_payload = success_data(service.memory_help(smith))
    assert "memory_execute" in help_payload["mcp"]["direct_tools"]
    assert help_payload["mcp"]["execute_only_operations"]["propose"] == (
        "propose",
        "propose_freeform",
        "propose_update",
    )
    workflow = json.loads(asyncio.run(server.resource_template_workflow("inspect"))["text"])
    assert [item["operation"] for item in workflow["operations"]] == ["search", "read"]
    propose_workflow = json.loads(asyncio.run(server.resource_template_workflow("propose"))["text"])
    assert [item["operation"] for item in propose_workflow["operations"]] == ["search", "read"]
    assert [item["operation"] for item in propose_workflow["execute_only_operations"]] == [
        "propose",
        "propose_freeform",
        "propose_update",
    ]


def test_skill_pack_tool_discovery_and_catalog_schemas(
    service: MemoryService,
    service_config: ServiceConfig,
) -> None:
    standard_server = _server_for(
        service,
        service_config.model_copy(update={"mcp": MCPConfig(tool_surface="standard")}),
    )
    standard_tools = {item["name"]: item for item in standard_server.discover_tools()["tools"]}
    assert "memory_skill_search" in standard_tools
    assert "memory_skill_get" in standard_tools
    assert "memory_skill_propose" in standard_tools
    assert "memory_skill_prune" in standard_tools
    assert "memory_execute" not in standard_tools
    assert standard_tools["memory_skill_search"]["annotations"] == {
        "roles": ["reader"],
        "operation": "skill_search",
    }
    assert (
        standard_tools["memory_skill_propose"]["inputSchema"]["properties"]["zip_base64"]["type"]
        == "string"
    )
    assert set(standard_tools["memory_skill_prune"]["inputSchema"]["required"]) == {
        "skill_name",
        "expected_revision",
        "idempotency_key",
    }

    read_only_server = _server_for(
        service,
        service_config.model_copy(update={"mcp": MCPConfig(tool_surface="read_only")}),
    )
    read_only_tools = {item["name"] for item in read_only_server.discover_tools()["tools"]}
    assert "memory_skill_search" in read_only_tools
    assert "memory_skill_get" in read_only_tools
    assert "memory_skill_propose" not in read_only_tools
    assert "memory_skill_prune" not in read_only_tools

    catalog = json.loads(asyncio.run(standard_server.resource_catalog())["text"])
    skill_ops = {item["operation"]: item for item in catalog["operations"]}
    assert skill_ops["skill_search"]["tool"] == "memory_skill_search"
    assert skill_ops["skill_search"]["commit_capable"] is False
    assert skill_ops["skill_proposal_apply"]["commit_capable"] is True
    assert skill_ops["skill_prune"]["commit_capable"] is True

    skill_pack_workflow = json.loads(
        asyncio.run(standard_server.resource_template_workflow("skill_pack"))["text"]
    )
    assert [item["operation"] for item in skill_pack_workflow["operations"]] == [
        "skill_search",
        "skill_get",
        "skill_propose",
        "skill_proposal_list",
        "skill_proposal_get",
        "skill_proposal_review",
        "skill_proposal_apply",
        "skill_prune",
    ]


def _skill_zip(skill_md: str, script: str = "console.log('ok')\n") -> tuple[str, bytes]:
    stream = io.BytesIO()
    with zipfile.ZipFile(stream, "w", compression=zipfile.ZIP_DEFLATED) as archive:
        archive.writestr("SKILL.md", skill_md)
        archive.writestr("scripts/run.ts", script)
    data = stream.getvalue()
    return base64.b64encode(data).decode("ascii"), data


def test_skill_pack_propose_review_apply_search_and_recall(
    service: MemoryService,
    repo_paths: GitRepositoryPaths,
    smith: ServiceContext,
    flint: ServiceContext,
) -> None:
    skill_md = "---\nname: demo-skill\ndescription: Demo\n---\n# Demo Skill\n"
    encoded, zip_bytes = _skill_zip(skill_md)
    proposed = service.memory_skill_propose(
        flint,
        skill_name="demo-skill",
        version="1.0.0",
        skill_md=skill_md,
        zip_base64=encoded,
        rationale="share complete skill",
    )
    proposal = success_data(proposed)["proposal"]
    assert proposal["status"] == "submitted"
    assert "zip_base64" not in proposal

    self_review = service.memory_skill_proposal_review(
        flint, proposal_id=proposal["proposal_id"], decision="approve"
    )
    assert self_review.status == "error"
    assert self_review.error_class == "forbidden"

    approved = service.memory_skill_proposal_review(
        smith,
        proposal_id=proposal["proposal_id"],
        decision="approve",
        comment="validated",
    )
    assert success_data(approved)["proposal"]["status"] == "approved"

    revision = get_main_revision(repo_paths)
    applied = service.memory_skill_proposal_apply(
        smith,
        proposal_id=proposal["proposal_id"],
        expected_revision=revision,
        idempotency_key="skill-demo-1",
    )
    assert applied.status == "success", applied.model_dump(mode="python")
    applied_data = success_data(applied)
    assert applied_data["proposal"]["status"] == "applied"
    assert "/skills/.versions/demo-skill/1.0.0.zip" in applied_data["changed_paths"]

    searched = success_data(service.memory_skill_search(flint, query="Demo Skill"))
    assert [(item["skill_name"], item["version"]) for item in searched["results"]] == [
        ("demo-skill", "1.0.0")
    ]
    recalled = success_data(service.memory_skill_get(flint, skill_name="demo-skill"))
    assert recalled["version"] == "1.0.0"
    assert recalled["versions"] == ["1.0.0"]
    assert base64.b64decode(recalled["zip_base64"]) == zip_bytes

    duplicate = service.memory_skill_propose(
        flint,
        skill_name="demo-skill",
        version="1.0.0",
        skill_md=skill_md,
        zip_base64=encoded,
    )
    assert duplicate.status == "error"
    assert duplicate.error_class == "conflict"


def test_memory_route_direct_execute_unknown_auth_and_malformed(
    service: MemoryService,
    service_config: ServiceConfig,
    smith: ServiceContext,
    flint: ServiceContext,
) -> None:
    enabled = service_config.model_copy(
        update={
            "intelligent_tiers": service_config.intelligent_tiers.model_copy(
                update={
                    "needle_router": service_config.intelligent_tiers.needle_router.model_copy(
                        update={"enabled": True}
                    )
                }
            )
        }
    )

    search_router = FakeNeedleRouter(
        '[{"name":"search_paths","arguments":{"query":"Piclaw","limit":1}}]'
    )
    routed_service = MemoryService(
        ServiceDependencies(
            config=enabled,
            repo_paths=service._deps.repo_paths,
            control_connection=service._deps.control_connection,
            derived_index=service._deps.derived_index,
            transaction_manager=service._deps.transaction_manager,
            model_client=service._deps.model_client,
            needle_router=search_router,
        )
    )
    result = routed_service.memory_route(smith, request="Find Piclaw")
    assert result.status == "success"
    data = success_data(result)
    assert data["executed"] is True
    assert data["action"]["action"] == "search_paths"
    assert data["result"]["status"] == "success"
    assert data["result"]["data"]["value"] == [{"path": "/projects/piclaw.md"}]
    assert search_router.calls[0][0] == "Find Piclaw"

    plan_router = FakeNeedleRouter('[{"name":"search_then_read","arguments":{"query":"Piclaw"}}]')
    plan_service = MemoryService(
        ServiceDependencies(
            config=enabled,
            repo_paths=service._deps.repo_paths,
            control_connection=service._deps.control_connection,
            derived_index=service._deps.derived_index,
            transaction_manager=service._deps.transaction_manager,
            model_client=service._deps.model_client,
            needle_router=plan_router,
        )
    )
    preview = plan_service.memory_route(smith, request="show Piclaw", execute=False)
    assert preview.status == "success"
    preview_data = success_data(preview)
    assert preview_data["executed"] is False
    assert preview_data["expansion"]["tool"] == "memory_execute"
    executed = plan_service.memory_route(smith, request="show Piclaw")
    assert executed.status == "success"
    assert success_data(executed)["result"]["status"] == "success"

    unknown_service = MemoryService(
        ServiceDependencies(
            config=enabled,
            repo_paths=service._deps.repo_paths,
            control_connection=service._deps.control_connection,
            derived_index=service._deps.derived_index,
            transaction_manager=service._deps.transaction_manager,
            model_client=service._deps.model_client,
            needle_router=FakeNeedleRouter('[{"name":"UNKNOWN","arguments":{}}]'),
        )
    )
    unknown = unknown_service.memory_route(smith, request="book a flight")
    assert unknown.status == "success"
    unknown_data = success_data(unknown)
    assert unknown_data["abstained"] is True
    assert unknown_data["executed"] is False

    read_service = MemoryService(
        ServiceDependencies(
            config=enabled,
            repo_paths=service._deps.repo_paths,
            control_connection=service._deps.control_connection,
            derived_index=service._deps.derived_index,
            transaction_manager=service._deps.transaction_manager,
            model_client=service._deps.model_client,
            needle_router=FakeNeedleRouter(
                '[{"name":"read_field","arguments":{"id_or_path":"/secret/ghost.md","field":"title"}}]'
            ),
        )
    )
    forbidden = read_service.memory_route(flint, request="show /secret/ghost.md title")
    assert forbidden.status == "success"
    assert success_data(forbidden)["result"]["status"] == "error"
    assert success_data(forbidden)["result"]["error_class"] == "forbidden"

    malformed_service = MemoryService(
        ServiceDependencies(
            config=enabled,
            repo_paths=service._deps.repo_paths,
            control_connection=service._deps.control_connection,
            derived_index=service._deps.derived_index,
            transaction_manager=service._deps.transaction_manager,
            model_client=service._deps.model_client,
            needle_router=FakeNeedleRouter('{"name":"search_paths"}'),
        )
    )
    malformed = malformed_service.memory_route(smith, request="bad")
    assert malformed.status == "error"
    assert malformed.error_class == "validation_error"


def test_memory_route_disabled_and_server_discovery(
    service: MemoryService,
    service_config: ServiceConfig,
    smith: ServiceContext,
) -> None:
    disabled = service.memory_route(smith, request="find Piclaw")
    assert disabled.status == "error"
    assert disabled.error_class == "validation_error"

    enabled = service_config.model_copy(
        update={
            "intelligent_tiers": service_config.intelligent_tiers.model_copy(
                update={
                    "needle_router": service_config.intelligent_tiers.needle_router.model_copy(
                        update={"enabled": True}
                    )
                }
            )
        }
    )
    server = _server_for(
        service,
        enabled.model_copy(update={"mcp": MCPConfig(tool_surface="compact")}),
        needle_router=FakeNeedleRouter('[{"name":"UNKNOWN","arguments":{}}]'),
    )
    tools = [item["name"] for item in server.discover_tools()["tools"]]
    assert tools == [
        "memory_help",
        "memory_status",
        "memory_search",
        "memory_read",
        "memory_route",
        "memory_execute",
    ]
    catalog = json.loads(asyncio.run(server.resource_catalog())["text"])
    assert any(
        item["operation"] == "route" and item["tool"] == "memory_route"
        for item in catalog["operations"]
    )


def test_server_rejects_duplicate_principal_names(
    service: MemoryService, service_config: ServiceConfig
) -> None:
    with pytest.raises(ValueError, match="duplicate principal name"):
        MementoMCPServer(
            service,
            bearer_tokens={
                "smith-a": Principal(name="smith", roles=("reader",)),
                "smith-b": Principal(name="smith", roles=("reader", "curator")),
            },
        )


def test_execute_search_read_and_projection(service: MemoryService, flint: ServiceContext) -> None:
    result = service.memory_execute(
        flint,
        plan={
            "operations": [
                {"op": "search", "args": {"query": "Piclaw"}, "save_as": "hits"},
                {"op": "read", "args": {"id_or_path": "$hits.results.0.path"}, "save_as": "doc"},
            ],
            "returns": [{"name": "title", "ref": "$doc.frontmatter.title"}],
        },
    )
    assert result.status == "success"
    assert success_data(result)["returns"]["title"] == "Piclaw"


def test_execute_rejects_invalid_references_and_multiple_commit_ops(
    service: MemoryService,
    smith: ServiceContext,
    flint: ServiceContext,
    repo_paths: GitRepositoryPaths,
) -> None:
    invalid = service.memory_execute(
        flint,
        plan={
            "operations": [
                {"op": "search", "args": {"query": "Piclaw"}, "save_as": "hits"},
                {"op": "read", "args": {"id_or_path": "$hits.results[0].path"}},
            ]
        },
    )
    assert invalid.status == "error"
    assert invalid.error_class == "validation_error"

    revision = get_main_revision(repo_paths)
    commit_heavy = service.memory_execute(
        smith,
        plan={
            "operations": [
                {
                    "op": "create",
                    "args": {
                        "path": "/projects/a.md",
                        "concept_type": "project",
                        "title": "A",
                        "body": "# A\n",
                        "expected_revision": revision,
                        "idempotency_key": "a-1",
                    },
                },
                {
                    "op": "patch",
                    "args": {
                        "path": "/projects/piclaw.md",
                        "expected_revision": revision,
                        "idempotency_key": "p-1",
                        "description": "x",
                    },
                },
            ]
        },
    )
    assert commit_heavy.status == "error"
    assert commit_heavy.error_class == "validation_error"


def test_execute_limits_auth_and_error_control(
    service: MemoryService,
    service_config: ServiceConfig,
    smith: ServiceContext,
    flint: ServiceContext,
) -> None:
    forbidden = service.memory_execute(
        flint,
        plan={
            "operations": [
                {
                    "op": "create",
                    "args": {
                        "path": "/projects/nope.md",
                        "concept_type": "project",
                        "title": "Nope",
                        "body": "# Nope\n",
                        "expected_revision": service.memory_status(flint).repo_revision,
                        "idempotency_key": "nope-1",
                    },
                }
            ]
        },
    )
    assert forbidden.status == "success"
    trace = success_data(forbidden)["trace"]
    assert trace[0]["status"] == "error"
    assert trace[0]["error_class"] == "forbidden"

    continued = service.memory_execute(
        flint,
        plan={
            "stop_on_error": False,
            "operations": [
                {"op": "read", "args": {"id_or_path": "/secret/ghost.md"}},
                {"op": "read", "args": {"id_or_path": "/projects/piclaw.md"}, "save_as": "doc"},
            ],
            "returns": [{"name": "path", "ref": "$doc.path"}],
        },
    )
    assert continued.status == "success"
    continued_data = success_data(continued)
    assert continued_data["trace"][0]["status"] == "error"
    assert continued_data["returns"]["path"] == "/projects/piclaw.md"

    limited_service = MemoryService(
        ServiceDependencies(
            config=service_config.model_copy(
                update={
                    "mcp": MCPConfig(
                        tool_surface="compact",
                        execute=MCPExecuteLimitsConfig(
                            max_operations=1, max_output_bytes=512, max_time_seconds=3.0
                        ),
                    )
                }
            ),
            repo_paths=service._deps.repo_paths,
            control_connection=service._deps.control_connection,
            derived_index=service._deps.derived_index,
            transaction_manager=service._deps.transaction_manager,
            model_client=service._deps.model_client,
        )
    )
    too_many = limited_service.memory_execute(
        flint,
        plan={
            "operations": [
                {"op": "status", "args": {}},
                {"op": "status", "args": {}},
            ]
        },
    )
    assert too_many.status == "error"
    assert too_many.error_class == "validation_error"

    too_large = limited_service.memory_execute(
        flint,
        plan={
            "operations": [{"op": "search", "args": {"query": "Piclaw"}, "save_as": "hits"}],
            "returns": [{"name": "hits", "ref": "$hits"}],
        },
    )
    assert too_large.status == "error"
    assert too_large.error_class == "validation_error"

    committed = limited_service.memory_execute(
        smith,
        plan={
            "operations": [
                {
                    "op": "create",
                    "args": {
                        "path": "/projects/large-trace.md",
                        "concept_type": "project",
                        "title": "Large Trace",
                        "body": "# Large Trace\n\n" + ("x" * 2000),
                        "expected_revision": get_main_revision(service._deps.repo_paths),
                        "idempotency_key": "large-trace-1",
                    },
                    "save_as": "created",
                }
            ],
            "returns": [{"name": "created", "ref": "$created"}],
        },
    )
    assert committed.status == "success"
    assert "memory_execute_output_truncated_after_commit" in committed.warnings
    committed_data = success_data(committed)
    assert committed_data["truncated"] is True
    assert committed_data["returns"] == {"truncated": True}
    assert (service._deps.repo_paths.current_dir / "projects" / "large-trace.md").exists()


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
            (datetime.now(tz=UTC) - timedelta(days=1))
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
            created_at=datetime(2026, 7, 17, 12, 0, tzinfo=UTC),
            updated_at=datetime(2026, 7, 17, 12, 0, tzinfo=UTC),
            updated_by="rui/tests",
        ),
        body=body,
    )
    path.write_text(serialize_concept(document), encoding="utf-8")
