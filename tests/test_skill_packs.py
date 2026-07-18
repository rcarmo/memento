from __future__ import annotations

import hashlib
import io
import zipfile
from collections.abc import Iterable

import pytest

from memento.skill_packs import (
    MAX_ARCHIVE_ENTRY_COUNT,
    MAX_COMPRESSION_RATIO,
    MAX_FILE_BYTES,
    MAX_FILE_COUNT,
    MAX_UNCOMPRESSED_BYTES,
    SKILL_MD_PATH,
    SkillPackValidationError,
    validate_skill_pack,
)


def _make_zip(
    entries: Iterable[tuple[str, bytes | None]], *, compression: int = zipfile.ZIP_DEFLATED
) -> bytes:
    buffer = io.BytesIO()
    with zipfile.ZipFile(buffer, mode="w", compression=compression) as archive:
        for path, data in entries:
            info = zipfile.ZipInfo(path)
            if path.endswith("/"):
                info.external_attr = 0o040755 << 16
                archive.writestr(info, b"")
            else:
                info.compress_type = compression
                info.external_attr = 0o100644 << 16
                archive.writestr(info, b"" if data is None else data)
    return buffer.getvalue()


def _pack(skill_md: str, *extra_entries: tuple[str, bytes | None]) -> bytes:
    return _make_zip([(SKILL_MD_PATH, skill_md.encode("utf-8")), *extra_entries])


def test_validate_skill_pack_accepts_valid_archive_and_builds_manifest() -> None:
    skill_md = "# Search\n\nThis is searchable.\n"
    image_bytes = b"\x89PNG\r\n\x1a\n" + b"png-data"
    pdf_bytes = b"%PDF-1.7\n%\xe2\xe3\xcf\xd3\n"
    zip_bytes = _pack(
        skill_md,
        ("docs/", None),
        ("docs/readme.txt", b"hello world\n"),
        ("scripts/run.sh", b"#!/bin/sh\necho ok\n"),
        ("images/icon.png", image_bytes),
        ("docs/spec.pdf", pdf_bytes),
    )

    validated = validate_skill_pack(
        skill_name="search-pack",
        version="1.2.3",
        skill_md=skill_md,
        zip_bytes=zip_bytes,
    )

    assert validated.zip_bytes == zip_bytes
    assert validated.manifest.sha256 == hashlib.sha256(zip_bytes).hexdigest()
    assert validated.manifest.file_count == 5
    assert validated.manifest.total_uncompressed_bytes == sum(
        len(payload)
        for payload in (
            skill_md.encode("utf-8"),
            b"hello world\n",
            b"#!/bin/sh\necho ok\n",
            image_bytes,
            pdf_bytes,
        )
    )
    assert [entry.path for entry in validated.manifest.entries] == [
        SKILL_MD_PATH,
        "docs/readme.txt",
        "scripts/run.sh",
        "images/icon.png",
        "docs/spec.pdf",
    ]
    assert validated.manifest.entries[0].media_type == "text/markdown"
    assert validated.manifest.entries[1].media_type == "text/plain"
    assert validated.manifest.entries[2].media_type in {"text/x-sh", "application/x-sh"}
    assert validated.manifest.entries[3].media_type == "image/png"
    assert validated.manifest.entries[4].media_type == "application/pdf"


def test_validate_skill_pack_rejects_invalid_skill_name() -> None:
    with pytest.raises(SkillPackValidationError, match="skill_name"):
        validate_skill_pack(
            skill_name="Bad_Name",
            version="1.2.3",
            skill_md="# ok\n",
            zip_bytes=_pack("# ok\n"),
        )


@pytest.mark.parametrize("version", ["1", "1.2", "v1.2.3", "1.2.3-rc.1", "1.2.3+build.1", "01.2.3"])
def test_validate_skill_pack_rejects_non_stable_semver(version: str) -> None:
    with pytest.raises(SkillPackValidationError, match="semantic version"):
        validate_skill_pack(
            skill_name="good-name",
            version=version,
            skill_md="# ok\n",
            zip_bytes=_pack("# ok\n"),
        )


def test_validate_skill_pack_requires_root_skill_md() -> None:
    zip_bytes = _make_zip([("nested/SKILL.md", b"# nested\n")])
    with pytest.raises(SkillPackValidationError, match="root SKILL.md"):
        validate_skill_pack(
            skill_name="good-name",
            version="1.2.3",
            skill_md="# nested\n",
            zip_bytes=zip_bytes,
        )


def test_validate_skill_pack_requires_exact_skill_md_bytes() -> None:
    zip_bytes = _pack("# not-the-same\n")
    with pytest.raises(SkillPackValidationError, match="exactly match"):
        validate_skill_pack(
            skill_name="good-name",
            version="1.2.3",
            skill_md="# expected\n",
            zip_bytes=zip_bytes,
        )


@pytest.mark.parametrize(
    ("path", "message"),
    [
        ("/abs.txt", "absolute paths"),
        ("dir\\evil.txt", "backslashes"),
        ("../evil.txt", "path traversal"),
        ("ok/../evil.txt", "path traversal"),
        ("bad\x1fname.txt", "control characters"),
    ],
)
def test_validate_skill_pack_rejects_unsafe_paths(path: str, message: str) -> None:
    zip_bytes = _make_zip([(SKILL_MD_PATH, b"# ok\n"), (path, b"x")])
    with pytest.raises(SkillPackValidationError, match=message):
        validate_skill_pack(
            skill_name="good-name",
            version="1.2.3",
            skill_md="# ok\n",
            zip_bytes=zip_bytes,
        )


def test_validate_skill_pack_rejects_duplicate_paths() -> None:
    buffer = io.BytesIO()
    with zipfile.ZipFile(buffer, mode="w", compression=zipfile.ZIP_STORED) as archive:
        archive.writestr(SKILL_MD_PATH, b"# ok\n")
        archive.writestr("dup.txt", b"a")
        archive.writestr("dup.txt", b"b")
    with pytest.raises(SkillPackValidationError, match="duplicate path"):
        validate_skill_pack(
            skill_name="good-name",
            version="1.2.3",
            skill_md="# ok\n",
            zip_bytes=buffer.getvalue(),
        )


def test_validate_skill_pack_rejects_too_many_archive_entries() -> None:
    entries: list[tuple[str, bytes | None]] = [(SKILL_MD_PATH, b"# ok\n")]
    entries.extend((f"dirs/{index}/", None) for index in range(MAX_ARCHIVE_ENTRY_COUNT))
    zip_bytes = _make_zip(entries)
    with pytest.raises(SkillPackValidationError, match="entry count"):
        validate_skill_pack(
            skill_name="good-name",
            version="1.2.3",
            skill_md="# ok\n",
            zip_bytes=zip_bytes,
        )


def test_validate_skill_pack_rejects_too_many_files() -> None:
    entries = [(SKILL_MD_PATH, b"# ok\n")]
    entries.extend((f"docs/{index}.txt", b"x") for index in range(MAX_FILE_COUNT))
    zip_bytes = _make_zip(entries)
    with pytest.raises(SkillPackValidationError, match="file count"):
        validate_skill_pack(
            skill_name="good-name",
            version="1.2.3",
            skill_md="# ok\n",
            zip_bytes=zip_bytes,
        )


def test_validate_skill_pack_rejects_file_larger_than_limit() -> None:
    zip_bytes = _pack("# ok\n", ("large.bin", b"x" * (MAX_FILE_BYTES + 1)))
    with pytest.raises(SkillPackValidationError, match="maximum uncompressed size"):
        validate_skill_pack(
            skill_name="good-name",
            version="1.2.3",
            skill_md="# ok\n",
            zip_bytes=zip_bytes,
        )


def test_validate_skill_pack_rejects_total_uncompressed_larger_than_limit() -> None:
    chunk = b"x" * (MAX_UNCOMPRESSED_BYTES // 2)
    zip_bytes = _pack("# ok\n", ("a.bin", chunk), ("b.bin", chunk), ("c.bin", b"y"))
    with pytest.raises(SkillPackValidationError, match="maximum uncompressed size"):
        validate_skill_pack(
            skill_name="good-name",
            version="1.2.3",
            skill_md="# ok\n",
            zip_bytes=zip_bytes,
        )


def test_validate_skill_pack_rejects_excessive_compression_ratio() -> None:
    payload = b"a" * ((MAX_COMPRESSION_RATIO + 1) * 1024)
    zip_bytes = _make_zip([(SKILL_MD_PATH, b"# ok\n"), ("bomb.txt", payload)])
    with pytest.raises(SkillPackValidationError, match="compression ratio"):
        validate_skill_pack(
            skill_name="good-name",
            version="1.2.3",
            skill_md="# ok\n",
            zip_bytes=zip_bytes,
        )


def test_validate_skill_pack_rejects_nested_archives() -> None:
    zip_bytes = _pack("# ok\n", ("payload.tar.gz", b"not really a tarball"))
    with pytest.raises(SkillPackValidationError, match="nested archive"):
        validate_skill_pack(
            skill_name="good-name",
            version="1.2.3",
            skill_md="# ok\n",
            zip_bytes=zip_bytes,
        )


def test_validate_skill_pack_rejects_symlinks() -> None:
    buffer = io.BytesIO()
    with zipfile.ZipFile(buffer, mode="w", compression=zipfile.ZIP_STORED) as archive:
        archive.writestr(SKILL_MD_PATH, b"# ok\n")
        info = zipfile.ZipInfo("link")
        info.external_attr = 0o120777 << 16
        archive.writestr(info, b"target")
    with pytest.raises(SkillPackValidationError, match="symlinks"):
        validate_skill_pack(
            skill_name="good-name",
            version="1.2.3",
            skill_md="# ok\n",
            zip_bytes=buffer.getvalue(),
        )


def test_validate_skill_pack_rejects_special_file_modes() -> None:
    buffer = io.BytesIO()
    with zipfile.ZipFile(buffer, mode="w", compression=zipfile.ZIP_STORED) as archive:
        archive.writestr(SKILL_MD_PATH, b"# ok\n")
        info = zipfile.ZipInfo("pipe")
        info.external_attr = 0o010644 << 16
        archive.writestr(info, b"ignored")
    with pytest.raises(SkillPackValidationError, match="special file types"):
        validate_skill_pack(
            skill_name="good-name",
            version="1.2.3",
            skill_md="# ok\n",
            zip_bytes=buffer.getvalue(),
        )


def test_validate_skill_pack_rejects_encrypted_entries() -> None:
    zip_bytes = bytearray(_pack("# ok\n", ("secret.txt", b"secret")))
    local_flag_offset = zip_bytes.find(b"PK\x03\x04") + 6
    central_flag_offset = zip_bytes.find(b"PK\x01\x02") + 8
    assert local_flag_offset >= 6
    assert central_flag_offset >= 8
    zip_bytes[local_flag_offset : local_flag_offset + 2] = b"\x01\x00"
    zip_bytes[central_flag_offset : central_flag_offset + 2] = b"\x01\x00"

    with pytest.raises(SkillPackValidationError, match="encrypted"):
        validate_skill_pack(
            skill_name="good-name",
            version="1.2.3",
            skill_md="# ok\n",
            zip_bytes=bytes(zip_bytes),
        )


@pytest.mark.parametrize(
    ("path", "payload", "message"),
    [
        ("bin/app", b"\x7fELFrest", "ELF"),
        ("bin/app.exe", b"MZrest", "PE"),
        ("bin/app.macho", b"\xfe\xed\xfa\xcfrest", "Mach-O"),
        ("bin/fat", b"\xca\xfe\xba\xbe\x00\x00\x00\x02rest", "Mach-O"),
    ],
)
def test_validate_skill_pack_rejects_native_binaries(
    path: str, payload: bytes, message: str
) -> None:
    zip_bytes = _pack("# ok\n", (path, payload))
    with pytest.raises(SkillPackValidationError, match=message):
        validate_skill_pack(
            skill_name="good-name",
            version="1.2.3",
            skill_md="# ok\n",
            zip_bytes=zip_bytes,
        )


def test_validate_skill_pack_accepts_non_native_binary_payloads() -> None:
    zip_bytes = _pack(
        "# ok\n",
        ("docs/report.pdf", b"%PDF-1.7\nrest"),
        ("images/logo.png", b"\x89PNG\r\n\x1a\nrest"),
    )
    validated = validate_skill_pack(
        skill_name="good-name",
        version="1.2.3",
        skill_md="# ok\n",
        zip_bytes=zip_bytes,
    )
    assert validated.manifest.file_count == 3


def test_validate_skill_pack_rejects_invalid_zip_bytes() -> None:
    with pytest.raises(SkillPackValidationError, match="valid ZIP"):
        validate_skill_pack(
            skill_name="good-name",
            version="1.2.3",
            skill_md="# ok\n",
            zip_bytes=b"not-a-zip",
        )
