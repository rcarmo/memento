Feature: models/semantic and embeddings

  The scenarios capture Python Memento behavior at 7f29e8b003557f0105f47ed353b7f65a33619456.
  Rules retain source-module traceability while features group related user behavior.

  Rule: Behavior captured from test_semantic.py

    @py-387a02dd622f @python_test_asset_only_revision_advances_ready_embeddings_without_recomputing @go_TestServiceSemanticSearchSelection
    Scenario: test_asset_only_revision_advances_ready_embeddings_without_recomputing
      Given the pinned Python reference fixtures and controlled inputs
      When DerivedIndex( db_path, semantic_config=semantic_config(), embedding_client=embedder, ).rebuild(semantic_repo_paths.current_dir, repo_revisi…; embedder.seen_texts.clear(); asset.parent.mkdir(parents=True).
      Then embedder.seen_texts == []; status.ready is True; status.embedding_revision == 'rev-assets'; revisions == [('rev-assets',)].

    @py-5a5d05a4ce9c @python_test_ctypes_wrapper_abi_lifecycle_info_embed_batch_cancel_and_errors @go_TestServiceSemanticSearchSelection
    Scenario: test_ctypes_wrapper_abi_lifecycle_info_embed_batch_cancel_and_errors
      Given the pinned Python reference fixtures and controlled inputs
      When build_rust_cdylib("memento-ffi", "libmemento_ffi"); model_path.write_bytes(synthetic_model_bytes()); token.cancel().
      Then library.path == ffi_library_path; library.vector_cosine((1.0, 0.0), (0.5, 0.0)) == pytest.approx(1.0); token.pointer is not None; info.abi_version == 1; info.dimensions == 4; model_info.dimensions == 4. expects model.embed("hello", cancelled=lambda: True) raises FfiCancelledError; closed_model.info() raises FfiClosedError; closed_model.embed("hello") raises FfiClosedError.

    @py-e05538a2833b @python_test_ctypes_wrapper_rejects_malformed_model_headers @go_TestServiceSemanticSearchSelection
    Scenario: test_ctypes_wrapper_rejects_malformed_model_headers
      Given the pinned Python reference fixtures and controlled inputs
      When build_rust_cdylib("memento-ffi", "libmemento_ffi"); model_path.write_bytes(synthetic_model_bytes(header_overrides={field_index: value})). expects library.load_model(model_path) raises FfiModelError matching "message".
      Then the result, state transitions, and error boundary match the captured behavior

    @py-aeb0c4ddce71 @python_test_embedding_text_is_truncated_to_configured_character_limit @go_TestServiceSemanticSearchSelection
    Scenario: test_embedding_text_is_truncated_to_configured_character_limit
      Given the pinned Python reference fixtures and controlled inputs
      When semantic_config(max_input_chars=32); get_main_revision(semantic_repo_paths); index.rebuild(semantic_repo_paths.current_dir, repo_revision=revision).
      Then embedder.seen_texts; all((len(text) <= 32 for text in embedder.seen_texts)).

    @py-038799913588 @python_test_failed_sqlite_extension_load_disables_further_extension_loading @go_TestServiceSemanticSearchSelection
    Scenario: test_failed_sqlite_extension_load_disables_further_extension_loading
      Given the pinned Python reference fixtures and controlled inputs
      When index._connect(); connection.execute("SELECT load_extension('/missing/extension')").fetchone(). expects connection.execute("SELECT load_extension('/missing/extension')").fetchone() raises sqlite3.OperationalError matching "not authorized".
      Then the result, state transitions, and error boundary match the captured behavior

    @py-bfbfdae87c2c @python_test_fake_embedder_full_rebuild_incremental_update_delete_and_model_invalidation @go_TestServiceSemanticSearchSelection
    Scenario: test_fake_embedder_full_rebuild_incremental_update_delete_and_model_invalidation
      Given the pinned Python reference fixtures and controlled inputs
      When semantic_config(); get_main_revision(semantic_repo_paths); index.rebuild(semantic_repo_paths.current_dir, repo_revision=revision).
      Then row == (4, 384, 384); row == ('ready', 'rev-2'); deleted == (0,); status.enabled is True; status.ready is False; status.embedding_revision == 'partial'.

    @py-be26cd6bef42 @python_test_hidden_best_match_cannot_change_visible_semantic_or_hybrid_scores @go_TestServiceSemanticSearchSelection
    Scenario: test_hidden_best_match_cannot_change_visible_semantic_or_hybrid_scores
      Given the pinned Python reference fixtures and controlled inputs
      When semantic_config(); index.rebuild( semantic_repo_paths.current_dir, repo_revision=get_main_revision(semantic_repo_paths) ); index.search( policy=visible_policy, query="red visible", search_mode=SearchMode.SEMANTIC, limit=10, ).
      Then [(item.path, round(item.score, 6)) for item in visible_semantic.results] == [(item.path, round(item.score, 6)) for item in visible_semantic_after.results]; [(item.path, round(item.score, 6)) for item in visible_hybrid.results] == [(item.path, round(item.score, 6)) for item in visible_hybrid_after.results]; '/secret/ghost.md' in [item.path for item in hidden_semantic.results]; visible_semantic_after.results[0].path != '/secret/ghost.md'.

    @py-88b7e20a237f @python_test_lexical_default_unchanged_and_hybrid_rrf_is_deterministic @go_TestServiceSemanticSearchSelection
    Scenario: test_lexical_default_unchanged_and_hybrid_rrf_is_deterministic
      Given the pinned Python reference fixtures and controlled inputs
      When semantic_config(); index.rebuild( semantic_repo_paths.current_dir, repo_revision=get_main_revision(semantic_repo_paths) ); index.search(policy=visible_policy, query="common", limit=10).
      Then [item.path for item in default_page.results] == [item.path for item in lexical_page.results]; [round(item.score, 6) for item in default_page.results] == [round(item.score, 6) for item in lexical_page.results]; expected[0] == '/projects/beta.md'; expected == [item.path for item in hybrid_runs[1].results]; expected == [item.path for item in hybrid_runs[2].results]; [round(item.score, 9) for item in hybrid_runs[0].results] == [round(item.score, 9) for item in hybrid_runs[1].results].

    @py-04c534241825 @python_test_memory_search_api_search_mode_and_status_warnings @go_TestServiceSemanticSearchSelection
    Scenario: test_memory_search_api_search_mode_and_status_warnings
      Given the pinned Python reference fixtures and controlled inputs
      When connect_control_db(tmp_path / "control.sqlite"); migrate_control_db(control); semantic_config( ffi_library_path="/tmp/libmemento_ffi.so", sqlite_extension_path="/definitely/missing/libmemento_sqlite_vector.so", defaul….
      Then status.status == 'success'; status.warnings; any(('sqlite_vector_extension_unavailable' in warning for warning in status.warnings)); status_data is not None; status_data['features']['semantic_search'] is True; status_data['readiness']['semantic_search']['ready'] is False.

    @py-0eb0cebb5a71 @python_test_model_info_revision_differs_for_same_basename_with_different_contents @go_TestServiceSemanticSearchSelection
    Scenario: test_model_info_revision_differs_for_same_basename_with_different_contents
      Given the pinned Python reference fixtures and controlled inputs
      When build_rust_cdylib("memento-ffi", "libmemento_ffi"); left_dir.mkdir(); right_dir.mkdir().
      Then left_info.model_id == right_info.model_id == 'synthetic.gte'; left_info.revision != right_info.revision.

    @py-d339629a4a50 @python_test_python_ctypes_wrapper_surfaces_vector_errors_and_sqlite_vector_matches_python_and_rust @go_TestServiceSemanticSearchSelection
    Scenario: test_python_ctypes_wrapper_surfaces_vector_errors_and_sqlite_vector_matches_python_and_rust
      Given the pinned Python reference fixtures and controlled inputs
      When build_rust_cdylib("memento-ffi", "libmemento_ffi"); build_rust_cdylib("memento-sqlite-vector", "libmemento_sqlite_vector"); pack_f32le(left).
      Then library.vector_cosine(left, right) == pytest.approx(expected, rel=1e-06); row is not None; row[0] == 1; row[1] == 4; row[2] == pytest.approx(expected, rel=1e-06); connection.execute('SELECT vector_is_valid(?), vector_dimensions(?)', (invalid, invalid)).fetchone() == (0, None). expects connection.execute("SELECT vector_cosine(?, ?)", (malformed, right_blob)).fetchone() raises sqlite3.OperationalError; connection.execute("SELECT vector_cosine(?, ?)", (nan_blob, right_blob)).fetchone() raises sqlite3.OperationalError; connection.execute( "SEL

    @py-0c1b609b8e45 @python_test_semantic_search_disabled_or_unavailable_falls_back_to_lexical @go_TestServiceSemanticSearchSelection
    Scenario: test_semantic_search_disabled_or_unavailable_falls_back_to_lexical
      Given the pinned Python reference fixtures and controlled inputs
      When get_main_revision(semantic_repo_paths); lexical_only.rebuild(semantic_repo_paths.current_dir, repo_revision=revision); lexical_only.search( policy=visible_policy, query="common", search_mode=SearchMode.LEXICAL ).
      Then [item.path for item in semantic_page.results] == [item.path for item in baseline.results]; [item.path for item in hybrid_page.results] == [item.path for item in baseline.results]; semantic_page.warnings and 'semantic_search_unavailable' in semantic_page.warnings[0]; hybrid_page.warnings and 'semantic_search_unavailable' in hybrid_page.warnings[0].

    @py-f240e6dd9915 @python_test_semantic_status_requires_embedding_rows_for_all_concepts @go_TestServiceSemanticSearchSelection
    Scenario: test_semantic_status_requires_embedding_rows_for_all_concepts
      Given the pinned Python reference fixtures and controlled inputs
      When get_main_revision(semantic_repo_paths); lexical.rebuild(semantic_repo_paths.current_dir, repo_revision=revision); connection.execute( "UPDATE index_state SET value = ? WHERE key = 'semantic_embedding_revision'", (revision,), ).
      Then status.ready is False; any(('4 of 4 embeddings not ready' in warning for warning in status.warnings)).

    @py-794b3b553cf0 @python_test_transaction_succeeds_when_embedding_update_degrades_but_lexical_advances @go_TestServiceSemanticSearchSelection
    Scenario: test_transaction_succeeds_when_embedding_update_degrades_but_lexical_advances
      Given the pinned Python reference fixtures and controlled inputs
      When connect_control_db(tmp_path / "control.sqlite"); migrate_control_db(control); semantic_config().
      Then result.result_revision == lexical.index_revision; [item.path for item in lexical.results] == ['/projects/beta.md']; semantic_status.ready is False; any(('semantic_embeddings_degraded' in warning for warning in semantic_status.warnings)); row is not None; row[0] == 'degraded'.

  Rule: Behavior captured from test_semantic_deferred.py

    @py-7901b01e67d9 @python_test_close_interrupts_database_retry_wait @go_TestModelsOffRuntimeSemanticWorker
    Scenario: test_close_interrupts_database_retry_wait
      Given the pinned Python reference fixtures and controlled inputs
      When _fast_policy(); worker.enqueue(tmp_path, "rev"); time.monotonic().
      Then wait_for(lambda: worker.state().pause_reason == 'database-busy', timeout_seconds=2); time.monotonic() - start < 0.5; not worker.state().alive; not worker.enqueue(tmp_path, 'rev').

    @py-24ed18c6dc0d @python_test_deferred_semantic_refresh_lags_and_catches_up @go_TestModelsOffRuntimeSemanticWorker
    Scenario: test_deferred_semantic_refresh_lags_and_catches_up
      Given the pinned Python reference fixtures and controlled inputs
      When semantic_config(); index.rebuild(semantic_bundle, repo_revision="rev-1"); index.semantic_status().
      Then index.get_state().index_revision == 'rev-1'; embedder.batch_sizes == []; status.ready is False; status.embedding_revision is None; status.ready is True; status.embedding_revision == 'rev-1'.

    @py-f426cbc7521b @python_test_embedding_refresh_worker_coalesces_latest_revision_and_close @go_TestModelsOffRuntimeSemanticWorker
    Scenario: test_embedding_refresh_worker_coalesces_latest_revision_and_close
      Given the pinned Python reference fixtures and controlled inputs
      When cast(DerivedIndex, fake_index); worker.close().
      Then worker.enqueue(tmp_path, 'rev-1') is True; fake_index.started.wait(timeout=1.0); worker.running is True; worker.enqueue(tmp_path, 'rev-2') is True; worker.enqueue(tmp_path, 'rev-3') is True; worker.wait_idle(timeout_seconds=2.0) is True.

    @py-5b056fde050b @python_test_failed_priority_path_does_not_block_progressive_queue @go_TestModelsOffRuntimeSemanticWorker
    Scenario: test_failed_priority_path_does_not_block_progressive_queue
      Given the pinned Python reference fixtures and controlled inputs
      When cast(DerivedIndex, fake_index); worker.close().
      Then wait_for(lambda: fake_index.path_calls == [('/.assets/file.json',), ('/valid.md',)], timeout_seconds=2.0); worker.state().last_error is None.

    @py-4ad13b65741b @python_test_model_revision_change_marks_persisted_embeddings_stale @go_TestModelsOffRuntimeSemanticWorker
    Scenario: test_model_revision_change_marks_persisted_embeddings_stale
      Given the pinned Python reference fixtures and controlled inputs
      When semantic_config(); first.rebuild(semantic_bundle, repo_revision="rev-1"); first.refresh_embeddings(semantic_bundle, repo_revision="rev-1").
      Then first.pending_embedding_paths(limit=10) == (); reopened.mark_model_stale() == 2; reopened.pending_embedding_paths(limit=10) == ('/projects/alpha.md', '/projects/beta.md').

    @py-99a636d90bff @python_test_pause_check_failure_does_not_drop_selected_work @go_TestModelsOffRuntimeSemanticWorker
    Scenario: test_pause_check_failure_does_not_drop_selected_work
      Given the pinned Python reference fixtures and controlled inputs
      When _fast_policy(); worker.close().
      Then wait_for(lambda: worker.state().pause_reason == 'error', timeout_seconds=2); worker.pending and worker.state().completed == 0; worker.wait_idle(timeout_seconds=3); index.path_calls == [('/manual.md',)]; worker.state().last_error is None.

    @py-fbda8894567b @python_test_pending_paths_skip_degraded_until_manual_or_content_change @go_TestModelsOffRuntimeSemanticWorker
    Scenario: test_pending_paths_skip_degraded_until_manual_or_content_change
      Given the pinned Python reference fixtures and controlled inputs
      When semantic_config(); index.rebuild(semantic_bundle, repo_revision="rev-1"); connection.execute( """ INSERT INTO concept_embeddings( concept_id,path,embedding_text_hash,model_id,dimensions,embedding_revision, status,….
      Then index.pending_embedding_paths(limit=10) == ('/projects/beta.md',).

    @py-2f66b2201bd9 @python_test_polling_database_lock_recovers_and_status_never_queries_sqlite @go_TestModelsOffRuntimeSemanticWorker
    Scenario: test_polling_database_lock_recovers_and_status_never_queries_sqlite
      Given the pinned Python reference fixtures and controlled inputs
      When _fast_policy(); worker.close().
      Then wait_for(lambda: worker.state().pause_reason == 'database-busy', timeout_seconds=2); worker.state().alive and worker.state().pending; 'database is locked' in worker.state().last_error or ''; worker.pending and worker.state().alive; index.polls == polls; worker.wait_idle(timeout_seconds=3).

    @py-1880b8223676 @python_test_progressive_worker_pauses_for_activity_load_and_pacing @go_TestModelsOffRuntimeSemanticWorker
    Scenario: test_progressive_worker_pauses_for_activity_load_and_pacing
      Given the pinned Python reference fixtures and controlled inputs
      When cast(DerivedIndex, fake_index); worker.close().
      Then wait_for(lambda: worker.state().pause_reason == 'startup', timeout_seconds=1.0); wait_for(lambda: worker.state().pause_reason == 'interactive', timeout_seconds=1.5); wait_for(lambda: worker.state().pause_reason == 'cpu', timeout_seconds=1.5); wait_for(lambda: fake_index.path_calls == [('/a.md',)], timeout_seconds=1.5); wait_for(lambda: worker.state().pause_reason == 'pacing', timeout_seconds=1.5); wait_for(lambda: fake_index.path_calls[-1] == ('/b.md',), timeout_seconds=1.5).

    @py-f33db94de747 @python_test_progressive_worker_resumes_pending_state_and_prioritizes_manual_paths @go_TestModelsOffRuntimeSemanticWorker
    Scenario: test_progressive_worker_resumes_pending_state_and_prioritizes_manual_paths
      Given the pinned Python reference fixtures and controlled inputs
      When cast(DerivedIndex, fake_index); worker.close().
      Then worker.enqueue(tmp_path, 'rev-1', paths=('/manual.md',)) is True; wait_for(lambda: len(fake_index.path_calls) == 3, timeout_seconds=2.0); fake_index.path_calls == [('/manual.md',), ('/a.md',), ('/b.md',)]; worker.state().completed == 3.

    @py-22bba7928bf8 @python_test_progressive_worker_waits_for_cpu_sample_before_processing @go_TestModelsOffRuntimeSemanticWorker
    Scenario: test_progressive_worker_waits_for_cpu_sample_before_processing
      Given the pinned Python reference fixtures and controlled inputs
      When samples.pop(0); worker.close().
      Then wait_for(lambda: worker.state().pause_reason == 'cpu-sampling', timeout_seconds=1.0); wait_for(lambda: fake_index.path_calls == [('/a.md',)], timeout_seconds=1.5).

    @py-dc672c3209ac @python_test_rebuild_preserves_ready_embeddings_and_marks_changed_content_stale @go_TestModelsOffRuntimeSemanticWorker
    Scenario: test_rebuild_preserves_ready_embeddings_and_marks_changed_content_stale
      Given the pinned Python reference fixtures and controlled inputs
      When semantic_config(); index.rebuild(semantic_bundle, repo_revision="rev-1"); index.refresh_embeddings(semantic_bundle, repo_revision="rev-1").
      Then embedder.batch_sizes == [2]; index.semantic_status().ready is True; index.semantic_status().embedding_revision == 'rev-2'; index.pending_embedding_paths() == (); embedder.batch_sizes == [2]; index.pending_embedding_paths(limit=10) == ('/projects/alpha.md',).

    @py-7b79581af215 @python_test_refresh_embeddings_batches_full_bundle @go_TestModelsOffRuntimeSemanticWorker
    Scenario: test_refresh_embeddings_batches_full_bundle
      Given the pinned Python reference fixtures and controlled inputs
      When write_concept( semantic_bundle / "projects" / "gamma.md", concept_id="gamma-id", title="Gamma", body="# Gamma\n\ngamma visible\n", ); write_concept( semantic_bundle / "projects" / "delta.md", concept_id="delta-id", title="Delta", body="# Delta\n\nvisible\n", ); write_concept( semantic_bundle / "projects" / "epsilon.md", concept_id="epsilon-id", title="Epsilon", body="# Epsilon\n\nvisible\n", ).
      Then embedder.batch_sizes == [2, 2, 1]; index.semantic_status().ready is True.

    @py-8859da0183d0 @python_test_runtime_subprocess_worker_reports_lag_then_catches_up_and_closes @go_TestModelsOffRuntimeSemanticWorker
    Scenario: test_runtime_subprocess_worker_reports_lag_then_catches_up_and_closes
      Given the pinned Python reference fixtures and controlled inputs
      When threading.Event(); monkeypatch.setattr("memento.app.SubprocessEmbeddingClient", lambda *args, **kwargs: embedder); build_runtime(config_path, bootstrap_seed=seed).
      Then runtime.embedding_refresh_worker is not None; embedder.batch_started.wait(timeout=1.0); status['index_revision'] == status['repo_revision']; status['semantic_search']['ready'] is False; wait_for(lambda: runtime.status_snapshot()['semantic_search']['ready'] is True, timeout_seconds=2.0); embedder.closed is True.

    @py-053549d9bdaa @python_test_status_and_enqueue_remain_responsive_while_polling_blocks @go_TestModelsOffRuntimeSemanticWorker
    Scenario: test_status_and_enqueue_remain_responsive_while_polling_blocks
      Given the pinned Python reference fixtures and controlled inputs
      When threading.Event(); _fast_policy(); worker.close().
      Then entered.wait(1); worker.state().alive and worker.pending; worker.enqueue(tmp_path, 'new-rev', paths=('/manual.md',)); time.monotonic() - started < 0.5; worker.wait_idle(timeout_seconds=3); index.path_calls == [('/manual.md',)].

    @py-0d43f69aa77b @python_test_transient_execution_lock_preserves_request @go_TestModelsOffRuntimeSemanticWorker
    Scenario: test_transient_execution_lock_preserves_request
      Given the pinned Python reference fixtures and controlled inputs
      When cast(DerivedIndex, index); worker.close().
      Then wait_for(lambda: worker.state().pause_reason == 'database-busy', timeout_seconds=2); worker.state().pending and worker.state().completed == 0; worker.wait_idle(timeout_seconds=3); index.attempts == 2 and worker.state().completed == 1; index.calls == ['rev'] if full else []; index.path_calls == [] if full else [('/manual.md',)].

    @py-e8b106926127 @python_test_unexpected_loop_exit_exposes_dead_worker @go_TestModelsOffRuntimeSemanticWorker
    Scenario: test_unexpected_loop_exit_exposes_dead_worker
      Given the pinned Python reference fixtures and controlled inputs
      When monkeypatch.setattr(SemanticEmbeddingRefreshWorker, "_run_loop", lambda self: None); cast(DerivedIndex, FakeRefreshIndex()); worker._thread.join(timeout=1).
      Then not state.alive and not state.running; state.pause_reason == 'worker-stopped'; state.last_error == 'embedding worker stopped unexpectedly'; not worker.enqueue(tmp_path, 'rev').

  Rule: Behavior captured from test_subprocess_embeddings.py

    @py-722fa79ca3a2 @python_test_auto_backend_fallback_is_bounded_and_remembered @go_TestDecodeEmbeddingResponse @go_TestLoadSubprocessSemanticClient
    Scenario: test_auto_backend_fallback_is_bounded_and_remembered
      Given the pinned Python reference fixtures and controlled inputs
      When model.write_bytes(b"same-model").
      Then client.embed('hello') == (1, 0); client.embed('again') == (1, 0); len(calls) == 3 and calls[0][-1] == 'auto' and all((len(c) == 2 for c in calls[1:])); client.last_backend and 'device lost' in str(client.last_backend['fallback_reason']); client.model_info().revision != cpu.model_info().revision.

    @py-5827d21a5c49 @python_test_auto_timeout_does_not_exceed_deadline_and_disables_next_gpu_attempt @go_TestDecodeEmbeddingResponse @go_TestLoadSubprocessSemanticClient
    Scenario: test_auto_timeout_does_not_exceed_deadline_and_disables_next_gpu_attempt
      Given the pinned Python reference fixtures and controlled inputs
      When model.write_bytes(b"model").
      Then len(calls) == 1; len(calls) == 2 and len(calls[1]) == 2. expects client.embed("test") raises SemanticSearchError matching "failed"; client.embed("test") raises SemanticSearchError.

    @py-eb9219b297d8 @python_test_explicit_vulkan_failure_does_not_fall_back @go_TestDecodeEmbeddingResponse @go_TestLoadSubprocessSemanticClient
    Scenario: test_explicit_vulkan_failure_does_not_fall_back
      Given the pinned Python reference fixtures and controlled inputs
      When model.write_bytes(b"model").
      Then calls == [['worker', str(model), 'vulkan', 'Intel']]. expects client.embed("test") raises SemanticSearchError matching "no hardware".

    @py-d30d82402e70 @python_test_malformed_metadata_auto_falls_back_without_conversion_errors @go_TestDecodeEmbeddingResponse @go_TestLoadSubprocessSemanticClient
    Scenario: test_malformed_metadata_auto_falls_back_without_conversion_errors
      Given the pinned Python reference fixtures and controlled inputs
      When model.write_bytes(b"model").
      Then client.embed('test') == (1, 0); len(calls) == 2 and len(calls[-1]) == 2; client.last_backend and client.last_backend['fallback_reason'].

    @py-f0ae4fd6e969 @python_test_subprocess_embedding_client_batches_and_exits @go_TestDecodeEmbeddingResponse @go_TestLoadSubprocessSemanticClient
    Scenario: test_subprocess_embedding_client_batches_and_exits
      Given the pinned Python reference fixtures and controlled inputs
      When fake_worker(worker); model_file(model).
      Then client.embed_batch(('one', 'two')) == ((1.0, 1.0, 1.0, 1.0), (2.0, 2.0, 2.0, 2.0)); client.embed('one') == (1.0, 1.0, 1.0, 1.0).

    @py-08ee38d49afa @python_test_subprocess_embedding_client_cancellation @go_TestDecodeEmbeddingResponse @go_TestLoadSubprocessSemanticClient
    Scenario: test_subprocess_embedding_client_cancellation
      Given the pinned Python reference fixtures and controlled inputs
      When fake_worker(worker); model_file(model). expects client.embed("one", cancelled=lambda: True) raises SemanticSearchError matching "cancelled".
      Then the result, state transitions, and error boundary match the captured behavior

    @py-e01644608815 @python_test_subprocess_embedding_client_enforces_limits_and_errors @go_TestDecodeEmbeddingResponse @go_TestLoadSubprocessSemanticClient
    Scenario: test_subprocess_embedding_client_enforces_limits_and_errors
      Given the pinned Python reference fixtures and controlled inputs
      When fake_worker(worker); model_file(model); fake_worker(failed_worker, fail=True). expects client.embed_batch(("one", "two")) raises SemanticSearchError matching "maximum"; client.embed("four") raises SemanticSearchError matching "character limit"; failed.embed("one") raises SemanticSearchError matching "exited 7".
      Then the result, state transitions, and error boundary match the captured behavior

    @py-9eb9286cd3bc @python_test_subprocess_embedding_client_uses_low_priority_single_thread_environment @go_TestDecodeEmbeddingResponse @go_TestLoadSubprocessSemanticClient
    Scenario: test_subprocess_embedding_client_uses_low_priority_single_thread_environment
      Given the pinned Python reference fixtures and controlled inputs
      When model_file(model).
      Then client.embed('one') == (1.0, 1.0, 1.0, 1.0); seen['command'] == ['nice', '-n', '15', '/worker', str(model)]; seen['env'][name] == '1'.

    @py-df1d8c772b68 @python_test_vulkan_configuration_is_opt_in_and_requires_subprocess @go_TestDecodeEmbeddingResponse @go_TestLoadSubprocessSemanticClient
    Scenario: test_vulkan_configuration_is_opt_in_and_requires_subprocess
      Given the pinned Python reference fixtures and controlled inputs
      When checks SemanticSearchConfig().backend == 'cpu'. expects SemanticSearchConfig(backend="vulkan", worker_mode="in_process") raises ValidationError matching "subprocess".
      Then the result, state transitions, and error boundary match the captured behavior

  Rule: Behavior captured from test_warm_subprocess_embeddings.py

    @py-3c21905a12f0 @python_test_default_configuration_remains_cold @go_TestSemanticWorkerFailuresAndWait @go_TestSemanticWorkerSelectedAndFull
    Scenario: test_default_configuration_remains_cold
      Given the pinned Python reference fixtures and controlled inputs
      When client_factory(idle_seconds=0.0); _events(worker_harness, "start").
      Then client.worker_status == {'reuse': False}; client.embed('alpha') == _vector_for_text('alpha'); client.embed('beta') == _vector_for_text('beta'); len(starts) == 2.

    @py-0f59da364b42 @python_test_environment_change_recycles_worker @go_TestSemanticWorkerFailuresAndWait @go_TestSemanticWorkerSelectedAndFull
    Scenario: test_environment_change_recycles_worker
      Given the pinned Python reference fixtures and controlled inputs
      When client_factory(idle_seconds=5.0); client.embed("before"); _status_int(client.worker_status, "pid").
      Then _status_int(client.worker_status, 'pid') != first; not _pid_exists(first).

    @py-00811f639c68 @python_test_failed_reap_is_retried_not_reused @go_TestSemanticWorkerFailuresAndWait @go_TestSemanticWorkerSelectedAndFull
    Scenario: test_failed_reap_is_retried_not_reused
      Given the pinned Python reference fixtures and controlled inputs
      When client_factory(idle_seconds=5.0); client.embed("before"); _status_int(client.worker_status, "pid").
      Then client.worker_status['pid'] is None; not _pid_exists(pid). expects client.close() raises SemanticSearchError matching "reaped".

    @py-a1fcc2c94002 @python_test_fallback_shares_original_deadline @go_TestSemanticWorkerFailuresAndWait @go_TestSemanticWorkerSelectedAndFull
    Scenario: test_fallback_shares_original_deadline
      Given the pinned Python reference fixtures and controlled inputs
      When monkeypatch.setenv("MEMENTO_FAKE_AUTO_FAIL", "1"); monkeypatch.setenv("MEMENTO_FAKE_AUTO_DELAY", "0.35"); monkeypatch.setenv("MEMENTO_FAKE_CPU_DELAY", "0.35").
      Then time.monotonic() - started < 1.3; [event['backend'] for event in _events(worker_harness, 'start')] == ['auto', 'cpu']; client.worker_status['pid'] is None. expects client.embed("test") raises SemanticSearchError matching "deadline".

    @py-81cd0af0d15b @python_test_queued_deadline_does_not_discard_active_worker @go_TestSemanticWorkerFailuresAndWait @go_TestSemanticWorkerSelectedAndFull
    Scenario: test_queued_deadline_does_not_discard_active_worker
      Given the pinned Python reference fixtures and controlled inputs
      When client_factory(timeout_seconds=2.0, idle_seconds=5.0); _marker(worker_harness, f"{token}.release").write_text("go").
      Then _wait_result(active) == _vector_for_text('active'); client.worker_status['starts'] == 1. expects client.embed("queued") raises SemanticSearchError matching "deadline".

    @py-3647c9e21dfa @python_test_warm_client_active_request_can_outlive_idle_window @go_TestSemanticWorkerFailuresAndWait @go_TestSemanticWorkerSelectedAndFull
    Scenario: test_warm_client_active_request_can_outlive_idle_window
      Given the pinned Python reference fixtures and controlled inputs
      When client_factory(idle_seconds=0.3, timeout_seconds=1.0); _marker(worker_harness, "active-idle.release").write_text("", encoding="utf-8").
      Then client.worker_status['pid'] == pid; client.worker_status['active'] is True; _wait_result(future) == _vector_for_text('alpha'); client.embed('beta') == _vector_for_text('beta'); status['pid'] == pid; status['starts'] == 1.

    @py-a0cd53eb2c29 @python_test_warm_client_auto_backend_falls_back_to_cpu_and_remembers_failure @go_TestSemanticWorkerFailuresAndWait @go_TestSemanticWorkerSelectedAndFull
    Scenario: test_warm_client_auto_backend_falls_back_to_cpu_and_remembers_failure
      Given the pinned Python reference fixtures and controlled inputs
      When monkeypatch.setenv("MEMENTO_FAKE_AUTO_FAIL", "1"); client_factory(backend="auto", timeout_seconds=1.0); _events(worker_harness, "start").
      Then client.embed('alpha') == _vector_for_text('alpha'); client.embed('beta') == _vector_for_text('beta'); [entry['backend'] for entry in starts] == ['auto', 'cpu']; [entry['backend'] for entry in requests] == ['auto', 'cpu', 'cpu']; client.worker_status['starts'] == 2; client.last_backend == {'selected': 'cpu', 'requested': 'auto', 'fallback_reason': 'embedding worker reported an invalid backend'}.

    @py-2dc9ab47d357 @python_test_warm_client_auto_backend_falls_back_when_selected_type_is_malformed @go_TestSemanticWorkerFailuresAndWait @go_TestSemanticWorkerSelectedAndFull
    Scenario: test_warm_client_auto_backend_falls_back_when_selected_type_is_malformed
      Given the pinned Python reference fixtures and controlled inputs
      When client_factory(backend="auto", timeout_seconds=1.0); _events(worker_harness, "start"); _events(worker_harness, "request").
      Then client.embed(_control(mode, f'bad-selected-{mode}', 'alpha')) == _vector_for_text('alpha'); [entry['backend'] for entry in starts] == ['auto', 'cpu']; [entry['backend'] for entry in requests] == ['auto', 'cpu']; client.last_backend == {'selected': 'cpu', 'requested': 'auto', 'fallback_reason': 'embedding worker reported an invalid backend'}.

    @py-b2b87c94f2e9 @python_test_warm_client_bad_responses_drop_process_and_recover @go_TestSemanticWorkerFailuresAndWait @go_TestSemanticWorkerSelectedAndFull
    Scenario: test_warm_client_bad_responses_drop_process_and_recover
      Given the pinned Python reference fixtures and controlled inputs
      When client_factory(timeout_seconds=1.0); _status_int(client.worker_status, "pid"); _wait_for(lambda: not _pid_exists(pid), timeout=1.5).
      Then client.embed('warmup') == _vector_for_text('warmup'); client.worker_status['pid'] is None; client.embed('recover') == _vector_for_text('recover'); client.worker_status['starts'] == 2. expects client.embed(_control(mode, f"bad-{mode}", "broken")) raises SemanticSearchError matching "match".

    @py-4115d2858113 @python_test_warm_client_cancels_active_call_and_recovers @go_TestSemanticWorkerFailuresAndWait @go_TestSemanticWorkerSelectedAndFull
    Scenario: test_warm_client_cancels_active_call_and_recovers
      Given the pinned Python reference fixtures and controlled inputs
      When client_factory(timeout_seconds=1.0); _status_int(client.worker_status, "pid"); _wait_for(lambda: not _pid_exists(pid), timeout=1.5).
      Then client.worker_status['pid'] is None; client.embed('beta') == _vector_for_text('beta'); client.worker_status['starts'] == 2. expects _wait_result(future) raises SemanticSearchError matching "cancelled".

    @py-2d2bbb6c5fe9 @python_test_warm_client_cancels_queued_call_without_killing_active_request @go_TestSemanticWorkerFailuresAndWait @go_TestSemanticWorkerSelectedAndFull
    Scenario: test_warm_client_cancels_queued_call_without_killing_active_request
      Given the pinned Python reference fixtures and controlled inputs
      When client_factory(timeout_seconds=1.0); _wait_for(lambda: len(_events(worker_harness, "request")) == 1); _events(worker_harness, "request").
      Then client.worker_status['pid'] == pid; _wait_result(active) == _vector_for_text('alpha'); client.embed('gamma') == _vector_for_text('gamma'); [entry['backend'] for entry in requests] == ['cpu', 'cpu']; client.worker_status['starts'] == 1. expects _wait_result(queued) raises SemanticSearchError matching "cancelled".

    @py-1c75fb7c4a93 @python_test_warm_client_close_during_active_request_aborts_and_reaps_worker @go_TestSemanticWorkerFailuresAndWait @go_TestSemanticWorkerSelectedAndFull
    Scenario: test_warm_client_close_during_active_request_aborts_and_reaps_worker
      Given the pinned Python reference fixtures and controlled inputs
      When client_factory(timeout_seconds=1.0); _status_int(client.worker_status, "pid"); _wait_for(lambda: not _pid_exists(pid), timeout=1.5).
      Then client.worker_status['closed'] is True; client.worker_status['pid'] is None. expects _wait_result(future) raises SemanticSearchError matching "closed".

    @py-97cd7e2fb9ee @python_test_warm_client_close_is_idempotent_and_blocks_future_use @go_TestSemanticWorkerFailuresAndWait @go_TestSemanticWorkerSelectedAndFull
    Scenario: test_warm_client_close_is_idempotent_and_blocks_future_use
      Given the pinned Python reference fixtures and controlled inputs
      When client_factory(); client.close().
      Then client.embed('before-close') == _vector_for_text('before-close'); client.worker_status['closed'] is True; client.worker_status['pid'] is None. expects client.embed("after-close") raises SemanticSearchError matching "closed".

    @py-cf672d4af0c4 @python_test_warm_client_drains_stderr_without_deadlock @go_TestSemanticWorkerFailuresAndWait @go_TestSemanticWorkerSelectedAndFull
    Scenario: test_warm_client_drains_stderr_without_deadlock
      Given the pinned Python reference fixtures and controlled inputs
      When monkeypatch.setenv("MEMENTO_FAKE_STDERR_FLOOD", "1"); client_factory(timeout_seconds=1.0).
      Then client.embed('stderr') == _vector_for_text('stderr'); len(_events(worker_harness, 'request')) == 1; client.worker_status['completed_requests'] == 1.

    @py-2804a49541ff @python_test_warm_client_expires_after_idle_and_restarts @go_TestDecodeEmbeddingResponse @go_TestDecodeEmbeddingResponseFailures
    Scenario: test_warm_client_expires_after_idle_and_restarts
      Given the pinned Python reference fixtures and controlled inputs
      When client_factory(idle_seconds=0.3); client.embed("idle"); _status_int(status1, "pid").
      Then client.embed('restart') == _vector_for_text('restart'); status2['generation'] == 2; status2['starts'] == 2; status2['completed_requests'] == 2; status2['pid'] is not None.

    @py-285ac528e8b5 @python_test_warm_client_explicit_vulkan_failure_does_not_fall_back @go_TestSemanticWorkerFailuresAndWait @go_TestSemanticWorkerSelectedAndFull
    Scenario: test_warm_client_explicit_vulkan_failure_does_not_fall_back
      Given the pinned Python reference fixtures and controlled inputs
      When monkeypatch.setenv("MEMENTO_FAKE_VULKAN_FORCE_CPU", "1"); client_factory(backend="vulkan", timeout_seconds=1.0); _events(worker_harness, "start").
      Then [entry['backend'] for entry in starts] == ['vulkan']; client.last_backend is None. expects client.embed("alpha") raises SemanticSearchError matching "did not use Vulkan".

    @py-25de89fc5f00 @python_test_warm_client_handles_partial_pipe_reads_and_writes @go_TestDecodeEmbeddingResponse @go_TestDecodeEmbeddingResponseFailures
    Scenario: test_warm_client_handles_partial_pipe_reads_and_writes
      Given the pinned Python reference fixtures and controlled inputs
      When monkeypatch.setenv("MEMENTO_FAKE_READ_CHUNK", "7"); monkeypatch.setenv("MEMENTO_FAKE_WRITE_CHUNK", "3"); client_factory(max_input_chars=len(text) + 1, timeout_seconds=1.0).
      Then client.embed(text) == _vector_for_text(text).

    @py-61c2a96f03dd @python_test_warm_client_missing_worker_executable_fails_cleanly @go_TestSemanticWorkerFailuresAndWait @go_TestSemanticWorkerSelectedAndFull
    Scenario: test_warm_client_missing_worker_executable_fails_cleanly
      Given the pinned Python reference fixtures and controlled inputs
      When model.write_bytes(b"model"); client.close().
      Then client.worker_status['pid'] is None. expects client.embed("alpha") raises SemanticSearchError matching "pipe failed".

    @py-8ec77a95caa3 @python_test_warm_client_restarts_after_request_count_cap @go_TestSemanticWorkerFailuresAndWait @go_TestSemanticWorkerSelectedAndFull
    Scenario: test_warm_client_restarts_after_request_count_cap
      Given the pinned Python reference fixtures and controlled inputs
      When monkeypatch.setattr(framed_worker, "_MAX_REQUESTS", 2); client_factory(timeout_seconds=1.0); _status_int(client.worker_status, "pid").
      Then client.embed('one') == _vector_for_text('one'); client.embed('two') == _vector_for_text('two'); client.embed('three') == _vector_for_text('three'); status['starts'] == 2; status['generation'] == 2; status['completed_requests'] == 3.

    @py-c65ab1db00d6 @python_test_warm_client_reuses_same_process_and_updates_counters @go_TestDecodeEmbeddingResponse @go_TestDecodeEmbeddingResponseFailures
    Scenario: test_warm_client_reuses_same_process_and_updates_counters
      Given the pinned Python reference fixtures and controlled inputs
      When client_factory(); client.embed("alpha"); client.embed("beta").
      Then first == _vector_for_text('alpha'); second == _vector_for_text('beta'); status1['pid'] == status2['pid']; status1['generation'] == 1; status2['generation'] == 1; status1['starts'] == 1.

    @py-f158147f2538 @python_test_warm_client_serialises_concurrent_calls_and_returns_unique_vectors @go_TestSemanticWorkerFailuresAndWait @go_TestSemanticWorkerSelectedAndFull
    Scenario: test_warm_client_serialises_concurrent_calls_and_returns_unique_vectors
      Given the pinned Python reference fixtures and controlled inputs
      When client_factory(timeout_seconds=1.0); _wait_for(lambda: len(_events(worker_harness, "request")) == 1); _events(worker_harness, "request").
      Then not second_future.done(); len(requests) == 2; requests[0]['pid'] == requests[1]['pid']; first == _vector_for_text('alpha'); second == _vector_for_text('beta'); first != second.

    @py-17f23e1a3383 @python_test_warm_client_times_out_on_hung_worker_and_recovers @go_TestSemanticWorkerFailuresAndWait @go_TestSemanticWorkerSelectedAndFull
    Scenario: test_warm_client_times_out_on_hung_worker_and_recovers
      Given the pinned Python reference fixtures and controlled inputs
      When client_factory(timeout_seconds=0.4, max_input_chars=2_000_000); _control("hang", "hung-reader", "alpha"); monkeypatch.setenv("MEMENTO_FAKE_NONREADER", "1").
      Then client.worker_status['pid'] is None; client.embed('recover') == _vector_for_text('recover'); client.worker_status['starts'] == 2; _marker(worker_harness, 'nonreader.started').exists(). expects client.embed(text) raises SemanticSearchError matching "deadline exceeded".

  Rule: Behavior captured from test_warm_worker_config.py

    @py-b014b909f53b @python_test_build_runtime_passes_worker_idle_seconds_to_subprocess_client @go_TestSemanticSearchConfig @go_TestNeedleRouterConfig
    Scenario: test_build_runtime_passes_worker_idle_seconds_to_subprocess_client
      Given the pinned Python reference fixtures and controlled inputs
      When _FakeLease(); monkeypatch.setattr(app, "SubprocessEmbeddingClient", fake_client); monkeypatch.setattr(app, "acquire_writer_lease", lambda *args, **kwargs: lease).
      Then seen['args'] == ('/worker', '/model.gte'); isinstance(kwargs, dict); kwargs['idle_seconds'] == 12.5; lease.released is True. expects app.build_runtime(config_path) raises _SentinelError matching "wiring".

    @py-dc13e65e6fea @python_test_worker_idle_seconds_accepts_bounds @go_TestSemanticSearchConfig @go_TestNeedleRouterConfig
    Scenario: test_worker_idle_seconds_accepts_bounds
      Given the pinned Python reference fixtures and controlled inputs
      When checks config.worker_idle_seconds == worker_idle_seconds.
      Then the result, state transitions, and error boundary match the captured behavior

    @py-83bf9d852e17 @python_test_worker_idle_seconds_defaults_to_zero_without_changing_cpu_backend @go_TestSemanticSearchConfig @go_TestNeedleRouterConfig
    Scenario: test_worker_idle_seconds_defaults_to_zero_without_changing_cpu_backend
      Given the pinned Python reference fixtures and controlled inputs
      When checks config.worker_idle_seconds == 0.0; config.backend == 'cpu'.
      Then the result, state transitions, and error boundary match the captured behavior

    @py-08e67d44592e @python_test_worker_idle_seconds_rejects_invalid_values @go_TestSemanticSearchConfig @go_TestNeedleRouterConfig
    Scenario: test_worker_idle_seconds_rejects_invalid_values
      Given the pinned Python reference fixtures and controlled inputs
      When expects SemanticSearchConfig(worker_idle_seconds=worker_idle_seconds) raises ValidationError.
      Then the result, state transitions, and error boundary match the captured behavior

    @py-d4c518aed500 @python_test_worker_idle_seconds_requires_subprocess_mode_when_positive @go_TestSemanticSearchConfig @go_TestNeedleRouterConfig
    Scenario: test_worker_idle_seconds_requires_subprocess_mode_when_positive
      Given the pinned Python reference fixtures and controlled inputs
      When checks SemanticSearchConfig(worker_mode='in_process', worker_idle_seconds=0.0).worker_mode == 'in_process'. expects SemanticSearchConfig(worker_mode="in_process", worker_idle_seconds=1.0) raises ValidationError matching "subprocess".
      Then the result, state transitions, and error boundary match the captured behavior

  Rule: Behavior captured from test_cpu_usage.py

    @py-35ee2225e458 @python_test_cpu_sampler_recovers_from_counter_reset @go_TestCPUSampler
    Scenario: test_cpu_sampler_recovers_from_counter_reset
      Given the pinned Python reference fixtures and controlled inputs
      When write_stat(stat, (100, 0, 100, 800, 0, 0, 0, 0)); write_stat(stat, (1, 0, 1, 8, 0, 0, 0, 0)).
      Then sampler.sample() is None; sampler.sample() is None.

    @py-76d6ae2b72cb @python_test_cpu_sampler_reports_busy_percent_over_window @go_TestCPUSampler
    Scenario: test_cpu_sampler_reports_busy_percent_over_window
      Given the pinned Python reference fixtures and controlled inputs
      When write_stat(stat, (100, 0, 100, 800, 0, 0, 0, 0)); write_stat(stat, (120, 0, 120, 860, 0, 0, 0, 0)); write_stat(stat, (130, 0, 130, 900, 0, 0, 0, 0)).
      Then sampler.sample() is None; sampler.sample() is None; sampler.sample() == 37.5.

    @py-e0a85ab78007 @python_test_cpu_sampler_treats_iowait_as_idle @go_TestCPUSampler
    Scenario: test_cpu_sampler_treats_iowait_as_idle
      Given the pinned Python reference fixtures and controlled inputs
      When write_stat(stat, (100, 0, 100, 800, 100, 0, 0, 0)); write_stat(stat, (110, 0, 110, 800, 180, 0, 0, 0)).
      Then sampler.sample() is None; sampler.sample() == 20.0.
