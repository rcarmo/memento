"""Deterministic MemoryExecutor loop fixtures with synthetic dispatch."""

from __future__ import annotations

import importlib
from dataclasses import dataclass
from typing import Any, Literal
from unittest.mock import patch


@dataclass(frozen=True)
class Reply:
    status: Literal["success", "error"]
    data: dict[str, Any] | None = None
    error_class: str = "validation_error"
    message: str = "failed"
    repo_revision: str | None = None
    index_revision: str | None = None
    operation_id: str | None = None


@dataclass(frozen=True)
class Scenario:
    name: str
    plan: dict[str, Any]
    replies: tuple[Reply, ...]
    clock: tuple[float, ...] = (0.0,) * 20
    limits: dict[str, Any] | None = None


class Service:
    def __init__(self, replies: tuple[Reply, ...], envelopes: Any) -> None:
        self.replies = list(replies)
        self.envelopes = envelopes
        self.calls: list[dict[str, Any]] = []

    def __getattr__(self, name: str) -> Any:
        if not name.startswith("memory_"):
            raise AttributeError(name)

        def call(context: Any, **args: Any) -> Any:
            del context
            self.calls.append({"method": name, "args": args})
            reply = self.replies.pop(0)
            if reply.status == "error":
                return self.envelopes.error_envelope(
                    reply.error_class,
                    reply.message,
                    repo_revision=reply.repo_revision,
                    index_revision=reply.index_revision,
                    operation_id=reply.operation_id,
                )
            return self.envelopes.success_envelope(
                reply.data or {},
                repo_revision=reply.repo_revision or "r",
                index_revision=reply.index_revision or "i",
                operation_id=reply.operation_id,
            )

        return call

    def _success(self, data: dict[str, Any], *, warnings: tuple[str, ...] = ()) -> Any:
        return self.envelopes.success_envelope(
            data, repo_revision="final", index_revision="index", warnings=warnings
        )


def reply_json(reply: Reply) -> dict[str, Any]:
    return {
        "status": reply.status,
        "data": reply.data,
        "error_class": reply.error_class,
        "message": reply.message,
        "repo_revision": reply.repo_revision,
        "index_revision": reply.index_revision,
        "operation_id": reply.operation_id,
    }


def fixtures() -> list[dict[str, Any]]:
    executor: Any = importlib.import_module("memento.executor")
    envelopes: Any = importlib.import_module("memento.envelopes")
    success = Reply(
        "success",
        data={"rows": [{"path": "/a"}, {"path": "/b"}]},
        operation_id="op",
    )
    failure = Reply("error")
    scenarios: tuple[Scenario, ...] = (
        Scenario("empty", {"operations": []}, ()),
        Scenario(
            "saved_reference",
            {
                "operations": [
                    {"op": "read", "args": {"id_or_path": "/a"}, "save_as": "first"},
                    {"op": "read", "args": {"id_or_path": "$first.rows.0.path"}},
                ]
            },
            (success, success),
        ),
        Scenario(
            "stop_on_error",
            {
                "operations": [
                    {"op": "search", "args": {"query": "x"}, "save_as": "bad"},
                    {"op": "read", "args": {"id_or_path": "/a"}},
                ]
            },
            (failure, success),
        ),
        Scenario(
            "continue_on_error",
            {
                "operations": [
                    {"op": "search", "args": {"query": "x"}, "save_as": "bad"},
                    {"op": "read", "args": {"id_or_path": "/a"}},
                ],
                "stop_on_error": False,
            },
            (failure, success),
        ),
        Scenario(
            "deadline_before_commit",
            {"operations": [{"op": "read", "args": {"id_or_path": "/a"}}]},
            (success,),
            (0.0, 4.0),
        ),
        Scenario(
            "deadline_after_commit",
            {
                "operations": [
                    {
                        "op": "create",
                        "args": {
                            "path": "/a",
                            "concept_type": "note",
                            "title": "t",
                            "body": "b",
                            "expected_revision": "r",
                            "idempotency_key": "k",
                        },
                    }
                ]
            },
            (success,),
            (0.0, 0.0, 4.0),
        ),
        Scenario(
            "post_commit_projection_error",
            {
                "operations": [
                    {
                        "op": "create",
                        "args": {
                            "path": "/a",
                            "concept_type": "note",
                            "title": "t",
                            "body": "b",
                            "expected_revision": "r",
                            "idempotency_key": "k",
                        },
                    }
                ],
                "returns": [{"ref": "$missing"}],
            },
            (success,),
        ),
        Scenario(
            "failed_save_return_skipped",
            {
                "operations": [{"op": "search", "args": {"query": "x"}, "save_as": "bad"}],
                "returns": [{"ref": "$bad"}],
            },
            (failure,),
        ),
        Scenario(
            "output_limit_before_commit",
            {"operations": [{"op": "read", "args": {"id_or_path": "/a"}}]},
            (success,),
            limits={"max_output_bytes": 512},
        ),
        Scenario(
            "output_limit_after_commit",
            {
                "operations": [
                    {
                        "op": "create",
                        "args": {
                            "path": "/a",
                            "concept_type": "note",
                            "title": "t",
                            "body": "b",
                            "expected_revision": "r",
                            "idempotency_key": "k",
                        },
                    }
                ]
            },
            (Reply("success", data={"blob": "x" * 2000}),),
            limits={"max_output_bytes": 512},
        ),
        Scenario(
            "commit",
            {
                "operations": [
                    {
                        "op": "create",
                        "args": {
                            "path": "/a",
                            "concept_type": "note",
                            "title": "t",
                            "body": "b",
                            "expected_revision": "r",
                            "idempotency_key": "k",
                        },
                    }
                ]
            },
            (success,),
        ),
    )
    results: list[dict[str, Any]] = []
    for scenario in scenarios:
        service = Service(scenario.replies, envelopes)
        clock = iter(scenario.clock)
        with patch.object(executor, "monotonic", side_effect=clock.__next__):
            limits = executor.ExecuteLimits.model_validate(scenario.limits or {})
            result = executor.MemoryExecutor(service, limits).run(None, plan=scenario.plan)
        results.append(
            {
                "name": scenario.name,
                "plan": scenario.plan,
                "replies": [reply_json(reply) for reply in scenario.replies],
                "limits": scenario.limits,
                "result": result.model_dump(mode="json"),
                "calls": service.calls,
            }
        )
    return results
