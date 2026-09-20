# Pinned upstream test crosswalk

Before the reference tree was removed, the parent compatibility workflow ran all 254 tests from `rcarmo/umcp@30cce7dfe08c6ee63de235f7d81754ba286dafbb`. The retained crosswalk groups Go coverage by observable behaviour rather than Python class identity:

| Upstream tests | Go tests |
| --- | --- |
| `test_annotations.py`, `test_introspection.py`, `test_schema_fallbacks.py`, `test_schema_generation.py`, `test_tools.py`, `test_coercion.py` | `tools_test.go`, `tools_extra_test.go`, `schema_test.go`, `tool_result_test.go` |
| `test_prompts.py`, `test_prompts_extra.py`, `test_async_prompts.py` | `prompts_test.go`, `prompts_extra_test.go` |
| `test_resources.py`, `test_resources_extra.py` | `resources_test.go`, `resources_extra_test.go` |
| `test_completion_logging.py` | `completion_test.go`, `completion_extra_test.go`, `server_extra_test.go` |
| `test_discovery_pagination.py` | `pagination_test.go`, registry list tests |
| `test_notifications.py`, `test_tool_outputs_and_runtime_notifications.py` | `notifications_test.go`, `tool_result_test.go`, `transport_notifications_test.go` |
| `test_progress_cancellation.py` | `progress_parity_test.go`, `dispatch_test.go`, `server_extra_test.go` |
| `test_protocol_errors.py`, `test_shared_negotiation_context.py` | `shared_test.go`, `dispatch_test.go`, `server_extra_test.go`, `http_rules_test.go` |
| `test_streamable_http_regressions.py`, `test_streamable_http_sessions.py`, `test_streamable_http_sync_async.py` | `http_test.go`, `http_extra_test.go`, `http_parser_test.go`, `http_sync_parser_test.go`, `http_sync_connection_test.go` |
| `test_transports.py` | `stdio_test.go`, `stream_transports_test.go`, `sse_test.go`, `transport_cli_test.go` |
| `test_async_servers.py`, `test_movieserver.py` | async listener/raw parser tests and complete registry/server integration tests |

The source-generated 22 fixture families provide exact cross-language payload/framing decisions. `TestDifferentialFixtureManifest` requires every family to exist and be referenced. Module statement coverage is 100%; race, fuzz and amd64-v1/ARM64 cross-build gates run independently.
