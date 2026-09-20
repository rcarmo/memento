Feature: mcp/assets and staging

  Each operation has a success contract, four canonical role outcomes, and a failure contract.
  Stable row tags link behavior to validation data without exposing implementation details.

  Rule: asset workflow

    @surface-43a98ab93c7b
    Scenario: memory_asset_metadata succeeds with its declared contract
      Given required arguments no required arguments
      And optional arguments asset_kind, cursor, file_limit, id_or_path, include_files, limit, path_prefix, version, version_limit
      And declared defaults {"asset_kind":null,"cursor":null,"file_limit":50,"id_or_path":null,"include_files":false,"limit":20,"path_prefix":null,"version":null,"version_limit":5}
      And policy scope reader role plus operation-specific visible/read/write namespace checks
      When an authorized client invokes memory_asset_metadata
      Then the response exposes status, data, warnings[], next_tools[], repo_revision, index_revision, index_stale, operation_id, results[], results[].concept_id, results[].concept_path, results[].asset_kinds, next_cursor
      And side effects are none
      And idempotency is not_applicable
      And pagination or range behavior is bounded limit plus policy/filter/revision-bound cursor; next_cursor is null/omitted at end

    @surface-43a98ab93c7b_role_reader
    Scenario: memory_asset_metadata as reader
      Given the canonical reader profile
      When that principal discovers or invokes memory_asset_metadata
      Then the outcome is discover_and_call

    @surface-43a98ab93c7b_role_proposer
    Scenario: memory_asset_metadata as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes memory_asset_metadata
      Then the outcome is discover_and_call

    @surface-43a98ab93c7b_role_curator
    Scenario: memory_asset_metadata as curator
      Given the canonical curator profile
      When that principal discovers or invokes memory_asset_metadata
      Then the outcome is discover_and_call

    @surface-43a98ab93c7b_role_admin
    Scenario: memory_asset_metadata as admin
      Given the canonical admin profile
      When that principal discovers or invokes memory_asset_metadata
      Then the outcome is discover_and_call

    @surface-43a98ab93c7b_failure
    Scenario: memory_asset_metadata rejects invalid or conflicting requests
      Given required arguments no required arguments and declared defaults {"asset_kind":null,"cursor":null,"file_limit":50,"id_or_path":null,"include_files":false,"limit":20,"path_prefix":null,"version":null,"version_limit":5}
      And malformed, missing, out-of-scope, unavailable, replayed, or conflicting inputs
      When the client invokes memory_asset_metadata
      Then one of the specified failures is validation_error: schema/domain validation; forbidden: required role or namespace grant absent; not_found/conflict/idempotency_conflict where defined; unexpected internal failures are transport-redacted; observed JSON-RPC -32602: Unrecognized parameter(s): principal
      And failed pre-publication calls do not apply none

    @surface-225a9d544903
    Scenario: memory_asset_stage_begin succeeds with its declared contract
      Given required arguments asset_kind, version, idempotency_key
      And optional arguments no optional arguments
      And declared defaults no declared defaults
      And policy scope proposer role plus operation-specific visible/read/write namespace checks
      When an authorized client invokes memory_asset_stage_begin
      Then the response exposes status, data, warnings[], next_tools[], repo_revision, index_revision, index_stale, operation_id, idempotency_key, upload_ticket, upload_url, expires_at, state
      And side effects are none
      And idempotency is not_applicable
      And pagination or range behavior is none

    @surface-225a9d544903_role_reader
    Scenario: memory_asset_stage_begin as reader
      Given the canonical reader profile
      When that principal discovers or invokes memory_asset_stage_begin
      Then the outcome is hidden_and_forbidden

    @surface-225a9d544903_role_proposer
    Scenario: memory_asset_stage_begin as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes memory_asset_stage_begin
      Then the outcome is discover_and_call

    @surface-225a9d544903_role_curator
    Scenario: memory_asset_stage_begin as curator
      Given the canonical curator profile
      When that principal discovers or invokes memory_asset_stage_begin
      Then the outcome is discover_and_call

    @surface-225a9d544903_role_admin
    Scenario: memory_asset_stage_begin as admin
      Given the canonical admin profile
      When that principal discovers or invokes memory_asset_stage_begin
      Then the outcome is discover_and_call

    @surface-225a9d544903_failure
    Scenario: memory_asset_stage_begin rejects invalid or conflicting requests
      Given required arguments asset_kind, version, idempotency_key and declared defaults no declared defaults
      And malformed, missing, out-of-scope, unavailable, replayed, or conflicting inputs
      When the client invokes memory_asset_stage_begin
      Then one of the specified failures is validation_error: schema/domain validation; forbidden: required role or namespace grant absent; not_found/conflict/idempotency_conflict where defined; unexpected internal failures are transport-redacted; observed JSON-RPC -32602: Required parameter 'asset_kind' is missing; observed JSON-RPC -32602: Unrecognized parameter(s): principal
      And failed pre-publication calls do not apply none

    @surface-125cc3f7941c
    Scenario: memory_asset_stage_status succeeds with its declared contract
      Given required arguments idempotency_key
      And optional arguments no optional arguments
      And declared defaults no declared defaults
      And policy scope proposer role plus operation-specific visible/read/write namespace checks
      When an authorized client invokes memory_asset_stage_status
      Then the response exposes status, data, warnings[], next_tools[], repo_revision, index_revision, index_stale, operation_id, idempotency_key, state, staged_asset_id, expires_at
      And side effects are none
      And idempotency is not_applicable
      And pagination or range behavior is none

    @surface-125cc3f7941c_role_reader
    Scenario: memory_asset_stage_status as reader
      Given the canonical reader profile
      When that principal discovers or invokes memory_asset_stage_status
      Then the outcome is hidden_and_forbidden

    @surface-125cc3f7941c_role_proposer
    Scenario: memory_asset_stage_status as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes memory_asset_stage_status
      Then the outcome is discover_and_call

    @surface-125cc3f7941c_role_curator
    Scenario: memory_asset_stage_status as curator
      Given the canonical curator profile
      When that principal discovers or invokes memory_asset_stage_status
      Then the outcome is discover_and_call

    @surface-125cc3f7941c_role_admin
    Scenario: memory_asset_stage_status as admin
      Given the canonical admin profile
      When that principal discovers or invokes memory_asset_stage_status
      Then the outcome is discover_and_call

    @surface-125cc3f7941c_failure
    Scenario: memory_asset_stage_status rejects invalid or conflicting requests
      Given required arguments idempotency_key and declared defaults no declared defaults
      And malformed, missing, out-of-scope, unavailable, replayed, or conflicting inputs
      When the client invokes memory_asset_stage_status
      Then one of the specified failures is validation_error: schema/domain validation; forbidden: required role or namespace grant absent; not_found/conflict/idempotency_conflict where defined; unexpected internal failures are transport-redacted; observed JSON-RPC -32602: Required parameter 'idempotency_key' is missing
      And failed pre-publication calls do not apply none

    @surface-ae92a01f6f63
    Scenario: memory_asset_get succeeds with its declared contract
      Given required arguments id_or_path, asset_kind
      And optional arguments expected_sha256, file_path, limit, offset, version, view
      And declared defaults {"expected_sha256":null,"file_path":null,"limit":null,"offset":0,"version":null,"view":"archive"}
      And policy scope reader role plus operation-specific visible/read/write namespace checks
      When an authorized client invokes memory_asset_get
      Then the response exposes status, data, warnings[], next_tools[], repo_revision, index_revision, index_stale, operation_id, concept_id, concept_path, asset_kind, version, versions[], zip_sha256, manifest, file, archive, next_offset
      And side effects are none
      And idempotency is not_applicable
      And pagination or range behavior is bounded byte offset/limit; response reports total/returned/truncated and next_offset

    @surface-ae92a01f6f63_role_reader
    Scenario: memory_asset_get as reader
      Given the canonical reader profile
      When that principal discovers or invokes memory_asset_get
      Then the outcome is discover_and_call

    @surface-ae92a01f6f63_role_proposer
    Scenario: memory_asset_get as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes memory_asset_get
      Then the outcome is discover_and_call

    @surface-ae92a01f6f63_role_curator
    Scenario: memory_asset_get as curator
      Given the canonical curator profile
      When that principal discovers or invokes memory_asset_get
      Then the outcome is discover_and_call

    @surface-ae92a01f6f63_role_admin
    Scenario: memory_asset_get as admin
      Given the canonical admin profile
      When that principal discovers or invokes memory_asset_get
      Then the outcome is discover_and_call

    @surface-ae92a01f6f63_failure
    Scenario: memory_asset_get rejects invalid or conflicting requests
      Given required arguments id_or_path, asset_kind and declared defaults {"expected_sha256":null,"file_path":null,"limit":null,"offset":0,"version":null,"view":"archive"}
      And malformed, missing, out-of-scope, unavailable, replayed, or conflicting inputs
      When the client invokes memory_asset_get
      Then one of the specified failures is validation_error: schema/domain validation; forbidden: required role or namespace grant absent; not_found/conflict/idempotency_conflict where defined; unexpected internal failures are transport-redacted; observed JSON-RPC -32602: Required parameter 'id_or_path' is missing
      And failed pre-publication calls do not apply none

    @surface-51bd9bde8934
    Scenario: memory_asset_prune succeeds with its declared contract
      Given required arguments id_or_path, asset_kind, expected_revision, idempotency_key
      And optional arguments keep
      And declared defaults {"keep":5}
      And policy scope curator role plus operation-specific visible/read/write namespace checks
      When an authorized client invokes memory_asset_prune
      Then the response exposes status, data, warnings[], next_tools[], repo_revision, index_revision, index_stale, operation_id, concept_id, asset_kind, removed_versions[], kept_versions[], repo_revision, operation_id
      And side effects are publishes a Git/control operation and updates derived state after commit
      And idempotency is required key; exact replay returns prior result; changed payload conflicts
      And pagination or range behavior is none

    @surface-51bd9bde8934_role_reader
    Scenario: memory_asset_prune as reader
      Given the canonical reader profile
      When that principal discovers or invokes memory_asset_prune
      Then the outcome is hidden_and_forbidden

    @surface-51bd9bde8934_role_proposer
    Scenario: memory_asset_prune as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes memory_asset_prune
      Then the outcome is hidden_and_forbidden

    @surface-51bd9bde8934_role_curator
    Scenario: memory_asset_prune as curator
      Given the canonical curator profile
      When that principal discovers or invokes memory_asset_prune
      Then the outcome is discover_and_call

    @surface-51bd9bde8934_role_admin
    Scenario: memory_asset_prune as admin
      Given the canonical admin profile
      When that principal discovers or invokes memory_asset_prune
      Then the outcome is discover_and_call

    @surface-51bd9bde8934_failure
    Scenario: memory_asset_prune rejects invalid or conflicting requests
      Given required arguments id_or_path, asset_kind, expected_revision, idempotency_key and declared defaults {"keep":5}
      And malformed, missing, out-of-scope, unavailable, replayed, or conflicting inputs
      When the client invokes memory_asset_prune
      Then one of the specified failures is validation_error: schema/domain validation; forbidden: required role or namespace grant absent; not_found/conflict/idempotency_conflict where defined; unexpected internal failures are transport-redacted; observed JSON-RPC -32602: Required parameter 'id_or_path' is missing
      And failed pre-publication calls do not apply publishes a Git/control operation and updates derived state after commit
