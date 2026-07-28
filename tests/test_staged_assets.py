from __future__ import annotations

import io
import json
import sqlite3
import zipfile
from collections.abc import Generator
from pathlib import Path

import pytest

from memento.config import Principal
from memento.control.db import connect_control_db, migrate_control_db
from memento.staged_assets import StagedAssetError, StagedAssetStore
from memento.staging_http import AssetStagingHTTPHandler


def zip_bytes(text: str = "hello") -> bytes:
    output = io.BytesIO()
    with zipfile.ZipFile(output, "w") as archive:
        archive.writestr("README.txt", text)
    return output.getvalue()


@pytest.fixture()
def connection(tmp_path: Path) -> Generator[sqlite3.Connection, None, None]:
    value = connect_control_db(tmp_path / "control.sqlite")
    migrate_control_db(value)
    yield value
    value.close()


def test_staging_store_is_idempotent_owned_and_consumable(connection: sqlite3.Connection) -> None:
    store = StagedAssetStore(connection)
    first, replayed = store.put(
        principal="flint",
        idempotency_key="stage-1",
        asset_kind="templates",
        version="1.0.0",
        zip_bytes=zip_bytes(),
    )
    assert replayed is False
    replay, replayed = store.put(
        principal="flint",
        idempotency_key="stage-1",
        asset_kind="templates",
        version="1.0.0",
        zip_bytes=zip_bytes(),
    )
    assert replayed is True
    assert replay.staged_asset_id == first.staged_asset_id
    with pytest.raises(StagedAssetError, match="different asset"):
        store.put(
            principal="flint",
            idempotency_key="stage-1",
            asset_kind="templates",
            version="1.0.0",
            zip_bytes=zip_bytes("changed"),
        )
    with pytest.raises(StagedAssetError, match="not found"):
        store.get(principal="other", staged_asset_id=first.staged_asset_id)
    with connection:
        connection.execute(
            """
            INSERT INTO proposals(
                proposal_id,author_principal,base_revision,intent,patch_json,patch_hash,status,
                created_at,updated_at
            ) VALUES('proposal-1','flint','rev','test','{}','hash','submitted','now','now')
            """
        )
    store.consume(
        principal="flint", staged_asset_ids=(first.staged_asset_id,), proposal_id="proposal-1"
    )
    consumed = store.get(principal="flint", staged_asset_id=first.staged_asset_id)
    assert consumed.state == "consumed"
    assert consumed.proposal_id == "proposal-1"
    assert consumed.blob_bytes == b""
    with pytest.raises(StagedAssetError, match="not ready"):
        store.get(principal="flint", staged_asset_id=first.staged_asset_id, require_ready=True)


def test_staging_expiry_removes_blob(connection: sqlite3.Connection) -> None:
    store = StagedAssetStore(connection)
    staged, _ = store.put(
        principal="flint",
        idempotency_key="stage-expire",
        asset_kind="templates",
        version="1.0.0",
        zip_bytes=zip_bytes(),
    )
    with connection:
        connection.execute(
            "UPDATE staged_assets SET expires_at='2000-01-01T00:00:00Z' WHERE staged_asset_id=?",
            (staged.staged_asset_id,),
        )
    assert store.expire() == 1
    expired = store.get(principal="flint", staged_asset_id=staged.staged_asset_id)
    assert expired.state == "expired"
    assert expired.blob_bytes == b""


def test_proposal_and_stage_consumption_can_share_one_transaction(
    connection: sqlite3.Connection,
) -> None:
    from memento.control.proposals import create_proposal

    store = StagedAssetStore(connection)
    with pytest.raises(StagedAssetError, match="could not be consumed"), connection:
        create_proposal(
            connection,
            proposal_id="rolled-back",
            author_principal="flint",
            client_instance_id=None,
            base_revision="rev",
            intent="test",
            rationale=None,
            patch={"changes": []},
            manage_transaction=False,
        )
        store.consume(
            principal="flint",
            staged_asset_ids=("missing",),
            proposal_id="rolled-back",
            manage_transaction=False,
        )
    assert (
        connection.execute(
            "SELECT proposal_id FROM proposals WHERE proposal_id='rolled-back'"
        ).fetchone()
        is None
    )


def test_staging_http_auth_validation_replay_and_status(connection: sqlite3.Connection) -> None:
    proposer = Principal(name="flint", roles=("reader", "proposer"))
    handler = AssetStagingHTTPHandler(
        StagedAssetStore(connection),
        lambda headers: proposer if headers.get("authorization") == "Bearer token" else None,
    )
    unauthorized = handler.handle(
        method="POST",
        path="/assets/staging",
        headers={"content-type": "application/zip"},
        body=zip_bytes(),
    )
    assert unauthorized is not None and unauthorized.status == 401
    created = handler.handle(
        method="POST",
        path="/assets/staging",
        headers={
            "authorization": "Bearer token",
            "content-type": "application/zip",
            "idempotency-key": "http-stage-1",
            "x-memento-asset-kind": "templates",
            "x-memento-asset-version": "1.0.0",
        },
        body=zip_bytes(),
    )
    assert created is not None and created.status == 201
    payload = json.loads(created.body)
    assert payload["state"] == "ready"
    assert payload["replayed"] is False
    replay = handler.handle(
        method="POST",
        path="/assets/staging",
        headers={
            "authorization": "Bearer token",
            "content-type": "application/zip",
            "idempotency-key": "http-stage-1",
            "x-memento-asset-kind": "templates",
            "x-memento-asset-version": "1.0.0",
        },
        body=zip_bytes(),
    )
    assert replay is not None and replay.status == 200
    assert json.loads(replay.body)["replayed"] is True
    status = handler.handle(
        method="GET",
        path=f"/assets/staging/{payload['staged_asset_id']}",
        headers={"authorization": "Bearer token"},
        body=b"",
    )
    assert status is not None and status.status == 200
    assert json.loads(status.body)["sha256"] == payload["sha256"]


def test_staging_metadata_headers_are_required(connection: sqlite3.Connection) -> None:
    proposer = Principal(name="flint", roles=("reader", "proposer"))
    handler = AssetStagingHTTPHandler(StagedAssetStore(connection), lambda _headers: proposer)
    response = handler.handle(
        method="POST",
        path="/assets/staging",
        headers={
            "content-type": "application/zip",
            "idempotency-key": "missing-metadata",
        },
        body=zip_bytes(),
    )
    assert response is not None and response.status == 400
    assert "stable semantic version" in json.loads(response.body)["error"]
