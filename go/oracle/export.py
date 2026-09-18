"""Export synthetic parity fixtures from the pinned Python/uMCP references.

Test tooling only; never imported or executed by the Go runtime.
"""

from __future__ import annotations

import argparse
import importlib
import json
import subprocess
import sys
from dataclasses import asdict
from pathlib import Path
from typing import Any

ROOT = Path(__file__).resolve().parents[2]
BASE = "0b0b8f94dd8b0410a0e3c0fd547e995d2b739b41"
UMCP_PIN = "9c89a708d14ae804e32aa65de10af7c02922617d"
UMCP_TIP = "30cce7dfe08c6ee63de235f7d81754ba286dafbb"
OUT = ROOT / "go/testdata/parity"


def git(path: Path, *args: str) -> str:
    return subprocess.check_output(["git", "-C", str(path), *args], text=True).strip()


def save(name: str, value: Any) -> None:
    (OUT / name).write_text(json.dumps(value, ensure_ascii=False, indent=2, sort_keys=True) + "\n")


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--umcp", type=Path, required=True)
    args = parser.parse_args()
    if git(args.umcp, "rev-parse", "HEAD") != UMCP_TIP:
        raise SystemExit("uMCP reference HEAD does not match pinned tip")
    for path in ("src", "rust", "pyproject.toml"):
        # Include working-tree changes in the comparison; never bless altered oracles.
        if git(ROOT, "diff", BASE, "--", path):
            raise SystemExit(f"reference {path} differs from pinned baseline")
    if git(args.umcp, "diff", UMCP_TIP, "--", "umcp.py", "aioumcp.py", "umcp_shared.py"):
        raise SystemExit("uMCP source has uncommitted edits")
    sys.path.insert(0, str(ROOT / "src"))
    sys.path.insert(0, str(args.umcp))
    shared = importlib.import_module("umcp_shared")
    extra = importlib.import_module("umcp_contracts")
    envelopes = importlib.import_module("memento.envelopes")
    registry = importlib.import_module("memento.mcp_registry")
    executor = importlib.import_module("memento.executor")
    server = importlib.import_module("memento.server")
    OUT.mkdir(parents=True, exist_ok=True)
    save(
        "baseline.json",
        {
            "memento_commit": BASE,
            "memento_release": "0.5.9",
            "umcp_deployed_commit": UMCP_PIN,
            "umcp_reference_commit": UMCP_TIP,
            "umcp_tip_delta": "documentation only; runtime unchanged from deployed pin",
            "fixture_data": "synthetic/public model fixtures only; no production content",
            "gte_original_go_commit": "d2ffa3a5aaf7be72b178970f48c835c0d8fda5bf",
            "gte_tokenizer_compatibility": "original Go algorithms with Memento Unicode character-boundary fix; no assembly/fast-math",
        },
    )
    cases: list[dict[str, Any]] = [
        {
            "name": "success-default",
            "kind": "success",
            "input": {"data": {"ready": True}, "repo_revision": "r1", "index_revision": "r1"},
        },
        {
            "name": "success-null-data",
            "kind": "success",
            "input": {
                "data": None,
                "repo_revision": "r1",
                "index_revision": "r0",
                "index_stale": True,
                "warnings": ["index delayed"],
                "next_tools": ["memory_status"],
                "operation_id": "op-1",
            },
        },
        {
            "name": "error-default",
            "kind": "error",
            "input": {"error_class": "forbidden", "message": "denied"},
        },
        {
            "name": "error-revisions",
            "kind": "error",
            "input": {
                "error_class": "conflict",
                "message": "changed",
                "repo_revision": "r1",
                "index_revision": "r0",
                "index_stale": True,
                "operation_id": "op-2",
                "warnings": ["reconcile"],
            },
        },
    ]
    for case in cases:
        fn = envelopes.success_envelope if case["kind"] == "success" else envelopes.error_envelope
        case["expected"] = fn(**case["input"]).model_dump(mode="json")
    save("envelopes.json", cases)
    ids: list[Any] = [None, "", "req", 0, -1, 10**40, True, False, 1.0, 1.5, [], {}]
    responses: list[Any] = [
        {},
        [],
        None,
        {"result": 1},
        {"id": None, "result": None},
        {"id": True, "result": 1},
        {"id": 1.0, "result": 1},
        {"id": 10**40, "result": {"ok": True}},
        {"id": "x", "error": {"code": -1, "message": ""}},
        {"id": "x", "error": {"code": True, "message": "bad"}},
        {"id": "x", "error": {"code": 1.0, "message": "bad"}},
        {"id": "x", "error": {"code": 1, "message": None}},
        {"id": "x", "error": None},
        {"id": "x", "error": {}},
        {"id": 1},
        {"id": 1, "result": None, "error": None},
    ]
    save(
        "umcp-shared.json",
        {
            "ids": [{"input": value, "valid": shared.is_valid_jsonrpc_id(value)} for value in ids],
            "responses": [
                {
                    "input": value,
                    "valid": isinstance(value, dict) and shared.is_valid_jsonrpc_response(value),
                }
                for value in responses
            ],
            "versions": [
                {
                    "accepted": value,
                    "preferred": "preferred-custom",
                    "negotiated": shared.exact_or_fallback(value, "preferred-custom"),
                    "error": shared.protocol_version_error(value),
                }
                for value in [None, "", "unknown", "2025-03-26", "2024-11-05"]
            ],
        },
    )
    ops = []
    for spec in registry.OPERATION_SPECS:
        item = asdict(spec)
        item["discovery_surfaces"] = sorted(spec.discovery_surfaces)
        ops.append(item)
    save("mcp-registry.json", ops)
    save("execute-schema.json", executor.execute_plan_schema())
    save(
        "tool-argument-schemas.json",
        {name: model.model_json_schema() for name, model in server._TOOL_ARG_MODELS.items()},
    )
    save(
        "surfaces.json",
        [
            {
                "surface": surface,
                "answer": answer,
                "route": route,
                "tools": registry.tool_names_for_surface(
                    surface, answer_enabled=answer, route_enabled=route
                ),
            }
            for surface in ["compact", "standard", "read_only", "curator", "admin"]
            for answer in [False, True]
            for route in [False, True]
        ],
    )
    save("umcp-http-rules.json", extra.shared_fixtures(shared))
    save("umcp-dispatch.json", extra.dispatch_fixtures(shared))
    save("umcp-progress.json", extra.progress_fixtures(shared))
    pagination = importlib.import_module("umcp_pagination")
    save("umcp-pagination.json", pagination.fixtures(shared))
    stdio = importlib.import_module("umcp_stdio")
    save("umcp-stdio.json", stdio.fixtures(shared))
    urls = importlib.import_module("umcp_urls")
    save("umcp-urls.json", urls.fixtures(shared))
    results = importlib.import_module("umcp_results")
    save("umcp-tool-results.json", results.fixtures())
    tools = importlib.import_module("umcp_tools")
    save("umcp-tools.json", tools.fixtures(shared))
    print(f"Exported pinned synthetic fixtures to {OUT}")


if __name__ == "__main__":
    main()
