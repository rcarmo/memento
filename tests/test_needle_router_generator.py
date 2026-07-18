from __future__ import annotations

import importlib.util
import json
from pathlib import Path
from types import ModuleType
from typing import Any

from memento.router import READ_FIELDS, ROUTER_ACTION_ADAPTER, STATUS_FIELDS


def _load_generator_module() -> ModuleType:
    path = (
        Path(__file__).resolve().parents[1]
        / "tools"
        / "experiments"
        / "needle"
        / "generate_router_v2.py"
    )
    spec = importlib.util.spec_from_file_location("generate_router_v2", path)
    assert spec is not None
    assert spec.loader is not None
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def test_generator_uses_valid_historical_router_field_enums() -> None:
    module = _load_generator_module()
    assert set(module.STATUS_FIELDS).issubset(set(STATUS_FIELDS))
    assert set(module.READ_FIELDS).issubset(set(READ_FIELDS))
    assert "frontmatter" not in module.READ_FIELDS


def test_generated_answers_validate_against_router_adapter() -> None:
    module = _load_generator_module()
    manifest, rows_by_split = module.build_manifest_and_rows()

    expected_counts = {"train": 1440, "val": 360, "test": 360}
    assert {split: info["count"] for split, info in manifest["splits"].items()} == expected_counts

    for rows in rows_by_split.values():
        for row in rows:
            answers = json.loads(row["answers"])
            assert len(answers) == 1
            answer = answers[0]
            payload: dict[str, Any] = {
                "action": answer["name"],
                **answer["arguments"],
            }
            parsed = ROUTER_ACTION_ADAPTER.validate_python(payload)
            assert parsed.action == answer["name"]
