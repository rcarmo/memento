Feature: repository/assets and skills

  The scenarios capture Python Memento behavior at 7f29e8b003557f0105f47ed353b7f65a33619456.
  Rules retain source-module traceability while features group related user behavior.

  Rule: Behavior captured from test_asset_migration.py

    @py-7b1533c71498 @python_test_migrate_legacy_skill_pack_to_concept_and_generic_asset @go_TestMigrateLegacySkillPacks
    Scenario: test_migrate_legacy_skill_pack_to_concept_and_generic_asset
      Given the pinned Python reference fixtures and controlled inputs
      When archive.writestr("SKILL.md", skill_md); validate_skill_pack( skill_name="demo", version="1.0.0", skill_md=skill_md, zip_bytes=stream.getvalue() ); write_skill_pack_version( tmp_path, pack, accepted_by="curator", source_proposal_id="12345678-abcd" ).
      Then '/skills/demo.md' in changed; concept.body == skill_md.strip(); 'skill' in concept.frontmatter.tags; list_asset_versions(tmp_path, concept.frontmatter.id, 'skill') == ('1.0.0',); metadata['concept_path'] == '/skills/demo.md'; not (tmp_path / 'skills/.versions/demo/1.0.0.zip').exists().

  Rule: Behavior captured from test_asset_pack_repository.py

    @py-4c11fcb34d53 @python_test_git_stages_generic_asset_as_an_ordinary_blob @go_TestAcceptedAssetReference
    Scenario: test_git_stages_generic_asset_as_an_ordinary_blob
      Given the pinned Python reference fixtures and controlled inputs
      When root.mkdir(); subprocess.run(["git", "init", "-b", "main", str(root)], check=True, capture_output=True); pack("1.0.0").
      Then staged == item.zip_bytes; not (root / '.gitattributes').exists().

    @py-08aea3e5cd13 @python_test_immutable_asset_version @go_TestAcceptedAssetReference
    Scenario: test_immutable_asset_version
      Given the pinned Python reference fixtures and controlled inputs
      When pack("1.0.0"); write(). expects write() raises ValueError matching "already exists".
      Then the result, state transitions, and error boundary match the captured behavior

    @py-e59813943efe @python_test_write_resolve_and_retention @go_TestAcceptedAssetReference
    Scenario: test_write_resolve_and_retention
      Given the pinned Python reference fixtures and controlled inputs
      When pack("1.9.0"); write_asset_version( tmp_path, concept_id="12345678-abcd-1234-abcd-123456789abc", concept_path="/projects/demo.md", asset_kind="templates",…; load_asset_metadata( tmp_path, "12345678-abcd-1234-abcd-123456789abc", "templates", "1.10.0" ).
      Then resolve_asset_version(tmp_path, '12345678-abcd-1234-abcd-123456789abc', 'templates', None) == '1.10.0'; list_asset_kinds(tmp_path, '12345678-abcd-1234-abcd-123456789abc') == ('templates',); list_asset_versions(tmp_path, '12345678-abcd-1234-abcd-123456789abc', 'templates') == ('1.9.0', '1.10.0'); metadata['concept_path'] == '/projects/demo.md'; metadata['created_at'] == '2026-08-26T04:30:00Z'; not (tmp_path / '.gitattributes').exis

  Rule: Behavior captured from test_asset_retrieval.py

    @py-f16d61b416d7 @python_test_file_digest_checks_bytes_outside_requested_slice @go_TestAssetRetrievalReference
    Scenario: test_file_digest_checks_bytes_outside_requested_slice
      Given the pinned Python reference fixtures and controlled inputs
      When expects verified_slice( io.BytesIO(b"prefixBAD"), total=9, digest=hashlib.sha256(b"prefixOK!").hexdigest(), offset=0, limit=6, ) raises AssetReadError matching "digest".
      Then the result, state transitions, and error boundary match the captured behavior

    @py-4a2b121af06b @python_test_file_reads_fail_closed_on_zip_manifest_disagreement @go_TestAssetRetrievalReference
    Scenario: test_file_reads_fail_closed_on_zip_manifest_disagreement
      Given the pinned Python reference fixtures and controlled inputs
      When archive.writestr(info, b"declared"); stream.getvalue(); manifest(raw). expects read_pack_file(io.BytesIO(raw), data, file_path="file.txt", offset=0, limit=4) raises AssetReadError.
      Then the result, state transitions, and error boundary match the captured behavior

    @py-235911583089 @python_test_invalid_ranges @go_TestAssetRetrievalReference
    Scenario: test_invalid_ranges
      Given the pinned Python reference fixtures and controlled inputs
      When expects verified_slice( io.BytesIO(b"abc"), total=3, digest=hashlib.sha256(b"abc").hexdigest(), offset=offset, limit=limit, ) raises AssetReadError.
      Then the result, state transitions, and error boundary match the captured behavior

    @py-c00694e7e18c @python_test_manifest_rejects_duplicate_and_noncanonical_entries @go_TestAssetRetrievalReference
    Scenario: test_manifest_rejects_duplicate_and_noncanonical_entries
      Given the pinned Python reference fixtures and controlled inputs
      When manifest(b"zip").model_dump(mode="json"). expects checked_manifest(data, data["sha256"]) raises AssetReadError; checked_manifest(data, data["sha256"]) raises AssetReadError.
      Then the result, state transitions, and error boundary match the captured behavior

  Rule: Behavior captured from test_skill_import.py

    @py-1171a0a91e4e @python_test_import_cli @go_TestImportSkillPack
    Scenario: test_import_cli
      Given the pinned Python reference fixtures and controlled inputs
      When skill_md.write_text("# Demo\n"); zip_path.write_bytes(pack("# Demo\n")); workspace.mkdir().
      Then main(['--workspace', str(workspace), '--name', 'demo', '--version', '1.0.0', '--skill-md', str(skill_md), '--zip', str(zip_path)]) == 0; capsys.readouterr().out.strip() ends with '.pi/skills/demo'.

    @py-5bcaeca1e7d6 @python_test_import_skill_pack_fails_if_destination_exists @go_TestImportSkillPack
    Scenario: test_import_skill_pack_fails_if_destination_exists
      Given the pinned Python reference fixtures and controlled inputs
      When destination.mkdir(parents=True); marker.write_text("keep").
      Then marker.read_text() == 'keep'. expects import_skill_pack( workspace=tmp_path, skill_name="demo", version="1.0.0", skill_md="# Demo\n", zip_bytes=pack("# Demo\n"), ) raises SkillImportConflictError.

    @py-1b1215c04e74 @python_test_import_skill_pack_leaves_no_partial_directory_on_validation_failure @go_TestImportSkillPack
    Scenario: test_import_skill_pack_leaves_no_partial_directory_on_validation_failure
      Given the pinned Python reference fixtures and controlled inputs
      When checks not (tmp_path / '.pi/skills/demo').exists(). expects import_skill_pack( workspace=tmp_path, skill_name="demo", version="1.0.0", skill_md="# Different\n", zip_bytes=pack("# Demo\n"), ) raises ValueError.
      Then the result, state transitions, and error boundary match the captured behavior

    @py-2a2f338f60d2 @python_test_import_skill_pack_rejects_symlinked_workspace_parents @go_TestImportSkillPack
    Scenario: test_import_skill_pack_rejects_symlinked_workspace_parents
      Given the pinned Python reference fixtures and controlled inputs
      When outside.mkdir(); target.parent.mkdir(parents=True, exist_ok=True); target.symlink_to(outside, target_is_directory=True).
      Then not (outside / 'demo').exists(). expects import_skill_pack( workspace=tmp_path, skill_name="demo", version="1.0.0", skill_md="# Demo\n", zip_bytes=pack("# Demo\n"), ) raises ValueError matching "symlink".

    @py-d180d3ff66b7 @python_test_import_skill_pack_writes_complete_tree_non_executable @go_TestImportSkillPack
    Scenario: test_import_skill_pack_writes_complete_tree_non_executable
      Given the pinned Python reference fixtures and controlled inputs
      When import_skill_pack( workspace=tmp_path, skill_name="demo", version="1.0.0", skill_md=skill_md, zip_bytes=pack(skill_md), ); os.stat(destination / "scripts/run.sh").
      Then destination == tmp_path / '.pi/skills/demo'; (destination / 'SKILL.md').read_text() == skill_md; (destination / 'assets/icon.png').read_bytes() starts with b'\x89PNG'; stat.S_IMODE(mode) == 420.

  Rule: Behavior captured from test_skill_packs.py

    @py-29ed13f79582 @python_test_validate_skill_pack_accepts_non_native_binary_payloads @go_TestMigrateLegacySkillPacks
    Scenario: test_validate_skill_pack_accepts_non_native_binary_payloads
      Given the pinned Python reference fixtures and controlled inputs
      When _pack( "# ok\n", ("docs/report.pdf", b"%PDF-1.7\nrest"), ("images/logo.png", b"\x89PNG\r\n\x1a\nrest"), ); validate_skill_pack( skill_name="good-name", version="1.2.3", skill_md="# ok\n", zip_bytes=zip_bytes, ).
      Then validated.manifest.file_count == 3.

    @py-9930a0922467 @python_test_validate_skill_pack_accepts_valid_archive_and_builds_manifest @go_TestMigrateLegacySkillPacks
    Scenario: test_validate_skill_pack_accepts_valid_archive_and_builds_manifest
      Given the pinned Python reference fixtures and controlled inputs
      When _pack( skill_md, ("docs/", None), ("docs/readme.txt", b"hello world\n"), ("scripts/run.sh", b"#!/bin/sh\necho ok\n"), ("images/icon.png", i…; validate_skill_pack( skill_name="search-pack", version="1.2.3", skill_md=skill_md, zip_bytes=zip_bytes, ).
      Then validated.zip_bytes == zip_bytes; validated.manifest.sha256 == hashlib.sha256(zip_bytes).hexdigest(); validated.manifest.file_count == 5; validated.manifest.total_uncompressed_bytes == sum((len(payload) for payload in (skill_md.encode('utf-8'), b'hello world\n', b'#!/bin/sh\necho ok\n', image_bytes, pdf_bytes))); [entry.path for entry in validated.manifest.entries] == [SKILL_MD_PATH, 'docs/readme.txt', 'scripts/run.sh', 'images/icon.p

    @py-422d37d7ac28 @python_test_validate_skill_pack_rejects_duplicate_paths @go_TestMigrateLegacySkillPacks
    Scenario: test_validate_skill_pack_rejects_duplicate_paths
      Given the pinned Python reference fixtures and controlled inputs
      When archive.writestr("dup.txt", b"a"). expects validate_skill_pack( skill_name="good-name", version="1.2.3", skill_md="# ok\n", zip_bytes=buffer.getvalue(), ) raises SkillPackValidationError matching "duplicate path".
      Then the result, state transitions, and error boundary match the captured behavior

    @py-e1be6e5b094f @python_test_validate_skill_pack_rejects_encrypted_entries @go_TestMigrateLegacySkillPacks
    Scenario: test_validate_skill_pack_rejects_encrypted_entries
      Given the pinned Python reference fixtures and controlled inputs
      When bytearray(_pack("# ok\n", ("secret.txt", b"secret"))); zip_bytes.find(b"PK\x03\x04"); zip_bytes.find(b"PK\x01\x02").
      Then local_flag_offset >= 6; central_flag_offset >= 8. expects validate_skill_pack( skill_name="good-name", version="1.2.3", skill_md="# ok\n", zip_bytes=bytes(zip_bytes), ) raises SkillPackValidationError matching "encrypted".

    @py-5cf500afbd66 @python_test_validate_skill_pack_rejects_excessive_compression_ratio @go_TestMigrateLegacySkillPacks
    Scenario: test_validate_skill_pack_rejects_excessive_compression_ratio
      Given the pinned Python reference fixtures and controlled inputs
      When _make_zip([(SKILL_MD_PATH, b"# ok\n"), ("bomb.txt", payload)]). expects validate_skill_pack( skill_name="good-name", version="1.2.3", skill_md="# ok\n", zip_bytes=zip_bytes, ) raises SkillPackValidationError matching "compression ratio".
      Then the result, state transitions, and error boundary match the captured behavior

    @py-594aa61b8bed @python_test_validate_skill_pack_rejects_file_larger_than_limit @go_TestMigrateLegacySkillPacks
    Scenario: test_validate_skill_pack_rejects_file_larger_than_limit
      Given the pinned Python reference fixtures and controlled inputs
      When _pack("# ok\n", ("large.bin", b"x" * (MAX_FILE_BYTES + 1))). expects validate_skill_pack( skill_name="good-name", version="1.2.3", skill_md="# ok\n", zip_bytes=zip_bytes, ) raises SkillPackValidationError matching "maximum uncompressed size".
      Then the result, state transitions, and error boundary match the captured behavior

    @py-1736f77d1746 @python_test_validate_skill_pack_rejects_invalid_skill_name @go_TestMigrateLegacySkillPacks
    Scenario: test_validate_skill_pack_rejects_invalid_skill_name
      Given the pinned Python reference fixtures and controlled inputs
      When expects validate_skill_pack( skill_name="Bad_Name", version="1.2.3", skill_md="# ok\n", zip_bytes=_pack("# ok\n"), ) raises SkillPackValidationError matching "skill_name".
      Then the result, state transitions, and error boundary match the captured behavior

    @py-9ac789451c25 @python_test_validate_skill_pack_rejects_invalid_zip_bytes @go_TestMigrateLegacySkillPacks
    Scenario: test_validate_skill_pack_rejects_invalid_zip_bytes
      Given the pinned Python reference fixtures and controlled inputs
      When expects validate_skill_pack( skill_name="good-name", version="1.2.3", skill_md="# ok\n", zip_bytes=b"not-a-zip", ) raises SkillPackValidationError matching "valid ZIP".
      Then the result, state transitions, and error boundary match the captured behavior

    @py-971404697cb3 @python_test_validate_skill_pack_rejects_native_binaries @go_TestMigrateLegacySkillPacks
    Scenario: test_validate_skill_pack_rejects_native_binaries
      Given the pinned Python reference fixtures and controlled inputs
      When _pack("# ok\n", (path, payload)). expects validate_skill_pack( skill_name="good-name", version="1.2.3", skill_md="# ok\n", zip_bytes=zip_bytes, ) raises SkillPackValidationError matching "message".
      Then the result, state transitions, and error boundary match the captured behavior

    @py-395af1e395f8 @python_test_validate_skill_pack_rejects_nested_archives @go_TestMigrateLegacySkillPacks
    Scenario: test_validate_skill_pack_rejects_nested_archives
      Given the pinned Python reference fixtures and controlled inputs
      When _pack("# ok\n", ("payload.tar.gz", b"not really a tarball")). expects validate_skill_pack( skill_name="good-name", version="1.2.3", skill_md="# ok\n", zip_bytes=zip_bytes, ) raises SkillPackValidationError matching "nested archive".
      Then the result, state transitions, and error boundary match the captured behavior

    @py-d88178b3047f @python_test_validate_skill_pack_rejects_non_stable_semver @go_TestMigrateLegacySkillPacks
    Scenario: test_validate_skill_pack_rejects_non_stable_semver
      Given the pinned Python reference fixtures and controlled inputs
      When expects validate_skill_pack( skill_name="good-name", version=version, skill_md="# ok\n", zip_bytes=_pack("# ok\n"), ) raises SkillPackValidationError matching "semantic version".
      Then the result, state transitions, and error boundary match the captured behavior

    @py-d8810f6ab3c1 @python_test_validate_skill_pack_rejects_raw_zip_larger_than_limit @go_TestMigrateLegacySkillPacks
    Scenario: test_validate_skill_pack_rejects_raw_zip_larger_than_limit
      Given the pinned Python reference fixtures and controlled inputs
      When expects validate_skill_pack( skill_name="good-name", version="1.2.3", skill_md="# ok\n", zip_bytes=b"x" * (MAX_ZIP_BYTES + 1), ) raises SkillPackValidationError matching "encoded size".
      Then the result, state transitions, and error boundary match the captured behavior

    @py-c79249650c73 @python_test_validate_skill_pack_rejects_special_file_modes @go_TestMigrateLegacySkillPacks
    Scenario: test_validate_skill_pack_rejects_special_file_modes
      Given the pinned Python reference fixtures and controlled inputs
      When archive.writestr(info, b"ignored"). expects validate_skill_pack( skill_name="good-name", version="1.2.3", skill_md="# ok\n", zip_bytes=buffer.getvalue(), ) raises SkillPackValidationError matching "special file types".
      Then the result, state transitions, and error boundary match the captured behavior

    @py-282ec979a8d2 @python_test_validate_skill_pack_rejects_symlinks @go_TestMigrateLegacySkillPacks
    Scenario: test_validate_skill_pack_rejects_symlinks
      Given the pinned Python reference fixtures and controlled inputs
      When archive.writestr(info, b"target"). expects validate_skill_pack( skill_name="good-name", version="1.2.3", skill_md="# ok\n", zip_bytes=buffer.getvalue(), ) raises SkillPackValidationError matching "symlinks".
      Then the result, state transitions, and error boundary match the captured behavior

    @py-99ae58c355dc @python_test_validate_skill_pack_rejects_too_many_archive_entries @go_TestMigrateLegacySkillPacks
    Scenario: test_validate_skill_pack_rejects_too_many_archive_entries
      Given the pinned Python reference fixtures and controlled inputs
      When entries.extend((f"dirs/{index}/", None) for index in range(MAX_ARCHIVE_ENTRY_COUNT)); _make_zip(entries). expects validate_skill_pack( skill_name="good-name", version="1.2.3", skill_md="# ok\n", zip_bytes=zip_bytes, ) raises SkillPackValidationError matching "entry count".
      Then the result, state transitions, and error boundary match the captured behavior

    @py-868b7d9f4b66 @python_test_validate_skill_pack_rejects_too_many_files @go_TestMigrateLegacySkillPacks
    Scenario: test_validate_skill_pack_rejects_too_many_files
      Given the pinned Python reference fixtures and controlled inputs
      When entries.extend((f"docs/{index}.txt", b"x") for index in range(MAX_FILE_COUNT)); _make_zip(entries). expects validate_skill_pack( skill_name="good-name", version="1.2.3", skill_md="# ok\n", zip_bytes=zip_bytes, ) raises SkillPackValidationError matching "file count".
      Then the result, state transitions, and error boundary match the captured behavior

    @py-01803d78cc2b @python_test_validate_skill_pack_rejects_total_uncompressed_larger_than_limit @go_TestMigrateLegacySkillPacks
    Scenario: test_validate_skill_pack_rejects_total_uncompressed_larger_than_limit
      Given the pinned Python reference fixtures and controlled inputs
      When _pack("# ok\n", ("a.bin", chunk), ("b.bin", chunk), ("c.bin", b"y")). expects validate_skill_pack( skill_name="good-name", version="1.2.3", skill_md="# ok\n", zip_bytes=zip_bytes, ) raises SkillPackValidationError matching "maximum uncompressed size".
      Then the result, state transitions, and error boundary match the captured behavior

    @py-7561308b42c1 @python_test_validate_skill_pack_rejects_unsafe_paths @go_TestMigrateLegacySkillPacks
    Scenario: test_validate_skill_pack_rejects_unsafe_paths
      Given the pinned Python reference fixtures and controlled inputs
      When _make_zip([(SKILL_MD_PATH, b"# ok\n"), (path, b"x")]). expects validate_skill_pack( skill_name="good-name", version="1.2.3", skill_md="# ok\n", zip_bytes=zip_bytes, ) raises SkillPackValidationError matching "message".
      Then the result, state transitions, and error boundary match the captured behavior

    @py-343ade46fd14 @python_test_validate_skill_pack_requires_exact_skill_md_bytes @go_TestMigrateLegacySkillPacks
    Scenario: test_validate_skill_pack_requires_exact_skill_md_bytes
      Given the pinned Python reference fixtures and controlled inputs
      When _pack("# not-the-same\n"). expects validate_skill_pack( skill_name="good-name", version="1.2.3", skill_md="# expected\n", zip_bytes=zip_bytes, ) raises SkillPackValidationError matching "exactly match".
      Then the result, state transitions, and error boundary match the captured behavior

    @py-2054b452779d @python_test_validate_skill_pack_requires_root_skill_md @go_TestMigrateLegacySkillPacks
    Scenario: test_validate_skill_pack_requires_root_skill_md
      Given the pinned Python reference fixtures and controlled inputs
      When _make_zip([("nested/SKILL.md", b"# nested\n")]). expects validate_skill_pack( skill_name="good-name", version="1.2.3", skill_md="# nested\n", zip_bytes=zip_bytes, ) raises SkillPackValidationError matching "root SKILL.md".
      Then the result, state transitions, and error boundary match the captured behavior

  Rule: Behavior captured from test_staged_assets.py

    @py-41a709f093e8 @python_test_proposal_and_stage_consumption_can_share_one_transaction @go_TestStagingReference @go_TestStagingToolReference
    Scenario: test_proposal_and_stage_consumption_can_share_one_transaction
      Given the pinned Python reference fixtures and controlled inputs
      When checks connection.execute("SELECT proposal_id FROM proposals WHERE proposal_id='rolled-back'").fetchone() is None. expects create_proposal( connection, proposal_id="rolled-back", author_principal="flint", client_instance_id=None, base_revision="rev", intent="tes… raises StagedAssetError matching "could not be consumed".
      Then the result, state transitions, and error boundary match the captured behavior

    @py-bb864db91af6 @python_test_staging_expiry_removes_blob @go_TestStagingReference @go_TestStagingToolReference
    Scenario: test_staging_expiry_removes_blob
      Given the pinned Python reference fixtures and controlled inputs
      When store.put( principal="flint", idempotency_key="stage-expire", asset_kind="templates", version="1.0.0", zip_bytes=zip_bytes(), ); connection.execute( "UPDATE staged_assets SET expires_at='2000-01-01T00:00:00Z' WHERE staged_asset_id=?", (staged.staged_asset_id,), ); store.get(principal="flint", staged_asset_id=staged.staged_asset_id).
      Then store.expire() == 1; expired.state == 'expired'; expired.blob_bytes == b''.

    @py-9b14a9a5ccbc @python_test_staging_http_auth_validation_replay_and_status @go_TestStagingReference @go_TestStagingToolReference
    Scenario: test_staging_http_auth_validation_replay_and_status
      Given the pinned Python reference fixtures and controlled inputs
      When headers.get("authorization"); handler.handle( method="POST", path="/assets/staging", headers={"content-type": "application/zip"}, body=zip_bytes(), ); handler.handle( method="POST", path="/assets/staging", headers={ "authorization": "Bearer token", "content-type": "application/zip", "idemp….
      Then unauthorized is not None and unauthorized.status == 401; created is not None and created.status == 201; payload['state'] == 'ready'; payload['replayed'] is False; replay is not None and replay.status == 200; json.loads(replay.body)['replayed'] is True.

    @py-f208f113048a @python_test_staging_http_ticket_upload_requires_no_bearer @go_TestStagingReference @go_TestStagingToolReference
    Scenario: test_staging_http_ticket_upload_requires_no_bearer
      Given the pinned Python reference fixtures and controlled inputs
      When store.begin_upload( principal="flint", idempotency_key="ticket-http-1", asset_kind="templates", version="1.0.0", ); handler.handle( method="POST", path="/assets/staging/upload", headers={ "content-type": "application/zip", "x-memento-upload-ticket": raw_t….
      Then response is not None and response.status == 201; payload['state'] == 'ready'; store.ticket_status(principal='flint', idempotency_key='ticket-http-1').staged_asset_id == payload['staged_asset_id'].

    @py-82bb4884c532 @python_test_staging_metadata_headers_are_required @go_TestStagingReference @go_TestStagingToolReference
    Scenario: test_staging_metadata_headers_are_required
      Given the pinned Python reference fixtures and controlled inputs
      When handler.handle( method="POST", path="/assets/staging", headers={ "content-type": "application/zip", "idempotency-key": "missing-metadata", ….
      Then response is not None and response.status == 400; 'stable semantic version' in json.loads(response.body)['error'].

    @py-419698579aeb @python_test_staging_store_is_idempotent_owned_and_consumable @go_TestStagingReference @go_TestStagingToolReference
    Scenario: test_staging_store_is_idempotent_owned_and_consumable
      Given the pinned Python reference fixtures and controlled inputs
      When store.put( principal="flint", idempotency_key="stage-1", asset_kind="templates", version="1.0.0", zip_bytes=zip_bytes(), ); connection.execute( """ INSERT INTO proposals( proposal_id,author_principal,base_revision,intent,patch_json,patch_hash,status, created_at,u…; store.consume( principal="flint", staged_asset_ids=(first.staged_asset_id,), proposal_id="proposal-1" ).
      Then replayed is False; replayed is True; replay.staged_asset_id == first.staged_asset_id; consumed.state == 'consumed'; consumed.proposal_id == 'proposal-1'; consumed.blob_bytes == b''. expects store.put( principal="flint", idempotency_key="stage-1", asset_kind="templates", version="1.0.0", zip_bytes=zip_bytes("changed

    @py-3981471230f7 @python_test_upload_ticket_is_one_time_principal_bound_and_reconcilable @go_TestStagingReference @go_TestStagingToolReference
    Scenario: test_upload_ticket_is_one_time_principal_bound_and_reconcilable
      Given the pinned Python reference fixtures and controlled inputs
      When store.begin_upload( principal="flint", idempotency_key="ticket-1", asset_kind="templates", version="1.0.0", ); store.put_with_ticket(raw_token=raw_token, zip_bytes=zip_bytes()).
      Then ticket.state == 'pending'; raw_token starts with 'memento_upload_'; replayed is False; store.ticket_status(principal='flint', idempotency_key='ticket-1').state == 'uploaded'; store.ticket_status(principal='flint', idempotency_key='ticket-1').staged_asset_id == staged.staged_asset_id; replayed is True. expects store.begin_upload( principal="flint", idempotency_key="ticket-1", asset_kind="templates", version="1.0.0", ) raises StagedAssetError matching "already issued"; store.ticket_status(principal="other"
