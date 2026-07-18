from __future__ import annotations

from dataclasses import dataclass
from typing import Any

READ_ONLY_SURFACE = "read_only"
COMPACT_SURFACE = "compact"
STANDARD_SURFACE = "standard"
CURATOR_SURFACE = "curator"
ADMIN_SURFACE = "admin"


@dataclass(frozen=True, slots=True)
class OperationSpec:
    op_name: str
    tool_name: str
    method_name: str
    description: str
    roles: tuple[str, ...]
    discovery_surfaces: frozenset[str]
    commit_capable: bool = False
    examples: tuple[dict[str, Any], ...] = ()


OPERATION_SPECS: tuple[OperationSpec, ...] = (
    OperationSpec(
        op_name="help",
        tool_name="memory_help",
        method_name="memory_help",
        description="Discover Memento workflows, resources, and execution guidance.",
        roles=("reader",),
        discovery_surfaces=frozenset(
            {READ_ONLY_SURFACE, COMPACT_SURFACE, STANDARD_SURFACE, CURATOR_SURFACE, ADMIN_SURFACE}
        ),
    ),
    OperationSpec(
        op_name="status",
        tool_name="memory_status",
        method_name="memory_status",
        description="Read service readiness, repository revision, and configured limits.",
        roles=("reader",),
        discovery_surfaces=frozenset(
            {READ_ONLY_SURFACE, COMPACT_SURFACE, STANDARD_SURFACE, CURATOR_SURFACE, ADMIN_SURFACE}
        ),
    ),
    OperationSpec(
        op_name="search",
        tool_name="memory_search",
        method_name="memory_search",
        description="Search visible concepts by lexical or semantic query.",
        roles=("reader",),
        discovery_surfaces=frozenset(
            {READ_ONLY_SURFACE, COMPACT_SURFACE, STANDARD_SURFACE, CURATOR_SURFACE, ADMIN_SURFACE}
        ),
        examples=({"query": "Piclaw", "limit": 3},),
    ),
    OperationSpec(
        op_name="read",
        tool_name="memory_read",
        method_name="memory_read",
        description="Read one visible concept by path or concept id.",
        roles=("reader",),
        discovery_surfaces=frozenset(
            {READ_ONLY_SURFACE, COMPACT_SURFACE, STANDARD_SURFACE, CURATOR_SURFACE, ADMIN_SURFACE}
        ),
        examples=({"id_or_path": "/projects/piclaw.md"},),
    ),
    OperationSpec(
        op_name="list",
        tool_name="memory_list",
        method_name="memory_list",
        description="List visible concepts under a path prefix.",
        roles=("reader",),
        discovery_surfaces=frozenset({READ_ONLY_SURFACE, STANDARD_SURFACE, ADMIN_SURFACE}),
    ),
    OperationSpec(
        op_name="graph",
        tool_name="memory_graph",
        method_name="memory_graph",
        description="Inspect graph neighbors and backlinks for a concept.",
        roles=("reader",),
        discovery_surfaces=frozenset({READ_ONLY_SURFACE, STANDARD_SURFACE, ADMIN_SURFACE}),
    ),
    OperationSpec(
        op_name="audit",
        tool_name="memory_audit",
        method_name="memory_audit",
        description="Audit repository issues visible to the caller.",
        roles=("reader",),
        discovery_surfaces=frozenset({READ_ONLY_SURFACE, STANDARD_SURFACE, ADMIN_SURFACE}),
    ),
    OperationSpec(
        op_name="answer",
        tool_name="memory_answer",
        method_name="memory_answer",
        description="Produce a bounded answer with exact citations when enabled.",
        roles=("reader",),
        discovery_surfaces=frozenset(
            {READ_ONLY_SURFACE, COMPACT_SURFACE, STANDARD_SURFACE, CURATOR_SURFACE, ADMIN_SURFACE}
        ),
        examples=({"question": "What is Piclaw?", "answer_mode": "summary"},),
    ),
    OperationSpec(
        op_name="route",
        tool_name="memory_route",
        method_name="memory_route",
        description="Classify one shallow read request through the optional Needle router and dispatch deterministically.",
        roles=("reader",),
        discovery_surfaces=frozenset({COMPACT_SURFACE, CURATOR_SURFACE, ADMIN_SURFACE}),
        examples=({"request": "find Piclaw", "execute": True},),
    ),
    OperationSpec(
        op_name="propose",
        tool_name="memory_propose",
        method_name="memory_propose",
        description="Create a deterministic proposal from explicit changes.",
        roles=("proposer",),
        discovery_surfaces=frozenset({STANDARD_SURFACE, ADMIN_SURFACE}),
    ),
    OperationSpec(
        op_name="propose_freeform",
        tool_name="memory_propose_freeform",
        method_name="memory_propose_freeform",
        description="Draft a proposal from freeform content when model proposals are enabled.",
        roles=("proposer",),
        discovery_surfaces=frozenset({STANDARD_SURFACE, ADMIN_SURFACE}),
    ),
    OperationSpec(
        op_name="propose_update",
        tool_name="memory_propose_update",
        method_name="memory_propose_update",
        description="Draft a proposal update from an instruction when model proposals are enabled.",
        roles=("proposer",),
        discovery_surfaces=frozenset({STANDARD_SURFACE, ADMIN_SURFACE}),
    ),
    OperationSpec(
        op_name="proposal_get",
        tool_name="memory_proposal_get",
        method_name="memory_proposal_get",
        description="Read one proposal record.",
        roles=("proposer",),
        discovery_surfaces=frozenset({STANDARD_SURFACE, CURATOR_SURFACE, ADMIN_SURFACE}),
    ),
    OperationSpec(
        op_name="proposal_list",
        tool_name="memory_proposal_list",
        method_name="memory_proposal_list",
        description="List visible proposal records.",
        roles=("proposer",),
        discovery_surfaces=frozenset({STANDARD_SURFACE, CURATOR_SURFACE, ADMIN_SURFACE}),
    ),
    OperationSpec(
        op_name="proposal_review",
        tool_name="memory_proposal_review",
        method_name="memory_proposal_review",
        description="Review a proposal as curator.",
        roles=("curator",),
        discovery_surfaces=frozenset({STANDARD_SURFACE, CURATOR_SURFACE, ADMIN_SURFACE}),
    ),
    OperationSpec(
        op_name="proposal_apply",
        tool_name="memory_proposal_apply",
        method_name="memory_proposal_apply",
        description="Apply an approved proposal through the Git transaction pipeline.",
        roles=("curator",),
        discovery_surfaces=frozenset({STANDARD_SURFACE, CURATOR_SURFACE, ADMIN_SURFACE}),
        commit_capable=True,
    ),
    OperationSpec(
        op_name="asset_get",
        tool_name="memory_asset_get",
        method_name="memory_asset_get",
        description="Read one asset pack by concept path/id or asset-specific identifier and optional version.",
        roles=("reader",),
        discovery_surfaces=frozenset(
            {COMPACT_SURFACE, READ_ONLY_SURFACE, STANDARD_SURFACE, CURATOR_SURFACE, ADMIN_SURFACE}
        ),
        examples=({"id_or_path": "/skills/deploy.md", "asset_kind": "skill", "version": "1.2.0"},),
    ),
    OperationSpec(
        op_name="skill_get",
        tool_name="memory_skill_get",
        method_name="memory_skill_get",
        description="Read one skill pack by name and optional version.",
        roles=("reader",),
        discovery_surfaces=frozenset(
            {COMPACT_SURFACE, READ_ONLY_SURFACE, STANDARD_SURFACE, CURATOR_SURFACE, ADMIN_SURFACE}
        ),
        examples=({"skill_name": "deploy", "version": "1.2.0"},),
    ),
    OperationSpec(
        op_name="skill_propose",
        tool_name="memory_skill_propose",
        method_name="memory_skill_propose",
        description="Convenience wrapper that proposes a normal skill memory with a bundled asset.",
        roles=("proposer",),
        discovery_surfaces=frozenset({STANDARD_SURFACE, ADMIN_SURFACE}),
    ),
    OperationSpec(
        op_name="asset_prune",
        tool_name="memory_asset_prune",
        method_name="memory_asset_prune",
        description="Prune retained asset pack versions as curator.",
        roles=("curator",),
        discovery_surfaces=frozenset({STANDARD_SURFACE, CURATOR_SURFACE, ADMIN_SURFACE}),
        commit_capable=True,
    ),
    OperationSpec(
        op_name="skill_prune",
        tool_name="memory_skill_prune",
        method_name="memory_skill_prune",
        description="Prune retained skill pack versions as curator.",
        roles=("curator",),
        discovery_surfaces=frozenset({STANDARD_SURFACE, CURATOR_SURFACE, ADMIN_SURFACE}),
        commit_capable=True,
    ),
    OperationSpec(
        op_name="create",
        tool_name="memory_create",
        method_name="memory_create",
        description="Create a concept directly as curator.",
        roles=("curator",),
        discovery_surfaces=frozenset({STANDARD_SURFACE, ADMIN_SURFACE}),
        commit_capable=True,
    ),
    OperationSpec(
        op_name="patch",
        tool_name="memory_patch",
        method_name="memory_patch",
        description="Patch a concept directly as curator.",
        roles=("curator",),
        discovery_surfaces=frozenset({STANDARD_SURFACE, ADMIN_SURFACE}),
        commit_capable=True,
    ),
    OperationSpec(
        op_name="rename",
        tool_name="memory_rename",
        method_name="memory_rename",
        description="Rename a concept directly as curator.",
        roles=("curator",),
        discovery_surfaces=frozenset({STANDARD_SURFACE, ADMIN_SURFACE}),
        commit_capable=True,
    ),
    OperationSpec(
        op_name="execute",
        tool_name="memory_execute",
        method_name="memory_execute",
        description="Execute a bounded declarative multi-step memory plan.",
        roles=("reader",),
        discovery_surfaces=frozenset({COMPACT_SURFACE, CURATOR_SURFACE, ADMIN_SURFACE}),
        examples=(
            {
                "plan": {
                    "operations": [
                        {"op": "search", "args": {"query": "Piclaw"}, "save_as": "hits"},
                        {
                            "op": "read",
                            "args": {"id_or_path": "$hits.results.0.path"},
                            "save_as": "doc",
                        },
                    ],
                    "returns": [{"name": "title", "ref": "$doc.frontmatter.title"}],
                }
            },
        ),
    ),
)

OPERATION_SPEC_BY_TOOL = {item.tool_name: item for item in OPERATION_SPECS}
OPERATION_SPEC_BY_OP = {item.op_name: item for item in OPERATION_SPECS}

WORKFLOW_TEMPLATES: dict[str, dict[str, Any]] = {
    "inspect": {
        "description": "Find relevant concepts, then read exact paths returned from search.",
        "operations": ["search", "read"],
    },
    "propose": {
        "description": "Search and read context, then submit a proposal instead of writing directly.",
        "operations": ["search", "read", "propose", "propose_freeform", "propose_update"],
    },
    "curate": {
        "description": "Review or apply approved proposals, and use direct mutations only in admin mode.",
        "operations": [
            "proposal_list",
            "proposal_get",
            "proposal_review",
            "proposal_apply",
            "asset_prune",
            "skill_prune",
            "create",
            "patch",
            "rename",
        ],
    },
    "skill_pack": {
        "description": "Discover, inspect, propose, review, apply, and prune versioned skill packs.",
        "operations": [
            "search",
            "asset_get",
            "skill_get",
            "skill_propose",
            "asset_prune",
            "skill_prune",
        ],
    },
}


def tool_names_for_surface(
    surface: str, *, answer_enabled: bool, route_enabled: bool = False
) -> tuple[str, ...]:
    names: list[str] = []
    for spec in OPERATION_SPECS:
        if surface not in spec.discovery_surfaces:
            continue
        if (
            spec.tool_name == "memory_answer"
            and not answer_enabled
            and surface in {COMPACT_SURFACE, CURATOR_SURFACE}
        ):
            continue
        if spec.tool_name == "memory_route" and not route_enabled:
            continue
        names.append(spec.tool_name)
    return tuple(names)
