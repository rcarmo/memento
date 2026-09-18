"""Test-only interoperability check using the unchanged Python embedding client."""

from __future__ import annotations

import argparse
import importlib
import json
import math
import sys
from pathlib import Path
from typing import Any

ROOT = Path(__file__).resolve().parents[2]


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--worker", type=Path, required=True)
    parser.add_argument("--model", type=Path, required=True)
    args = parser.parse_args()
    sys.path.insert(0, str(ROOT / "src"))
    module: Any = importlib.import_module("memento.subprocess_embeddings")
    client = module.SubprocessEmbeddingClient(
        args.worker.resolve(),
        args.model.resolve(),
        dimensions=384,
        max_batch=16,
        max_input_chars=8192,
        timeout_seconds=120,
    )
    fixtures = json.loads((ROOT / "go/testdata/parity/gte-real.json").read_text())
    for fixture in fixtures:
        if client.model_info().revision != fixture["model_sha256"]:
            raise AssertionError("worker model identity differs")
        outputs = client.embed_batch(fixture["texts"])
        for text, got, expected in zip(fixture["texts"], outputs, fixture["outputs"], strict=True):
            maximum = max(abs(a - b) for a, b in zip(got, expected, strict=True))
            norm = math.sqrt(sum(v * v for v in got))
            if maximum > 1e-5 or abs(norm - 1) > 1e-5:
                raise AssertionError(f"worker output differs for {text!r}: {maximum}, {norm}")
        print(
            f"Unchanged Python client accepted {len(outputs)} Go worker embeddings and model identity"
        )


if __name__ == "__main__":
    main()
