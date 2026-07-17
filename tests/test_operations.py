from __future__ import annotations

import asyncio
import io
import json
import shutil
import sqlite3
from contextlib import redirect_stdout
from datetime import datetime, timezone
from pathlib import Path

import pytest

from memento.app import RuntimeClosedError, build_runtime, load_service_config
from memento.backup import create_backup, restore_backup
from memento.cli import _serve, main
from memento.config import ServiceConfig
from memento.logging import JsonLogger
from memento.metrics import render_prometheus_text
from memento.repository.frontmatter import serialize_concept
from memento.repository.git import get_main_revision
from memento.repository.schema import ConceptDocument, ConceptFrontmatter, ConceptStatus


@pytest.fixture()
def seeded_root(tmp_path: Path) -> tuple[Path, Path]:
    root = tmp_path / "state"
    seed = tmp_path / "seed"
    write_concept(seed / "instances" / "smith.md", title="Smith", body="# Smith\n")
    write_concept(seed / "projects" / "piclaw.md", title="Piclaw", body="# Piclaw\n")
    config_path = tmp_path / "config.json"
    config_path.write_text(
        json.dumps(
            {
                "schema_version": 2,
                "repository": {"root_path": str(root), "bundle_root": "/"},
                "authorization": {
                    "principals": {
                        "smith": {
                            "roles": ["reader", "proposer", "curator"],
                            "token_env": "MEMENTO_TOKEN_SMITH",
                            "read_prefixes": ["/instances/", "/projects/"],
                            "write_prefixes": ["/instances/", "/projects/"],
                        }
                    }
                },
            }
        ),
        encoding="utf-8",
    )
    return config_path, seed


def test_config_loading_and_composition_root(seeded_root: tuple[Path, Path]) -> None:
    config_path, seed = seeded_root
    config = load_service_config(config_path)
    assert isinstance(config, ServiceConfig)
    runtime = build_runtime(config_path, bootstrap_seed=seed)
    try:
        status = runtime.status_snapshot()
        assert status["visible_concepts"] == 2
        assert status["repo_revision"] == get_main_revision(runtime.paths.repo_paths)
    finally:
        runtime.close()


def test_cli_status_and_rebuild_index(seeded_root: tuple[Path, Path]) -> None:
    config_path, seed = seeded_root
    build_runtime(config_path, bootstrap_seed=seed).close()

    output = io.StringIO()
    with redirect_stdout(output):
        assert main(["--config", str(config_path), "status"]) == 0
    payload = json.loads(output.getvalue())
    assert payload["visible_concepts"] == 2

    output = io.StringIO()
    with redirect_stdout(output):
        assert main(["--config", str(config_path), "rebuild-index"]) == 0
    rebuilt = json.loads(output.getvalue())
    assert rebuilt["parity_matches"] is True


def test_cli_prometheus_output(seeded_root: tuple[Path, Path]) -> None:
    config_path, seed = seeded_root
    build_runtime(config_path, bootstrap_seed=seed).close()
    output = io.StringIO()
    with redirect_stdout(output):
        assert main(["--config", str(config_path), "status", "--format", "prometheus"]) == 0
    text = output.getvalue()
    assert "memento_service_up 1" in text
    assert "memento_visible_concepts 2" in text


def test_backup_restore_and_audit(seeded_root: tuple[Path, Path], tmp_path: Path) -> None:
    config_path, seed = seeded_root
    runtime = build_runtime(config_path, bootstrap_seed=seed)
    backup_dir = tmp_path / "backup"
    try:
        manifest = create_backup(runtime, backup_dir)
        assert manifest.repo_revision == get_main_revision(runtime.paths.repo_paths)
    finally:
        runtime.close()

    root = load_service_config(config_path).repository.root_path
    shutil_root = Path(root)
    for child in shutil_root.iterdir():
        if child.is_dir():
            shutil.rmtree(child)
        else:
            child.unlink()

    restored = restore_backup(load_service_config(config_path), backup_dir)
    assert restored["rebuild_derived"] is True

    output = io.StringIO()
    with redirect_stdout(output):
        assert main(["--config", str(config_path), "audit"]) == 0
    payload = json.loads(output.getvalue())
    assert payload["ok"] is True


def test_runtime_close_closes_sqlite_connection(seeded_root: tuple[Path, Path]) -> None:
    config_path, seed = seeded_root
    runtime = build_runtime(config_path, bootstrap_seed=seed)
    runtime.close()
    with pytest.raises(RuntimeClosedError):
        runtime.status_snapshot()
    with pytest.raises(sqlite3.ProgrammingError):
        runtime.control_connection.execute("SELECT 1")


def test_structured_logging_redacts_secrets() -> None:
    stream = io.StringIO()
    logger = JsonLogger(stream=stream)
    logger.info("test", authorization="Bearer secret", nested={"token": "x", "ok": 1})
    payload = json.loads(stream.getvalue())
    assert payload["authorization"] == "<redacted>"
    assert payload["nested"]["token"] == "<redacted>"
    assert payload["nested"]["ok"] == 1


def test_graceful_server_drain(
    monkeypatch: pytest.MonkeyPatch,
    seeded_root: tuple[Path, Path],
) -> None:
    config_path, seed = seeded_root
    runtime = build_runtime(config_path, bootstrap_seed=seed)
    logger = JsonLogger(stream=io.StringIO())

    class FakeServer:
        def __init__(self) -> None:
            self.closed = False

        async def close(self) -> None:
            self.closed = True

    server = FakeServer()
    monkeypatch.setattr(type(runtime), "build_server", lambda self: server)

    async def fake_run_server(*args: object, **kwargs: object) -> int:
        await asyncio.sleep(60)
        return 0

    def fake_install_signal_handlers(shutdown_event: asyncio.Event, logger: JsonLogger) -> None:
        asyncio.get_running_loop().call_soon(shutdown_event.set)

    import memento.cli as cli_module

    monkeypatch.setattr(cli_module, "run_server", fake_run_server)
    monkeypatch.setattr(cli_module, "install_signal_handlers", fake_install_signal_handlers)
    try:
        assert (
            asyncio.run(
                _serve(runtime, host="127.0.0.1", port=8000, endpoint="/mcp", logger=logger)
            )
            == 0
        )
        assert server.closed is True
    finally:
        runtime.close()


def test_metrics_renderer(seeded_root: tuple[Path, Path]) -> None:
    config_path, seed = seeded_root
    runtime = build_runtime(config_path, bootstrap_seed=seed)
    try:
        text = render_prometheus_text(runtime)
        assert "memento_repo_revision_info" in text
    finally:
        runtime.close()


def write_concept(path: Path, *, title: str, body: str) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    document = ConceptDocument(
        frontmatter=ConceptFrontmatter(
            schema_version=1,
            id=title.lower() + "-id",
            type="instance",
            title=title,
            description=title,
            tags=(),
            aliases=(),
            source_refs=(),
            supersedes=(),
            status=ConceptStatus.ACTIVE,
            created_at=datetime(2026, 7, 17, tzinfo=timezone.utc),
            updated_at=datetime(2026, 7, 17, tzinfo=timezone.utc),
            updated_by="tests",
        ),
        body=body,
    )
    path.write_text(serialize_concept(document), encoding="utf-8")
