from __future__ import annotations

import argparse
import io
import shutil
import tempfile
import zipfile
from pathlib import Path

from memento.skill_packs import ValidatedSkillPack, validate_skill_pack


class SkillImportConflictError(FileExistsError):
    """Raised when a workspace already contains the recalled skill."""


def main(argv: list[str] | None = None) -> int:
    """Import a recalled ZIP and exact SKILL.md into a workspace."""
    parser = argparse.ArgumentParser(description="Import a validated Memento skill pack")
    parser.add_argument("--workspace", default=".")
    parser.add_argument("--name", required=True)
    parser.add_argument("--version", required=True)
    parser.add_argument("--skill-md", required=True)
    parser.add_argument("--zip", dest="zip_path", required=True)
    args = parser.parse_args(argv)
    destination = import_skill_pack(
        workspace=Path(args.workspace),
        skill_name=args.name,
        version=args.version,
        skill_md=Path(args.skill_md).read_text(encoding="utf-8"),
        zip_bytes=Path(args.zip_path).read_bytes(),
    )
    print(destination)
    return 0


def import_skill_pack(
    *,
    workspace: Path,
    skill_name: str,
    version: str,
    skill_md: str,
    zip_bytes: bytes,
) -> Path:
    """Validate and atomically import a recalled pack into `.pi/skills/<name>`."""
    validated = validate_skill_pack(
        skill_name=skill_name,
        version=version,
        skill_md=skill_md,
        zip_bytes=zip_bytes,
    )
    _reject_symlinked_import_root(workspace)
    destination = workspace / ".pi" / "skills" / skill_name
    if destination.exists():
        raise SkillImportConflictError(f"skill destination already exists: {destination}")
    destination.parent.mkdir(parents=True, exist_ok=True)
    temp_dir = Path(tempfile.mkdtemp(prefix=f".{skill_name}-", dir=destination.parent))
    try:
        _extract_validated_pack(validated, temp_dir)
        temp_dir.rename(destination)
    except Exception:
        shutil.rmtree(temp_dir, ignore_errors=True)
        raise
    return destination


def _reject_symlinked_import_root(workspace: Path) -> None:
    workspace = workspace.resolve(strict=True)
    current = workspace
    for part in (".pi", "skills"):
        current = current / part
        if current.is_symlink():
            raise ValueError(f"skill import parent must not be a symlink: {current}")
        if current.exists() and not current.is_dir():
            raise ValueError(f"skill import parent must be a directory: {current}")


def _extract_validated_pack(pack: ValidatedSkillPack, destination: Path) -> None:
    allowed = {entry.path: entry for entry in pack.manifest.entries}
    with zipfile.ZipFile(io.BytesIO(pack.zip_bytes)) as archive:
        for info in archive.infolist():
            if info.is_dir():
                continue
            entry = allowed.get(info.filename)
            if entry is None:
                raise ValueError(f"validated manifest is missing ZIP entry: {info.filename}")
            target = destination / entry.path
            target.parent.mkdir(parents=True, exist_ok=True)
            data = archive.read(info)
            if len(data) != entry.size:
                raise ValueError(f"ZIP entry size changed during import: {entry.path}")
            target.write_bytes(data)
            target.chmod(0o644)
    for directory in sorted(
        (path for path in destination.rglob("*") if path.is_dir()), reverse=True
    ):
        directory.chmod(0o755)
    destination.chmod(0o755)
