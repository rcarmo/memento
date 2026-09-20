Feature: mcp/protocol errors and context

  The scenarios capture Python uMCP behavior at 30cce7dfe08c6ee63de235f7d81754ba286dafbb.
  Rules retain source-module traceability while features group related user behavior.

  Rule: Behavior captured from test_protocol_errors.py

    @umcp-dc6daba2cf4e @go_TestDispatcherContextIsolation @go_TestDispatcherFailuresAndCancellation @go_TestSyncAsyncDispatcherParity
    Scenario: test_sync_invalid_json_returns_parse_error
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When sync invalid json returns parse error
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-8456f692a67f @go_TestDispatcherContextIsolation @go_TestDispatcherFailuresAndCancellation @go_TestSyncAsyncDispatcherParity
    Scenario: test_sync_non_object_top_level_json_is_invalid_request
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When sync non object top level json is invalid request
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-5ff0b12e40d7 @go_TestDispatcherContextIsolation @go_TestDispatcherFailuresAndCancellation @go_TestSyncAsyncDispatcherParity
    Scenario: test_sync_wrong_jsonrpc_version_rejected
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When sync wrong jsonrpc version rejected
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-da145c530a0d @go_TestDispatcherContextIsolation @go_TestDispatcherFailuresAndCancellation @go_TestSyncAsyncDispatcherParity
    Scenario: test_sync_invalid_ids_and_malformed_responses_are_rejected
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When sync invalid ids and malformed responses are rejected
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-56f18ebea82c @go_TestDispatcherContextIsolation @go_TestDispatcherFailuresAndCancellation @go_TestSyncAsyncDispatcherParity
    Scenario: test_sync_missing_or_non_string_method_is_invalid_request
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When sync missing or non string method is invalid request
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-4301bc7f2c70 @go_TestDispatcherContextIsolation @go_TestDispatcherFailuresAndCancellation @go_TestSyncAsyncDispatcherParity
    Scenario: test_sync_client_response_returns_none
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When sync client response returns none
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-67c6ac8366ed @go_TestDispatcherContextIsolation @go_TestDispatcherFailuresAndCancellation @go_TestSyncAsyncDispatcherParity
    Scenario: test_sync_unknown_method_returns_minus_32601
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When sync unknown method returns minus 32601
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-4180b1d97b21 @go_TestDispatcherContextIsolation @go_TestDispatcherFailuresAndCancellation @go_TestSyncAsyncDispatcherParity
    Scenario: test_sync_non_object_params_returns_invalid_params
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When sync non object params returns invalid params
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-3a946d70f06e @go_TestDispatcherContextIsolation @go_TestDispatcherFailuresAndCancellation @go_TestSyncAsyncDispatcherParity
    Scenario: test_sync_tools_call_missing_name
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When sync tools call missing name
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-7f36e90e5429 @go_TestDispatcherContextIsolation @go_TestDispatcherFailuresAndCancellation @go_TestSyncAsyncDispatcherParity
    Scenario: test_sync_tools_call_unknown_tool
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When sync tools call unknown tool
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-abb8166665a1 @go_TestDispatcherContextIsolation @go_TestDispatcherFailuresAndCancellation @go_TestSyncAsyncDispatcherParity
    Scenario: test_sync_tools_call_with_unknown_argument_is_rejected
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When sync tools call with unknown argument is rejected
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-854be0d8a9b2 @go_TestDispatcherContextIsolation @go_TestDispatcherFailuresAndCancellation @go_TestSyncAsyncDispatcherParity
    Scenario: test_sync_tool_that_raises_does_not_crash_server
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When sync tool that raises does not crash server
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-803e77a40ede @go_TestDispatcherContextIsolation @go_TestDispatcherFailuresAndCancellation @go_TestSyncAsyncDispatcherParity
    Scenario: test_sync_prompts_get_unknown_returns_error
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When sync prompts get unknown returns error
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-7358f042ceac @go_TestDispatcherContextIsolation @go_TestDispatcherFailuresAndCancellation @go_TestSyncAsyncDispatcherParity
    Scenario: test_sync_notifications_initialized_returns_none
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When sync notifications initialized returns none
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-45265eb98d4d @go_TestDispatcherContextIsolation @go_TestDispatcherFailuresAndCancellation @go_TestSyncAsyncDispatcherParity
    Scenario: test_async_invalid_json_returns_parse_error
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When async invalid json returns parse error
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-7770742a87fd @go_TestDispatcherContextIsolation @go_TestDispatcherFailuresAndCancellation @go_TestSyncAsyncDispatcherParity
    Scenario: test_async_non_object_top_level_json_is_invalid_request
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When async non object top level json is invalid request
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-96014de5bb34 @go_TestDispatcherContextIsolation @go_TestDispatcherFailuresAndCancellation @go_TestSyncAsyncDispatcherParity
    Scenario: test_async_invalid_ids_and_malformed_responses_are_rejected
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When async invalid ids and malformed responses are rejected
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-13f5555d9c54 @go_TestDispatcherContextIsolation @go_TestDispatcherFailuresAndCancellation @go_TestSyncAsyncDispatcherParity
    Scenario: test_async_missing_or_non_string_method_is_invalid_request
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When async missing or non string method is invalid request
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-7ef12e827d2a @go_TestDispatcherContextIsolation @go_TestDispatcherFailuresAndCancellation @go_TestSyncAsyncDispatcherParity
    Scenario: test_async_client_response_returns_none
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When async client response returns none
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-100762c25600 @go_TestDispatcherContextIsolation @go_TestDispatcherFailuresAndCancellation @go_TestSyncAsyncDispatcherParity
    Scenario: test_async_unknown_method_returns_minus_32601
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When async unknown method returns minus 32601
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-a94a9ff2e05a @go_TestDispatcherContextIsolation @go_TestDispatcherFailuresAndCancellation @go_TestSyncAsyncDispatcherParity
    Scenario: test_async_non_object_params_returns_invalid_params
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When async non object params returns invalid params
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-5809fa868dba @go_TestDispatcherContextIsolation @go_TestDispatcherFailuresAndCancellation @go_TestSyncAsyncDispatcherParity
    Scenario: test_async_tools_call_unknown_tool
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When async tools call unknown tool
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-dc27bd9c5bd2 @go_TestDispatcherContextIsolation @go_TestDispatcherFailuresAndCancellation @go_TestSyncAsyncDispatcherParity
    Scenario: test_async_tool_that_raises_does_not_crash_server
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When async tool that raises does not crash server
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-90dfe4758253 @go_TestDispatcherContextIsolation @go_TestDispatcherFailuresAndCancellation @go_TestSyncAsyncDispatcherParity
    Scenario: test_async_notifications_initialized_returns_none
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When async notifications initialized returns none
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

  Rule: Behavior captured from test_shared_negotiation_context.py

    @umcp-d9e0176d7ae9 @go_TestDispatcherContextIsolation @go_TestDispatcherFailuresAndCancellation @go_TestSyncAsyncDispatcherParity
    Scenario: test_exact_or_fallback_prefers_supported_version
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When exact or fallback prefers supported version
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-36fc32ec4acb @go_TestDispatcherContextIsolation @go_TestDispatcherFailuresAndCancellation @go_TestSyncAsyncDispatcherParity
    Scenario: test_request_context_roundtrip
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When request context roundtrip
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-c76afbbb5c0f @go_TestDispatcherContextIsolation @go_TestDispatcherFailuresAndCancellation @go_TestSyncAsyncDispatcherParity
    Scenario: test_sync_initialize_negotiates_supported_and_falls_back
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When sync initialize negotiates supported and falls back
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-5fac35c71574 @go_TestDispatcherContextIsolation @go_TestDispatcherFailuresAndCancellation @go_TestSyncAsyncDispatcherParity
    Scenario: test_async_initialize_negotiation_matches_sync
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When async initialize negotiation matches sync
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-b0e2d9e99a61 @go_TestDispatcherContextIsolation @go_TestDispatcherFailuresAndCancellation @go_TestSyncAsyncDispatcherParity
    Scenario: test_sync_initialize_capabilities_are_exact
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When sync initialize capabilities are exact
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-928e5a585175 @go_TestDispatcherContextIsolation @go_TestDispatcherFailuresAndCancellation @go_TestSyncAsyncDispatcherParity
    Scenario: test_async_initialize_capabilities_match_sync_exactly
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When async initialize capabilities match sync exactly
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-62785dca6999 @go_TestDispatcherContextIsolation @go_TestDispatcherFailuresAndCancellation @go_TestSyncAsyncDispatcherParity
    Scenario: test_request_context_headers_are_defensively_immutable
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When request context headers are defensively immutable
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-615dac5bf8fd @go_TestDispatcherContextIsolation @go_TestDispatcherFailuresAndCancellation @go_TestSyncAsyncDispatcherParity
    Scenario: test_principal_metadata_is_defensively_immutable
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When principal metadata is defensively immutable
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error
