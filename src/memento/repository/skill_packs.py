from __future__ import annotations

import json
from dataclasses import dataclass
from pathlib import Path

from memento.skill_packs import (
    SkillPackManifest,
    SkillPackValidationError,
    ValidatedSkillPack,
    parse_stable_semver,
)

LFS_ATTRIBUTES_LINE = "skills/.versions/**/*.zip filter=lfs diff=lfs merge=lfs -text"


@dataclass(frozen=True, slots=True)
class SkillPackPaths:
    latest_document: str
    version_document: str
    zip_path: str


def skill_pack_paths(skill_name: str, version: str) -> SkillPackPaths:
    parse_stable_semver(version)
    return SkillPackPaths(
        latest_document=f"/skills/{skill_name}.md",
        version_document=f"/skills/.versions/{skill_name}/{version}.md",
        zip_path=f"/skills/.versions/{skill_name}/{version}.zip",
    )


def ensure_lfs_attributes(worktree: Path) -> bool:
    path = worktree / ".gitattributes"
    lines = path.read_text(encoding="utf-8").splitlines() if path.exists() else []
    if LFS_ATTRIBUTES_LINE in lines:
        return False
    lines.append(LFS_ATTRIBUTES_LINE)
    path.write_text("\n".join(lines) + "\n", encoding="utf-8")
    return True


def write_skill_pack_version(
    worktree: Path,
    pack: ValidatedSkillPack,
    *,
    accepted_by: str,
    source_proposal_id: str,
) -> tuple[str, ...]:
    paths = skill_pack_paths(pack.skill_name, pack.version)
    version_doc = worktree / paths.version_document.removeprefix("/")
    zip_path = worktree / paths.zip_path.removeprefix("/")
    latest_doc = worktree / paths.latest_document.removeprefix("/")
    if version_doc.exists() or zip_path.exists():
        raise SkillPackValidationError(
            f"accepted skill version already exists: {pack.skill_name} {pack.version}"
        )
    version_doc.parent.mkdir(parents=True, exist_ok=True)
    zip_path.write_bytes(pack.zip_bytes)
    rendered = render_skill_pack_document(
        pack, accepted_by=accepted_by, source_proposal_id=source_proposal_id
    )
    version_doc.write_text(rendered, encoding="utf-8")
    latest = max(list_skill_pack_versions(worktree, pack.skill_name), key=parse_stable_semver)
    latest_source = worktree / skill_pack_paths(
        pack.skill_name, latest
    ).version_document.removeprefix("/")
    latest_doc.parent.mkdir(parents=True, exist_ok=True)
    latest_doc.write_bytes(latest_source.read_bytes())
    changed = [paths.version_document, paths.zip_path, paths.latest_document]
    if ensure_lfs_attributes(worktree):
        changed.append("/.gitattributes")
    return tuple(sorted(changed))


def render_skill_pack_document(
    pack: ValidatedSkillPack,
    *,
    accepted_by: str,
    source_proposal_id: str,
) -> str:
    metadata = {
        "schema_version": 1,
        "kind": "skill_pack_version",
        "skill_name": pack.skill_name,
        "version": pack.version,
        "zip_path": skill_pack_paths(pack.skill_name, pack.version).zip_path,
        "zip_sha256": pack.manifest.sha256,
        "total_uncompressed_bytes": pack.manifest.total_uncompressed_bytes,
        "file_count": pack.manifest.file_count,
        "accepted_by": accepted_by,
        "source_proposal_id": source_proposal_id,
        "manifest": pack.manifest.model_dump(mode="json"),
    }
    return (
        "---skill-pack-json\n"
        + json.dumps(metadata, sort_keys=True, separators=(",", ":"))
        + "\n---\n"
        + pack.skill_md
    )


def parse_skill_pack_document(text: str) -> tuple[dict[str, object], str]:
    prefix = "---skill-pack-json\n"
    separator = "\n---\n"
    if not text.startswith(prefix) or separator not in text:
        raise SkillPackValidationError("invalid skill pack metadata document")
    raw_metadata, skill_md = text[len(prefix) :].split(separator, 1)
    try:
        metadata = json.loads(raw_metadata)
    except json.JSONDecodeError as exc:
        raise SkillPackValidationError("invalid skill pack metadata JSON") from exc
    if not isinstance(metadata, dict) or metadata.get("kind") != "skill_pack_version":
        raise SkillPackValidationError("invalid skill pack metadata kind")
    SkillPackManifest.model_validate(metadata.get("manifest"))
    return metadata, skill_md


def list_skill_pack_versions(root: Path, skill_name: str) -> tuple[str, ...]:
    directory = root / "skills" / ".versions" / skill_name
    if not directory.exists():
        return ()
    versions = []
    for path in directory.glob("*.md"):
        version = path.stem
        parse_stable_semver(version)
        versions.append(version)
    return tuple(sorted(versions, key=parse_stable_semver))


def resolve_skill_pack_version(root: Path, skill_name: str, version: str | None = None) -> str:
    versions = list_skill_pack_versions(root, skill_name)
    if not versions:
        raise FileNotFoundError(f"unknown skill pack: {skill_name}")
    if version is None:
        return versions[-1]
    parse_stable_semver(version)
    if version not in versions:
        raise FileNotFoundError(f"unknown skill pack version: {skill_name} {version}")
    return version


def retention_partition(
    versions: tuple[str, ...], *, keep: int = 5
) -> tuple[tuple[str, ...], tuple[str, ...]]:
    if keep < 1:
        raise ValueError("keep must be at least one")
    ordered = tuple(sorted(set(versions), key=parse_stable_semver, reverse=True))
    return ordered[:keep], ordered[keep:]
