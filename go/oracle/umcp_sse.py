"""Capture legacy SSE handshakes and authenticated message/event pairs."""

from __future__ import annotations

import http.client
import json
import os
import selectors
import subprocess
import sys
from pathlib import Path
from typing import Any

SERVER = r"""
import asyncio,logging,sys
from umcp import MCPServer
from aioumcp import AsyncMCPServer
from umcp_shared import MCPPrincipal,get_request_context
mode=sys.argv[1]
def setup(self):
 self.logger=logging.getLogger("oracle-sse")
 self.logger.addHandler(logging.NullHandler());self.logger.propagate=False
def auth(self,*,method,path,headers,peer):
 token=headers.get("authorization", "")
 if token=="Bearer crash":raise RuntimeError("private")
 return MCPPrincipal(token[7:]) if token.startswith("Bearer ") else None
def authorize(self,principal,*,rpc_method,tool_name):
 if rpc_method=="crash":raise RuntimeError("private")
 return principal.name!="denied" and rpc_method!="forbidden"
def initialize(self,request_id,params):
 ctx=get_request_context()
 return self.create_response(request_id,{"transport":ctx.transport,"principal":ctx.principal,"session":bool(ctx.session_id),"params":params},None)
base=MCPServer if mode=="sync" else AsyncMCPServer
server=type("SSEOracle",(base,),{"_setup_logging":setup,"handle_initialize":initialize,"authenticate_request":auth,"authorize_request":authorize})()
if mode=="sync":server.run_sse(port=0,max_request_bytes=256)
else:asyncio.run(server.run_sse_async(port=0,max_request_bytes=256))
"""


def scenarios() -> list[dict[str, Any]]:
    init = '{"jsonrpc":"2.0","id":1,"method":"initialize"}'
    result = []

    def add(method: str, path: str, body: str = "", event: bool = False, /, **headers: str) -> None:
        result.append(
            {
                "method": method,
                "path": path,
                "body": body,
                "event": event,
                "headers": {
                    "Authorization": "Bearer reader",
                    "Content-Type": "application/json",
                    **headers,
                },
            }
        )

    add("OPTIONS", "/sse", Origin="http://localhost")
    add("OPTIONS", "/message")
    add("OPTIONS", "/missing", Origin="http://localhost")
    add("GET", "/missing")
    add("GET", "/sse", Accept="application/json")
    add("GET", "/sse", Authorization="")
    add("GET", "/sse", Authorization="Bearer denied")
    add("GET", "/sse", Authorization="Bearer crash")
    add("GET", "/sse", Origin="https://evil.example")
    add("POST", "/message", init)
    add("POST", "/missing", init)
    add("POST", "/message?sessionId=missing", init)
    path = "/message?sessionId={{SESSION}}"
    add("POST", path, init, Authorization="")
    add("POST", path, init, Authorization="Bearer other")
    add("POST", path, init, Authorization="Bearer crash")
    add("POST", path, init, **{"Content-Type": "text/plain"})
    add("POST", path, init, Accept="text/event-stream")
    add("POST", path, "x" * 257)
    add("POST", path, init, True, Origin="http://localhost")
    add("POST", "/message?sessionId=&sessionId={{SESSION}}", init, True)
    add("POST", path, "{", True)
    add("POST", path, "null", True)
    add("POST", path, '{"jsonrpc":"2.0","id":2,"method":"unknown"}', True)
    add("POST", path, '{"jsonrpc":"2.0","method":"notifications/initialized"}')
    add("POST", path, '{"jsonrpc":"2.0","id":2,"result":{}}')
    add("POST", path, '{"jsonrpc":"2.0","id":2,"method":"forbidden"}')
    add("POST", path, '{"jsonrpc":"2.0","id":2,"method":"crash"}')
    add(
        "POST",
        path,
        '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"missing"}}',
        True,
    )
    for method in ["resources/subscribe", "resources/unsubscribe"]:
        add(
            "POST",
            path,
            json.dumps(
                {
                    "jsonrpc": "2.0",
                    "id": 3,
                    "method": method,
                    "params": {"uri": "test://one", "_session_id": "spoof"},
                }
            ),
            True,
        )
    add("PUT", "/sse")
    # Mode-specific ordering: async parser rejects size before origin/routing;
    # sync handlers only check size for POST /message, after media validation.
    add("POST", "/missing", "x" * 257)
    add("POST", path, "x" * 257, Origin="https://evil.example")
    add("POST", path, "x" * 257, **{"Content-Type": "text/plain"})
    return result


def event(response: http.client.HTTPResponse) -> tuple[str, str]:
    kind, data = "", ""
    while True:
        line = response.readline()
        if not line:
            raise AssertionError("SSE stream ended before expected event")
        if line == b"\n" and kind:
            return kind, data
        if line.startswith(b"event: "):
            kind = line[7:].decode().strip()
        elif line.startswith(b"data: "):
            data = line[6:].decode().strip()


def selected(response: http.client.HTTPResponse) -> dict[str, str]:
    names = [
        "content-type",
        "cache-control",
        "www-authenticate",
        "allow",
        "access-control-allow-origin",
        "access-control-allow-methods",
        "access-control-allow-headers",
        "access-control-expose-headers",
        "vary",
    ]
    return {k: v for k in names if (v := response.getheader(k)) is not None}


def capture(mode: str, reference: Path) -> dict[str, Any]:
    process = subprocess.Popen(
        [sys.executable, "-c", SERVER, mode],
        cwd=reference,
        env={**os.environ, "PYTHONPATH": str(reference)},
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
    )
    stream = None
    try:
        assert process.stdout is not None
        with selectors.DefaultSelector() as selector:
            selector.register(process.stdout, selectors.EVENT_READ)
            if not selector.select(timeout=10):
                raise AssertionError("SSE oracle startup timeout")
            line = process.stdout.readline().decode().strip()
        port = int(line.rsplit(":", 1)[1].split("/")[0])
        stream = http.client.HTTPConnection("127.0.0.1", port, timeout=5)
        stream.request(
            "GET", "/sse", headers={"Authorization": "Bearer reader", "Origin": "http://localhost"}
        )
        response = stream.getresponse()
        kind, endpoint = event(response)
        assert kind == "endpoint" and endpoint.startswith("/message?sessionId=")
        session = endpoint.split("=", 1)[1]
        results = []
        for case in scenarios():
            connection = http.client.HTTPConnection("127.0.0.1", port, timeout=5)
            try:
                connection.request(
                    case["method"],
                    case["path"].replace("{{SESSION}}", session),
                    body=case["body"].encode(),
                    headers=case["headers"],
                )
                reply = connection.getresponse()
                expected = {"status": reply.status, "headers": selected(reply)}
                reply.read()  # sync unsupported-method HTML is raw-server scope
                if case["event"]:
                    kind, data = event(response)
                    assert kind == "message"
                    expected["event"] = json.loads(data)
                results.append({"input": case, "expected": expected})
            finally:
                connection.close()
        response.close()
        return {
            "mode": mode,
            "handshake": {"status": response.status, "headers": selected(response)},
            "cases": results,
        }
    finally:
        if stream:
            stream.close()
        process.terminate()
        try:
            process.wait(timeout=3)
        except subprocess.TimeoutExpired:
            process.kill()
            process.wait()
        if process.stdout:
            process.stdout.close()
        if process.stderr:
            process.stderr.close()


def fixtures(reference: Path) -> list[dict[str, Any]]:
    return [capture(mode, reference) for mode in ["sync", "async"]]
