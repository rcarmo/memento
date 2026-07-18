from __future__ import annotations

from pathlib import Path

from memento.repository.asset_packs import write_asset_version
from memento.repository.frontmatter import parse_concept_file, serialize_concept
from memento.repository.legacy_skill_packs import (
    list_skill_pack_versions,
    parse_skill_pack_document,
    skill_pack_paths,
)
from memento.repository.schema import ConceptDocument, ConceptFrontmatter, ConceptStatus
from memento.skill_packs import SkillPackManifest


def migrate_legacy_skill_packs(
    worktree: Path, *, actor: str = "memento-migration"
) -> tuple[str, ...]:
    """Convert accepted first-class skill packs into ordinary concepts and assets."""
    versions_root = worktree / "skills" / ".versions"
    if not versions_root.exists():
        return ()
    changed: set[str] = set()
    for skill_dir in sorted(path for path in versions_root.iterdir() if path.is_dir()):
        skill_name = skill_dir.name
        versions = list_skill_pack_versions(worktree, skill_name)
        if not versions:
            continue
        latest = versions[-1]
        latest_meta_path = worktree / skill_pack_paths(
            skill_name, latest
        ).version_document.removeprefix("/")
        latest_metadata, latest_body = parse_skill_pack_document(
            latest_meta_path.read_text(encoding="utf-8")
        )
        concept_path = f"/skills/{skill_name}.md"
        concept_file = worktree / concept_path.removeprefix("/")
        if concept_file.exists():
            try:
                existing = parse_concept_file(concept_file)
                concept_id = existing.frontmatter.id
                concept = ConceptDocument(
                    frontmatter=existing.frontmatter.model_copy(
                        update={"tags": tuple(sorted(set(existing.frontmatter.tags) | {"skill"}))}
                    ),
                    body=latest_body,
                )
            except Exception:
                concept_id = str(
                    latest_metadata.get("source_proposal_id") or f"legacy-{skill_name}"
                )
                concept = _legacy_concept(skill_name, concept_id, latest_body)
        else:
            concept_id = str(latest_metadata.get("source_proposal_id") or f"legacy-{skill_name}")
            concept = _legacy_concept(skill_name, concept_id, latest_body)
        concept_file.parent.mkdir(parents=True, exist_ok=True)
        concept_file.write_text(serialize_concept(concept), encoding="utf-8")
        changed.add(concept_path)
        for version in versions:
            paths = skill_pack_paths(skill_name, version)
            metadata_file = worktree / paths.version_document.removeprefix("/")
            zip_file = worktree / paths.zip_path.removeprefix("/")
            metadata, _body = parse_skill_pack_document(metadata_file.read_text(encoding="utf-8"))
            manifest = SkillPackManifest.model_validate(metadata["manifest"])
            changed.update(
                write_asset_version(
                    worktree,
                    concept_id=concept_id,
                    concept_path=concept_path,
                    asset_kind="skill",
                    version=version,
                    zip_bytes=zip_file.read_bytes(),
                    manifest=manifest,
                    accepted_by=str(metadata.get("accepted_by") or actor),
                    source_proposal_id=str(metadata.get("source_proposal_id") or "legacy"),
                )
            )
            metadata_file.unlink()
            zip_file.unlink()
            changed.update((paths.version_document, paths.zip_path))
        latest_doc = worktree / "skills" / f"{skill_name}.md"
        if latest_doc.exists() and latest_doc != concept_file:
            latest_doc.unlink()
    return tuple(sorted(changed))


def _legacy_concept(skill_name: str, concept_id: str, body: str) -> ConceptDocument:
    from datetime import UTC, datetime

    now = datetime.now(tz=UTC).replace(microsecond=0)
    return ConceptDocument(
        frontmatter=ConceptFrontmatter(
            id=concept_id,
            type="concept",
            title=skill_name.replace("-", " ").title(),
            status=ConceptStatus.ACTIVE,
            description=f"Versioned agent skill {skill_name}.",
            tags=("skill",),
            created_at=now,
            updated_at=now,
            updated_by="memento-migration",
        ),
        body=body,
    )
