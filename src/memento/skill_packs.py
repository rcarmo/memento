from __future__ import annotations

import hashlib
import io
import mimetypes
import re
import stat
import zipfile
from pathlib import PurePosixPath

from pydantic import BaseModel, ConfigDict, Field

SKILL_NAME_PATTERN = re.compile(r"^[a-z0-9]+(?:-[a-z0-9]+)*$")
STABLE_SEMVER_PATTERN = re.compile(r"^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)$")

MAX_UNCOMPRESSED_BYTES = 50 * 1024 * 1024
MAX_FILE_COUNT = 512
MAX_ARCHIVE_ENTRY_COUNT = 1024
MAX_FILE_BYTES = 16 * 1024 * 1024
MAX_COMPRESSION_RATIO = 100
SKILL_MD_PATH = "SKILL.md"
_NESTED_ARCHIVE_SUFFIXES = (
    ".zip",
    ".tar",
    ".tgz",
    ".tbz",
    ".tbz2",
    ".txz",
    ".tar.gz",
    ".tar.bz2",
    ".tar.xz",
    ".gz",
    ".bz2",
    ".xz",
    ".7z",
    ".rar",
)


class SkillPackValidationError(ValueError):
    """Raised when a skill pack fails validation."""


class SkillPackManifestEntry(BaseModel):
    model_config = ConfigDict(extra="forbid", frozen=True)

    path: str = Field(min_length=1)
    size: int = Field(ge=0)
    media_type: str = Field(min_length=1)
    sha256: str = Field(min_length=64, max_length=64)


class SkillPackManifest(BaseModel):
    model_config = ConfigDict(extra="forbid", frozen=True)

    entries: tuple[SkillPackManifestEntry, ...]
    sha256: str = Field(min_length=64, max_length=64)
    total_uncompressed_bytes: int = Field(ge=0)
    file_count: int = Field(ge=0)


class ValidatedSkillPack(BaseModel):
    model_config = ConfigDict(extra="forbid", frozen=True)

    skill_name: str = Field(min_length=1)
    version: str = Field(min_length=1)
    skill_md: str
    zip_bytes: bytes
    manifest: SkillPackManifest


def parse_stable_semver(version: str) -> tuple[int, int, int]:
    """Parse a stable semantic version into a sortable tuple."""
    match = STABLE_SEMVER_PATTERN.fullmatch(version)
    if match is None:
        raise SkillPackValidationError("version must be a stable semantic version")
    return tuple(int(part) for part in match.groups())  # type: ignore[return-value]


def validate_skill_pack(
    *, skill_name: str, version: str, skill_md: str, zip_bytes: bytes
) -> ValidatedSkillPack:
    """Validate a skill ZIP payload and return immutable metadata plus the original bytes."""
    if not SKILL_NAME_PATTERN.fullmatch(skill_name):
        raise SkillPackValidationError(
            "skill_name must be lowercase alphanumeric words joined by hyphens"
        )
    parse_stable_semver(version)

    expected_skill_md = skill_md.encode("utf-8")
    pack_sha256 = hashlib.sha256(zip_bytes).hexdigest()

    try:
        archive = zipfile.ZipFile(io.BytesIO(zip_bytes))
    except zipfile.BadZipFile as exc:
        raise SkillPackValidationError("zip_bytes is not a valid ZIP archive") from exc

    seen_paths: set[str] = set()
    files: list[SkillPackManifestEntry] = []
    total_uncompressed_bytes = 0
    root_skill_md_found = False

    with archive:
        archive_entries = archive.infolist()
        if len(archive_entries) > MAX_ARCHIVE_ENTRY_COUNT:
            raise SkillPackValidationError("archive exceeds maximum entry count")
        for info in archive_entries:
            path = _validate_member_path(info.filename)
            if path in seen_paths:
                raise SkillPackValidationError(f"duplicate path in archive: {path}")
            seen_paths.add(path)

            _validate_member_metadata(info, path=path)

            if info.is_dir():
                continue

            if _has_nested_archive_suffix(path):
                raise SkillPackValidationError(f"nested archive entries are not allowed: {path}")

            if info.file_size > MAX_FILE_BYTES:
                raise SkillPackValidationError(f"file exceeds maximum uncompressed size: {path}")

            total_uncompressed_bytes += info.file_size
            if total_uncompressed_bytes > MAX_UNCOMPRESSED_BYTES:
                raise SkillPackValidationError("archive exceeds maximum uncompressed size")

            file_count = len(files) + 1
            if file_count > MAX_FILE_COUNT:
                raise SkillPackValidationError("archive exceeds maximum file count")

            _validate_compression_ratio(info, path=path)
            data = archive.read(info)
            _validate_file_magic(data, path=path)

            if path == SKILL_MD_PATH:
                if data != expected_skill_md:
                    raise SkillPackValidationError(
                        "root SKILL.md contents must exactly match supplied skill_md UTF-8 bytes"
                    )
                root_skill_md_found = True

            files.append(
                SkillPackManifestEntry(
                    path=path,
                    size=info.file_size,
                    media_type=_guess_media_type(path),
                    sha256=hashlib.sha256(data).hexdigest(),
                )
            )

    if not root_skill_md_found:
        raise SkillPackValidationError("archive must contain root SKILL.md")

    manifest = SkillPackManifest(
        entries=tuple(files),
        sha256=pack_sha256,
        total_uncompressed_bytes=total_uncompressed_bytes,
        file_count=len(files),
    )
    return ValidatedSkillPack(
        skill_name=skill_name,
        version=version,
        skill_md=skill_md,
        zip_bytes=zip_bytes,
        manifest=manifest,
    )


def _validate_member_path(raw_path: str) -> str:
    if not raw_path:
        raise SkillPackValidationError("archive contains an empty path")
    if "\\" in raw_path:
        raise SkillPackValidationError(f"backslashes are not allowed in archive paths: {raw_path}")
    if raw_path.startswith("/"):
        raise SkillPackValidationError(f"absolute paths are not allowed in archive: {raw_path}")
    if any(ord(char) < 32 or ord(char) == 127 for char in raw_path):
        raise SkillPackValidationError(
            f"control characters are not allowed in archive paths: {raw_path!r}"
        )

    path = PurePosixPath(raw_path)
    parts = path.parts
    if not parts:
        raise SkillPackValidationError("archive contains an empty path")
    if any(part in {"", ".", ".."} for part in parts):
        raise SkillPackValidationError(f"path traversal is not allowed in archive: {raw_path}")

    normalized = path.as_posix()
    if raw_path.endswith("/") and not normalized.endswith("/"):
        normalized = f"{normalized}/"
    return normalized


def _validate_member_metadata(info: zipfile.ZipInfo, *, path: str) -> None:
    if info.flag_bits & 0x1:
        raise SkillPackValidationError(f"encrypted entries are not allowed: {path}")

    mode = (info.external_attr >> 16) & 0o177777
    if mode == 0:
        return

    file_type = stat.S_IFMT(mode)
    if info.is_dir():
        if file_type not in {0, stat.S_IFDIR}:
            raise SkillPackValidationError(f"directory entry has unsupported file mode: {path}")
        return

    if file_type in {0, stat.S_IFREG}:
        return
    if file_type == stat.S_IFLNK:
        raise SkillPackValidationError(f"symlinks are not allowed in archive: {path}")
    raise SkillPackValidationError(f"special file types are not allowed in archive: {path}")


def _validate_compression_ratio(info: zipfile.ZipInfo, *, path: str) -> None:
    if info.file_size == 0:
        return
    if info.compress_size == 0:
        raise SkillPackValidationError(f"invalid compressed size for entry: {path}")
    if info.file_size / info.compress_size > MAX_COMPRESSION_RATIO:
        raise SkillPackValidationError(f"compression ratio exceeds limit: {path}")


def _has_nested_archive_suffix(path: str) -> bool:
    lower_path = path.lower()
    return any(lower_path.endswith(suffix) for suffix in _NESTED_ARCHIVE_SUFFIXES)


def _guess_media_type(path: str) -> str:
    media_type, _encoding = mimetypes.guess_type(path, strict=False)
    return media_type or "application/octet-stream"


def _validate_file_magic(data: bytes, *, path: str) -> None:
    if data.startswith(b"\x7fELF"):
        raise SkillPackValidationError(f"ELF binaries are not allowed: {path}")
    if data.startswith(b"MZ"):
        raise SkillPackValidationError(f"PE binaries are not allowed: {path}")
    if data.startswith(
        (b"\xfe\xed\xfa\xce", b"\xfe\xed\xfa\xcf", b"\xce\xfa\xed\xfe", b"\xcf\xfa\xed\xfe")
    ):
        raise SkillPackValidationError(f"Mach-O binaries are not allowed: {path}")
    if data.startswith(b"\xca\xfe\xba\xbe") and len(data) >= 8:
        architecture_count = int.from_bytes(data[4:8], byteorder="big", signed=False)
        if 0 < architecture_count < 0x100:
            raise SkillPackValidationError(f"Mach-O binaries are not allowed: {path}")
