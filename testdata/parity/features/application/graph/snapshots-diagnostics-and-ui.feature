Feature: graph/snapshots diagnostics and ui

  The scenarios capture Python Memento behavior at 7f29e8b003557f0105f47ed353b7f65a33619456.
  Rules retain source-module traceability while features group related user behavior.

  Rule: Behavior captured from test_graph_debug.py

    @py-6d597626a314 @python_test_disabled_graph_routes_are_indistinguishable_404s @go_TestGraphHTTPThroughUMCP @go_TestUIAuditServer
    Scenario: test_disabled_graph_routes_are_indistinguishable_404s
      Given the pinned Python reference fixtures and controlled inputs
      When request(handler, path).
      Then response is not None; response.status == 404; response.body == b''; request(handler, '/mcp') is None; request(handler, '/not-graph') is None.

    @py-504e22c85bef @python_test_enabled_graph_boundary_serves_ui_status_and_assets @go_TestGraphHTTPThroughUMCP @go_TestUIAuditServer
    Scenario: test_enabled_graph_boundary_serves_ui_status_and_assets
      Given the pinned Python reference fixtures and controlled inputs
      When request(handler, "/graph"); request(handler, "/graph/api/v1/status"); request(handler, "/graph/assets/app.css").
      Then page is not None and page.status == 200; page.content_type == 'text/html; charset=utf-8'; b'trusted networks only' in page.body; status is not None and status.status == 200; payload == {'enabled': True, 'route_prefix': '/graph', 'schema_version': 1, 'warning': 'Unauthenticated visual debugger; trusted networks only.'}; css is not None and css.status == 200.

    @py-cedb7964bc62 @python_test_graph_api_decodes_url_encoded_ids @go_TestGraphHTTPThroughUMCP @go_TestUIAuditServer
    Scenario: test_graph_api_decodes_url_encoded_ids
      Given the pinned Python reference fixtures and controlled inputs
      When _Snapshot(); cast(GraphSnapshotService, snapshot); request(handler, "/graph/api/v1/clusters/cluster%3Askills%3A0%3Aabc").
      Then cluster is not None and cluster.status == 200; json.loads(cluster.body)['cluster_id'] == snapshot.cluster_id; detail is not None and detail.status == 200; json.loads(detail.body)['node']['id'] == snapshot.detail_id.

    @py-9d94c261b69d @python_test_graph_boundary_rejects_methods_and_bodies_without_touching_mcp @go_TestGraphHTTPThroughUMCP @go_TestUIAuditServer
    Scenario: test_graph_boundary_rejects_methods_and_bodies_without_touching_mcp
      Given the pinned Python reference fixtures and controlled inputs
      When request(handler, "/graph/api/v1/status", method="POST"); request(handler, "/graph", body=b"unexpected").
      Then post is not None and post.status == 405; ('Allow', 'GET') in post.headers; body is not None and body.status == 400; request(handler, '/mcp', method='POST', body=b'{}') is None.

    @py-453c4545197d @python_test_graph_principal_simulation_exposes_safe_metadata_and_scopes_requests @go_TestGraphHTTPThroughUMCP @go_TestUIAuditServer
    Scenario: test_graph_principal_simulation_exposes_safe_metadata_and_scopes_requests
      Given the pinned Python reference fixtures and controlled inputs
      When _Snapshot(); simulated_authorization(); request(handler, "/graph/api/v1/principals").
      Then principals is not None and principals.status == 200; payload == {'principals': [{'name': 'projects-reader', 'protected_read_prefixes': ['/private/'], 'read_prefixes': ['/projects/'], 'roles': ['reader'], 'write_prefixes': []}], 'protected_read_prefixes': ['/private/'], 'schema_version': 1}; response is not None and response.status == 200; simulated is not None; simulated.read_prefixes == ('/projects/',); simulated.protected_read_prefixes == ('/private/',).

    @py-a5d86f41895c @python_test_graph_route_prefix_is_strict_and_configurable @go_TestGraphHTTPThroughUMCP @go_TestUIAuditServer
    Scenario: test_graph_route_prefix_is_strict_and_configurable
      Given the pinned Python reference fixtures and controlled inputs
      When request(handler, "/debug-graph").
      Then request(handler, '/graph') is None; page is not None and page.status == 200. expects GraphExplorerConfig(route_prefix=value) raises ValidationError.

    @py-15c8abc6bce1 @python_test_graph_search_post_returns_results @go_TestGraphHTTPThroughUMCP @go_TestUIAuditServer
    Scenario: test_graph_search_post_returns_results
      Given the pinned Python reference fixtures and controlled inputs
      When _Snapshot(); cast(GraphSnapshotService, snapshot); request( handler, "/graph/api/v1/search", method="POST", body=b'{"query":"alpha graph"}', ).
      Then response is not None and response.status == 200; json.loads(response.body)['results'][0]['path'] == '/projects/a.md'; invalid is not None and invalid.status == 400.

  Rule: Behavior captured from test_graph_diagnostics.py

    @py-d80c013a66b9 @python_test_diagnostics_are_explainable_and_keep_semantics_derived @go_TestExtendedDiagnosticBranches @go_TestExtendedDiagnosticsFixture
    Scenario: test_diagnostics_are_explainable_and_keep_semantics_derived
      Given the pinned Python reference fixtures and controlled inputs
      When node("b", embedding=GraphEmbeddingState(status="missing")); diagnose_graph( nodes, (), revisions=GraphRevisions(repository="rev", index="old", embedding="old", stale=True), content_hashes={"a": "same…; item.rule.startswith("embedding_").
      Then {'orphan', 'broken_links', 'index_stale', 'embedding_missing', 'embedding_failed', 'pending_proposals', 'exact_duplicate', 'size_outlier', 'tag_drift'} <= rules; semantic and all((item.derived for item in semantic)); all((item.measured for item in diagnostics)); with_ids[0].anomaly_ids.

    @py-acd72734954f @python_test_namespace_outlier_uses_only_explicit_edges @go_TestExtendedDiagnosticBranches @go_TestExtendedDiagnosticsFixture
    Scenario: test_namespace_outlier_uses_only_explicit_edges
      Given the pinned Python reference fixtures and controlled inputs
      When node("b", namespace="/systems/"); diagnose_graph( nodes, (edge("a", "b"), edge("a", "c"), edge("a", "d")), revisions=GraphRevisions(repository="rev", index="rev", stale=Fals….
      Then outlier.concept_ids == ('a',); outlier.derived is False.

  Rule: Behavior captured from test_graph_export.py

    @py-4550a7b1b9bf @python_test_json_and_svg_exports_are_bounded_and_safe @go_TestGraphHTTPExport
    Scenario: test_json_and_svg_exports_are_bounded_and_safe
      Given the pinned Python reference fixtures and controlled inputs
      When _node("b", "B"); export_graph_json(nodes, edges, revisions=revisions, settings={"size": "combined"}); export_graph_svg(nodes, edges).decode().
      Then len(payload['nodes']) == 2; 'embedding_blob' not in encoded.decode(); 'authorization' not in encoded.decode().casefold(); svg starts with '<svg'; '<script>' not in svg; '&lt;script&gt;' in svg.

  Rule: Behavior captured from test_graph_layout.py

    @py-042337a8f00c @python_test_aggregate_layout_groups_sparse_namespace_isolates @go_TestAggregateLayoutBranches @go_TestAggregateLayoutFixtures
    Scenario: test_aggregate_layout_groups_sparse_namespace_isolates
      Given the pinned Python reference fixtures and controlled inputs
      When range(10); aggregate_layout(nodes, (), repository_revision="rev", cluster_limit=20).
      Then len(layout.clusters) == 1; layout.clusters[0].namespace == '/skills/'; layout.clusters[0].member_count == 10.

    @py-79fc1de84e1e @python_test_aggregate_layout_handles_isolates_and_cluster_bound @go_TestAggregateLayoutBranches @go_TestAggregateLayoutFixtures
    Scenario: test_aggregate_layout_handles_isolates_and_cluster_bound
      Given the pinned Python reference fixtures and controlled inputs
      When range(20); aggregate_layout(nodes, (), repository_revision="rev", cluster_limit=5).
      Then len(layout.clusters) == 5; sum((cluster.member_count for cluster in layout.clusters)) == 20; any((cluster.id == 'cluster:overflow' for cluster in layout.clusters)).

    @py-4df4552c7f20 @python_test_aggregate_layout_is_deterministic_and_input_order_independent @go_TestAggregateLayoutBranches @go_TestAggregateLayoutFixtures
    Scenario: test_aggregate_layout_is_deterministic_and_input_order_independent
      Given the pinned Python reference fixtures and controlled inputs
      When node(0); edge(0, 1); aggregate_layout(nodes, edges, repository_revision="rev", cluster_limit=10).
      Then first == second; len(first.clusters) == 2; len(first.edges) == 1; sum((cluster.member_count for cluster in first.clusters)) == 4; all((coordinate == coordinate for cluster in first.clusters for coordinate in (cluster.coarse_position.x, cluster.coarse_position.y, cluster.coarse_position.z))).

    @py-ff450127bc69 @python_test_aggregate_semantic_edges_retain_similarity @go_TestAggregateLayoutBranches @go_TestAggregateLayoutFixtures
    Scenario: test_aggregate_semantic_edges_retain_similarity
      Given the pinned Python reference fixtures and controlled inputs
      When node(1, "/a/"); edge(1, 2).model_copy(update={"kind": "semantic_similarity", "similarity": 0.91}); aggregate_layout(nodes, (semantic,), repository_revision="revision", cluster_limit=10).
      Then result.edges[0].similarity == 0.91; result.edges[0].canonical is False.

    @py-ee3f6b20c9de @python_test_all_shared_forces_have_bounded_relationships_and_do_not_affect_canonical_layout @go_TestAggregateLayoutBranches @go_TestAggregateLayoutFixtures
    Scenario: test_all_shared_forces_have_bounded_relationships_and_do_not_affect_canonical_layout
      Given the pinned Python reference fixtures and controlled inputs
      When node(i, tags=("same",)).model_copy(update={"provenance_keys": ("hashed-ref",)}); _overlay_edges(nodes, "rev", 100).
      Then {item.kind for item in edges} == {'shared_tag', 'shared_namespace', 'shared_type', 'shared_provenance'}; len(edges) == 12; all((not item.canonical and item.resolution == 'derived' for item in edges)); len(_overlay_edges(nodes, 'rev', 4)) == 4; len({item.kind for item in _overlay_edges(nodes, 'rev', 4)}) == 4; _overlay_edges(list(reversed(nodes)), 'rev', 100) == edges.

    @py-3a715df4347a @python_test_overlay_edges_connect_adjacent_members_with_shared_tags @go_TestAggregateLayoutBranches @go_TestAggregateLayoutFixtures
    Scenario: test_overlay_edges_connect_adjacent_members_with_shared_tags
      Given the pinned Python reference fixtures and controlled inputs
      When node(3, tags=("other",)); _overlay_edges(nodes, "rev", 10).
      Then [(edge.kind, edge.raw_target) for edge in edges if edge.kind == 'shared_tag'] == [('shared_tag', 'other'), ('shared_tag', 'shared'), ('shared_tag', 'shared')]; all((edge.canonical is False and edge.resolution == 'derived' for edge in edges)).

    @py-843036da6e5a @python_test_scale_fixtures_are_bounded @go_TestAggregateLayoutBranches @go_TestAggregateLayoutFixtures
    Scenario: test_scale_fixtures_are_bounded
      Given the pinned Python reference fixtures and controlled inputs
      When range(size).
      Then len(layout.clusters) <= 500; sum((cluster.member_count for cluster in layout.clusters)) == size; len(layout.edges) <= len(edges).

    @py-0f69b2bde618 @python_test_sparse_overview_detection_requires_many_orphaned_nodes @go_TestAggregateLayoutBranches @go_TestAggregateLayoutFixtures
    Scenario: test_sparse_overview_detection_requires_many_orphaned_nodes
      Given the pinned Python reference fixtures and controlled inputs
      When range(20).
      Then _is_sparse_overview(sparse_nodes, ()) is True; _is_sparse_overview(sparse_nodes, [edge(index, index + 1) for index in range(10)]) is False; _is_sparse_overview(sparse_nodes[:4], ()) is False.

    @py-6d0b1ca67b67 @python_test_trash_is_named_and_reserved_when_clusters_overflow @go_TestAggregateLayoutBranches @go_TestAggregateLayoutFixtures
    Scenario: test_trash_is_named_and_reserved_when_clusters_overflow
      Given the pinned Python reference fixtures and controlled inputs
      When range(5); nodes.append(node(5, "/trash/")); aggregate_layout(nodes, (), repository_revision="revision", cluster_limit=3).
      Then trash.label == 'Trash'; trash.member_count == 1.

  Rule: Behavior captured from test_graph_refresh.py

    @py-96dbdf27b222 @python_test_dead_worker_is_unavailable_and_enqueue_rejected @go_TestRefreshErrorFixture @go_TestRefreshFixture
    Scenario: test_dead_worker_is_unavailable_and_enqueue_rejected
      Given the pinned Python reference fixtures and controlled inputs
      When coordinator(tmp_path, DeadWorker()); refresh.state_dict().
      Then state['alive'] is False and state['available'] is False; state['last_error'] == 'embedding worker stopped unexpectedly'. expects refresh.enqueue(scope="selected", concept_ids=("a",)) raises GraphSnapshotError matching "stopped".

    @py-5fcd3772d35e @python_test_refresh_rejects_unknown_bounds_and_unavailable_worker @go_TestRefreshErrorFixture @go_TestRefreshFixture
    Scenario: test_refresh_rejects_unknown_bounds_and_unavailable_worker
      Given the pinned Python reference fixtures and controlled inputs
      When _Worker(); coordinator(tmp_path, worker); coordinator(tmp_path).
      Then unavailable.state().available is False. expects refresh.enqueue(scope="selected", concept_ids=("missing",)) raises GraphSnapshotError matching "unknown"; refresh.enqueue(scope="visible", concept_ids=("a", "b", "c")) raises GraphSnapshotError matching "path limit"; refresh.enqueue(scope="other") raises GraphSnapshotError matching "unsupported".

    @py-138c7f2eca26 @python_test_selected_visible_and_confirmed_full_refresh @go_TestRefreshErrorFixture @go_TestRefreshFixture
    Scenario: test_selected_visible_and_confirmed_full_refresh
      Given the pinned Python reference fixtures and controlled inputs
      When _Worker(); coordinator(tmp_path, worker); refresh.enqueue(scope="selected", concept_ids=("a",)).
      Then selected.available and selected.alive and selected.queued_paths == 1; selected.pause_reason == 'interactive'; selected.completed == 4; worker.calls[-1] == (tmp_path, 'rev', ('/projects/a.md',)); visible_paths is not None; tuple(visible_paths) == ('/projects/a.md', '/projects/b.md'). expects refresh.enqueue(scope="full") raises GraphSnapshotError matching "confirmation".

  Rule: Behavior captured from test_graph_snapshot.py

    @py-1b3b638c29fb @python_test_detail_and_neighbourhood_are_bounded_and_revision_aware @go_TestOverviewFixture @go_TestSemanticOverviewFixture
    Scenario: test_detail_and_neighbourhood_are_bounded_and_revision_aware
      Given the pinned Python reference fixtures and controlled inputs
      When _snapshot(tmp_path); service.detail("a-id"); service.neighbourhood("a-id").
      Then detail.node.updated_by == 'tester'; detail.preview == 'Alpha body w'; detail.preview_truncated is True; len(detail.outbound) == 1; detail.outbound[0].canonical is True; detail.outbound[0].kind == 'explicit'. expects service.neighbourhood("a-id", depth=2) raises GraphSnapshotError matching "depth"; service.detail("missing") raises GraphSnapshotError matching "unknown"; service.expand_cluster("missing") raises GraphSnapshotError matching "unknown cluster".

    @py-5de1ddab867c @python_test_graph_search_uses_fts_and_returns_bounded_metadata @go_TestOverviewFixture @go_TestSemanticOverviewFixture
    Scenario: test_graph_search_uses_fts_and_returns_bounded_metadata
      Given the pinned Python reference fixtures and controlled inputs
      When _snapshot(tmp_path); service.search("alpha shared").
      Then payload['schema_version'] == 1; payload['results'] == [{'id': 'a-id', 'path': '/projects/a.md', 'title': 'A', 'type': 'project', 'tags': ('graph',), 'snippet': 'Alpha body with more text'}]. expects service.search("---") raises GraphSnapshotError matching "contain words".

    @py-73b6b14f4889 @python_test_overview_adds_bounded_semantic_edges_without_vectors @go_TestOverviewFixture @go_TestSemanticOverviewFixture
    Scenario: test_overview_adds_bounded_semantic_edges_without_vectors
      Given the pinned Python reference fixtures and controlled inputs
      When _snapshot(tmp_path, direct_limit=2).overview(); overview.model_dump_json().
      Then len(semantic) == 1; {edge.source, edge.target} == {'a-id', 'b-id'}; edge.similarity is not None and edge.similarity > 0.99; edge.model_id == 'gte'; edge.embedding_revision == 'rev-1'; 'SECRET-VECTOR' not in payload.

    @py-5aa00ead5437 @python_test_overview_is_bounded_deterministic_and_omits_vectors @go_TestOverviewFixture @go_TestSemanticOverviewFixture
    Scenario: test_overview_is_bounded_deterministic_and_omits_vectors
      Given the pinned Python reference fixtures and controlled inputs
      When _snapshot(tmp_path, direct_limit=1); service.overview(); service.expand_cluster(cluster.id).
      Then first == second; isinstance(first, GraphOverview); first.truncated is True; first.mode == 'aggregated'; first.nodes == (); sum((cluster.member_count for cluster in first.clusters)) == 2.

    @py-62ab0aa7f402 @python_test_simulated_prefix_scope_hides_nodes_search_links_and_metrics @go_TestOverviewFixture @go_TestSemanticOverviewFixture
    Scenario: test_simulated_prefix_scope_hides_nodes_search_links_and_metrics
      Given the pinned Python reference fixtures and controlled inputs
      When _snapshot(tmp_path, direct_limit=10, edge_limit=1); _write_concept( service._repository_root, "/private/c.md", concept_id="c-id", title="Private C", body="Hidden alpha secret.\n", ); connection.execute("INSERT INTO graph_metrics VALUES(?,?,?,?,?)", ("c-id", 1, 1, 0, 0)).
      Then overview.mode == 'direct'; {node.path for node in overview.nodes} == {'/projects/a.md', '/projects/b.md'}; overview.metrics.memory_count == 2; overview.metrics.explicit_edges == 1; overview.metrics.broken_edges == 0; overview.metrics.orphan_count == 0. expects service.detail("c-id", policy=policy) raises GraphSnapshotError matching "unknown memory"; service.export_selection(("a-id", "c-id"), policy=policy) raise

    @py-46746714b87a @python_test_snapshot_reports_stale_revisions @go_TestOverviewFixture @go_TestSemanticOverviewFixture
    Scenario: test_snapshot_reports_stale_revisions
      Given the pinned Python reference fixtures and controlled inputs
      When _snapshot(tmp_path); sqlite3.connect(service._derived_db_path); connection.execute("UPDATE index_state SET value='rev-0' WHERE key='index_revision'").
      Then service.overview().revisions.stale is True.

  Rule: Behavior captured from test_graph_vendor.py

    @py-07f1211e94bc @python_test_graph_application_assets_are_available_as_package_resources @go_TestGraphStaticAssets
    Scenario: test_graph_application_assets_are_available_as_package_resources
      Given the pinned Python reference fixtures and controlled inputs
      When files("memento.graph_debug").joinpath("static"); root.joinpath(relative).is_file().
      Then root.joinpath(relative).is_file().

    @py-a7ef4dd27ab2 @python_test_vendored_graph_modules_match_manifest_and_ship_licences @go_TestGraphStaticAssets
    Scenario: test_vendored_graph_modules_match_manifest_and_ship_licences
      Given the pinned Python reference fixtures and controlled inputs
      When files("memento.graph_debug").joinpath("static", "vendor"); root.joinpath("manifest.json").read_text(encoding="utf-8"); root.joinpath(item["file"]).read_bytes().
      Then manifest['schema_version'] == 1; [item['name'] for item in manifest['libraries']] == ['three', 'three-core', 'preact', 'preact-hooks']; hashlib.sha256(body).hexdigest() == item['sha256']; item['version']; item['license'] == 'MIT'; 'Three.js 0.180.0' in licences.
