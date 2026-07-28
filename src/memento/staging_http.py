from __future__ import annotations

import json
from collections.abc import Callable, Mapping
from urllib.parse import urlsplit

from umcp_shared import MCPHTTPResponse

from memento.config import Principal
from memento.skill_packs import MAX_ZIP_BYTES, SkillPackValidationError
from memento.staged_assets import StagedAssetError, StagedAssetStore

_HEADERS = (("Cache-Control", "no-store"), ("X-Content-Type-Options", "nosniff"))


class AssetStagingHTTPHandler:
    def __init__(
        self,
        store: StagedAssetStore,
        authenticate: Callable[[Mapping[str, str]], Principal | None],
    ) -> None:
        self._store = store
        self._authenticate = authenticate

    def handle(
        self, *, method: str, path: str, headers: Mapping[str, str], body: bytes
    ) -> MCPHTTPResponse | None:
        route = urlsplit(path)
        if route.path != "/assets/staging" and not route.path.startswith("/assets/staging/"):
            return None
        principal = self._authenticate(headers)
        if principal is None or "proposer" not in principal.roles:
            return self._json({"error": "proposer bearer credential required"}, 401)
        try:
            if method == "POST" and route.path == "/assets/staging":
                if headers.get("content-type", "").split(";", 1)[0].strip() != "application/zip":
                    return self._json({"error": "Content-Type must be application/zip"}, 415)
                if len(body) > MAX_ZIP_BYTES:
                    return self._json({"error": "ZIP archive exceeds maximum encoded size"}, 413)
                asset_kind = headers.get("x-memento-asset-kind", "")
                version = headers.get("x-memento-asset-version", "")
                staged, replayed = self._store.put(
                    principal=principal.name,
                    idempotency_key=headers.get("idempotency-key", ""),
                    asset_kind=asset_kind,
                    version=version,
                    zip_bytes=body,
                )
                return self._json(
                    {**staged.public_payload(), "replayed": replayed}, 200 if replayed else 201
                )
            if method == "GET" and route.path.startswith("/assets/staging/"):
                staged_asset_id = route.path.removeprefix("/assets/staging/")
                staged = self._store.get(principal=principal.name, staged_asset_id=staged_asset_id)
                return self._json(staged.public_payload())
        except (StagedAssetError, SkillPackValidationError, ValueError) as exc:
            return self._json({"error": str(exc)}, 400)
        return self._json({"error": "not found"}, 404)

    @staticmethod
    def _json(payload: dict[str, object], status: int = 200) -> MCPHTTPResponse:
        return MCPHTTPResponse(
            status,
            body=json.dumps(payload).encode(),
            content_type="application/json; charset=utf-8",
            headers=_HEADERS,
        )
