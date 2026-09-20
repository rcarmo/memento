Feature: mcp/service tools and workflows

  The scenarios capture Python Memento behavior at 7f29e8b003557f0105f47ed353b7f65a33619456.
  Rules retain source-module traceability while features group related user behavior.

  Rule: Behavior captured from test_service_mcp.py

    @py-ef5fe61f75d4 @python_test_accepted_archive_ranges_resume_and_verify_digest @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestCatalogReference
    Scenario: test_accepted_archive_ranges_resume_and_verify_digest
      Given the pinned Python reference fixtures and controlled inputs
      When success_data(service.memory_asset_get(smith, id_or_path=path, asset_kind="document")); success_data( service.memory_asset_get( smith, id_or_path=path, asset_kind="document", version=first["version"], offset=first["next_offset"…; base64.b64decode(first["zip_base64"]).
      Then first['returned_bytes'] == 65536; first['truncated'] is True; 'manifest' not in first; reconstructed == raw; result['next_offset'] is None; result['returned_bytes'] == len(raw) - 65536.

    @py-d824fa8e91d1 @python_test_accepted_asset_symlink_containment @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestCatalogReference
    Scenario: test_accepted_asset_symlink_containment
      Given the pinned Python reference fixtures and controlled inputs
      When success_data( service.memory_asset_get(smith, id_or_path=path, asset_kind="document", view="manifest") ); target.rename(outside); target.symlink_to(outside, target_is_directory=outside.is_dir()).
      Then result.status == 'error'.

    @py-07d799fcd17e @python_test_accepted_file_chunks_match_proposal_reads @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestCatalogReference
    Scenario: test_accepted_file_chunks_match_proposal_reads
      Given the pinned Python reference fixtures and controlled inputs
      When hashlib.sha256(accepted_read_pack[2]).hexdigest(); success_data( service.memory_asset_get( smith, id_or_path=path, asset_kind="document", version="1.0.0", view="file", file_path="notes.txt",…; list_proposal_assets(service._deps.control_connection, proposal_id=proposal).
      Then actual == proposed; actual['encoding'] == encoding; actual['content_sha256'] == hashlib.sha256(chunk).hexdigest(); decoded == chunk.

    @py-b649a725b7cd @python_test_accepted_manifest_does_not_read_zip_and_execute_preserves_entries @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestCatalogReference
    Scenario: test_accepted_manifest_does_not_read_zip_and_execute_preserves_entries
      Given the pinned Python reference fixtures and controlled inputs
      When monkeypatch.setattr(Path, "open", guarded_open); success_data( service.memory_asset_get(smith, id_or_path=path, asset_kind="document", view="manifest") ); success_data( service.memory_execute( smith, plan={ "operations": [ { "op": "asset_get", "save_as": "pack", "args": {"id_or_path": path, "a….
      Then 'zip_base64' not in manifest; manifest['zip_bytes'] == len(raw); manifest['zip_sha256'] == hashlib.sha256(raw).hexdigest(); manifest['file_count'] == 55; len(manifest['manifest']['entries']) == 55; len(result['trace'][0]['data']['manifest']['entries']) == 55.

    @py-7c949b0a46ae @python_test_accepted_metadata_digest_mismatch_and_invalid_file_range @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestCatalogReference
    Scenario: test_accepted_metadata_digest_mismatch_and_invalid_file_range
      Given the pinned Python reference fixtures and controlled inputs
      When success_data( service.memory_asset_get(smith, id_or_path=path, asset_kind="document", view="manifest") ); metadata_path.read_text(); metadata_path.write_text(json.dumps(metadata)).
      Then service.memory_asset_get(smith, id_or_path=path, asset_kind='document', version='1.0.0', expected_sha256=digest, view='file', file_path='notes.txt', offset=6).status == 'error'; service.memory_asset_get(smith, id_or_path=path, asset_kind='document', version='1.0.0', expected_sha256=digest, offset=len(raw) + 1).status == 'error'; response.status == 'error'; 'digest' in response.message.

    @py-b6347816535a @python_test_accepted_reads_detect_corruption_and_pruned_payloads @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestCatalogReference
    Scenario: test_accepted_reads_detect_corruption_and_pruned_payloads
      Given the pinned Python reference fixtures and controlled inputs
      When success_data( service.memory_asset_get(smith, id_or_path=path, asset_kind="document", view="manifest") ); archive.write_bytes(raw[:-1] + bytes([raw[-1] ^ 1])); service.memory_asset_get(smith, id_or_path=path, asset_kind="document", **args).
      Then result.status == 'error' and 'digest' in result.message; service.memory_asset_get(smith, id_or_path=path, asset_kind='document', view=view, file_path='notes.txt' if view == 'file' else None).status == 'error'.

    @py-7defada3eccc @python_test_accepted_version_pinning_trash_and_pruned_versions @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestCatalogReference
    Scenario: test_accepted_version_pinning_trash_and_pruned_versions
      Given the pinned Python reference fixtures and controlled inputs
      When success_data( service.memory_asset_get(smith, id_or_path=path, asset_kind="document", view="manifest") ); archive.writestr("updated.txt", "replacement version"); updated.getvalue().
      Then success_data(service.memory_asset_get(smith, id_or_path=path, asset_kind='document', view='manifest'))['version'] == '2.0.0'; base64.b64decode(pinned['zip_base64']) == raw[65536:]; service.memory_asset_get(smith, id_or_path=path, asset_kind='document', view='manifest', expected_sha256=old['zip_sha256']).status == 'error'; service.memory_asset_get(smith, id_or_path=path, asset_kind='document').status == 'error'; success_data(service.memory_asset_get(smith, id_or_path=trashed, asset_kind='document', vi

    @py-f5a69c7ad35f @python_test_accepted_views_enforce_authorisation_ranges_and_safe_paths @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestCatalogReference
    Scenario: test_accepted_views_enforce_authorisation_ranges_and_safe_paths
      Given the pinned Python reference fixtures and controlled inputs
      When service.memory_asset_get(narrow, **args); service.memory_asset_get( smith, id_or_path=path, asset_kind="document", **override ); service.memory_asset_get( smith, id_or_path=path, asset_kind="document", view="file", file_path=name ).
      Then denied.status == 'error' and denied.error_class == 'forbidden'; service.memory_asset_get(smith, id_or_path=path, asset_kind='document', **override).status == 'error'; service.memory_asset_get(smith, id_or_path=path, asset_kind='document', view='file', file_path=name).status == 'error'; empty['returned_bytes'] == 0 and empty['next_offset'] is None.

    @py-cce57314174e @python_test_archival_impact_filters_hidden_and_revoked_backlinks @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestCatalogReference
    Scenario: test_archival_impact_filters_hidden_and_revoked_backlinks
      Given the pinned Python reference fixtures and controlled inputs
      When get_main_revision(service._deps.repo_paths); success_data( service.memory_propose( smith, intent="scope", base_revision=revision, changes=[{"kind": "trash", "path": "/projects/piclaw.m…; replace( service._policy(smith), read_prefixes=("/projects/",), write_prefixes=("/projects/",) ).
      Then report[0]['inbound_references'] == []; service._archival_impact(policy, [TrashChange(kind='trash', path='/projects/piclaw.md')], revision)[0]['inbound_references'] == []; isinstance(policy, EffectivePolicy).

    @py-d11d4f60571a @python_test_archival_impact_retains_asset_versions_and_scopes_references @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestCatalogReference
    Scenario: test_archival_impact_retains_asset_versions_and_scopes_references
      Given the pinned Python reference fixtures and controlled inputs
      When _skill_zip(body); success_data( service.memory_propose( smith, intent="fixture", base_revision=get_main_revision(service._deps.repo_paths), changes=[ { "kind…; success_data( service.memory_proposal_review( smith, proposal_id=proposal["proposal_id"], decision="approve" ) ).
      Then archived['archival_impact'][0]['assets'][0]['version'] == '1.0.0'; archived['archival_impact'][0]['assets'][0]['action'] == 'retained'; success_data(service.memory_asset_get(smith, id_or_path='/trash/projects/review-asset.md', asset_kind='skill'))['version'] == '1.0.0'.

    @py-44f46c291fa1 @python_test_archival_proposal_rejects_mixed_duplicate_and_stale_requests @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestCatalogReference
    Scenario: test_archival_proposal_rejects_mixed_duplicate_and_stale_requests
      Given the pinned Python reference fixtures and controlled inputs
      When get_main_revision(service._deps.repo_paths); service.memory_propose( smith, intent="invalid", base_revision=revision, changes=changes ); success_data( service.memory_propose(smith, intent="stale", base_revision=revision, changes=[trash]) ).
      Then service.memory_propose(smith, intent='invalid', base_revision=revision, changes=changes).status == 'error'; service.memory_proposal_review(smith, proposal_id=proposed['proposal_id'], decision='approve').status == 'error'; service.memory_propose(smith, intent='old', base_revision=revision, changes=[trash]).status == 'error'.

    @py-0158b4bc1fce @python_test_asset_get_direct_mcp_supports_manifest_view @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestCatalogReference
    Scenario: test_asset_get_direct_mcp_supports_manifest_view
      Given the pinned Python reference fixtures and controlled inputs
      When _server_for(service, service_config); monkeypatch.setattr(server, "_context", lambda: smith); asyncio.run( server.tool_memory_asset_get(accepted_read_pack[0], "document", view="manifest") ).
      Then result['status'] == 'success'; len(result['data']['manifest']['entries']) == 55; 'zip_base64' not in result['data']; {'view', 'offset', 'limit', 'file_path', 'expected_sha256'} <= schema['properties'].keys().

    @py-fabe514fae5c @python_test_asset_limits_are_discoverable @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestCatalogReference
    Scenario: test_asset_limits_are_discoverable
      Given the pinned Python reference fixtures and controlled inputs
      When success_data(service.memory_status(smith)).
      Then limits['max_archive_bytes'] == limits['max_upload_zip_bytes'] == 50 * 1024 * 1024; limits['max_file_bytes'] == 16 * 1024 * 1024; limits['max_chunk_bytes'] == 262144; limits['max_file_count'] == 512.

    @py-ed8dfb0c386d @python_test_asset_metadata_is_execute_only_with_service_execute_parity @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestCatalogReference
    Scenario: test_asset_metadata_is_execute_only_with_service_execute_parity
      Given the pinned Python reference fixtures and controlled inputs
      When _server_for( service, service_config.model_copy(update={"mcp": MCPConfig(tool_surface="compact")}), ); server.discover_tools(); asyncio.run(server.resource_catalog()).
      Then 'memory_asset_metadata' not in tools; any((item['operation'] == 'asset_metadata' for item in catalog['execute_only_operations'])); executed == direct.

    @py-b4ae0623c2d7 @python_test_asset_metadata_prunes_protected_paths_before_parsing_and_pages_batches @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestCatalogReference
    Scenario: test_asset_metadata_prunes_protected_paths_before_parsing_and_pages_batches
      Given the pinned Python reference fixtures and controlled inputs
      When write_concept( repo_paths.current_dir / "skills" / "a.md", concept_id="asset-a-id", concept_type="concept", title="Asset A", description="B…; write_concept( repo_paths.current_dir / "skills" / "b.md", concept_id="asset-b-id", concept_type="concept", title="Asset B", description="B…; malformed.write_text("not valid frontmatter\n", encoding="utf-8").
      Then [item['path'] for item in first['entries']] == ['/skills/a.md']; first['entries'][0]['asset_present'] is False; first['next_cursor'] == '/skills/a.md'; [item['path'] for item in second['entries']] == ['/skills/b.md']; second['next_cursor'] is None; all((not item['path'].startswith('/secret/') for item in visible['entries'])).

    @py-042d41615fd1 @python_test_asset_metadata_returns_generic_versions_files_timestamps_and_skill_parity @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestCatalogReference
    Scenario: test_asset_metadata_returns_generic_versions_files_timestamps_and_skill_parity
      Given the pinned Python reference fixtures and controlled inputs
      When write_concept( repo_paths.current_dir / "skills" / "metadata.md", concept_id=concept_id, concept_type="concept", title="Metadata skill", de…; datetime(2026, 8, 26, 6, 30, tzinfo=UTC); write_asset_version( repo_paths.current_dir, concept_id=concept_id, concept_path="/skills/original-metadata.md", asset_kind="skill", versio….
      Then payload['next_cursor'] is None; len(payload['entries']) == 1; entry['path'] == '/skills/metadata.md'; entry['current_concept_body_sha256'] == hashlib.sha256(skill_body.encode('utf-8')).hexdigest(); entry['current_concept_body_bytes'] == len(skill_body.encode('utf-8')); entry['asset_present'] is True.

    @py-5d055b2783e5 @python_test_asset_metadata_validates_scope_bounds_and_persisted_metadata @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestCatalogReference
    Scenario: test_asset_metadata_validates_scope_bounds_and_persisted_metadata
      Given the pinned Python reference fixtures and controlled inputs
      When service.memory_asset_metadata(flint, **arguments); write_concept( repo_paths.current_dir / "projects" / "malformed-asset.md", concept_id=concept_id, concept_type="concept", title="Malformed …; metadata_dir.mkdir(parents=True).
      Then result.status == 'error'; result.error_class == 'validation_error'; malformed.status == 'error'; malformed.error_class == 'validation_error'; linked.status == 'error'; linked.error_class == 'validation_error'.

    @py-a83350d600e1 @python_test_asset_pack_skill_lifecycle_uses_generic_propose_review_apply_and_get @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestCatalogReference
    Scenario: test_asset_pack_skill_lifecycle_uses_generic_propose_review_apply_and_get
      Given the pinned Python reference fixtures and controlled inputs
      When _skill_zip(skill_md); get_main_revision(repo_paths); service.memory_propose( smith, intent="Share complete skill", base_revision=base_revision, rationale="share complete skill", changes=[ { "k….
      Then proposal['status'] == 'submitted'; 'zip_base64' not in proposal['changes'][1]; proposal['changes'][1]['asset_kind'] == 'skill'; proposal['changes'][1]['version'] == '1.0.0'; proposal['changes'][1]['zip_sha256'] == proposal['changes'][1]['manifest']['sha256']; {entry['path'] for entry in proposal['changes'][1]['manifest']['entries']} == {'SKILL.md', 'scripts/run.ts'}.

    @py-bed0eb54c9e1 @python_test_asset_pack_tool_discovery_and_catalog_schemas @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestCatalogReference
    Scenario: test_asset_pack_tool_discovery_and_catalog_schemas
      Given the pinned Python reference fixtures and controlled inputs
      When _server_for( service, service_config.model_copy(update={"mcp": MCPConfig(tool_surface="standard")}), ); standard_server.discover_tools(); asyncio.run(standard_server.resource_template_catalog("propose")).
      Then 'memory_asset_get' in standard_tools; 'memory_asset_prune' in standard_tools; 'memory_execute' not in standard_tools; standard_tools['memory_asset_get']['annotations'] == {'roles': ['reader'], 'operation': 'asset_get'}; changes_schema['type'] == 'array'; 'anyOf' in changes_schema['items'].

    @py-a7df6d6eb4bd @python_test_audit_graph_diagnostics_are_role_scoped_paginated_and_stale_safe @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestCatalogReference
    Scenario: test_audit_graph_diagnostics_are_role_scoped_paginated_and_stale_safe
      Given the pinned Python reference fixtures and controlled inputs
      When success_data(service.memory_audit(smith, limit=1)); success_data(service.memory_audit(smith, limit=1, cursor=graph["next_cursor"])); service.memory_audit( smith, severity="warning", limit=1, cursor=graph["next_cursor"], ).
      Then graph['available'] is True; len(graph['diagnostics']) == 1; graph['next_cursor'] is not None; graph['diagnostics'][0]['repair_guidance']['read_only'] is True; second['graph_diagnostics']['diagnostics']; mismatched_cursor.status == 'error'. expects authorize_path(protected_scope, "/secret/other.md", action="read") raises AuthorizationError.

    @py-b3a708f2bf63 @python_test_audit_skips_protected_content_and_broken_targets @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestCatalogReference
    Scenario: test_audit_skips_protected_content_and_broken_targets
      Given the pinned Python reference fixtures and controlled inputs
      When visible.write_text( visible.read_text(encoding="utf-8") + "\n[Visible missing](/projects/missing.md)\n" + "[Protected missing](/secret/miss…; hidden.write_text("not valid frontmatter\n", encoding="utf-8"); success_data(service.memory_list(flint)).
      Then '/secret/malformed.md' not in [item['path'] for item in listed['entries']]; messages == ['broken link to /projects/missing.md']; all(('/secret/' not in message for message in messages)).

    @py-7b8fe16b592e @python_test_auth_visibility_and_standard_envelopes @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestCatalogReference
    Scenario: test_auth_visibility_and_standard_envelopes
      Given the pinned Python reference fixtures and controlled inputs
      When service.memory_search(flint, query="Ghost"); service.memory_read(flint, id_or_path="/secret/ghost.md"); success_data(service.memory_list(flint)).
      Then search.status == 'success'; success_data(search)['results'] == []; search.repo_revision == search.index_revision; read_hidden.status == 'error'; read_hidden.error_class == 'forbidden'; '/secret/ghost.md' not in [item['path'] for item in listed['entries']].

    @py-0c04076f6f57 @python_test_commit_operation_reconciliation_is_principal_scoped_and_actionable @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestCatalogReference
    Scenario: test_commit_operation_reconciliation_is_principal_scoped_and_actionable
      Given the pinned Python reference fixtures and controlled inputs
      When service.memory_create( smith, path="/projects/reconciled.md", concept_type="project", title="Reconciled", body="# Reconciled\n", expected_r…; success_data( service.memory_operation_get(smith, idempotency_key="reconcile-created") ); service._deps.config.model_copy( update={ "authorization": service._deps.config.authorization.model_copy( update={ "principals": { **servic….
      Then created.status == 'success'; committed['final_state'] == 'committed'; committed['operation']['operation_id'] == created.operation_id; committed['operation']['result_revision'] == created.repo_revision; committed['changed_paths'] == ('/projects/reconciled.md',); committed['partial'] is False.

    @py-1d1667f6b959 @python_test_compare_manifest_classifies_generic_entries_and_assets @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestCatalogReference
    Scenario: test_compare_manifest_classifies_generic_entries_and_assets
      Given the pinned Python reference fixtures and controlled inputs
      When write_concept( repo_paths.current_dir / "compare" / f"{name}.md", concept_id=concept_id, concept_type="concept", title=name, description="M…; _skill_zip("comparison asset\n"); validate_asset_pack(asset_kind="templates", version="1.0.0", zip_bytes=zip_bytes).
      Then comparison['counts'] == {'matching': 1, 'differing': 2, 'local_only': 1, 'memento_only': 1}; comparison['matching'][0]['name'] == 'same'; comparison['matching'][0]['body_match'] is True; differing['old-name']['memento_path'] == '/compare/renamed.md'; differing['old-name']['likely_newer'] == 'local'; differing['old-name']['asset_present'] is True.

    @py-044c92f8cb9c @python_test_compare_manifest_enforces_scope_and_bounds_before_content_access @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestCatalogReference
    Scenario: test_compare_manifest_enforces_scope_and_bounds_before_content_access
      Given the pinned Python reference fixtures and controlled inputs
      When hidden.write_text("not valid frontmatter\n", encoding="utf-8"); datetime(2026, 7, 17, 12, 0, tzinfo=UTC); success_data( service.memory_compare_manifest( flint, path_prefix="/projects/", items=[local_item], ) ).
      Then visible['counts'] == {'matching': 0, 'differing': 1, 'local_only': 0, 'memento_only': 0}; forbidden.status == 'error'; forbidden.error_class == 'forbidden'; oversized.status == 'error'; oversized.error_class == 'validation_error'; 'exceeds 50 concepts' in oversized.message.

    @py-f036a7fd7333 @python_test_concurrent_rebase_and_reject_leave_proposal_rejected @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestCatalogReference
    Scenario: test_concurrent_rebase_and_reject_leave_proposal_rejected
      Given the pinned Python reference fixtures and controlled inputs
      When _continuity_proposal(service, flint); _advance_unrelated(service, smith); _server_for(service, service_config).
      Then results[1].status == 'success'; current['status'] == 'rejected' and current['review_comment'] == 'Not wanted'; success_data(service.memory_status(smith))['proposal_backlog'] == 0.

    @py-4b3ecc72a9e5 @python_test_conflicted_deleted_target_remains_reviewable @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestCatalogReference
    Scenario: test_conflicted_deleted_target_remains_reviewable
      Given the pinned Python reference fixtures and controlled inputs
      When _continuity_proposal(service, flint); success_data( service.memory_trash( smith, path="/projects/piclaw.md", expected_revision=get_main_revision(service._deps.repo_paths), idemp…; success_data(service.memory_proposal_list(smith, status="conflicted")).
      Then listed[0]['proposal_id'] == proposal['proposal_id']; success_data(service.memory_status(smith))['proposal_backlog'] == 1; failed.status == 'error' and failed.error_class == 'conflict'; success_data(review)['proposal']['review_comment'] == 'Target retired'.

    @py-9f20f634bfa8 @python_test_conflicted_proposal_detailed_view_survives_deleted_target @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestCatalogReference
    Scenario: test_conflicted_proposal_detailed_view_survives_deleted_target
      Given the pinned Python reference fixtures and controlled inputs
      When _continuity_proposal(service, flint); success_data( service.memory_trash( smith, path="/projects/piclaw.md", expected_revision=get_main_revision(service._deps.repo_paths), idemp…; success_data(service.memory_proposal_get(flint, proposal_id=proposal["proposal_id"])).
      Then view['status'] == 'conflicted'; view['changes'] == proposal['changes']; view['conflicts'][0]['conflicting_paths'] == ('/projects/piclaw.md',); 'Preview unavailable' in view['diff'].

    @py-c7b59ea7aa15 @python_test_control_schema_upgrade_keeps_proposal_review_and_assets @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestCatalogReference
    Scenario: test_control_schema_upgrade_keeps_proposal_review_and_assets
      Given the pinned Python reference fixtures and controlled inputs
      When _continuity_proposal(service, flint); success_data( service.memory_proposal_review( smith, proposal_id=proposal["proposal_id"], decision="request_changes", comment="Legacy revie…; connection.execute( "SELECT * FROM proposals WHERE proposal_id=?", (proposal["proposal_id"],) ).fetchone().
      Then tuple(connection.execute('SELECT * FROM proposals WHERE proposal_id=?', (proposal['proposal_id'],)).fetchone()) == before; 'Legacy review' in view['history'][0]['details_json'].

    @py-518cdb5a3472 @python_test_direct_mutations_warn_and_preserve_generic_asset_parity @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestCatalogReference
    Scenario: test_direct_mutations_warn_and_preserve_generic_asset_parity
      Given the pinned Python reference fixtures and controlled inputs
      When _skill_zip(skill_md); get_main_revision(repo_paths); success_data( service.memory_propose( smith, intent="Publish parity fixture", base_revision=base_revision, changes=[ { "kind": "create", "p….
      Then service.memory_proposal_review(smith, proposal_id=proposal['proposal_id'], decision='approve').status == 'success'; applied.status == 'success'; parity_break.status == 'error'; parity_break.error_class == 'conflict'; 'asset parity review' in parity_break.message; metadata_only.status == 'success'.

    @py-e8d3521ccfdb @python_test_direct_rename_rewrites_inbound_links_atomically @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestCatalogReference
    Scenario: test_direct_rename_rewrites_inbound_links_atomically
      Given the pinned Python reference fixtures and controlled inputs
      When get_main_revision(repo_paths); service.memory_rename( smith, path="/projects/piclaw.md", new_path="/projects/shared-piclaw.md", expected_revision=revision, idempotency_ke…; success_data(renamed).
      Then renamed.status == 'success'; '/instances/smith.md' in renamed_data['changed_paths']; '/projects/shared-piclaw.md' in renamed_data['changed_paths']; '/projects/shared-piclaw.md' in updated; '/projects/piclaw.md' not in updated.

    @py-f6fb8189c0d2 @python_test_execute_applies_and_replays_status_only_patches @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestCatalogReference
    Scenario: test_execute_applies_and_replays_status_only_patches
      Given the pinned Python reference fixtures and controlled inputs
      When get_main_revision(repo_paths); service.memory_execute(smith, plan=plan); success_data(service.memory_read(smith, id_or_path="/projects/piclaw.md")).
      Then first.status == 'success'; success_data(first)['trace'][0]['status'] == 'success'; concept['frontmatter']['status'] == status; replay.status == 'success'; success_data(replay)['trace'][0]['data']['replayed'] is True.

    @py-8ad3bf33eb1c @python_test_execute_bad_projection_preserves_committed_result @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestCatalogReference
    Scenario: test_execute_bad_projection_preserves_committed_result
      Given the pinned Python reference fixtures and controlled inputs
      When service.memory_execute( smith, plan={ "operations": [ { "op": "create", "args": { "path": "/projects/committed.md", "concept_type": "projec…; success_data(result).
      Then data['stopped'] is True; data['revisions'][0]['operation_id']; data['revisions'][0]['repo_revision'] == get_main_revision(service._deps.repo_paths); result.warnings.

    @py-91473d93111d @python_test_execute_chains_typed_asset_offsets_and_digest_references @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestCatalogReference
    Scenario: test_execute_chains_typed_asset_offsets_and_digest_references
      Given the pinned Python reference fixtures and controlled inputs
      When success_data( service.memory_execute( smith, plan={ "operations": [ { "op": "asset_get", "save_as": "first", "args": { "id_or_path": path, ….
      Then decode(first) + decode(last) == 'a界b'.encode(); last['offset'] == 2 and last['returned_bytes'] == 3 and last['next_offset'] is None.

    @py-69da15129e23 @python_test_execute_deadline_without_commit_remains_an_error @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestCatalogReference
    Scenario: test_execute_deadline_without_commit_remains_an_error
      Given the pinned Python reference fixtures and controlled inputs
      When iter((0.0, 0.0, 4.0, 4.0)); monkeypatch.setattr("memento.executor.monotonic", lambda: next(ticks)); service.memory_execute( flint, plan={"operations": [{"op": "status", "args": {}}]}, ).
      Then result.status == 'error'; result.error_class == 'validation_error'; result.message == 'plan exceeded configured max_time_seconds'.

    @py-aa388d0629f0 @python_test_execute_default_asset_chunk_fits_default_budget @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestCatalogReference
    Scenario: test_execute_default_asset_chunk_fits_default_budget
      Given the pinned Python reference fixtures and controlled inputs
      When success_data( service.memory_execute( smith, plan={ "operations": [ {"op": "asset_get", "args": {"id_or_path": path, "asset_kind": "documen…; success_data( service.memory_asset_get( smith, id_or_path=path, asset_kind="document", view="file", file_path="data.bin", limit=65536, ) ); success_data( service.memory_asset_get( smith, id_or_path=path, asset_kind="document", version="1.0.0", expected_sha256=first["zip_sha256"]….
      Then 0 < chunk['returned_bytes'] < len(raw); base64.b64decode(chunk['zip_base64']) == raw[:chunk['returned_bytes']]; chunk['next_offset'] == chunk['returned_bytes']; first['file']['encoding'] == last['file']['encoding'] == 'base64'; base64.b64decode(first['file']['con

    @py-445dad67cce9 @python_test_execute_limits_auth_and_error_control @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestCatalogReference
    Scenario: test_execute_limits_auth_and_error_control
      Given the pinned Python reference fixtures and controlled inputs
      When service.memory_execute( flint, plan={ "operations": [ { "op": "proposal_review", "args": {"proposal_id": "missing", "decision": "approve"},…; success_data(failed_saved_operation); service.memory_execute( flint, plan={ "stop_on_error": False, "operations": [ {"op": "status", "args": {}, "save_as": "result"}, { "op": "p….
      Then failed_saved_operation.status == 'success'; failed_data['trace'][0]['status'] == 'error'; failed_data['trace'][0]['error_class'] == 'forbidden'; failed_data['returns'] == {}; failed_data['stop_reason'] == 'operation 1 failed'; success_data(continued_after_failed_overwrite)['returns']['principal'] == 'flint'.

    @py-f3d31c7d1d81 @python_test_execute_rebase_preserves_control_commit_on_later_reference_error @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestCatalogReference
    Scenario: test_execute_rebase_preserves_control_commit_on_later_reference_error
      Given the pinned Python reference fixtures and controlled inputs
      When _continuity_proposal(service, flint); _advance_unrelated(service, smith); service.memory_execute( flint, plan={ "operations": [ { "op": "proposal_rebase", "save_as": "rebased", "args": { "proposal_id": proposal["p….
      Then result.status == 'success'; success_data(result)['stopped']; success_data(result)['trace'][0]['operation_id']; result.warnings; success_data(service.memory_operation_get(flint, idempotency_key='execute-rebase'))['final_state'] == 'committed'.

    @py-aa8a2c039bbe @python_test_execute_reference_schema_does_not_loosen_direct_asset_schema @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestCatalogReference
    Scenario: test_execute_reference_schema_does_not_loosen_direct_asset_schema
      Given the pinned Python reference fixtures and controlled inputs
      When AssetGetArgs.model_json_schema(); execute_plan_schema().
      Then direct['properties']['offset']['type'] == 'integer'; offset['anyOf'][0]['minimum'] == 0; offset['anyOf'][1]['pattern'] starts with '^\\$'; schema['$defs']['AssetGetOperation']['properties']['op']['const'] == 'asset_get'.

    @py-4ff1245d5d0f @python_test_execute_rejects_invalid_references_and_multiple_commit_ops @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestCatalogReference
    Scenario: test_execute_rejects_invalid_references_and_multiple_commit_ops
      Given the pinned Python reference fixtures and controlled inputs
      When service.memory_execute( flint, plan={ "operations": [ {"op": "search", "args": {"query": "Piclaw"}, "save_as": "hits"}, {"op": "read", "arg…; get_main_revision(repo_paths); service.memory_execute( smith, plan={ "operations": [ { "op": "create", "args": { "path": "/projects/a.md", "concept_type": "project", "tit….
      Then invalid.status == 'error'; invalid.error_class == 'validation_error'; commit_heavy.status == 'error'; commit_heavy.error_class == 'validation_error'.

    @py-efcac34f76d8 @python_test_execute_reports_success_when_deadline_expires_after_commit @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestCatalogReference
    Scenario: test_execute_reports_success_when_deadline_expires_after_commit
      Given the pinned Python reference fixtures and controlled inputs
      When iter((0.0, 0.0, 4.0)); monkeypatch.setattr("memento.executor.monotonic", lambda: next(ticks)); service.memory_execute( smith, plan={ "operations": [ { "op": "create", "args": { "path": "/projects/post-commit-deadline.md", "concept_typ….
      Then result.status == 'success'; 'memory_execute_deadline_exceeded_after_commit' in result.warnings; data['stopped'] is True; data['stop_reason'] == 'deadline exceeded after committed operation'; len(data['trace']) == 1; data['revisions'][0]['repo_revision'] == get_main_revision(repo_paths).

    @py-dc481f58abe3 @python_test_execute_search_read_and_projection @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestCatalogReference
    Scenario: test_execute_search_read_and_projection
      Given the pinned Python reference fixtures and controlled inputs
      When service.memory_execute( flint, plan={ "operations": [ {"op": "search", "args": {"query": "Piclaw"}, "save_as": "hits"}, {"op": "read", "arg….
      Then result.status == 'success'; success_data(result)['returns']['title'] == 'Piclaw'.

    @py-38f0bd623cf9 @python_test_execute_tool_schema_and_normalization_support_both_argument_forms @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestCatalogReference
    Scenario: test_execute_tool_schema_and_normalization_support_both_argument_forms
      Given the pinned Python reference fixtures and controlled inputs
      When execute_tool_schema().
      Then set(schema['properties']) >= {'plan', 'operations', 'stop_on_error', 'returns'}; schema['additionalProperties'] is False; normalize_execute_tool_arguments(plan=plan) is plan; normalize_execute_tool_arguments(operations=plan['operations'], stop_on_error=False, returns=[]) == {'operations': plan['operations'], 'stop_on_error': False, 'returns': []}. expects normalize_execute_tool_arguments(plan=plan, operations=plan["operations"]) raises ValueError matching "either plan or top-level"; normalize_execute_tool_arguments() raises ValueError matching "requires plan or operations".

    @py-c2d8e1cf7acd @python_test_execute_worker_does_not_block_event_loop @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestCatalogReference
    Scenario: test_execute_worker_does_not_block_event_loop
      Given the pinned Python reference fixtures and controlled inputs
      When _server_for(service, service_config); monkeypatch.setattr(server, "_context", lambda: smith); monkeypatch.setattr(server, "_execute_in_worker", delayed).
      Then the result, state transitions, and error boundary match the captured behavior

    @py-0b6681a36f1d @python_test_explicit_creates_enforce_concept_invariants @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestCatalogReference
    Scenario: test_explicit_creates_enforce_concept_invariants
      Given the pinned Python reference fixtures and controlled inputs
      When get_main_revision(service._deps.repo_paths); service.memory_create( smith, path=path, concept_type="project", title="Invalid", body=body, expected_revision=revision, idempotency_key="i…; service.memory_propose( smith, intent="Invalid", base_revision=revision, changes=[ { "kind": "create", "path": path, "concept_type": "proje….
      Then result.status == 'error'; get_main_revision(service._deps.repo_paths) == revision; proposal.status == 'error'.

    @py-f3daf49b8473 @python_test_generic_asset_proposal_rejects_duplicate_and_rename_mix @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestCatalogReference
    Scenario: test_generic_asset_proposal_rejects_duplicate_and_rename_mix
      Given the pinned Python reference fixtures and controlled inputs
      When _skill_zip("# Template\n"); service.memory_propose( flint, intent="Attach templates", base_revision=get_main_revision(repo_paths), changes=changes, ); service.memory_propose( flint, intent="Duplicate templates", base_revision=get_main_revision(repo_paths), changes=changes, ).
      Then first.status == 'success'; duplicate.status == 'error'; duplicate.error_class == 'conflict'; mixed.status == 'error'; 'separate proposals' in mixed.message.

    @py-e410a0a750bd @python_test_invalid_saved_references_and_limits_are_compact @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestCatalogReference
    Scenario: test_invalid_saved_references_and_limits_are_compact
      Given the pinned Python reference fixtures and controlled inputs
      When monkeypatch.setattr( service, "memory_status", lambda _: service._success({"values": [0], "excess": 262145, "zero": 0}), ); monkeypatch.setattr(service, "memory_asset_get", lambda *args, **kwargs: calls.append(kwargs)); service.memory_execute( smith, plan={ "operations": [ {"op": "status", "save_as": "source"}, { "op": "asset_get", "args": {"id_or_path": "/….
      Then result.status == 'error' and 'operation 2 (asset_get)' in result.message; len(result.message) < 512 and calls == [].

    @py-af0eb3f6906a @python_test_legacy_stale_clean_rebase_does_not_renew_expiry @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestCatalogReference
    Scenario: test_legacy_stale_clean_rebase_does_not_renew_expiry
      Given the pinned Python reference fixtures and controlled inputs
      When _continuity_proposal(service, flint); _advance_unrelated(service, smith); service._deps.control_connection.execute( "UPDATE proposals SET status='stale' WHERE proposal_id=?", (proposal["proposal_id"],) ).
      Then view['expires_at'] == proposal['expires_at']; view['created_at'] == proposal['created_at']; view['status'] == 'submitted'.

    @py-2088d641c243 @python_test_managed_access_instructions_and_tool_descriptions @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestCatalogReference
    Scenario: test_managed_access_instructions_and_tool_descriptions
      Given the pinned Python reference fixtures and controlled inputs
      When access_store.create( actor="bootstrap", name="onboarding-admin", roles=("admin", "reader"), read_prefixes=("/",), write_prefixes=(), idempo…; monkeypatch.setattr( "memento.server.get_request_context", lambda: SimpleNamespace(principal="onboarding-admin", session_id="session-admin"…; server.get_instructions().
      Then 'direct access_* tools, not memory_execute operations' in instructions; 'administrator bearer token out of ordinary agent runtimes' in instructions; 'tool input or output' in instructions; 'separate admin profile' in instructions; 'credentials returned once by create or rotate' in instructions; 'least-privilege principal' in create_tool['description'].

    @py-08d5630482ae @python_test_mcp_asset_stage_ticket_and_status_bridge @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestCatalogReference
    Scenario: test_mcp_asset_stage_ticket_and_status_bridge
      Given the pinned Python reference fixtures and controlled inputs
      When monkeypatch.setattr( "memento.server.get_request_context", lambda: SimpleNamespace(principal="smith", session_id="session-1"), ); asyncio.run( server.tool_memory_asset_stage_begin( asset_kind="templates", version="1.0.0", idempotency_key="mcp-ticket-1", ) ); asyncio.run(server.tool_memory_asset_stage_status("mcp-ticket-1")).
      Then begun['status'] == 'success'; begun['data']['upload_path'] == '/assets/staging/upload'; begun['data']['upload_ticket_header'] == 'X-Memento-Upload-Ticket'; begun['data']['workflow'] == 'memory://workflow/asset_pack'; begun['data']['proposal_contract'] == 'memory://catalog/propose'; begun['next_tools'] == ['memory_asset_stage_status', 'memory://workflow/asset

    @py-b0335a8dca2d @python_test_memory_graph_serializes_typed_edges_and_preserves_scope @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestCatalogReference
    Scenario: test_memory_graph_serializes_typed_edges_and_preserves_scope
      Given the pinned Python reference fixtures and controlled inputs
      When success_data(service.memory_graph(flint, id_or_path="/instances/smith.md")); success_data(service.memory_graph(narrow, id_or_path="/instances/smith.md")); service.memory_graph(flint, id_or_path="/secret/ghost.md").
      Then payload == {'center_id': 'smith-id', 'outbound': [{'concept_id': 'piclaw-id', 'path': '/projects/piclaw.md', 'title': 'Piclaw', 'depth': 1, 'direction': 'outbound', 'broken_link_count': 0, 'orphan_flag': False}], 'inbound': [{'concept_id': 'piclaw-id', 'path': '/projects/piclaw.md', 'title': 'Piclaw', 'depth': 1, 'direction': 'inbound', 'broken_link_count': 0, 'orphan_flag': False}], 'broken_targets': ()}; scoped['outbound'] == []; scoped['inbound'] == []; protected.s

    @py-df51d56f8f7f @python_test_memory_inventory_filters_protected_namespaces_before_parsing @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestCatalogReference
    Scenario: test_memory_inventory_filters_protected_namespaces_before_parsing
      Given the pinned Python reference fixtures and controlled inputs
      When hidden.write_text("not valid frontmatter\n", encoding="utf-8"); success_data(service.memory_inventory(flint, fields=["path"])); service.memory_inventory(flint, path_prefix="/secret/").
      Then [entry['path'] for entry in visible['entries']] == ['/instances/smith.md', '/projects/piclaw.md']; forbidden.status == 'error'; forbidden.error_class == 'forbidden'; [entry['path'] for entry in protected['entries']] == ['/secret/ghost.md'].

    @py-71fd4c793629 @python_test_memory_inventory_is_available_directly_and_via_execute @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestCatalogReference
    Scenario: test_memory_inventory_is_available_directly_and_via_execute
      Given the pinned Python reference fixtures and controlled inputs
      When _server_for( service, service_config.model_copy(update={"mcp": MCPConfig(tool_surface="compact")}), ); server.discover_tools(); service.memory_execute( flint, plan={ "operations": [ { "op": "inventory", "args": { "path_prefix": "/projects/", "fields": ["path", "body_….
      Then 'memory_inventory' in tools; inventory['entries'][0]['path'] == '/projects/piclaw.md'; 'body_sha256' in inventory['entries'][0]; 'body' not in inventory['entries'][0].

    @py-d4945ffb9f77 @python_test_memory_inventory_parses_only_the_bounded_page @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestCatalogReference
    Scenario: test_memory_inventory_parses_only_the_bounded_page
      Given the pinned Python reference fixtures and controlled inputs
      When malformed.write_text("not valid frontmatter\n", encoding="utf-8"); success_data( service.memory_inventory( flint, path_prefix="/projects/", fields=["path"], limit=1, ) ); service.memory_inventory( flint, path_prefix="/projects/", fields=["path"], limit=1, cursor=first["next_cursor"], ).
      Then first['entries'] == [{'path': '/projects/piclaw.md'}]; first['next_cursor'] == '/projects/piclaw.md'; invalid_page.status == 'error'; invalid_page.error_class == 'validation_error'.

    @py-c84e7b9f4cef @python_test_memory_inventory_returns_stable_digests_assets_and_pagination @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestCatalogReference
    Scenario: test_memory_inventory_returns_stable_digests_assets_and_pagination
      Given the pinned Python reference fixtures and controlled inputs
      When write_concept( repo_paths.current_dir / "skills" / "inventory.md", concept_id=concept_id, concept_type="concept", title="Inventory asset", …; write_concept( repo_paths.current_dir / "root.md", concept_id="root-id", concept_type="concept", title="Root concept", description="Paginat…; write_asset_version( repo_paths.current_dir, concept_id=concept_id, concept_path="/skills/inventory.md", asset_kind="templates", version=ve….
      Then payload['next_cursor'] is None; len(payload['entries']) == 1; entry['path'] == '/skills/inventory.md'; entry['created_at'] == '2026-07-17T12:00:00Z'; entry['updated_at'] == '2026-07-17T12:00:00Z'; entry['updated_by'] == 'rui/tests'.

    @py-552c058afa37 @python_test_memory_list_includes_light_frontmatter_metadata @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestCatalogReference
    Scenario: test_memory_list_includes_light_frontmatter_metadata
      Given the pinned Python reference fixtures and controlled inputs
      When success_data(service.memory_list(flint, path_prefix="/projects/")).
      Then payload['entries'] == [{'path': '/projects/piclaw.md', 'id': 'piclaw-id', 'title': 'Piclaw', 'type': 'project', 'status': 'active', 'description': 'Visible project.', 'aliases': (), 'tags': ('shared',)}].

    @py-db18374da9f9 @python_test_memory_proposal_list_filters_status_in_control_query @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestCatalogReference
    Scenario: test_memory_proposal_list_filters_status_in_control_query
      Given the pinned Python reference fixtures and controlled inputs
      When monkeypatch.setattr("memento.service.list_proposals", tracked_list_proposals); service.memory_proposal_list(smith, status="submitted", limit=1).
      Then listed.status == 'success'; observed_statuses == [ProposalStatus.SUBMITTED].

    @py-28509b3f6159 @python_test_memory_route_direct_execute_unknown_auth_and_malformed @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestCatalogReference
    Scenario: test_memory_route_direct_execute_unknown_auth_and_malformed
      Given the pinned Python reference fixtures and controlled inputs
      When service_config.model_copy( update={ "intelligent_tiers": service_config.intelligent_tiers.model_copy( update={ "needle_router": service_con…; routed_service.memory_route(smith, request="Find Piclaw"); success_data(result).
      Then result.status == 'success'; data['executed'] is True; data['action']['action'] == 'search_paths'; data['result']['status'] == 'success'; data['result']['data']['value'] == [{'path': '/projects/piclaw.md'}]; search_router.calls[0][0] == 'Find Piclaw'.

    @py-ccbebd4c7992 @python_test_memory_route_disabled_and_server_discovery @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestCatalogReference
    Scenario: test_memory_route_disabled_and_server_discovery
      Given the pinned Python reference fixtures and controlled inputs
      When service.memory_route(smith, request="find Piclaw"); service_config.model_copy( update={ "intelligent_tiers": service_config.intelligent_tiers.model_copy( update={ "needle_router": service_con…; _server_for( service, enabled.model_copy(update={"mcp": MCPConfig(tool_surface="compact")}), needle_router=FakeNeedleRouter('[{"name":"UNKN….
      Then disabled.status == 'error'; disabled.error_class == 'validation_error'; tools == ['memory_help', 'memory_status', 'memory_search', 'memory_read', 'memory_inventory', 'memory_route', 'memory_asset_stage_begin', 'memory_asset_stage_status', 'memory_asset_get', 'memory_execute']; any((item['operation'] == 'route' and item['tool'] == 'memory_route' for

    @py-933d667bd223 @python_test_operation_lookup_rechecks_after_writer_finishes @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestCatalogReference
    Scenario: test_operation_lookup_rechecks_after_writer_finishes
      Given the pinned Python reference fixtures and controlled inputs
      When _continuity_proposal(service, flint); _advance_unrelated(service, smith); service.memory_proposal_rebase( flint, proposal_id=proposal["proposal_id"], expected_revision=revision, idempotency_key="lookup-race", ).
      Then result.status == 'success'; state['final_state'] == 'committed' and not state['safe_to_retry']; calls == 2.

    @py-1ccc89061326 @python_test_original_proposer_rebase_preserves_identity_and_reconciles @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestCatalogReference
    Scenario: test_original_proposer_rebase_preserves_identity_and_reconciles
      Given the pinned Python reference fixtures and controlled inputs
      When _continuity_proposal(service, flint); success_data( service.memory_proposal_review( smith, proposal_id=proposal["proposal_id"], decision="approve", comment="Initial approval", )…; _advance_unrelated(service, smith).
      Then success_data(rebased)['status'] == 'submitted'; view[key] == proposal[key]; view['base_revision'] == revision and view['reviewed_by'] is None; {e['action'] for e in view['history']} == {'review', 'repository_advanced', 'rebase'}; 'Initial approval' in view['history'][0]['details_json']; success_data(replay)['replayed'] and replay.operation_id == rebased.operation_id.

    @py-cf4da8f06ec2 @python_test_plan_errors_do_not_dump_union_or_arguments @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestCatalogReference
    Scenario: test_plan_errors_do_not_dump_union_or_arguments
      Given the pinned Python reference fixtures and controlled inputs
      When service.memory_execute(smith, plan={"operations": [invalid]}).
      Then result.status == 'error'; len(result.message) <= 512; 'NO-SECRET-ECHO' not in result.message; 'For further information' not in result.message.

    @py-2af5d35547a0 @python_test_proposal_lifecycle_self_approval_stale_apply_and_idempotency @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestCatalogReference
    Scenario: test_proposal_lifecycle_self_approval_stale_apply_and_idempotency
      Given the pinned Python reference fixtures and controlled inputs
      When get_main_revision(repo_paths); service.memory_propose( smith, intent="Update Piclaw", base_revision=base_revision, changes=[ { "kind": "patch", "path": "/projects/piclaw.…; success_data(proposed).
      Then proposed.status == 'success'; 'Updated by proposal' in proposed_data['proposal']['diff']; approved.status == 'success'; approved_proposal['status'] == 'approved'; approved_proposal['author_principal'] == 'smith'; approved_proposal['reviewed_by'] == 'smith'.

    @py-84bae50d75d9 @python_test_proposal_list_visibility_and_expiry @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestCatalogReference
    Scenario: test_proposal_list_visibility_and_expiry
      Given the pinned Python reference fixtures and controlled inputs
      When service.memory_propose( flint, intent="Visible only to author or curator", base_revision=get_main_revision(repo_paths), changes=[{"kind": "…; success_data(proposal); update_proposal_status( control_connection, proposal_id, status=ProposalStatus.SUBMITTED, ).
      Then expired.proposal_id == proposal_id; len(author_visible_data['proposals']) == 1; author_visible_data['proposals'][0]['status'] == 'expired'; len(success_data(curator_visible)['proposals']) == 1.

    @py-b40d07b7b416 @python_test_proposal_rebase_catalog_is_execute_only @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestCatalogReference
    Scenario: test_proposal_rebase_catalog_is_execute_only
      Given the pinned Python reference fixtures and controlled inputs
      When _server_for(service, service_config); server._catalog_operation("proposal_rebase", direct_tool_available=False).
      Then catalog['available_via_execute'] and not catalog['direct_tool_available']; catalog['input_schema']['properties']['idempotency_key']['minLength'] == 1.

    @py-225024aa415e @python_test_proposal_review_keeps_role_and_write_scope_checks @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestCatalogReference
    Scenario: test_proposal_review_keeps_role_and_write_scope_checks
      Given the pinned Python reference fixtures and controlled inputs
      When service.memory_propose( smith, intent="Check review authorization", base_revision=get_main_revision(repo_paths), changes=[ { "kind": "patch…; success_data(proposed); service.memory_proposal_review(flint, proposal_id=proposal_id, decision="approve").
      Then non_curator.status == 'error'; non_curator.error_class == 'forbidden'; outside_write_scope.status == 'error'; outside_write_scope.error_class == 'forbidden'; current['status'] == 'submitted'; current['reviewed_by'] is None.

    @py-32f890f6a10c @python_test_proposal_summary_pagination_and_bounded_asset_inspection @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestCatalogReference
    Scenario: test_proposal_summary_pagination_and_bounded_asset_inspection
      Given the pinned Python reference fixtures and controlled inputs
      When _skill_zip( concept_body, script="console.log('inspectable')\n", entry_path="CONCEPT.md", ); success_data( service.memory_propose( smith, intent="Inspectable proposal", base_revision=get_main_revision(repo_paths), changes=[ { "kind"…; success_data( service.memory_propose( smith, intent="Second bounded proposal", base_revision=get_main_revision(repo_paths), changes=[ { "ki….
      Then len(page_one['proposals']) == 1; page_one['next_cursor'] is not None; listed_ids == {first['proposal_id'], second_id}; concept_body not in json.dumps(page_one); concept_body not in json.dumps(page_two); summary['changes'] == [{'index': 0, 'kind': 'create', 'path': '/projects/inspectable.md'}, {'index': 1, 'ki

    @py-53d48c5b0373 @python_test_protected_proposals_are_hidden_without_explicit_read_access @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestCatalogReference
    Scenario: test_protected_proposals_are_hidden_without_explicit_read_access
      Given the pinned Python reference fixtures and controlled inputs
      When service.memory_propose( flint, intent="Update protected memory", base_revision=get_main_revision(repo_paths), changes=[ { "kind": "patch", …; success_data(proposed); service.memory_proposal_get(flint, proposal_id=proposal_id).
      Then fetched.status == 'error'; fetched.error_class == 'forbidden'; proposal_id not in [item['proposal_id'] for item in listed['proposals']].

    @py-200558c704c6 @python_test_pruning_versions_removes_all_read_views @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestCatalogReference
    Scenario: test_pruning_versions_removes_all_read_views
      Given the pinned Python reference fixtures and controlled inputs
      When archive.writestr("new.txt", "version two"); success_data( service.memory_propose( smith, intent="new version", base_revision=get_main_revision(service._deps.repo_paths), changes=[ { "…; success_data( service.memory_proposal_review( smith, proposal_id=proposal["proposal_id"], decision="approve" ) ).
      Then service.memory_asset_get(smith, id_or_path=path, asset_kind='document', version='1.0.0', view=view, file_path='notes.txt' if view == 'file' else None).status == 'error'; success_data(service.memory_asset_get(smith, id_or_path=path, asset_kind='document', view='manifest'))['version'] == '2.0.0'.

    @py-b85bf0910ee4 @python_test_purge_removes_accepted_assets_and_restore_collision_is_safe @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestCatalogReference
    Scenario: test_purge_removes_accepted_assets_and_restore_collision_is_safe
      Given the pinned Python reference fixtures and controlled inputs
      When _skill_zip(body); success_data( service.memory_propose( smith, intent="trash asset", base_revision=get_main_revision(service._deps.repo_paths), changes=[ { "…; success_data( service.memory_proposal_review( smith, proposal_id=proposal["proposal_id"], decision="approve" ) ).
      Then before; all((path.exists() for path in before)); service.memory_restore(smith, path='/trash/projects/trash-asset.md', expected_revision=revision, idempotency_key='collision-restore').status == 'error'; get_main_revision(service._deps.repo_paths) == revision; result['trace'][0]['status'] == 'success'; not any((path.exists() for path in before)).

    @py-b9e10d408da6 @python_test_rebase_control_transaction_rolls_back_and_can_retry @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestCatalogReference
    Scenario: test_rebase_control_transaction_rolls_back_and_can_retry
      Given the pinned Python reference fixtures and controlled inputs
      When _continuity_proposal(service, flint); _advance_unrelated(service, smith); monkeypatch.setattr(service, "_journal_proposal_control", fail).
      Then row['base_revision'] == proposal['base_revision'] and row['status'] == 'needs_rebase'; success_data(service.memory_operation_get(flint, idempotency_key='atomic'))['safe_to_retry']; service.memory_proposal_rebase(flint, proposal_id=proposal['proposal_id'], expected_revision=revision, idempotency_key='atomic').status == 'success'. expects service.memory_proposal_rebase( flint, proposal_id=proposal["proposal_id"], expected_revision=revision, idempotency_key="atomic", ) raises RuntimeError matching "before control commit".

    @py-b08f8b55f946 @python_test_rebase_keeps_asset_bytes_without_upload @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestCatalogReference
    Scenario: test_rebase_keeps_asset_bytes_without_upload
      Given the pinned Python reference fixtures and controlled inputs
      When _advance_unrelated(service, smith, key="asset-target"); pack.writestr("readme.txt", "unchanged asset"); base64.b64encode(buffer.getvalue()).decode().
      Then after == before; applied.status == 'success'.

    @py-fce6b15be514 @python_test_rebase_permissions_expiry_and_legacy_state @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestCatalogReference
    Scenario: test_rebase_permissions_expiry_and_legacy_state
      Given the pinned Python reference fixtures and controlled inputs
      When _continuity_proposal(service, smith); _advance_unrelated(service, smith); service.memory_proposal_rebase( flint, proposal_id=proposal["proposal_id"], expected_revision=revision, idempotency_key="foreign", ).
      Then result.status == 'error' and result.error_class == 'forbidden'; result.status == 'error' and result.error_class == 'forbidden'; success_data(service.memory_status(smith))['proposal_backlog'] == 1; success_data(service.memory_proposal_list(smith, status='needs_rebase'))['proposals'][0]['status'] == 'needs_rebase'; success_data(service.memory_status(smith))['proposal_backlog'] == 0; expired.status == 'error' and 'expired' in expired.message.

    @py-4211305a7ad3 @python_test_rebase_retains_skill_root_body_parity @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestCatalogReference
    Scenario: test_rebase_retains_skill_root_body_parity
      Given the pinned Python reference fixtures and controlled inputs
      When _skill_zip(body); success_data( service.memory_propose( flint, intent="Skill continuity", base_revision=get_main_revision(service._deps.repo_paths), changes=…; _advance_unrelated(service, smith).
      Then bytes(service._deps.control_connection.execute('SELECT blob_bytes FROM proposal_assets WHERE proposal_id=? AND asset_id=?', (proposal['proposal_id'], asset_id)).fetchone()[0]) == zip_bytes; summary['assets'][0]['concept_body_matches_asset'] is True; recalled['file']['content'] == body.

    @py-82982893535b @python_test_rebase_worker_timeout_reconciles_original_key_after_commit @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestCatalogReference
    Scenario: test_rebase_worker_timeout_reconciles_original_key_after_commit
      Given the pinned Python reference fixtures and controlled inputs
      When _continuity_proposal(service, flint); _advance_unrelated(service, smith); _server_for(service, service_config).
      Then the result, state transitions, and error boundary match the captured behavior

    @py-14f4f0f26ede @python_test_rename_rejects_unauthorised_backlink_rewrites @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestCatalogReference
    Scenario: test_rename_rejects_unauthorised_backlink_rewrites
      Given the pinned Python reference fixtures and controlled inputs
      When write_concept( original, concept_id="visible-id", concept_type="project", title="Visible", description="Visible project.", tags=("visible",…; write_concept( protected, concept_id="backlink-id", concept_type="project", title="Protected backlink", description="Protected backlink.", ….
      Then original.exists(); not (worktree / 'projects' / 'renamed.md').exists(); '/projects/visible.md' in protected.read_text(encoding='utf-8'). expects service._apply_rename( worktree, RenameChange( kind="rename", path="/projects/visible.md", new_path="/projects/renamed.md", ), actor="smith… raises AuthorizationError matching "cannot write /secret/backlink.md".

    @py-d305400c6e5f @python_test_resolved_integer_types_and_bounds_fail_before_dispatch @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestCatalogReference
    Scenario: test_resolved_integer_types_and_bounds_fail_before_dispatch
      Given the pinned Python reference fixtures and controlled inputs
      When monkeypatch.setattr(service, "memory_status", lambda _: service._success({"offset": value})); monkeypatch.setattr(service, "memory_asset_get", lambda *args, **kwargs: calls.append(kwargs)); service.memory_execute( smith, plan={ "operations": [ {"op": "status", "save_as": "source"}, { "op": "asset_get", "args": { "id_or_path": "….
      Then result.status == 'error'; 'operation 2 (asset_get)' in result.message and 'args.offset' in result.message; len(result.message) < 512 and "Input should be 'help'" not in result.message; calls == [].

    @py-74352392a175 @python_test_resolved_references_cannot_widen_authorisation @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestCatalogReference
    Scenario: test_resolved_references_cannot_widen_authorisation
      Given the pinned Python reference fixtures and controlled inputs
      When monkeypatch.setattr( service, "memory_status", lambda _: service._success({"path": "/projects/piclaw.md"}) ); success_data( service.memory_execute( narrow, plan={ "operations": [ {"op": "status", "save_as": "source"}, {"op": "read", "args": {"id_or_….
      Then result['trace'][1]['error_class'] == 'forbidden'.

    @py-da021852fc90 @python_test_reviewed_archival_rejects_collision_and_oversized_impact @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestCatalogReference
    Scenario: test_reviewed_archival_rejects_collision_and_oversized_impact
      Given the pinned Python reference fixtures and controlled inputs
      When destination.parent.mkdir(parents=True); destination.write_text("collision"); get_main_revision(service._deps.repo_paths).
      Then service.memory_propose(smith, **args).status == 'error'; result.status == 'error'; '100 inbound' in result.message; len(list_proposals(service._deps.control_connection)) == before.

    @py-e00d6cf2fb83 @python_test_reviewed_archival_reports_backlinks_and_replays @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestCatalogReference
    Scenario: test_reviewed_archival_reports_backlinks_and_replays
      Given the pinned Python reference fixtures and controlled inputs
      When get_main_revision(service._deps.repo_paths); success_data( service.memory_execute( smith, plan={ "operations": [ { "op": "propose", "args": { "intent": "Archive fixture", "base_revisio…; success_data(service.memory_proposal_review(smith, proposal_id=pid, decision="approve")).
      Then proposed['status'] == 'success'; report['inbound_references'][0]['path'] == '/instances/smith.md'; report['inbound_references'][0]['after_archival'] == 'unresolved'; report['history_retained'] and report['assets'] == []; service.memory_proposal_get(narrow, proposal_id=pid).status == 'error'; service.memory_proposal_apply(smith, proposal_id=pid, expected_revision=revision, idempotency_key='reviewed-trash').s

    @py-eb268fd71c02 @python_test_saved_values_revalidated_in_nested_arrays_and_booleans @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestCatalogReference
    Scenario: test_saved_values_revalidated_in_nested_arrays_and_booleans
      Given the pinned Python reference fixtures and controlled inputs
      When monkeypatch.setattr( service, "memory_status", lambda _: service._success({"fields": ["path", "id"], "flag": False}), ); success_data( service.memory_execute( smith, plan={ "operations": [ {"op": "status", "save_as": "source"}, { "op": "inventory", "args": { "….
      Then result['trace'][1]['status'] == 'success'; result['trace'][1]['data']['fields'] == ['path', 'id']; result['trace'][2]['status'] == 'error'; 'confirm=true' in result['trace'][2]['message'].

    @py-f62307f2af31 @python_test_server_rejects_duplicate_principal_names @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestCatalogReference
    Scenario: test_server_rejects_duplicate_principal_names
      Given the pinned Python reference fixtures and controlled inputs
      When expects MementoMCPServer( service, bearer_tokens={ "smith-a": Principal(name="smith", roles=("reader",)), "smith-b": Principal(name="smith", roles=… raises ValueError matching "duplicate principal name".
      Then the result, state transitions, and error boundary match the captured behavior

    @py-fc5894882d44 @python_test_skill_asset_proposal_rejects_noncanonical_or_mismatched_root @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestCatalogReference
    Scenario: test_skill_asset_proposal_rejects_noncanonical_or_mismatched_root
      Given the pinned Python reference fixtures and controlled inputs
      When _skill_zip(root_skill_md); service.memory_propose( smith, intent="Reject non-canonical skill", base_revision=get_main_revision(repo_paths), changes=[ { "kind": "creat….
      Then result.status == 'error'; message in result.message.

    @py-b7ac71a2d989 @python_test_staged_skill_asset_is_consumed_by_proposal @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestCatalogReference
    Scenario: test_staged_skill_asset_is_consumed_by_proposal
      Given the pinned Python reference fixtures and controlled inputs
      When _skill_zip(skill_md); store.put( principal="flint", idempotency_key="staged-skill-upload-1", asset_kind="skill", version="1.0.0", zip_bytes=zip_bytes, ); staged_service.memory_propose( flint, intent="Share staged skill", base_revision=get_main_revision(repo_paths), changes=[ { "kind": "create….
      Then 'staged_asset_id' not in proposal['changes'][1]; consumed.state == 'consumed'; consumed.proposal_id == proposal['proposal_id']; replay.status == 'error'; 'not ready' in replay.message.

    @py-ae400645895a @python_test_stale_filter_uses_effective_status_with_bounded_query @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestCatalogReference
    Scenario: test_stale_filter_uses_effective_status_with_bounded_query
      Given the pinned Python reference fixtures and controlled inputs
      When get_main_revision(service._deps.repo_paths); success_data( service.memory_propose( smith, intent="stale", base_revision=revision, changes=[{"kind": "patch", "path": "/projects/piclaw.m…; success_data( service.memory_create( smith, path="/projects/advance.md", concept_type="project", title="Advance", body="advance", expected_….
      Then page['proposals'][0]['proposal_id'] == proposal['proposal']['proposal_id']; observed == [2].

    @py-fcdf5faf0c11 @python_test_stale_proposal_conflicts_and_safe_subset_revision @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestCatalogReference
    Scenario: test_stale_proposal_conflicts_and_safe_subset_revision
      Given the pinned Python reference fixtures and controlled inputs
      When _skill_zip(skill_md); get_main_revision(repo_paths); success_data( service.memory_propose( smith, intent="Independent stale suggestions", base_revision=base_revision, changes=[ { "kind": "patc….
      Then advanced.status == 'success'; summary['status'] == 'conflicted'; summary['current_revision'] == current_revision; summary['conflicts'] == [{'index': 0, 'status': 'conflict', 'conflicting_paths': ('/projects/piclaw.md',)}, {'index': 1, 'status': 'clean', 'conflicting_paths': ()}, {'index': 2, 'status': 'clean', 'conflicting_paths': ()}, {'index': 3, 'status': 'clean', 'conflicting_paths': ()}]; blocked.status == 'error'; blocked.error_class == 'conflict'.

    @py-96e4b29042cd @python_test_status_reports_canonical_state_when_derived_index_is_empty @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestCatalogReference
    Scenario: test_status_reports_canonical_state_when_derived_index_is_empty
      Given the pinned Python reference fixtures and controlled inputs
      When empty_service.memory_status(flint); success_data(result); get_main_revision(service._deps.repo_paths).
      Then result.status == 'success'; data['repo_revision'] == revision; data['index_revision'] == ''; data['index_stale'] is True; data['visible_concepts'] == 2; result.repo_revision == revision.

    @py-5aef6eaa1153 @python_test_streamable_http_delivers_subscribed_notifications_after_reconnect @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestCatalogReference
    Scenario: test_streamable_http_delivers_subscribed_notifications_after_reconnect
      Given the pinned Python reference fixtures and controlled inputs
      When _server_for(service, service_config); control.request( "DELETE", "/mcp", headers={ "Authorization": "Bearer smith-token", "MCP-Protocol-Version": "2025-03-26", "Mcp-Session-Id":….
      Then session_id is not None; subscribed.status == 200; subscribed_payload['result'] == {}; stream.status == 200; stream.getheader('Content-Type') == 'text/event-stream'; stream.readline() starts with b': connected'.

    @py-f66614d9be30 @python_test_streamable_http_explains_invalid_protocol_version @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestCatalogReference
    Scenario: test_streamable_http_explains_invalid_protocol_version
      Given the pinned Python reference fixtures and controlled inputs
      When _server_for(service, service_config); headers.append(f"MCP-Protocol-Version: {version}"); headers.append(f"Content-Length: {len(body)}").
      Then b'400 Bad Request' in response_headers; b'Content-Type: application/json' in response_headers; payload['error'] == expected_error; payload['expected'] == '2025-03-26'; payload['supported'] == ['2025-03-26', '2024-11-05']; payload['received'] == version.

    @py-7eea25a29054 @python_test_streamable_http_reuses_post_connection_and_preserves_session @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestCatalogReference
    Scenario: test_streamable_http_reuses_post_connection_and_preserves_session
      Given the pinned Python reference fixtures and controlled inputs
      When _server_for(service, service_config); replacement.request( "DELETE", "/mcp", headers={ "Authorization": "Bearer smith-token", "MCP-Protocol-Version": "2025-03-26", "Mcp-Session-….
      Then initialized.status == 200; initialized.will_close is False; session_id is not None; payload['result']['serverInfo'] == {'name': 'memento', 'version': __version__}; payload['result']['capabilities'] == {'tools': {}, 'resources': {'subscribe': True, 'listChanged': True}, 'logging': {}}; listed.status == 200.

    @py-fe236a638649 @python_test_tool_discovery_surfaces_and_catalog_resources @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestCatalogReference
    Scenario: test_tool_discovery_surfaces_and_catalog_resources
      Given the pinned Python reference fixtures and controlled inputs
      When _server_for( service, service_config.model_copy(update={"mcp": MCPConfig(tool_surface=surface)}), ); _server_for(service, service_config); asyncio.run(server.resource_catalog()).
      Then len(tools) == count; 'operations' in catalog; [item['operation'] for item in catalog['operations']] == ['help', 'status', 'search', 'read', 'inventory', 'asset_stage_begin', 'asset_stage_status', 'asset_get', 'execute']; any((item['operation'] == 'proposal_apply' for item in catalog['execute_only_operations'])); any((item['operation'] == 'compare_manifest' for item in catalog['execute_only_operations'])); any((item['operation'] == 'asset_metadata' for item in catalog['execute_only_operations'])).

    @py-20679ee7b9a4 @python_test_trash_graph_view_filters_original_acl @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestCatalogReference
    Scenario: test_trash_graph_view_filters_original_acl
      Given the pinned Python reference fixtures and controlled inputs
      When success_data( service.memory_trash( smith, path="/projects/piclaw.md", expected_revision=get_main_revision(service._deps.repo_paths), idemp…; graph.overview(policy=service._policy(smith)); graph.overview(policy=service._policy(smith), include_trash=True).
      Then graph is not None; trash.metrics.memory_count == normal.metrics.memory_count + 1; restricted.metrics.memory_count == 1; entries[0]['path'] == '/trash/projects/piclaw.md'.

    @py-8667ecc6d7f9 @python_test_trash_move_indexes_destination_before_lexically_later_source @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestCatalogReference
    Scenario: test_trash_move_indexes_destination_before_lexically_later_source
      Given the pinned Python reference fixtures and controlled inputs
      When success_data( service.memory_create( smith, path="/projects/zz.md", concept_type="project", title="Lexical", body="body", expected_revision…; success_data( service.memory_trash( smith, path="/projects/zz.md", expected_revision=get_main_revision(service._deps.repo_paths), idempoten…; success_data( service.memory_restore( smith, path="/trash/projects/zz.md", expected_revision=get_main_revision(service._deps.repo_paths), i….
      Then success_data(service.memory_search(smith, query='Lexical'))['results'][0]['path'] == '/projects/zz.md'.

    @py-03634d5bcfea @python_test_trash_restore_purge_retains_history_and_original_permissions @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestCatalogReference
    Scenario: test_trash_restore_purge_retains_history_and_original_permissions
      Given the pinned Python reference fixtures and controlled inputs
      When get_main_revision(paths); (paths.current_dir / "projects/piclaw.md").read_bytes(); success_data( service.memory_trash( smith, path="/projects/piclaw.md", expected_revision=revision, idempotency_key="trash-piclaw", ) ).
      Then first['destination'] == '/trash/projects/piclaw.md'; (paths.current_dir / 'trash/projects/piclaw.md').read_bytes() == original; not (paths.current_dir / 'projects/piclaw.md').exists(); success_data(service.memory_trash(smith, path='/projects/piclaw.md', expected_revision=revision, idempotency_key='trash-piclaw'))['replayed']; service.memory_read(narrow, id_or_path='/trash/projects/piclaw.md').status == 'error'; service.memory_restore(narrow, path='/trash/projects/

    @py-930874159404 @python_test_typed_reference_failure_after_commit_preserves_operation @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestCatalogReference
    Scenario: test_typed_reference_failure_after_commit_preserves_operation
      Given the pinned Python reference fixtures and controlled inputs
      When success_data( service.memory_execute( smith, plan={ "operations": [ { "op": "create", "save_as": "written", "args": { "path": "/projects/ty….
      Then result['stopped']; 'operation 2 (asset_get)' in result['stop_reason']; result['revisions'][0]['operation_id']; result['revisions'][0]['repo_revision'] == get_main_revision(service._deps.repo_paths); success_data(service.memory_operation_get(smith, idempotency_key='typed-ref-commit'))['final_state'] == 'committed'.

    @py-9ba0c7e3143d @python_test_typed_reference_from_eof_is_rejected_without_restarting_download @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestCatalogReference
    Scenario: test_typed_reference_from_eof_is_rejected_without_restarting_download
      Given the pinned Python reference fixtures and controlled inputs
      When service.memory_execute( smith, plan={ "operations": [ { "op": "asset_get", "save_as": "end", "args": { "id_or_path": path, "asset_kind": "d….
      Then result.status == 'error'; 'operation 2 (asset_get)' in result.message and 'args.offset' in result.message.

    @py-50e9e93a20f9 @python_test_typed_references_pass_direct_mcp_plan_boundary @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestCatalogReference
    Scenario: test_typed_references_pass_direct_mcp_plan_boundary
      Given the pinned Python reference fixtures and controlled inputs
      When _server_for(service, service_config); monkeypatch.setattr(server, "_context", lambda: smith); asyncio.run( server.tool_memory_execute( plan={ "operations": [ { "op": "asset_get", "save_as": "first", "args": { "id_or_path": path, "ass….
      Then result['status'] == 'success'; result['data']['trace'][1]['status'] == 'success'; result['data']['trace'][1]['data']['file']['offset'] == 2; result['data']['trace'][1]['data']['file']['next_offset'] is None.

    @py-0fb4f5d003c2 @python_test_unavailable_historical_base_does_not_hide_queue @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestCatalogReference
    Scenario: test_unavailable_historical_base_does_not_hide_queue
      Given the pinned Python reference fixtures and controlled inputs
      When _continuity_proposal(service, flint); connection.execute( "UPDATE proposals SET base_revision=?,status='stale' WHERE proposal_id=?", (base, proposal["proposal_id"]), ); connection.commit().
      Then success_data(service.memory_status(smith))['proposal_backlog'] == 1; listed[0]['status'] == 'conflicted'; view['base_revision'] == base; view['changes'] == proposal['changes']; view['conflicts'][0]['reason'] starts with 'base_revision_unavailable'; result.status == 'error' and result.error_class == 'conflict'.

    @py-c5636f092251 @python_test_unrelated_commit_keeps_proposal_reviewable_and_audited @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestCatalogReference
    Scenario: test_unrelated_commit_keeps_proposal_reviewable_and_audited
      Given the pinned Python reference fixtures and controlled inputs
      When _continuity_proposal(service, flint); _advance_unrelated(service, smith); success_data(service.memory_status(smith)).
      Then state['proposal_backlog'] == 1; [p['proposal_id'] for p in queue] == [proposal['proposal_id']]; queue[0]['status'] == 'needs_rebase'; view['conflicts'][0]['status'] == 'clean'; view['history'][0]['action'] == 'repository_advanced'; view['history'][0]['repo_revision'] == revision.

    @py-1706a62736f0 @python_test_worker_commit_reconciliation_and_shutdown @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestCatalogReference
    Scenario: test_worker_commit_reconciliation_and_shutdown
      Given the pinned Python reference fixtures and controlled inputs
      When _server_for(service, service_config); monkeypatch.setattr(server, "_context", lambda: smith); threading.Event().
      Then the result, state transitions, and error boundary match the captured behavior

    @py-f514bf70de47 @python_test_worker_rebase_replay_and_concurrent_applies @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestCatalogReference
    Scenario: test_worker_rebase_replay_and_concurrent_applies
      Given the pinned Python reference fixtures and controlled inputs
      When _server_for(service, service_config); _advance_unrelated(service, smith, key="second-target"); _continuity_proposal(service, flint).
      Then sorted((r.status for r in results)) == ['error', 'success']; success_data(service.memory_status(smith))['proposal_backlog'] == 1; all((r.status == 'success' for r in rebases)); rebases[0].operation_id == rebases[1].operation_id; sorted((success_data(r)['replayed'] for r in rebases)) == [False, True]; success_data(service.memory_operation_get(flint, idempotency_key='concurrent-rebase'))['final_state'] == 'committed'.
