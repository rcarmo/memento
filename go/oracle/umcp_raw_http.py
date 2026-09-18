"""Exercise pinned async HTTP parsers with exact byte streams, without client sanitisation."""

from __future__ import annotations

import asyncio
import importlib
import json
import logging
from typing import Any


def scenarios() -> list[dict[str, Any]]:
    cases: list[dict[str, Any]] = []

    def add(
        name: str, prefix: bytes, repeat: bytes = b"", count: int = 0, suffix: bytes = b""
    ) -> None:
        cases.append(
            {
                "name": name,
                "prefix": list(prefix),
                "repeat": list(repeat),
                "count": count,
                "suffix": list(suffix),
            }
        )

    add("empty", b"")
    add("minimal", b"GET /probe HTTP/1.1\r\nHost: localhost\r\n\r\n")
    add("http10", b"GET /probe HTTP/1.0\n\n")
    add("http10-keepalive", b"GET /probe HTTP/1.0\nConnection: keep-alive\n\n")
    add("http11-close", b"GET /probe HTTP/1.1\nHost: localhost\nConnection: close, keep-alive\n\n")
    add("extra-spaces", b"GET  /probe HTTP/1.1\nHost: localhost\n\n")
    add("tabs", b"GET\t/probe\tHTTP/1.1\nHost: localhost\n\n")
    add("edge-whitespace", b" \tGET /probe HTTP/1.1 \r\nHost: localhost\n\n")
    add("empty-method", b" /probe HTTP/1.1\nHost: localhost\n\n")
    add("bad-version", b"GET /probe HTTP/2.0\nHost: localhost\n\n")
    add("bad-request", b"GET /probe\n\n")
    add("nonascii-request", b"GET /\xff HTTP/1.1\nHost: localhost\n\n")
    add("missing-host", b"GET /probe HTTP/1.1\n\n")
    for key, values in [
        (b"Host", [b"a", b"b"]),
        (b"Accept", [b"application/json", b"text/event-stream"]),
        (b"Authorization", [b"Bearer a", b"Bearer b"]),
        (b"X-Value", [b"first", b"second"]),
    ]:
        add(
            "duplicate-" + key.decode(),
            b"GET /probe HTTP/1.1\n"
            + (b"Host: localhost\n" if key != b"Host" else b"")
            + b"".join(key + b": " + value + b"\n" for value in values)
            + b"\n",
        )
    for name, header in [
        ("leading-space", b" X-Test: value"),
        ("folded", b"X-Test: value\n continued"),
        ("empty-name", b": value"),
        ("space-in-name", b"X Test: value"),
        ("latin1-name", b"X-\xc9: val"),
        ("latin1-value", b"X-Test: \xa0\xff\x85"),
        ("missing-colon", b"Bad"),
        ("trimmed-key", b"X-Test \t: value"),
        ("nul-name", b"X-\x00: value"),
        ("ambiguous-host", b"Host: a,b"),
        ("origin-allowed", b"Origin: http://localhost"),
        ("origin-denied", b"Origin: https://evil.example"),
        ("origin-duplicate", b"Origin: http://localhost\nOrigin: http://localhost"),
        ("te-empty", b"Transfer-Encoding:"),
        ("te-chunked", b"Transfer-Encoding: chunked"),
    ]:
        add(
            name,
            b"GET /probe HTTP/1.1\n"
            + (b"" if name == "ambiguous-host" else b"Host: localhost\n")
            + header
            + b"\n\n",
        )
    for length in [
        b"",
        b"+0",
        b"-0",
        b"-1",
        b"2",
        b"2_0",
        b"_2",
        b"2__0",
        b"2.0",
        b"0x2",
        b"1,1",
        b"\xa02\xa0",
        b"\x1c2",
        b"9" * 100,
        b"0" * 4301,
    ]:
        label = length.decode("latin1") if len(length) < 30 else f"digits-{len(length)}"
        add(
            "length-" + label,
            b"POST /probe HTTP/1.1\nHost: localhost\nContent-Length: " + length + b"\n\n{}",
        )
    add("short-body", b"POST /probe HTTP/1.1\nHost: localhost\nContent-Length: 3\n\n{}")
    add("partial-headers", b"GET /probe HTTP/1.1\nHost: localhost\n")
    add(
        "bad-field-before-overflow",
        b"GET /probe HTTP/1.1\nHost: localhost\nBad\nX: ",
        b"a",
        70000,
        b"\n\n",
    )
    add("final-header", b"GET /probe HTTP/1.1\nHost: localhost")
    for size in [8192, 8193, 65536, 65537]:
        prefix, suffix = b"GET /", b" HTTP/1.1\n"
        add(
            f"request-line-{size}",
            prefix,
            b"x",
            size - len(prefix) - len(suffix),
            suffix + b"Host: localhost\n\n",
        )
    for count in [98, 99, 100, 101]:
        add(
            f"headers-{count}",
            b"GET /probe HTTP/1.1\nHost: localhost\n",
            b"X: a\n",
            count - 1,
            b"\n",
        )
    for size in [65535, 65536, 65537, 70000]:
        add(
            f"header-line-{size}",
            b"GET /probe HTTP/1.1\nHost: localhost\nX: ",
            b"a",
            size - 4,
            b"\n\n",
        )
    add(
        "origin-with-bad-length",
        b"POST /probe HTTP/1.1\nHost: localhost\nOrigin: http://localhost\nContent-Length: -1\n\n",
    )
    add(
        "origin-denied-with-bad-length",
        b"POST /probe HTTP/1.1\nHost: localhost\nOrigin: https://evil.example\nContent-Length: -1\n\n",
    )
    add(
        "origin-with-duplicate-length",
        b"POST /probe HTTP/1.1\nHost: localhost\nOrigin: http://localhost\nContent-Length: 0\nContent-Length: 0\n\n",
    )
    return cases


class Writer:
    def __init__(self) -> None:
        self.data = bytearray()
        self.closed = False

    def write(self, data: bytes) -> None:
        self.data.extend(data)

    async def drain(self) -> None:
        pass

    def close(self) -> None:
        self.closed = True

    async def wait_closed(self) -> None:
        pass

    def get_extra_info(self, name: str) -> Any:
        return ("127.0.0.1", 12345) if name == "peername" else None


def fixtures(shared: Any) -> list[dict[str, Any]]:
    module: Any = importlib.import_module("aioumcp")

    def setup(self: Any) -> None:
        self.logger = logging.getLogger("oracle-raw-http")
        self.logger.addHandler(logging.NullHandler())
        self.logger.propagate = False

    async def route(self: Any, **kwargs: Any) -> Any:
        body = kwargs.pop("body")
        kwargs.pop("peer")
        kwargs["body"] = list(body)
        # Echo parsed fields through the real Streamable HTTP connection handler.
        return shared.MCPHTTPResponse(200, json.dumps(kwargs).encode(), "application/json")

    cls = type(
        "RawHTTPOracle",
        (module.AsyncMCPServer,),
        {"_setup_logging": setup, "handle_http_request_async": route},
    )

    async def capture() -> list[dict[str, Any]]:
        results = []
        for mode in ["streamable", "sse"]:
            for case in scenarios():
                raw = (
                    bytes(case["prefix"])
                    + bytes(case["repeat"]) * case["count"]
                    + bytes(case["suffix"])
                )
                reader = asyncio.StreamReader()
                reader.feed_data(raw)
                reader.feed_eof()
                server = cls()
                if mode == "sse":
                    try:
                        (
                            method,
                            target,
                            version,
                            headers,
                            counts,
                            body,
                        ) = await server._sse_read_http_request(reader, max_request_bytes=1 << 20)
                        expected = {
                            "status": 200,
                            "method": method,
                            "target": target,
                            "version": version,
                            "headers": headers,
                            "counts": counts,
                            "body": list(body),
                        }
                    except OverflowError:
                        expected = {"status": 413}
                    except (ConnectionError, ValueError, UnicodeError, asyncio.IncompleteReadError):
                        expected = {"status": 400}
                else:
                    writer = Writer()
                    server.streamable_http_max_requests_per_connection = 1
                    await server._handle_streamable_http_client(reader, writer, "/mcp", [], 1 << 20)
                    if not writer.data:
                        expected = {"status": 0}
                    else:
                        head, _, body_bytes = writer.data.partition(b"\r\n\r\n")
                        status = int(head.split(b" ", 2)[1])
                        headers_out = {
                            k.decode().lower(): v.decode().strip()
                            for k, v in (line.split(b":", 1) for line in head.split(b"\r\n")[1:])
                        }
                        expected = {
                            "status": status,
                            "origin": headers_out.get("access-control-allow-origin", ""),
                        }
                        if status == 200:
                            expected.update(json.loads(body_bytes))
                results.append({"mode": mode, "input": case, "expected": expected})
        return results

    return asyncio.run(capture())
