Feature: runtime/config packaging and operations

  The scenarios capture Python Memento behavior at 7f29e8b003557f0105f47ed353b7f65a33619456.
  Rules retain source-module traceability while features group related user behavior.

  Rule: Behavior captured from test_derived_plane.py

    @py-be1cef3a030c @python_test_concurrent_first_open_initializes_schema_once @go_TestContentReference @go_TestGraphReference
    Scenario: test_concurrent_first_open_initializes_schema_once
      Given the pinned Python reference fixtures and controlled inputs
      When threading.Thread(target=read_state); thread.start(); thread.join().
      Then errors == []; states == ['ready'] * 8.

    @py-5a8f01e0ca76 @python_test_corruption_is_quarantined_and_rebuild_recovers @go_TestContentReference @go_TestGraphReference
    Scenario: test_corruption_is_quarantined_and_rebuild_recovers
      Given the pinned Python reference fixtures and controlled inputs
      When index.rebuild(repo_paths.current_dir, repo_revision=get_main_revision(repo_paths)); extra.exists(); index.db_path.write_bytes(b"not sqlite").
      Then quarantines; [result.path for result in index.search(policy=policy, query='visible').results] == ['/instances/smith.md']. expects index.search(policy=policy, query="visible") raises DerivedIndexCorruptionError.

    @py-94e20c530e89 @python_test_external_links_are_not_broken_even_after_existing_index_migration @go_TestContentReference @go_TestGraphReference
    Scenario: test_external_links_are_not_broken_even_after_existing_index_migration
      Given the pinned Python reference fixtures and controlled inputs
      When path.write_text( path.read_text() + "\n[web](https://example.org/x) [mail](mailto:hi@example.org) [network](//example.org/x)\n" ); derived_index.update_paths( bundle_root, repo_revision="external", changed_paths=("/instances/smith.md",) ); connection.execute("DELETE FROM index_state WHERE key='external_links_classified'").
      Then derived_index.metrics('smith-id').broken_link_count == 1; connection.execute("SELECT count(*) FROM links WHERE resolution_state='external'").fetchone()[0] == 3; migrated.metrics('smith-id').broken_link_count == 1.

    @py-f4860553757d @python_test_fts_syntax_error_returns_validation_error_without_quarantine @go_TestContentReference @go_TestGraphReference
    Scenario: test_fts_syntax_error_returns_validation_error_without_quarantine
      Given the pinned Python reference fixtures and controlled inputs
      When index.rebuild(repo_paths.current_dir, repo_revision=get_main_revision(repo_paths)); index.search(policy=policy, query="visible---", query_syntax="plain"); index.search( policy=policy, query="What is the primary visible instance?", query_syntax="plain", ).
      Then [result.path for result in plain.results] == ['/instances/smith.md']; [result.path for result in natural_language.results] == ['/instances/smith.md']; natural_language.results[0].score > 0; natural_language.results[0].snippet; [result.path for result in explicit.results] == ['/instances/smith.md']; state.status == 'ready'. expects index.search(policy=policy, query='"unterminated', query_syntax="fts5") raises ValueError matching

    @py-f684d2d7e74e @python_test_full_rebuild_search_filters_and_hidden_namespace_do_not_leak_ranking @go_TestContentReference @go_TestGraphReference
    Scenario: test_full_rebuild_search_filters_and_hidden_namespace_do_not_leak_ranking
      Given the pinned Python reference fixtures and controlled inputs
      When derived_index.search(policy=policy, query="visible", limit=10); derived_index.search(policy=policy, query="Piclaw", concept_type="project"); derived_index.search(policy=policy, query="concept", tags=("orphan",)).
      Then '/instances/smith.md' in visible_paths; '/secret/ghost.md' not in visible_paths; [result.path for result in typed.results] == ['/projects/piclaw.md']; [result.path for result in tagged.results] == ['/projects/orphan.md']; [result.path for result in prefixed.results] == ['/instances/smith.md']; '/secret/ghost.md' in [result.path for result in hidden.results].

    @py-752ebfad5855 @python_test_graph_metrics_and_bounded_neighborhood @go_TestContentReference @go_TestGraphReference
    Scenario: test_graph_metrics_and_bounded_neighborhood
      Given the pinned Python reference fixtures and controlled inputs
      When connection.executemany( "INSERT INTO links VALUES(?,?,?,?,?,?,?,?,?)", ( ( "smith-id", None, "/projects/secret-alias.md", "/secret/missing.…; derived_index.graph(policy=policy, concept_id="smith-id", depth=2); derived_index.metrics("smith-id").
      Then [edge.path for edge in graph.outbound] == ['/projects/piclaw.md']; [edge.path for edge in graph.inbound] == ['/projects/piclaw.md']; all((edge.broken_link_count == 0 for edge in (*graph.outbound, *graph.inbound))); graph.broken_targets == ('/projects/missing.md',); smith_metrics.inbound_degree == 1; smith_metrics.outbound_degree == 2.

    @py-54533fd8bea4 @python_test_incremental_update_ignores_non_markdown_artifacts @go_TestContentReference @go_TestGraphReference
    Scenario: test_incremental_update_ignores_non_markdown_artifacts
      Given the pinned Python reference fixtures and controlled inputs
      When artifact.parent.mkdir(parents=True); artifact.write_bytes(b"PK-not-a-concept"); derived_index.rebuild(tmp_path, repo_revision="rev-empty").
      Then state.repo_revision == 'rev-artifact'; state.index_revision == 'rev-artifact'.

    @py-4fbf1678faeb @python_test_incremental_update_matches_clean_rebuild_and_supports_delete_then_rebuild @go_TestContentReference @go_TestGraphReference
    Scenario: test_incremental_update_matches_clean_rebuild_and_supports_delete_then_rebuild
      Given the pinned Python reference fixtures and controlled inputs
      When write_concept( target, concept_id="piclaw-id", concept_type="project", title="Piclaw", description="Updated project.", tags=("project", "up…; orphan.unlink(); write_concept( repo_paths.current_dir / "projects" / "new.md", concept_id="new-id", concept_type="project", title="New", description="Brand….
      Then parity.matches is True; [result.path for result in rebuilt.results] == ['/projects/piclaw.md'].

    @py-401d394e0dbd @python_test_status_snapshot_counts_only_authorized_concepts @go_TestContentReference @go_TestGraphReference
    Scenario: test_status_snapshot_counts_only_authorized_concepts
      Given the pinned Python reference fixtures and controlled inputs
      When derived_index.status_snapshot(policy); derived_index.status_snapshot(hidden_policy).
      Then visible.state == all_concepts.state; visible.visible_concepts == 3; all_concepts.visible_concepts == 4.

    @py-fea0b8b2de13 @python_test_strict_freshness_waits_until_index_matches_repo_revision @go_TestContentReference @go_TestGraphReference
    Scenario: test_strict_freshness_waits_until_index_matches_repo_revision
      Given the pinned Python reference fixtures and controlled inputs
      When derived_index.set_repo_revision("rev-fresh"); threading.Thread(target=delayed_update); thread.start().
      Then state.index_revision == 'rev-fresh'; results == ['done'].

    @py-620016d92497 @python_test_transaction_apply_updates_derived_index_before_success @go_TestContentReference @go_TestGraphReference
    Scenario: test_transaction_apply_updates_derived_index_before_success
      Given the pinned Python reference fixtures and controlled inputs
      When derived_index.rebuild(repo_paths.current_dir, repo_revision=get_main_revision(repo_paths)); manager.apply( TransactionRequest( operation=OperationRequest( op_id="op-derived", principal="smith", idempotency_key="idem-derived", tool_…; derived_index.search( policy=policy, query="transaction visible", freshness=SearchFreshness.STRICT, timeout_seconds=0.5, ).
      Then page.index_revision == result.result_revision; [item.path for item in page.results] == ['/projects/piclaw.md'].

    @py-e9ee41390d3c @python_test_transient_lock_does_not_quarantine_derived_index @go_TestContentReference @go_TestGraphReference
    Scenario: test_transient_lock_does_not_quarantine_derived_index
      Given the pinned Python reference fixtures and controlled inputs
      When get_main_revision(repo_paths); index.rebuild(repo_paths.current_dir, repo_revision=revision); index.get_state().
      Then index.db_path.is_file(); list(tmp_path.glob('locked.quarantine-*.sqlite')) == []; state.status == 'ready'; state.index_revision == revision. expects index._with_quarantine(locked) raises DerivedIndexUnavailableError matching "database is locked".

  Rule: Behavior captured from test_load_harness.py

    @py-9edb7299d692 @python_test_build_report_serializes_expected_shape @go_TestSemanticBlobCosineAllocationBudget @go_TestSemanticCosineFailureBranches
    Scenario: test_build_report_serializes_expected_shape
      Given the pinned Python reference fixtures and controlled inputs
      When compile_scenario( "shape", datetime.now(UTC), datetime.now(UTC), records=[], thresholds=(build_threshold("errors", 0, "==", 0, "ok"),), ); build_report(config, [scenario]).
      Then report['passed'] is True; report['scenario_count'] == 1; report['git_revision']; 'local development checks' in encoded.

    @py-08e1f56b92e7 @python_test_compile_scenario_tracks_thresholds_and_failures @go_TestSemanticBlobCosineAllocationBudget @go_TestSemanticCosineFailureBranches
    Scenario: test_compile_scenario_tracks_thresholds_and_failures
      Given the pinned Python reference fixtures and controlled inputs
      When datetime.now(UTC); compile_scenario( "unit", started, ended, records=[], invariant_failures=("boom",), thresholds=(build_threshold("errors", 1, "==", 0, "test….
      Then scenario.passed is False; scenario.invariant_failures == ('boom',); scenario.thresholds[0].passed is False; scenario.latency.max_ms == 0.0.

    @py-9ddf946472d3 @python_test_direct_load_report_contains_metrics_and_passes @go_TestSemanticBlobCosineAllocationBudget @go_TestSemanticCosineFailureBranches
    Scenario: test_direct_load_report_contains_metrics_and_passes
      Given the pinned Python reference fixtures and controlled inputs
      When run_direct_load(env, workers=2, requests=8).
      Then scenario.name == 'direct_functional_load'; scenario.count == 8; scenario.error_count == 0; scenario.ok_count == 8; scenario.latency.p50_ms >= 0.0; scenario.latency.p99_ms >= scenario.latency.p95_ms.

    @py-a70d260333fd @python_test_operational_scenarios_enforce_invariants @go_TestSemanticBlobCosineAllocationBudget @go_TestSemanticCosineFailureBranches
    Scenario: test_operational_scenarios_enforce_invariants
      Given the pinned Python reference fixtures and controlled inputs
      When run_write_contention(env, workers=4).
      Then write_result.passed is True; replay_result.passed is True; proposal_result.passed is True; backup_result.passed is True; write_result.details['target_path'] starts with '/projects/'; replay_result.details['idempotency_key'] == 'replay-storm-key'.

    @py-5c271f11505c @python_test_percentile_and_latency_summary_are_stable @go_TestSemanticBlobCosineAllocationBudget @go_TestSemanticCosineFailureBranches
    Scenario: test_percentile_and_latency_summary_are_stable
      Given the pinned Python reference fixtures and controlled inputs
      When summarize_latencies(values).
      Then percentile(values, 0.0) == 10.0; percentile(values, 0.5) == 30.0; percentile(values, 1.0) == 50.0; summary.p50_ms == 30.0; summary.p95_ms >= 40.0; summary.p99_ms >= summary.p95_ms.

  Rule: Behavior captured from test_package.py

    @py-e51de85aed7e @python_test_package_version @go_TestRunWrapperAndMain @go_TestReleaseScripts
    Scenario: test_package_version
      Given the pinned Python reference fixtures and controlled inputs
      When checks __version__ == '0.5.9'.
      Then the result, state transitions, and error boundary match the captured behavior

  Rule: Behavior captured from test_release_deploy.py

    @py-8c63def8ae6f @python_test_compose_uses_bounded_persisted_state_startup_grace @go_TestReleaseScripts @go_TestModelsOffRuntimeHTTPHooks
    Scenario: test_compose_uses_bounded_persisted_state_startup_grace
      Given the pinned Python reference fixtures and controlled inputs
      When yaml.safe_load(release_deploy.compose("0.3.23")); yaml.safe_load((ROOT / "deploy/diskstation.compose.yaml").read_text(encoding="utf-8")); (ROOT / "Dockerfile").read_text(encoding="utf-8").
      Then service['image'] == 'ghcr.io/rcarmo/memento:0.3.23'; service['healthcheck'] == expected_healthcheck; static['services']['memento']['healthcheck'] == expected_healthcheck; '--start-period=5m' in dockerfile.

    @py-419cd8c8780e @python_test_deploy_propagates_stack_update_failure @go_TestReleaseScripts @go_TestModelsOffRuntimeHTTPHooks
    Scenario: test_deploy_propagates_stack_update_failure
      Given the pinned Python reference fixtures and controlled inputs
      When monkeypatch.setattr(release_deploy, "portainer_request", request); monkeypatch.setattr( release_deploy, "release_image", lambda _version: "ghcr.io/rcarmo/memento@sha256:" + "a" * 64, ). expects release_deploy.deploy(deploy_args()) raises SystemExit matching "stack update failed".
      Then the result, state transitions, and error boundary match the captured behavior

    @py-47928d7a9cd7 @python_test_stack_image_replacement_preserves_all_other_configuration @go_TestReleaseScripts @go_TestModelsOffRuntimeHTTPHooks
    Scenario: test_stack_image_replacement_preserves_all_other_configuration
      Given the pinned Python reference fixtures and controlled inputs
      When checks release_deploy.replace_stack_image(stack, image) == stack.replace('ghcr.io/rcarmo/memento:0.4.2', image). expects release_deploy.replace_stack_image("services: {}", image) raises SystemExit.
      Then the result, state transitions, and error boundary match the captured behavior

    @py-d2f704e365c8 @python_test_update_config_reports_failure_and_still_removes_helper @go_TestReleaseScripts @go_TestModelsOffRuntimeHTTPHooks
    Scenario: test_update_config_reports_failure_and_still_removes_helper
      Given the pinned Python reference fixtures and controlled inputs
      When monkeypatch.setattr(release_deploy, "portainer_request", request); monkeypatch.setattr( release_deploy, "uuid4", lambda: type("Uuid", (), {"hex": "failedhelper"})() ).
      Then calls[-1] == ('DELETE', '/api/endpoints/18/docker/containers/failed-helper-id?force=true&v=true'). expects release_deploy.update_config(deploy_args()) raises SystemExit matching "failed with exit code 17".

    @py-2755a34f6cdc @python_test_update_config_waits_for_success_and_removes_unique_helper @go_TestReleaseScripts @go_TestModelsOffRuntimeHTTPHooks
    Scenario: test_update_config_waits_for_success_and_removes_unique_helper
      Given the pinned Python reference fixtures and controlled inputs
      When iter(("running", "exited")); monkeypatch.setattr(release_deploy, "portainer_request", request); monkeypatch.setattr( release_deploy, "uuid4", lambda: type("Uuid", (), {"hex": "abc123def456"})() ).
      Then create[0] == 'POST'; create[1] ends with 'name=memento-config-update-abc123def456'; payload['User'] == '65532:65532'; payload['NetworkDisabled'] is True; "temporary.open('w')" in script; 'os.fsync(stream.fileno())' in script.

  Rule: Behavior captured from test_runtime_models.py

    @py-f8a8abb0faff @python_test_download_extracts_and_verifies_exact_archive @go_TestRuntimeConfigAndPaths @go_TestBuildModelsOffRuntime
    Scenario: test_download_extracts_and_verifies_exact_archive
      Given the pinned Python reference fixtures and controlled inputs
      When load_tool(); archive(files); manifest(files, bundle).
      Then (tmp_path / name).read_bytes() == contents; isinstance(asset_name, str); not (tmp_path / asset_name).exists().

    @py-c8a2538f04d7 @python_test_download_rejects_unexpected_archive_members @go_TestRuntimeConfigAndPaths @go_TestBuildModelsOffRuntime
    Scenario: test_download_rejects_unexpected_archive_members
      Given the pinned Python reference fixtures and controlled inputs
      When load_tool(); archive({**files, "unexpected": b"no"}); manifest(files, bundle). expects tool.download(tmp_path, None, payload) raises SystemExit matching "unexpected runtime model archive members".
      Then the result, state transitions, and error boundary match the captured behavior

    @py-5fd0c2ae9605 @python_test_manifest_is_the_only_tracked_runtime_model_source @go_TestRuntimeConfigAndPaths @go_TestBuildModelsOffRuntime
    Scenario: test_manifest_is_the_only_tracked_runtime_model_source
      Given the pinned Python reference fixtures and controlled inputs
      When (ROOT / "models/runtime-models.json").read_text(encoding="utf-8"); isinstance(name, str).
      Then payload['release_tag'] == 'model-assets-v1'; payload['asset_name'] starts with 'runtime-models-'; payload['asset_name'] ends with '.tar'; len(payload['asset_sha256']) == 64; set(payload['files']) == {'models/gte/gte-small.gtemodel', 'models/needle/memento-router.ndl', 'models/needle/needle.model'}; isinstance(name, str).

  Rule: Behavior captured from test_workflows.py

    @py-f4d6af2caa12 @python_test_ci_runs_for_main_and_pull_requests_only_and_cancels_superseded_runs @go_TestReleaseScripts
    Scenario: test_ci_runs_for_main_and_pull_requests_only_and_cancels_superseded_runs
      Given the pinned Python reference fixtures and controlled inputs
      When workflow("ci.yml"); cast(dict[str, Any], ci.get("on") or ci.get(cast(Any, True))).
      Then triggers['push'] == {'branches': ['main']}; triggers['pull_request'] is None; ci['concurrency'] == {'group': 'ci-${{ github.workflow }}-${{ github.event.pull_request.number || github.ref }}', 'cancel-in-progress': True}.

    @py-9b334589b775 @python_test_container_builders_download_prepared_model_artifact @go_TestReleaseScripts
    Scenario: test_container_builders_download_prepared_model_artifact
      Given the pinned Python reference fixtures and controlled inputs
      When workflow("ci.yml"); workflow("release.yml").
      Then any((step.get('with', {}).get('name') == 'runtime-models' for step in ci['steps'])); any((step.get('with', {}).get('name') == 'release-runtime-models' for step in release['steps'])).

    @py-6c5a4bdf2643 @python_test_each_workflow_prepares_one_verified_runtime_model_artifact @go_TestReleaseScripts
    Scenario: test_each_workflow_prepares_one_verified_runtime_model_artifact
      Given the pinned Python reference fixtures and controlled inputs
      When workflow(name).
      Then sum(('--key-only' in command for command in commands)) == 1; sum(('--key-only' not in command for command in commands)) == 1; cached_paths == RUNTIME_PATHS.

    @py-bf07ecfb9d0e @python_test_release_remains_tag_scoped_and_never_cancels_running_releases @go_TestReleaseScripts
    Scenario: test_release_remains_tag_scoped_and_never_cancels_running_releases
      Given the pinned Python reference fixtures and controlled inputs
      When workflow("release.yml"); cast(dict[str, Any], release.get("on") or release.get(cast(Any, True))).
      Then triggers['push'] == {'tags': ['v*']}; 'workflow_dispatch' in triggers; release['concurrency'] == {'group': 'release-${{ github.ref }}', 'cancel-in-progress': False}.

    @py-80aacd96b71b @python_test_release_retention_excludes_runtime_model_asset_releases @go_TestReleaseScripts
    Scenario: test_release_retention_excludes_runtime_model_asset_releases
      Given the pinned Python reference fixtures and controlled inputs
      When workflow("release.yml").
      Then '.filter(release => /^v\\d+\\.\\d+\\.\\d+(?:[-+].*)?$/.te... in script; script.index('.filter(release =>') < script.index('releases.slice(keep)').

    @py-f9a91313622c @python_test_training_and_checkpoint_paths_are_absent_from_workflows @go_TestReleaseScripts
    Scenario: test_training_and_checkpoint_paths_are_absent_from_workflows
      Given the pinned Python reference fixtures and controlled inputs
      When "\n".join( (ROOT / ".github" / "workflows" / name).read_text(encoding="utf-8") for name in ("ci.yml", "release.yml") ).
      Then path not in text.

    @py-e55fed197964 @python_test_workflows_have_no_lfs_configuration_or_commands @go_TestReleaseScripts
    Scenario: test_workflows_have_no_lfs_configuration_or_commands
      Given the pinned Python reference fixtures and controlled inputs
      When (ROOT / ".github" / "workflows" / name).read_text(encoding="utf-8").
      Then forbidden not in text.lower(); 'lfs' not in step.get('with', {}).

    @py-76cc566c96cf @python_test_workflows_pin_cache_and_artifact_actions_to_node24_releases @go_TestReleaseScripts
    Scenario: test_workflows_pin_cache_and_artifact_actions_to_node24_releases
      Given the pinned Python reference fixtures and controlled inputs
      When uses.partition("@").
      Then separator == '@'; revision == NODE24_ACTIONS[repository]; seen == set(NODE24_ACTIONS).
