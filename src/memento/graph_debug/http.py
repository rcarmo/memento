from __future__ import annotations

import json
from collections.abc import Mapping
from importlib.resources import files
from pathlib import PurePosixPath
from urllib.parse import unquote

from umcp_shared import MCPHTTPResponse

from memento.config import GraphExplorerConfig
from memento.graph_debug.export import export_graph_json, export_graph_svg
from memento.graph_debug.refresh import GraphEmbeddingRefreshCoordinator
from memento.graph_debug.snapshot import GraphSnapshotError, GraphSnapshotService

_CACHE_HEADERS = (("Cache-Control", "no-store"), ("X-Content-Type-Options", "nosniff"))
_STATIC_CONTENT_TYPES = {
    ".css": "text/css; charset=utf-8",
    ".html": "text/html; charset=utf-8",
    ".js": "text/javascript; charset=utf-8",
    ".json": "application/json; charset=utf-8",
    ".svg": "image/svg+xml",
}


class GraphDebugHTTPHandler:
    """Serve the explicitly enabled, unauthenticated graph debugging boundary."""

    def __init__(
        self,
        config: GraphExplorerConfig,
        *,
        snapshot_service: GraphSnapshotService | None = None,
        refresh_coordinator: GraphEmbeddingRefreshCoordinator | None = None,
    ) -> None:
        self._config = config
        self._snapshot_service = snapshot_service
        self._refresh_coordinator = refresh_coordinator
        self._static_root = files("memento.graph_debug").joinpath("static")

    def handle(
        self,
        *,
        method: str,
        path: str,
        headers: Mapping[str, str],
        body: bytes,
        peer: str | None,
    ) -> MCPHTTPResponse | None:
        del headers, peer
        prefix = self._config.route_prefix
        if path != prefix and not path.startswith(f"{prefix}/"):
            return None
        if not self._config.enabled:
            return self._not_found()
        search_path = f"{prefix}/api/v1/search"
        if method == "POST" and path == search_path:
            return self._search(body)
        refresh_path = f"{prefix}/api/v1/embeddings/refresh"
        if method == "POST" and path == refresh_path:
            return self._refresh(body)
        if method == "POST" and path in {
            f"{prefix}/api/v1/export/json",
            f"{prefix}/api/v1/export/svg",
        }:
            return self._export(path.rsplit("/", 1)[-1], body)
        if method != "GET":
            return MCPHTTPResponse(
                405,
                headers=((*_CACHE_HEADERS, ("Allow", "GET"))),
            )
        if body:
            return MCPHTTPResponse(400, headers=_CACHE_HEADERS)
        if path in {prefix, f"{prefix}/"}:
            response = self._static("index.html")
            return response.__class__(
                response.status,
                body=response.body.replace(b"__GRAPH_PREFIX__", prefix.encode("utf-8")),
                content_type=response.content_type,
                headers=response.headers,
            )
        if path == f"{prefix}/api/v1/status":
            return self._json(
                {
                    "schema_version": 1,
                    "enabled": True,
                    "warning": "Unauthenticated visual debugger; trusted networks only.",
                    "route_prefix": prefix,
                }
            )
        try:
            if path == f"{prefix}/api/v1/embeddings/status":
                if self._refresh_coordinator is None:
                    return self._json({"available": False})
                return self._json(self._refresh_coordinator.state_dict())
            if path == f"{prefix}/api/v1/overview":
                return self._snapshot_json("overview")
            cluster_prefix = f"{prefix}/api/v1/clusters/"
            if path.startswith(cluster_prefix):
                return self._snapshot_json("cluster", unquote(path.removeprefix(cluster_prefix)))
            memory_prefix = f"{prefix}/api/v1/memories/"
            if path.startswith(memory_prefix):
                return self._snapshot_json("detail", unquote(path.removeprefix(memory_prefix)))
            neighbourhood_prefix = f"{prefix}/api/v1/neighbourhood/"
            if path.startswith(neighbourhood_prefix):
                return self._snapshot_json(
                    "neighbourhood", unquote(path.removeprefix(neighbourhood_prefix))
                )
        except GraphSnapshotError as exc:
            return self._json({"error": str(exc)}, status=404)
        if path.startswith(f"{prefix}/assets/"):
            relative = path.removeprefix(f"{prefix}/assets/")
            return self._static(relative)
        return self._not_found()

    def _static(self, relative: str) -> MCPHTTPResponse:
        path = PurePosixPath(relative)
        if (
            path.is_absolute()
            or not path.parts
            or any(part in {"", ".", ".."} for part in path.parts)
        ):
            return self._not_found()
        resource = self._static_root.joinpath(*path.parts)
        try:
            if not resource.is_file():
                return self._not_found()
            body = resource.read_bytes()
        except (FileNotFoundError, OSError):
            return self._not_found()
        content_type = _STATIC_CONTENT_TYPES.get(path.suffix.casefold(), "application/octet-stream")
        return MCPHTTPResponse(200, body=body, content_type=content_type, headers=_CACHE_HEADERS)

    def _export(self, export_type: str, body: bytes) -> MCPHTTPResponse:
        if self._snapshot_service is None:
            return self._json({"error": "graph snapshot unavailable"}, status=503)
        try:
            request = json.loads(body or b"{}")
            if not isinstance(request, dict):
                raise GraphSnapshotError("export body must be an object")
            ids = request.get("concept_ids", [])
            if not isinstance(ids, list) or not all(isinstance(item, str) for item in ids):
                raise GraphSnapshotError("concept_ids must be an array of strings")
            nodes, edges, revisions = self._snapshot_service.export_selection(tuple(ids))
            if export_type == "json":
                output = export_graph_json(
                    nodes,
                    edges,
                    revisions=revisions,
                    settings=request.get("settings")
                    if isinstance(request.get("settings"), dict)
                    else None,
                )
                content_type = "application/json; charset=utf-8"
            else:
                output = export_graph_svg(nodes, edges)
                content_type = "image/svg+xml"
            return MCPHTTPResponse(
                200, body=output, content_type=content_type, headers=_CACHE_HEADERS
            )
        except (json.JSONDecodeError, UnicodeDecodeError):
            return self._json({"error": "invalid JSON"}, status=400)
        except (GraphSnapshotError, ValueError) as exc:
            return self._json({"error": str(exc)}, status=400)

    def _search(self, body: bytes) -> MCPHTTPResponse:
        if self._snapshot_service is None:
            return self._json({"error": "graph snapshot unavailable"}, status=503)
        try:
            payload = json.loads(body or b"{}")
            if not isinstance(payload, dict):
                raise GraphSnapshotError("search body must be an object")
            query = payload.get("query")
            if not isinstance(query, str):
                raise GraphSnapshotError("search query must be a string")
            return self._json(self._snapshot_service.search(query))
        except (json.JSONDecodeError, UnicodeDecodeError):
            return self._json({"error": "invalid JSON"}, status=400)
        except GraphSnapshotError as exc:
            return self._json({"error": str(exc)}, status=400)

    def _refresh(self, body: bytes) -> MCPHTTPResponse:
        if self._refresh_coordinator is None:
            return self._json({"error": "semantic embedding refresh is unavailable"}, status=503)
        try:
            payload = json.loads(body)
            if not isinstance(payload, dict):
                raise GraphSnapshotError("embedding refresh body must be an object")
            ids = payload.get("concept_ids", [])
            if not isinstance(ids, list) or not all(isinstance(item, str) for item in ids):
                raise GraphSnapshotError("concept_ids must be an array of strings")
            self._refresh_coordinator.enqueue(
                scope=str(payload.get("scope") or ""),
                concept_ids=tuple(ids),
                confirm_full=payload.get("confirm_full") is True,
            )
            return self._json(self._refresh_coordinator.state_dict(), status=202)
        except (json.JSONDecodeError, UnicodeDecodeError):
            return self._json({"error": "invalid JSON"}, status=400)
        except GraphSnapshotError as exc:
            return self._json({"error": str(exc)}, status=400)

    def _snapshot_json(self, operation: str, concept_id: str | None = None) -> MCPHTTPResponse:
        if self._snapshot_service is None:
            return self._json({"error": "graph snapshot unavailable"}, status=503)
        if operation == "overview":
            encoded = self._snapshot_service.overview().model_dump_json()
        elif operation == "cluster" and concept_id:
            encoded = self._snapshot_service.expand_cluster(concept_id).model_dump_json()
        elif operation == "detail" and concept_id:
            encoded = self._snapshot_service.detail(concept_id).model_dump_json()
        elif operation == "neighbourhood" and concept_id:
            encoded = self._snapshot_service.neighbourhood(concept_id).model_dump_json()
        else:  # pragma: no cover - internal dispatch invariant
            raise GraphSnapshotError("invalid graph snapshot operation")
        return MCPHTTPResponse(
            200,
            body=encoded.encode("utf-8"),
            content_type="application/json; charset=utf-8",
            headers=_CACHE_HEADERS,
        )

    @staticmethod
    def _json(payload: dict[str, object], *, status: int = 200) -> MCPHTTPResponse:
        body = json.dumps(payload, sort_keys=True, separators=(",", ":")).encode("utf-8")
        return MCPHTTPResponse(
            status,
            body=body,
            content_type="application/json; charset=utf-8",
            headers=_CACHE_HEADERS,
        )

    @staticmethod
    def _not_found() -> MCPHTTPResponse:
        return MCPHTTPResponse(404, headers=_CACHE_HEADERS)
