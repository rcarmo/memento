#!/usr/bin/env python3
from __future__ import annotations

import argparse
import ast
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]


def inventory(reference: Path) -> str:
    lines = []
    for filename in ("umcp.py", "aioumcp.py", "umcp_shared.py"):
        tree = ast.parse((reference / filename).read_text())
        for node in tree.body:
            if isinstance(
                node, (ast.FunctionDef, ast.AsyncFunctionDef)
            ) and not node.name.startswith("_"):
                lines.append(f"{filename} {node.name}")
            if isinstance(node, ast.ClassDef) and not node.name.startswith("_"):
                for method in node.body:
                    if isinstance(
                        method, (ast.FunctionDef, ast.AsyncFunctionDef)
                    ) and not method.name.startswith("_"):
                        lines.append(f"{filename} {node.name}.{method.name}")
    return "\n".join(lines) + "\n"


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--umcp", type=Path, required=True)
    args = parser.parse_args()
    expected = (ROOT / "go/umcp/testdata/python-public-api.txt").read_text()
    actual = inventory(args.umcp)
    if actual != expected:
        raise SystemExit("pinned Python uMCP public API inventory changed")
    print(f"verified {len(actual.splitlines())} public Python symbols")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
