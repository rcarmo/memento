"""Capture sync/async run argument dispatch without opening sockets or user files."""

from __future__ import annotations

import asyncio
import builtins
import importlib
import io
import logging
from typing import Any
from unittest.mock import patch


def scenarios() -> list[list[str]]:
    cases = [
        [],
        ["file.json"],
        ["one", "two"],
        ["--unknown"],
        ["--"],
        ["--port"],
        ["--host"],
        ["--endpoint"],
        ["--max-request-bytes"],
        ["--allowed-origin"],
        ["--transport"],
    ]
    for mode in ["stdio", "tcp", "sse", "streamable-http", "bad", ""]:
        for prefix in [[], ["--port", "0"]]:
            cases.append(prefix + ["--transport", mode])
    for flag in ["--tcp", "--http", "--sse"]:
        cases.extend(
            [
                [flag],
                [flag, "--port", "0"],
                [flag, flag, "-p", "12"],
                [flag, "--transport", "stdio"],
                ["--transport", "tcp", flag, "-p", "1"],
            ]
        )
    for flag in ["--port", "--max-request-bytes"]:
        for value in [
            "0",
            "-1",
            "+1_024",
            "１２",
            " 42 ",
            "1e2",
            "",
            "--http",
            "0x10",
            "1__0",
            "9" * 30,
        ]:
            cases.append([flag, value])
    cases.extend(
        [
            [
                "--port",
                "1",
                "-p",
                "2",
                "--host",
                "localhost",
                "--http",
                "--endpoint",
                "/custom",
                "--max-request-bytes",
                "128",
                "--allowed-origin",
                "http://one",
                "--allowed-origin",
                "http://two",
                "ignored.json",
            ],
            ["--host", "::1", "--sse", "--port", "0"],
            ["--endpoint", "bad", "--http", "--port", "0"],
            ["--port=10"],
            ["--host", "--tcp"],
            ["--transport", "", "--http"],
            ["--transport", "stdio", "file.json"],
            ["file.json", "--port", "0"],
            ["--port", "0", "--max-request-bytes", "-1"],
            ["--port", "0", "--max-request-bytes", "0"],
        ]
    )
    return cases


def fixtures(shared: Any) -> list[dict[str, Any]]:
    results = []
    for args in scenarios():
        outcomes = []
        for module_name, class_name in [("umcp", "MCPServer"), ("aioumcp", "AsyncMCPServer")]:
            module: Any = importlib.import_module(module_name)
            calls: list[dict[str, Any]] = []

            def setup(self: Any) -> None:
                self.logger = logging.getLogger("oracle-cli")
                self.logger.addHandler(logging.NullHandler())
                self.logger.propagate = False

            def capture(
                mode: str, calls: list[dict[str, Any]] = calls, module_name: str = module_name
            ) -> Any:
                def sync(self: Any, **kwargs: Any) -> None:
                    calls.append({"mode": mode, **kwargs})

                async def asynchronous(self: Any, **kwargs: Any) -> None:
                    sync(self, **kwargs)

                return sync if module_name == "umcp" else asynchronous

            opened: list[str] = []

            def process(
                self: Any,
                raw: str,
                context: Any = None,
                calls: list[dict[str, Any]] = calls,
                opened: list[str] = opened,
            ) -> None:
                calls.append({"mode": "file", "path": opened[0]})

            async def process_async(
                self: Any, raw: str, context: Any = None, process: Any = process
            ) -> None:
                process(self, raw, context)

            def open_file(path: str, opened: list[str] = opened, **kwargs: Any) -> Any:
                opened.append(path)
                return io.StringIO("{}")

            attrs: dict[str, Any] = {"_setup_logging": setup}
            suffix = "" if module_name == "umcp" else "_async"
            for method, mode in [
                ("run_socket", "tcp"),
                ("run_streamable_http", "streamable-http"),
                ("run_sse", "sse"),
            ]:
                attrs[method + suffix] = capture(mode)
            attrs["process_request" + suffix] = process if module_name == "umcp" else process_async
            cls = type("CLIOracle", (getattr(module, class_name),), attrs)
            previous = module._stdin_bin
            module._stdin_bin = io.BytesIO()
            try:
                with patch.object(builtins, "open", open_file):
                    try:
                        if module_name == "umcp":
                            cls().run(args)
                        else:
                            asyncio.run(cls().run_async(args))
                        outcomes.append(calls[0] if calls else {"mode": "stdio"})
                    except ValueError as exc:
                        outcomes.append({"error": str(exc)})
            finally:
                module._stdin_bin = previous
        if outcomes[0] != outcomes[1]:
            raise AssertionError(f"CLI mode divergence: {args!r}: {outcomes!r}")
        results.append({"args": args, "expected": outcomes[0]})
    return results
