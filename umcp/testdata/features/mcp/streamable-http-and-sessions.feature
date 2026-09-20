Feature: mcp/streamable http and sessions

  The scenarios capture Python uMCP behavior at 30cce7dfe08c6ee63de235f7d81754ba286dafbb.
  Rules retain source-module traceability while features group related user behavior.

  Rule: Behavior captured from test_streamable_http_regressions.py

    @umcp-b19b2ba5d202 @go_TestAsyncHTTPConnectionParserAndCancellation @go_TestAsyncHTTPConnectionPipeline @go_TestHTTPRawListenerAndSSE
    Scenario: test_sync_non_dict_json_rpcs_return_invalid_request
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When sync non dict json rpcs return invalid request
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-73770948dc48 @go_TestAsyncHTTPConnectionParserAndCancellation @go_TestAsyncHTTPConnectionPipeline @go_TestHTTPRawListenerAndSSE
    Scenario: test_async_non_dict_json_rpcs_return_invalid_request
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When async non dict json rpcs return invalid request
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-6fa69aa4b7db @go_TestAsyncHTTPConnectionParserAndCancellation @go_TestAsyncHTTPConnectionPipeline @go_TestHTTPRawListenerAndSSE
    Scenario: test_context_isolated_across_threads_and_async_tasks
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When context isolated across threads and async tasks
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-f8f60ecfc44e @go_TestAsyncHTTPConnectionParserAndCancellation @go_TestAsyncHTTPConnectionPipeline @go_TestHTTPRawListenerAndSSE
    Scenario: test_stdio_and_file_modes_expose_transport_context
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When stdio and file modes expose transport context
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-354cba428ef7 @go_TestAsyncHTTPConnectionParserAndCancellation @go_TestAsyncHTTPConnectionPipeline @go_TestHTTPRawListenerAndSSE
    Scenario: test_async_context_propagates_into_sync_tool
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When async context propagates into sync tool
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-643814a3336e @go_TestAsyncHTTPConnectionParserAndCancellation @go_TestAsyncHTTPConnectionPipeline @go_TestHTTPRawListenerAndSSE
    Scenario: test_async_http_202_for_notification_and_response_object
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When async http 202 for notification and response object
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-cf94f8a3b054 @go_TestAsyncHTTPConnectionParserAndCancellation @go_TestAsyncHTTPConnectionPipeline @go_TestHTTPRawListenerAndSSE
    Scenario: test_async_http_tool_sees_authenticated_context
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When async http tool sees authenticated context
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-225ab47c2df9 @go_TestAsyncHTTPConnectionParserAndCancellation @go_TestAsyncHTTPConnectionPipeline @go_TestHTTPRawListenerAndSSE
    Scenario: test_origin_validation_is_exact_and_remote_safe
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When origin validation is exact and remote safe
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-a87a4c12bfb8 @go_TestAsyncHTTPConnectionParserAndCancellation @go_TestAsyncHTTPConnectionPipeline @go_TestHTTPRawListenerAndSSE
    Scenario: test_accept_negotiation_matches_actual_response_type
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When accept negotiation matches actual response type
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-5187d68210ec @go_TestAsyncHTTPConnectionParserAndCancellation @go_TestAsyncHTTPConnectionPipeline @go_TestHTTPRawListenerAndSSE
    Scenario: test_cli_rejects_unknown_conflicting_and_incomplete_transports
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When cli rejects unknown conflicting and incomplete transports
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-312e9b4f43c0 @go_TestAsyncHTTPConnectionParserAndCancellation @go_TestAsyncHTTPConnectionPipeline @go_TestHTTPRawListenerAndSSE
    Scenario: test_async_cors_headers_are_returned_on_preflight_and_post
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When async cors headers are returned on preflight and post
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-ecd6f451c9ed @go_TestAsyncHTTPConnectionParserAndCancellation @go_TestAsyncHTTPConnectionPipeline @go_TestHTTPRawListenerAndSSE
    Scenario: test_async_auxiliary_http_routes_are_bounded_and_mcp_is_unchanged
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When async auxiliary http routes are bounded and mcp is unchanged
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-3412055144c3 @go_TestAsyncHTTPConnectionParserAndCancellation @go_TestAsyncHTTPConnectionPipeline @go_TestHTTPRawListenerAndSSE
    Scenario: test_async_http_rejects_bad_content_length_and_oversize
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When async http rejects bad content length and oversize
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-a79845283665 @go_TestAsyncHTTPConnectionParserAndCancellation @go_TestAsyncHTTPConnectionPipeline @go_TestHTTPRawListenerAndSSE
    Scenario: test_sync_auth_hook_aliases_and_new_hooks_are_bidirectional
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When sync auth hook aliases and new hooks are bidirectional
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-1e54ba12d421 @go_TestAsyncHTTPConnectionParserAndCancellation @go_TestAsyncHTTPConnectionPipeline @go_TestHTTPRawListenerAndSSE
    Scenario: test_async_auth_hook_aliases_and_new_hooks_are_bidirectional
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When async auth hook aliases and new hooks are bidirectional
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-c5f5bf16c9e0 @go_TestAsyncHTTPConnectionParserAndCancellation @go_TestAsyncHTTPConnectionPipeline @go_TestHTTPRawListenerAndSSE
    Scenario: test_async_http_rejects_missing_version_bad_accept_and_unauthorized
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When async http rejects missing version bad accept and unauthorized
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-9759303d185d @go_TestAsyncHTTPConnectionParserAndCancellation @go_TestAsyncHTTPConnectionPipeline @go_TestHTTPRawListenerAndSSE
    Scenario: test_async_http_preflight_requires_valid_origin_and_authorization_can_forbid
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When async http preflight requires valid origin and authorization can forbid
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-1fad2ac487e0 @go_TestAsyncHTTPConnectionParserAndCancellation @go_TestAsyncHTTPConnectionPipeline @go_TestHTTPRawListenerAndSSE
    Scenario: test_async_http_allowed_origin_gets_cors_on_terminal_responses[GET /mcp HTTP/1.1\nOrigin: http://allowed\nContent-Length: 0--401 Unauthorized]
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When async http allowed origin gets cors on terminal responses[GET /mcp HTTP/1.1\nOrigin: http://allowed\nContent Length: 0  401 Unauthorized]
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-73527eb67563 @go_TestAsyncHTTPConnectionParserAndCancellation @go_TestAsyncHTTPConnectionPipeline @go_TestHTTPRawListenerAndSSE
    Scenario: test_async_http_allowed_origin_gets_cors_on_terminal_responses[POST /mcp HTTP/1.1\nOrigin: http://allowed\nContent-Type: text/plain\nAccept: application/json\nAuthorization: Bearer ok\nContent-Length: 0--415 Unsupported Media Type]
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When async http allowed origin gets cors on terminal responses[POST /mcp HTTP/1.1\nOrigin: http://allowed\nContent Type: text/plain\nAccept: application/json\nAuthorization: Bearer ok\nContent Length: 0  415 Unsupported Media Type]
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-bd485b8eb58d @go_TestAsyncHTTPConnectionParserAndCancellation @go_TestAsyncHTTPConnectionPipeline @go_TestHTTPRawListenerAndSSE
    Scenario: test_async_http_allowed_origin_gets_cors_on_terminal_responses[POST /mcp HTTP/1.1\nOrigin: http://allowed\nContent-Type: application/json\nAccept: text/plain\nAuthorization: Bearer ok\nContent-Length: 0--406 Not Acceptable]
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When async http allowed origin gets cors on terminal responses[POST /mcp HTTP/1.1\nOrigin: http://allowed\nContent Type: application/json\nAccept: text/plain\nAuthorization: Bearer ok\nContent Length: 0  406 Not Acceptable]
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-94994abbfb6b @go_TestAsyncHTTPConnectionParserAndCancellation @go_TestAsyncHTTPConnectionPipeline @go_TestHTTPRawListenerAndSSE
    Scenario: test_async_http_allowed_origin_gets_cors_on_terminal_responses[POST /mcp HTTP/1.1\nOrigin: http://allowed\nContent-Type: application/json\nAccept: application/json\nMCP-Protocol-Version: 2025-03-26\nContent-Length: 0--401 Unauthorized]
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When async http allowed origin gets cors on terminal responses[POST /mcp HTTP/1.1\nOrigin: http://allowed\nContent Type: application/json\nAccept: application/json\nMCP Protocol Version: 2025 03 26\nContent Length: 0  401 Unauthorized]
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-6cdc615c2bbd @go_TestAsyncHTTPConnectionParserAndCancellation @go_TestAsyncHTTPConnectionPipeline @go_TestHTTPRawListenerAndSSE
    Scenario: test_async_http_allowed_origin_gets_cors_on_terminal_responses[POST /mcp HTTP/1.1\nOrigin: http://allowed\nContent-Type: application/json\nAccept: application/json\nAuthorization: Bearer ok\nMCP-Protocol-Version: 2025-03-26\nContent-Length: 0--200 OK]
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When async http allowed origin gets cors on terminal responses[POST /mcp HTTP/1.1\nOrigin: http://allowed\nContent Type: application/json\nAccept: application/json\nAuthorization: Bearer ok\nMCP Protocol Version: 2025 03 26\nContent Length: 0  200 OK]
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-9a9465aaa3ec @go_TestAsyncHTTPConnectionParserAndCancellation @go_TestAsyncHTTPConnectionPipeline @go_TestHTTPRawListenerAndSSE
    Scenario: test_async_http_allowed_origin_gets_cors_on_terminal_responses[POST /mcp HTTP/1.1\nOrigin: http://allowed\nContent-Type: application/json\nAccept: application/json\nAuthorization: Bearer ok\nContent-Length: nope--400 Bad Request]
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When async http allowed origin gets cors on terminal responses[POST /mcp HTTP/1.1\nOrigin: http://allowed\nContent Type: application/json\nAccept: application/json\nAuthorization: Bearer ok\nContent Length: nope  400 Bad Request]
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-168e7079151e @go_TestAsyncHTTPConnectionParserAndCancellation @go_TestAsyncHTTPConnectionPipeline @go_TestHTTPRawListenerAndSSE
    Scenario: test_async_http_allowed_origin_gets_cors_on_terminal_responses[POST /mcp HTTP/1.1\nOrigin: http://allowed\nContent-Type: application/json\nAccept: application/json\nAuthorization: Bearer ok\nContent-Length: 9--413 Payload Too Large]
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When async http allowed origin gets cors on terminal responses[POST /mcp HTTP/1.1\nOrigin: http://allowed\nContent Type: application/json\nAccept: application/json\nAuthorization: Bearer ok\nContent Length: 9  413 Payload Too Large]
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-178eec2548ef @go_TestAsyncHTTPConnectionParserAndCancellation @go_TestAsyncHTTPConnectionPipeline @go_TestHTTPRawListenerAndSSE
    Scenario: test_async_http_allowed_origin_gets_cors_on_json_errors_and_duplicate_length_and_transfer_encoding
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When async http allowed origin gets cors on json errors and duplicate length and transfer encoding
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-259293ad90bf @go_TestAsyncHTTPConnectionParserAndCancellation @go_TestAsyncHTTPConnectionPipeline @go_TestHTTPRawListenerAndSSE
    Scenario: test_origin_validation_rejects_malformed_loopback_forms
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When origin validation rejects malformed loopback forms
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-c980620186c3 @go_TestAsyncHTTPConnectionParserAndCancellation @go_TestAsyncHTTPConnectionPipeline @go_TestHTTPRawListenerAndSSE
    Scenario: test_remote_safe_prompt_errors
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When remote safe prompt errors
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-54f33871e7f0 @go_TestAsyncHTTPConnectionParserAndCancellation @go_TestAsyncHTTPConnectionPipeline @go_TestHTTPRawListenerAndSSE
    Scenario: test_remote_transports_hide_internal_errors
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When remote transports hide internal errors
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-6f5a6036dd95 @go_TestAsyncHTTPConnectionParserAndCancellation @go_TestAsyncHTTPConnectionPipeline @go_TestHTTPRawListenerAndSSE
    Scenario: test_async_http_rejects_invalid_utf8_version_and_ambiguous_host
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When async http rejects invalid utf8 version and ambiguous host
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-8c309ca383b3 @go_TestAsyncHTTPConnectionParserAndCancellation @go_TestAsyncHTTPConnectionPipeline @go_TestHTTPRawListenerAndSSE
    Scenario: test_async_streamable_http_rejects_invalid_host_and_duplicate_singleton_headers
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When async streamable http rejects invalid host and duplicate singleton headers
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-b40142829680 @go_TestAsyncHTTPConnectionParserAndCancellation @go_TestAsyncHTTPConnectionPipeline @go_TestHTTPRawListenerAndSSE
    Scenario: test_async_streamable_http_http10_allows_missing_host_and_query_path
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When async streamable http http10 allows missing host and query path
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-521dcdedbdce @go_TestAsyncHTTPConnectionParserAndCancellation @go_TestAsyncHTTPConnectionPipeline @go_TestHTTPRawListenerAndSSE
    Scenario: test_async_streamable_http_options_only_on_endpoint_and_hook_failures_are_500
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When async streamable http options only on endpoint and hook failures are 500
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-580d7856c919 @go_TestPythonHTTPRules @go_TestPythonQuality @go_TestAsyncHTTPConnectionParserAndCancellation
    Scenario: test_async_sse_origin_auth_media_and_body_rules
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When async sse origin auth media and body rules
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-8965446b01f6 @go_TestSessionGenerationFailures @go_TestSessionLifecycle @go_TestAsyncHTTPConnectionParserAndCancellation
    Scenario: test_async_sse_binds_sessions_to_authenticated_principal
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When async sse binds sessions to authenticated principal
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-8f81489be868 @go_TestAsyncHTTPConnectionParserAndCancellation @go_TestAsyncHTTPConnectionPipeline @go_TestHTTPRawListenerAndSSE
    Scenario: test_async_sse_rejects_invalid_utf8_body
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When async sse rejects invalid utf8 body
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-b6480e1d464a @go_TestAsyncHTTPConnectionParserAndCancellation @go_TestAsyncHTTPConnectionPipeline @go_TestHTTPRawListenerAndSSE
    Scenario: test_async_sse_session_cleanup_race_returns_404
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When async sse session cleanup race returns 404
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-94741595cd46 @go_TestAsyncHTTPConnectionParserAndCancellation @go_TestAsyncHTTPConnectionPipeline @go_TestHTTPRawListenerAndSSE
    Scenario: test_async_sse_hook_failures_and_duplicate_host_are_500_or_400
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When async sse hook failures and duplicate host are 500 or 400
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-d750a7160468 @go_TestAsyncHTTPConnectionParserAndCancellation @go_TestAsyncHTTPConnectionPipeline @go_TestHTTPRawListenerAndSSE
    Scenario: test_authentication_and_authorization_paths
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When authentication and authorization paths
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

  Rule: Behavior captured from test_streamable_http_sessions.py

    @umcp-0348d33ff9d2 @go_TestAsyncHTTPConnectionParserAndCancellation @go_TestAsyncHTTPConnectionPipeline @go_TestHTTPRawListenerAndSSE
    Scenario: test_http_11_reuses_connection_and_explicitly_closes[sync]
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When http 11 reuses connection and explicitly closes[sync]
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-f8a9df7dc32a @go_TestAsyncHTTPConnectionParserAndCancellation @go_TestAsyncHTTPConnectionPipeline @go_TestHTTPRawListenerAndSSE
    Scenario: test_http_11_reuses_connection_and_explicitly_closes[async]
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When http 11 reuses connection and explicitly closes[async]
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-3175fe76c8cc @go_TestAsyncHTTPConnectionParserAndCancellation @go_TestAsyncHTTPConnectionPipeline @go_TestHTTPRawListenerAndSSE
    Scenario: test_http_10_close_and_keep_alive_are_explicit[sync]
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When http 10 close and keep alive are explicit[sync]
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-5dd798240c6d @go_TestAsyncHTTPConnectionParserAndCancellation @go_TestAsyncHTTPConnectionPipeline @go_TestHTTPRawListenerAndSSE
    Scenario: test_http_10_close_and_keep_alive_are_explicit[async]
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When http 10 close and keep alive are explicit[async]
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-b88301ad99a5 @go_TestAsyncHTTPConnectionParserAndCancellation @go_TestAsyncHTTPConnectionPipeline @go_TestHTTPRawListenerAndSSE
    Scenario: test_session_binding_notifications_reconnect_and_delete[sync]
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When session binding notifications reconnect and delete[sync]
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-1782646e1950 @go_TestAsyncHTTPConnectionParserAndCancellation @go_TestAsyncHTTPConnectionPipeline @go_TestHTTPRawListenerAndSSE
    Scenario: test_session_binding_notifications_reconnect_and_delete[async]
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When session binding notifications reconnect and delete[async]
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-794d1107467e @go_TestAsyncHTTPConnectionParserAndCancellation @go_TestAsyncHTTPConnectionPipeline @go_TestHTTPRawListenerAndSSE
    Scenario: test_cors_exposes_session_and_allows_stream_headers[sync]
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When cors exposes session and allows stream headers[sync]
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-1a8e88cab8ba @go_TestAsyncHTTPConnectionParserAndCancellation @go_TestAsyncHTTPConnectionPipeline @go_TestHTTPRawListenerAndSSE
    Scenario: test_cors_exposes_session_and_allows_stream_headers[async]
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When cors exposes session and allows stream headers[async]
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-1a1060947435 @go_TestAsyncHTTPConnectionParserAndCancellation @go_TestAsyncHTTPConnectionPipeline @go_TestHTTPRawListenerAndSSE
    Scenario: test_concurrent_session_posts_are_isolated[sync]
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When concurrent session posts are isolated[sync]
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-c64e6912db47 @go_TestAsyncHTTPConnectionParserAndCancellation @go_TestAsyncHTTPConnectionPipeline @go_TestHTTPRawListenerAndSSE
    Scenario: test_concurrent_session_posts_are_isolated[async]
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When concurrent session posts are isolated[async]
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-2c548d3ab60d @go_TestAsyncHTTPConnectionParserAndCancellation @go_TestAsyncHTTPConnectionPipeline @go_TestHTTPRawListenerAndSSE
    Scenario: test_idle_timeout_expiry_and_duplicate_session_headers[sync]
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When idle timeout expiry and duplicate session headers[sync]
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-9b04872fc9fa @go_TestAsyncHTTPConnectionParserAndCancellation @go_TestAsyncHTTPConnectionPipeline @go_TestHTTPRawListenerAndSSE
    Scenario: test_idle_timeout_expiry_and_duplicate_session_headers[async]
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When idle timeout expiry and duplicate session headers[async]
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-bdafdc8f6343 @go_TestAsyncHTTPConnectionParserAndCancellation @go_TestAsyncHTTPConnectionPipeline @go_TestHTTPRawListenerAndSSE
    Scenario: test_event_stream_requires_session_owner_and_version[sync]
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When event stream requires session owner and version[sync]
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-5d33113c880e @go_TestAsyncHTTPConnectionParserAndCancellation @go_TestAsyncHTTPConnectionPipeline @go_TestHTTPRawListenerAndSSE
    Scenario: test_event_stream_requires_session_owner_and_version[async]
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When event stream requires session owner and version[async]
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

  Rule: Behavior captured from test_streamable_http_sync_async.py

    @umcp-e5ed2b6845ab @go_TestAsyncHTTPConnectionParserAndCancellation @go_TestAsyncHTTPConnectionPipeline @go_TestHTTPRawListenerAndSSE
    Scenario: test_sync_tool_context_from_process_request
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When sync tool context from process request
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-9ad23774c0f2 @go_TestAsyncHTTPConnectionParserAndCancellation @go_TestAsyncHTTPConnectionPipeline @go_TestHTTPRawListenerAndSSE
    Scenario: test_async_tool_context_from_process_request
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When async tool context from process request
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-cf4ba520596c @go_TestAsyncHTTPConnectionParserAndCancellation @go_TestAsyncHTTPConnectionPipeline @go_TestHTTPRawListenerAndSSE
    Scenario: test_async_streamable_http_accepts_json_and_context
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When async streamable http accepts json and context
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error

    @umcp-4bbab1c7c978 @go_TestAsyncHTTPConnectionParserAndCancellation @go_TestAsyncHTTPConnectionPipeline @go_TestHTTPRawListenerAndSSE
    Scenario: test_async_streamable_http_explains_missing_and_unsupported_protocol_versions
      Given the pinned Python uMCP server or client and controlled protocol inputs
      When async streamable http explains missing and unsupported protocol versions
      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error
