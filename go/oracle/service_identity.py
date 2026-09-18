"""Synthetic transport identity and managed/static policy precedence."""

from __future__ import annotations

import importlib
from dataclasses import asdict
from types import SimpleNamespace
from typing import Any
from unittest.mock import patch


def fixtures() -> dict[str, Any]:
    server: Any = importlib.import_module("memento.server")
    service: Any = importlib.import_module("memento.service")
    config: Any = importlib.import_module("memento.config")
    author = config.Principal(
        name="author", roles=("reader", "proposer"), metadata={"synthetic": "static"}
    )
    admin = config.Principal(name="admin", roles=("admin",), metadata={})
    authorization = config.AuthorizationConfig(
        principals={
            "author": config.NamespacePolicy(
                roles=("reader", "proposer"),
                read_prefixes=("/public/",),
                write_prefixes=("/public/",),
                token_env="SYNTHETIC",
            ),
            "admin": config.NamespacePolicy(
                roles=("admin",),
                read_prefixes=("/",),
                write_prefixes=(),
                token_env="SYNTHETIC_ADMIN",
            ),
        },
        protected_read_prefixes=("/private/",),
    )
    managed_policy = config.NamespacePolicy(
        roles=("reader",),
        read_prefixes=("/managed/",),
        write_prefixes=(),
        token_env="MANAGED_ACCESS",
    )
    runtime = object.__new__(server.MementoMCPServer)
    runtime._bearer_tokens = {"token": author, "admin-token": admin, "": author}
    runtime._principals_by_name = {"author": author, "admin": admin}
    runtime._activity = SimpleNamespace(touch=lambda: None)
    cases = []
    for managed in [False, True]:
        store = (
            SimpleNamespace(
                authenticate=lambda token: (
                    config.Principal(name="dynamic", roles=("reader",))
                    if token == "managed-token"
                    else None
                ),
                policy=lambda name: managed_policy if name in {"author", "dynamic"} else None,
            )
            if managed
            else None
        )
        runtime._access_store = store
        memory = object.__new__(service.MemoryService)
        memory._deps = SimpleNamespace(
            access_store=store, config=SimpleNamespace(authorization=authorization)
        )
        for headers in [
            {},
            {"Authorization": "Bearer token"},
            {"authorization": "bearer token"},
            {"authorization": "Bearer token"},
            {"authorization": "Bearer token "},
            {"authorization": "Bearer  token"},
            {"authorization": "Bearer "},
            {"authorization": "Bearer admin-token"},
            {"authorization": "Bearer managed-token"},
        ]:
            principal = runtime._authenticate_headers(headers)
            cases.append(
                {
                    "kind": "authenticate",
                    "managed": managed,
                    "headers": headers,
                    "expected": principal.model_dump(mode="json") if principal else None,
                }
            )
        for name in [None, "author", "admin", "dynamic", "missing"]:
            item: dict[str, Any] = {"kind": "context", "managed": managed, "name": name}
            with patch.object(
                server,
                "get_request_context",
                lambda name=name: SimpleNamespace(
                    principal=name,
                    session_id="session",
                    client_instance_id="untrusted",
                    source_chat="untrusted",
                ),
            ):
                try:
                    context = runtime._context()
                    item["context"] = {
                        **asdict(context),
                        "principal": context.principal.model_dump(mode="json"),
                    }
                    item["policy"] = asdict(memory._policy(context))
                except RuntimeError as exc:
                    item["error"] = str(exc)
            cases.append(item)
    authorize = []
    for principal in [None, author, admin]:
        for tool in [
            None,
            "memory_proposal_apply",
            "access_principal_list",
            "access_",
            "Access_principal_list",
        ]:
            authorize.append(
                {
                    "principal": principal.model_dump(mode="json") if principal else None,
                    "tool": tool,
                    "expected": runtime.authorize_request(
                        principal, rpc_method="tools/call", tool_name=tool
                    ),
                }
            )
    return {
        "authorization": authorization.model_dump(mode="json"),
        "managed_policy": managed_policy.model_dump(mode="json"),
        "cases": cases,
        "authorize": authorize,
    }
