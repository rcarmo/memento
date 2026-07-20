from __future__ import annotations

import json
from collections.abc import Mapping
from importlib.resources import files
from pathlib import PurePosixPath

from umcp_shared import MCPHTTPResponse  # type: ignore[import-not-found]

from memento.config import GraphExplorerConfig

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

    def __init__(self, config: GraphExplorerConfig) -> None:
        self._config = config
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
        if method != "GET":
            return MCPHTTPResponse(
                405,
                headers=((*_CACHE_HEADERS, ("Allow", "GET"))),
            )
        if body:
            return MCPHTTPResponse(400, headers=_CACHE_HEADERS)
        if path in {prefix, f"{prefix}/"}:
            return self._static("index.html")
        if path == f"{prefix}/api/v1/status":
            return self._json(
                {
                    "schema_version": 1,
                    "enabled": True,
                    "warning": "Unauthenticated visual debugger; trusted networks only.",
                    "route_prefix": prefix,
                }
            )
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

    @staticmethod
    def _json(payload: dict[str, object]) -> MCPHTTPResponse:
        body = json.dumps(payload, sort_keys=True, separators=(",", ":")).encode("utf-8")
        return MCPHTTPResponse(
            200,
            body=body,
            content_type="application/json; charset=utf-8",
            headers=_CACHE_HEADERS,
        )

    @staticmethod
    def _not_found() -> MCPHTTPResponse:
        return MCPHTTPResponse(404, headers=_CACHE_HEADERS)
