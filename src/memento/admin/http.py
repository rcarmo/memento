from __future__ import annotations

import json
from collections.abc import Mapping
from dataclasses import asdict
from importlib.resources import files
from typing import Any

from umcp_shared import MCPHTTPResponse

from memento.access import AccessError, AccessStore
from memento.config import Principal

_HEADERS = (("Cache-Control", "no-store"), ("X-Content-Type-Options", "nosniff"))


class AdminHTTPHandler:
    def __init__(self, store: AccessStore | None) -> None:
        self._store = store
        self._static_root = files("memento.admin").joinpath("static")

    def handle(
        self, *, method: str, path: str, headers: Mapping[str, str], body: bytes
    ) -> MCPHTTPResponse | None:
        if path != "/admin" and not path.startswith("/admin/"):
            return None
        if self._store is None:
            return self._json({"error": "access management is not configured"}, 503)
        if method == "GET" and path in {"/admin", "/admin/"}:
            return MCPHTTPResponse(
                200,
                body=self._static_root.joinpath("index.html").read_bytes(),
                content_type="text/html; charset=utf-8",
                headers=_HEADERS,
            )
        if method == "GET" and path == "/admin/app.js":
            return MCPHTTPResponse(
                200,
                body=self._static_root.joinpath("app.js").read_bytes(),
                content_type="text/javascript; charset=utf-8",
                headers=_HEADERS,
            )
        if not path.startswith("/admin/api/"):
            return self._json({"error": "not found"}, 404)
        actor = self._authenticate(headers)
        if actor is None or "admin" not in actor.roles:
            return self._json({"error": "admin bearer credential required"}, 401)
        try:
            payload = json.loads(body or b"{}")
            if not isinstance(payload, dict):
                raise AccessError("request body must be an object")
            if method == "GET" and path == "/admin/api/principals":
                return self._json({"principals": [asdict(item) for item in self._store.list()]})
            if method == "GET" and path == "/admin/api/activity":
                return self._json({"events": list(self._store.audit())})
            if method == "POST" and path == "/admin/api/principals":
                item, token = self._store.create(
                    actor=actor.name,
                    name=str(payload.get("name", "")),
                    roles=tuple(payload.get("roles", ())),
                    read_prefixes=tuple(payload.get("read_prefixes", ())),
                    write_prefixes=tuple(payload.get("write_prefixes", ())),
                )
                return self._json({"principal": asdict(item), "credential": token}, 201)
            prefix = "/admin/api/principals/"
            if method == "POST" and path.startswith(prefix):
                tail = path.removeprefix(prefix).split("/")
                name = tail[0]
                action = tail[1] if len(tail) > 1 else "update"
                result: dict[str, Any]
                if action == "update":
                    item = self._store.update(
                        actor=actor.name,
                        name=name,
                        roles=tuple(payload.get("roles", ())),
                        read_prefixes=tuple(payload.get("read_prefixes", ())),
                        write_prefixes=tuple(payload.get("write_prefixes", ())),
                    )
                    result = {"principal": asdict(item)}
                elif action == "rename":
                    result = {
                        "principal": asdict(
                            self._store.rename(
                                actor=actor.name,
                                name=name,
                                new_name=str(payload.get("new_name", "")),
                            )
                        )
                    }
                elif action == "disable":
                    result = {
                        "principal": asdict(
                            self._store.set_enabled(actor=actor.name, name=name, enabled=False)
                        )
                    }
                elif action == "enable":
                    result = {
                        "principal": asdict(
                            self._store.set_enabled(actor=actor.name, name=name, enabled=True)
                        )
                    }
                elif action == "rotate":
                    result = {
                        "name": name,
                        "credential": self._store.rotate(actor=actor.name, name=name),
                    }
                elif action == "revoke":
                    result = {"principal": asdict(self._store.revoke(actor=actor.name, name=name))}
                elif action == "delete":
                    result = {"principal": asdict(self._store.delete(actor=actor.name, name=name))}
                else:
                    return self._json({"error": "not found"}, 404)
                return self._json(result)
        except (AccessError, json.JSONDecodeError, TypeError) as exc:
            return self._json({"error": str(exc)}, 400)
        return self._json({"error": "not found"}, 404)

    def _authenticate(self, headers: Mapping[str, str]) -> Principal | None:
        value = headers.get("authorization", "")
        if not value.startswith("Bearer ") or self._store is None:
            return None
        return self._store.authenticate(value.removeprefix("Bearer "))

    @staticmethod
    def _json(payload: dict[str, Any], status: int = 200) -> MCPHTTPResponse:
        return MCPHTTPResponse(
            status,
            body=json.dumps(payload).encode(),
            content_type="application/json; charset=utf-8",
            headers=_HEADERS,
        )
