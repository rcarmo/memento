from __future__ import annotations

import importlib.util
import json
from pathlib import Path
from types import ModuleType


def _generator(root: Path) -> ModuleType:
    path = root / "tools/experiments/needle/generate_router_v2.py"
    spec = importlib.util.spec_from_file_location("memento_generate_router_v2", path)
    if spec is None or spec.loader is None:
        raise RuntimeError(f"could not load {path}")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def test_router_v2_generator_matches_vendored_corpus_manifest() -> None:
    root = Path(__file__).parents[1]
    generator = _generator(root)
    expected = json.loads(
        (root / "docs/evidence/needle/router-v2-manifest.json").read_text(encoding="utf-8")
    )
    manifest, rows = generator.build_manifest_and_rows()
    assert manifest == expected
    assert {name: len(items) for name, items in rows.items()} == {
        "train": 1440,
        "val": 360,
        "test": 360,
    }
