from __future__ import annotations

import hashlib
from pathlib import Path

import pytest

from memento.repository.legacy_blob_migration import (
    migrate_legacy_blobs_to_git,
    repository_needs_legacy_blob_migration,
)


def pointer(contents: bytes) -> bytes:
    return (
        "version https://git-" + "lfs.github.com/spec/v1\n"
        f"oid sha256:{hashlib.sha256(contents).hexdigest()}\n"
        f"size {len(contents)}\n"
    ).encode()


def test_migrates_pointer_and_removes_legacy_filter_attributes(tmp_path: Path) -> None:
    worktree = tmp_path / "worktree"
    hydrated = tmp_path / "hydrated"
    worktree.mkdir()
    hydrated.mkdir()
    asset = Path(".assets/concept/templates/1.0.0.zip")
    contents = b"ordinary Git blob"
    (worktree / asset).parent.mkdir(parents=True)
    (hydrated / asset).parent.mkdir(parents=True)
    (worktree / asset).write_bytes(pointer(contents))
    (hydrated / asset).write_bytes(contents)
    (worktree / ".gitattributes").write_text(
        "*.txt text\n.assets/**/*.zip filter="
        + "lfs"
        + " diff="
        + "lfs"
        + " merge="
        + "lfs"
        + " -text\n",
        encoding="utf-8",
    )

    assert repository_needs_legacy_blob_migration(worktree)
    changed = migrate_legacy_blobs_to_git(worktree, hydrated_root=hydrated)

    assert changed == ("/.assets/concept/templates/1.0.0.zip", "/.gitattributes")
    assert (worktree / asset).read_bytes() == contents
    assert (worktree / ".gitattributes").read_text(encoding="utf-8") == "*.txt text\n"
    assert not repository_needs_legacy_blob_migration(worktree)


def test_rejects_hydrated_blob_with_wrong_digest(tmp_path: Path) -> None:
    worktree = tmp_path / "worktree"
    hydrated = tmp_path / "hydrated"
    worktree.mkdir()
    hydrated.mkdir()
    asset = Path(".assets/concept/templates/1.0.0.zip")
    (worktree / asset).parent.mkdir(parents=True)
    (hydrated / asset).parent.mkdir(parents=True)
    (worktree / asset).write_bytes(pointer(b"wanted"))
    (hydrated / asset).write_bytes(b"wrong!")

    with pytest.raises(ValueError, match="failed verification"):
        migrate_legacy_blobs_to_git(worktree, hydrated_root=hydrated)
