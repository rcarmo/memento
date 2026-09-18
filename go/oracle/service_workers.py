"""Capture _memory_call admission, shielded cancellation and drain without 30s waits."""

from __future__ import annotations

import asyncio
import importlib
import threading
from typing import Any
from unittest.mock import patch


def fixtures() -> list[dict[str, Any]]:
    module: Any = importlib.import_module("memento.server")

    async def run() -> list[dict[str, Any]]:
        cases = []
        for closing, active, busy, method in [
            (False, 0, False, "memory_propose"),
            (True, 0, False, "memory_propose"),
            (False, 2, False, "memory_propose"),
            (False, 0, True, "memory_propose"),
            (False, 0, True, "memory_operation_get"),
            (False, 2, True, "memory_operation_get"),
        ]:
            server = object.__new__(module.MementoMCPServer)
            server._closing = closing
            server._execute_busy = busy
            server._worker_tasks = set(range(active))
            server._call_in_worker = lambda *args: {"worked": True}
            result = await server._memory_call(method, None)
            cases.append(
                {
                    "kind": "admission",
                    "closing": closing,
                    "active": active,
                    "busy": busy,
                    "method": method,
                    "expected": result
                    if isinstance(result, dict)
                    else result.model_dump(mode="json"),
                }
            )
        for scenario in ["timeout", "cancel", "error"]:
            server = object.__new__(module.MementoMCPServer)
            server._closing = False
            server._execute_busy = False
            server._worker_tasks = set()
            started = threading.Event()
            release = threading.Event()
            finished = threading.Event()

            def work(
                *args: Any,
                started: threading.Event = started,
                release: threading.Event = release,
                finished: threading.Event = finished,
                scenario: str = scenario,
            ) -> Any:
                started.set()
                release.wait(5)
                finished.set()
                if scenario == "error":
                    raise RuntimeError("synthetic")
                return {"worked": True}

            server._call_in_worker = work
            original_wait = asyncio.wait_for

            async def short_wait(
                future: Any, timeout: float, original_wait: Any = original_wait
            ) -> Any:
                return await original_wait(future, timeout=0.01)

            item: dict[str, Any] = {"kind": scenario}
            with patch.object(module.asyncio, "wait_for", short_wait):
                call = asyncio.create_task(server._memory_call("memory_propose", None))
                await asyncio.to_thread(started.wait, 2)
                if scenario == "cancel":
                    call.cancel()
                if scenario == "error":
                    release.set()
                try:
                    result = await call
                    item["expected"] = (
                        result if isinstance(result, dict) else result.model_dump(mode="json")
                    )
                except asyncio.CancelledError:
                    item["error"] = "cancelled"
                except RuntimeError as exc:
                    item["error"] = str(exc)
                item["active_before_release"] = len(server._worker_tasks)
                release.set()
                await server.drain_workers()
                item["finished"] = finished.is_set()
                item["closing"] = server._closing
                item["active_after_drain"] = len(server._worker_tasks)
            cases.append(item)
        return cases

    return asyncio.run(run())
