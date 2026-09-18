"""Capture managed-access tool discovery metadata from MementoMCPServer."""

from __future__ import annotations

import importlib
from typing import Any


def fixtures() -> dict[str, Any]:
    server: Any = importlib.import_module("memento.server")
    tools = server.MementoMCPServer._access_tools()
    return {"count": len(tools), "tools": tools}
