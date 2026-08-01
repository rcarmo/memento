from __future__ import annotations

import hashlib
import re
from pathlib import Path

_POINTER = re.compile(
    rb"\Aversion https://" + b"git-" + rb"lfs.github.com/spec/v1\n"
    rb"oid sha256:([0-9a-f]{64})\nsize ([0-9]+)\n?\Z"
)
_LEGACY_FILTER = "filter=" + "lfs"


class LegacyBlobMigrationError(ValueError):
    pass


def repository_needs_legacy_blob_migration(root: Path) -> bool:
    attributes = root / ".gitattributes"
    if attributes.exists() and _LEGACY_FILTER in attributes.read_text(
        encoding="utf-8", errors="replace"
    ):
        return True
    return any(
        _POINTER.fullmatch(path.read_bytes()) is not None
        for path in root.rglob("*")
        if path.is_file() and path.stat().st_size <= 256
    )


def migrate_legacy_blobs_to_git(worktree: Path, *, object_root: Path) -> tuple[str, ...]:
    """Replace legacy external-blob pointers with verified ordinary Git blobs."""
    changed: set[str] = set()
    attributes = worktree / ".gitattributes"
    if attributes.exists():
        original = attributes.read_text(encoding="utf-8")
        retained = [line for line in original.splitlines() if _LEGACY_FILTER not in line]
        if retained:
            replacement = "\n".join(retained) + "\n"
            if replacement != original:
                attributes.write_text(replacement, encoding="utf-8")
                changed.add("/.gitattributes")
        else:
            attributes.unlink()
            changed.add("/.gitattributes")
    for target in sorted(path for path in worktree.rglob("*") if path.is_file()):
        payload = target.read_bytes()
        match = _POINTER.fullmatch(payload)
        if match is None:
            continue
        relative = target.relative_to(worktree)
        expected_digest = match.group(1).decode()
        expected_size = int(match.group(2))
        source = object_root / expected_digest[:2] / expected_digest[2:4] / expected_digest
        if not source.is_file():
            raise LegacyBlobMigrationError(f"legacy blob is unavailable: /{relative.as_posix()}")
        blob = source.read_bytes()
        if len(blob) != expected_size or hashlib.sha256(blob).hexdigest() != expected_digest:
            raise LegacyBlobMigrationError(
                f"legacy blob failed verification: /{relative.as_posix()}"
            )
        target.write_bytes(blob)
        changed.add(f"/{relative.as_posix()}")
    return tuple(sorted(changed))
