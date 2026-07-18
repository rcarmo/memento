from __future__ import annotations

import hashlib
import io
import subprocess
import zipfile
from pathlib import Path

import pytest

from memento.repository.asset_packs import (
    LFS_ATTRIBUTES_LINE,
    asset_version_paths,
    list_asset_versions,
    load_asset_metadata,
    resolve_asset_version,
    retention_partition,
    write_asset_version,
)
from memento.skill_packs import ValidatedSkillPack, validate_asset_pack


def pack(version: str) -> ValidatedSkillPack:
    stream = io.BytesIO()
    with zipfile.ZipFile(stream, "w") as archive:
        archive.writestr("template.txt", "hello")
    return validate_asset_pack(asset_kind="templates", version=version, zip_bytes=stream.getvalue())


def test_write_resolve_and_retention(tmp_path: Path) -> None:
    first, second = pack("1.9.0"), pack("1.10.0")
    for item in (first, second):
        write_asset_version(
            tmp_path,
            concept_id="12345678-abcd-1234-abcd-123456789abc",
            concept_path="/projects/demo.md",
            asset_kind="templates",
            version=item.version,
            zip_bytes=item.zip_bytes,
            manifest=item.manifest,
            accepted_by="curator",
            source_proposal_id=f"p-{item.version}",
        )
    assert (
        resolve_asset_version(tmp_path, "12345678-abcd-1234-abcd-123456789abc", "templates", None)
        == "1.10.0"
    )
    assert list_asset_versions(tmp_path, "12345678-abcd-1234-abcd-123456789abc", "templates") == (
        "1.9.0",
        "1.10.0",
    )
    metadata = load_asset_metadata(
        tmp_path, "12345678-abcd-1234-abcd-123456789abc", "templates", "1.10.0"
    )
    assert metadata["concept_path"] == "/projects/demo.md"
    assert LFS_ATTRIBUTES_LINE in (tmp_path / ".gitattributes").read_text()
    assert retention_partition(("1.0.0", "1.1.0", "1.2.0", "1.3.0", "1.4.0", "1.5.0")) == (
        ("1.5.0", "1.4.0", "1.3.0", "1.2.0", "1.1.0"),
        ("1.0.0",),
    )


def test_immutable_asset_version(tmp_path: Path) -> None:
    item = pack("1.0.0")

    def write() -> tuple[str, ...]:
        return write_asset_version(
            tmp_path,
            concept_id="12345678-abcd-1234-abcd-123456789abc",
            concept_path="/projects/demo.md",
            asset_kind="templates",
            version=item.version,
            zip_bytes=item.zip_bytes,
            manifest=item.manifest,
            accepted_by="curator",
            source_proposal_id="p1",
        )

    write()
    with pytest.raises(ValueError, match="already exists"):
        write()


def test_git_lfs_tracks_generic_asset(tmp_path: Path) -> None:
    root = tmp_path / "repo"
    root.mkdir()
    subprocess.run(["git", "init", "-b", "main", str(root)], check=True, capture_output=True)
    item = pack("1.0.0")
    write_asset_version(
        root,
        concept_id="12345678-abcd-1234-abcd-123456789abc",
        concept_path="/projects/demo.md",
        asset_kind="templates",
        version="1.0.0",
        zip_bytes=item.zip_bytes,
        manifest=item.manifest,
        accepted_by="curator",
        source_proposal_id="p1",
    )
    subprocess.run(["git", "-C", str(root), "lfs", "install", "--local"], check=True)
    subprocess.run(["git", "-C", str(root), "add", "--all"], check=True)
    _metadata, zip_path = asset_version_paths(
        "12345678-abcd-1234-abcd-123456789abc", "templates", "1.0.0"
    )
    staged = subprocess.run(
        ["git", "-C", str(root), "show", f":{zip_path.removeprefix('/')}"],
        check=True,
        capture_output=True,
    ).stdout
    assert staged.startswith(b"version https://git-lfs.github.com/spec/v1\n")
    assert (
        hashlib.sha256((root / zip_path.removeprefix("/")).read_bytes()).hexdigest()
        == item.manifest.sha256
    )
