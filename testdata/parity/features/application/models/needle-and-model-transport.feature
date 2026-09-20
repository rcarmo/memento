Feature: models/needle and model transport

  The scenarios capture Python Memento behavior at 7f29e8b003557f0105f47ed353b7f65a33619456.
  Rules retain source-module traceability while features group related user behavior.

  Rule: Behavior captured from test_model_transport.py

    @py-153148b58aa8 @python_test_model_response_is_bounded @go_TestEndpointModelClients
    Scenario: test_model_response_is_bounded
      Given the pinned Python reference fixtures and controlled inputs
      When monkeypatch.setattr(response, "read", read); monkeypatch.setattr("urllib.request.urlopen", urlopen).
      Then sizes == [1048577]. expects client.complete( ModelRequest(task="answer", prompt="test", max_output_chars=100, timeout_seconds=1) ) raises ModelClientError matching "1 MiB".

  Rule: Behavior captured from test_needle_corpus.py

    @py-9d8385eacc14 @python_test_router_v2_generator_matches_vendored_corpus_manifest @go_TestRealNeedleCorpus
    Scenario: test_router_v2_generator_matches_vendored_corpus_manifest
      Given the pinned Python reference fixtures and controlled inputs
      When _generator(root); (root / "docs/evidence/needle/router-v2-manifest.json").read_text(encoding="utf-8"); generator.build_manifest_and_rows().
      Then manifest == expected; {name: len(items) for name, items in rows.items()} == {'train': 1440, 'val': 360, 'test': 360}.

  Rule: Behavior captured from test_needle_ffi.py

    @py-1b3b0de5736b @python_test_needle_router_config_defaults_are_disabled_with_default_paths @go_TestRealNeedleModel
    Scenario: test_needle_router_config_defaults_are_disabled_with_default_paths
      Given the pinned Python reference fixtures and controlled inputs
      When checks config.needle_router.enabled is False; config.needle_router.ffi_library_path ends with 'libmemento_needle_ffi.so'; config.needle_router.model_path ends with 'memento-router.ndl'; config.needle_router.tokenizer_path ends with 'needle.model'.
      Then the result, state transitions, and error boundary match the captured behavior

    @py-fe2dbf04711e @python_test_python_ctypes_wrapper_router_lifecycle_generate_cancel_and_errors @go_TestRealNeedleModel
    Scenario: test_python_ctypes_wrapper_router_lifecycle_generate_cancel_and_errors
      Given the pinned Python reference fixtures and controlled inputs
      When require_prepared_model(); build_rust_cdylib("memento-needle-ffi", "libmemento_needle_ffi"); token.cancel().
      Then token.pointer is not None; info.abi_version == 1; info.d_model == 512; info.vocab_size == 8192; 'memory_search' in result; 'Piclaw' in result. expects router.generate("Find Piclaw", DEFAULT_TOOLS_JSON, cancelled=lambda: True) raises NeedleFfiCancelledError; router.generate("Find Piclaw", DEFAULT_TOOLS_JSON, max_gen_len=0) raises NeedleFfiBoundsError; closed_router.info() raises NeedleFfiClosedError.

    @py-b7e9d7fada49 @python_test_real_ffi_router_output_parses_to_one_action @go_TestRealNeedleModel
    Scenario: test_real_ffi_router_output_parses_to_one_action
      Given the pinned Python reference fixtures and controlled inputs
      When require_prepared_model(); build_rust_cdylib("memento-needle-ffi", "libmemento_needle_ffi"); parse_needle_router_output( router.generate("find Piclaw", CANONICAL_TRAINED_SHALLOW_TOOLS_JSON) ).
      Then parsed.action in {'search_then_read', 'search_paths', 'status_field', 'search_then_graph', 'read_field', 'UNKNOWN'}.

  Rule: Behavior captured from test_needle_router_generator.py

    @py-fc8d34f2c437 @python_test_generated_answers_validate_against_router_adapter @go_TestRealNeedleGeneration
    Scenario: test_generated_answers_validate_against_router_adapter
      Given the pinned Python reference fixtures and controlled inputs
      When _load_generator_module(); module.build_manifest_and_rows(); rows_by_split.values().
      Then {split: info['count'] for split, info in manifest['splits'].items()} == expected_counts; len(answers) == 1; parsed.action == answer['name'].

    @py-90c9dcc01c34 @python_test_generator_uses_valid_historical_router_field_enums @go_TestRealNeedleGeneration
    Scenario: test_generator_uses_valid_historical_router_field_enums
      Given the pinned Python reference fixtures and controlled inputs
      When _load_generator_module().
      Then set(module.STATUS_FIELDS).issubset(set(STATUS_FIELDS)); set(module.READ_FIELDS).issubset(set(READ_FIELDS)); 'frontmatter' not in module.READ_FIELDS.
