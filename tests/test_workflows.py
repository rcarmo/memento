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
NODE24_ACTIONS = {
    "actions/cache": "55cc8345863c7cc4c66a329aec7e433d2d1c52a9",
    "actions/upload-artifact": "043fb46d1a93c77aae656e7c1c64a875d1fc6a0a",
    "actions/download-artifact": "3e5f45b2cfb9172054b4087a40e8e0b5a5461e7c",
}


def workflow(name: str) -> dict[str, Any]:
    payload = yaml.safe_load((ROOT / ".github" / "workflows" / name).read_text(encoding="utf-8"))
    return cast(dict[str, Any], payload)


def steps(document: dict[str, Any]) -> Iterator[dict[str, Any]]:
    for job in document["jobs"].values():
        yield from cast(list[dict[str, Any]], job.get("steps", []))


def test_ci_runs_for_main_and_pull_requests_only_and_cancels_superseded_runs() -> None:
    ci = workflow("ci.yml")
    triggers = cast(dict[str, Any], ci.get("on") or ci.get(cast(Any, True)))
    assert triggers["push"] == {"branches": ["main"]}
    assert triggers["pull_request"] is None
    assert ci["concurrency"] == {
        "group": "ci-${{ github.workflow }}-${{ github.event.pull_request.number || github.ref }}",
        "cancel-in-progress": True,
    }


def test_release_remains_tag_scoped_and_never_cancels_running_releases() -> None:
    release = workflow("release.yml")
    triggers = cast(dict[str, Any], release.get("on") or release.get(cast(Any, True)))
    assert triggers["push"] == {"tags": ["v*"]}
    assert "workflow_dispatch" in triggers
    assert release["concurrency"] == {
        "group": "release-${{ github.ref }}",
        "cancel-in-progress": False,
    }


def test_workflows_pin_cache_and_artifact_actions_to_node24_releases() -> None:
    seen: set[str] = set()
    for name in ("ci.yml", "release.yml"):
        for step in steps(workflow(name)):
            uses = str(step.get("uses", ""))
            repository, separator, revision = uses.partition("@")
            if repository in NODE24_ACTIONS:
                seen.add(repository)
                assert separator == "@"
                assert revision == NODE24_ACTIONS[repository]
    assert seen == set(NODE24_ACTIONS)


def test_workflows_have_no_lfs_configuration_or_commands() -> None:
    for name in ("ci.yml", "release.yml"):
        text = (ROOT / ".github" / "workflows" / name).read_text(encoding="utf-8")
        for forbidden in ("git " + "lfs", "git-" + "lfs", "git_" + "lfs"):
            assert forbidden not in text.lower()
        for step in steps(workflow(name)):
            if str(step.get("uses", "")).startswith("actions/checkout@"):
                assert "lfs" not in step.get("with", {})


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
