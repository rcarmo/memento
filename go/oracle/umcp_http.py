"""Run local pinned Python sync/async servers and capture HTTP wire scenarios."""

from __future__ import annotations

import http.client
import json
import os
import socket
import subprocess
import sys
import time
from pathlib import Path
from typing import Any

SERVER = r"""
import asyncio,sys,logging
from umcp import MCPServer
from aioumcp import AsyncMCPServer
from umcp_shared import MCPPrincipal,MCPHTTPResponse
mode,port=sys.argv[1],int(sys.argv[2])
def setup(self):
 self.logger=logging.getLogger("oracle-http")
 self.logger.addHandler(logging.NullHandler());self.logger.propagate=False
def auth(self,*,method,path,headers,peer):
 token=headers.get("authorization", "")
 if token=="Bearer crash": raise RuntimeError("private auth error")
 return MCPPrincipal(token[7:]) if token.startswith("Bearer ") else None
def authorize(self,principal,*,rpc_method,tool_name):
 return principal.name!="denied"
def route(self,*,method,path,headers,body,peer):
 if path=="/ok":return MCPHTTPResponse(201,b"ok","text/plain",(("X-Extra","yes"),))
 if path=="/bad-route":raise RuntimeError("private route error")
 return None
async def route_async(self,**kwargs):return route(self,**kwargs)
base=MCPServer if mode=="sync" else AsyncMCPServer
attrs={"_setup_logging":setup,"authenticate_request":auth,"authorize_request":authorize}
if mode=="sync":attrs["handle_http_request"]=route
else:attrs["handle_http_request_async"]=route_async
cls=type("HTTPOracle",(base,),attrs);server=cls()
server.streamable_http_keepalive_seconds=.05
if mode=="sync":server.run_streamable_http(port=port,max_request_bytes=256)
else:asyncio.run(server.run_streamable_http_async(port=port,max_request_bytes=256))
"""


def scenarios() -> list[dict[str, Any]]:
    initialize = json.dumps(
        {
            "jsonrpc": "2.0",
            "id": 1,
            "method": "initialize",
            "params": {"protocolVersion": "2025-03-26"},
        }
    )
    tools = json.dumps({"jsonrpc": "2.0", "id": 2, "method": "tools/list"})

    def req(method: str, path: str, body: str = "", **headers: str) -> dict[str, Any]:
        base = {
            "Host": "localhost",
            "Authorization": "Bearer reader",
            "MCP-Protocol-Version": "2025-03-26",
        }
        if method == "POST":
            base["Content-Type"] = "application/json"
        base.update(headers)
        return {"method": method, "path": path, "body": body, "headers": base}

    return [
        req("OPTIONS", "/mcp", Origin="http://localhost"),
        req("OPTIONS", "/mcp"),
        req("OPTIONS", "/ok", Origin="http://localhost"),
        req("GET", "/ok", Authorization=""),
        req("GET", "/ok", Authorization="Bearer denied"),
        req("POST", "/mcp", initialize, **{"MCP-Protocol-Version": ""}),
        req("POST", "/mcp", tools),
        req("POST", "/mcp", tools, **{"Mcp-Session-Id": "{{SESSION}}"}),
        req(
            "POST",
            "/mcp",
            tools,
            **{"Mcp-Session-Id": "{{SESSION}}", "Authorization": "Bearer other"},
        ),
        req(
            "POST",
            "/mcp",
            tools,
            **{"Mcp-Session-Id": "{{SESSION}}", "MCP-Protocol-Version": "2024-11-05"},
        ),
        req("POST", "/mcp", tools, **{"Mcp-Session-Id": "missing"}),
        req("POST", "/mcp", initialize, **{"Mcp-Session-Id": "{{SESSION}}"}),
        req("POST", "/mcp", tools, **{"Authorization": ""}),
        req("POST", "/mcp", tools, **{"Authorization": "Bearer denied"}),
        req("POST", "/mcp", tools, **{"Authorization": "Bearer crash"}),
        req("POST", "/mcp", tools, **{"Accept": "text/event-stream"}),
        req("POST", "/mcp", tools, **{"Content-Type": "text/plain"}),
        req("POST", "/mcp", tools, **{"MCP-Protocol-Version": "bad"}),
        req("POST", "/mcp", tools, Origin="https://not.allowed"),
        req("POST", "/mcp", tools, Origin="http://localhost"),
        req("POST", "/mcp", "{"),
        req("POST", "/mcp", "[]"),
        req("POST", "/mcp", "null"),
        req("POST", "/mcp", '{"jsonrpc":"2.0","id":3,"result":{}}'),
        req("POST", "/mcp", '{"jsonrpc":"2.0","method":"unknown"}'),
        req("POST", "/mcp", '{"jsonrpc":"2.0","id":3,"method":"unknown"}'),
        req("POST", "/mcp", '{"jsonrpc":"2.0","id":3,"method":"notifications/initialized"}'),
        req(
            "POST",
            "/mcp",
            '{"jsonrpc":"2.0","id":4,"method":"resources/subscribe","params":{"uri":"u","_session_id":"spoof"}}',
            **{"Mcp-Session-Id": "{{SESSION}}"},
        ),
        req(
            "POST",
            "/mcp",
            '{"jsonrpc":"2.0","id":4,"method":"resources/unsubscribe","params":{"uri":"u"}}',
            **{"Mcp-Session-Id": "{{SESSION}}"},
        ),
        req("GET", "/mcp", Accept="application/json"),
        req("GET", "/mcp", **{"MCP-Protocol-Version": "bad"}),
        req("GET", "/mcp"),
        req("GET", "/ok"),
        req("POST", "/ok", "body", Origin="http://localhost"),
        req("GET", "/unknown"),
        req("GET", "/bad-route"),
        req("POST", "/mcp", "x" * 257),
        req("POST", "/mcp", tools, **{"Authorization": "a,b"}),
        req("DELETE", "/mcp"),
        req("DELETE", "/mcp", **{"Mcp-Session-Id": "{{SESSION}}"}),
        req("POST", "/mcp", tools, **{"Mcp-Session-Id": "{{SESSION}}"}),
    ]


def mode_scenarios() -> list[dict[str, Any]]:
    base = {
        "Host": "localhost",
        "Content-Type": "application/json",
        "MCP-Protocol-Version": "2025-03-26",
        "Authorization": "",
    }
    cases = []
    for method, body, headers in [
        ("POST", "{}", {"Content-Type": "text/plain"}),
        ("POST", "{}", {"Accept": "text/event-stream"}),
        ("GET", "", {"Accept": "application/json"}),
        ("GET", "", {"MCP-Protocol-Version": "bad"}),
        ("DELETE", "", {"MCP-Protocol-Version": "bad"}),
        ("POST", "{", {}),
        ("POST", "x" * 257, {}),
        ("OPTIONS", "x" * 257, {"Origin": "http://localhost"}),
    ]:
        cases.append(
            {"method": method, "path": "/mcp", "body": body, "headers": {**base, **headers}}
        )
    return cases


def capture(
    mode: str, reference: Path, cases: list[dict[str, Any]] | None = None
) -> list[dict[str, Any]]:
    with socket.socket() as sock:
        sock.bind(("127.0.0.1", 0))
        port = sock.getsockname()[1]
    env = {**os.environ, "PYTHONPATH": str(reference)}
    process = subprocess.Popen(
        [sys.executable, "-c", SERVER, mode, str(port)],
        cwd=reference,
        env=env,
        stdout=subprocess.DEVNULL,
        stderr=subprocess.PIPE,
    )
    results = []
    try:
        deadline = time.monotonic() + 10
        while True:
            if process.poll() is not None:
                raise AssertionError(
                    f"oracle server exited: {process.stderr.read() if process.stderr else b''!r}"
                )
            try:
                with socket.create_connection(("127.0.0.1", port), timeout=0.05):
                    break
            except OSError:
                if time.monotonic() > deadline:
                    raise AssertionError("oracle server startup timeout") from None
                time.sleep(0.02)
        session = ""
        for case in scenarios() if cases is None else cases:
            headers = {k: v.replace("{{SESSION}}", session) for k, v in case["headers"].items()}
            connection = http.client.HTTPConnection("127.0.0.1", port, timeout=5)
            connection.request(
                case["method"], case["path"], body=case["body"].encode(), headers=headers
            )
            response = connection.getresponse()
            raw = response.read()
            selected = {}
            for name in [
                "content-type",
                "www-authenticate",
                "allow",
                "access-control-allow-origin",
                "access-control-allow-methods",
                "access-control-allow-headers",
                "access-control-expose-headers",
                "vary",
                "x-extra",
            ]:
                if response.getheader(name) is not None:
                    selected[name] = response.getheader(name)
            created = response.getheader("Mcp-Session-Id")
            if created:
                session = created
                selected["mcp-session-id"] = "{{SESSION}}"
            expected = {
                "status": response.status,
                "headers": selected,
                "body": json.loads(raw)
                if raw and response.getheader("Content-Type") == "application/json"
                else raw.decode(),
            }
            results.append({"input": case, "expected": expected})
            connection.close()
    finally:
        process.terminate()
        try:
            process.wait(timeout=3)
        except subprocess.TimeoutExpired:
            process.kill()
            process.wait()
        if process.stderr:
            process.stderr.close()
    return results


def fixtures(reference: Path) -> list[dict[str, Any]]:
    sync, asynchronous = capture("sync", reference), capture("async", reference)
    for i, (a, b) in enumerate(zip(sync, asynchronous, strict=True)):
        if a != b:
            raise AssertionError(f"HTTP sync/async difference case {i}: {a!r} != {b!r}")
    return sync


def mode_fixtures(reference: Path) -> list[dict[str, Any]]:
    return [
        {"mode": mode, "cases": capture(mode, reference, mode_scenarios())}
        for mode in ["sync", "async"]
    ]
