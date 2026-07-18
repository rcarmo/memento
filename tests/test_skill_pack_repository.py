from __future__ import annotations

import io
import json
import subprocess
import zipfile
from pathlib import Path

import pytest

from memento.skill_packs import ValidatedSkillPack, parse_stable_semver, validate_skill_pack
from memento.repository.skill_packs import (
    LFS_ATTRIBUTES_LINE,
    list_skill_pack_versions,
    parse_skill_pack_document,
    resolve_skill_pack_version,
    retention_partition,
    skill_pack_paths,
    write_skill_pack_version,
)


def make_pack(name: str, version: str, skill_md: str) -> ValidatedSkillPack:
    stream = io.BytesIO()
    with zipfile.ZipFile(stream, "w", compression=zipfile.ZIP_DEFLATED) as archive:
        archive.writestr("SKILL.md", skill_md)
        archive.writestr("scripts/run.ts", "console.log('ok')\n")
    return validate_skill_pack(
        skill_name=name, version=version, skill_md=skill_md, zip_bytes=stream.getvalue()
    )


def test_semver_ordering_is_numeric_and_stable_only() -> None:
    assert sorted(("1.10.0", "2.0.0", "1.9.9"), key=parse_stable_semver) == [
        "1.9.9",
        "1.10.0",
        "2.0.0",
    ]


def test_write_resolve_and_retention(tmp_path: Path) -> None:
    first = make_pack("web-search", "1.9.0", "# Web Search\n")
    second = make_pack("web-search", "1.10.0", "# Web Search 1.10\n")
    write_skill_pack_version(tmp_path, first, accepted_by="curator", source_proposal_id="p1")
    changed = write_skill_pack_version(
        tmp_path, second, accepted_by="curator", source_proposal_id="p2"
    )
    assert "/skills/.versions/web-search/1.10.0.zip" in changed
    assert resolve_skill_pack_version(tmp_path, "web-search") == "1.10.0"
    assert list_skill_pack_versions(tmp_path, "web-search") == ("1.9.0", "1.10.0")
    latest = (tmp_path / "skills/web-search.md").read_text()
    metadata, body = parse_skill_pack_document(latest)
    assert metadata["version"] == "1.10.0"
    assert body == "# Web Search 1.10\n"
    assert LFS_ATTRIBUTES_LINE in (tmp_path / ".gitattributes").read_text()
    assert retention_partition(("1.0.0", "1.1.0", "1.2.0", "1.3.0", "1.4.0", "1.5.0")) == (
        ("1.5.0", "1.4.0", "1.3.0", "1.2.0", "1.1.0"),
        ("1.0.0",),
    )


def test_immutable_version_cannot_be_replaced(tmp_path: Path) -> None:
    pack = make_pack("demo", "1.0.0", "# Demo\n")
    write_skill_pack_version(tmp_path, pack, accepted_by="curator", source_proposal_id="p1")
    with pytest.raises(ValueError, match="already exists"):
        write_skill_pack_version(tmp_path, pack, accepted_by="curator", source_proposal_id="p2")


def test_git_lfs_tracks_zip_and_checkout_restores_bytes(tmp_path: Path) -> None:
    root = tmp_path / "repo"
    root.mkdir()
    subprocess.run(["git", "init", "-b", "main", str(root)], check=True, capture_output=True)
    pack = make_pack("demo", "1.0.0", "# Demo\n")
    write_skill_pack_version(root, pack, accepted_by="curator", source_proposal_id="p1")
    subprocess.run(["git", "-C", str(root), "lfs", "install", "--local"], check=True)
    subprocess.run(["git", "-C", str(root), "add", "--all"], check=True)
    staged = subprocess.run(
        ["git", "-C", str(root), "show", ":skills/.versions/demo/1.0.0.zip"],
        check=True,
        capture_output=True,
    ).stdout
    assert staged.startswith(b"version https://git-lfs.github.com/spec/v1\n")
    metadata, _body = parse_skill_pack_document((root / "skills/demo.md").read_text())
    assert isinstance(metadata["manifest"], dict)
    assert metadata["manifest"]["sha256"] == pack.manifest.sha256
    assert (root / "skills/.versions/demo/1.0.0.zip").read_bytes() == pack.zip_bytes


def test_paths_are_deterministic() -> None:
    paths = skill_pack_paths("go-pprof", "2.3.4")
    assert paths.latest_document == "/skills/go-pprof.md"
    assert paths.version_document == "/skills/.versions/go-pprof/2.3.4.md"
