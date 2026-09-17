"""Exercise actual sync/async uMCP stdio loops with synthetic byte streams."""

from __future__ import annotations

import asyncio
import importlib
import io
import json
import logging
from typing import Any


def fixtures(shared: Any) -> list[dict[str, Any]]:
    sync: Any = importlib.import_module("umcp")
    asynchronous: Any = importlib.import_module("aioumcp")

    def setup(self: Any) -> None:
        self.logger = logging.getLogger("oracle-stdio")
        self.logger.addHandler(logging.NullHandler())
        self.logger.propagate = False

    def handle(self: Any, request_id: Any, params: dict[str, Any]) -> Any:
        return self.create_response(
            request_id,
            {"transport": shared.get_request_context().transport, "params": params},
            None,
        )

    streams = [
        b"",
        b" \t\r\n\x1c\n",
        b"{}\nnull\n",
        b'{"jsonrpc":"2.0","method":"notifications/initialized"}\n',
        b'{"jsonrpc":"2.0","id":1,"method":"initialize"}\n{"jsonrpc":"2.0","id":2,"method":"unknown"}',
        b'{"jsonrpc":"2.0","id":"\xe2\x82","method":"unknown"}\n',
        b'{"jsonrpc":"2.0","id":"\xed\xa0\x80","method":"unknown"}',
        b'{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"text":"'
        + b"x" * 70000
        + b'"}}\n',
    ]
    out = []
    for payload in streams:
        outputs = []
        for module, base in [(sync, sync.MCPServer), (asynchronous, asynchronous.AsyncMCPServer)]:
            cls = type(
                "StdioOracle", (base,), {"_setup_logging": setup, "handle_initialize": handle}
            )
            original = (module._stdin_bin, module._stdout_bin)
            module._stdin_bin = io.BytesIO(payload)
            writer = io.BytesIO()
            module._stdout_bin = writer
            try:
                if module is sync:
                    cls().run([])
                else:
                    asyncio.run(cls().run_async([]))
                outputs.append([json.loads(line) for line in writer.getvalue().splitlines()])
            finally:
                module._stdin_bin, module._stdout_bin = original
        if outputs[0] != outputs[1]:
            raise AssertionError("stdio sync/async mismatch")
        # Do not bloat fixtures with one repeated stress input; keep it separately tested in Go.
        if len(payload) < 1000:
            out.append({"input_bytes": list(payload), "expected": outputs[0]})
    return out
