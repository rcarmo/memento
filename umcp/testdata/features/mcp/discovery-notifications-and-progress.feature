Feature: mcp/discovery notifications and progress

  The scenarios capture Python uMCP behavior at 30cce7dfe08c6ee63de235f7d81754ba286dafbb.
  Rules retain source-module traceability while features group related user behavior.

  Rule: Behavior captured from test_discovery_pagination.py

    @umcp-9652351e4dd1 @go_TestPaginationPythonParity @go_TestExplicitToolDiscoveryOrder @go_TestArgumentErrorText
    Scenario: test_sync_dynamic_discovery_registration_and_pagination
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When sync dynamic discovery registration and pagination
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-6d898f2a530e @go_TestPaginationPythonParity @go_TestExplicitToolDiscoveryOrder @go_TestArgumentErrorText
    Scenario: test_sync_invalid_cursor_and_default_no_cursor_compatibility
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When sync invalid cursor and default no cursor compatibility
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-c6053ea3387d @go_TestPaginationPythonParity @go_TestExplicitToolDiscoveryOrder @go_TestArgumentErrorText
    Scenario: test_sync_cursor_is_principal_safe
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When sync cursor is principal safe
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-3b167e4ca97c @go_TestPaginationPythonParity @go_TestExplicitToolDiscoveryOrder @go_TestArgumentErrorText
    Scenario: test_async_dynamic_discovery_registration_and_pagination
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When async dynamic discovery registration and pagination
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-52a6cac0624a @go_TestPaginationPythonParity @go_TestExplicitToolDiscoveryOrder @go_TestArgumentErrorText
    Scenario: test_list_changed_notifications_target_existing_transports
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When list changed notifications target existing transports
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

  Rule: Behavior captured from test_notifications.py

    @umcp-3e632c214980 @go_TestNotificationFallbackReference @go_TestServerNotificationOutput
    Scenario: test_sync_list_changed_always_emits
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When sync list changed always emits
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-7906537937b8 @go_TestNotificationFallbackReference @go_TestServerNotificationOutput
    Scenario: test_sync_updated_skipped_without_subscription
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When sync updated skipped without subscription
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-feb4fb28cdc3 @go_TestNotificationFallbackReference @go_TestServerNotificationOutput
    Scenario: test_sync_updated_emits_only_for_subscribed_uri
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When sync updated emits only for subscribed uri
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-5b429a0381e9 @go_TestNotificationFallbackReference @go_TestServerNotificationOutput
    Scenario: test_sync_unsubscribe_stops_notifications
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When sync unsubscribe stops notifications
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-cfab471aecff @go_TestNotificationFallbackReference @go_TestServerNotificationOutput
    Scenario: test_sync_subscribe_via_protocol_then_notify
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When sync subscribe via protocol then notify
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-efcc8ceaa07e @go_TestNotificationFallbackReference @go_TestServerNotificationOutput
    Scenario: test_sync_sse_updated_targets_only_subscribed_session
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When sync sse updated targets only subscribed session
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-96cfa7100f4c @go_TestNotificationFallbackReference @go_TestServerNotificationOutput
    Scenario: test_sync_sse_list_changed_broadcasts_to_all_sessions
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When sync sse list changed broadcasts to all sessions
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-97efaf1cb633 @go_TestNotificationFallbackReference @go_TestServerNotificationOutput
    Scenario: test_async_list_changed_always_emits
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When async list changed always emits
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-3e657e0a4e5e @go_TestNotificationFallbackReference @go_TestServerNotificationOutput
    Scenario: test_async_updated_gated_by_subscription
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When async updated gated by subscription
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-deac04a69846 @go_TestNotificationFallbackReference @go_TestServerNotificationOutput
    Scenario: test_async_sse_updated_targets_only_subscribed_session
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When async sse updated targets only subscribed session
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-8f2b816160e6 @go_TestNotificationFallbackReference @go_TestServerNotificationOutput
    Scenario: test_async_sse_list_changed_broadcasts_to_all_sessions
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When async sse list changed broadcasts to all sessions
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

  Rule: Behavior captured from test_progress_cancellation.py

    @umcp-d71811f1f416 @go_TestSyncAsyncProgressParity @go_TestDispatcherContextIsolation @go_TestDispatcherFailuresAndCancellation
    Scenario: test_sync_progress_exact_payload_and_absent_token_noop
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When sync progress exact payload and absent token noop
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-f529c0092aa7 @go_TestSyncAsyncProgressParity @go_TestDispatcherContextIsolation @go_TestDispatcherFailuresAndCancellation
    Scenario: test_async_progress_exact_payload_for_integer_token
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When async progress exact payload for integer token
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-7e523580cef8 @go_TestSyncAsyncProgressParity @go_TestDispatcherContextIsolation @go_TestDispatcherFailuresAndCancellation
    Scenario: test_invalid_progress_token_and_invalid_progress_are_rejected
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When invalid progress token and invalid progress are rejected
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-21b26bec488b @go_TestDispatcherContextIsolation @go_TestDispatcherFailuresAndCancellation @go_TestSyncAsyncDispatcherParity
    Scenario: test_sync_cooperative_cancellation_and_cleanup
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When sync cooperative cancellation and cleanup
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-92233c31da1e @go_TestDispatcherContextIsolation @go_TestDispatcherFailuresAndCancellation @go_TestSyncAsyncDispatcherParity
    Scenario: test_sync_cancellation_isolated_between_concurrent_requests
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When sync cancellation isolated between concurrent requests
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-6af42ebab4ee @go_TestDispatcherContextIsolation @go_TestDispatcherFailuresAndCancellation @go_TestSyncAsyncDispatcherParity
    Scenario: test_async_active_cancellation_and_cleanup
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When async active cancellation and cleanup
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-cdcdaa3e924e @go_TestDispatcherContextIsolation @go_TestDispatcherFailuresAndCancellation @go_TestSyncAsyncDispatcherParity
    Scenario: test_async_cancellation_isolated_between_concurrent_requests
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When async cancellation isolated between concurrent requests
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-c850acb753f4 @go_TestSyncAsyncProgressParity @go_TestDispatcherContextIsolation @go_TestDispatcherFailuresAndCancellation
    Scenario: test_sync_progress_uses_sse_transport_when_present
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When sync progress uses sse transport when present
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-c61cc4016ca8 @go_TestSyncAsyncProgressParity @go_TestDispatcherContextIsolation @go_TestDispatcherFailuresAndCancellation
    Scenario: test_async_progress_uses_sse_transport_when_present
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When async progress uses sse transport when present
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

  Rule: Behavior captured from test_tool_outputs_and_runtime_notifications.py

    @umcp-871d3f58ee43 @go_TestOrderedJSONDuplicatesAndErrors @go_TestPythonToolResultParity @go_TestStructuredValueEdges
    Scenario: test_sync_register_unregister_and_notify_stdio
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When sync register unregister and notify stdio
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-347478781a5b @go_TestOrderedJSONDuplicatesAndErrors @go_TestPythonToolResultParity @go_TestStructuredValueEdges
    Scenario: test_sync_register_unregister_and_notify_sse
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When sync register unregister and notify sse
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-db1301a590a1 @go_TestOrderedJSONDuplicatesAndErrors @go_TestPythonToolResultParity @go_TestStructuredValueEdges
    Scenario: test_async_register_unregister_and_notify_stdio
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When async register unregister and notify stdio
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-2dec70e66b20 @go_TestOrderedJSONDuplicatesAndErrors @go_TestPythonToolResultParity @go_TestStructuredValueEdges
    Scenario: test_async_register_unregister_and_notify_sse
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When async register unregister and notify sse
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-bf2124757038 @go_TestOrderedJSONDuplicatesAndErrors @go_TestPythonToolResultParity @go_TestStructuredValueEdges
    Scenario: test_tools_list_advertises_inferred_and_explicit_output_schemas
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When tools list advertises inferred and explicit output schemas
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-48e8eb3b4ad8 @go_TestOrderedJSONDuplicatesAndErrors @go_TestPythonToolResultParity @go_TestStructuredValueEdges
    Scenario: test_sync_mapping_return_adds_structured_content
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When sync mapping return adds structured content
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-ed1fea538be7 @go_TestOrderedJSONDuplicatesAndErrors @go_TestPythonToolResultParity @go_TestStructuredValueEdges
    Scenario: test_sync_typed_return_adds_structured_content
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When sync typed return adds structured content
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-faae02c4fda0 @go_TestOrderedJSONDuplicatesAndErrors @go_TestPythonToolResultParity @go_TestStructuredValueEdges
    Scenario: test_async_mapping_return_adds_structured_content
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When async mapping return adds structured content
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-45da0c68685a @go_TestOrderedJSONDuplicatesAndErrors @go_TestPythonToolResultParity @go_TestStructuredValueEdges
    Scenario: test_async_typed_return_adds_structured_content
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When async typed return adds structured content
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-af1d7f1db12e @go_TestOrderedJSONDuplicatesAndErrors @go_TestPythonToolResultParity @go_TestStructuredValueEdges
    Scenario: test_sync_static_malformed_output_is_local_detailed
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When sync static malformed output is local detailed
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-da87e528ef40 @go_TestOrderedJSONDuplicatesAndErrors @go_TestPythonToolResultParity @go_TestStructuredValueEdges
    Scenario: test_sync_dynamic_malformed_output_is_remote_safe
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When sync dynamic malformed output is remote safe
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-aa42cdcf60d1 @go_TestOrderedJSONDuplicatesAndErrors @go_TestPythonToolResultParity @go_TestStructuredValueEdges
    Scenario: test_async_static_malformed_output_is_local_detailed
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When async static malformed output is local detailed
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-55edd23a3ac5 @go_TestOrderedJSONDuplicatesAndErrors @go_TestPythonToolResultParity @go_TestStructuredValueEdges
    Scenario: test_async_dynamic_malformed_output_is_remote_safe
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When async dynamic malformed output is remote safe
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error
