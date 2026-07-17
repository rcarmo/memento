from __future__ import annotations

import os
import stat
from dataclasses import dataclass
from pathlib import Path

RESERVED_FILENAMES = frozenset({"index.md", ".memory-schema.json"})
ROOT_RESERVED_FILENAMES = frozenset({"log.md"})


class PathSafetyError(Exception):
    """Raised when a repository path is unsafe."""


@dataclass(frozen=True, slots=True)
class SafeRepositoryPath:
    bundle_path: str
    absolute_path: Path


def _reject_unsafe_parts(relative_path: Path) -> None:
    if relative_path.is_absolute():
        raise PathSafetyError("absolute paths are not allowed")
    if any(part in {"", ".", ".."} for part in relative_path.parts):
        raise PathSafetyError("path traversal is not allowed")


def _check_existing_parents(root: Path, relative_path: Path) -> None:
    current = root
    for part in relative_path.parts[:-1]:
        current = current / part
        if not current.exists():
            continue
        mode = os.lstat(current).st_mode
        if stat.S_ISLNK(mode):
            raise PathSafetyError("symlink path components are not allowed")
        if not stat.S_ISDIR(mode):
            raise PathSafetyError("non-directory parent path component")


def _check_existing_target(path: Path) -> None:
    if not path.exists():
        return
    mode = os.lstat(path).st_mode
    if stat.S_ISLNK(mode):
        raise PathSafetyError("symlink target is not allowed")
    if not stat.S_ISREG(mode):
        raise PathSafetyError("special file target is not allowed")


def validate_repository_write_path(root: Path, bundle_path: str) -> SafeRepositoryPath:
    if not bundle_path.startswith("/"):
        raise PathSafetyError("bundle paths must start with '/'")
    relative_path = Path(bundle_path.removeprefix("/"))
    _reject_unsafe_parts(relative_path)
    filename = relative_path.name
    if filename in RESERVED_FILENAMES:
        raise PathSafetyError(f"reserved file cannot be written: {filename}")
    if len(relative_path.parts) == 1 and filename in ROOT_RESERVED_FILENAMES:
        raise PathSafetyError(f"reserved root file cannot be written: {filename}")
    absolute_path = root / relative_path
    _check_existing_parents(root, relative_path)
    _check_existing_target(absolute_path)
    return SafeRepositoryPath(bundle_path=bundle_path, absolute_path=absolute_path)


def validate_repository_read_path(root: Path, bundle_path: str) -> SafeRepositoryPath:
    safe_path = validate_repository_write_path(root, bundle_path)
    if not safe_path.absolute_path.exists():
        raise PathSafetyError(f"path does not exist: {bundle_path}")
    return safe_path


def is_reserved_bundle_path(bundle_path: str) -> bool:
    relative_path = Path(bundle_path.removeprefix("/"))
    if relative_path.name in RESERVED_FILENAMES:
        return True
    return len(relative_path.parts) == 1 and relative_path.name in ROOT_RESERVED_FILENAMES
