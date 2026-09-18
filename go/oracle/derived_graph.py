"""Graph ACL traversal, raw/scoped metrics and revision-only freshness gates."""

from __future__ import annotations

import importlib
import tempfile
from dataclasses import asdict
from datetime import UTC, datetime
from pathlib import Path
from typing import Any
from unittest.mock import patch


def fixtures() -> dict[str, Any]:
    module: Any = importlib.import_module("memento.derived.index")
    authz: Any = importlib.import_module("memento.authz")
    schema: Any = importlib.import_module("memento.repository.schema")
    frontmatter: Any = importlib.import_module("memento.repository.frontmatter")
    policies = [
        authz.EffectivePolicy("reader", ("reader",), ("/",), (), ("/private/",)),
        authz.EffectivePolicy("reader", ("reader",), ("/", "/private/nested/"), (), ("/private/",)),
        authz.EffectivePolicy("admin", ("admin",), ("/",), (), ("/private/",)),
        authz.EffectivePolicy("limited", ("reader",), ("/public/",), (), ()),
    ]
    files = {}
    stamp = datetime(2026, 1, 1, tzinfo=UTC)
    for id, path, body in [
        (
            "a",
            "/public/a.md",
            "[b](/public/b.md) [b2](/public/b.md) [c](/public/c.md) [p](/private/p.md) [t](/trash/public/t.md) [missing](/public/missing.md) [again](/public/missing.md) [anchor](/public/missing.md#anchor) [secret missing](/private/missing.md) [relative](foo) [web](https://example.org) [self](/public/a.md)",
        ),
        ("b", "/public/b.md", "[d](/public/d.md) [missing](/private/missing.md)"),
        ("c", "/public/c.md", "[d](/public/d.md) [a](/public/a.md)"),
        ("d", "/public/d.md", "[a](/public/a.md)"),
        ("p", "/private/p.md", "[hidden bridge](/public/isolated.md) [a](/public/a.md)"),
        ("n", "/private/nested/n.md", "[a](/public/a.md)"),
        ("i", "/public/isolated.md", "[missing](/public/missing.md)"),
        ("t", "/trash/public/t.md", "[a](/public/a.md)"),
    ]:
        files[path] = frontmatter.serialize_concept(
            schema.ConceptDocument(
                frontmatter=schema.ConceptFrontmatter(
                    id=id,
                    type="concept",
                    title=id,
                    status="active",
                    created_at=stamp,
                    updated_at=stamp,
                    updated_by="actor",
                ),
                body=body,
            )
        )
    graphs = []
    statuses = []
    metrics = []
    waits = []
    with tempfile.TemporaryDirectory(prefix="memento-graph-") as directory:
        root = Path(directory) / "bundle"
        root.mkdir()
        for path, text in files.items():
            file = root / path.removeprefix("/")
            file.parent.mkdir(parents=True, exist_ok=True)
            file.write_text(text)
        index = module.DerivedIndex(Path(directory) / "index.sqlite")
        index.rebuild(root, repo_revision="r1")
        for policy in policies:
            statuses.append(
                {"policy": asdict(policy), "expected": asdict(index.status_snapshot(policy))}
            )
            for center in ["a", "p", "t", "i", "missing"]:
                for depth in [-1, 0, 1, 2, 3]:
                    item: dict[str, Any] = {
                        "policy": asdict(policy),
                        "center": center,
                        "depth": depth,
                    }
                    try:
                        item["expected"] = asdict(
                            index.graph(policy=policy, concept_id=center, depth=depth)
                        )
                    except KeyError as exc:
                        item["error_type"] = type(exc).__name__
                        item["error"] = str(exc)
                    graphs.append(item)
        for id in ["a", "b", "i", "t", "missing"]:
            item = {"id": id}
            try:
                item["expected"] = asdict(index.metrics(id))
            except Exception as exc:
                item["error_type"] = type(exc).__name__
            metrics.append(item)
        for status, repo, indexed, timeout in [
            ("ready", "r1", "r1", -1),
            ("quarantined", "r1", "r1", 0),
            ("building", "r1", "r1", 0),
            ("ready", "r2", "r1", 0),
            ("ready", "r2", "r1", 0.1),
        ]:
            state = module.DerivedIndexState(repo, indexed, "2", status, None)
            clock = [0.0]
            reads = [0]

            def get_state(
                state: Any = state, reads: list[int] = reads, timeout: float = timeout
            ) -> Any:
                reads[0] += 1
                if timeout > 0 and reads[0] > 1:
                    return module.DerivedIndexState("r2", "r2", "2", state.status, None)
                return state

            def sleep(seconds: float, clock: list[float] = clock) -> None:
                clock[0] += seconds

            item = {"state": asdict(state), "timeout": timeout}
            with (
                patch.object(index, "get_state", get_state),
                patch.object(module.time, "monotonic", lambda clock=clock: clock[0]),
                patch.object(module.time, "sleep", sleep),
            ):
                try:
                    item["expected"] = asdict(index.wait_for_freshness(timeout_seconds=timeout))
                except Exception as exc:
                    item["error"] = str(exc)
            item["reads"] = reads[0]
            waits.append(item)
    return {
        "files": files,
        "graphs": graphs,
        "statuses": statuses,
        "metrics": metrics,
        "waits": waits,
    }
