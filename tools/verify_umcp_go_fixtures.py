#!/usr/bin/env python3
from __future__ import annotations

import argparse
import filecmp
import os
import shutil
import subprocess
import sys
import tempfile
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--umcp", type=Path, required=True)
    args = parser.parse_args()
    parent = ROOT / "go/testdata/parity"
    module = ROOT / "go/umcp/testdata/parity"
    with tempfile.TemporaryDirectory() as temp:
        backup = Path(temp)
        files = sorted(parent.glob("umcp-*.json"))
        for path in files:
            shutil.copy2(path, backup / path.name)
        env = dict(os.environ)
        env["PYTHONPATH"] = str(ROOT / "src") + os.pathsep + env.get("PYTHONPATH", "")
        subprocess.run(
            [sys.executable, str(ROOT / "go/oracle/export.py"), "--umcp", str(args.umcp)],
            check=True,
            env=env,
        )
        for original in sorted(backup.glob("umcp-*.json")):
            generated = parent / original.name
            owned = module / original.name
            if not filecmp.cmp(original, generated, shallow=False):
                raise SystemExit(f"regenerated fixture changed: {original.name}")
            if not filecmp.cmp(original, owned, shallow=False):
                raise SystemExit(f"module fixture differs: {original.name}")
    print(f"verified {len(files)} uMCP fixture families")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
