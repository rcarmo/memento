from __future__ import annotations

import json
import re
from pathlib import Path

from memento.skill_packs import SkillPackManifest, SkillPackValidationError, parse_stable_semver

ASSET_KIND_PATTERN = re.compile(r"^[a-z0-9]+(?:-[a-z0-9]+)*$")
LFS_ATTRIBUTES_LINE = ".assets/**/*.zip filter=lfs diff=lfs merge=lfs -text"


def validate_asset_kind(asset_kind: str) -> str:
    if not ASSET_KIND_PATTERN.fullmatch(asset_kind):
        raise SkillPackValidationError(
            "asset_kind must be lowercase alphanumeric words joined by hyphens"
        )
    return asset_kind


def asset_version_paths(concept_id: str, asset_kind: str, version: str) -> tuple[str, str]:
    if not re.fullmatch(r"[0-9a-fA-F-]{8,64}", concept_id):
        raise SkillPackValidationError("invalid concept id for asset storage")
    validate_asset_kind(asset_kind)
    parse_stable_semver(version)
    base = f"/.assets/{concept_id}/{asset_kind}/{version}"
    return f"{base}.json", f"{base}.zip"


def ensure_asset_lfs_attributes(worktree: Path) -> bool:
    path = worktree / ".gitattributes"
    lines = path.read_text(encoding="utf-8").splitlines() if path.exists() else []
    if LFS_ATTRIBUTES_LINE in lines:
        return False
    lines.append(LFS_ATTRIBUTES_LINE)
    path.write_text("\n".join(lines) + "\n", encoding="utf-8")
    return True


def write_asset_version(
    worktree: Path,
    *,
    concept_id: str,
    concept_path: str,
    asset_kind: str,
    version: str,
    zip_bytes: bytes,
    manifest: SkillPackManifest,
    accepted_by: str,
    source_proposal_id: str,
) -> tuple[str, ...]:
    metadata_path, zip_path = asset_version_paths(concept_id, asset_kind, version)
    metadata_file = worktree / metadata_path.removeprefix("/")
    zip_file = worktree / zip_path.removeprefix("/")
    if metadata_file.exists() or zip_file.exists():
        raise SkillPackValidationError(
            f"accepted asset version already exists: {concept_path} {asset_kind} {version}"
        )
    metadata_file.parent.mkdir(parents=True, exist_ok=True)
    metadata = {
        "schema_version": 1,
        "kind": "asset_pack_version",
        "concept_id": concept_id,
        "concept_path": concept_path,
        "asset_kind": asset_kind,
        "version": version,
        "zip_sha256": manifest.sha256,
        "accepted_by": accepted_by,
        "source_proposal_id": source_proposal_id,
        "manifest": manifest.model_dump(mode="json"),
    }
    metadata_file.write_text(json.dumps(metadata, indent=2, sort_keys=True) + "\n")
    zip_file.write_bytes(zip_bytes)
    changed = [metadata_path, zip_path]
    if ensure_asset_lfs_attributes(worktree):
        changed.append("/.gitattributes")
    return tuple(sorted(changed))


def list_asset_versions(root: Path, concept_id: str, asset_kind: str) -> tuple[str, ...]:
    validate_asset_kind(asset_kind)
    directory = root / ".assets" / concept_id / asset_kind
    if not directory.exists():
        return ()
    versions = [path.stem for path in directory.glob("*.json")]
    for version in versions:
        parse_stable_semver(version)
    return tuple(sorted(versions, key=parse_stable_semver))


def resolve_asset_version(root: Path, concept_id: str, asset_kind: str, version: str | None) -> str:
    versions = list_asset_versions(root, concept_id, asset_kind)
    if not versions:
        raise FileNotFoundError(f"no {asset_kind} asset pack for concept")
    if version is None:
        return versions[-1]
    parse_stable_semver(version)
    if version not in versions:
        raise FileNotFoundError(f"unknown {asset_kind} asset version: {version}")
    return version


def load_asset_metadata(
    root: Path, concept_id: str, asset_kind: str, version: str
) -> dict[str, object]:
    metadata_path, _zip_path = asset_version_paths(concept_id, asset_kind, version)
    value = json.loads((root / metadata_path.removeprefix("/")).read_text())
    if not isinstance(value, dict) or value.get("kind") != "asset_pack_version":
        raise SkillPackValidationError("invalid asset metadata")
    return value


def retention_partition(
    versions: tuple[str, ...], *, keep: int = 5
) -> tuple[tuple[str, ...], tuple[str, ...]]:
    if keep < 1:
        raise ValueError("keep must be at least one")
    ordered = tuple(sorted(set(versions), key=parse_stable_semver, reverse=True))
    return ordered[:keep], ordered[keep:]
