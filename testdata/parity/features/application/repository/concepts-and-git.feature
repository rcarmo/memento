Feature: repository/concepts and git

  The scenarios capture Python Memento behavior at 7f29e8b003557f0105f47ed353b7f65a33619456.
  Rules retain source-module traceability while features group related user behavior.

  Rule: Behavior captured from test_repository_core.py

    @py-8652ddb7d662 @python_test_authorization_rejects_noncanonical_paths @go_TestPythonBundleFixtures @go_TestPythonStructuralLinks
    Scenario: test_authorization_rejects_noncanonical_paths
      Given the pinned Python reference fixtures and controlled inputs
      When resolve_policy(config, Principal(name="reader", roles=("reader",))). expects authorize_path(policy, path, action="read") raises AuthorizationError.
      Then the result, state transitions, and error boundary match the captured behavior

    @py-abe841ad77b0 @python_test_concept_body_normalization_preserves_unicode_code_points @go_TestPythonBundleFixtures @go_TestPythonStructuralLinks
    Scenario: test_concept_body_normalization_preserves_unicode_code_points
      Given the pinned Python reference fixtures and controlled inputs
      When checks normalize_concept_body(body) == body; normalize_concept_body(f'\r\n{body} \r\n') == body.
      Then the result, state transitions, and error boundary match the captured behavior

    @py-4215c22f8c78 @python_test_concept_schema_validation_model @go_TestPythonBundleFixtures @go_TestPythonStructuralLinks
    Scenario: test_concept_schema_validation_model
      Given the pinned Python reference fixtures and controlled inputs
      When ConceptFrontmatter.model_validate( { "schema_version": 1, "id": "5c8fd31c-35f4-4fb2-a9b7-dd2e5935443d", "type": "instance", "title": "Smith….
      Then document.frontmatter.aliases == ('smith',); document.frontmatter.tags == ('a', 'z'). expects ConceptFrontmatter.model_validate( { "schema_version": 2, "id": "x", "type": "invalid", "title": "Smith", "created_at": "2026-07-16T19:00:0… raises ValidationError; ConceptFrontmatter.model_validate( { "schema_version": 1, "id": "ok", "type": "instance", "title": "Bad\nTitle", "created_at": "2026-07-16T… raises ValidationError; ConceptFrontmatter.model_validate( { "schema_version": 1, "id": "ok", "type": "instance", "title": "Smith", "created_at":

    @py-7049a31a81c5 @python_test_concept_serialization_is_thread_safe @go_TestPythonBundleFixtures @go_TestPythonStructuralLinks
    Scenario: test_concept_serialization_is_thread_safe
      Given the pinned Python reference fixtures and controlled inputs
      When parse_concept_text(VALID_CONCEPT); serialize_concept(document); executor.map(lambda _: serialize_concept(document), range(100)).
      Then outputs == [expected] * 100.

    @py-b54fa05fe746 @python_test_deterministic_index_and_log_generation @go_TestPythonBundleFixtures @go_TestPythonStructuralLinks
    Scenario: test_deterministic_index_and_log_generation
      Given the pinned Python reference fixtures and controlled inputs
      When (bundle_root / "instances").mkdir(parents=True); (bundle_root / "projects").mkdir(parents=True); (bundle_root / "instances" / "smith.md").write_text(VALID_CONCEPT, encoding="utf-8").
      Then indexes['/'] == generate_directory_indexes(bundle)['/']; '[instances](/instances/index.md)' in indexes['/']; '[projects](/projects/index.md)' in indexes['/']; log_output == generate_root_log(bundle); '[Smith](/instances/smith.md)' in log_output.

    @py-332fb56eeacd @python_test_envelopes_are_strict @go_TestPythonBundleFixtures @go_TestPythonStructuralLinks
    Scenario: test_envelopes_are_strict
      Given the pinned Python reference fixtures and controlled inputs
      When success_envelope({"ok": True}, repo_revision="abc", index_revision="abc"); error_envelope("validation_error", "bad request").
      Then success.model_dump()['status'] == 'success'; error.model_dump()['status'] == 'error'. expects success.__class__.model_validate({"status": "success", "data": {}, "repo_revision": "a"}) raises ValidationError.

    @py-5b169f241e0b @python_test_extract_links_and_rewrite_rename @go_TestPythonBundleFixtures @go_TestPythonStructuralLinks
    Scenario: test_extract_links_and_rewrite_rename
      Given the pinned Python reference fixtures and controlled inputs
      When extract_structural_links(body); rewrite_links_for_rename( body, old_path="/projects/piclaw.md", new_path="/projects/piclaw-core.md", ).
      Then len(links) == 1; links[0].href == '/projects/piclaw.md#overview'; rewritten.changed is True; '/projects/piclaw-core.md#overview' in rewritten.content; '/projects/piclaw-core.md' in rewritten.content.

    @py-65fc712f270f @python_test_frontmatter_parse_and_deterministic_serialize @go_TestPythonBundleFixtures @go_TestPythonStructuralLinks
    Scenario: test_frontmatter_parse_and_deterministic_serialize
      Given the pinned Python reference fixtures and controlled inputs
      When parse_concept_text(VALID_CONCEPT); serialize_concept(document); serialize_concept(parse_concept_text(serialized_once)).
      Then serialized_once == serialized_twice; ' - assistant' in serialized_once; serialized_once ends with '\n'; 'smith-piclaw\n' in serialized_once.

    @py-8d7d25f29240 @python_test_frontmatter_rejects_unknown_keys @go_TestPythonBundleFixtures @go_TestPythonStructuralLinks
    Scenario: test_frontmatter_rejects_unknown_keys
      Given the pinned Python reference fixtures and controlled inputs
      When VALID_CONCEPT.replace("updated_by: rui/tablet", "updated_by: rui/tablet\nunknown: no"). expects parse_concept_text(invalid) raises FrontmatterError.
      Then the result, state transitions, and error boundary match the captured behavior

    @py-cd616a5569a7 @python_test_generated_indexes_and_log_escape_markdown_titles_and_authors @go_TestPythonBundleFixtures @go_TestPythonStructuralLinks
    Scenario: test_generated_indexes_and_log_escape_markdown_titles_and_authors
      Given the pinned Python reference fixtures and controlled inputs
      When (bundle_root / "projects").mkdir(parents=True); VALID_CONCEPT.replace("title: Smith", "title: Link [trap](x)").replace( "updated_by: rui/tablet", "updated_by: agent_*`demo`" ); (bundle_root / "projects" / "escaped.md").write_text(escaped, encoding="utf-8").
      Then '[Link \\[trap\\]\\(x\\)](/projects/escaped.md)' in indexes['/projects/']; 'agent\\_\\*\\`demo\\`' in log_output.

    @py-6ab123080ad0 @python_test_protected_namespace_prefixes_are_strict @go_TestPythonBundleFixtures @go_TestPythonStructuralLinks
    Scenario: test_protected_namespace_prefixes_are_strict
      Given the pinned Python reference fixtures and controlled inputs
      When expects AuthorizationConfig(principals={}, protected_read_prefixes=(prefix,)) raises ValidationError matching "protected namespace prefixes".
      Then the result, state transitions, and error boundary match the captured behavior

    @py-7c4209f54ddd @python_test_protected_namespaces_require_explicit_read_grants @go_TestPythonBundleFixtures @go_TestPythonStructuralLinks
    Scenario: test_protected_namespaces_require_explicit_read_grants
      Given the pinned Python reference fixtures and controlled inputs
      When resolve_policy(authorization, Principal(name="root-reader", roles=("reader",))); authorize_path(root_reader, "/projects/visible.md", action="read"); resolve_policy( authorization, Principal(name="protected-reader", roles=("reader",)) ).
      Then broad_read_grant_warning(roles=root_reader.roles, read_prefixes=root_reader.read_prefixes, protected_read_prefixes=root_reader.protected_read_prefixes) is not None; broad_read_grant_warning(roles=protected_reader.roles, read_prefixes=protected_reader.read_prefixes, protected_read_prefixes=protected_reader.protected_read_prefixes) is None; broad_read_grant_warning(roles=admin.roles, read_prefixes=admin.read_prefixes, protected_read_prefixes=admin.

    @py-055c54e5d303 @python_test_rename_preserves_markdown_source_and_code @go_TestPythonBundleFixtures @go_TestPythonStructuralLinks
    Scenario: test_rename_preserves_markdown_source_and_code
      Given the pinned Python reference fixtures and controlled inputs
      When text.replace("](/old.md#anchor)", "](/new.md#anchor)").replace( "[ref]: /old.md", "[ref]: /new.md" ); rewrite_links_for_rename(text, old_path="/old.md", new_path="/new.md").
      Then result.content == expected; len(extract_structural_links(result.content)) == 2.

    @py-708a5e3a40b2 @python_test_repository_audit_reports_duplicates_and_broken_links @go_TestPythonBundleFixtures @go_TestPythonStructuralLinks
    Scenario: test_repository_audit_reports_duplicates_and_broken_links
      Given the pinned Python reference fixtures and controlled inputs
      When (bundle_root / "instances").mkdir(parents=True); (bundle_root / "projects").mkdir(parents=True); (bundle_root / "instances" / "smith.md").write_text(VALID_CONCEPT, encoding="utf-8").
      Then 'duplicate_id' in codes; 'broken_link' in codes; audit.generated_indexes['/'] starts with '# Index\n'; audit.generated_log starts with '# Mutation Log\n'.

    @py-557fce9b171a @python_test_safe_path_containment_rejects_traversal_symlink_special_and_reserved @go_TestPythonBundleFixtures @go_TestPythonStructuralLinks
    Scenario: test_safe_path_containment_rejects_traversal_symlink_special_and_reserved
      Given the pinned Python reference fixtures and controlled inputs
      When (tmp_path / "instances").mkdir(); validate_repository_write_path(tmp_path, "/instances/smith.md"); target.mkdir().
      Then safe.absolute_path == tmp_path / 'instances' / 'smith.md'; stat.S_ISFIFO(os.lstat(fifo_path).st_mode). expects validate_repository_write_path(tmp_path, "/../etc/passwd") raises PathSafetyError; validate_repository_write_path(tmp_path, "/index.md") raises PathSafetyError; validate_repository_write_path(tmp_path, "/linked/escape.md") raises PathSafetyError.

    @py-c9a534c038d3 @python_test_strict_config_and_authorization @go_TestPythonBundleFixtures @go_TestPythonStructuralLinks
    Scenario: test_strict_config_and_authorization
      Given the pinned Python reference fixtures and controlled inputs
      When ServiceConfig.model_validate( { "schema_version": 2, "repository": {"root_path": "/tmp/repo", "bundle_root": "/"}, "authorization": { "prin…; resolve_policy(config.authorization, principal); authorize_path(policy, "/instances/smith.md", action="read").
      Then filter_authorized_paths(policy, ['/instances/a.md', '/secret/a.md'], action='read') == ['/instances/a.md']. expects authorize_path(policy, "/secret/a.md", action="read") raises AuthorizationError; ServiceConfig.model_validate( { "schema_version": 2, "repository": {"root_path": "/tmp/repo", "bundle_root": "/"}, "authorization": {"princ… raises ValidationError.

    @py-e2211d55a76a @python_test_write_path_rejects_dangling_symlink @go_TestPythonBundleFixtures @go_TestPythonStructuralLinks
    Scenario: test_write_path_rejects_dangling_symlink
      Given the pinned Python reference fixtures and controlled inputs
      When root.mkdir(); (root / "link.md").symlink_to(outside).
      Then not outside.exists(). expects validate_repository_write_path(root, "/link.md") raises PathSafetyError.

  Rule: Behavior captured from test_legacy_blob_migration.py

    @py-d48f5eb8a065 @python_test_migrates_pointer_and_removes_legacy_filter_attributes @go_TestMigrateLegacyBlobs
    Scenario: test_migrates_pointer_and_removes_legacy_filter_attributes
      Given the pinned Python reference fixtures and controlled inputs
      When worktree.mkdir(); (worktree / asset).parent.mkdir(parents=True); (worktree / asset).write_bytes(pointer(contents)).
      Then repository_needs_legacy_blob_migration(worktree); changed == ('/.assets/concept/templates/1.0.0.zip', '/.gitattributes'); (worktree / asset).read_bytes() == contents; (worktree / '.gitattributes').read_text(encoding='utf-8') == '*.txt text\n'; not repository_needs_legacy_blob_migration(worktree).

    @py-5a5283758d5b @python_test_rejects_cached_blob_with_wrong_digest @go_TestMigrateLegacyBlobs
    Scenario: test_rejects_cached_blob_with_wrong_digest
      Given the pinned Python reference fixtures and controlled inputs
      When worktree.mkdir(); hashlib.sha256(wanted).hexdigest(); object_path.parent.mkdir(parents=True). expects migrate_legacy_blobs_to_git(worktree, object_root=objects) raises ValueError matching "failed verification".
      Then the result, state transitions, and error boundary match the captured behavior
