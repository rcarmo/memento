from __future__ import annotations

from typing import cast

import pytest
from pydantic import ValidationError

from memento.executor import ExecutePlan, GraphOperation, ReadOperation, SearchOperation
from memento.router import (
    READ_FIELDS,
    ROUTER_ACTION_ADAPTER,
    STATUS_FIELDS,
    RouterAction,
    SearchMode,
    SearchPathsAction,
    SearchThenGraphAction,
    expand_router_action,
)


def _parse(payload: dict[str, object]) -> RouterAction:
    return ROUTER_ACTION_ADAPTER.validate_python(payload)


def test_expand_search_then_read() -> None:
    action = _parse({"action": "search_then_read", "query": "Piclaw"})
    expanded = expand_router_action(action)
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
    expanded = expand_router_action(action)
    assert expanded is not None
    assert expanded.kind == "direct"
    assert expanded.tool == "memory_search"
    assert expanded.args == {"query": "Piclaw", "limit": 5, "search_mode": "semantic"}
    assert expanded.projection is not None
    assert expanded.projection.ref == "results"
    assert expanded.projection.fields == ("path",)
    assert expanded.projection.limit == 5


def test_expand_status_field() -> None:
    action = _parse({"action": "status_field", "field": "principal"})
    expanded = expand_router_action(action)
    assert expanded is not None
    assert expanded.kind == "direct"
    assert expanded.tool == "memory_status"
    assert expanded.args == {}
    assert expanded.projection is not None
    assert expanded.projection.ref == "principal"


def test_expand_search_then_graph() -> None:
    action = _parse(
        {
            "action": "search_then_graph",
            "query": "Piclaw",
            "depth": 2,
            "search_mode": "hybrid",
        }
    )
    expanded = expand_router_action(action)
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


def test_expand_read_field() -> None:
    action = _parse({"action": "read_field", "id_or_path": "/projects/piclaw.md", "field": "body"})
    expanded = expand_router_action(action)
    assert expanded is not None
    assert expanded.kind == "direct"
    assert expanded.tool == "memory_read"
    assert expanded.args == {"id_or_path": "/projects/piclaw.md"}
    assert expanded.projection is not None
    assert expanded.projection.ref == "body"


def test_unknown_has_no_execution() -> None:
    action = _parse({"action": "UNKNOWN"})
    assert expand_router_action(action) is None


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
        expanded = expand_router_action(action)
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

    a = expand_router_action(_parse({"action": "search_paths", "query": "Piclaw"}))
    b = expand_router_action(_parse({"action": "search_paths", "query": "Piclaw"}))
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
        "readiness": {},
    }
    read_payload = {
        "path": "/projects/piclaw.md",
        "frontmatter": {"title": "Piclaw"},
        "body": "# Piclaw",
    }
    assert set(STATUS_FIELDS) == set(status_payload)
    assert set(READ_FIELDS) == set(read_payload)


def test_injection_like_strings_remain_data_not_code() -> None:
    payload = '"}; $doc.path; ${evil}; ../../etc/passwd'
    expanded = expand_router_action(
        _parse({"action": "search_then_read", "query": payload, "search_mode": "semantic"})
    )
    assert expanded is not None
    plan = ExecutePlan.model_validate(expanded.args["plan"])
    first = cast(SearchOperation, plan.operations[0])
    second = cast(ReadOperation, plan.operations[1])
    assert first.args.query == payload
    assert second.args.id_or_path == "$hits.results.0.path"
