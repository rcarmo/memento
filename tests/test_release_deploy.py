from __future__ import annotations

import argparse
import importlib.util
import sys
from pathlib import Path
from typing import Any, cast

import pytest
import yaml  # type: ignore[import-untyped]

ROOT = Path(__file__).parents[1]
TOOL_PATH = Path(__file__).parents[1] / "tools" / "release_deploy.py"
SPEC = importlib.util.spec_from_file_location("release_deploy", TOOL_PATH)
assert SPEC is not None and SPEC.loader is not None
release_deploy = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = release_deploy
SPEC.loader.exec_module(release_deploy)


def deploy_args(version: str = "0.3.23") -> argparse.Namespace:
    return argparse.Namespace(
        version=version,
        pull_timeout=300.0,
        deploy_timeout=300.0,
    )


def test_compose_uses_bounded_persisted_state_startup_grace() -> None:
    document = yaml.safe_load(release_deploy.compose("0.3.23"))
    service = document["services"]["memento"]
    assert service["image"] == "ghcr.io/rcarmo/memento:0.3.23"
    expected_healthcheck = {
        "test": [
            "CMD",
            "python",
            "-c",
            "import socket; socket.create_connection(('127.0.0.1', 8000), 2).close()",
        ],
        "interval": "30s",
        "timeout": "5s",
        "start_period": "5m",
        "retries": 3,
    }
    assert service["healthcheck"] == expected_healthcheck
    static = yaml.safe_load((ROOT / "deploy/diskstation.compose.yaml").read_text(encoding="utf-8"))
    assert static["services"]["memento"]["healthcheck"] == expected_healthcheck
    dockerfile = (ROOT / "Dockerfile").read_text(encoding="utf-8")
    assert "--start-period=5m" in dockerfile


def test_update_config_waits_for_success_and_removes_unique_helper(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    calls: list[tuple[str, str, object | None]] = []
    states = iter(("running", "exited"))

    def request(
        method: str, path: str, *, data: object | None = None, timeout: float = 60.0
    ) -> Any:
        calls.append((method, path, data))
        if method == "POST" and "/containers/create?" in path:
            return {"Id": "helper-container-id"}
        if method == "GET":
            status = next(states)
            return {"State": {"Status": status, "ExitCode": 0 if status == "exited" else None}}
        return None

    monkeypatch.setattr(release_deploy, "portainer_request", request)
    monkeypatch.setattr(
        release_deploy, "uuid4", lambda: type("Uuid", (), {"hex": "abc123def456"})()
    )
    monkeypatch.setattr(release_deploy.time, "sleep", lambda _seconds: None)

    release_deploy.update_config(deploy_args())

    create = calls[0]
    assert create[0] == "POST"
    assert create[1].endswith("name=memento-config-update-abc123def456")
    payload = cast(dict[str, Any], create[2])
    assert payload["User"] == "65532:65532"
    assert payload["NetworkDisabled"] is True
    script = payload["Cmd"][1]
    assert "temporary.open('w')" in script
    assert "os.fsync(stream.fileno())" in script
    assert "os.replace(temporary, path)" in script
    assert "os.fsync(directory)" in script
    assert payload["HostConfig"] == {
        "Binds": ["/volume1/docker/memento/config:/config"],
        "AutoRemove": False,
        "ReadonlyRootfs": True,
        "CapDrop": ["ALL"],
        "SecurityOpt": ["no-new-privileges:true"],
    }
    assert [method for method, _path, _data in calls] == ["POST", "POST", "GET", "GET", "DELETE"]
    assert calls[-1][1].endswith("/helper-container-id?force=true&v=true")


def test_update_config_reports_failure_and_still_removes_helper(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    calls: list[tuple[str, str]] = []

    def request(
        method: str, path: str, *, data: object | None = None, timeout: float = 60.0
    ) -> Any:
        calls.append((method, path))
        if method == "POST" and "/containers/create?" in path:
            return {"Id": "failed-helper-id"}
        if method == "GET":
            return {"State": {"Status": "exited", "ExitCode": 17}}
        return None

    monkeypatch.setattr(release_deploy, "portainer_request", request)
    monkeypatch.setattr(
        release_deploy, "uuid4", lambda: type("Uuid", (), {"hex": "failedhelper"})()
    )

    with pytest.raises(SystemExit, match="failed with exit code 17"):
        release_deploy.update_config(deploy_args())

    assert calls[-1] == (
        "DELETE",
        "/api/endpoints/18/docker/containers/failed-helper-id?force=true&v=true",
    )


def test_deploy_propagates_stack_update_failure(monkeypatch: pytest.MonkeyPatch) -> None:
    def request(
        method: str, path: str, *, data: object | None = None, timeout: float = 60.0
    ) -> Any:
        if method == "PUT" and path.startswith("/api/stacks/111?"):
            raise SystemExit("stack update failed")
        if path.endswith("/file"):
            return {
                "StackFileContent": "services:\n  memento:\n    image: ghcr.io/rcarmo/memento:0.4.2\n"
            }
        if "/images/" in path and method == "GET":
            return {"RepoDigests": ["ghcr.io/rcarmo/memento@sha256:" + "a" * 64]}
        return {"Env": []}

    monkeypatch.setattr(release_deploy, "portainer_request", request)
    monkeypatch.setattr(
        release_deploy,
        "release_image",
        lambda _version: "ghcr.io/rcarmo/memento@sha256:" + "a" * 64,
    )

    with pytest.raises(SystemExit, match="stack update failed"):
        release_deploy.deploy(deploy_args())


def test_stack_image_replacement_preserves_all_other_configuration() -> None:
    stack = "services:\n  memento:\n    image: ghcr.io/rcarmo/memento:0.4.2\n    volumes:\n      - models:/models\n    environment:\n      CUSTOM: retained\n"
    image = "ghcr.io/rcarmo/memento@sha256:" + "b" * 64
    assert release_deploy.replace_stack_image(stack, image) == stack.replace(
        "ghcr.io/rcarmo/memento:0.4.2", image
    )
    with pytest.raises(SystemExit):
        release_deploy.replace_stack_image("services: {}", image)
