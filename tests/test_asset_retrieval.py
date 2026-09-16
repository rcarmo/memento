from __future__ import annotations

import hashlib
import io
import stat
import zipfile

import pytest

from memento.asset_retrieval import AssetReadError, checked_manifest, read_pack_file, verified_slice
from memento.skill_packs import SkillPackManifest, SkillPackManifestEntry


def manifest(raw: bytes, content: bytes = b"declared") -> SkillPackManifest:
    return SkillPackManifest(
        entries=(
            SkillPackManifestEntry(
                path="file.txt",
                size=len(content),
                media_type="text/plain",
                sha256=hashlib.sha256(content).hexdigest(),
            ),
        ),
        sha256=hashlib.sha256(raw).hexdigest(),
        total_uncompressed_bytes=len(content),
        file_count=1,
    )


@pytest.mark.parametrize(
    "attack", ["symlink", "undeclared", "missing", "duplicate", "traversal", "digest"]
)
def test_file_reads_fail_closed_on_zip_manifest_disagreement(attack: str) -> None:
    stream = io.BytesIO()
    with zipfile.ZipFile(stream, "w") as archive:
        if attack == "symlink":
            info = zipfile.ZipInfo("file.txt")
            info.create_system = 3
            info.external_attr = (stat.S_IFLNK | 0o777) << 16
            archive.writestr(info, b"declared")
        elif attack == "digest":
            archive.writestr("file.txt", b"replaced")
        elif attack != "missing":
            archive.writestr("file.txt", b"declared")
        if attack == "duplicate":
            with pytest.warns(UserWarning):
                archive.writestr("file.txt", b"declared")
        elif attack in {"undeclared", "traversal"}:
            archive.writestr("extra.txt" if attack == "undeclared" else "../evil.txt", b"x")
    raw = stream.getvalue()
    data = manifest(raw)
    with pytest.raises(AssetReadError):
        read_pack_file(io.BytesIO(raw), data, file_path="file.txt", offset=0, limit=4)


def test_file_digest_checks_bytes_outside_requested_slice() -> None:
    with pytest.raises(AssetReadError, match="digest"):
        verified_slice(
            io.BytesIO(b"prefixBAD"),
            total=9,
            digest=hashlib.sha256(b"prefixOK!").hexdigest(),
            offset=0,
            limit=6,
        )


@pytest.mark.parametrize("offset,limit", [(-1, 1), (10, 1), (0, 0), (0, 262145), (True, 1)])
def test_invalid_ranges(offset: int, limit: int) -> None:
    with pytest.raises(AssetReadError):
        verified_slice(
            io.BytesIO(b"abc"),
            total=3,
            digest=hashlib.sha256(b"abc").hexdigest(),
            offset=offset,
            limit=limit,
        )


def test_manifest_rejects_duplicate_and_noncanonical_entries() -> None:
    data = manifest(b"zip").model_dump(mode="json")
    data["entries"] *= 2
    data["file_count"] = 2
    data["total_uncompressed_bytes"] *= 2
    with pytest.raises(AssetReadError):
        checked_manifest(data, data["sha256"])
    data["entries"][0]["path"] = "./file.txt"
    with pytest.raises(AssetReadError):
        checked_manifest(data, data["sha256"])
