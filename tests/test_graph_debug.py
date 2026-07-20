from __future__ import annotations

import json

import pytest
from pydantic import ValidationError
from umcp_shared import MCPHTTPResponse  # type: ignore[import-not-found]

from memento.config import GraphExplorerConfig
from memento.graph_debug import GraphDebugHTTPHandler


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


def test_graph_route_prefix_is_strict_and_configurable() -> None:
    for value in ("graph", "/", "/graph/", "/graph?x", "/graph#x", "/graph/../x"):
        with pytest.raises(ValidationError):
            GraphExplorerConfig(route_prefix=value)
    handler = GraphDebugHTTPHandler(GraphExplorerConfig(enabled=True, route_prefix="/debug-graph"))
    assert request(handler, "/graph") is None
    page = request(handler, "/debug-graph")
    assert page is not None and page.status == 200
