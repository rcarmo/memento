from __future__ import annotations

import hashlib
import json
import shutil
import sqlite3
import subprocess
import tarfile
from dataclasses import dataclass
from pathlib import Path
from tempfile import TemporaryDirectory
from typing import Any

from memento.app import MementoRuntime, runtime_paths_for
from memento.config import ServiceConfig
from memento.repository.git import materialize_current_checkout


@dataclass(frozen=True, slots=True)
class BackupManifest:
    schema_version: int
    repo_revision: str
    files: dict[str, str]

    def as_dict(self) -> dict[str, Any]:
        return {
            "schema_version": self.schema_version,
            "repo_revision": self.repo_revision,
            "files": self.files,
        }


MANIFEST_NAME = "manifest.json"


def create_backup(runtime: MementoRuntime, destination: Path) -> BackupManifest:
    destination.mkdir(parents=True, exist_ok=True)
    repo_tar = destination / "repo.git.tar.gz"
    control_copy = destination / "control.sqlite"
    derived_copy = destination / "derived.sqlite"

    with tarfile.open(repo_tar, "w:gz") as archive:
        archive.add(runtime.paths.repo_paths.bare_dir, arcname="repo.git")
    _sqlite_backup(runtime.paths.control_db, control_copy)
    if runtime.paths.derived_db.exists():
        _sqlite_backup(runtime.paths.derived_db, derived_copy)

    files = {
        repo_tar.name: _sha256(repo_tar),
        control_copy.name: _sha256(control_copy),
    }
    if derived_copy.exists():
        files[derived_copy.name] = _sha256(derived_copy)

    manifest = BackupManifest(
        schema_version=1,
        repo_revision=runtime.status_snapshot()["repo_revision"],
        files=files,
    )
    (destination / MANIFEST_NAME).write_text(
        json.dumps(manifest.as_dict(), indent=2, sort_keys=True) + "\n",
        encoding="utf-8",
    )
    return manifest


def restore_backup(
    config: ServiceConfig,
    backup_dir: Path,
    *,
    rebuild_derived: bool = True,
) -> dict[str, Any]:
    manifest_path = backup_dir / MANIFEST_NAME
    manifest = BackupManifest(**json.loads(manifest_path.read_text(encoding="utf-8")))
    for name, digest in manifest.files.items():
        path = backup_dir / name
        if not path.exists():
            raise ValueError(f"backup file is missing: {name}")
        if _sha256(path) != digest:
            raise ValueError(f"checksum mismatch for {name}")

    paths = runtime_paths_for(config)
    temp_parent = paths.root.parent if paths.root.parent.exists() else backup_dir.parent
    with TemporaryDirectory(dir=str(temp_parent)) as temp_dir:
        staging_root = Path(temp_dir) / "state"
        staging_root.mkdir(parents=True, exist_ok=True)
        with tarfile.open(backup_dir / "repo.git.tar.gz", "r:gz") as archive:
            _safe_extract_tar(archive, staging_root)
        shutil.copy2(backup_dir / "control.sqlite", staging_root / "control.sqlite")
        derived_backup = backup_dir / "derived.sqlite"
        derived_verified = manifest.files.get("derived.sqlite")
        if not rebuild_derived and derived_verified is not None:
            shutil.copy2(derived_backup, staging_root / "derived.sqlite")
        staged_config = config.model_copy(
            update={
                "repository": config.repository.model_copy(update={"root_path": str(staging_root)})
            }
        )
        staged_paths = runtime_paths_for(staged_config)
        archived_head = subprocess.run(
            [
                "git",
                "--git-dir",
                str(staged_paths.repo_paths.bare_dir),
                "rev-parse",
                "refs/heads/main",
            ],
            capture_output=True,
            text=True,
            check=True,
        ).stdout.strip()
        if archived_head != manifest.repo_revision:
            raise ValueError("backup manifest revision does not match archived main")
        materialize_current_checkout(staged_paths.repo_paths, revision=manifest.repo_revision)
        if rebuild_derived:
            from memento.derived.index import DerivedIndex

            derived = DerivedIndex(staged_paths.derived_db)
            derived.rebuild(
                staged_paths.repo_paths.current_dir,
                repo_revision=manifest.repo_revision,
            )
        backup_old = paths.root.with_name(paths.root.name + ".pre-restore")
        if backup_old.exists():
            shutil.rmtree(backup_old)
        if paths.root.exists():
            paths.root.rename(backup_old)
        staging_root.rename(paths.root)
        if backup_old.exists():
            shutil.rmtree(backup_old)
    return {
        "repo_revision": manifest.repo_revision,
        "restored_root": str(paths.root),
        "rebuild_derived": rebuild_derived,
    }


def _sqlite_backup(source_path: Path, target_path: Path) -> None:
    source = sqlite3.connect(source_path)
    target = sqlite3.connect(target_path)
    try:
        source.backup(target)
    finally:
        target.close()
        source.close()


def _safe_extract_tar(archive: tarfile.TarFile, destination: Path) -> None:
    destination = destination.resolve()
    for member in archive.getmembers():
        if member.issym() or member.islnk():
            raise ValueError(f"refusing to restore linked archive member: {member.name}")
        target = (destination / member.name).resolve()
        if destination not in target.parents and target != destination:
            raise ValueError(
                f"refusing to restore archive member outside destination: {member.name}"
            )
    archive.extractall(destination, filter="data")


def _sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        while True:
            block = handle.read(65_536)
            if not block:
                break
            digest.update(block)
    return digest.hexdigest()
