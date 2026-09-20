Feature: proposals/control and assets

  The scenarios capture Python Memento behavior at 7f29e8b003557f0105f47ed353b7f65a33619456.
  Rules retain source-module traceability while features group related user behavior.

  Rule: Behavior captured from test_control_plane.py

    @py-2f2376db6fde @python_test_control_db_migrations_enable_wal_and_v1_tables @go_TestPythonControlMigrationFixtures
    Scenario: test_control_db_migrations_enable_wal_and_v1_tables
      Given the pinned Python reference fixtures and controlled inputs
      When control_connection.execute("PRAGMA journal_mode").fetchone(); control_connection.execute("PRAGMA foreign_keys").fetchone(); control_connection.execute( "SELECT name FROM sqlite_master WHERE type = 'table'" ).fetchall().
      Then mode.lower() == 'wal'; foreign_keys == 1; {'operations', 'proposal_assets', 'proposals', 'scheduler_runs', 'service_state', 'dream_signals', 'staged_assets', 'asset_upload_tickets', 'proposal_events'} <= tables; schema_version == '10'.

    @py-e411df339201 @python_test_empty_repository_bootstrap_materializes_and_recovers @go_TestPythonControlMigrationFixtures
    Scenario: test_empty_repository_bootstrap_materializes_and_recovers
      Given the pinned Python reference fixtures and controlled inputs
      When bootstrap_repository(paths); connect_control_db(tmp_path / "empty-control.sqlite"); migrate_control_db(connection).
      Then result.changed_paths == (); paths.current_dir.is_dir(); recovered == (); paths.current_dir.is_dir().

    @py-59d07e7ab2d9 @python_test_idempotency_replays_per_principal_and_rejects_payload_conflicts @go_TestPythonControlMigrationFixtures
    Scenario: test_idempotency_replays_per_principal_and_rejects_payload_conflicts
      Given the pinned Python reference fixtures and controlled inputs
      When create_operation(control_connection, request); create_operation( control_connection, OperationRequest( op_id="op-2", principal="flint", idempotency_key="idem-1", tool_name="memory_patch"….
      Then first.op_id == replay.op_id; replay.state is OperationState.QUEUED; other_principal.op_id == 'op-2'. expects create_operation( control_connection, OperationRequest( op_id="op-3", principal="smith", idempotency_key="idem-1", tool_name="memory_patch"… raises IdempotencyConflictError.

    @py-c816f6c9b3c8 @python_test_materialize_ignores_persisted_repository_hooks @go_TestPythonControlMigrationFixtures
    Scenario: test_materialize_ignores_persisted_repository_hooks
      Given the pinned Python reference fixtures and controlled inputs
      When seed.mkdir(); (seed / "memory.md").write_text("# Memory\n", encoding="utf-8"); bootstrap_repository(paths, seed).
      Then result.path.joinpath('memory.md').read_text(encoding='utf-8') == '# Memory\n'.

    @py-36f83caaf941 @python_test_path_commit_timestamp_comes_from_git_history @go_TestPythonControlMigrationFixtures
    Scenario: test_path_commit_timestamp_comes_from_git_history
      Given the pinned Python reference fixtures and controlled inputs
      When get_path_commit_timestamp( repo_paths, revision=get_main_revision(repo_paths), repository_path="/instances/smith.md", ); datetime.fromisoformat(timestamp).
      Then parsed.tzinfo is not None. expects get_path_commit_timestamp( repo_paths, revision=get_main_revision(repo_paths), repository_path="/instances/../secret.md", ) raises GitError matching "invalid repository path".

    @py-38c8091c0bd4 @python_test_startup_recovery_classifies_publication_after_crash @go_TestPythonControlMigrationFixtures
    Scenario: test_startup_recovery_classifies_publication_after_crash
      Given the pinned Python reference fixtures and controlled inputs
      When get_operation(control_connection, "op-crash"); TransactionManager(control_connection, repo_paths).recover_startup().
      Then pending.state is OperationState.RUNNING; recovery[0].classification == 'published'; get_operation(control_connection, 'op-crash').state is OperationState.SUCCEEDED; (repo_paths.current_dir / 'instances' / 'smith.md').read_text(encoding='utf-8') ends with 'Published before crash.\n'. expects manager.apply( TransactionRequest( operation=OperationRequest( op_id="op-crash", principal="smith", idempotency_key="idem-crash", tool_name… raises CheckpointError.

    @py-9f9c9b9e6f55 @python_test_startup_recovery_uses_committed_diff_not_worktree_scan @go_TestPythonControlMigrationFixtures
    Scenario: test_startup_recovery_uses_committed_diff_not_worktree_scan
      Given the pinned Python reference fixtures and controlled inputs
      When TransactionManager(control_connection, repo_paths).recover_startup(); get_operation(control_connection, "op-crash-diff").
      Then recovered[0].classification == 'published'; payload == {'changed_paths': ['/instances/smith.md']}; not (repo_paths.current_dir / 'instances' / 'ignored.md').exists(). expects manager.apply( TransactionRequest( operation=OperationRequest( op_id="op-crash-diff", principal="smith", idempotency_key="idem-crash-diff",… raises CheckpointError.

    @py-c2eb4651d12a @python_test_transaction_pipeline_commits_and_materializes_current @go_TestPythonControlMigrationFixtures
    Scenario: test_transaction_pipeline_commits_and_materializes_current
      Given the pinned Python reference fixtures and controlled inputs
      When get_main_revision(repo_paths); manager.apply( TransactionRequest( operation=OperationRequest( op_id="op-success", principal="smith", idempotency_key="idem-success", tool_….
      Then result.replayed is False; result.base_revision == expected_revision; result.changed_paths == ('/instances/smith.md',); (repo_paths.current_dir / 'instances' / 'smith.md').read_text(encoding='utf-8') ends with 'Updated.\n'; get_operation(control_connection, 'op-success').state is OperationState.SUCCEEDED.

    @py-2079a5320d07 @python_test_transaction_pipeline_rejects_stale_revision @go_TestPythonControlMigrationFixtures
    Scenario: test_transaction_pipeline_rejects_stale_revision
      Given the pinned Python reference fixtures and controlled inputs
      When checks get_operation(control_connection, 'op-stale').state is OperationState.CONFLICT. expects manager.apply( TransactionRequest( operation=OperationRequest( op_id="op-stale", principal="smith", idempotency_key="idem-stale", tool_name… raises TransactionConflictError.
      Then the result, state transitions, and error boundary match the captured behavior

    @py-2d8cc9d860f5 @python_test_transaction_pipeline_retries_failed_operation_with_persisted_id @go_TestPythonControlMigrationFixtures
    Scenario: test_transaction_pipeline_retries_failed_operation_with_persisted_id
      Given the pinned Python reference fixtures and controlled inputs
      When get_main_revision(repo_paths); manager.apply( TransactionRequest( operation=OperationRequest( op_id="op-failed-retry-request", principal="smith", idempotency_key="idem-re….
      Then get_operation(control_connection, 'op-failed-first').state is OperationState.FAILED; result.operation.op_id == 'op-failed-first'; result.operation.state is OperationState.SUCCEEDED. expects manager.apply( TransactionRequest( operation=OperationRequest( op_id="op-failed-first", principal="smith", idempotency_key="idem-retry-fail… raises RuntimeError matching "transient failure"; get_operation(control_connection, "op-failed-retry-request") raises KeyError.

    @py-64620d46b08f @python_test_transaction_pipeline_stages_exact_paths_only @go_TestPythonControlMigrationFixtures
    Scenario: test_transaction_pipeline_stages_exact_paths_only
      Given the pinned Python reference fixtures and controlled inputs
      When manager.apply( TransactionRequest( operation=OperationRequest( op_id="op-exact", principal="smith", idempotency_key="idem-exact", tool_name….
      Then result.changed_paths == ('/instances/smith.md',); not (repo_paths.current_dir / 'instances' / 'ignored.md').exists().

    @py-372a5a46e62d @python_test_unpublished_crashes_never_recover_as_success @go_TestPythonControlMigrationFixtures
    Scenario: test_unpublished_crashes_never_recover_as_success
      Given the pinned Python reference fixtures and controlled inputs
      When get_main_revision(repo_paths); TransactionManager(control_connection, repo_paths).recover_startup(); TransactionManager(control_connection, repo_paths).apply( request, lambda root: _write_file(root, "/instances/smith.md", "new content") ).
      Then recovery[0].classification == 'retryable'; get_operation(control_connection, 'unpublished').state is not OperationState.SUCCEEDED; get_main_revision(repo_paths) == revision; not retry.replayed; retry.result_revision != revision. expects manager.apply(request, lambda root: _write_file(root, "/instances/smith.md", "new content")) raises CheckpointError.

    @py-9cee62241a77 @python_test_writer_lease_reports_contention @go_TestPythonControlMigrationFixtures
    Scenario: test_writer_lease_reports_contention
      Given the pinned Python reference fixtures and controlled inputs
      When acquire_writer_lease(lock_path, owner="writer-a"); acquire_writer_lease(lock_path, owner="writer-b"). expects acquire_writer_lease(lock_path, owner="writer-b") raises WriterLeaseError matching "writer-a".
      Then the result, state transitions, and error boundary match the captured behavior

  Rule: Behavior captured from test_operations.py

    @py-de660cc1f26e @python_test_backup_rejects_destination_inside_state_root @go_TestOperationGetReference
    Scenario: test_backup_rejects_destination_inside_state_root
      Given the pinned Python reference fixtures and controlled inputs
      When build_runtime(config_path, bootstrap_seed=seed); create_backup(runtime, runtime.paths.root / "backups" / "latest"). expects create_backup(runtime, runtime.paths.root / "backups" / "latest") raises ValueError matching "backup destination must be outside repository root_path".
      Then the result, state transitions, and error boundary match the captured behavior

    @py-7c21dac2d72b @python_test_backup_restore_and_audit @go_TestOperationGetReference
    Scenario: test_backup_restore_and_audit
      Given the pinned Python reference fixtures and controlled inputs
      When build_runtime(config_path, bootstrap_seed=seed); create_backup(runtime, backup_dir); load_service_config(config_path).
      Then manifest.repo_revision == get_main_revision(runtime.paths.repo_paths); restored['rebuild_derived'] is True; main(['--config', str(config_path), 'audit']) == 0; payload['ok'] is True.

    @py-e687289248d7 @python_test_backup_restore_rejects_manifest_revision_mismatch @go_TestOperationGetReference
    Scenario: test_backup_restore_rejects_manifest_revision_mismatch
      Given the pinned Python reference fixtures and controlled inputs
      When build_runtime(config_path, bootstrap_seed=seed); create_backup(runtime, backup_dir); manifest_path.read_text(encoding="utf-8"). expects restore_backup(load_service_config(config_path), backup_dir) raises ValueError matching "backup manifest revision does not match archived main".
      Then the result, state transitions, and error boundary match the captured behavior

    @py-239b7ee150e2 @python_test_cli_prometheus_output_is_clean_stdout @go_TestOperationGetReference
    Scenario: test_cli_prometheus_output_is_clean_stdout
      Given the pinned Python reference fixtures and controlled inputs
      When build_runtime(config_path, bootstrap_seed=seed); render_prometheus_text(runtime); runtime.close().
      Then main(['--config', str(config_path), 'status', '--format', 'prometheus']) == 0; text == expected; '"event": "status_rendered"' not in text; logs.getvalue().count('"event": "runtime_closed"') == 1.

    @py-6e49361e52b6 @python_test_cli_status_and_rebuild_index @go_TestOperationGetReference
    Scenario: test_cli_status_and_rebuild_index
      Given the pinned Python reference fixtures and controlled inputs
      When build_runtime(config_path, bootstrap_seed=seed).close(); main(["--config", str(config_path), "status"]); output.getvalue().
      Then main(['--config', str(config_path), 'status']) == 0; payload['visible_concepts'] == 2; '"event": "command_completed"' in logs.getvalue(); main(['--config', str(config_path), 'rebuild-index']) == 0; rebuilt['parity_matches'] is True; '"event": "command_completed"' in logs.getvalue().

    @py-bbc01a67f402 @python_test_compose_example_uses_env_file_and_example_env_lists_required_tokens @go_TestOperationGetReference
    Scenario: test_compose_example_uses_env_file_and_example_env_lists_required_tokens
      Given the pinned Python reference fixtures and controlled inputs
      When (root / "compose.example.yaml").read_text(encoding="utf-8"); (root / "examples/memento.env.example").read_text(encoding="utf-8").
      Then 'env_file:' in compose; '- .env' in compose; 'MEMENTO_ADMIN_MASTER_KEY=' in env_example; 'MEMENTO_TOKEN_SANDBOX_BOOTSTRAP=' in env_example; 'MEMENTO_TOKEN_WORK_AGENT_BOOTSTRAP=' in env_example; 'Bootstrap/recovery credentials' in env_example.

    @py-d1cf250c12ae @python_test_config_loading_and_composition_root @go_TestOperationGetReference
    Scenario: test_config_loading_and_composition_root
      Given the pinned Python reference fixtures and controlled inputs
      When load_service_config(config_path); build_runtime(config_path, bootstrap_seed=seed); runtime.status_snapshot().
      Then isinstance(config, ServiceConfig); status['visible_concepts'] == 2; status['repo_revision'] == get_main_revision(runtime.paths.repo_paths).

    @py-de170a2a1cb7 @python_test_diskstation_progressive_embeddings_are_persistent_and_single_threaded @go_TestOperationGetReference
    Scenario: test_diskstation_progressive_embeddings_are_persistent_and_single_threaded
      Given the pinned Python reference fixtures and controlled inputs
      When (root / "deploy/diskstation.compose.yaml").read_text(encoding="utf-8"); (root / "tools/release_deploy.py").read_text(encoding="utf-8"); (root / "deploy/diskstation.config.example.json").read_text().
      Then semantic['max_batch_size'] == 1; semantic['progressive_enabled'] is True; semantic['progressive_startup_delay_seconds'] == 120; semantic['progressive_interactive_idle_seconds'] == 15; semantic['progressive_delay_seconds'] == 30; semantic['progressive_cpu_busy_limit_percent'] == 75.

    @py-02679489aa89 @python_test_embedding_refresh_paths_exclude_asset_and_runtime_changes @go_TestOperationGetReference
    Scenario: test_embedding_refresh_paths_exclude_asset_and_runtime_changes
      Given the pinned Python reference fixtures and controlled inputs
      When checks embedding_refresh_paths(('/skills/demo.md', '/.assets/id/skill/1.0.0.json', '/.assets/id/skill/1.0.0.zip', '/.gitattributes', '/projects/other.md')) == ('/skills/demo.md', '/projects/other.md').
      Then the result, state transitions, and error boundary match the captured behavior

    @py-b78d75b0cb15 @python_test_graceful_server_drain @go_TestOperationGetReference
    Scenario: test_graceful_server_drain
      Given the pinned Python reference fixtures and controlled inputs
      When build_runtime(config_path, bootstrap_seed=seed); monkeypatch.setattr(type(runtime), "build_server", lambda self: server); monkeypatch.setattr(cli_module, "run_server", fake_run_server).
      Then asyncio.run(_serve(runtime, host='127.0.0.1', port=8000, endpoint='/mcp', logger=logger)) == 0; server.closed is True.

    @py-a887b1154701 @python_test_metrics_renderer @go_TestOperationGetReference
    Scenario: test_metrics_renderer
      Given the pinned Python reference fixtures and controlled inputs
      When build_runtime(config_path, bootstrap_seed=seed); runtime.close().
      Then 'memento_repo_revision_info' in text.

    @py-a21f3f1c30b3 @python_test_restore_refuses_active_writer @go_TestOperationGetReference
    Scenario: test_restore_refuses_active_writer
      Given the pinned Python reference fixtures and controlled inputs
      When load_service_config(seeded_root[0]); config_path.write_text(config.model_dump_json()); build_config_runtime(config_path).
      Then runtime.paths.control_db.exists(). expects restore_backup(config, tmp_path / "backup") raises WriterLeaseError.

    @py-63692e1a04cc @python_test_restore_rejects_backup_source_inside_state_root @go_TestOperationGetReference
    Scenario: test_restore_rejects_backup_source_inside_state_root
      Given the pinned Python reference fixtures and controlled inputs
      When build_runtime(config_path, bootstrap_seed=seed); nested_backup.mkdir(parents=True, exist_ok=True). expects restore_backup(load_service_config(config_path), nested_backup) raises ValueError matching "backup source must be outside repository root_path".
      Then the result, state transitions, and error boundary match the captured behavior

    @py-f91027a16d73 @python_test_restore_requires_mandatory_checksums @go_TestOperationGetReference
    Scenario: test_restore_requires_mandatory_checksums
      Given the pinned Python reference fixtures and controlled inputs
      When load_service_config(seeded_root[0]); backup.mkdir(); (backup / "manifest.json").write_text( json.dumps({"schema_version": 1, "repo_revision": "unused", "files": {}}) ). expects restore_backup(config, backup) raises ValueError matching "required checksums".
      Then the result, state transitions, and error boundary match the captured behavior

    @py-cab450579e96 @python_test_runtime_close_closes_sqlite_connection @go_TestOperationGetReference
    Scenario: test_runtime_close_closes_sqlite_connection
      Given the pinned Python reference fixtures and controlled inputs
      When build_runtime(config_path, bootstrap_seed=seed); runtime.close(). expects runtime.status_snapshot() raises RuntimeClosedError; runtime.control_connection.execute("SELECT 1") raises sqlite3.ProgrammingError.
      Then the result, state transitions, and error boundary match the captured behavior

    @py-4f6689ce0bc6 @python_test_runtime_loads_and_closes_needle_router @go_TestOperationGetReference
    Scenario: test_runtime_loads_and_closes_needle_router
      Given the pinned Python reference fixtures and controlled inputs
      When config_path.read_text(encoding="utf-8"); config_path.write_text(json.dumps(config), encoding="utf-8"); monkeypatch.setattr("memento.app.NeedleFfiLibrary", FakeLibrary).
      Then runtime.status_snapshot()['needle_router']['loaded'] is True; fake_router.closed is True.

    @py-b0ec8f32b2e4 @python_test_runtime_server_uses_writable_state_log @go_TestOperationGetReference
    Scenario: test_runtime_server_uses_writable_state_log
      Given the pinned Python reference fixtures and controlled inputs
      When monkeypatch.setenv("MEMENTO_TOKEN_SMITH", "test-token"); build_runtime(config_path, bootstrap_seed=seed); runtime.build_server().
      Then server._umcp_log_file == expected_log; expected_log.parent.is_dir(); expected_log.parent.stat().st_mode & 128.

    @py-00eb0d9be584 @python_test_structured_logging_redacts_secrets @go_TestOperationGetReference
    Scenario: test_structured_logging_redacts_secrets
      Given the pinned Python reference fixtures and controlled inputs
      When logger.info("test", authorization="Bearer secret", nested={"token": "x", "ok": 1}); stream.getvalue().
      Then payload['authorization'] == '<redacted>'; payload['nested']['token'] == '<redacted>'; payload['nested']['ok'] == 1.

    @py-519bd9251d22 @python_test_systemd_timers_are_disabled_by_default_for_exclusive_maintenance @go_TestOperationGetReference
    Scenario: test_systemd_timers_are_disabled_by_default_for_exclusive_maintenance
      Given the pinned Python reference fixtures and controlled inputs
      When timer.read_text(encoding="utf-8").
      Then '[Timer]' in content; 'WantedBy=timers.target' not in content; 'exclusive lease' in content.

    @py-429e91866d7f @python_test_systemd_units_use_installed_console_script @go_TestOperationGetReference
    Scenario: test_systemd_units_use_installed_console_script
      Given the pinned Python reference fixtures and controlled inputs
      When unit.read_text(encoding="utf-8"); backup_unit.read_text(encoding="utf-8").
      Then 'ExecStart=/opt/memento/.venv/bin/memento-serve ' in content; 'NoNewPrivileges=true' in content; 'ProtectSystem=strict' in content; 'EnvironmentFile=-/etc/memento/memento.env' in content; 'StateDirectory=memento' in content; 'ReadWritePaths=/var/lib/memento' in service_unit.read_text(encoding='utf-8').

  Rule: Behavior captured from test_proposal_assets.py

    @py-d42b91c6870f @python_test_create_proposal_rolls_back_when_asset_insert_fails @go_TestProposalSubmitReference @go_TestWorktreeAssetReference
    Scenario: test_create_proposal_rolls_back_when_asset_insert_fails
      Given the pinned Python reference fixtures and controlled inputs
      When checks list_proposal_assets(connection, proposal_id='proposal-rollback') == (). expects create_proposal( connection, proposal_id="proposal-rollback", author_principal="author", client_instance_id=None, base_revision="abc123", i… raises sqlite3.IntegrityError; get_proposal(connection, "proposal-rollback") raises KeyError.
      Then the result, state transitions, and error boundary match the captured behavior

    @py-08011a339581 @python_test_create_proposal_stores_assets_atomically @go_TestProposalSubmitReference @go_TestWorktreeAssetReference
    Scenario: test_create_proposal_stores_assets_atomically
      Given the pinned Python reference fixtures and controlled inputs
      When create_proposal( connection, proposal_id="proposal-1", author_principal="author", client_instance_id="client-1", base_revision="abc123", in…; get_proposal_asset(connection, "proposal-1", "asset-1"); connection.execute("DELETE FROM proposals WHERE proposal_id = ?", ("proposal-1",)).
      Then created.status is ProposalStatus.SUBMITTED; fetched.blob_bytes == b'zip-bytes'; fetched.manifest == manifest; list_proposal_assets(connection, proposal_id='proposal-1') == (fetched, get_proposal_asset(connection, 'proposal-1', 'asset-2')); list_proposal_assets(connection, concept_path='/skills/demo.md') == (fetched, get_proposal_asset(connection, 'proposal-1', 'asset-2')); list_proposal_assets(connect

    @py-061d9341936c @python_test_migrate_v5_skill_pack_proposals_into_generic_assets @go_TestProposalSubmitReference @go_TestWorktreeAssetReference
    Scenario: test_migrate_v5_skill_pack_proposals_into_generic_assets
      Given the pinned Python reference fixtures and controlled inputs
      When connect_control_db(path); connection.execute( "SELECT value FROM service_state WHERE key = 'schema_version'" ).fetchone().
      Then proposal.intent == 'Attach skill asset demo 1.0.0'; proposal.status is ProposalStatus.APPROVED; proposal.reviewed_by == 'curator'; proposal.review_comment == 'looks good'; proposal.created_at == created_at; proposal.updated_at == updated_at.

    @py-af71ea92db02 @python_test_migrate_v5_skips_unfeasible_rows_when_proposal_id_is_taken @go_TestProposalSubmitReference @go_TestWorktreeAssetReference
    Scenario: test_migrate_v5_skips_unfeasible_rows_when_proposal_id_is_taken
      Given the pinned Python reference fixtures and controlled inputs
      When connect_control_db(path); connection.execute( "INSERT INTO service_state(key, value, updated_at) VALUES('schema_version', '5', '2026-07-17T00:00:00Z')" ).
      Then proposal.intent == 'memory_patch'; list_proposal_assets(connection, proposal_id='shared-id') == ().
