"""Bounded, integrity-checked reads shared by accepted and proposed asset packs."""

from __future__ import annotations

import base64
import hashlib
import json
import re
import stat
import zipfile
import zlib
from typing import IO, Any

from memento.skill_packs import (
    MAX_ARCHIVE_ENTRY_COUNT,
    MAX_FILE_BYTES,
    MAX_FILE_COUNT,
    MAX_UNCOMPRESSED_BYTES,
    MAX_ZIP_BYTES,
    SkillPackManifest,
)

DEFAULT_CHUNK_BYTES = 65_536
MAX_CHUNK_BYTES = 262_144
MAX_INLINE_ARCHIVE_BYTES = 16_384
MAX_METADATA_BYTES = 2 * 1024 * 1024


def retrieval_limits() -> dict[str, int]:
    return {
        "max_upload_zip_bytes": MAX_ZIP_BYTES,
        "max_archive_bytes": MAX_ZIP_BYTES,
        "max_uncompressed_bytes": MAX_UNCOMPRESSED_BYTES,
        "max_file_bytes": MAX_FILE_BYTES,
        "max_file_count": MAX_FILE_COUNT,
        "default_chunk_bytes": DEFAULT_CHUNK_BYTES,
        "max_chunk_bytes": MAX_CHUNK_BYTES,
        "max_inline_archive_bytes": MAX_INLINE_ARCHIVE_BYTES,
        "max_metadata_bytes": MAX_METADATA_BYTES,
    }


class AssetReadError(ValueError):
    """Asset input, stored metadata or returned bytes cannot be trusted."""


def validate_range(offset: int, limit: int, total: int | None = None) -> None:
    if type(offset) is not int or offset < 0:
        raise AssetReadError("asset offset must be a nonnegative integer")
    if type(limit) is not int or not 1 <= limit <= MAX_CHUNK_BYTES:
        raise AssetReadError(f"asset limit must be between 1 and {MAX_CHUNK_BYTES}")
    if total is not None and offset > total:
        raise AssetReadError("asset offset exceeds total bytes")


def validate_file_path(path: str) -> None:
    if (
        not path
        or "\\" in path
        or any(part in {"", ".", ".."} for part in path.split("/"))
        or any(ord(char) < 32 or ord(char) == 127 for char in path)
    ):
        raise AssetReadError("asset file_path must be a canonical manifest-relative path")


def checked_manifest(value: object, zip_sha256: str) -> SkillPackManifest:
    if not re.fullmatch(r"[0-9a-f]{64}", zip_sha256):
        raise AssetReadError("invalid ZIP digest in asset metadata")
    manifest = SkillPackManifest.model_validate(value)
    if manifest.sha256 != zip_sha256:
        raise AssetReadError("asset manifest/ZIP digest mismatch")
    if (
        manifest.file_count != len(manifest.entries)
        or not 1 <= manifest.file_count <= MAX_FILE_COUNT
    ):
        raise AssetReadError("invalid asset manifest file count")
    seen: set[str] = set()
    for entry in manifest.entries:
        validate_file_path(entry.path)
        if entry.path in seen or not 0 <= entry.size <= MAX_FILE_BYTES:
            raise AssetReadError("invalid or duplicate asset manifest entry")
        if not re.fullmatch(r"[0-9a-f]{64}", entry.sha256):
            raise AssetReadError("invalid asset file digest")
        seen.add(entry.path)
    total = sum(entry.size for entry in manifest.entries)
    if manifest.total_uncompressed_bytes != total or total > MAX_UNCOMPRESSED_BYTES:
        raise AssetReadError("invalid asset manifest uncompressed size")
    return manifest


def manifest_digest(manifest: SkillPackManifest) -> str:
    return hashlib.sha256(
        json.dumps(manifest.model_dump(mode="json"), sort_keys=True, separators=(",", ":")).encode()
    ).hexdigest()


def check_expected_digest(expected: str | None, actual: str) -> None:
    if expected is not None and (not re.fullmatch(r"[0-9a-f]{64}", expected) or expected != actual):
        raise AssetReadError("asset digest mismatch; pin the resolved version and ZIP SHA-256")


def verified_slice(stream: IO[bytes], *, total: int, digest: str, offset: int, limit: int) -> bytes:
    """Hash the full bounded stream; retain only the requested slice in memory."""
    validate_range(offset, limit, total)
    hasher = hashlib.sha256()
    chunk = bytearray()
    consumed = 0
    while block := stream.read(65_536):
        end = consumed + len(block)
        if end > total:
            raise AssetReadError("asset size differs from metadata")
        hasher.update(block)
        start_slice = max(offset, consumed)
        end_slice = min(offset + limit, end)
        if end_slice > start_slice:
            chunk.extend(block[start_slice - consumed : end_slice - consumed])
        consumed = end
    if consumed != total or hasher.hexdigest() != digest:
        raise AssetReadError("asset digest/size mismatch")
    return bytes(chunk)


def range_payload(content: bytes, *, offset: int, total: int) -> dict[str, Any]:
    next_offset = offset + len(content)
    return {
        "offset": offset,
        "returned_bytes": len(content),
        "total_bytes": total,
        "truncated": offset > 0 or next_offset < total,
        "next_offset": next_offset if next_offset < total else None,
        "content_sha256": hashlib.sha256(content).hexdigest(),
    }


def read_pack_file(
    stream: IO[bytes], manifest: SkillPackManifest, *, file_path: str, offset: int, limit: int
) -> dict[str, Any]:
    try:
        return _read_pack_file(stream, manifest, file_path=file_path, offset=offset, limit=limit)
    except (zipfile.BadZipFile, zlib.error, EOFError, NotImplementedError, RuntimeError) as exc:
        raise AssetReadError("asset ZIP validation failed") from exc


def _read_pack_file(
    stream: IO[bytes], manifest: SkillPackManifest, *, file_path: str, offset: int, limit: int
) -> dict[str, Any]:
    validate_file_path(file_path)
    entries = {entry.path: entry for entry in manifest.entries}
    entry = entries.get(file_path)
    if entry is None:
        raise KeyError("asset file is not declared in the manifest")
    validate_range(offset, limit, entry.size)
    with zipfile.ZipFile(stream) as archive:
        infos = archive.infolist()
        if len(infos) > MAX_ARCHIVE_ENTRY_COUNT:
            raise AssetReadError("asset archive has too many entries")
        declared: set[str] = set()
        all_names: set[str] = set()
        for info in infos:
            name = info.filename[:-1] if info.is_dir() else info.filename
            validate_file_path(name)
            mode = info.external_attr >> 16
            if info.filename in all_names or info.flag_bits & 1:
                raise AssetReadError("duplicate or encrypted ZIP entry")
            all_names.add(info.filename)
            kind = stat.S_IFMT(mode)
            if info.is_dir():
                if kind not in {0, stat.S_IFDIR}:
                    raise AssetReadError("unsafe ZIP directory entry")
                continue
            if kind not in {0, stat.S_IFREG}:
                raise AssetReadError("symlink or special ZIP entry")
            if name not in entries or info.file_size != entries[name].size:
                raise AssetReadError("ZIP entries differ from the manifest")
            declared.add(name)
        if declared != set(entries):
            raise AssetReadError("ZIP is missing manifest entries")
        with archive.open(file_path) as source:
            content = verified_slice(
                source, total=entry.size, digest=entry.sha256, offset=offset, limit=limit
            )
    result = {
        "path": file_path,
        "media_type": entry.media_type,
        "sha256": entry.sha256,
        **range_payload(content, offset=offset, total=entry.size),
    }
    try:
        result.update(encoding="utf-8", content=content.decode("utf-8"))
    except UnicodeDecodeError:
        result.update(encoding="base64", content=base64.b64encode(content).decode("ascii"))
    return result


def validate_archive_size(total: int) -> None:
    if not 0 < total <= MAX_ZIP_BYTES:
        raise AssetReadError("invalid asset ZIP size")
