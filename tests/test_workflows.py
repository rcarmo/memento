from __future__ import annotations

from collections.abc import Iterator
from pathlib import Path
from typing import Any, cast

import yaml  # type: ignore[import-untyped]

ROOT = Path(__file__).parents[1]
RUNTIME_PATHS = {
    "models/gte/gte-small.gtemodel",
    "models/needle/memento-router.ndl",
    "models/needle/needle.model",
}


def workflow(name: str) -> dict[str, Any]:
    payload = yaml.safe_load((ROOT / ".github" / "workflows" / name).read_text(encoding="utf-8"))
    return cast(dict[str, Any], payload)


def steps(document: dict[str, Any]) -> Iterator[dict[str, Any]]:
    for job in document["jobs"].values():
        yield from cast(list[dict[str, Any]], job.get("steps", []))


def test_workflows_never_contact_git_lfs() -> None:
    for name in ("ci.yml", "release.yml"):
        for step in steps(workflow(name)):
            if str(step.get("uses", "")).startswith("actions/checkout@"):
                assert step.get("with", {}).get("lfs") is not True
            assert "git lfs " not in str(step.get("run", ""))


def test_each_workflow_prepares_one_verified_runtime_model_artifact() -> None:
    for name in ("ci.yml", "release.yml"):
        document = workflow(name)
        commands = [
            str(step.get("run", ""))
            for step in steps(document)
            if "prepare_runtime_models.py" in str(step.get("run", ""))
        ]
        assert sum("--key-only" in command for command in commands) == 1
        assert sum("--key-only" not in command for command in commands) == 1
        model_job = document["jobs"]["runtime-models"]
        cache = next(
            step
            for step in model_job["steps"]
            if str(step.get("uses", "")).startswith("actions/cache@")
        )
        cached_paths = set(str(cache["with"]["path"]).splitlines())
        assert cached_paths == RUNTIME_PATHS


def test_container_builders_download_prepared_model_artifact() -> None:
    ci = workflow("ci.yml")["jobs"]["container"]
    release = workflow("release.yml")["jobs"]["build"]
    assert any(step.get("with", {}).get("name") == "runtime-models" for step in ci["steps"])
    assert any(
        step.get("with", {}).get("name") == "release-runtime-models" for step in release["steps"]
    )


def test_training_and_checkpoint_paths_are_absent_from_workflows() -> None:
    text = "\n".join(
        (ROOT / ".github" / "workflows" / name).read_text(encoding="utf-8")
        for name in ("ci.yml", "release.yml")
    )
    for path in (
        "memento-router.pkl",
        "train.jsonl",
        "train-hard.jsonl",
        "test.jsonl",
        "val.jsonl",
        "needle.vocab",
    ):
        assert path not in text
