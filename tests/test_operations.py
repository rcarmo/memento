from __future__ import annotations

import asyncio
import io
import json
import shutil
import sqlite3
from contextlib import redirect_stderr, redirect_stdout
from datetime import UTC, datetime
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


def test_systemd_units_use_installed_console_script() -> None:
    root = Path(__file__).parents[1]
    service_unit = root / "deploy/systemd/memento.service"
    audit_unit = root / "deploy/systemd/memento-audit.service"
    backup_unit = root / "deploy/systemd/memento-backup.service"
    for unit in (service_unit, audit_unit, backup_unit):
        content = unit.read_text(encoding="utf-8")
        assert "ExecStart=/opt/memento/.venv/bin/memento-serve " in content
        assert "NoNewPrivileges=true" in content
        assert "ProtectSystem=strict" in content
        assert "EnvironmentFile=-/etc/memento/memento.env" in content
        assert "StateDirectory=memento" in content

    assert "ReadWritePaths=/var/lib/memento" in service_unit.read_text(encoding="utf-8")
    assert "ReadWritePaths=/var/lib/memento" in audit_unit.read_text(encoding="utf-8")

    backup_content = backup_unit.read_text(encoding="utf-8")
    assert "backup --output /srv/memento-backups/latest" in backup_content
    assert "ReadWritePaths=/var/lib/memento /srv/memento-backups" in backup_content


def test_systemd_timers_are_disabled_by_default_for_exclusive_maintenance() -> None:
    root = Path(__file__).parents[1]
    for timer in (
        root / "deploy/systemd/memento-audit.timer",
        root / "deploy/systemd/memento-backup.timer",
    ):
        content = timer.read_text(encoding="utf-8")
        assert "[Timer]" in content
        assert "WantedBy=timers.target" not in content
        assert "exclusive lease" in content


def test_compose_example_uses_env_file_and_example_env_lists_required_tokens() -> None:
    root = Path(__file__).parents[1]
    compose = (root / "compose.example.yaml").read_text(encoding="utf-8")
    env_example = (root / "examples/memento.env.example").read_text(encoding="utf-8")
    assert "env_file:" in compose
    assert "- .env" in compose
    assert "MEMENTO_TOKEN_SMITH=" in env_example
    assert "MEMENTO_TOKEN_FLINT=" in env_example
    assert "required by examples/config.v1.json" in env_example


def test_runtime_loads_and_closes_needle_router(
    monkeypatch: pytest.MonkeyPatch, seeded_root: tuple[Path, Path]
) -> None:
    config_path, seed = seeded_root
    config = json.loads(config_path.read_text(encoding="utf-8"))
    config["intelligent_tiers"] = {
        "needle_router": {
            "enabled": True,
            "ffi_library_path": "/tmp/libmemento_needle_ffi.so",
            "model_path": "/tmp/memento-router.ndl",
            "tokenizer_path": "/tmp/needle.model",
        }
    }
    config_path.write_text(json.dumps(config), encoding="utf-8")

    class FakeRouter:
        def __init__(self) -> None:
            self.closed = False

        def generate(self, query: str, tools_json: str, **_: object) -> str:
            return '[{"name":"UNKNOWN","arguments":{}}]'

        def close(self) -> None:
            self.closed = True

    fake_router = FakeRouter()

    class FakeLibrary:
        def __init__(self, library_path: str) -> None:
            self.library_path = library_path

        def load_router(self, model_path: str, tokenizer_path: str) -> FakeRouter:
            assert model_path == "/tmp/memento-router.ndl"
            assert tokenizer_path == "/tmp/needle.model"
            return fake_router

    monkeypatch.setattr("memento.app.NeedleFfiLibrary", FakeLibrary)
    runtime = build_runtime(config_path, bootstrap_seed=seed)
    try:
        assert runtime.status_snapshot()["needle_router"]["loaded"] is True
    finally:
        runtime.close()
    assert fake_router.closed is True


def test_cli_status_and_rebuild_index(seeded_root: tuple[Path, Path]) -> None:
    config_path, seed = seeded_root
    build_runtime(config_path, bootstrap_seed=seed).close()

    output = io.StringIO()
    logs = io.StringIO()
    with redirect_stdout(output), redirect_stderr(logs):
        assert main(["--config", str(config_path), "status"]) == 0
    payload = json.loads(output.getvalue())
    assert payload["visible_concepts"] == 2
    assert '"event": "command_completed"' in logs.getvalue()

    output = io.StringIO()
    logs = io.StringIO()
    with redirect_stdout(output), redirect_stderr(logs):
        assert main(["--config", str(config_path), "rebuild-index"]) == 0
    rebuilt = json.loads(output.getvalue())
    assert rebuilt["parity_matches"] is True
    assert '"event": "command_completed"' in logs.getvalue()


def test_cli_prometheus_output_is_clean_stdout(seeded_root: tuple[Path, Path]) -> None:
    config_path, seed = seeded_root
    runtime = build_runtime(config_path, bootstrap_seed=seed)
    expected = render_prometheus_text(runtime)
    runtime.close()

    output = io.StringIO()
    logs = io.StringIO()
    with redirect_stdout(output), redirect_stderr(logs):
        assert main(["--config", str(config_path), "status", "--format", "prometheus"]) == 0
    text = output.getvalue()
    assert text == expected
    assert '"event": "status_rendered"' not in text
    assert logs.getvalue().count('"event": "runtime_closed"') == 1


def test_backup_restore_rejects_manifest_revision_mismatch(
    seeded_root: tuple[Path, Path], tmp_path: Path
) -> None:
    config_path, seed = seeded_root
    runtime = build_runtime(config_path, bootstrap_seed=seed)
    backup_dir = tmp_path / "backup-mismatch"
    try:
        create_backup(runtime, backup_dir)
    finally:
        runtime.close()

    manifest_path = backup_dir / "manifest.json"
    manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
    manifest["repo_revision"] = "0" * 40
    manifest_path.write_text(
        json.dumps(manifest, indent=2, sort_keys=True) + "\n", encoding="utf-8"
    )

    with pytest.raises(ValueError, match="backup manifest revision does not match archived main"):
        restore_backup(load_service_config(config_path), backup_dir)


def test_backup_rejects_destination_inside_state_root(seeded_root: tuple[Path, Path]) -> None:
    config_path, seed = seeded_root
    runtime = build_runtime(config_path, bootstrap_seed=seed)
    try:
        with pytest.raises(
            ValueError, match="backup destination must be outside repository root_path"
        ):
            create_backup(runtime, runtime.paths.root / "backups" / "latest")
    finally:
        runtime.close()


def test_restore_rejects_backup_source_inside_state_root(seeded_root: tuple[Path, Path]) -> None:
    config_path, seed = seeded_root
    runtime = build_runtime(config_path, bootstrap_seed=seed)
    try:
        nested_backup = runtime.paths.root / "restore-source"
        nested_backup.mkdir(parents=True, exist_ok=True)
        with pytest.raises(ValueError, match="backup source must be outside repository root_path"):
            restore_backup(load_service_config(config_path), nested_backup)
    finally:
        runtime.close()


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


def test_runtime_server_uses_writable_state_log(seeded_root: tuple[Path, Path]) -> None:
    config_path, seed = seeded_root
    runtime = build_runtime(config_path, bootstrap_seed=seed)
    try:
        server = runtime.build_server()
        assert server.log_file == runtime.paths.root / "logs" / "umcp.log"
        assert server.log_file.exists()
    finally:
        runtime.close()


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
            created_at=datetime(2026, 7, 17, tzinfo=UTC),
            updated_at=datetime(2026, 7, 17, tzinfo=UTC),
            updated_by="tests",
        ),
        body=body,
    )
    path.write_text(serialize_concept(document), encoding="utf-8")
