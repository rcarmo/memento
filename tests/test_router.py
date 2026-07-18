from __future__ import annotations

import json
from typing import cast

import pytest
from pydantic import ValidationError

from memento.executor import ExecutePlan, GraphOperation, ReadOperation, SearchOperation
from memento.router import (
    CANONICAL_TRAINED_SHALLOW_TOOLS_JSON,
    READ_FIELD_PROJECTIONS,
    READ_FIELDS,
    ROUTER_ACTION_ADAPTER,
    STATUS_FIELD_PROJECTIONS,
    STATUS_FIELDS,
    ExpandedRouterAction,
    ReadFieldAction,
    RouterAction,
    SearchMode,
    SearchPathsAction,
    SearchThenGraphAction,
    StatusFieldAction,
    expand_router_action,
    parse_needle_router_output,
)


def _expand(action: RouterAction) -> ExpandedRouterAction:
    request = getattr(action, "query", getattr(action, "id_or_path", "status request"))
    return expand_router_action(action, request=request)


def _parse(payload: dict[str, object]) -> RouterAction:
    return ROUTER_ACTION_ADAPTER.validate_python(payload)


def test_expand_search_then_read() -> None:
    action = _parse({"action": "search_then_read", "query": "Piclaw"})
    expanded = _expand(action)
    assert expanded is not None
    assert expanded.kind == "execute"
    assert expanded.tool == "memory_execute"
    plan = ExecutePlan.model_validate(expanded.args["plan"])
    first = cast(SearchOperation, plan.operations[0])
    second = cast(ReadOperation, plan.operations[1])
    assert first.op == "search"
    assert first.args.query == "Piclaw"
    assert first.args.limit == 1
    assert first.args.search_mode is None
    assert second.op == "read"
    assert second.args.id_or_path == "$hits.results.0.path"
    assert plan.returns[0].name == "document"
    assert plan.returns[0].ref == "$doc"


def test_expand_search_paths() -> None:
    action = _parse(
        {
            "action": "search_paths",
            "query": "Piclaw",
            "limit": 5,
            "search_mode": "semantic",
        }
    )
    expanded = _expand(action)
    assert expanded is not None
    assert expanded.kind == "direct"
    assert expanded.tool == "memory_search"
    assert expanded.args == {"query": "Piclaw", "limit": 5, "search_mode": "semantic"}
    assert expanded.projection is not None
    assert expanded.projection.ref == "results"
    assert expanded.projection.fields == ("path",)
    assert expanded.projection.limit == 5


@pytest.mark.parametrize(
    ("field", "expected_ref"),
    (
        ("principal", "principal"),
        ("semantic_search_ready", "readiness.semantic_search.ready"),
        ("semantic_search_model_id", "readiness.semantic_search.model_id"),
        ("semantic_search_dimensions", "readiness.semantic_search.dimensions"),
        (
            "semantic_search_embedding_revision",
            "readiness.semantic_search.embedding_revision",
        ),
        (
            "semantic_search_sqlite_vector_enabled",
            "readiness.semantic_search.sqlite_vector_enabled",
        ),
    ),
)
def test_expand_status_field(field: str, expected_ref: str) -> None:
    action = _parse({"action": "status_field", "field": field})
    expanded = _expand(action)
    assert expanded is not None
    assert expanded.kind == "direct"
    assert expanded.tool == "memory_status"
    assert expanded.args == {}
    assert expanded.projection is not None
    assert expanded.projection.ref == expected_ref


def test_expand_search_then_graph() -> None:
    action = _parse(
        {
            "action": "search_then_graph",
            "query": "Piclaw",
            "depth": 2,
            "search_mode": "hybrid",
        }
    )
    expanded = _expand(action)
    assert expanded is not None
    assert expanded.kind == "execute"
    plan = ExecutePlan.model_validate(expanded.args["plan"])
    first = cast(SearchOperation, plan.operations[0])
    second = cast(GraphOperation, plan.operations[1])
    assert first.op == "search"
    assert first.args.search_mode == "hybrid"
    assert second.op == "graph"
    assert second.args.id_or_path == "$hits.results.0.path"
    assert second.args.depth == 2
    assert plan.returns[0].ref == "$graph"


@pytest.mark.parametrize(
    ("field", "expected_ref"),
    (
        ("body", "body"),
        ("path", "path"),
        ("title", "frontmatter.title"),
        ("type", "frontmatter.type"),
        ("status", "frontmatter.status"),
        ("tags", "frontmatter.tags"),
        ("aliases", "frontmatter.aliases"),
    ),
)
def test_expand_read_field(field: str, expected_ref: str) -> None:
    action = _parse({"action": "read_field", "id_or_path": "/projects/piclaw.md", "field": field})
    expanded = _expand(action)
    assert expanded is not None
    assert expanded.kind == "direct"
    assert expanded.tool == "memory_read"
    assert expanded.args == {"id_or_path": "/projects/piclaw.md"}
    assert expanded.projection is not None
    assert expanded.projection.ref == expected_ref


def test_unknown_has_no_execution() -> None:
    action = _parse({"action": "UNKNOWN"})
    assert _expand(action) is None


def test_execute_plan_validation_for_two_step_actions() -> None:
    payloads: tuple[dict[str, object], ...] = (
        {"action": "search_then_read", "query": "Piclaw", "search_mode": "lexical"},
        {
            "action": "search_then_graph",
            "query": "Piclaw",
            "depth": 1,
            "search_mode": "semantic",
        },
    )
    for payload in payloads:
        action = _parse(payload)
        expanded = _expand(action)
        assert expanded is not None
        assert expanded.kind == "execute"
        plan = ExecutePlan.model_validate(expanded.args["plan"])
        assert plan.model_dump(mode="python") == expanded.args["plan"]


def test_invalid_and_extra_fields_are_rejected() -> None:
    with pytest.raises(ValidationError):
        _parse({"action": "search_paths", "query": "Piclaw", "limit": 4})
    with pytest.raises(ValidationError):
        _parse({"action": "search_then_graph", "query": "Piclaw", "depth": 3})
    with pytest.raises(ValidationError):
        _parse({"action": "read_field", "id_or_path": "/x", "field": "nope"})
    with pytest.raises(ValidationError):
        _parse({"action": "UNKNOWN", "extra": True})


def test_defaults_are_deterministic() -> None:
    paths_action = cast(SearchPathsAction, _parse({"action": "search_paths", "query": "Piclaw"}))
    assert paths_action.limit == 3
    assert paths_action.search_mode is SearchMode.LEXICAL

    graph_action = cast(
        SearchThenGraphAction,
        _parse({"action": "search_then_graph", "query": "Piclaw"}),
    )
    assert graph_action.depth == 1
    assert graph_action.search_mode is SearchMode.LEXICAL

    a = _expand(_parse({"action": "search_paths", "query": "Piclaw"}))
    b = _expand(_parse({"action": "search_paths", "query": "Piclaw"}))
    assert a is not None and b is not None
    assert a.model_dump(mode="python") == b.model_dump(mode="python")


def test_status_and_read_field_mappings_match_sample_payload_shapes() -> None:
    status_payload = {
        "service_version": "0.1.0",
        "schema_version": 2,
        "repo_revision": "abc",
        "index_revision": "abc",
        "index_stale": False,
        "principal": "smith",
        "visible_concepts": 2,
        "proposal_backlog": 0,
        "limits": {},
        "roles": ("reader",),
        "features": {},
        "readiness": {
            "semantic_search": {
                "ready": True,
                "model_id": "fake-384",
                "dimensions": 384,
                "embedding_revision": "rev-1",
                "sqlite_vector_enabled": True,
            }
        },
    }
    read_payload = {
        "path": "/projects/piclaw.md",
        "frontmatter": {
            "title": "Piclaw",
            "type": "service",
            "status": "active",
            "tags": ["ops"],
            "aliases": ["piclaw"],
        },
        "body": "# Piclaw",
    }
    assert set(STATUS_FIELDS) == set(STATUS_FIELD_PROJECTIONS)
    assert set(READ_FIELDS) == set(READ_FIELD_PROJECTIONS)
    assert STATUS_FIELD_PROJECTIONS["principal"] == "principal"
    assert STATUS_FIELD_PROJECTIONS["semantic_search_ready"] == "readiness.semantic_search.ready"
    assert (
        STATUS_FIELD_PROJECTIONS["semantic_search_model_id"] == "readiness.semantic_search.model_id"
    )
    assert (
        STATUS_FIELD_PROJECTIONS["semantic_search_dimensions"]
        == "readiness.semantic_search.dimensions"
    )
    assert (
        STATUS_FIELD_PROJECTIONS["semantic_search_embedding_revision"]
        == "readiness.semantic_search.embedding_revision"
    )
    assert (
        STATUS_FIELD_PROJECTIONS["semantic_search_sqlite_vector_enabled"]
        == "readiness.semantic_search.sqlite_vector_enabled"
    )
    assert READ_FIELD_PROJECTIONS["title"] == "frontmatter.title"
    assert READ_FIELD_PROJECTIONS["type"] == "frontmatter.type"
    assert READ_FIELD_PROJECTIONS["status"] == "frontmatter.status"
    assert READ_FIELD_PROJECTIONS["tags"] == "frontmatter.tags"
    assert READ_FIELD_PROJECTIONS["aliases"] == "frontmatter.aliases"
    assert READ_FIELD_PROJECTIONS["path"] == "path"
    assert READ_FIELD_PROJECTIONS["body"] == "body"
    assert set(status_payload) == {
        "service_version",
        "schema_version",
        "repo_revision",
        "index_revision",
        "index_stale",
        "principal",
        "visible_concepts",
        "proposal_backlog",
        "limits",
        "roles",
        "features",
        "readiness",
    }
    assert set(read_payload) == {"path", "frontmatter", "body"}


def test_parse_needle_router_output_normalizes_bounded_field_aliases() -> None:
    status = parse_needle_router_output('[{"name":"status_field","arguments":{"field":"indexed"}}]')
    assert status == StatusFieldAction(action="status_field", field="index_revision")
    read = parse_needle_router_output(
        '[{"name":"read_field","arguments":{"id_or_path":"x","field":"contents"}}]'
    )
    assert read == ReadFieldAction(action="read_field", id_or_path="x", field="body")


def test_parse_needle_router_output_requires_exactly_one_call() -> None:
    action = parse_needle_router_output(
        '[{"name":"search_paths","arguments":{"query":"Piclaw","limit":3}}]'
    )
    assert action.action == "search_paths"
    with pytest.raises(ValueError):
        parse_needle_router_output("[]")
    with pytest.raises(ValueError):
        parse_needle_router_output(
            '[{"name":"UNKNOWN","arguments":{}},{"name":"UNKNOWN","arguments":{}}]'
        )


def test_canonical_trained_shallow_tools_json_is_valid() -> None:
    payload = json.loads(CANONICAL_TRAINED_SHALLOW_TOOLS_JSON)
    assert [item["name"] for item in payload] == [
        "search_then_read",
        "search_paths",
        "status_field",
        "search_then_graph",
        "read_field",
        "UNKNOWN",
    ]


def test_injection_like_strings_remain_data_not_code() -> None:
    payload = '"}; $doc.path; ${evil}; ../../etc/passwd'
    expanded = _expand(
        _parse({"action": "search_then_read", "query": payload, "search_mode": "semantic"})
    )
    assert expanded is not None
    plan = ExecutePlan.model_validate(expanded.args["plan"])
    first = cast(SearchOperation, plan.operations[0])
    second = cast(ReadOperation, plan.operations[1])
    assert first.args.query == payload
    assert second.args.id_or_path == "$hits.results.0.path"
