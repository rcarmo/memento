"""Capture service exception mapping and explicit revision envelope defaults."""

from __future__ import annotations

import importlib
from types import SimpleNamespace
from typing import Any
from unittest.mock import patch


def fixtures() -> dict[str, Any]:
    module: Any = importlib.import_module("memento.service")
    authz: Any = importlib.import_module("memento.authz")
    operations: Any = importlib.import_module("memento.control.operations")
    assets: Any = importlib.import_module("memento.asset_retrieval")
    staged: Any = importlib.import_module("memento.staged_assets")
    packs: Any = importlib.import_module("memento.skill_packs")
    derived: Any = importlib.import_module("memento.derived.index")
    transactions: Any = importlib.import_module("memento.repository.transactions")
    runtime = object.__new__(module.MemoryService)
    runtime._deps = SimpleNamespace(repo_paths=None)
    errors: list[tuple[str, Exception]] = [
        ("auth", authz.AuthorizationError("denied")),
        ("forbidden", module.ForbiddenError("denied")),
        ("not-found", module.NotFoundError("missing")),
        ("conflict", module.ConflictError("conflict")),
        ("needs-rebase", module.NeedsRebaseError("stale")),
        ("service", module.ServiceError("bad request")),
        ("bundle", module.BundleError("bundle")),
        ("frontmatter", module.FrontmatterError("frontmatter")),
        ("path", module.PathSafetyError("path")),
        ("asset", assets.AssetReadError("asset")),
        ("staged", staged.StagedAssetError("staged")),
        ("pack", packs.SkillPackValidationError("pack")),
        ("search", derived.DerivedSearchError("query")),
        (
            "idempotency",
            operations.IdempotencyConflictError(
                "idempotency key already used for a different request"
            ),
        ),
        ("transaction", transactions.TransactionConflictError("transaction")),
        ("git", module.GitError("git")),
        ("unavailable", derived.DerivedIndexUnavailableError("busy")),
        ("file", KeyError("asset file is not declared in the manifest")),
        ("asset-id", KeyError(("proposal", "asset"))),
        ("os", OSError("I/O")),
        ("runtime", RuntimeError("unexpected")),
        ("corruption", derived.DerivedIndexCorruptionError("corrupt")),
        ("timeout", TimeoutError("late")),
    ]
    failures = []
    for name, exc in errors:
        item: dict[str, Any] = {"name": name}
        try:
            item["expected"] = runtime._failure(exc).model_dump(mode="json")
        except Exception as propagated:
            item["propagated"] = type(propagated).__name__
        failures.append(item)
    keys = []
    for key in [
        "simple",
        "quote'",
        "both'\"",
        "slash\\",
        "\n\r\t\x00\x1f\x7f",
        "é 中文 😀",
        "\u200b\U000e0001",
        "\xa0",
        "",
    ]:
        keys.append(
            {"key": key, "expected": runtime._failure(KeyError(key)).model_dump(mode="json")}
        )
    successes = []
    option_cases: list[dict[str, Any]] = [
        {},
        {"repo_revision": ""},
        {
            "repo_revision": "published",
            "index_revision": "old",
            "index_stale": True,
            "operation_id": "op",
            "warnings": ("warning",),
            "next_tools": ("memory_read",),
        },
        {"index_revision": ""},
    ]
    for options in option_cases:
        with patch.object(module, "get_main_revision", lambda _: "main"):
            successes.append(
                {
                    "options": options,
                    "expected": runtime._success(
                        {"items": [], "nothing": None, "large": 9007199254740993}, **options
                    ).model_dump(mode="json"),
                }
            )
    return {"failures": failures, "keys": keys, "successes": successes}
