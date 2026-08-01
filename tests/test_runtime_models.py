from __future__ import annotations

import hashlib
import importlib.util
import io
import json
import tarfile
from pathlib import Path
from typing import Any

import pytest

ROOT = Path(__file__).parents[1]
TOOL_PATH = ROOT / "tools" / "prepare_runtime_models.py"


def load_tool() -> Any:
    spec = importlib.util.spec_from_file_location("prepare_runtime_models", TOOL_PATH)
    assert spec is not None and spec.loader is not None
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def archive(files: dict[str, bytes]) -> bytes:
    output = io.BytesIO()
    with tarfile.open(fileobj=output, mode="w") as bundle:
        for name, contents in files.items():
            member = tarfile.TarInfo(name)
            member.size = len(contents)
            bundle.addfile(member, io.BytesIO(contents))
    return output.getvalue()


def manifest(files: dict[str, bytes], bundle: bytes) -> dict[str, object]:
    return {
        "schema_version": 1,
        "release_tag": "model-assets-v1",
        "asset_name": "runtime-models-test.tar",
        "asset_sha256": hashlib.sha256(bundle).hexdigest(),
        "files": {name: hashlib.sha256(contents).hexdigest() for name, contents in files.items()},
    }


def test_manifest_is_the_only_tracked_runtime_model_source() -> None:
    payload = json.loads((ROOT / "models/runtime-models.json").read_text(encoding="utf-8"))
    assert payload["release_tag"] == "model-assets-v1"
    assert payload["asset_name"].startswith("runtime-models-")
    assert payload["asset_name"].endswith(".tar")
    assert len(payload["asset_sha256"]) == 64
    assert set(payload["files"]) == {
        "models/gte/gte-small.gtemodel",
        "models/needle/memento-router.ndl",
        "models/needle/needle.model",
    }
    for name in payload["files"]:
        assert isinstance(name, str)
        assert not (ROOT / name).is_symlink()


def test_download_extracts_and_verifies_exact_archive(
    monkeypatch: pytest.MonkeyPatch, tmp_path: Path
) -> None:
    tool = load_tool()
    files = {
        "models/gte/gte-small.gtemodel": b"gte",
        "models/needle/memento-router.ndl": b"ndl",
        "models/needle/needle.model": b"tokenizer",
    }
    bundle = archive(files)
    payload = manifest(files, bundle)
    monkeypatch.setattr(tool.urllib.request, "urlopen", lambda request, timeout: io.BytesIO(bundle))

    tool.download(tmp_path, None, payload)
    tool.verify(tmp_path, payload)

    for name, contents in files.items():
        assert (tmp_path / name).read_bytes() == contents
    asset_name = payload["asset_name"]
    assert isinstance(asset_name, str)
    assert not (tmp_path / asset_name).exists()


def test_download_rejects_unexpected_archive_members(
    monkeypatch: pytest.MonkeyPatch, tmp_path: Path
) -> None:
    tool = load_tool()
    files = {"models/needle/needle.model": b"tokenizer"}
    bundle = archive({**files, "unexpected": b"no"})
    payload = manifest(files, bundle)
    monkeypatch.setattr(tool.urllib.request, "urlopen", lambda request, timeout: io.BytesIO(bundle))

    with pytest.raises(SystemExit, match="unexpected runtime model archive members"):
        tool.download(tmp_path, None, payload)
