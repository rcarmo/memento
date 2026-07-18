from __future__ import annotations

import hashlib
import io
import json
import sqlite3
import zipfile
from collections.abc import Generator
from pathlib import Path

import pytest

from memento.control.db import connect_control_db, migrate_control_db
from memento.control.proposals import (
    ProposalAssetInput,
    ProposalStatus,
    create_proposal,
    get_proposal,
    get_proposal_asset,
    list_proposal_assets,
)
from memento.skill_packs import ValidatedSkillPack, validate_skill_pack


@pytest.fixture()
def connection(tmp_path: Path) -> Generator[sqlite3.Connection, None, None]:
    value = connect_control_db(tmp_path / "control.sqlite")
    migrate_control_db(value)
    try:
        yield value
    finally:
        value.close()


def test_create_proposal_stores_assets_atomically(connection: sqlite3.Connection) -> None:
    manifest = {"entries": [{"path": "SKILL.md", "size": 7}], "sha256": "m" * 64}
    created = create_proposal(
        connection,
        proposal_id="proposal-1",
        author_principal="author",
        client_instance_id="client-1",
        base_revision="abc123",
        intent="attach_asset_pack",
        rationale="share a bundle",
        patch={"attach_asset_pack": {"asset_id": "asset-1"}},
        assets=(
            ProposalAssetInput(
                asset_id="asset-1",
                concept_path="/skills/demo.md",
                asset_kind="skill_pack",
                version="1.0.0",
                media_type="application/zip",
                sha256="a" * 64,
                blob_bytes=b"zip-bytes",
                manifest_json=json.dumps(manifest, sort_keys=True),
            ),
            ProposalAssetInput(
                asset_id="asset-2",
                concept_path="/skills/demo.md",
                asset_kind="image",
                version="1",
                media_type="image/png",
                sha256="b" * 64,
                blob_bytes=b"png-bytes",
                manifest_json=json.dumps({"width": 1, "height": 1}, sort_keys=True),
            ),
        ),
    )

    assert created.status is ProposalStatus.SUBMITTED
    fetched = get_proposal_asset(connection, "proposal-1", "asset-1")
    assert fetched.blob_bytes == b"zip-bytes"
    assert fetched.manifest == manifest
    assert list_proposal_assets(connection, proposal_id="proposal-1") == (
        fetched,
        get_proposal_asset(connection, "proposal-1", "asset-2"),
    )
    assert list_proposal_assets(connection, concept_path="/skills/demo.md") == (
        fetched,
        get_proposal_asset(connection, "proposal-1", "asset-2"),
    )

    with connection:
        connection.execute("DELETE FROM proposals WHERE proposal_id = ?", ("proposal-1",))
    assert list_proposal_assets(connection, proposal_id="proposal-1") == ()


def test_create_proposal_rolls_back_when_asset_insert_fails(connection: sqlite3.Connection) -> None:
    asset = ProposalAssetInput(
        asset_id="duplicate",
        concept_path="/skills/demo.md",
        asset_kind="skill_pack",
        version="1.0.0",
        media_type="application/zip",
        sha256="c" * 64,
        blob_bytes=b"zip-bytes",
        manifest_json=json.dumps({"entries": []}, sort_keys=True),
    )

    with pytest.raises(sqlite3.IntegrityError):
        create_proposal(
            connection,
            proposal_id="proposal-rollback",
            author_principal="author",
            client_instance_id=None,
            base_revision="abc123",
            intent="attach_asset_pack",
            rationale=None,
            patch={"attach_asset_pack": {"asset_id": "duplicate"}},
            assets=(asset, asset),
        )

    with pytest.raises(KeyError):
        get_proposal(connection, "proposal-rollback")
    assert list_proposal_assets(connection, proposal_id="proposal-rollback") == ()


def test_migrate_v5_skill_pack_proposals_into_generic_assets(tmp_path: Path) -> None:
    path = tmp_path / "control-v5.sqlite"
    connection = connect_control_db(path)
    try:
        _create_v5_schema(connection)
        pack = _validated_pack()
        created_at = "2026-07-17T12:00:00Z"
        updated_at = "2026-07-17T12:30:00Z"
        expires_at = "2026-08-16T12:00:00Z"
        with connection:
            connection.execute(
                "INSERT INTO service_state(key, value, updated_at) VALUES('schema_version', '5', ?)",
                (created_at,),
            )
            connection.execute(
                """
                INSERT INTO skill_pack_proposals(
                    proposal_id, author_principal, base_revision, skill_name, version,
                    rationale, skill_md, zip_sha256, zip_bytes, manifest_json,
                    status, reviewed_by, review_comment, applied_operation_id,
                    applied_revision, created_at, updated_at, expires_at
                ) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
                """,
                (
                    "skill-proposal-1",
                    "author",
                    "base-rev",
                    pack.skill_name,
                    pack.version,
                    "share it",
                    pack.skill_md,
                    pack.manifest.sha256,
                    pack.zip_bytes,
                    pack.manifest.model_dump_json(),
                    "approved",
                    "curator",
                    "looks good",
                    None,
                    None,
                    created_at,
                    updated_at,
                    expires_at,
                ),
            )

        migrate_control_db(connection)

        proposal = get_proposal(connection, "skill-proposal-1")
        assert proposal.intent == "Attach skill asset demo 1.0.0"
        assert proposal.status is ProposalStatus.APPROVED
        assert proposal.reviewed_by == "curator"
        assert proposal.review_comment == "looks good"
        assert proposal.created_at == created_at
        assert proposal.updated_at == updated_at
        assert proposal.expires_at == expires_at
        concept_path = f"/skills/{pack.skill_name}.md"
        asset_id = f"skill-pack:{pack.skill_name}:{pack.version}"
        expected_patch = {
            "changes": [
                {
                    "kind": "create",
                    "path": concept_path,
                    "concept_type": "concept",
                    "title": "Demo",
                    "body": pack.skill_md,
                    "description": "Versioned agent skill demo.",
                    "tags": ["skill"],
                    "aliases": [],
                },
                {
                    "kind": "attach_asset_pack",
                    "path": concept_path,
                    "asset_id": asset_id,
                    "asset_kind": "skill",
                    "zip_sha256": pack.manifest.sha256,
                    "version": pack.version,
                    "manifest": pack.manifest.model_dump(mode="json"),
                },
            ]
        }
        assert proposal.patch == expected_patch
        assert (
            proposal.patch_hash
            == hashlib.sha256(
                json.dumps(expected_patch, sort_keys=True).encode("utf-8")
            ).hexdigest()
        )

        asset = get_proposal_asset(
            connection,
            "skill-proposal-1",
            asset_id,
        )
        assert asset.concept_path == f"/skills/{pack.skill_name}.md"
        assert asset.asset_kind == "skill"
        assert asset.version == pack.version
        assert asset.media_type == "application/zip"
        assert asset.sha256 == pack.manifest.sha256
        assert asset.blob_bytes == pack.zip_bytes
        assert asset.manifest == pack.manifest.model_dump(mode="json")
        assert asset.created_at == created_at

        schema_version = connection.execute(
            "SELECT value FROM service_state WHERE key = 'schema_version'"
        ).fetchone()[0]
        assert schema_version == "6"
    finally:
        connection.close()


def test_migrate_v5_skips_unfeasible_rows_when_proposal_id_is_taken(tmp_path: Path) -> None:
    path = tmp_path / "control-conflict.sqlite"
    connection = connect_control_db(path)
    try:
        _create_v5_schema(connection)
        pack = _validated_pack()
        patch = {"changes": [{"path": "/skills/demo.md"}]}
        patch_json = json.dumps(patch, sort_keys=True)
        with connection:
            connection.execute(
                "INSERT INTO service_state(key, value, updated_at) VALUES('schema_version', '5', '2026-07-17T00:00:00Z')"
            )
            connection.execute(
                """
                INSERT INTO proposals(
                    proposal_id, author_principal, client_instance_id, base_revision,
                    intent, rationale, patch_json, patch_hash, status,
                    created_at, updated_at, expires_at
                ) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
                """,
                (
                    "shared-id",
                    "existing-author",
                    None,
                    "base-rev",
                    "memory_patch",
                    None,
                    patch_json,
                    hashlib.sha256(patch_json.encode("utf-8")).hexdigest(),
                    "submitted",
                    "2026-07-17T00:00:00Z",
                    "2026-07-17T00:00:00Z",
                    None,
                ),
            )
            connection.execute(
                """
                INSERT INTO skill_pack_proposals(
                    proposal_id, author_principal, base_revision, skill_name, version,
                    rationale, skill_md, zip_sha256, zip_bytes, manifest_json,
                    status, reviewed_by, review_comment, applied_operation_id,
                    applied_revision, created_at, updated_at, expires_at
                ) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
                """,
                (
                    "shared-id",
                    "author",
                    "base-rev",
                    pack.skill_name,
                    pack.version,
                    None,
                    pack.skill_md,
                    pack.manifest.sha256,
                    pack.zip_bytes,
                    pack.manifest.model_dump_json(),
                    "submitted",
                    None,
                    None,
                    None,
                    None,
                    "2026-07-17T01:00:00Z",
                    "2026-07-17T01:00:00Z",
                    None,
                ),
            )

        migrate_control_db(connection)

        proposal = get_proposal(connection, "shared-id")
        assert proposal.intent == "memory_patch"
        assert list_proposal_assets(connection, proposal_id="shared-id") == ()
    finally:
        connection.close()


def _validated_pack() -> ValidatedSkillPack:
    stream = io.BytesIO()
    with zipfile.ZipFile(stream, "w") as archive:
        archive.writestr("SKILL.md", "# Demo\n")
        archive.writestr("images/icon.txt", "icon")
    return validate_skill_pack(
        skill_name="demo",
        version="1.0.0",
        skill_md="# Demo\n",
        zip_bytes=stream.getvalue(),
    )


def _create_v5_schema(connection: sqlite3.Connection) -> None:
    with connection:
        connection.execute(
            "CREATE TABLE service_state (key TEXT PRIMARY KEY, value TEXT NOT NULL, updated_at TEXT NOT NULL)"
        )
        connection.execute("CREATE TABLE operations (op_id TEXT PRIMARY KEY)")
        connection.execute(
            """
            CREATE TABLE proposals (
                proposal_id TEXT PRIMARY KEY,
                author_principal TEXT NOT NULL,
                client_instance_id TEXT,
                base_revision TEXT NOT NULL,
                intent TEXT NOT NULL,
                rationale TEXT,
                patch_json TEXT NOT NULL,
                patch_hash TEXT NOT NULL,
                status TEXT NOT NULL,
                reviewed_by TEXT,
                review_comment TEXT,
                applied_operation_id TEXT,
                applied_revision TEXT,
                created_at TEXT NOT NULL,
                updated_at TEXT NOT NULL,
                expires_at TEXT,
                FOREIGN KEY(applied_operation_id) REFERENCES operations(op_id)
            )
            """
        )
        connection.execute(
            """
            CREATE TABLE skill_pack_proposals (
                proposal_id TEXT PRIMARY KEY,
                author_principal TEXT NOT NULL,
                base_revision TEXT NOT NULL,
                skill_name TEXT NOT NULL,
                version TEXT NOT NULL,
                rationale TEXT,
                skill_md TEXT NOT NULL,
                zip_sha256 TEXT NOT NULL,
                zip_bytes BLOB NOT NULL,
                manifest_json TEXT NOT NULL,
                status TEXT NOT NULL,
                reviewed_by TEXT,
                review_comment TEXT,
                applied_operation_id TEXT,
                applied_revision TEXT,
                created_at TEXT NOT NULL,
                updated_at TEXT NOT NULL,
                expires_at TEXT,
                FOREIGN KEY(applied_operation_id) REFERENCES operations(op_id)
            )
            """
        )
