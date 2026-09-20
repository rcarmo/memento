Feature: mcp/transports and integrated servers

  The scenarios capture Python uMCP behavior at 30cce7dfe08c6ee63de235f7d81754ba286dafbb.
  Rules retain source-module traceability while features group related user behavior.

  Rule: Behavior captured from test_async_servers.py

    @umcp-b13443a5d78e @go_TestAsyncHTTPConnectionParserAndCancellation @go_TestAsyncHTTPConnectionPipeline @go_TestHTTPRawListenerAndSSE
    Scenario: test_introspection
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When introspection
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-947957ba3a3b @go_TestAsyncHTTPConnectionParserAndCancellation @go_TestAsyncHTTPConnectionPipeline @go_TestHTTPRawListenerAndSSE
    Scenario: test_async_performance
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When async performance
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

  Rule: Behavior captured from test_movieserver.py

    @umcp-1014a9d135a6 @go_TestConcurrentPromptRegistry @go_TestPromptLifecycleAndEdges @go_TestPromptMetadataEdges
    Scenario: test_basic_functionality
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When basic functionality
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

  Rule: Behavior captured from test_transports.py

    @umcp-4ea8e852b72a @go_TestPythonStdioParity @go_TestStdioErrorPaths @go_TestStdioFramingAndNotifications
    Scenario: test_sync_stdio_streaming_round_trip
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When sync stdio streaming round trip
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-d44ac184a19a @go_TestPythonStdioParity @go_TestStdioErrorPaths @go_TestStdioFramingAndNotifications
    Scenario: test_sync_stdio_resources_round_trip
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When sync stdio resources round trip
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-7a8871616f0b @go_TestPythonTransportArgs @go_TestRunTransportClearsPreviousSink @go_TestRunTransportFileAndStdio
    Scenario: test_sync_tcp_transport_round_trip[calculator_server.py]
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When sync tcp transport round trip[calculator server.py]
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-3672760045e8 @go_TestAsyncSSETransferEncodingRouteOrder @go_TestLegacySSEFlushFailures @go_TestLegacySSENotificationsAndDelivery
    Scenario: test_sync_streamable_http_round_trip_and_error_paths
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When sync streamable http round trip and error paths
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-6b5670e5a8e3 @go_TestAsyncSSETransferEncodingRouteOrder @go_TestLegacySSEFlushFailures @go_TestLegacySSENotificationsAndDelivery
    Scenario: test_sync_streamable_http_auxiliary_routes_and_mcp_auth_boundary
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When sync streamable http auxiliary routes and mcp auth boundary
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-e38e82651c1f @go_TestAsyncSSETransferEncodingRouteOrder @go_TestLegacySSEFlushFailures @go_TestLegacySSENotificationsAndDelivery
    Scenario: test_sync_streamable_http_auth_401_and_403
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When sync streamable http auth 401 and 403
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-3518b90916a3 @go_TestAsyncSSETransferEncodingRouteOrder @go_TestLegacySSEFlushFailures @go_TestLegacySSENotificationsAndDelivery
    Scenario: test_sync_streamable_http_legacy_auth_alias_cors_errors_and_length_guards
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When sync streamable http legacy auth alias cors errors and length guards
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-f8a0c4a20b43 @go_TestAsyncSSETransferEncodingRouteOrder @go_TestLegacySSEFlushFailures @go_TestLegacySSENotificationsAndDelivery
    Scenario: test_sync_streamable_http_query_host_duplicate_headers_and_options_path
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When sync streamable http query host duplicate headers and options path
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-e5ee4bb2149b @go_TestAsyncSSETransferEncodingRouteOrder @go_TestLegacySSEFlushFailures @go_TestLegacySSENotificationsAndDelivery
    Scenario: test_sync_streamable_http_rejects_async_auth_hooks_cleanly
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When sync streamable http rejects async auth hooks cleanly
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-3c4a9bdb305b @go_TestAsyncSSETransferEncodingRouteOrder @go_TestLegacySSEFlushFailures @go_TestLegacySSENotificationsAndDelivery
    Scenario: test_sync_sse_security_origin_and_body_guards
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When sync sse security origin and body guards
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-b95032096897 @go_TestAsyncSSETransferEncodingRouteOrder @go_TestLegacySSEFlushFailures @go_TestLegacySSENotificationsAndDelivery
    Scenario: test_sync_sse_transport_handshake_and_request
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When sync sse transport handshake and request
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-310b17c3ad15 @go_TestAsyncSSETransferEncodingRouteOrder @go_TestLegacySSEFlushFailures @go_TestLegacySSENotificationsAndDelivery
    Scenario: test_sync_sse_session_cleanup_race_returns_404
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When sync sse session cleanup race returns 404
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-1ee3757945b5 @go_TestAsyncSSETransferEncodingRouteOrder @go_TestLegacySSEFlushFailures @go_TestLegacySSENotificationsAndDelivery
    Scenario: test_help_flag_does_not_crash
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When help flag does not crash
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error
