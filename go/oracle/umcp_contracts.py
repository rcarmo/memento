"""Synthetic HTTP-helper and request-dispatch outputs from both uMCP bases."""

from __future__ import annotations

import asyncio
import importlib
import json
import logging
from typing import Any


def shared_fixtures(shared: Any) -> dict[str, Any]:
    accepts = [
        None,
        "",
        "application/json",
        "APPLICATION/JSON; charset=utf-8",
        "text/*",
        "*/*",
        "image/*",
        "text/event-stream",
        "application/json;q=0",
        "text/event-stream;q=0, application/json;q=0.5",
        "application/json;q=bad",
        "application/json; q=0;q=1",
        "application/json;q=nan",
        "application/json;q=-nan",
        "application/json;q=inf",
        "application/json;q=-inf",
        "application/json;q=1e999",
        "application/json;q=0x1p0",
        "application/json;q=1_0",
        "application/json;q=1__0",
        "application/json;q=٠.٥",
        "application/json;q=𝟘.𝟝",
        "application/json;q=0e-999",
        "text/plain",
    ]
    counts = [
        {},
        {"host": 1},
        {"host": 2},
        {"host": -1},
        {"authorization": 2, "host": 1},
        {"accept": 2, "host": 1},
        {"content-type": 2, "host": 1},
        {"x-custom": 5, "host": 1},
    ]
    headers = [
        {},
        {"host": "a,b"},
        {"accept": "a,b"},
        {"authorization": "a,b"},
        {"Host": "a,b"},
        {"host": "single"},
        {"mcp-session-id": "a,b"},
    ]
    response_inputs: list[dict[str, Any]] = [
        {"status": 200, "body": "", "content_type": None, "headers": [], "max_bytes": 0},
        {"status": 99, "body": "", "content_type": None, "headers": [], "max_bytes": 100},
        {"status": 600, "body": "", "content_type": None, "headers": [], "max_bytes": 100},
        {"status": 599, "body": "x", "content_type": None, "headers": [], "max_bytes": 1},
        {"status": 200, "body": "☃", "content_type": None, "headers": [], "max_bytes": 2},
        {"status": 200, "body": "☃", "content_type": None, "headers": [], "max_bytes": 3},
        {
            "status": 200,
            "body": "",
            "content_type": "application/json",
            "headers": [],
            "max_bytes": 28,
        },
        {
            "status": 200,
            "body": "",
            "content_type": "application/json",
            "headers": [],
            "max_bytes": 27,
        },
        {"status": 200, "body": "", "content_type": "x\r\ny", "headers": [], "max_bytes": 100},
        {
            "status": 200,
            "body": "",
            "content_type": None,
            "headers": [["x\r", "y"]],
            "max_bytes": 100,
        },
        {
            "status": 200,
            "body": "",
            "content_type": None,
            "headers": [["x", "y\n"]],
            "max_bytes": 100,
        },
        {"status": 200, "body": "", "content_type": None, "headers": [["x", "☃"]], "max_bytes": 2},
        {"status": 200, "body": "", "content_type": None, "headers": [["x", "☃"]], "max_bytes": 1},
        {"status": 200, "body": "", "content_type": None, "headers": [], "max_bytes": -1},
    ]
    for case in response_inputs:
        response = shared.MCPHTTPResponse(
            case["status"],
            case["body"].encode(),
            case["content_type"],
            tuple(tuple(h) for h in case["headers"]),
        )
        case["valid"] = (
            shared.validate_http_response(response, max_bytes=case["max_bytes"]) is not None
        )
    return {
        "accepts": [
            {
                "input": v,
                "json": shared.media_accepts_json(v),
                "sse": shared.media_accepts_event_stream(v),
            }
            for v in accepts
        ],
        "content_types": [
            {"input": v, "valid": shared.content_type_is_json(v)}
            for v in [
                None,
                "",
                " APPLICATION/JSON ; charset=utf-8",
                "text/json",
                "application/jsonp",
                "application/json",
            ]
        ],
        "counts": [
            {
                "counts": v,
                "version": version,
                "invalid": shared.has_singleton_header_violations(v, http_version=version),
            }
            for version in ["HTTP/1.1", "HTTP/1.0", "HTTP/2"]
            for v in counts
        ],
        "headers": [
            {"headers": v, "ambiguous": shared.has_ambiguous_singleton_values(v)} for v in headers
        ],
        "statuses": [{"code": v, "line": shared.http_status_line(v)} for v in range(99, 601)],
        "responses": response_inputs,
    }


def dispatch_fixtures(shared: Any) -> list[dict[str, Any]]:
    sync = importlib.import_module("umcp")
    asynchronous = importlib.import_module("aioumcp")

    def setup(self: Any) -> None:
        self.logger = logging.getLogger("oracle-no-file-output")
        self.logger.addHandler(logging.NullHandler())
        self.logger.propagate = False

    def result() -> dict[str, Any]:
        ctx = shared.get_request_context()
        return {
            "request_id": ctx.request_id,
            "progress_token": ctx.progress_token,
            "principal": ctx.principal,
            "transport": ctx.transport,
            "session_id": ctx.session_id,
            "headers": dict(ctx.headers),
        }

    def handle(self: Any, request_id: Any, params: Any) -> Any:
        if params.get("cancel"):
            raise shared.MCPRequestCancelled("Request cancelled")
        return self.create_response(request_id, {"params": params, "context": result()}, None)

    # Test only the dispatcher: the delegated method is intentionally identical.
    # Method semantics and discovery have their own later contract fixtures.
    sync_cls = type(
        "OracleSync", (sync.MCPServer,), {"_setup_logging": setup, "handle_initialize": handle}
    )
    async_cls = type(
        "OracleAsync",
        (asynchronous.AsyncMCPServer,),
        {"_setup_logging": setup, "handle_initialize": handle},
    )
    context = shared.MCPRequestContext(
        transport="streamable-http",
        principal="oracle-reader",
        session_id="session-1",
        headers={"x-test": "one"},
    )
    cases: list[Any] = [
        None,
        [],
        1,
        "text",
        {},
        {"jsonrpc": "2.0"},
        {"jsonrpc": "1.0", "id": 3, "method": "initialize"},
        {"jsonrpc": "1.0", "id": True, "params": []},
        {"jsonrpc": "2.0", "id": 1, "params": []},
        {"jsonrpc": "2.0", "id": True, "method": "initialize"},
        {"jsonrpc": "2.0", "id": 1.0, "method": "initialize"},
        {"jsonrpc": "2.0", "id": 10**40, "method": "initialize"},
        {"jsonrpc": "2.0", "id": "abc", "method": "initialize", "params": None},
        {
            "jsonrpc": "2.0",
            "id": None,
            "method": "initialize",
            "params": {"_meta": {"progressToken": "progress-1"}},
        },
        {"jsonrpc": "2.0", "method": "initialize", "params": {"_meta": {"progressToken": 0}}},
        {
            "jsonrpc": "2.0",
            "id": 4,
            "method": "initialize",
            "params": {"_meta": {"progressToken": True}},
        },
        {"jsonrpc": "2.0", "id": 4, "method": "initialize", "params": {"_meta": []}},
        {
            "jsonrpc": "2.0",
            "id": 4,
            "method": "initialize",
            "params": {"_meta": {"progressToken": None}},
        },
        {"jsonrpc": "2.0", "id": 4, "method": False},
        {"jsonrpc": "2.0", "id": 4, "method": "unknown"},
        {"jsonrpc": "2.0", "method": "unknown"},
        {"jsonrpc": "2.0", "id": 2, "result": None},
        {"jsonrpc": "2.0", "id": 2, "result": None, "error": None},
        {"jsonrpc": "2.0", "id": 2, "error": {"code": -1, "message": "x"}},
        {"jsonrpc": "2.0", "method": "notifications/initialized"},
        {"jsonrpc": "2.0", "id": 1, "method": "notifications/initialized"},
    ]
    for request_id in [None, 1]:
        cancel_ids: list[Any] = [None, False, [], "x", 10**40]
        for cancel_id in cancel_ids:
            cases.append(
                {
                    "jsonrpc": "2.0",
                    "id": request_id,
                    "method": "notifications/cancelled",
                    "params": {"requestId": cancel_id},
                }
            )
        cases.append(
            {"jsonrpc": "2.0", "id": request_id, "method": "initialize", "params": {"cancel": True}}
        )
    raw = ["", "{", "{} {}", '{"jsonrpc":"2.0","id":NaN}', *[json.dumps(c) for c in cases]]
    fixtures = []
    for text in raw:
        s = sync_cls().process_request(text, context=context)
        a = asyncio.run(async_cls().process_request_async(text, context=context))
        if s != a:
            raise AssertionError(f"sync/async divergence for {text}: {s!r} != {a!r}")
        fixtures.append({"input": text, "expected": s})
    return fixtures


def progress_fixtures(shared: Any) -> list[dict[str, Any]]:
    sync = importlib.import_module("umcp")
    asynchronous = importlib.import_module("aioumcp")
    cases: list[dict[str, Any]] = [
        {"token": None, "progress": True, "total": None, "message": None},
        {"token": "p", "progress": 0, "total": None, "message": None},
        {"token": 0, "progress": 1.5, "total": 2, "message": "hello\\x00 world"},
        {"token": "p", "progress": 1, "total": 2, "message": "界" * 4098},
        {"token": "p", "progress": 0, "total": 0, "message": "\\x00"},
        {"token": "p", "progress": True, "total": None, "message": None},
        {"token": "p", "progress": -1, "total": None, "message": None},
        {"token": "p", "progress": "1", "total": None, "message": None},
        {"token": "p", "progress": 0, "total": True, "message": None},
        {"token": "p", "progress": 2, "total": 1, "message": None},
        {"token": "p", "progress": 9007199254740993, "total": 9007199254740992, "message": None},
        {"token": "p", "progress": 0, "total": -1, "message": None},
    ]
    out = []
    for case in cases:
        expected = []
        for base in [sync.MCPServer, asynchronous.AsyncMCPServer]:
            notifications: list[Any] = []
            server = object.__new__(base)

            async def send_async(method: str, params: Any, sink: list[Any] = notifications) -> None:
                sink.append({"method": method, "params": params})

            server._send_notification = lambda method, params, sink=notifications: sink.append(
                {"method": method, "params": params}
            )
            server._send_notification_async = send_async
            token = shared.set_request_context(
                shared.MCPRequestContext(progress_token=case["token"])
            )
            runtime = shared.set_request_runtime(None)
            try:
                error = None
                try:
                    result = server.notify_progress(
                        case["progress"], case["total"], case["message"]
                    )
                    if base is asynchronous.AsyncMCPServer:
                        asyncio.run(result)
                except ValueError as exc:
                    error = str(exc)
                expected.append({"notifications": notifications, "error": error})
            finally:
                shared.reset_request_context(token)
                shared.reset_request_runtime(runtime)
        if expected[0] != expected[1]:
            raise AssertionError(f"progress sync/async mismatch: {case!r}")
        out.append({"input": case, "expected": expected[0]})
    return out
