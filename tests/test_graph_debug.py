from __future__ import annotations

import json
from typing import cast

import pytest
from pydantic import ValidationError
from umcp_shared import MCPHTTPResponse

from memento.config import GraphExplorerConfig
from memento.graph_debug import GraphDebugHTTPHandler
from memento.graph_debug.snapshot import GraphSnapshotService


class _Payload:
    def __init__(self, value: dict[str, object]) -> None:
        self._value = value

    def model_dump_json(self) -> str:
        return json.dumps(self._value)


class _Snapshot:
    def __init__(self) -> None:
        self.cluster_id = "cluster:skills:0:abc"
        self.detail_id = "node:1"

    def expand_cluster(self, cluster_id: str) -> _Payload:
        assert cluster_id == self.cluster_id
        return _Payload({"cluster_id": cluster_id})

    def detail(self, concept_id: str) -> _Payload:
        assert concept_id == self.detail_id
        return _Payload({"node": {"id": concept_id}})


def request(
    handler: GraphDebugHTTPHandler,
    path: str,
    *,
    method: str = "GET",
    body: bytes = b"",
) -> MCPHTTPResponse | None:
    return handler.handle(method=method, path=path, headers={}, body=body, peer="127.0.0.1")


def test_disabled_graph_routes_are_indistinguishable_404s() -> None:
    handler = GraphDebugHTTPHandler(GraphExplorerConfig())
    for path in (
        "/graph",
        "/graph/",
        "/graph/assets/app.css",
        "/graph/api/v1/status",
        "/graph/missing",
    ):
        response = request(handler, path)
        assert response is not None
        assert response.status == 404
        assert response.body == b""
    assert request(handler, "/mcp") is None
    assert request(handler, "/not-graph") is None


def test_enabled_graph_boundary_serves_ui_status_and_assets() -> None:
    handler = GraphDebugHTTPHandler(GraphExplorerConfig(enabled=True))
    page = request(handler, "/graph")
    assert page is not None and page.status == 200
    assert page.content_type == "text/html; charset=utf-8"
    assert b"trusted networks only" in page.body
    status = request(handler, "/graph/api/v1/status")
    assert status is not None and status.status == 200
    payload = json.loads(status.body)
    assert payload == {
        "enabled": True,
        "route_prefix": "/graph",
        "schema_version": 1,
        "warning": "Unauthenticated visual debugger; trusted networks only.",
    }
    css = request(handler, "/graph/assets/app.css")
    assert css is not None and css.status == 200
    assert css.content_type == "text/css; charset=utf-8"
    traversal = request(handler, "/graph/assets/../index.html")
    missing = request(handler, "/graph/assets/missing.js")
    assert traversal is not None and traversal.status == 404
    assert missing is not None and missing.status == 404


def test_graph_boundary_rejects_methods_and_bodies_without_touching_mcp() -> None:
    handler = GraphDebugHTTPHandler(GraphExplorerConfig(enabled=True))
    post = request(handler, "/graph/api/v1/status", method="POST")
    assert post is not None and post.status == 405
    assert ("Allow", "GET") in post.headers
    body = request(handler, "/graph", body=b"unexpected")
    assert body is not None and body.status == 400
    assert request(handler, "/mcp", method="POST", body=b"{}") is None


def test_graph_api_decodes_url_encoded_ids() -> None:
    snapshot = _Snapshot()
    handler = GraphDebugHTTPHandler(
        GraphExplorerConfig(enabled=True), snapshot_service=cast(GraphSnapshotService, snapshot)
    )
    cluster = request(handler, "/graph/api/v1/clusters/cluster%3Askills%3A0%3Aabc")
    assert cluster is not None and cluster.status == 200
    assert json.loads(cluster.body)["cluster_id"] == snapshot.cluster_id
    detail = request(handler, "/graph/api/v1/memories/node%3A1")
    assert detail is not None and detail.status == 200
    assert json.loads(detail.body)["node"]["id"] == snapshot.detail_id


def test_graph_route_prefix_is_strict_and_configurable() -> None:
    for value in ("graph", "/", "/graph/", "/graph?x", "/graph#x", "/graph/../x"):
        with pytest.raises(ValidationError):
            GraphExplorerConfig(route_prefix=value)
    handler = GraphDebugHTTPHandler(GraphExplorerConfig(enabled=True, route_prefix="/debug-graph"))
    assert request(handler, "/graph") is None
    page = request(handler, "/debug-graph")
    assert page is not None and page.status == 200
