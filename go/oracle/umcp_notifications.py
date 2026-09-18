"""Capture source SSE/Streamable notification fallback policy using synthetic sessions."""

from __future__ import annotations

import asyncio
import importlib
import io
import json
import logging
from queue import Queue
from threading import Lock
from typing import Any


class Writer:
    def __init__(self, fail: bool = False) -> None:
        self.data = bytearray()
        self.fail = fail

    def write(self, data: bytes) -> None:
        if self.fail:
            raise BrokenPipeError("synthetic")
        self.data.extend(data)

    async def drain(self) -> None:
        pass


def fixtures() -> list[dict[str, Any]]:
    def setup(self: Any) -> None:
        self.logger = logging.getLogger("oracle-notifications")
        self.logger.addHandler(logging.NullHandler())
        self.logger.propagate = False

    results = []
    for mode in ["sync", "async"]:
        module: Any = importlib.import_module("umcp" if mode == "sync" else "aioumcp")
        base = module.MCPServer if mode == "sync" else module.AsyncMCPServer
        cls = type("NotificationOracle", (base,), {"_setup_logging": setup})
        for active in [False, True]:
            for session in [False, True]:
                # Transport multiplexing is not exposed by the source CLI.
                if active and session:
                    continue
                for targets in [None, [], ["one"], ["absent"]]:
                    for params in [None, {}, {"value": 1}]:
                        server = cls()
                        server._streamable_http_active = active
                        server._sse_sessions = {}
                        stream = Writer()
                        queue: Queue[bytes] = Queue()
                        if session:
                            server._sse_sessions["one"] = (
                                (queue, "reader")
                                if mode == "sync"
                                else (stream, asyncio.Event(), asyncio.Lock(), "reader")
                            )
                        output = io.BytesIO()
                        previous = module._stdout_bin
                        module._stdout_bin = output
                        try:
                            if mode == "sync":
                                server._sse_lock = Lock()
                                server._send_notification(
                                    "notifications/test", params, session_ids=targets
                                )
                            else:
                                asyncio.run(
                                    server._send_notification_async(
                                        "notifications/test", params, session_ids=targets
                                    )
                                )
                        finally:
                            module._stdout_bin = previous
                        if mode == "sync":
                            while not queue.empty():
                                stream.data.extend(queue.get_nowait())
                        events = [
                            json.loads(line[6:])
                            for line in stream.data.splitlines()
                            if line.startswith(b"data: ")
                        ]
                        results.append(
                            {
                                "mode": mode,
                                "active_http": active,
                                "session": session,
                                "targets": targets,
                                "params": params,
                                "events": events,
                                "stdout": [
                                    json.loads(line) for line in output.getvalue().splitlines()
                                ],
                            }
                        )
    return results
