"""Retention planning and mutation against source service with synthetic publisher."""

from __future__ import annotations

import base64
import importlib
import tempfile
import threading
from dataclasses import asdict
from pathlib import Path
from types import SimpleNamespace
from typing import Any
from unittest.mock import patch

from asset_get import fixtures as read_fixtures


def fixtures() -> list[dict[str, Any]]:
    service: Any = importlib.import_module("memento.service")
    db: Any = importlib.import_module("memento.control.db")
    proposals: Any = importlib.import_module("memento.control.proposals")
    transactions: Any = importlib.import_module("memento.repository.transactions")
    authz: Any = importlib.import_module("memento.authz")
    files = read_fixtures()["files"]
    scenarios = [
        "prune",
        "no-op",
        "invalid-keep",
        "reader",
        "denied-path",
        "denied-id",
        "id-write-only",
        "missing",
        "invalid-kind",
        "empty",
        "submitted",
        "approved",
        "draft",
        "rejected",
        "expired",
        "stale",
        "needs_rebase",
        "applied",
        "conflicted",
        "protected-no-op",
        "transaction-error",
        "expected-error",
        "repeat",
        "missing-zip",
        "unlink-failure",
        "protect-old",
        "replay",
        "keep-true",
        "keep-false",
    ]
    cases = []
    for scenario in scenarios:
        with tempfile.TemporaryDirectory(prefix="memento-prune-") as directory:
            root = Path(directory) / "current"
            root.mkdir()
            for name, encoded in files.items():
                target = root / name[1:]
                target.parent.mkdir(parents=True, exist_ok=True)
                target.write_bytes(base64.b64decode(encoded))
            scenario_files = dict(files)
            if scenario == "protect-old":
                for extension in ("json", "zip"):
                    source = "/.assets/12345678/docs/1.10.0." + extension
                    destination = "/.assets/12345678/docs/2.0.0." + extension
                    content = base64.b64decode(files[source])
                    if extension == "json":
                        content = content.replace(b"1.10.0", b"2.0.0")
                    (root / destination[1:]).write_bytes(content)
                    scenario_files[destination] = base64.b64encode(content).decode()
            connection = db.connect_control_db(Path(directory) / "control.sqlite")
            db.migrate_control_db(connection)
            status = "submitted" if scenario in {"protected-no-op", "protect-old"} else scenario
            protected_statuses = {
                "submitted",
                "approved",
                "draft",
                "rejected",
                "expired",
                "stale",
                "needs_rebase",
                "applied",
                "conflicted",
            }
            if status in protected_statuses:
                proposals.create_proposal(
                    connection,
                    proposal_id="active",
                    author_principal="other",
                    client_instance_id=None,
                    base_revision="base",
                    intent="synthetic",
                    rationale=None,
                    patch={"changes": []},
                    assets=(
                        proposals.ProposalAssetInput(
                            asset_id="asset",
                            concept_path="/public/a.md",
                            asset_kind="docs",
                            version="1.9.0",
                            media_type="application/zip",
                            sha256="a" * 64,
                            blob_bytes=b"synthetic",
                            manifest_json="{}",
                        ),
                    ),
                )
                connection.execute(
                    "UPDATE proposals SET status=?,expires_at='2000-01-01T00:00:00Z',created_at='2026-09-18T00:00:00Z',updated_at='2026-09-18T00:00:00Z'",
                    (status,),
                )
                connection.commit()
            before = [asdict(row) for row in proposals.list_proposals(connection)]
            roles = ("reader",) if scenario == "reader" else ("curator",)
            policy = authz.EffectivePolicy(
                "actor",
                roles,
                () if scenario == "id-write-only" else ("/",),
                () if scenario.startswith("denied") else ("/",),
                (),
            )
            args: dict[str, Any] = {
                "id_or_path": "/public/a.md",
                "asset_kind": "docs",
                "keep": 1,
                "expected_revision": "main",
                "idempotency_key": "key",
            }
            if scenario in {"denied-id", "id-write-only"}:
                args["id_or_path"] = "12345678"
            if scenario == "missing":
                args["id_or_path"] = "missing"
            if scenario == "empty":
                args["id_or_path"] = "/public/empty.md"
            if scenario == "invalid-kind":
                args["asset_kind"] = "INVALID"
            if scenario == "invalid-keep":
                args["keep"] = 0
            if scenario.startswith("keep-"):
                args["keep"] = scenario == "keep-true"
            if scenario == "no-op":
                args["keep"] = 5
            if scenario in {"no-op", "protected-no-op", "expected-error"}:
                args["expected_revision"] = "old"
            missing_zip = "/.assets/12345678/docs/1.9.0.zip" if scenario == "missing-zip" else None
            if missing_zip:
                (root / missing_zip[1:]).unlink()
            if scenario == "unlink-failure":
                target = root / ".assets/12345678/docs/1.9.0.zip"
                target.unlink()
                target.mkdir()
            state: Any = SimpleNamespace(revision="main", requests=[], changed=[])

            def apply(
                request: Any,
                mutate: Any,
                state: Any = state,
                scenario: str = scenario,
                root: Path = root,
            ) -> Any:
                state.requests.append(asdict(request))
                if scenario == "transaction-error":
                    raise service.ConflictError("synthetic transaction failure")
                if scenario == "replay":
                    return SimpleNamespace(
                        replayed=True,
                        result_revision="previous",
                        operation=SimpleNamespace(op_id="original"),
                    )
                if request.expected_revision != state.revision:
                    raise service.ConflictError("synthetic expected revision mismatch")
                state.changed = list(mutate(root))
                state.revision = "result"
                return SimpleNamespace(
                    replayed=False,
                    result_revision="result",
                    operation=SimpleNamespace(op_id=request.operation.op_id),
                )

            memory = object.__new__(service.MemoryService)
            memory._deps = SimpleNamespace(
                repo_paths=SimpleNamespace(current_dir=root),
                control_connection=connection,
                transaction_manager=SimpleNamespace(apply=apply),
            )
            memory._policy = lambda _, policy=policy: policy
            context = SimpleNamespace(
                principal=SimpleNamespace(name="actor"),
                client_instance_id="client",
                mcp_session_id="session",
                source_chat="chat",
            )
            steps = []
            with (
                patch.object(service, "get_main_revision", lambda _, state=state: state.revision),
                patch.object(service, "uuid4", lambda: "00000000-0000-4000-8000-000000000000"),
                patch.object(transactions, "_transaction_lock", lambda _: threading.RLock()),
            ):
                for _ in range(2 if scenario == "repeat" else 1):
                    try:
                        result = memory.memory_asset_prune(context, **args).model_dump(mode="json")
                        exception = None
                    except Exception as exc:
                        result = None
                        exception = type(exc).__name__
                    remaining = sorted(
                        "/" + str(p.relative_to(root)) for p in root.rglob("*") if p.is_file()
                    )
                    steps.append(
                        {
                            "expected": result,
                            "exception": exception,
                            "remaining": remaining,
                            "changed": list(state.changed),
                            "requests": list(state.requests),
                        }
                    )
            cases.append(
                {
                    "scenario": scenario,
                    "files": scenario_files,
                    "proposals": before,
                    "policy": asdict(policy),
                    "arguments": args,
                    "missing_zip": missing_zip,
                    "steps": steps,
                }
            )
            connection.close()
    return cases
