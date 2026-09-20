Feature: mcp/discovery and reading

  Each operation has a success contract, four canonical role outcomes, and a failure contract.
  Stable row tags link behavior to validation data without exposing implementation details.

  Rule: help workflow

    @surface-c6906a04e6cb
    Scenario: memory_help succeeds with its declared contract
      Given required arguments no required arguments
      And optional arguments no optional arguments
      And declared defaults no declared defaults
      And policy scope reader role plus operation-specific visible/read/write namespace checks
      When an authorized client invokes memory_help
      Then the response exposes status, data, warnings[], next_tools[], repo_revision, index_revision, index_stale, operation_id, workflows[], resources[], tool_surface, limits
      And side effects are none
      And idempotency is not_applicable
      And pagination or range behavior is none

    @surface-c6906a04e6cb_role_reader
    Scenario: memory_help as reader
      Given the canonical reader profile
      When that principal discovers or invokes memory_help
      Then the outcome is discover_and_call

    @surface-c6906a04e6cb_role_proposer
    Scenario: memory_help as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes memory_help
      Then the outcome is discover_and_call

    @surface-c6906a04e6cb_role_curator
    Scenario: memory_help as curator
      Given the canonical curator profile
      When that principal discovers or invokes memory_help
      Then the outcome is discover_and_call

    @surface-c6906a04e6cb_role_admin
    Scenario: memory_help as admin
      Given the canonical admin profile
      When that principal discovers or invokes memory_help
      Then the outcome is discover_and_call

    @surface-c6906a04e6cb_failure
    Scenario: memory_help rejects invalid or conflicting requests
      Given required arguments no required arguments and declared defaults no declared defaults
      And malformed, missing, out-of-scope, unavailable, replayed, or conflicting inputs
      When the client invokes memory_help
      Then one of the specified failures is validation_error: schema/domain validation; forbidden: required role or namespace grant absent; not_found/conflict/idempotency_conflict where defined; unexpected internal failures are transport-redacted
      And failed pre-publication calls do not apply none

  Rule: status workflow

    @surface-213a4c43ed32
    Scenario: memory_status succeeds with its declared contract
      Given required arguments no required arguments
      And optional arguments no optional arguments
      And declared defaults no declared defaults
      And policy scope reader role plus operation-specific visible/read/write namespace checks
      When an authorized client invokes memory_status
      Then the response exposes status, data, warnings[], next_tools[], repo_revision, index_revision, index_stale, operation_id, service_version, schema_version, principal, roles[], repo_revision, index_revision, index_stale, visible_concepts, proposal_backlog, readiness, features, limits
      And side effects are none
      And idempotency is not_applicable
      And pagination or range behavior is none

    @surface-213a4c43ed32_role_reader
    Scenario: memory_status as reader
      Given the canonical reader profile
      When that principal discovers or invokes memory_status
      Then the outcome is discover_and_call

    @surface-213a4c43ed32_role_proposer
    Scenario: memory_status as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes memory_status
      Then the outcome is discover_and_call

    @surface-213a4c43ed32_role_curator
    Scenario: memory_status as curator
      Given the canonical curator profile
      When that principal discovers or invokes memory_status
      Then the outcome is discover_and_call

    @surface-213a4c43ed32_role_admin
    Scenario: memory_status as admin
      Given the canonical admin profile
      When that principal discovers or invokes memory_status
      Then the outcome is discover_and_call

    @surface-213a4c43ed32_failure
    Scenario: memory_status rejects invalid or conflicting requests
      Given required arguments no required arguments and declared defaults no declared defaults
      And malformed, missing, out-of-scope, unavailable, replayed, or conflicting inputs
      When the client invokes memory_status
      Then one of the specified failures is validation_error: schema/domain validation; forbidden: required role or namespace grant absent; not_found/conflict/idempotency_conflict where defined; unexpected internal failures are transport-redacted; observed JSON-RPC -32602: Unrecognized parameter(s): principal
      And failed pre-publication calls do not apply none

  Rule: search workflow

    @surface-6c54892a4a07
    Scenario: memory_search succeeds with its declared contract
      Given required arguments query
      And optional arguments concept_type, cursor, limit, query_syntax, search_mode
      And declared defaults {"concept_type":null,"cursor":null,"limit":20,"query_syntax":"plain","search_mode":null}
      And policy scope reader role plus operation-specific visible/read/write namespace checks
      When an authorized client invokes memory_search
      Then the response exposes status, data, warnings[], next_tools[], repo_revision, index_revision, index_stale, operation_id, search_mode, query_syntax, results[], results[].id, results[].path, results[].title, results[].score, results[].snippet, next_cursor
      And side effects are none
      And idempotency is not_applicable
      And pagination or range behavior is bounded limit plus policy/filter/revision-bound cursor; next_cursor is null/omitted at end

    @surface-6c54892a4a07_role_reader
    Scenario: memory_search as reader
      Given the canonical reader profile
      When that principal discovers or invokes memory_search
      Then the outcome is discover_and_call

    @surface-6c54892a4a07_role_proposer
    Scenario: memory_search as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes memory_search
      Then the outcome is discover_and_call

    @surface-6c54892a4a07_role_curator
    Scenario: memory_search as curator
      Given the canonical curator profile
      When that principal discovers or invokes memory_search
      Then the outcome is discover_and_call

    @surface-6c54892a4a07_role_admin
    Scenario: memory_search as admin
      Given the canonical admin profile
      When that principal discovers or invokes memory_search
      Then the outcome is discover_and_call

    @surface-6c54892a4a07_failure
    Scenario: memory_search rejects invalid or conflicting requests
      Given required arguments query and declared defaults {"concept_type":null,"cursor":null,"limit":20,"query_syntax":"plain","search_mode":null}
      And malformed, missing, out-of-scope, unavailable, replayed, or conflicting inputs
      When the client invokes memory_search
      Then one of the specified failures is validation_error: schema/domain validation; forbidden: required role or namespace grant absent; not_found/conflict/idempotency_conflict where defined; unexpected internal failures are transport-redacted
      And failed pre-publication calls do not apply none

  Rule: read workflow

    @surface-4447ab44f238
    Scenario: memory_read succeeds with its declared contract
      Given required arguments id_or_path
      And optional arguments no optional arguments
      And declared defaults no declared defaults
      And policy scope reader role plus operation-specific visible/read/write namespace checks
      When an authorized client invokes memory_read
      Then the response exposes status, data, warnings[], next_tools[], repo_revision, index_revision, index_stale, operation_id, path, frontmatter, body
      And side effects are none
      And idempotency is not_applicable
      And pagination or range behavior is none

    @surface-4447ab44f238_role_reader
    Scenario: memory_read as reader
      Given the canonical reader profile
      When that principal discovers or invokes memory_read
      Then the outcome is discover_and_call

    @surface-4447ab44f238_role_proposer
    Scenario: memory_read as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes memory_read
      Then the outcome is discover_and_call

    @surface-4447ab44f238_role_curator
    Scenario: memory_read as curator
      Given the canonical curator profile
      When that principal discovers or invokes memory_read
      Then the outcome is discover_and_call

    @surface-4447ab44f238_role_admin
    Scenario: memory_read as admin
      Given the canonical admin profile
      When that principal discovers or invokes memory_read
      Then the outcome is discover_and_call

    @surface-4447ab44f238_failure
    Scenario: memory_read rejects invalid or conflicting requests
      Given required arguments id_or_path and declared defaults no declared defaults
      And malformed, missing, out-of-scope, unavailable, replayed, or conflicting inputs
      When the client invokes memory_read
      Then one of the specified failures is validation_error: schema/domain validation; forbidden: required role or namespace grant absent; not_found/conflict/idempotency_conflict where defined; unexpected internal failures are transport-redacted
      And failed pre-publication calls do not apply none

  Rule: list workflow

    @surface-729f7a949314
    Scenario: memory_list succeeds with its declared contract
      Given required arguments no required arguments
      And optional arguments path_prefix
      And declared defaults {"path_prefix":"/"}
      And policy scope reader role plus operation-specific visible/read/write namespace checks
      When an authorized client invokes memory_list
      Then the response exposes status, data, warnings[], next_tools[], repo_revision, index_revision, index_stale, operation_id, results[], results[].id, results[].path, results[].title, next_cursor
      And side effects are none
      And idempotency is not_applicable
      And pagination or range behavior is bounded limit plus policy/filter/revision-bound cursor; next_cursor is null/omitted at end

    @surface-729f7a949314_role_reader
    Scenario: memory_list as reader
      Given the canonical reader profile
      When that principal discovers or invokes memory_list
      Then the outcome is discover_and_call

    @surface-729f7a949314_role_proposer
    Scenario: memory_list as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes memory_list
      Then the outcome is discover_and_call

    @surface-729f7a949314_role_curator
    Scenario: memory_list as curator
      Given the canonical curator profile
      When that principal discovers or invokes memory_list
      Then the outcome is discover_and_call

    @surface-729f7a949314_role_admin
    Scenario: memory_list as admin
      Given the canonical admin profile
      When that principal discovers or invokes memory_list
      Then the outcome is discover_and_call

    @surface-729f7a949314_failure
    Scenario: memory_list rejects invalid or conflicting requests
      Given required arguments no required arguments and declared defaults {"path_prefix":"/"}
      And malformed, missing, out-of-scope, unavailable, replayed, or conflicting inputs
      When the client invokes memory_list
      Then one of the specified failures is validation_error: schema/domain validation; forbidden: required role or namespace grant absent; not_found/conflict/idempotency_conflict where defined; unexpected internal failures are transport-redacted
      And failed pre-publication calls do not apply none

  Rule: inventory workflow

    @surface-0e9b58d5dc55
    Scenario: memory_inventory succeeds with its declared contract
      Given required arguments no required arguments
      And optional arguments cursor, fields, limit, path_prefix
      And declared defaults {"cursor":null,"fields":["path","id","title","status","type","tags","created_at","updated_at","updated_by","body_sha256","body_bytes","assets"],"limit":50,"path_prefix":"/"}
      And policy scope reader role plus operation-specific visible/read/write namespace checks
      When an authorized client invokes memory_inventory
      Then the response exposes status, data, warnings[], next_tools[], repo_revision, index_revision, index_stale, operation_id, path_prefix, fields[], results[], next_cursor
      And side effects are none
      And idempotency is not_applicable
      And pagination or range behavior is bounded limit plus policy/filter/revision-bound cursor; next_cursor is null/omitted at end

    @surface-0e9b58d5dc55_role_reader
    Scenario: memory_inventory as reader
      Given the canonical reader profile
      When that principal discovers or invokes memory_inventory
      Then the outcome is discover_and_call

    @surface-0e9b58d5dc55_role_proposer
    Scenario: memory_inventory as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes memory_inventory
      Then the outcome is discover_and_call

    @surface-0e9b58d5dc55_role_curator
    Scenario: memory_inventory as curator
      Given the canonical curator profile
      When that principal discovers or invokes memory_inventory
      Then the outcome is discover_and_call

    @surface-0e9b58d5dc55_role_admin
    Scenario: memory_inventory as admin
      Given the canonical admin profile
      When that principal discovers or invokes memory_inventory
      Then the outcome is discover_and_call

    @surface-0e9b58d5dc55_failure
    Scenario: memory_inventory rejects invalid or conflicting requests
      Given required arguments no required arguments and declared defaults {"cursor":null,"fields":["path","id","title","status","type","tags","created_at","updated_at","updated_by","body_sha256","body_bytes","assets"],"limit":50,"path_prefix":"/"}
      And malformed, missing, out-of-scope, unavailable, replayed, or conflicting inputs
      When the client invokes memory_inventory
      Then one of the specified failures is validation_error: schema/domain validation; forbidden: required role or namespace grant absent; not_found/conflict/idempotency_conflict where defined; unexpected internal failures are transport-redacted; observed JSON-RPC -32602: Unrecognized parameter(s): principal
      And failed pre-publication calls do not apply none

  Rule: compare workflow

    @surface-b5cbb09a4d48
    Scenario: memory_compare_manifest succeeds with its declared contract
      Given required arguments items
      And optional arguments include_asset_metadata, match, path_prefix
      And declared defaults {"include_asset_metadata":false,"match":null,"path_prefix":"/"}
      And policy scope reader role plus operation-specific visible/read/write namespace checks
      When an authorized client invokes memory_compare_manifest
      Then the response exposes status, data, warnings[], next_tools[], repo_revision, index_revision, index_stale, operation_id, path_prefix, matching[], differing[], local_only[], memento_only[], matching_count, differing_count, local_only_count, memento_only_count
      And side effects are none
      And idempotency is not_applicable
      And pagination or range behavior is none

    @surface-b5cbb09a4d48_role_reader
    Scenario: memory_compare_manifest as reader
      Given the canonical reader profile
      When that principal discovers or invokes memory_compare_manifest
      Then the outcome is discover_and_call

    @surface-b5cbb09a4d48_role_proposer
    Scenario: memory_compare_manifest as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes memory_compare_manifest
      Then the outcome is discover_and_call

    @surface-b5cbb09a4d48_role_curator
    Scenario: memory_compare_manifest as curator
      Given the canonical curator profile
      When that principal discovers or invokes memory_compare_manifest
      Then the outcome is discover_and_call

    @surface-b5cbb09a4d48_role_admin
    Scenario: memory_compare_manifest as admin
      Given the canonical admin profile
      When that principal discovers or invokes memory_compare_manifest
      Then the outcome is discover_and_call

    @surface-b5cbb09a4d48_failure
    Scenario: memory_compare_manifest rejects invalid or conflicting requests
      Given required arguments items and declared defaults {"include_asset_metadata":false,"match":null,"path_prefix":"/"}
      And malformed, missing, out-of-scope, unavailable, replayed, or conflicting inputs
      When the client invokes memory_compare_manifest
      Then one of the specified failures is validation_error: schema/domain validation; forbidden: required role or namespace grant absent; not_found/conflict/idempotency_conflict where defined; unexpected internal failures are transport-redacted; observed JSON-RPC -32602: Required parameter 'path_prefix' is missing; observed JSON-RPC -32602: Required parameter 'items' is missing
      And failed pre-publication calls do not apply none

  Rule: graph workflow

    @surface-0f2a0410c00b
    Scenario: memory_graph succeeds with its declared contract
      Given required arguments id_or_path
      And optional arguments depth
      And declared defaults {"depth":1}
      And policy scope reader role plus operation-specific visible/read/write namespace checks
      When an authorized client invokes memory_graph
      Then the response exposes status, data, warnings[], next_tools[], repo_revision, index_revision, index_stale, operation_id, center_id, inbound[], outbound[], broken_links[], repo_revision, index_revision
      And side effects are none
      And idempotency is not_applicable
      And pagination or range behavior is none

    @surface-0f2a0410c00b_role_reader
    Scenario: memory_graph as reader
      Given the canonical reader profile
      When that principal discovers or invokes memory_graph
      Then the outcome is discover_and_call

    @surface-0f2a0410c00b_role_proposer
    Scenario: memory_graph as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes memory_graph
      Then the outcome is discover_and_call

    @surface-0f2a0410c00b_role_curator
    Scenario: memory_graph as curator
      Given the canonical curator profile
      When that principal discovers or invokes memory_graph
      Then the outcome is discover_and_call

    @surface-0f2a0410c00b_role_admin
    Scenario: memory_graph as admin
      Given the canonical admin profile
      When that principal discovers or invokes memory_graph
      Then the outcome is discover_and_call

    @surface-0f2a0410c00b_failure
    Scenario: memory_graph rejects invalid or conflicting requests
      Given required arguments id_or_path and declared defaults {"depth":1}
      And malformed, missing, out-of-scope, unavailable, replayed, or conflicting inputs
      When the client invokes memory_graph
      Then one of the specified failures is validation_error: schema/domain validation; forbidden: required role or namespace grant absent; not_found/conflict/idempotency_conflict where defined; unexpected internal failures are transport-redacted
      And failed pre-publication calls do not apply none
