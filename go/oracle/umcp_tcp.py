"""Capture raw TCP responses from both pinned Python transports, including differences."""

from __future__ import annotations

import json
import os
import selectors
import socket
import subprocess
import sys
from pathlib import Path
from typing import Any

SERVER = r"""
import asyncio,logging,sys
from umcp import MCPServer
from aioumcp import AsyncMCPServer
from umcp_shared import get_request_context
mode=sys.argv[1]
def setup(self):
 self.logger=logging.getLogger("oracle-tcp")
 self.logger.addHandler(logging.NullHandler());self.logger.propagate=False
def initialize(self,request_id,params):
 return self.create_response(request_id,{"transport":get_request_context().transport,"params":params},None)
base=MCPServer if mode=="sync" else AsyncMCPServer
server=type("TCPOracle",(base,),{"_setup_logging":setup,"handle_initialize":initialize})()
if mode=="sync":server.run_socket(port=0)
else:asyncio.run(server.run_socket_async(port=0))
"""


def scenarios() -> list[dict[str, Any]]:
    request = b'{"jsonrpc":"2.0","id":1,"method":"initialize"}'
    cases = []

    def add(name: str, raw: bytes, padding: int = 0, suffix: bytes = b"", chunk: int = 0) -> None:
        cases.append(
            {
                "name": name,
                "prefix": list(raw),
                "padding": padding,
                "suffix": list(suffix),
                "chunk": chunk,
            }
        )

    add("empty", b"")
    add("blank", b" \t\r\n\x1c\n")
    add("final-line", request)
    add("fragmented", request + b"\r\n", chunk=1)
    add(
        "multiple",
        b'{"jsonrpc":"2.0","method":"notifications/initialized"}\n' + request + b"\n" + request,
    )
    add("invalid-json", b"{\nnull\n{}\n[]\n{} {}\n")
    add(
        "utf8-replacement",
        b'{"jsonrpc":"2.0","id":"\xff","method":"initialize"}\n' + request + b"\n",
    )
    add("incomplete-utf8", b'{"jsonrpc":"2.0","id":"\xe2\x82","method":"initialize"}\n')
    add("valid-unicode", '{"jsonrpc":"2.0","id":"日本 café","method":"initialize"}\n'.encode())
    for length in [65535, 65536, 65537, 70000]:
        for newline in [False, True]:
            add(
                f"line-{length}-{'lf' if newline else 'eof'}",
                request,
                length - len(request),
                b"\n" if newline else b"",
            )
    return cases


def capture(mode: str, reference: Path) -> list[dict[str, Any]]:
    process = subprocess.Popen(
        [sys.executable, "-c", SERVER, mode],
        cwd=reference,
        env={**os.environ, "PYTHONPATH": str(reference)},
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
    )
    results = []
    try:
        assert process.stdout is not None
        with selectors.DefaultSelector() as selector:
            selector.register(process.stdout, selectors.EVENT_READ)
            if not selector.select(timeout=10):
                raise AssertionError("TCP oracle startup timeout")
            line = process.stdout.readline().decode().strip()
        if not line.startswith("Listening on "):
            raise AssertionError(f"TCP oracle startup failed: {line!r}")
        port = int(line.rsplit(":", 1)[1])
        for case in scenarios():
            payload = bytes(case["prefix"]) + b" " * case["padding"] + bytes(case["suffix"])
            with socket.create_connection(("127.0.0.1", port), timeout=5) as conn:
                chunk = case["chunk"] or max(1, len(payload))
                for offset in range(0, len(payload), chunk):
                    conn.sendall(payload[offset : offset + chunk])
                conn.shutdown(socket.SHUT_WR)
                parts = []
                while True:
                    try:
                        part = conn.recv(8192)
                    except ConnectionResetError:
                        break
                    if not part:
                        break
                    parts.append(part)
            expected = [json.loads(line) for line in b"".join(parts).splitlines()]
            results.append({"mode": mode, "input": case, "expected": expected})
    finally:
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
    return results


def fixtures(reference: Path) -> list[dict[str, Any]]:
    return capture("sync", reference) + capture("async", reference)
