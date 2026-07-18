from __future__ import annotations

import io
import shutil
import tempfile
import zipfile
from pathlib import Path

from memento.skill_packs import ValidatedSkillPack, validate_skill_pack


class SkillImportConflictError(FileExistsError):
    """Raised when a workspace already contains the recalled skill."""


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
