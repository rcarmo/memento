from __future__ import annotations

import asyncio
import base64
import hashlib
import http.client
import io
import json
import socket
import sqlite3
import threading
import time
import zipfile
from collections.abc import Generator
from contextlib import contextmanager, suppress
from datetime import UTC, datetime, timedelta
from pathlib import Path
from types import SimpleNamespace
from typing import Any, Literal, cast

import pytest

from memento import __version__
from memento.access import AccessStore
from memento.authz import AuthorizationError
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
from memento.repository.asset_packs import write_asset_version
from memento.repository.frontmatter import serialize_concept
from memento.repository.git import GitRepositoryPaths, bootstrap_repository, get_main_revision
from memento.repository.schema import ConceptDocument, ConceptFrontmatter, ConceptStatus
from memento.repository.transactions import TransactionManager
from memento.server import (
    MementoMCPServer,
    execute_tool_schema,
    normalize_execute_tool_arguments,
)
from memento.service import MemoryService, RenameChange, ServiceContext, ServiceDependencies
from memento.skill_packs import validate_asset_pack
from memento.staged_assets import StagedAssetStore


class HTTPTestReader:
    def __init__(self, request: str, body: bytes) -> None:
        self._lines = [line.encode("ascii") + b"\r\n" for line in request.splitlines()]
        self._lines.append(b"\r\n")
        self._body = body

    async def readline(self) -> bytes:
        return self._lines.pop(0) if self._lines else b""

    async def readexactly(self, size: int) -> bytes:
        return self._body[:size]


class HTTPTestWriter:
    def __init__(self) -> None:
        self.chunks: list[bytes] = []
        self.closed = False

    def write(self, data: bytes) -> None:
        self.chunks.append(data)

    async def drain(self) -> None:
        return None

    def close(self) -> None:
        self.closed = True

    async def wait_closed(self) -> None:
        return None

    def is_closing(self) -> bool:
        return self.closed

    def get_extra_info(self, name: str) -> tuple[str, int] | None:
        return ("127.0.0.1", 50000) if name == "peername" else None


@contextmanager
def running_streamable_server(
    server: MementoMCPServer,
) -> Generator[tuple[int, asyncio.AbstractEventLoop], None, None]:
    with socket.socket() as probe:
        probe.bind(("127.0.0.1", 0))
        port = int(probe.getsockname()[1])
    ready = threading.Event()
    holder: dict[str, Any] = {}
    errors: list[BaseException] = []

    def run() -> None:
        async def serve() -> None:
            task = asyncio.current_task()
            assert task is not None
            holder["loop"] = asyncio.get_running_loop()
            holder["task"] = task
            ready.set()
            with suppress(asyncio.CancelledError):
                await server.run_streamable_http_async(host="127.0.0.1", port=port)

        try:
            asyncio.run(serve())
        except BaseException as exc:  # pragma: no cover - surfaced in the caller
            errors.append(exc)

    thread = threading.Thread(target=run, name="memento-http-test", daemon=True)
    thread.start()
    assert ready.wait(timeout=2.0)
    deadline = time.monotonic() + 2.0
    while time.monotonic() < deadline:
        try:
            with socket.create_connection(("127.0.0.1", port), timeout=0.05):
                break
        except OSError:
            time.sleep(0.02)
    else:
        raise AssertionError("Memento Streamable HTTP server did not start")
    try:
        yield port, cast(asyncio.AbstractEventLoop, holder["loop"])
    finally:
        loop = cast(asyncio.AbstractEventLoop, holder["loop"])
        task = cast(asyncio.Task[None], holder["task"])
        loop.call_soon_threadsafe(task.cancel)
        thread.join(timeout=2.0)
        assert not thread.is_alive()
        if errors:
            raise errors[0]


def mcp_post(
    connection: http.client.HTTPConnection,
    payload: dict[str, Any],
    *,
    session_id: str | None = None,
) -> tuple[http.client.HTTPResponse, dict[str, Any]]:
    body = json.dumps(payload).encode("utf-8")
    headers = {
        "Accept": "application/json",
        "Authorization": "Bearer smith-token",
        "Content-Type": "application/json",
        "Content-Length": str(len(body)),
    }
    if payload.get("method") != "initialize":
        headers["MCP-Protocol-Version"] = "2025-03-26"
    if session_id is not None:
        headers["Mcp-Session-Id"] = session_id
    connection.request("POST", "/mcp", body=body, headers=headers)
    response = connection.getresponse()
    response_body = response.read()
    return response, cast(dict[str, Any], json.loads(response_body))


def open_mcp_event_stream(
    port: int, session_id: str
) -> tuple[http.client.HTTPConnection, http.client.HTTPResponse]:
    connection = http.client.HTTPConnection("127.0.0.1", port, timeout=2.0)
    connection.request(
        "GET",
        "/mcp",
        headers={
            "Accept": "text/event-stream",
            "Authorization": "Bearer smith-token",
            "MCP-Protocol-Version": "2025-03-26",
            "Mcp-Session-Id": session_id,
        },
    )
    return connection, connection.getresponse()


def read_sse_payload(response: http.client.HTTPResponse) -> dict[str, Any]:
    while True:
        line = response.readline()
        if line == b"":
            raise AssertionError("SSE stream closed before an event arrived")
        if line.startswith(b"data: "):
            payload = cast(dict[str, Any], json.loads(line.removeprefix(b"data: ")))
            assert response.readline() in (b"\r\n", b"\n")
            return payload


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
                    read_prefixes=("/",),
                    write_prefixes=("/projects/", "/skills/", "/secret/"),
                ),
                "ghost": NamespacePolicy(
                    roles=("reader",),
                    token_env="MEMENTO_TOKEN_GHOST",
                    read_prefixes=("/secret/",),
                    write_prefixes=(),
                ),
                "narrow": NamespacePolicy(
                    roles=("reader", "proposer", "curator"),
                    token_env="MEMENTO_TOKEN_NARROW",
                    read_prefixes=("/instances/",),
                    write_prefixes=("/instances/",),
                ),
            },
            protected_read_prefixes=("/secret/",),
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
def narrow() -> ServiceContext:
    return ServiceContext(Principal(name="narrow", roles=("reader", "proposer", "curator")))


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

    listed = success_data(service.memory_list(flint))
    assert "/secret/ghost.md" not in [item["path"] for item in listed["entries"]]
    status = success_data(service.memory_status(flint))
    assert status["visible_concepts"] == 2

    asset = service.memory_asset_get(flint, id_or_path="/secret/ghost.md", asset_kind="skill")
    assert asset.status == "error"
    assert asset.error_class == "forbidden"

    hidden_visible = service.memory_read(ghost, id_or_path="/secret/ghost.md")
    assert hidden_visible.status == "success"
    assert success_data(hidden_visible)["frontmatter"]["title"] == "Ghost"


def test_audit_skips_protected_content_and_broken_targets(
    service: MemoryService,
    repo_paths: GitRepositoryPaths,
    flint: ServiceContext,
) -> None:
    visible = repo_paths.current_dir / "projects" / "piclaw.md"
    visible.write_text(
        visible.read_text(encoding="utf-8")
        + "\n[Visible missing](/projects/missing.md)\n"
        + "[Protected missing](/secret/missing.md)\n",
        encoding="utf-8",
    )
    hidden = repo_paths.current_dir / "secret" / "malformed.md"
    hidden.write_text("not valid frontmatter\n", encoding="utf-8")

    listed = success_data(service.memory_list(flint))
    assert "/secret/malformed.md" not in [item["path"] for item in listed["entries"]]
    payload = success_data(service.memory_audit(flint))
    messages = [issue["message"] for issue in payload["issues"]]
    assert messages == ["broken link to /projects/missing.md"]
    assert all("/secret/" not in message for message in messages)


def test_protected_proposals_are_hidden_without_explicit_read_access(
    service: MemoryService,
    repo_paths: GitRepositoryPaths,
    flint: ServiceContext,
) -> None:
    proposed = service.memory_propose(
        flint,
        intent="Update protected memory",
        base_revision=get_main_revision(repo_paths),
        changes=[
            {
                "kind": "patch",
                "path": "/secret/ghost.md",
                "body": "# Updated ghost\n",
            }
        ],
    )
    proposal_id = success_data(proposed)["proposal"]["proposal_id"]

    fetched = service.memory_proposal_get(flint, proposal_id=proposal_id)
    assert fetched.status == "error"
    assert fetched.error_class == "forbidden"
    listed = success_data(service.memory_proposal_list(flint))
    assert proposal_id not in [item["proposal_id"] for item in listed["proposals"]]


def test_memory_list_includes_light_frontmatter_metadata(
    service: MemoryService, flint: ServiceContext
) -> None:
    payload = success_data(service.memory_list(flint, path_prefix="/projects/"))
    assert payload["entries"] == [
        {
            "path": "/projects/piclaw.md",
            "id": "piclaw-id",
            "title": "Piclaw",
            "type": "project",
            "status": "active",
            "description": "Visible project.",
            "aliases": (),
            "tags": ("shared",),
        }
    ]


def test_memory_inventory_returns_stable_digests_assets_and_pagination(
    service: MemoryService,
    repo_paths: GitRepositoryPaths,
    smith: ServiceContext,
    flint: ServiceContext,
) -> None:
    concept_id = "12345678-abcd-1234-abcd-123456789abc"
    body = "# Inventory asset\n\nStable body."
    write_concept(
        repo_paths.current_dir / "skills" / "inventory.md",
        concept_id=concept_id,
        concept_type="concept",
        title="Inventory asset",
        description="Digest fixture.",
        tags=("inventory", "skill"),
        body=body,
    )
    write_concept(
        repo_paths.current_dir / "root.md",
        concept_id="root-id",
        concept_type="concept",
        title="Root concept",
        description="Pagination ordering fixture.",
        tags=(),
        body="# Root",
    )
    latest_sha256 = ""
    for version in ("1.0.0", "1.1.0"):
        _encoded, zip_bytes = _skill_zip(f"asset {version}\n")
        pack = validate_asset_pack(asset_kind="templates", version=version, zip_bytes=zip_bytes)
        write_asset_version(
            repo_paths.current_dir,
            concept_id=concept_id,
            concept_path="/skills/inventory.md",
            asset_kind="templates",
            version=version,
            zip_bytes=pack.zip_bytes,
            manifest=pack.manifest,
            accepted_by="smith",
            source_proposal_id=f"proposal-{version}",
        )
        latest_sha256 = pack.manifest.sha256

    payload = success_data(service.memory_inventory(smith, path_prefix="/skills/", limit=50))
    assert payload["next_cursor"] is None
    assert len(payload["entries"]) == 1
    entry = payload["entries"][0]
    assert entry["path"] == "/skills/inventory.md"
    assert entry["created_at"] == "2026-07-17T12:00:00Z"
    assert entry["updated_at"] == "2026-07-17T12:00:00Z"
    assert entry["updated_by"] == "rui/tests"
    assert entry["body_sha256"] == hashlib.sha256(body.encode("utf-8")).hexdigest()
    assert entry["body_bytes"] == len(body.encode("utf-8"))
    assert "body" not in entry
    assert entry["assets"] == [
        {
            "kind": "templates",
            "versions": ("1.0.0", "1.1.0"),
            "latest_version": "1.1.0",
            "latest_sha256": latest_sha256,
        }
    ]

    first_page = success_data(service.memory_inventory(flint, limit=1, fields=["path"]))
    assert first_page["entries"] == [{"path": "/instances/smith.md"}]
    assert first_page["next_cursor"] == "/instances/smith.md"
    second_page = success_data(
        service.memory_inventory(
            flint,
            limit=1,
            fields=["path"],
            cursor=first_page["next_cursor"],
        )
    )
    assert second_page["entries"] == [{"path": "/projects/piclaw.md"}]
    assert second_page["next_cursor"] == "/projects/piclaw.md"
    final_page = success_data(
        service.memory_inventory(
            flint,
            limit=1,
            fields=["path"],
            cursor=second_page["next_cursor"],
        )
    )
    assert final_page["entries"] == [{"path": "/root.md"}]
    assert final_page["next_cursor"] == "/root.md"
    last_page = success_data(
        service.memory_inventory(
            flint,
            limit=1,
            fields=["path"],
            cursor=final_page["next_cursor"],
        )
    )
    assert last_page["entries"] == [{"path": "/skills/inventory.md"}]
    assert last_page["next_cursor"] is None


def test_memory_inventory_filters_protected_namespaces_before_parsing(
    service: MemoryService,
    repo_paths: GitRepositoryPaths,
    flint: ServiceContext,
    ghost: ServiceContext,
) -> None:
    hidden = repo_paths.current_dir / "secret" / "malformed.md"
    hidden.write_text("not valid frontmatter\n", encoding="utf-8")

    visible = success_data(service.memory_inventory(flint, fields=["path"]))
    assert [entry["path"] for entry in visible["entries"]] == [
        "/instances/smith.md",
        "/projects/piclaw.md",
    ]
    forbidden = service.memory_inventory(flint, path_prefix="/secret/")
    assert forbidden.status == "error"
    assert forbidden.error_class == "forbidden"

    hidden.unlink()
    protected = success_data(service.memory_inventory(ghost, path_prefix="/secret/"))
    assert [entry["path"] for entry in protected["entries"]] == ["/secret/ghost.md"]


def test_memory_inventory_parses_only_the_bounded_page(
    service: MemoryService,
    repo_paths: GitRepositoryPaths,
    flint: ServiceContext,
) -> None:
    malformed = repo_paths.current_dir / "projects" / "zzz-malformed.md"
    malformed.write_text("not valid frontmatter\n", encoding="utf-8")

    first = success_data(
        service.memory_inventory(
            flint,
            path_prefix="/projects/",
            fields=["path"],
            limit=1,
        )
    )
    assert first["entries"] == [{"path": "/projects/piclaw.md"}]
    assert first["next_cursor"] == "/projects/piclaw.md"

    invalid_page = service.memory_inventory(
        flint,
        path_prefix="/projects/",
        fields=["path"],
        limit=1,
        cursor=first["next_cursor"],
    )
    assert invalid_page.status == "error"
    assert invalid_page.error_class == "validation_error"


def test_memory_inventory_is_available_directly_and_via_execute(
    service: MemoryService,
    service_config: ServiceConfig,
    flint: ServiceContext,
) -> None:
    server = _server_for(
        service,
        service_config.model_copy(update={"mcp": MCPConfig(tool_surface="compact")}),
    )
    tools = {item["name"] for item in server.discover_tools()["tools"]}
    assert "memory_inventory" in tools
    result = service.memory_execute(
        flint,
        plan={
            "operations": [
                {
                    "op": "inventory",
                    "args": {
                        "path_prefix": "/projects/",
                        "fields": ["path", "body_sha256", "updated_at"],
                        "limit": 10,
                    },
                }
            ]
        },
    )
    inventory = success_data(result)["trace"][0]["data"]
    assert inventory["entries"][0]["path"] == "/projects/piclaw.md"
    assert "body_sha256" in inventory["entries"][0]
    assert "body" not in inventory["entries"][0]


def test_asset_metadata_returns_generic_versions_files_timestamps_and_skill_parity(
    service: MemoryService,
    repo_paths: GitRepositoryPaths,
    smith: ServiceContext,
) -> None:
    concept_id = "12345678-abcd-1234-abcd-123456789abc"
    skill_body = "---\nname: metadata-skill\ndescription: Metadata fixture\n---\n# Metadata skill"
    write_concept(
        repo_paths.current_dir / "skills" / "metadata.md",
        concept_id=concept_id,
        concept_type="concept",
        title="Metadata skill",
        description="Asset metadata fixture.",
        tags=("skill",),
        body=skill_body,
    )
    created_at = datetime(2026, 8, 26, 6, 30, tzinfo=UTC)
    expected_zip_bytes = 0
    expected_zip_sha256 = ""
    for version in ("1.0.0", "1.1.0"):
        _encoded, zip_bytes = _skill_zip(skill_body, script=f"console.log('{version}')\n")
        pack = validate_asset_pack(asset_kind="skill", version=version, zip_bytes=zip_bytes)
        write_asset_version(
            repo_paths.current_dir,
            concept_id=concept_id,
            concept_path="/skills/original-metadata.md",
            asset_kind="skill",
            version=version,
            zip_bytes=pack.zip_bytes,
            manifest=pack.manifest,
            accepted_by="smith",
            source_proposal_id=f"proposal-{version}",
            created_at=created_at,
        )
        expected_zip_bytes = len(pack.zip_bytes)
        expected_zip_sha256 = pack.manifest.sha256

    for version in ("2.0.0", "2.1.0", "2.2.0"):
        _encoded, template_bytes = _skill_zip(f"template payload {version}\n")
        template_pack = validate_asset_pack(
            asset_kind="templates", version=version, zip_bytes=template_bytes
        )
        write_asset_version(
            repo_paths.current_dir,
            concept_id=concept_id,
            concept_path="/skills/original-metadata.md",
            asset_kind="templates",
            version=version,
            zip_bytes=template_pack.zip_bytes,
            manifest=template_pack.manifest,
            accepted_by="smith",
            source_proposal_id=f"proposal-templates-{version}",
            created_at=created_at,
        )

    payload = success_data(
        service.memory_asset_metadata(
            smith,
            id_or_path="/skills/metadata.md",
            asset_kind="skill",
            include_files=True,
        )
    )
    assert payload["next_cursor"] is None
    assert len(payload["entries"]) == 1
    entry = payload["entries"][0]
    assert entry["path"] == "/skills/metadata.md"
    assert (
        entry["current_concept_body_sha256"]
        == hashlib.sha256(skill_body.encode("utf-8")).hexdigest()
    )
    assert entry["current_concept_body_bytes"] == len(skill_body.encode("utf-8"))
    assert entry["asset_present"] is True
    assert len(entry["assets"]) == 1
    asset = entry["assets"][0]
    assert asset["kind"] == "skill"
    assert asset["versions"] == ("1.0.0", "1.1.0")
    assert asset["latest_version"] == "1.1.0"
    assert asset["latest_sha256"] == expected_zip_sha256
    assert asset["version_metadata_truncated"] is False
    assert [item["version"] for item in asset["version_metadata"]] == [
        "1.1.0",
        "1.0.0",
    ]
    latest = asset["version_metadata"][0]
    assert latest["created_at"] == "2026-08-26T06:30:00Z"
    assert latest["created_by"] == "smith"
    assert latest["source_proposal_id"] == "proposal-1.1.0"
    assert latest["zip_sha256"] == expected_zip_sha256
    assert latest["zip_bytes"] == expected_zip_bytes
    assert latest["file_count"] == 2
    assert latest["total_uncompressed_bytes"] == sum(item["bytes"] for item in latest["files"])
    assert latest["files_truncated"] is False
    assert [item["path"] for item in latest["files"]] == [
        "SKILL.md",
        "scripts/run.ts",
    ]
    assert latest["concept_body_sha256_at_publish"] == entry["current_concept_body_sha256"]
    assert latest["kind_invariants"] == {"skill_root_matches_current_concept_body": True}
    assert "body" not in entry
    assert "body" not in latest
    assert "zip_base64" not in latest

    generic = success_data(
        service.memory_asset_metadata(
            smith,
            id_or_path="/skills/metadata.md",
            asset_kind="templates",
            version_limit=2,
            include_files=True,
        )
    )["entries"][0]["assets"][0]
    assert generic["kind"] == "templates"
    assert generic["versions"] == ("2.1.0", "2.2.0")
    assert generic["version_count"] == 3
    assert generic["versions_truncated"] is True
    assert generic["version_metadata_truncated"] is True
    assert [item["version"] for item in generic["version_metadata"]] == [
        "2.2.0",
        "2.1.0",
    ]
    assert "kind_invariants" not in generic["version_metadata"][0]
    assert "concept_body_sha256_at_publish" not in generic["version_metadata"][0]

    truncated_files = success_data(
        service.memory_asset_metadata(
            smith,
            id_or_path="/skills/metadata.md",
            asset_kind="skill",
            version="1.1.0",
            include_files=True,
            file_limit=1,
        )
    )["entries"][0]["assets"][0]["version_metadata"][0]
    assert len(truncated_files["files"]) == 1
    assert truncated_files["files_truncated"] is True

    write_concept(
        repo_paths.current_dir / "skills" / "metadata.md",
        concept_id=concept_id,
        concept_type="concept",
        title="Metadata skill",
        description="Asset metadata fixture.",
        tags=("skill",),
        body=skill_body + "\n\nChanged.",
    )
    changed = success_data(
        service.memory_asset_metadata(
            smith,
            id_or_path=concept_id,
            asset_kind="skill",
            version="1.0.0",
        )
    )
    selected = changed["entries"][0]["assets"][0]["version_metadata"]
    assert [item["version"] for item in selected] == ["1.0.0"]
    assert selected[0]["kind_invariants"] == {"skill_root_matches_current_concept_body": False}
    missing = success_data(
        service.memory_asset_metadata(
            smith,
            id_or_path="/skills/metadata.md",
            asset_kind="skill",
            version="9.0.0",
        )
    )["entries"][0]
    assert missing["asset_present"] is True
    assert missing["assets"][0]["requested_version_present"] is False
    assert missing["assets"][0]["versions"] == ()
    assert missing["assets"][0]["version_metadata"] == []


def test_asset_metadata_prunes_protected_paths_before_parsing_and_pages_batches(
    service: MemoryService,
    repo_paths: GitRepositoryPaths,
    flint: ServiceContext,
) -> None:
    write_concept(
        repo_paths.current_dir / "skills" / "a.md",
        concept_id="asset-a-id",
        concept_type="concept",
        title="Asset A",
        description="Batch fixture A.",
        tags=(),
        body="# Asset A",
    )
    write_concept(
        repo_paths.current_dir / "skills" / "b.md",
        concept_id="asset-b-id",
        concept_type="concept",
        title="Asset B",
        description="Batch fixture B.",
        tags=(),
        body="# Asset B",
    )
    malformed = repo_paths.current_dir / "secret" / "malformed.md"
    malformed.write_text("not valid frontmatter\n", encoding="utf-8")

    first = success_data(service.memory_asset_metadata(flint, path_prefix="/skills/", limit=1))
    assert [item["path"] for item in first["entries"]] == ["/skills/a.md"]
    assert first["entries"][0]["asset_present"] is False
    assert first["next_cursor"] == "/skills/a.md"
    second = success_data(
        service.memory_asset_metadata(
            flint,
            path_prefix="/skills/",
            limit=1,
            cursor=first["next_cursor"],
        )
    )
    assert [item["path"] for item in second["entries"]] == ["/skills/b.md"]
    assert second["next_cursor"] is None

    visible = success_data(service.memory_asset_metadata(flint, path_prefix="/"))
    assert all(not item["path"].startswith("/secret/") for item in visible["entries"])
    forbidden = service.memory_asset_metadata(flint, id_or_path="/secret/malformed.md")
    assert forbidden.status == "error"
    assert forbidden.error_class == "forbidden"


def test_asset_metadata_validates_scope_bounds_and_persisted_metadata(
    service: MemoryService,
    repo_paths: GitRepositoryPaths,
    flint: ServiceContext,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    invalid_calls = (
        {
            "id_or_path": "/projects/piclaw.md",
            "path_prefix": "/projects/",
        },
        {"id_or_path": "/projects/piclaw.md", "cursor": "/projects/piclaw.md"},
        {"id_or_path": "/projects/piclaw.md", "version": "1.0.0"},
        {"path_prefix": "/projects/", "limit": 21},
        {"path_prefix": "/projects/", "version_limit": 6},
        {"path_prefix": "/projects/", "file_limit": 101},
        {"path_prefix": "/projects/", "asset_kind": "Invalid Kind"},
    )
    for arguments in invalid_calls:
        result = service.memory_asset_metadata(flint, **arguments)
        assert result.status == "error"
        assert result.error_class == "validation_error"

    concept_id = "87654321-abcd-1234-abcd-123456789abc"
    write_concept(
        repo_paths.current_dir / "projects" / "malformed-asset.md",
        concept_id=concept_id,
        concept_type="concept",
        title="Malformed asset",
        description="Persisted metadata boundary fixture.",
        tags=(),
        body="# Malformed asset",
    )
    metadata_dir = repo_paths.current_dir / ".assets" / concept_id / "templates"
    metadata_dir.mkdir(parents=True)
    metadata_file = metadata_dir / "1.0.0.json"
    metadata_file.write_text(json.dumps({"kind": "asset_pack_version"}), encoding="utf-8")
    malformed = service.memory_asset_metadata(flint, id_or_path="/projects/malformed-asset.md")
    assert malformed.status == "error"
    assert malformed.error_class == "validation_error"

    metadata_file.unlink()
    metadata_file.symlink_to(repo_paths.current_dir / "secret" / "ghost.md")

    def fail_if_loaded(*_args: object, **_kwargs: object) -> dict[str, object]:
        pytest.fail("symbolic-link metadata must be rejected before loading")

    monkeypatch.setattr("memento.service.load_asset_metadata", fail_if_loaded)
    linked = service.memory_asset_metadata(flint, id_or_path="/projects/malformed-asset.md")
    assert linked.status == "error"
    assert linked.error_class == "validation_error"


def test_asset_metadata_is_execute_only_with_service_execute_parity(
    service: MemoryService,
    service_config: ServiceConfig,
    flint: ServiceContext,
) -> None:
    server = _server_for(
        service,
        service_config.model_copy(update={"mcp": MCPConfig(tool_surface="compact")}),
    )
    tools = {item["name"] for item in server.discover_tools()["tools"]}
    assert "memory_asset_metadata" not in tools
    catalog = json.loads(asyncio.run(server.resource_catalog())["text"])
    assert any(item["operation"] == "asset_metadata" for item in catalog["execute_only_operations"])

    direct = success_data(service.memory_asset_metadata(flint, id_or_path="/projects/piclaw.md"))
    executed = success_data(
        service.memory_execute(
            flint,
            plan={
                "operations": [
                    {
                        "op": "asset_metadata",
                        "args": {"id_or_path": "/projects/piclaw.md"},
                    }
                ]
            },
        )
    )["trace"][0]["data"]
    assert executed == direct


def test_compare_manifest_classifies_generic_entries_and_assets(
    service: MemoryService,
    repo_paths: GitRepositoryPaths,
    flint: ServiceContext,
) -> None:
    concepts = {
        "same": ("same-id", "# Same"),
        "renamed": ("12345678-abcd-1234-abcd-123456789abc", "# Remote old"),
        "memento-newer": ("newer-id", "# Remote new"),
        "remote-only": ("remote-only-id", "# Remote only"),
    }
    for name, (concept_id, body) in concepts.items():
        write_concept(
            repo_paths.current_dir / "compare" / f"{name}.md",
            concept_id=concept_id,
            concept_type="concept",
            title=name,
            description="Manifest comparison fixture.",
            tags=("comparison",),
            body=body,
        )

    _encoded, zip_bytes = _skill_zip("comparison asset\n")
    pack = validate_asset_pack(asset_kind="templates", version="1.0.0", zip_bytes=zip_bytes)
    write_asset_version(
        repo_paths.current_dir,
        concept_id=concepts["renamed"][0],
        concept_path="/compare/renamed.md",
        asset_kind="templates",
        version="1.0.0",
        zip_bytes=pack.zip_bytes,
        manifest=pack.manifest,
        accepted_by="flint",
        source_proposal_id="proposal-compare",
    )

    def digest(body: str) -> str:
        return hashlib.sha256(body.encode("utf-8")).hexdigest()

    result = service.memory_execute(
        flint,
        plan={
            "operations": [
                {
                    "op": "compare_manifest",
                    "args": {
                        "path_prefix": "/compare/",
                        "items": [
                            {
                                "name": "same",
                                "local_path": "/local/same.md",
                                "local_updated_at": "2026-07-17T12:00:00Z",
                                "local_body_sha256": digest("# Same"),
                                "local_bytes": len(b"# Same"),
                            },
                            {
                                "name": "old-name",
                                "local_path": "/local/renamed.md",
                                "local_updated_at": "2026-08-25T12:00:00Z",
                                "local_body_sha256": digest("# Local new"),
                                "local_bytes": len(b"# Local new"),
                            },
                            {
                                "name": "memento-newer",
                                "local_path": "/local/memento-newer.md",
                                "local_updated_at": "2026-01-01T00:00:00Z",
                                "local_body_sha256": digest("# Local old"),
                                "local_bytes": len(b"# Local old"),
                            },
                            {
                                "name": "local-only",
                                "local_path": "/local/local-only.md",
                                "local_updated_at": "2026-08-25T12:00:00Z",
                                "local_body_sha256": digest("# Local only"),
                                "local_bytes": len(b"# Local only"),
                            },
                        ],
                        "match": {
                            "path_template": "/compare/{name}.md",
                            "aliases": {"old-name": "renamed"},
                        },
                        "include_asset_metadata": True,
                    },
                }
            ]
        },
    )
    comparison = success_data(result)["trace"][0]["data"]

    assert comparison["counts"] == {
        "matching": 1,
        "differing": 2,
        "local_only": 1,
        "memento_only": 1,
    }
    assert comparison["matching"][0]["name"] == "same"
    assert comparison["matching"][0]["body_match"] is True
    differing = {entry["name"]: entry for entry in comparison["differing"]}
    assert differing["old-name"]["memento_path"] == "/compare/renamed.md"
    assert differing["old-name"]["likely_newer"] == "local"
    assert differing["old-name"]["asset_present"] is True
    assert differing["old-name"]["assets"] == [
        {
            "kind": "templates",
            "latest_version": "1.0.0",
            "latest_sha256": pack.manifest.sha256,
        }
    ]
    assert differing["memento-newer"]["likely_newer"] == "memento"
    assert comparison["local_only"][0]["memento_path"] == "/compare/local-only.md"
    assert comparison["memento_only"][0]["memento_path"] == "/compare/remote-only.md"
    assert '"body":' not in json.dumps(comparison)


def test_compare_manifest_enforces_scope_and_bounds_before_content_access(
    service: MemoryService,
    repo_paths: GitRepositoryPaths,
    flint: ServiceContext,
) -> None:
    hidden = repo_paths.current_dir / "secret" / "malformed.md"
    hidden.write_text("not valid frontmatter\n", encoding="utf-8")
    local_item = {
        "name": "piclaw",
        "local_path": "/local/piclaw.md",
        "memento_path": "/projects/piclaw.md",
        "local_updated_at": datetime(2026, 7, 17, 12, 0, tzinfo=UTC),
        "local_body_sha256": "0" * 64,
        "local_bytes": 1,
    }
    visible = success_data(
        service.memory_compare_manifest(
            flint,
            path_prefix="/projects/",
            items=[local_item],
        )
    )
    assert visible["counts"] == {
        "matching": 0,
        "differing": 1,
        "local_only": 0,
        "memento_only": 0,
    }

    forbidden = service.memory_compare_manifest(
        flint,
        path_prefix="/",
        items=[{**local_item, "memento_path": "/secret/ghost.md"}],
    )
    assert forbidden.status == "error"
    assert forbidden.error_class == "forbidden"

    for index in range(51):
        write_concept(
            repo_paths.current_dir / "bulk" / f"{index:02d}.md",
            concept_id=f"bulk-{index}",
            concept_type="concept",
            title=f"Bulk {index}",
            description="Bounded comparison fixture.",
            tags=(),
            body=f"# Bulk {index}",
        )
    oversized = service.memory_compare_manifest(
        flint,
        path_prefix="/bulk/",
        items=[
            {
                **local_item,
                "name": "bulk-0",
                "memento_path": "/bulk/00.md",
            }
        ],
    )
    assert oversized.status == "error"
    assert oversized.error_class == "validation_error"
    assert "exceeds 50 concepts" in oversized.message


def test_memory_graph_serializes_typed_edges_and_preserves_scope(
    service: MemoryService, flint: ServiceContext, narrow: ServiceContext
) -> None:
    payload = success_data(service.memory_graph(flint, id_or_path="/instances/smith.md"))
    assert payload == {
        "center_id": "smith-id",
        "outbound": [
            {
                "concept_id": "piclaw-id",
                "path": "/projects/piclaw.md",
                "title": "Piclaw",
                "depth": 1,
                "direction": "outbound",
                "broken_link_count": 0,
                "orphan_flag": False,
            }
        ],
        "inbound": [
            {
                "concept_id": "piclaw-id",
                "path": "/projects/piclaw.md",
                "title": "Piclaw",
                "depth": 1,
                "direction": "inbound",
                "broken_link_count": 0,
                "orphan_flag": False,
            }
        ],
        "broken_targets": (),
    }

    scoped = success_data(service.memory_graph(narrow, id_or_path="/instances/smith.md"))
    assert scoped["outbound"] == []
    assert scoped["inbound"] == []

    protected = service.memory_graph(flint, id_or_path="/secret/ghost.md")
    assert protected.status == "error"
    assert protected.error_class == "forbidden"


def test_proposal_lifecycle_self_approval_stale_apply_and_idempotency(
    service: MemoryService,
    control_connection: sqlite3.Connection,
    repo_paths: GitRepositoryPaths,
    smith: ServiceContext,
    flint: ServiceContext,
) -> None:
    base_revision = get_main_revision(repo_paths)
    proposed = service.memory_propose(
        smith,
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

    approved = service.memory_proposal_review(
        smith, proposal_id=proposal_id, decision="approve", comment="ok"
    )
    assert approved.status == "success"
    approved_proposal = success_data(approved)["proposal"]
    assert approved_proposal["status"] == "approved"
    assert approved_proposal["author_principal"] == "smith"
    assert approved_proposal["reviewed_by"] == "smith"

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


def test_proposal_review_keeps_role_and_write_scope_checks(
    service: MemoryService,
    repo_paths: GitRepositoryPaths,
    smith: ServiceContext,
    flint: ServiceContext,
    narrow: ServiceContext,
) -> None:
    proposed = service.memory_propose(
        smith,
        intent="Check review authorization",
        base_revision=get_main_revision(repo_paths),
        changes=[
            {
                "kind": "patch",
                "path": "/projects/piclaw.md",
                "body": "# Piclaw\n\nReview authorization test.\n",
            }
        ],
    )
    proposal_id = success_data(proposed)["proposal"]["proposal_id"]

    non_curator = service.memory_proposal_review(flint, proposal_id=proposal_id, decision="approve")
    assert non_curator.status == "error"
    assert non_curator.error_class == "forbidden"

    outside_write_scope = service.memory_proposal_review(
        narrow, proposal_id=proposal_id, decision="approve"
    )
    assert outside_write_scope.status == "error"
    assert outside_write_scope.error_class == "forbidden"

    current = success_data(service.memory_proposal_get(smith, proposal_id=proposal_id))["proposal"]
    assert current["status"] == "submitted"
    assert current["reviewed_by"] is None


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


def test_rename_rejects_unauthorised_backlink_rewrites(
    service: MemoryService,
    smith: ServiceContext,
    tmp_path: Path,
) -> None:
    worktree = tmp_path / "rename-scope"
    original = worktree / "projects" / "visible.md"
    protected = worktree / "secret" / "backlink.md"
    write_concept(
        original,
        concept_id="visible-id",
        concept_type="project",
        title="Visible",
        description="Visible project.",
        tags=("visible",),
        body="# Visible\n",
    )
    write_concept(
        protected,
        concept_id="backlink-id",
        concept_type="project",
        title="Protected backlink",
        description="Protected backlink.",
        tags=("hidden",),
        body="# Protected\n\nSee [Visible](/projects/visible.md).\n",
    )

    with pytest.raises(AuthorizationError, match="cannot write /secret/backlink.md"):
        service._apply_rename(
            worktree,
            RenameChange(
                kind="rename",
                path="/projects/visible.md",
                new_path="/projects/renamed.md",
            ),
            actor="smith",
            policy=service._policy(smith),
        )

    assert original.exists()
    assert not (worktree / "projects" / "renamed.md").exists()
    assert "/projects/visible.md" in protected.read_text(encoding="utf-8")


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


@pytest.mark.parametrize(
    ("version", "expected_error"),
    [
        (None, "missing MCP-Protocol-Version header"),
        ("not-a-version", "unsupported MCP-Protocol-Version header"),
    ],
)
def test_streamable_http_explains_invalid_protocol_version(
    service: MemoryService,
    service_config: ServiceConfig,
    version: str | None,
    expected_error: str,
) -> None:
    server = _server_for(service, service_config)
    body = json.dumps({"jsonrpc": "2.0", "id": 3, "method": "tools/list", "params": {}}).encode(
        "utf-8"
    )
    headers = [
        "POST /mcp HTTP/1.1",
        "Host: memento.test",
        "Authorization: Bearer smith-token",
        "Content-Type: application/json",
        "Accept: application/json, text/event-stream",
    ]
    if version is not None:
        headers.append(f"MCP-Protocol-Version: {version}")
    headers.append(f"Content-Length: {len(body)}")
    reader = HTTPTestReader("\n".join(headers), body)
    writer = HTTPTestWriter()

    asyncio.run(
        cast(Any, server)._handle_streamable_http_client(reader, writer, "/mcp", [], 1024 * 1024)
    )
    response_headers, response_body = b"".join(writer.chunks).split(b"\r\n\r\n", 1)
    payload = json.loads(response_body)

    assert b"400 Bad Request" in response_headers
    assert b"Content-Type: application/json" in response_headers
    assert payload["error"] == expected_error
    assert payload["expected"] == "2025-03-26"
    assert payload["supported"] == ["2025-03-26", "2024-11-05"]
    if version is not None:
        assert payload["received"] == version


def test_streamable_http_reuses_post_connection_and_preserves_session(
    service: MemoryService,
    service_config: ServiceConfig,
) -> None:
    server = _server_for(service, service_config)
    with running_streamable_server(server) as (port, _loop):
        connection = http.client.HTTPConnection("127.0.0.1", port, timeout=2.0)
        initialized, payload = mcp_post(
            connection,
            {
                "jsonrpc": "2.0",
                "id": 1,
                "method": "initialize",
                "params": {
                    "protocolVersion": "2025-03-26",
                    "capabilities": {},
                    "clientInfo": {"name": "Memento integration test", "version": "1"},
                },
            },
        )
        session_id = initialized.getheader("Mcp-Session-Id")
        assert initialized.status == 200
        assert initialized.will_close is False
        assert session_id is not None
        assert payload["result"]["serverInfo"] == {"name": "memento", "version": __version__}
        assert payload["result"]["capabilities"] == {
            "tools": {},
            "resources": {"subscribe": True, "listChanged": True},
            "logging": {},
        }
        original_socket = connection.sock

        listed, listed_payload = mcp_post(
            connection,
            {"jsonrpc": "2.0", "id": 2, "method": "tools/list", "params": {}},
            session_id=session_id,
        )
        assert listed.status == 200
        assert listed.will_close is False
        assert connection.sock is original_socket
        assert "memory_status" in {tool["name"] for tool in listed_payload["result"]["tools"]}
        connection.close()

        replacement = http.client.HTTPConnection("127.0.0.1", port, timeout=2.0)
        resumed, resumed_payload = mcp_post(
            replacement,
            {"jsonrpc": "2.0", "id": 3, "method": "tools/list", "params": {}},
            session_id=session_id,
        )
        assert resumed.status == 200
        assert resumed_payload["id"] == 3
        replacement.request(
            "DELETE",
            "/mcp",
            headers={
                "Authorization": "Bearer smith-token",
                "MCP-Protocol-Version": "2025-03-26",
                "Mcp-Session-Id": session_id,
            },
        )
        deleted = replacement.getresponse()
        assert deleted.status == 200
        deleted.read()
        replacement.close()


def test_streamable_http_delivers_subscribed_notifications_after_reconnect(
    service: MemoryService,
    service_config: ServiceConfig,
) -> None:
    server = _server_for(service, service_config)
    server.streamable_http_keepalive_seconds = 0.05
    with running_streamable_server(server) as (port, loop):
        control = http.client.HTTPConnection("127.0.0.1", port, timeout=2.0)
        initialized, _payload = mcp_post(
            control,
            {
                "jsonrpc": "2.0",
                "id": 1,
                "method": "initialize",
                "params": {
                    "protocolVersion": "2025-03-26",
                    "capabilities": {},
                    "clientInfo": {"name": "Memento SSE test", "version": "1"},
                },
            },
        )
        session_id = initialized.getheader("Mcp-Session-Id")
        assert session_id is not None
        subscribed, subscribed_payload = mcp_post(
            control,
            {
                "jsonrpc": "2.0",
                "id": 2,
                "method": "resources/subscribe",
                "params": {"uri": "memory://status"},
            },
            session_id=session_id,
        )
        assert subscribed.status == 200
        assert subscribed_payload["result"] == {}

        stream_connection, stream = open_mcp_event_stream(port, session_id)
        assert stream.status == 200
        assert stream.getheader("Content-Type") == "text/event-stream"
        assert stream.readline().startswith(b": connected")
        assert stream.readline() in (b"\r\n", b"\n")
        asyncio.run_coroutine_threadsafe(
            server.notify_resource_updated("memory://status"), loop
        ).result(timeout=2.0)
        notification = read_sse_payload(stream)
        assert notification["method"] == "notifications/resources/updated"
        assert notification["params"] == {"uri": "memory://status"}

        stream.close()
        stream_connection.close()
        session = cast(Any, server)._streamable_http_sessions[session_id]
        deadline = time.monotonic() + 1.0
        while session.writer is not None and time.monotonic() < deadline:
            time.sleep(0.02)
        assert session.writer is None

        reconnected_connection, reconnected = open_mcp_event_stream(port, session_id)
        assert reconnected.status == 200
        assert reconnected.readline().startswith(b": connected")
        assert reconnected.readline() in (b"\r\n", b"\n")
        asyncio.run_coroutine_threadsafe(
            server.notify_resource_updated("memory://status"), loop
        ).result(timeout=2.0)
        assert read_sse_payload(reconnected)["method"] == "notifications/resources/updated"

        control.request(
            "DELETE",
            "/mcp",
            headers={
                "Authorization": "Bearer smith-token",
                "MCP-Protocol-Version": "2025-03-26",
                "Mcp-Session-Id": session_id,
            },
        )
        deleted = control.getresponse()
        assert deleted.status == 200
        deleted.read()
        assert reconnected.readline() == b""
        reconnected.close()
        reconnected_connection.close()
        control.close()
        assert cast(Any, server)._streamable_http_sessions == {}


def test_status_reports_canonical_state_when_derived_index_is_empty(
    tmp_path: Path,
    service: MemoryService,
    flint: ServiceContext,
) -> None:
    empty_index = DerivedIndex(tmp_path / "empty-derived.sqlite")
    empty_service = MemoryService(
        ServiceDependencies(
            config=service._deps.config,
            repo_paths=service._deps.repo_paths,
            control_connection=service._deps.control_connection,
            derived_index=empty_index,
            transaction_manager=service._deps.transaction_manager,
        )
    )

    result = empty_service.memory_status(flint)

    assert result.status == "success"
    data = success_data(result)
    revision = get_main_revision(service._deps.repo_paths)
    assert data["repo_revision"] == revision
    assert data["index_revision"] == ""
    assert data["index_stale"] is True
    assert data["visible_concepts"] == 2
    assert result.repo_revision == revision
    assert result.index_revision == ""
    assert result.index_stale is True
    assert "derived_index_stale" in result.warnings


def test_execute_tool_schema_and_normalization_support_both_argument_forms() -> None:
    schema = execute_tool_schema()
    assert set(schema["properties"]) >= {"plan", "operations", "stop_on_error", "returns"}
    assert schema["additionalProperties"] is False

    plan = {"operations": [{"op": "status", "args": {}}]}
    assert normalize_execute_tool_arguments(plan=plan) is plan
    assert normalize_execute_tool_arguments(
        operations=plan["operations"], stop_on_error=False, returns=[]
    ) == {
        "operations": plan["operations"],
        "stop_on_error": False,
        "returns": [],
    }
    with pytest.raises(ValueError, match="either plan or top-level"):
        normalize_execute_tool_arguments(plan=plan, operations=plan["operations"])
    with pytest.raises(ValueError, match="requires plan or operations"):
        normalize_execute_tool_arguments()


def test_tool_discovery_surfaces_and_catalog_resources(
    service: MemoryService,
    service_config: ServiceConfig,
    smith: ServiceContext,
) -> None:
    expected_counts: tuple[
        tuple[Literal["compact", "standard", "read_only", "curator", "admin"], int], ...
    ] = (
        ("compact", 9),
        ("standard", 23),
        ("read_only", 10),
        ("curator", 14),
        ("admin", 24),
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
        "inventory",
        "asset_stage_begin",
        "asset_stage_status",
        "asset_get",
        "execute",
    ]
    assert any(item["operation"] == "proposal_apply" for item in catalog["execute_only_operations"])
    assert any(
        item["operation"] == "compare_manifest" for item in catalog["execute_only_operations"]
    )
    assert any(item["operation"] == "asset_metadata" for item in catalog["execute_only_operations"])
    asset_metadata_operation = json.loads(
        asyncio.run(server.resource_template_catalog("asset_metadata"))["text"]
    )
    assert asset_metadata_operation["direct_tool_available"] is False
    assert asset_metadata_operation["available_via_execute"] is True
    assert asset_metadata_operation["input_schema"]["properties"]["limit"]["maximum"] == 20
    assert asset_metadata_operation["input_schema"]["properties"]["file_limit"]["maximum"] == 100
    comparison_operation = json.loads(
        asyncio.run(server.resource_template_catalog("compare_manifest"))["text"]
    )
    assert comparison_operation["direct_tool_available"] is False
    assert comparison_operation["available_via_execute"] is True
    assert comparison_operation["input_schema"]["properties"]["items"]["maxItems"] == 50
    operation = json.loads(asyncio.run(server.resource_template_catalog("propose"))["text"])
    assert operation["tool"] == "memory_propose"
    assert operation["direct_tool_available"] is False
    assert operation["available_via_execute"] is True
    changes_schema = operation["input_schema"]["properties"]["changes"]
    assert changes_schema["type"] == "array"
    assert "anyOf" in changes_schema["items"]
    attachment = operation["input_schema"]["$defs"]["ProposeAssetChange"]
    assert {"zip_base64", "staged_asset_id"} <= set(attachment["properties"])
    assert "asset_id" not in attachment["properties"]
    assert "manifest" not in attachment["properties"]
    assert len(attachment["oneOf"]) == 2
    assert {"kind", "path", "asset_kind", "version"} <= set(attachment["required"])
    help_payload = success_data(service.memory_help(smith))
    assert "memory_execute" in help_payload["mcp"]["direct_tools"]
    assert help_payload["goals"]["skills"] == [
        "memory_search",
        "memory_read",
    ]
    assert help_payload["mcp"]["execute_only_operations"]["inspect"] == (
        "compare_manifest",
        "asset_metadata",
    )
    assert help_payload["mcp"]["execute_only_operations"]["propose"] == (
        "propose",
        "propose_freeform",
        "propose_update",
    )
    assert help_payload["catalog"]["workflow_values"] == (
        "inspect",
        "propose",
        "curate",
        "asset_pack",
    )
    assert help_payload["catalog"]["asset_publication"] == {
        "workflow": "memory://workflow/asset_pack",
        "proposal_contract": "memory://catalog/propose",
        "prompt": "publish_asset_pack",
    }
    workflow = json.loads(asyncio.run(server.resource_template_workflow("inspect"))["text"])
    assert [item["operation"] for item in workflow["operations"]] == [
        "search",
        "inventory",
        "read",
    ]
    assert [item["operation"] for item in workflow["execute_only_operations"]] == [
        "compare_manifest",
        "asset_metadata",
    ]
    propose_workflow = json.loads(asyncio.run(server.resource_template_workflow("propose"))["text"])
    assert [item["operation"] for item in propose_workflow["operations"]] == ["search", "read"]
    assert [item["operation"] for item in propose_workflow["execute_only_operations"]] == [
        "propose",
        "propose_freeform",
        "propose_update",
    ]
    asset_workflow = json.loads(
        asyncio.run(server.resource_template_workflow("asset_pack"))["text"]
    )
    assert [step["operation"] for step in asset_workflow["steps"]] == [
        "prepare_asset_pack",
        "propose",
        "proposal_get",
        "proposal_review",
        "proposal_apply",
        "asset_get",
    ]
    assert "no trailing whitespace or final newline" in asset_workflow["steps"][0]["result"]
    proposal_change = asset_workflow["steps"][1]["arguments"]["changes"][0]
    assert proposal_change == {
        "kind": "attach_asset_pack",
        "path": "/skills/example.md",
        "asset_kind": "skill",
        "version": "1.1.0",
        "zip_base64": "<base64 ZIP bytes>",
    }
    assert asset_workflow["staging_fallback"]["proposal_field"] == "staged_asset_id"
    templates = server.discover_resource_templates()["resourceTemplates"]
    by_template = {item["uriTemplate"]: item for item in templates}
    assert "asset_stage_begin" in by_template["memory://catalog/{operation}"]["description"]
    assert "asset_pack" in by_template["memory://workflow/{goal}"]["description"]
    prompts = server.discover_prompts()["prompts"]
    assert [item["name"] for item in prompts] == ["publish_asset_pack"]
    operation_completion = asyncio.run(
        server.handle_completion_complete_async(
            1,
            {
                "ref": {
                    "type": "ref/resource",
                    "uri": "memory://catalog/{operation}",
                },
                "argument": {"name": "operation", "value": "asset_stage"},
            },
        )
    )
    assert operation_completion["result"]["completion"]["values"] == [
        "asset_stage_begin",
        "asset_stage_status",
    ]
    goal_completion = asyncio.run(
        server.handle_completion_complete_async(
            2,
            {
                "ref": {
                    "type": "ref/resource",
                    "uri": "memory://workflow/{goal}",
                },
                "argument": {"name": "goal", "value": "asset"},
            },
        )
    )
    assert goal_completion["result"]["completion"]["values"] == ["asset_pack"]
    prompt = asyncio.run(
        server.prompt_publish_asset_pack(
            target_path="/skills/example.md", asset_kind="skill", version="1.1.0"
        )
    )
    assert "memory://workflow/asset_pack" in prompt
    assert '"zip_base64": "<base64 ZIP bytes>"' in prompt
    assert "canonical UTF-8/LF text with no trailing whitespace or final newline" in prompt
    assert "same authenticated curator profile" in prompt
    assert "memory_asset_stage_begin" in prompt
    assert "only when the MCP request would exceed" in prompt
    execute_schema = next(
        item["inputSchema"]
        for item in server.discover_tools()["tools"]
        if item["name"] == "memory_execute"
    )
    serialized_execute_schema = json.dumps(execute_schema, sort_keys=True)
    assert "attach_asset_pack" in serialized_execute_schema
    assert "staged_asset_id" in serialized_execute_schema


def test_managed_access_instructions_and_tool_descriptions(
    service: MemoryService,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    access_store = AccessStore(service._deps.control_connection, "test-master-key")
    access_store.create(
        actor="bootstrap",
        name="onboarding-admin",
        roles=("admin", "reader"),
        read_prefixes=("/",),
        write_prefixes=(),
        idempotency_key="create-onboarding-admin",
    )
    server = MementoMCPServer(service, bearer_tokens={}, access_store=access_store)
    monkeypatch.setattr(
        "memento.server.get_request_context",
        lambda: SimpleNamespace(principal="onboarding-admin", session_id="session-admin"),
    )

    instructions = server.get_instructions()
    assert "direct access_* tools, not memory_execute operations" in instructions
    assert "administrator bearer token out of ordinary agent runtimes" in instructions
    assert "tool input or output" in instructions
    assert "separate admin profile" in instructions
    assert "credentials returned once by create or rotate" in instructions

    tools = {item["name"]: item for item in server.discover_tools()["tools"]}
    create_tool = tools["access_principal_create"]
    assert "least-privilege principal" in create_tool["description"]
    assert "MCP params.arguments" in create_tool["description"]
    for field in ("name", "roles", "read_prefixes", "write_prefixes", "idempotency_key"):
        assert f"`{field}`" in create_tool["description"]
    assert "never `memory://` resource URIs" in create_tool["description"]
    assert "separate from routine curation" in tools["access_principal_update"]["description"]
    assert "secret-store capture" in tools["access_credential_rotate"]["description"]
    assert create_tool["annotations"]["roles"] == ["admin"]

    schema = create_tool["inputSchema"]
    assert schema["required"] == [
        "name",
        "roles",
        "read_prefixes",
        "write_prefixes",
        "idempotency_key",
    ]
    assert schema["properties"]["name"]["pattern"] == "^[a-z0-9][a-z0-9-]{0,62}$"
    assert schema["properties"]["roles"]["items"]["enum"] == [
        "reader",
        "proposer",
        "curator",
        "admin",
    ]
    assert schema["properties"]["read_prefixes"]["items"]["pattern"] == "^/(?:.*/)?$"
    assert "not memory:// resource URIs" in schema["properties"]["write_prefixes"]["description"]

    empty_call = asyncio.run(
        server.process_request_async(
            json.dumps(
                {
                    "jsonrpc": "2.0",
                    "id": 1,
                    "method": "tools/call",
                    "params": {"name": "access_principal_create", "arguments": {}},
                }
            )
        )
    )
    assert empty_call is not None
    assert empty_call["error"]["code"] == -32602
    assert empty_call["error"]["message"] == (
        "access_principal_create requires MCP params.arguments fields: name, roles, "
        "read_prefixes, write_prefixes, idempotency_key. Prefixes are namespace paths such as "
        "'/skills/', not memory:// resource URIs."
    )

    valid_call = asyncio.run(
        server.process_request_async(
            json.dumps(
                {
                    "jsonrpc": "2.0",
                    "id": 2,
                    "method": "tools/call",
                    "params": {
                        "name": "access_principal_create",
                        "arguments": {
                            "name": "codex-probe",
                            "roles": ["reader"],
                            "read_prefixes": ["/codex-probe/"],
                            "write_prefixes": [],
                            "idempotency_key": "codex-probe-create",
                        },
                    },
                }
            )
        )
    )
    assert valid_call is not None
    assert valid_call["result"]["structuredContent"]["principal"]["name"] == "codex-probe"
    assert valid_call["result"]["structuredContent"]["principal"]["read_prefixes"] == [
        "/codex-probe/"
    ]
    assert valid_call["result"]["structuredContent"]["credential"].startswith("memento_")


def test_mcp_asset_stage_ticket_and_status_bridge(
    service: MemoryService,
    service_config: ServiceConfig,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    store = StagedAssetStore(service._deps.control_connection)
    staged_service = MemoryService(
        ServiceDependencies(
            config=service_config,
            repo_paths=service._deps.repo_paths,
            control_connection=service._deps.control_connection,
            derived_index=service._deps.derived_index,
            transaction_manager=service._deps.transaction_manager,
            staged_asset_store=store,
        )
    )
    server = MementoMCPServer(
        staged_service,
        bearer_tokens={
            "smith-token": Principal(name="smith", roles=("curator", "proposer", "reader"))
        },
    )
    monkeypatch.setattr(
        "memento.server.get_request_context",
        lambda: SimpleNamespace(principal="smith", session_id="session-1"),
    )
    begun = asyncio.run(
        server.tool_memory_asset_stage_begin(
            asset_kind="templates",
            version="1.0.0",
            idempotency_key="mcp-ticket-1",
        )
    )
    assert begun["status"] == "success"
    assert begun["data"]["upload_path"] == "/assets/staging/upload"
    assert begun["data"]["upload_ticket_header"] == "X-Memento-Upload-Ticket"
    assert begun["data"]["workflow"] == "memory://workflow/asset_pack"
    assert begun["data"]["proposal_contract"] == "memory://catalog/propose"
    assert begun["next_tools"] == [
        "memory_asset_stage_status",
        "memory://workflow/asset_pack",
        "memory://catalog/propose",
        "memory_execute",
    ]
    raw_token = begun["data"]["upload_ticket"]
    assert raw_token.startswith("memento_upload_")
    pending = asyncio.run(server.tool_memory_asset_stage_status("mcp-ticket-1"))
    assert pending["data"]["state"] == "pending"
    staged, _ = store.put_with_ticket(raw_token=raw_token, zip_bytes=_skill_zip("# Template\n")[1])
    uploaded = asyncio.run(server.tool_memory_asset_stage_status("mcp-ticket-1"))
    assert uploaded["data"]["state"] == "uploaded"
    assert uploaded["data"]["staged_asset_id"] == staged.staged_asset_id
    assert uploaded["data"]["staged_asset"]["sha256"] == staged.sha256
    assert "memory://workflow/asset_pack" in uploaded["next_tools"]
    assert "memory://catalog/propose" in uploaded["next_tools"]
    assert "memory_execute" in uploaded["next_tools"]


def test_asset_pack_tool_discovery_and_catalog_schemas(
    service: MemoryService,
    service_config: ServiceConfig,
) -> None:
    standard_server = _server_for(
        service,
        service_config.model_copy(update={"mcp": MCPConfig(tool_surface="standard")}),
    )
    standard_tools = {item["name"]: item for item in standard_server.discover_tools()["tools"]}
    assert "memory_asset_get" in standard_tools
    assert "memory_asset_prune" in standard_tools
    assert "memory_execute" not in standard_tools
    assert standard_tools["memory_asset_get"]["annotations"] == {
        "roles": ["reader"],
        "operation": "asset_get",
    }
    propose_entry = json.loads(
        asyncio.run(standard_server.resource_template_catalog("propose"))["text"]
    )
    changes_schema = propose_entry["input_schema"]["properties"]["changes"]
    assert changes_schema["type"] == "array"
    assert "anyOf" in changes_schema["items"]
    assert set(standard_tools["memory_asset_prune"]["inputSchema"]["required"]) == {
        "id_or_path",
        "asset_kind",
        "expected_revision",
        "idempotency_key",
    }
    assert set(standard_tools["memory_asset_prune"]["inputSchema"]["required"]) == {
        "id_or_path",
        "asset_kind",
        "expected_revision",
        "idempotency_key",
    }

    read_only_server = _server_for(
        service,
        service_config.model_copy(update={"mcp": MCPConfig(tool_surface="read_only")}),
    )
    read_only_tools = {item["name"] for item in read_only_server.discover_tools()["tools"]}
    assert "memory_asset_get" in read_only_tools
    assert "memory_asset_prune" not in read_only_tools

    catalog = json.loads(asyncio.run(standard_server.resource_catalog())["text"])
    asset_ops = {item["operation"]: item for item in catalog["operations"]}
    assert asset_ops["asset_get"]["tool"] == "memory_asset_get"
    assert asset_ops["asset_get"]["commit_capable"] is False
    assert asset_ops["asset_prune"]["commit_capable"] is True

    asset_pack_workflow = json.loads(
        asyncio.run(standard_server.resource_template_workflow("asset_pack"))["text"]
    )
    assert [item["operation"] for item in asset_pack_workflow["operations"]] == [
        "search",
        "read",
        "propose",
        "proposal_get",
        "proposal_review",
        "proposal_apply",
        "asset_get",
        "asset_stage_begin",
        "asset_stage_status",
        "asset_prune",
    ]


def _skill_zip(skill_md: str, script: str = "console.log('ok')\n") -> tuple[str, bytes]:
    stream = io.BytesIO()
    with zipfile.ZipFile(stream, "w", compression=zipfile.ZIP_DEFLATED) as archive:
        archive.writestr("SKILL.md", skill_md)
        archive.writestr("scripts/run.ts", script)
    data = stream.getvalue()
    return base64.b64encode(data).decode("ascii"), data


def test_staged_skill_asset_is_consumed_by_proposal(
    service: MemoryService,
    repo_paths: GitRepositoryPaths,
    flint: ServiceContext,
) -> None:
    store = StagedAssetStore(service._deps.control_connection)
    staged_service = MemoryService(
        ServiceDependencies(
            config=service._deps.config,
            repo_paths=service._deps.repo_paths,
            control_connection=service._deps.control_connection,
            derived_index=service._deps.derived_index,
            transaction_manager=service._deps.transaction_manager,
            staged_asset_store=store,
        )
    )
    skill_md = "---\nname: staged-skill\ndescription: Staged\n---\n# Staged Skill"
    _encoded, zip_bytes = _skill_zip(skill_md)
    staged, _ = store.put(
        principal="flint",
        idempotency_key="staged-skill-upload-1",
        asset_kind="skill",
        version="1.0.0",
        zip_bytes=zip_bytes,
    )
    proposed = staged_service.memory_propose(
        flint,
        intent="Share staged skill",
        base_revision=get_main_revision(repo_paths),
        changes=[
            {
                "kind": "create",
                "path": "/skills/staged-skill.md",
                "concept_type": "project",
                "title": "Staged Skill",
                "tags": ["skill"],
                "body": skill_md,
            },
            {
                "kind": "attach_asset_pack",
                "path": "/skills/staged-skill.md",
                "asset_kind": "skill",
                "version": "1.0.0",
                "staged_asset_id": staged.staged_asset_id,
            },
        ],
    )
    proposal = success_data(proposed)["proposal"]
    assert "staged_asset_id" not in proposal["changes"][1]
    consumed = store.get(principal="flint", staged_asset_id=staged.staged_asset_id)
    assert consumed.state == "consumed"
    assert consumed.proposal_id == proposal["proposal_id"]
    replay = staged_service.memory_propose(
        flint,
        intent="Reuse staged skill",
        base_revision=get_main_revision(repo_paths),
        changes=[
            {
                "kind": "attach_asset_pack",
                "path": "/skills/staged-skill.md",
                "asset_kind": "skill",
                "version": "1.0.0",
                "staged_asset_id": staged.staged_asset_id,
            }
        ],
    )
    assert replay.status == "error"
    assert "not ready" in replay.message


@pytest.mark.parametrize(
    ("body", "root_skill_md", "message"),
    [
        ("# Demo\n", "# Demo\n", "canonical UTF-8 text"),
        ("# Demo\r\nBody", "# Demo\r\nBody", "canonical UTF-8 text"),
        ("# Demo\rBody", "# Demo\rBody", "canonical UTF-8 text"),
        ("# Demo\nBody  ", "# Demo\nBody  ", "canonical UTF-8 text"),
        ("# Demo\nBody", "# Demo\r\nBody", "exactly match"),
    ],
)
def test_skill_asset_proposal_rejects_noncanonical_or_mismatched_root(
    service: MemoryService,
    repo_paths: GitRepositoryPaths,
    smith: ServiceContext,
    body: str,
    root_skill_md: str,
    message: str,
) -> None:
    encoded, _zip_bytes = _skill_zip(root_skill_md)
    result = service.memory_propose(
        smith,
        intent="Reject non-canonical skill",
        base_revision=get_main_revision(repo_paths),
        changes=[
            {
                "kind": "create",
                "path": "/skills/canonical-skill.md",
                "concept_type": "project",
                "title": "Canonical Skill",
                "tags": ["skill"],
                "body": body,
            },
            {
                "kind": "attach_asset_pack",
                "path": "/skills/canonical-skill.md",
                "asset_kind": "skill",
                "version": "1.0.0",
                "zip_base64": encoded,
            },
        ],
    )

    assert result.status == "error"
    assert message in result.message


def test_asset_pack_skill_lifecycle_uses_generic_propose_review_apply_and_get(
    service: MemoryService,
    repo_paths: GitRepositoryPaths,
    smith: ServiceContext,
    flint: ServiceContext,
) -> None:
    skill_md = "---\nname: demo-skill\ndescription: Demo\n---\n# Demo Skill\n\nCafe\u0301"
    encoded, zip_bytes = _skill_zip(skill_md)
    base_revision = get_main_revision(repo_paths)
    proposed = service.memory_propose(
        smith,
        intent="Share complete skill",
        base_revision=base_revision,
        rationale="share complete skill",
        changes=[
            {
                "kind": "create",
                "path": "/skills/demo-skill.md",
                "concept_type": "project",
                "title": "Demo Skill",
                "description": "Demo",
                "tags": ["skill"],
                "body": skill_md,
            },
            {
                "kind": "attach_asset_pack",
                "path": "/skills/demo-skill.md",
                "asset_kind": "skill",
                "version": "1.0.0",
                "zip_base64": encoded,
            },
        ],
    )
    proposal = success_data(proposed)["proposal"]
    assert proposal["status"] == "submitted"
    assert "zip_base64" not in proposal["changes"][1]
    assert proposal["changes"][1]["asset_kind"] == "skill"
    assert proposal["changes"][1]["version"] == "1.0.0"
    assert proposal["changes"][1]["zip_sha256"] == proposal["changes"][1]["manifest"]["sha256"]
    assert {entry["path"] for entry in proposal["changes"][1]["manifest"]["entries"]} == {
        "SKILL.md",
        "scripts/run.ts",
    }

    approved = service.memory_proposal_review(
        smith,
        proposal_id=proposal["proposal_id"],
        decision="approve",
        comment="validated",
    )
    approved_proposal = success_data(approved)["proposal"]
    assert approved_proposal["status"] == "approved"
    assert approved_proposal["author_principal"] == "smith"
    assert approved_proposal["reviewed_by"] == "smith"

    applied = service.memory_proposal_apply(
        smith,
        proposal_id=proposal["proposal_id"],
        expected_revision=base_revision,
        idempotency_key="skill-demo-1",
    )
    assert applied.status == "success", applied.model_dump(mode="python")
    applied_data = success_data(applied)
    assert applied_data["proposal"]["status"] == "applied"
    assert "/skills/demo-skill.md" in applied_data["changed_paths"]
    assert any(
        path.startswith("/.assets/") and path.endswith("/skill/1.0.0.zip")
        for path in applied_data["changed_paths"]
    )
    replayed = success_data(
        service.memory_proposal_apply(
            smith,
            proposal_id=proposal["proposal_id"],
            expected_revision=base_revision,
            idempotency_key="skill-demo-1",
        )
    )
    assert replayed["replayed"] is True
    assert replayed["changed_paths"] == applied_data["changed_paths"]

    searched = success_data(service.memory_search(smith, query="Demo Skill"))
    assert [item["path"] for item in searched["results"]] == ["/skills/demo-skill.md"]
    read_back = success_data(service.memory_read(smith, id_or_path="/skills/demo-skill.md"))
    assert read_back["frontmatter"]["title"] == "Demo Skill"
    assert "skill" in read_back["frontmatter"]["tags"]
    recalled = success_data(
        service.memory_asset_get(smith, id_or_path="/skills/demo-skill.md", asset_kind="skill")
    )
    assert recalled["concept_path"] == "/skills/demo-skill.md"
    assert recalled["version"] == "1.0.0"
    assert recalled["versions"] == ["1.0.0"]
    assert recalled["zip_sha256"] == proposal["changes"][1]["zip_sha256"]
    assert recalled["manifest"] == proposal["changes"][1]["manifest"]
    recalled_zip = base64.b64decode(recalled["zip_base64"])
    assert recalled_zip == zip_bytes
    root_entry = next(
        entry for entry in recalled["manifest"]["entries"] if entry["path"] == "SKILL.md"
    )
    body_bytes = read_back["body"].encode("utf-8")
    assert body_bytes == skill_md.encode("utf-8")
    assert hashlib.sha256(body_bytes).hexdigest() == root_entry["sha256"]
    with zipfile.ZipFile(io.BytesIO(recalled_zip)) as archive:
        assert archive.read("SKILL.md") == body_bytes

    duplicate = service.memory_propose(
        flint,
        intent="Duplicate skill asset",
        base_revision=get_main_revision(repo_paths),
        changes=[
            {
                "kind": "patch",
                "path": "/skills/demo-skill.md",
                "body": skill_md,
            },
            {
                "kind": "attach_asset_pack",
                "path": "/skills/demo-skill.md",
                "asset_kind": "skill",
                "version": "1.0.0",
                "zip_base64": encoded,
            },
        ],
    )
    assert duplicate.status == "success"
    duplicate_id = success_data(duplicate)["proposal"]["proposal_id"]
    duplicate_review = service.memory_proposal_review(
        smith, proposal_id=duplicate_id, decision="approve"
    )
    assert duplicate_review.status == "success"
    duplicate_apply = service.memory_proposal_apply(
        smith,
        proposal_id=duplicate_id,
        expected_revision=get_main_revision(repo_paths),
        idempotency_key="skill-demo-duplicate",
    )
    assert duplicate_apply.status == "error"
    assert duplicate_apply.error_class == "validation_error"
    assert "accepted asset version already exists" in duplicate_apply.message


def test_generic_asset_proposal_rejects_duplicate_and_rename_mix(
    service: MemoryService,
    repo_paths: GitRepositoryPaths,
    flint: ServiceContext,
) -> None:
    encoded, _bytes = _skill_zip("# Template\n")
    changes = [
        {
            "kind": "attach_asset_pack",
            "path": "/projects/piclaw.md",
            "asset_kind": "templates",
            "version": "1.0.0",
            "zip_base64": encoded,
        }
    ]
    first = service.memory_propose(
        flint,
        intent="Attach templates",
        base_revision=get_main_revision(repo_paths),
        changes=changes,
    )
    assert first.status == "success"
    duplicate = service.memory_propose(
        flint,
        intent="Duplicate templates",
        base_revision=get_main_revision(repo_paths),
        changes=changes,
    )
    assert duplicate.status == "error"
    assert duplicate.error_class == "conflict"

    mixed = service.memory_propose(
        flint,
        intent="Rename and attach",
        base_revision=get_main_revision(repo_paths),
        changes=[
            {
                "kind": "rename",
                "path": "/projects/piclaw.md",
                "new_path": "/projects/piclaw-new.md",
            },
            *changes,
        ],
    )
    assert mixed.status == "error"
    assert "separate proposals" in mixed.message


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
        "memory_inventory",
        "memory_route",
        "memory_asset_stage_begin",
        "memory_asset_stage_status",
        "memory_asset_get",
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


@pytest.mark.parametrize("status", ["deprecated", "tombstone"])
def test_execute_applies_and_replays_status_only_patches(
    service: MemoryService,
    smith: ServiceContext,
    repo_paths: GitRepositoryPaths,
    status: str,
) -> None:
    plan = {
        "operations": [
            {
                "op": "patch",
                "args": {
                    "path": "/projects/piclaw.md",
                    "expected_revision": get_main_revision(repo_paths),
                    "idempotency_key": f"status-only-{status}",
                    "status": status,
                },
            }
        ]
    }

    first = service.memory_execute(smith, plan=plan)
    assert first.status == "success", first.model_dump(mode="python")
    assert success_data(first)["trace"][0]["status"] == "success"
    concept = success_data(service.memory_read(smith, id_or_path="/projects/piclaw.md"))
    assert concept["frontmatter"]["status"] == status

    replay = service.memory_execute(smith, plan=plan)
    assert replay.status == "success", replay.model_dump(mode="python")
    assert success_data(replay)["trace"][0]["data"]["replayed"] is True


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


def test_execute_reports_success_when_deadline_expires_after_commit(
    service: MemoryService,
    smith: ServiceContext,
    repo_paths: GitRepositoryPaths,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    ticks = iter((0.0, 0.0, 4.0))
    monkeypatch.setattr("memento.executor.monotonic", lambda: next(ticks))
    result = service.memory_execute(
        smith,
        plan={
            "operations": [
                {
                    "op": "create",
                    "args": {
                        "path": "/projects/post-commit-deadline.md",
                        "concept_type": "project",
                        "title": "Post-commit Deadline",
                        "body": "Committed before the response deadline check.\n",
                        "expected_revision": get_main_revision(repo_paths),
                        "idempotency_key": "post-commit-deadline-1",
                    },
                    "save_as": "created",
                },
                {"op": "status", "args": {}},
            ]
        },
    )
    assert result.status == "success"
    assert "memory_execute_deadline_exceeded_after_commit" in result.warnings
    data = success_data(result)
    assert data["stopped"] is True
    assert data["stop_reason"] == "deadline exceeded after committed operation"
    assert len(data["trace"]) == 1
    assert data["revisions"][0]["repo_revision"] == get_main_revision(repo_paths)
    assert data["revisions"][0]["operation_id"] is not None
    assert (repo_paths.current_dir / "projects/post-commit-deadline.md").is_file()


def test_execute_deadline_without_commit_remains_an_error(
    service: MemoryService,
    flint: ServiceContext,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    ticks = iter((0.0, 0.0, 4.0, 4.0))
    monkeypatch.setattr("memento.executor.monotonic", lambda: next(ticks))
    result = service.memory_execute(
        flint,
        plan={"operations": [{"op": "status", "args": {}}]},
    )
    assert result.status == "error"
    assert result.error_class == "validation_error"
    assert result.message == "plan exceeded configured max_time_seconds"


def test_execute_limits_auth_and_error_control(
    service: MemoryService,
    service_config: ServiceConfig,
    smith: ServiceContext,
    flint: ServiceContext,
) -> None:
    failed_saved_operation = service.memory_execute(
        flint,
        plan={
            "operations": [
                {
                    "op": "proposal_review",
                    "args": {"proposal_id": "missing", "decision": "approve"},
                    "save_as": "review",
                }
            ],
            "returns": [{"name": "review", "ref": "$review"}],
        },
    )
    assert failed_saved_operation.status == "success"
    failed_data = success_data(failed_saved_operation)
    assert failed_data["trace"][0]["status"] == "error"
    assert failed_data["trace"][0]["error_class"] == "forbidden"
    assert failed_data["returns"] == {}
    assert failed_data["stop_reason"] == "operation 1 failed"

    continued_after_failed_overwrite = service.memory_execute(
        flint,
        plan={
            "stop_on_error": False,
            "operations": [
                {"op": "status", "args": {}, "save_as": "result"},
                {
                    "op": "proposal_review",
                    "args": {"proposal_id": "missing", "decision": "approve"},
                    "save_as": "result",
                },
            ],
            "returns": [{"name": "principal", "ref": "$result.principal"}],
        },
    )
    assert success_data(continued_after_failed_overwrite)["returns"]["principal"] == "flint"

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
