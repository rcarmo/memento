"""Control operation transitions and exact persisted JSON/hash reference."""

from __future__ import annotations

import importlib
import tempfile
from dataclasses import asdict
from pathlib import Path
from typing import Any
from unittest.mock import patch


def fixtures() -> dict[str, Any]:
    db: Any = importlib.import_module("memento.control.db")
    module: Any = importlib.import_module("memento.control.operations")
    commands: list[dict[str, Any]] = []

    def add(action: str, **kwargs: Any) -> None:
        commands.append({"action": action, **kwargs})

    base = {
        "op_id": "op-one",
        "principal": "agent",
        "idempotency_key": "key",
        "tool_name": "memory_write",
        "request_json": '{"x":1}',
        "client_instance_id": "client",
        "mcp_session_id": "session",
        "source_chat": "synthetic",
    }
    add("create", request=base)
    add("create", request={**base, "op_id": "discarded", "tool_name": "different"})
    add("create", request={**base, "op_id": "conflict", "request_json": '{"x": 1}'})
    add("running", op_id="op-one", base="base")
    add("failed", op_id="op-one", error_class="ValueError", message="synthetic failure")
    add("create", request={**base, "op_id": "retry"})
    add("running", op_id="op-one", base="base-two")
    add(
        "succeeded",
        op_id="op-one",
        revision="result",
        result={"z": "日本 😀", "changed_paths": ["/a.md"], "n": 1.0},
    )
    add("get", op_id="op-one")
    add("conflict", op_id="op-one", message="blind state update")
    add("succeeded", op_id="op-one", revision="result-two", result={})
    add("create", request={**base, "op_id": "op-other", "principal": "other"})
    add("create", request={**base, "op_id": "op-one", "principal": "third"})
    add("get", op_id="missing")
    add("running", op_id="missing", base="base")
    add(
        "create",
        request={
            **base,
            "op_id": "op-zero",
            "idempotency_key": "another",
            "request_json": "not necessarily JSON",
        },
    )
    add("interrupted")
    results = []
    with tempfile.TemporaryDirectory(prefix="memento-operation-oracle-") as directory:
        connection = db.connect_control_db(Path(directory) / "control.sqlite")
        db.migrate_control_db(connection)
        with patch.object(module, "_utcnow", lambda: "2026-09-18T00:00:00Z"):
            for command in commands:
                action = command["action"]
                item: dict[str, Any] = {"input": command}
                try:
                    if action == "create":
                        record = module.create_operation(
                            connection, module.OperationRequest(**command["request"])
                        )
                    elif action == "get":
                        record = module.get_operation(connection, command["op_id"])
                    elif action == "running":
                        record = module.mark_operation_running(
                            connection, command["op_id"], base_revision=command["base"]
                        )
                    elif action == "failed":
                        record = module.mark_operation_failed(
                            connection,
                            command["op_id"],
                            error_class=command["error_class"],
                            error_message=command["message"],
                        )
                    elif action == "conflict":
                        record = module.mark_operation_conflict(
                            connection, command["op_id"], error_message=command["message"]
                        )
                    elif action == "succeeded":
                        record = module.mark_operation_succeeded(
                            connection,
                            command["op_id"],
                            result_revision=command["revision"],
                            result=command["result"],
                        )
                    else:
                        item["records"] = [
                            asdict(record)
                            for record in module.list_interrupted_operations(connection)
                        ]
                        results.append(item)
                        continue
                    item["record"] = asdict(record)
                except Exception as exc:
                    item["error"] = type(exc).__name__
                results.append(item)
        rows = [dict(row) for row in connection.execute("SELECT * FROM operations ORDER BY op_id")]
        connection.close()
    return {"cases": results, "rows": rows}
