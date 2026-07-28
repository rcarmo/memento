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


def test_workflows_never_use_implicit_full_lfs_checkout() -> None:
    for name in ("ci.yml", "release.yml"):
        for step in steps(workflow(name)):
            if str(step.get("uses", "")).startswith("actions/checkout@"):
                assert step.get("with", {}).get("lfs") is not True


def test_each_workflow_fetches_only_runtime_models_once() -> None:
    for name in ("ci.yml", "release.yml"):
        document = workflow(name)
        fetches = [
            step["run"]
            for step in steps(document)
            if "git lfs fetch origin" in str(step.get("run", ""))
        ]
        assert len(fetches) == 1
        command = fetches[0]
        assert all(path in command for path in RUNTIME_PATHS)
        assert "memento-router.pkl" not in command
        assert "train.jsonl" not in command
        assert "train-hard.jsonl" not in command
        assert "test.jsonl" not in command
        assert "val.jsonl" not in command
        assert "needle.vocab" not in command


def test_quality_jobs_fetch_only_tiny_tokenizer() -> None:
    for name, job_name in (("ci.yml", "test"), ("release.yml", "quality")):
        commands = "\n".join(
            str(step.get("run", "")) for step in workflow(name)["jobs"][job_name]["steps"]
        )
        assert 'git lfs pull --include="models/needle/needle.model"' in commands
        assert "gte-small.gtemodel" not in commands
        assert "memento-router.ndl" not in commands


def test_container_builders_download_prepared_model_artifact() -> None:
    ci = workflow("ci.yml")["jobs"]["container"]
    release = workflow("release.yml")["jobs"]["build"]
    assert any(step.get("with", {}).get("name") == "runtime-models" for step in ci["steps"])
    assert any(
        step.get("with", {}).get("name") == "release-runtime-models" for step in release["steps"]
    )
