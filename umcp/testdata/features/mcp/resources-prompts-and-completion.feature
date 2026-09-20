Feature: mcp/resources prompts and completion

  The scenarios capture Python uMCP behavior at 30cce7dfe08c6ee63de235f7d81754ba286dafbb.
  Rules retain source-module traceability while features group related user behavior.

  Rule: Behavior captured from test_async_prompts.py

    @umcp-272bab2fca9a @go_TestConcurrentPromptRegistry @go_TestPromptLifecycleAndEdges @go_TestPromptMetadataEdges
    Scenario: test_async_prompts_list
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When async prompts list
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-744264bc0329 @go_TestConcurrentPromptRegistry @go_TestPromptLifecycleAndEdges @go_TestPromptMetadataEdges
    Scenario: test_async_prompt_get_sync_return
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When async prompt get sync return
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-d5051bec5c94 @go_TestConcurrentPromptRegistry @go_TestPromptLifecycleAndEdges @go_TestPromptMetadataEdges
    Scenario: test_async_prompt_get_async_return
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When async prompt get async return
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-9c049739c356 @go_TestConcurrentPromptRegistry @go_TestPromptLifecycleAndEdges @go_TestPromptMetadataEdges
    Scenario: test_async_prompts_missing_required_argument_returns_invalid_params
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When async prompts missing required argument returns invalid params
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

  Rule: Behavior captured from test_completion_logging.py

    @umcp-21d34e1828c0 @go_TestCompletionProviderAndLimits @go_TestCompletionSyncAsyncParity @go_TestServerLoggingAndNotifications
    Scenario: test_sync_initialize_only_advertises_logging_without_completion_support
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When sync initialize only advertises logging without completion support
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-3c26188c4bb3 @go_TestCompletionProviderAndLimits @go_TestCompletionSyncAsyncParity @go_TestServerLoggingAndNotifications
    Scenario: test_sync_initialize_advertises_completions_exactly_when_available
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When sync initialize advertises completions exactly when available
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-782a17b97200 @go_TestCompletionProviderAndLimits @go_TestCompletionSyncAsyncParity @go_TestServerLoggingAndNotifications
    Scenario: test_async_initialize_advertises_completions_exactly_when_available
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When async initialize advertises completions exactly when available
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-28db721b0126 @go_TestCompletionProviderAndLimits @go_TestCompletionSyncAsyncParity @go_TestServerLoggingAndNotifications
    Scenario: test_sync_completion_supports_prompt_and_resource_refs_literal_and_enum
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When sync completion supports prompt and resource refs literal and enum
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-d0979968f941 @go_TestCompletionProviderAndLimits @go_TestCompletionSyncAsyncParity @go_TestServerLoggingAndNotifications
    Scenario: test_completion_supports_registered_sync_and_async_providers_and_context[False]
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When completion supports registered sync and async providers and context[False]
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-7facb781e10d @go_TestCompletionProviderAndLimits @go_TestCompletionSyncAsyncParity @go_TestServerLoggingAndNotifications
    Scenario: test_completion_supports_registered_sync_and_async_providers_and_context[True]
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When completion supports registered sync and async providers and context[True]
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-9c39b3b32a0d @go_TestCompletionProviderAndLimits @go_TestCompletionSyncAsyncParity @go_TestServerLoggingAndNotifications
    Scenario: test_completion_supports_schema_enums_for_registered_prompts[False]
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When completion supports schema enums for registered prompts[False]
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-f6562fd6dd04 @go_TestCompletionProviderAndLimits @go_TestCompletionSyncAsyncParity @go_TestServerLoggingAndNotifications
    Scenario: test_completion_supports_schema_enums_for_registered_prompts[True]
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When completion supports schema enums for registered prompts[True]
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-66cbfffe7d5c @go_TestCompletionProviderAndLimits @go_TestCompletionSyncAsyncParity @go_TestServerLoggingAndNotifications
    Scenario: test_completion_limits_to_max_100_and_sets_total_and_has_more[False]
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When completion limits to max 100 and sets total and has more[False]
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-5f965d32bee1 @go_TestCompletionProviderAndLimits @go_TestCompletionSyncAsyncParity @go_TestServerLoggingAndNotifications
    Scenario: test_completion_limits_to_max_100_and_sets_total_and_has_more[True]
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When completion limits to max 100 and sets total and has more[True]
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-02cb98105cc2 @go_TestCompletionProviderAndLimits @go_TestCompletionSyncAsyncParity @go_TestServerLoggingAndNotifications
    Scenario: test_completion_invalid_refs_args_and_outputs_are_remote_safe[False]
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When completion invalid refs args and outputs are remote safe[False]
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-cd96b78d0b31 @go_TestCompletionProviderAndLimits @go_TestCompletionSyncAsyncParity @go_TestServerLoggingAndNotifications
    Scenario: test_completion_invalid_refs_args_and_outputs_are_remote_safe[True]
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When completion invalid refs args and outputs are remote safe[True]
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-6fc09c396965 @go_TestCompletionProviderAndLimits @go_TestCompletionSyncAsyncParity @go_TestServerLoggingAndNotifications
    Scenario: test_logging_set_level_accepts_standard_levels_and_threshold_ordering[False]
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When logging set level accepts standard levels and threshold ordering[False]
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-50b508cb90a3 @go_TestCompletionProviderAndLimits @go_TestCompletionSyncAsyncParity @go_TestServerLoggingAndNotifications
    Scenario: test_logging_set_level_accepts_standard_levels_and_threshold_ordering[True]
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When logging set level accepts standard levels and threshold ordering[True]
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-b13d9a679db3 @go_TestCompletionProviderAndLimits @go_TestCompletionSyncAsyncParity @go_TestServerLoggingAndNotifications
    Scenario: test_sync_logging_stdio_payload_redacts_sensitive_keys_recursively
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When sync logging stdio payload redacts sensitive keys recursively
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-f1d7dd57275b @go_TestCompletionProviderAndLimits @go_TestCompletionSyncAsyncParity @go_TestServerLoggingAndNotifications
    Scenario: test_sync_logging_stdio_payload_preserves_data_when_sanitize_false
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When sync logging stdio payload preserves data when sanitize false
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-94d21ab49f39 @go_TestCompletionProviderAndLimits @go_TestCompletionSyncAsyncParity @go_TestServerLoggingAndNotifications
    Scenario: test_sync_logging_sse_payload_exact_and_logger_optional
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When sync logging sse payload exact and logger optional
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-5da7a47f7bde @go_TestCompletionProviderAndLimits @go_TestCompletionSyncAsyncParity @go_TestServerLoggingAndNotifications
    Scenario: test_async_logging_stdio_and_sse_payloads
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When async logging stdio and sse payloads
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

  Rule: Behavior captured from test_prompts.py

    @umcp-9ea07661246b @go_TestConcurrentPromptRegistry @go_TestPromptLifecycleAndEdges @go_TestPromptMetadataEdges
    Scenario: test_prompts_list
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When prompts list
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-a88fbd8ee45b @go_TestConcurrentPromptRegistry @go_TestPromptLifecycleAndEdges @go_TestPromptMetadataEdges
    Scenario: test_prompts_get_missing_required_argument_returns_invalid_params
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When prompts get missing required argument returns invalid params
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-fe003cc0bcd5 @go_TestConcurrentPromptRegistry @go_TestPromptLifecycleAndEdges @go_TestPromptMetadataEdges
    Scenario: test_prompts_get_with_arguments
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When prompts get with arguments
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-46974c62c915 @go_TestConcurrentPromptRegistry @go_TestPromptLifecycleAndEdges @go_TestPromptMetadataEdges
    Scenario: test_prompts_get_list_messages
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When prompts get list messages
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

  Rule: Behavior captured from test_prompts_extra.py

    @umcp-05f03b0975bf @go_TestConcurrentPromptRegistry @go_TestPromptLifecycleAndEdges @go_TestPromptMetadataEdges
    Scenario: test_prompts_list_returns_full_metadata
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When prompts list returns full metadata
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-7722ddd67393 @go_TestConcurrentPromptRegistry @go_TestPromptLifecycleAndEdges @go_TestPromptMetadataEdges
    Scenario: test_prompts_categories_are_parsed_from_docstring
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When prompts categories are parsed from docstring
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-d8e3e675e9d7 @go_TestConcurrentPromptRegistry @go_TestPromptLifecycleAndEdges @go_TestPromptMetadataEdges
    Scenario: test_prompts_arguments_track_required_vs_optional
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When prompts arguments track required vs optional
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-c5b6413129fa @go_TestConcurrentPromptRegistry @go_TestPromptLifecycleAndEdges @go_TestPromptMetadataEdges
    Scenario: test_prompts_get_string_return_wraps_as_user_message
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When prompts get string return wraps as user message
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-37a56c76dd35 @go_TestConcurrentPromptRegistry @go_TestPromptLifecycleAndEdges @go_TestPromptMetadataEdges
    Scenario: test_prompts_get_list_return_passes_messages_through
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When prompts get list return passes messages through
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-ffc79ac4ea9b @go_TestConcurrentPromptRegistry @go_TestPromptLifecycleAndEdges @go_TestPromptMetadataEdges
    Scenario: test_prompts_get_dict_return_preserves_top_level_fields
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When prompts get dict return preserves top level fields
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-1a2a13a6425b @go_TestConcurrentPromptRegistry @go_TestPromptLifecycleAndEdges @go_TestPromptMetadataEdges
    Scenario: test_prompts_get_missing_required_argument_returns_invalid_params
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When prompts get missing required argument returns invalid params
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-df6a0fd59368 @go_TestConcurrentPromptRegistry @go_TestPromptLifecycleAndEdges @go_TestPromptMetadataEdges
    Scenario: test_prompts_get_unknown_name_returns_error
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When prompts get unknown name returns error
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-3b6eb9ca3d8e @go_TestConcurrentPromptRegistry @go_TestPromptLifecycleAndEdges @go_TestPromptMetadataEdges
    Scenario: test_prompts_get_unknown_argument_returns_invalid_params
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When prompts get unknown argument returns invalid params
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-8f3be6bf1230 @go_TestConcurrentPromptRegistry @go_TestPromptLifecycleAndEdges @go_TestPromptMetadataEdges
    Scenario: test_initialize_declares_prompts_capability
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When initialize declares prompts capability
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-924b5bc25605 @go_TestConcurrentPromptRegistry @go_TestPromptLifecycleAndEdges @go_TestPromptMetadataEdges
    Scenario: test_async_prompts_list_includes_categories
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When async prompts list includes categories
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-fdcd16ab87fc @go_TestConcurrentPromptRegistry @go_TestPromptLifecycleAndEdges @go_TestPromptMetadataEdges
    Scenario: test_async_prompt_with_async_return
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When async prompt with async return
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-0bb0d01d5072 @go_TestConcurrentPromptRegistry @go_TestPromptLifecycleAndEdges @go_TestPromptMetadataEdges
    Scenario: test_async_base_supports_sync_prompt_methods
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When async base supports sync prompt methods
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

  Rule: Behavior captured from test_resources.py

    @umcp-7fd3b8a764eb @go_TestConcurrentResourceRegistry @go_TestResourceEdges @go_TestResourceSyncAsyncParity
    Scenario: test_sync_resources_list_includes_static
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When sync resources list includes static
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-b56073bf6692 @go_TestConcurrentResourceRegistry @go_TestResourceEdges @go_TestResourceSyncAsyncParity
    Scenario: test_sync_templates_list_includes_template
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When sync templates list includes template
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-f545b6c5359f @go_TestConcurrentResourceRegistry @go_TestResourceEdges @go_TestResourceSyncAsyncParity
    Scenario: test_sync_read_text_resource
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When sync read text resource
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-e22566b4a914 @go_TestConcurrentResourceRegistry @go_TestResourceEdges @go_TestResourceSyncAsyncParity
    Scenario: test_sync_read_binary_resource_is_base64
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When sync read binary resource is base64
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-cd713efd951b @go_TestConcurrentResourceRegistry @go_TestResourceEdges @go_TestResourceSyncAsyncParity
    Scenario: test_sync_read_template_resource_binds_param
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When sync read template resource binds param
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-99547675c3db @go_TestConcurrentResourceRegistry @go_TestResourceEdges @go_TestResourceSyncAsyncParity
    Scenario: test_sync_read_unknown_uri_returns_minus_32002
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When sync read unknown uri returns minus 32002
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-8c276f3df296 @go_TestConcurrentResourceRegistry @go_TestResourceEdges @go_TestResourceSyncAsyncParity
    Scenario: test_sync_subscribe_unsubscribe_roundtrip
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When sync subscribe unsubscribe roundtrip
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-4d884f0d86d9 @go_TestConcurrentResourceRegistry @go_TestResourceEdges @go_TestResourceSyncAsyncParity
    Scenario: test_sync_dynamic_register_resource
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When sync dynamic register resource
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-7e2688d21dd1 @go_TestConcurrentResourceRegistry @go_TestResourceEdges @go_TestResourceSyncAsyncParity
    Scenario: test_sync_initialize_declares_resources_capability
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When sync initialize declares resources capability
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-f378d046baa3 @go_TestConcurrentResourceRegistry @go_TestResourceEdges @go_TestResourceSyncAsyncParity
    Scenario: test_async_resources_list
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When async resources list
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-5f1475655df8 @go_TestConcurrentResourceRegistry @go_TestResourceEdges @go_TestResourceSyncAsyncParity
    Scenario: test_async_read_async_text_resource
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When async read async text resource
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-08b93ea1c7ed @go_TestConcurrentResourceRegistry @go_TestResourceEdges @go_TestResourceSyncAsyncParity
    Scenario: test_async_read_sync_method_in_async_server
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When async read sync method in async server
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-e69e95994aff @go_TestConcurrentResourceRegistry @go_TestResourceEdges @go_TestResourceSyncAsyncParity
    Scenario: test_async_read_template_resource
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When async read template resource
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-1a586e5d548d @go_TestConcurrentResourceRegistry @go_TestResourceEdges @go_TestResourceSyncAsyncParity
    Scenario: test_async_unknown_uri_returns_minus_32002
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When async unknown uri returns minus 32002
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-0dcb44633e60 @go_TestConcurrentResourceRegistry @go_TestResourceEdges @go_TestResourceSyncAsyncParity
    Scenario: test_async_initialize_declares_resources_capability
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When async initialize declares resources capability
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

  Rule: Behavior captured from test_resources_extra.py

    @umcp-707ba85853c6 @go_TestConcurrentResourceRegistry @go_TestResourceEdges @go_TestResourceSyncAsyncParity
    Scenario: test_multi_param_template_listed_with_both_placeholders
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When multi param template listed with both placeholders
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-396c27b69cff @go_TestConcurrentResourceRegistry @go_TestResourceEdges @go_TestResourceSyncAsyncParity
    Scenario: test_multi_param_template_read_binds_both_groups
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When multi param template read binds both groups
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-fee3fe0db209 @go_TestConcurrentResourceRegistry @go_TestResourceEdges @go_TestResourceSyncAsyncParity
    Scenario: test_static_resource_raise_is_caught_as_minus_32603
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When static resource raise is caught as minus 32603
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-dd09bd78fa9d @go_TestConcurrentResourceRegistry @go_TestResourceEdges @go_TestResourceSyncAsyncParity
    Scenario: test_template_resource_raise_is_caught_as_minus_32603
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When template resource raise is caught as minus 32603
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-968952d03327 @go_TestConcurrentResourceRegistry @go_TestResourceEdges @go_TestResourceSyncAsyncParity
    Scenario: test_list_of_dicts_return_yields_multiple_content_entries
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When list of dicts return yields multiple content entries
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-761738ff1deb @go_TestConcurrentResourceRegistry @go_TestResourceEdges @go_TestResourceSyncAsyncParity
    Scenario: test_register_resource_template_then_read
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When register resource template then read
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-72ca34191bfd @go_TestConcurrentResourceRegistry @go_TestResourceEdges @go_TestResourceSyncAsyncParity
    Scenario: test_custom_uri_schemes_round_trip
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When custom uri schemes round trip
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-a330c44eadea @go_TestConcurrentResourceRegistry @go_TestResourceEdges @go_TestResourceSyncAsyncParity
    Scenario: test_resource_annotations_are_exposed_on_list
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When resource annotations are exposed on list
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-0ea7aaa5a97b @go_TestConcurrentResourceRegistry @go_TestResourceEdges @go_TestResourceSyncAsyncParity
    Scenario: test_override_attribute_replaces_default_uri_and_name
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When override attribute replaces default uri and name
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-90a028e8bd78 @go_TestConcurrentResourceRegistry @go_TestResourceEdges @go_TestResourceSyncAsyncParity
    Scenario: test_resources_list_paginates_when_requested
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When resources list paginates when requested
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-9d31157d9146 @go_TestConcurrentResourceRegistry @go_TestResourceEdges @go_TestResourceSyncAsyncParity
    Scenario: test_resources_templates_list_without_cursor_preserves_compatibility
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When resources templates list without cursor preserves compatibility
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-cde9a436c57d @go_TestConcurrentResourceRegistry @go_TestResourceEdges @go_TestResourceSyncAsyncParity
    Scenario: test_async_register_resource_then_read
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When async register resource then read
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-a2a9f747368d @go_TestConcurrentResourceRegistry @go_TestResourceEdges @go_TestResourceSyncAsyncParity
    Scenario: test_async_resource_that_raises_is_caught_as_minus_32603
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When async resource that raises is caught as minus 32603
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-2662a7fd1b88 @go_TestConcurrentResourceRegistry @go_TestResourceEdges @go_TestResourceSyncAsyncParity
    Scenario: test_subscribe_to_unknown_uri_is_still_accepted
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When subscribe to unknown uri is still accepted
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-d4c8bb66214c @go_TestConcurrentResourceRegistry @go_TestResourceEdges @go_TestResourceSyncAsyncParity
    Scenario: test_unsubscribe_unknown_uri_is_a_noop
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When unsubscribe unknown uri is a noop
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-b1213eb0676a @go_TestConcurrentResourceRegistry @go_TestResourceEdges @go_TestResourceSyncAsyncParity
    Scenario: test_subscribe_without_uri_returns_invalid_params
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When subscribe without uri returns invalid params
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error
