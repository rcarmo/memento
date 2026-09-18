"""Pin derived lifecycle recovery and inode-cached migration boundaries."""

from __future__ import annotations

import importlib
import sqlite3
import tempfile
from dataclasses import asdict
from pathlib import Path
from typing import Any
from unittest.mock import patch


def fixtures() -> list[dict[str, Any]]:
    module: Any = importlib.import_module("memento.derived.index")
    cases = []
    for scenario in [
        "missing",
        "zero",
        "invalid-write",
        "invalid-read",
        "unsupported-write",
        "unsupported-read",
        "cached-version",
        "replaced-inode",
        "missing-table",
        "bad-concept",
        "quarantined-write",
        "quarantined-read",
        "query-error",
        "busy",
    ]:
        with tempfile.TemporaryDirectory(prefix="memento-lifecycle-") as directory:
            root = Path(directory)
            bundle = root / "bundle"
            bundle.mkdir()
            file = root / "index.sqlite"
            index = module.DerivedIndex(file)
            if scenario.startswith("invalid"):
                file.write_bytes(b"invalid header")
            elif scenario == "zero":
                file.touch()
            elif scenario != "missing":
                connection = index._connect()
                connection.close()
            statements = []
            if scenario.startswith("unsupported") or scenario == "cached-version":
                statements = ["UPDATE index_state SET value='99' WHERE key='schema_version'"]
            elif scenario == "missing-table":
                statements = ["DROP TABLE graph_metrics"]
            elif scenario.startswith("quarantined"):
                statements = ["UPDATE index_state SET value='quarantined' WHERE key='status'"]
            for statement in statements:
                with sqlite3.connect(file) as connection:
                    connection.execute(statement)
            if scenario.startswith("unsupported"):
                index = module.DerivedIndex(file)
            if scenario == "replaced-inode":
                file.rename(root / "old.sqlite")
                with sqlite3.connect(file) as connection:
                    connection.execute("CREATE TABLE marker(value TEXT)")
            if scenario == "bad-concept":
                (bundle / "bad.md").write_text("invalid")
            item: dict[str, Any] = {"scenario": scenario, "statements": statements}
            with patch.object(module.time, "time", lambda: 1789689600.125):
                try:
                    if scenario == "busy":
                        index._with_quarantine(
                            lambda _: (_ for _ in ()).throw(
                                sqlite3.OperationalError("database is locked")
                            )
                        )
                    elif scenario in {"invalid-read", "unsupported-read", "quarantined-read"}:
                        item["state"] = asdict(index.get_state())
                    elif scenario == "missing-table":
                        index.metrics("unknown")
                    elif scenario == "quarantined-write":
                        index.update_paths(bundle, repo_revision="r", changed_paths=())
                    elif scenario == "query-error":
                        authz: Any = importlib.import_module("memento.authz")
                        index.search(
                            policy=authz.EffectivePolicy("reader", ("reader",), ("/",), (), ()),
                            query='"unterminated',
                        )
                    else:
                        index.rebuild(bundle, repo_revision="r")
                except Exception as exc:
                    item["error_type"] = type(exc).__name__
                    item["error"] = str(exc)
            item["files"] = sorted(path.name for path in root.glob("*.sqlite"))
            item["quarantine_files"] = sorted(
                path.name for path in root.glob("*.quarantine-*.sqlite")
            )
            try:
                with sqlite3.connect(file) as connection:
                    state = dict(connection.execute("SELECT key,value FROM index_state"))
                    if "quarantine_path" in state:
                        state["quarantine_path"] = Path(state["quarantine_path"]).name
                    item["final_state"] = state
            except sqlite3.DatabaseError:
                item["final_state"] = None
            cases.append(item)
    return cases
