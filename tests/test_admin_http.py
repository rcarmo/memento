from __future__ import annotations

import json
from pathlib import Path
from typing import Any

from umcp_shared import MCPHTTPResponse

from memento.access import AccessStore
from memento.admin import AdminHTTPHandler
from memento.config import AuthorizationConfig, NamespacePolicy
from memento.control.db import connect_control_db, migrate_control_db


def handler(tmp_path: Path) -> AdminHTTPHandler:
    connection = connect_control_db(tmp_path / "control.sqlite")
    migrate_control_db(connection)
    store = AccessStore(connection, "nenhuma")
    store.bootstrap(
        AuthorizationConfig(
            principals={
                "piclaw-workspace": NamespacePolicy(
                    roles=("reader", "proposer", "curator"),
                    token_env="TOKEN",
                    read_prefixes=("/",),
                    write_prefixes=("/projects/",),
                ),
                "reader": NamespacePolicy(
                    roles=("reader",),
                    token_env="READER",
                    read_prefixes=("/skills/",),
                ),
                "broad-reader": NamespacePolicy(
                    roles=("reader",),
                    token_env="BROAD_READER",
                    read_prefixes=("/",),
                ),
            },
            protected_read_prefixes=("/personal/",),
        ),
        {
            "piclaw-workspace": "admin-token",
            "reader": "reader-token",
            "broad-reader": "broad-reader-token",
        },
    )
    return AdminHTTPHandler(store, protected_read_prefixes=("/personal/",))


def request(
    http: AdminHTTPHandler,
    method: str,
    path: str,
    token: str = "",
    body: dict[str, Any] | None = None,
) -> tuple[MCPHTTPResponse, dict[str, Any] | None]:
    response = http.handle(
        method=method,
        path=path,
        headers={"authorization": f"Bearer {token}"} if token else {},
        body=json.dumps(body or {}).encode(),
    )
    assert response is not None
    return response, json.loads(response.body) if response.content_type.startswith(
        "application/json"
    ) else None


def test_admin_page_and_api_auth(tmp_path: Path) -> None:
    http = handler(tmp_path)
    page, _ = request(http, "GET", "/admin")
    assert page.status == 200
    assert b"Memento Access" in page.body
    unauthorized, _ = request(http, "GET", "/admin/api/principals", "reader-token")
    assert unauthorized.status == 401
    allowed, payload = request(http, "GET", "/admin/api/principals", "admin-token")
    assert allowed.status == 200
    assert payload is not None
    assert any(item["name"] == "sandbox" for item in payload["principals"])
    broad_reader = next(item for item in payload["principals"] if item["name"] == "broad-reader")
    assert "explicit read prefixes" in broad_reader["warnings"][0]
    sandbox = next(item for item in payload["principals"] if item["name"] == "sandbox")
    assert "warnings" not in sandbox


def test_admin_create_returns_one_time_credential(tmp_path: Path) -> None:
    http = handler(tmp_path)
    response, payload = request(
        http,
        "POST",
        "/admin/api/principals",
        "admin-token",
        {
            "name": "gates",
            "roles": ["reader", "proposer"],
            "read_prefixes": ["/work/", "/skills/", "/public/"],
            "write_prefixes": ["/work/"],
        },
    )
    assert response.status == 201
    assert payload is not None
    assert payload["principal"]["name"] == "gates"
    assert payload["credential"].startswith("memento_")
    listed, list_payload = request(http, "GET", "/admin/api/principals", "admin-token")
    assert listed.status == 200
    assert list_payload is not None
    assert "credential" not in next(
        item for item in list_payload["principals"] if item["name"] == "gates"
    )
