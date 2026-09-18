"""Compare actual sync/async one-shot UTF-8 file transports."""

from __future__ import annotations

import asyncio
import importlib
import io
import json
import logging
import tempfile
from pathlib import Path
from typing import Any


def fixtures(shared: Any) -> list[dict[str, Any]]:
    sync: Any = importlib.import_module("umcp")
    asynchronous: Any = importlib.import_module("aioumcp")

    def setup(self: Any) -> None:
        self.logger = logging.getLogger("oracle-file")
        self.logger.addHandler(logging.NullHandler())
        self.logger.propagate = False

    def initialize(self: Any, request_id: Any, params: Any) -> Any:
        return self.create_response(
            request_id,
            {"transport": shared.get_request_context().transport, "params": params},
            None,
        )

    payloads = [
        b"",
        b"{} {}",
        b"null",
        b'{"jsonrpc":"2.0","id":1,"method":"initialize"}',
        b'{\r\n"jsonrpc":"2.0",\r"id":2,\n"method":"initialize"\r\n}',
        b'{"jsonrpc":"2.0","method":"notifications/initialized"}',
        b'{"jsonrpc":"2.0","id":"\xff","method":"unknown"}',
    ]
    cases = []
    with tempfile.TemporaryDirectory(prefix="umcp-file-oracle-") as directory:
        path = Path(directory) / "request.json"
        for payload in payloads:
            path.write_bytes(payload)
            outcomes = []
            for module, base in [
                (sync, sync.MCPServer),
                (asynchronous, asynchronous.AsyncMCPServer),
            ]:
                cls = type(
                    "FileOracle",
                    (base,),
                    {"_setup_logging": setup, "handle_initialize": initialize},
                )
                previous = module._stdout_bin
                output = io.BytesIO()
                module._stdout_bin = output
                try:
                    error = False
                    try:
                        if module is sync:
                            cls().run([str(path)])
                        else:
                            asyncio.run(cls().run_async([str(path)]))
                    except UnicodeDecodeError:
                        error = True
                    outcomes.append(
                        {
                            "expected": [
                                json.loads(line) for line in output.getvalue().splitlines()
                            ],
                            "error": error,
                        }
                    )
                finally:
                    module._stdout_bin = previous
            if outcomes[0] != outcomes[1]:
                raise AssertionError("file-mode sync/async mismatch")
            cases.append({"input_bytes": list(payload), **outcomes[0]})
    return cases
