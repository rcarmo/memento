"""Capture BaseHTTPRequestHandler framing used by pinned synchronous uMCP.

The interpreter identity is recorded because this behaviour lives in stdlib,
not the uMCP revision. Synthetic byte streams never contact production.
"""

from __future__ import annotations

import http.server
import importlib
import io
import json
import logging
import platform
from typing import Any

from umcp_raw_http import scenarios as async_scenarios


def scenarios() -> list[dict[str, Any]]:
    cases = async_scenarios()

    def add(name: str, raw: bytes) -> None:
        cases.append({"name": name, "prefix": list(raw), "repeat": [], "count": 0, "suffix": []})

    for request in [
        b"\n",
        b"GET\n",
        b"GET /probe\n",
        b"POST /probe\n",
        b"HEAD /probe\n",
        b"GET / x HTTP/1.1\n",
        b"GET /\xff HTTP/1.1\n",
        b"GET //example/path?q=x HTTP/1.1\n",
    ]:
        add("request-" + repr(request), request + b"Host: localhost\n\n")
    for version in [
        b"HTTP/01.01",
        b"HTTP/1.2",
        b"HTTP/0.9",
        b"HTTP/0.8",
        b"HTTP/00000000001.1",
        b"HTTP/1.\xb2",
        b"HTTP/1.x",
        b"XYZ",
        b"HTTP/2.0",
        b"HTTP/1.99",
        b"HTTP/1",
        b"HTTP/.1",
    ]:
        add("version-" + version.decode("latin1"), b"GET / " + version + b"\nHost: localhost\n\n")
    for name, header in [
        ("fold", b"X-Test: first\r\n second\r\n\tthird\r\n"),
        ("empty-then-fold", b"X-Test: first\n: missing\n continuation\n"),
        ("from-then-fold", b"X-Test: first\nFrom someone\n continuation\n"),
        ("empty-before-new", b"X-Test: first\n: missing\nX-Next: next\n"),
        ("initial-fold", b" continuation\nHost: localhost\n"),
        ("unix-from", b"From someone\nHost: localhost\n"),
        ("from-middle", b"Host: localhost\nFrom someone\nX-Test: a\n"),
        ("from-last", b"Host: localhost\nFrom someone\n"),
        ("bad-stops", b"Bad\nHost: localhost\n"),
        ("bare-cr", b"Host: localhost\rX-Test: value\r"),
        ("whitespace-value", b"X-Test: \t x  \t\n"),
        ("expect", b"Host: localhost\nExpect: 100-continue\n"),
        ("connection-close", b"Connection: close\n"),
        ("connection-list", b"Connection: close, keep-alive\n"),
        ("connection-duplicate", b"Connection: keep-alive\nConnection: close\n"),
        ("duplicate-values", b"X-Test: first\nX-Test: last\n"),
    ]:
        add(name, b"GET /probe HTTP/1.1\n" + header + b"\nTAIL")
    for name, raw in [
        ("unknown-method", b"PUT /probe HTTP/1.1\nHost: localhost\n\n"),
        ("head-method", b"HEAD /probe HTTP/1.1\nHost: localhost\n\n"),
        ("method-escape", b"<PUT> /probe HTTP/1.1\nHost: localhost\n\n"),
        ("mcp-get-no-body-read", b"GET /mcp HTTP/1.1\nHost: localhost\nContent-Length: 5\n\n"),
        (
            "mcp-post-short",
            b"POST /mcp HTTP/1.1\nHost: localhost\nContent-Length: 5\nContent-Type: application/json\n\n{}",
        ),
        (
            "mcp-json",
            b"POST /mcp HTTP/1.1\nHost: localhost\nContent-Length: 2\nContent-Type: application/json\n\n{}",
        ),
        ("empty-host", b"GET /probe HTTP/1.1\nHost:\n\n"),
    ]:
        add(name, raw)
    return cases


class Handler(http.server.BaseHTTPRequestHandler):
    def __init__(self, raw: bytes, legacy: bool) -> None:
        self.protocol_version = "HTTP/1.0" if legacy else "HTTP/1.1"
        self.rfile = io.BytesIO(raw)
        self.wfile = io.BytesIO()
        self.errors: list[dict[str, Any]] = []
        self.continue_sent = False

    def log_message(self, format: str, *args: Any) -> None:
        pass

    def send_error(self, code: int, message: str | None = None, explain: str | None = None) -> None:
        short, long = self.responses.get(code, ("???", "???"))
        self.errors.append(
            {
                "status": int(code),
                "message": short if message is None else message,
                "explain": long if explain is None else explain,
                "version": self.request_version,
            }
        )

    def handle_expect_100(self) -> bool:
        self.continue_sent = True
        return True

    def capture(self) -> dict[str, Any]:
        self.raw_requestline = self.rfile.readline(65537)
        if len(self.raw_requestline) > 65536:
            self.requestline = ""
            self.request_version = ""
            self.command = ""
            self.send_error(414)
            return self.errors[0]
        if not self.raw_requestline:
            return {"status": 0}
        if not self.parse_request():
            return self.errors[0] if self.errors else {"status": 0}
        pairs = list(self.headers.items())
        counts = {
            name.lower(): sum(1 for key, _ in pairs if key.lower() == name.lower())
            for name, _ in pairs
        }
        return {
            "status": 200,
            "method": self.command,
            "target": self.path,
            "version": self.request_version,
            "first": {name.lower(): self.headers.get(name) for name, _ in pairs},
            "headers": {name.lower(): value for name, value in pairs},
            "counts": counts,
            "pairs": pairs,
            "keep_alive": not self.close_connection,
            "expect_continue": self.continue_sent,
            "remaining": list(self.rfile.read()),
        }


def fixtures() -> dict[str, Any]:
    return {
        "python": platform.python_version(),
        "cases": [
            {
                "legacy": legacy,
                "input": case,
                "expected": Handler(
                    bytes(case["prefix"])
                    + bytes(case["repeat"]) * case["count"]
                    + bytes(case["suffix"]),
                    legacy,
                ).capture(),
            }
            for legacy in [False, True]
            for case in scenarios()
        ],
    }


def wire_fixtures(shared: Any) -> dict[str, Any]:
    module: Any = importlib.import_module("umcp")

    def setup(self: Any) -> None:
        self.logger = logging.getLogger("oracle-sync-wire")
        self.logger.addHandler(logging.NullHandler())
        self.logger.propagate = False

    def route(self: Any, **kwargs: Any) -> Any:
        kwargs.pop("peer")
        kwargs["body"] = list(kwargs["body"])
        return shared.MCPHTTPResponse(200, json.dumps(kwargs).encode(), "application/json")

    cls = type(
        "SyncWireOracle",
        (module.MCPServer,),
        {"_setup_logging": setup, "handle_http_request": route},
    )
    saved = module.ThreadingHTTPServer

    class Connection:
        def settimeout(self, timeout: float) -> None:
            pass

    results = []
    for legacy in [False, True]:
        captured: list[Any] = []

        class CaptureHTTPServer:
            def __init__(self, address: Any, handler: Any, captured: list[Any] = captured) -> None:
                self.server_address = ("127.0.0.1", 12345)
                captured.append(handler)

            def serve_forever(self) -> None:
                pass

            def server_close(self) -> None:
                pass

        module.ThreadingHTTPServer = CaptureHTTPServer
        try:
            server = cls()
            server.streamable_http_max_requests_per_connection = 1
            # Source prints a listening message even for the synthetic server.
            import contextlib

            with contextlib.redirect_stdout(io.StringIO()):
                if legacy:
                    server.run_sse(max_request_bytes=1 << 20)
                else:
                    server.run_streamable_http(max_request_bytes=1 << 20)
        finally:
            module.ThreadingHTTPServer = saved
        for case in scenarios():
            handler = object.__new__(captured[0])
            handler.rfile = io.BytesIO(
                bytes(case["prefix"])
                + bytes(case["repeat"]) * case["count"]
                + bytes(case["suffix"])
            )
            handler.wfile = io.BytesIO()
            handler.client_address = ("127.0.0.1", 12345)
            handler.server = None
            handler.connection = Connection()
            handler._request_count = 0
            handler.date_time_string = lambda: "Fri, 18 Sep 2026 00:00:00 GMT"
            handler.handle_one_request()
            raw = handler.wfile.getvalue()
            version = getattr(handler, "request_version", "")
            if not raw:
                expected: dict[str, Any] = {"status": 0, "body": "", "headers": {}}
            elif version == "HTTP/0.9":
                expected = {"status": 0, "body": raw.decode(), "headers": {}}
            else:
                interim = raw.startswith(b"HTTP/1.1 100 Continue\r\n\r\n")
                if interim:
                    raw = raw.split(b"\r\n\r\n", 1)[1]
                head, _, body = raw.partition(b"\r\n\r\n")
                status = int(head.split(b" ", 2)[1])
                headers = {
                    k.decode("latin1").lower(): v.decode("latin1").strip()
                    for k, v in (line.split(b":", 1) for line in head.split(b"\r\n")[1:])
                }
                headers.pop("date", None)
                headers.pop("server", None)
                # JSON spacing is irrelevant; all HTML error bytes are exact.
                if headers.get("content-type") == "application/json":
                    headers.pop("content-length", None)
                    body_value: Any = json.loads(body)
                else:
                    body_value = body.decode()
                expected = {
                    "status": status,
                    "protocol": head.split(b" ", 1)[0].decode(),
                    "headers": headers,
                    "body": body_value,
                    "interim": interim,
                }
            results.append({"legacy": legacy, "input": case, "expected": expected})
    return {"python": platform.python_version(), "cases": results}
