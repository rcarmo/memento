from __future__ import annotations

import io
import sqlite3
import zipfile
from collections.abc import Generator
from pathlib import Path

import pytest

from memento.control.db import connect_control_db, migrate_control_db
from memento.control.skill_pack_proposals import (
    SkillPackProposalStatus,
    create_skill_pack_proposal,
    get_skill_pack_proposal,
    list_skill_pack_proposals,
    set_skill_pack_proposal_status,
)
from memento.skill_packs import ValidatedSkillPack, validate_skill_pack


@pytest.fixture()
def connection(tmp_path: Path) -> Generator[sqlite3.Connection, None, None]:
    value = connect_control_db(tmp_path / "control.sqlite")
    migrate_control_db(value)
    yield value
    value.close()


def pack() -> ValidatedSkillPack:
    stream = io.BytesIO()
    with zipfile.ZipFile(stream, "w") as archive:
        archive.writestr("SKILL.md", "# Demo\n")
    return validate_skill_pack(
        skill_name="demo", version="1.0.0", skill_md="# Demo\n", zip_bytes=stream.getvalue()
    )


def test_skill_pack_proposal_lifecycle(connection: sqlite3.Connection) -> None:
    validated = pack()
    created = create_skill_pack_proposal(
        connection,
        author_principal="author",
        base_revision="abc",
        skill_name=validated.skill_name,
        version=validated.version,
        rationale="share it",
        skill_md=validated.skill_md,
        zip_bytes=validated.zip_bytes,
        manifest=validated.manifest,
    )
    assert created.status is SkillPackProposalStatus.SUBMITTED
    assert created.zip_bytes == validated.zip_bytes
    assert list_skill_pack_proposals(connection, status="submitted") == (created,)

    approved = set_skill_pack_proposal_status(
        connection,
        created.proposal_id,
        status=SkillPackProposalStatus.APPROVED,
        reviewed_by="curator",
        review_comment="looks good",
    )
    assert approved.reviewed_by == "curator"
    assert (
        get_skill_pack_proposal(connection, created.proposal_id).status
        is SkillPackProposalStatus.APPROVED
    )


def test_duplicate_active_version_is_rejected(connection: sqlite3.Connection) -> None:
    validated = pack()

    def create() -> object:
        return create_skill_pack_proposal(
            connection,
            author_principal="author",
            base_revision="abc",
            skill_name=validated.skill_name,
            version=validated.version,
            rationale=None,
            skill_md=validated.skill_md,
            zip_bytes=validated.zip_bytes,
            manifest=validated.manifest,
        )

    create()
    with pytest.raises(ValueError, match="already exists"):
        create()
