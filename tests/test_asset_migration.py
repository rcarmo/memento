from __future__ import annotations

import io
import zipfile
from pathlib import Path

from memento.repository.asset_migration import migrate_legacy_skill_packs
from memento.repository.asset_packs import list_asset_versions, load_asset_metadata
from memento.repository.bundle import read_bundle_entry
from memento.repository.legacy_skill_packs import write_skill_pack_version
from memento.skill_packs import validate_skill_pack


def test_migrate_legacy_skill_pack_to_concept_and_generic_asset(tmp_path: Path) -> None:
    skill_md = "# Demo\n"
    stream = io.BytesIO()
    with zipfile.ZipFile(stream, "w") as archive:
        archive.writestr("SKILL.md", skill_md)
        archive.writestr("scripts/run.ts", "console.log('ok')\n")
    pack = validate_skill_pack(
        skill_name="demo", version="1.0.0", skill_md=skill_md, zip_bytes=stream.getvalue()
    )
    write_skill_pack_version(
        tmp_path, pack, accepted_by="curator", source_proposal_id="12345678-abcd"
    )

    changed = migrate_legacy_skill_packs(tmp_path)

    assert "/skills/demo.md" in changed
    concept = read_bundle_entry(tmp_path, "/skills/demo.md").document
    assert concept.body == skill_md.strip()
    assert "skill" in concept.frontmatter.tags
    assert list_asset_versions(tmp_path, concept.frontmatter.id, "skill") == ("1.0.0",)
    metadata = load_asset_metadata(tmp_path, concept.frontmatter.id, "skill", "1.0.0")
    assert metadata["concept_path"] == "/skills/demo.md"
    assert not (tmp_path / "skills/.versions/demo/1.0.0.zip").exists()
