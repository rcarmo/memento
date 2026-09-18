"""Explicit simple argument contracts and actual lax/strict model validation."""

from __future__ import annotations

import hashlib
import importlib
import json
import platform
import random
from pathlib import Path
from typing import Any

import pydantic
import pydantic_core


def fixtures() -> dict[str, Any]:
    module: Any = importlib.import_module("memento.executor")
    source = Path(__file__).resolve().parents[2] / "src/memento/executor.py"
    if Path(module.__file__).resolve() != source:
        raise SystemExit("Executor oracle imported a different worktree")
    definitions = []
    cases = []
    for name, operation in module._OPERATION_MODELS.items():
        if name in {"propose", "compare_manifest"}:
            continue
        model = operation.model_fields["args"].annotation
        schema = model.model_json_schema()
        fields = []
        baseline: dict[str, Any] = {}
        for key, field in model.model_fields.items():
            prop = dict(schema["properties"][key])
            prop.pop("title", None)
            if "anyOf" in prop:
                branches = prop.pop("anyOf")
                prop.update(next(b for b in branches if b.get("type") != "null"))
                prop["nullable"] = True
            prop["required"] = field.is_required()
            prop["name"] = key
            fields.append(prop)
            if field.is_required():
                baseline[key] = (
                    prop.get("enum", ["synthetic"])[0] if prop["type"] == "string" else [0]
                )
        definitions.append({"operation": name, "fields": fields})
        item = module._PlannedOperation(op=name, args={})

        def record(args: dict[str, Any], strict: bool, name: str = name, item: Any = item) -> None:
            try:
                validated = module._validated_operation(item, args=args, strict=strict)
                # Tuples are the reference Python representation; Go uses the
                # wire-equivalent JSON arrays without rounding any integers.
                result = {"expected": json.loads(json.dumps(validated))}
            except module.ValidationError as exc:
                result = {
                    "error": module._validation_message(exc, label=f"operation 1 ({name})"),
                    "issues": [
                        {"path": ".".join(map(str, error["loc"])), "message": error["msg"]}
                        for error in exc.errors()
                    ],
                }
            cases.append({"operation": name, "arguments": args, "strict": strict, **result})

        for strict in [False, True]:
            record({}, strict)
            record(baseline, strict)
            record({**baseline, "unknown": 1}, strict)
            for field in fields:
                key = field["name"]
                values: list[Any] = [
                    None,
                    True,
                    False,
                    0,
                    1,
                    -1,
                    1.0,
                    1.5,
                    "",
                    "2",
                    "false",
                    [],
                    {},
                    ["x"],
                    [1],
                ]
                if field["type"] == "integer":
                    values += [
                        "1_000",
                        "2.0",
                        " 2 ",
                        10**40,
                        -(10**40),
                        "1_",
                        "_1",
                        "1__0",
                        "1_0.0",
                        "1.0_0",
                        "-0",
                        "1e1",
                        "\u20031\u2003",
                        "\x1c1\x1c",
                        "\x851\x85",
                        "١",
                        "０",
                        "0" * 4301 + "1",
                        "1" * 4301,
                        2.0**63,
                        -(2.0**63),
                    ]
                    for bound in ["minimum", "maximum"]:
                        if bound in field:
                            values.extend([field[bound] - 1, field[bound], field[bound] + 1])
                if field["type"] == "string":
                    values += field.get("enum", [])
                    if "minLength" in field:
                        values += ["a" * field["minLength"]]
                    if "maxLength" in field:
                        values += ["é" * field["maxLength"], "é" * (field["maxLength"] + 1)]
                    if "pattern" in field:
                        values += [
                            "a" * 64,
                            "A" * 64,
                            "a" * 64 + "\n",
                            "1.2.3",
                            "01.2.3",
                            "1.2.3\n",
                            "1٢.3.4",
                            "١.2.3",
                            "a-b",
                            "a--b",
                            "a-b\n",
                        ]
                if field["type"] == "array":
                    values += [
                        ["x", None, 1],
                        [True, "2", 1.5],
                        ["path", "title", "path"],
                        [0, 1, 0],
                    ]
                for value in values:
                    record({**baseline, key: value}, strict)
            # Fixed-seed, multi-field perturbations check ordering and that
            # after-model validators do not run following field failures.
            rng = random.Random(428)
            for _ in range(24):
                mixed = dict(baseline)
                for field in fields:
                    if rng.randrange(3) == 0:
                        mixed[field["name"]] = rng.choice([None, 0, True, [], {}, "x", "2"])
                record(mixed, strict)
            if name == "asset_metadata":
                scope_cases: list[dict[str, Any]] = [
                    {"id_or_path": "a", "path_prefix": "/"},
                    {"id_or_path": "a", "cursor": "c"},
                    {"id_or_path": "a", "cursor": "cccc"},
                    {"id_or_path": "a", "path_prefix": "/", "cursor": "cccc", "version": "1.0.0"},
                    {"id_or_path": "a", "path_prefix": "/", "limit": 0},
                    {"id_or_path": "a", "path_prefix": "/", "unknown": True},
                    {"version": "1.0.0"},
                    {"version": "1.0.0", "asset_kind": "docs"},
                ]
                for extra in scope_cases:
                    record(extra, strict)
    return {
        "source": {
            "path": "src/memento/executor.py",
            "sha256": hashlib.sha256(source.read_bytes()).hexdigest(),
            "python": platform.python_version(),
            "pydantic": pydantic.__version__,
            "pydantic_core": pydantic_core.__version__,
        },
        "definitions": definitions,
        "cases": cases,
    }
