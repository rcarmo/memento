Feature: mcp/direct mutations

  Each operation has a success contract, four canonical role outcomes, and a failure contract.
  Stable row tags link behavior to validation data without exposing implementation details.

  Rule: create workflow

    @surface-125c710ec91b
    Scenario: memory_create succeeds with its declared contract
      Given required arguments path, concept_type, title, body, expected_revision, idempotency_key
      And optional arguments aliases, description, tags
      And declared defaults {"aliases":[],"description":null,"tags":[]}
      And policy scope curator role plus operation-specific visible/read/write namespace checks
      When an authorized client invokes memory_create
      Then the response exposes status, data, warnings[], next_tools[], repo_revision, index_revision, index_stale, operation_id, path, id, repo_revision, changed_paths[], operation_id
      And side effects are publishes a Git/control operation and updates derived state after commit
      And idempotency is required key; exact replay returns prior result; changed payload conflicts
      And pagination or range behavior is none

    @surface-125c710ec91b_role_reader
    Scenario: memory_create as reader
      Given the canonical reader profile
      When that principal discovers or invokes memory_create
      Then the outcome is hidden_and_forbidden

    @surface-125c710ec91b_role_proposer
    Scenario: memory_create as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes memory_create
      Then the outcome is hidden_and_forbidden

    @surface-125c710ec91b_role_curator
    Scenario: memory_create as curator
      Given the canonical curator profile
      When that principal discovers or invokes memory_create
      Then the outcome is discover_and_call

    @surface-125c710ec91b_role_admin
    Scenario: memory_create as admin
      Given the canonical admin profile
      When that principal discovers or invokes memory_create
      Then the outcome is discover_and_call

    @surface-125c710ec91b_failure
    Scenario: memory_create rejects invalid or conflicting requests
      Given required arguments path, concept_type, title, body, expected_revision, idempotency_key and declared defaults {"aliases":[],"description":null,"tags":[]}
      And malformed, missing, out-of-scope, unavailable, replayed, or conflicting inputs
      When the client invokes memory_create
      Then one of the specified failures is validation_error: schema/domain validation; forbidden: required role or namespace grant absent; not_found/conflict/idempotency_conflict where defined; unexpected internal failures are transport-redacted; observed JSON-RPC -32602: Required parameter 'path' is missing
      And failed pre-publication calls do not apply publishes a Git/control operation and updates derived state after commit

  Rule: patch workflow

    @surface-69ad95b805dc
    Scenario: memory_patch succeeds with its declared contract
      Given required arguments path, expected_revision, idempotency_key
      And optional arguments aliases, body, description, status, tags, title
      And declared defaults {"aliases":null,"body":null,"description":null,"status":null,"tags":null,"title":null}
      And policy scope curator role plus operation-specific visible/read/write namespace checks
      When an authorized client invokes memory_patch
      Then the response exposes status, data, warnings[], next_tools[], repo_revision, index_revision, index_stale, operation_id, path, id, repo_revision, changed_paths[], operation_id
      And side effects are publishes a Git/control operation and updates derived state after commit
      And idempotency is required key; exact replay returns prior result; changed payload conflicts
      And pagination or range behavior is none

    @surface-69ad95b805dc_role_reader
    Scenario: memory_patch as reader
      Given the canonical reader profile
      When that principal discovers or invokes memory_patch
      Then the outcome is hidden_and_forbidden

    @surface-69ad95b805dc_role_proposer
    Scenario: memory_patch as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes memory_patch
      Then the outcome is hidden_and_forbidden

    @surface-69ad95b805dc_role_curator
    Scenario: memory_patch as curator
      Given the canonical curator profile
      When that principal discovers or invokes memory_patch
      Then the outcome is discover_and_call

    @surface-69ad95b805dc_role_admin
    Scenario: memory_patch as admin
      Given the canonical admin profile
      When that principal discovers or invokes memory_patch
      Then the outcome is discover_and_call

    @surface-69ad95b805dc_failure
    Scenario: memory_patch rejects invalid or conflicting requests
      Given required arguments path, expected_revision, idempotency_key and declared defaults {"aliases":null,"body":null,"description":null,"status":null,"tags":null,"title":null}
      And malformed, missing, out-of-scope, unavailable, replayed, or conflicting inputs
      When the client invokes memory_patch
      Then one of the specified failures is validation_error: schema/domain validation; forbidden: required role or namespace grant absent; not_found/conflict/idempotency_conflict where defined; unexpected internal failures are transport-redacted; observed JSON-RPC -32602: Required parameter 'path' is missing
      And failed pre-publication calls do not apply publishes a Git/control operation and updates derived state after commit

  Rule: trash workflow

    @surface-4c8b7b60fd20
    Scenario: memory_trash succeeds with its declared contract
      Given required arguments path, expected_revision, idempotency_key
      And optional arguments no optional arguments
      And declared defaults no declared defaults
      And policy scope curator role plus operation-specific visible/read/write namespace checks
      When an authorized client invokes memory_trash
      Then the response exposes status, data, warnings[], next_tools[], repo_revision, index_revision, index_stale, operation_id, path, trash_path, repo_revision, changed_paths[], operation_id
      And side effects are publishes a Git/control operation and updates derived state after commit
      And idempotency is required key; exact replay returns prior result; changed payload conflicts
      And pagination or range behavior is none

    @surface-4c8b7b60fd20_role_reader
    Scenario: memory_trash as reader
      Given the canonical reader profile
      When that principal discovers or invokes memory_trash
      Then the outcome is hidden_and_forbidden

    @surface-4c8b7b60fd20_role_proposer
    Scenario: memory_trash as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes memory_trash
      Then the outcome is hidden_and_forbidden

    @surface-4c8b7b60fd20_role_curator
    Scenario: memory_trash as curator
      Given the canonical curator profile
      When that principal discovers or invokes memory_trash
      Then the outcome is discover_and_call

    @surface-4c8b7b60fd20_role_admin
    Scenario: memory_trash as admin
      Given the canonical admin profile
      When that principal discovers or invokes memory_trash
      Then the outcome is discover_and_call

    @surface-4c8b7b60fd20_failure
    Scenario: memory_trash rejects invalid or conflicting requests
      Given required arguments path, expected_revision, idempotency_key and declared defaults no declared defaults
      And malformed, missing, out-of-scope, unavailable, replayed, or conflicting inputs
      When the client invokes memory_trash
      Then one of the specified failures is validation_error: schema/domain validation; forbidden: required role or namespace grant absent; not_found/conflict/idempotency_conflict where defined; unexpected internal failures are transport-redacted; observed JSON-RPC -32602: Required parameter 'path' is missing
      And failed pre-publication calls do not apply publishes a Git/control operation and updates derived state after commit

  Rule: restore workflow

    @surface-51275a4cb14f
    Scenario: memory_restore succeeds with its declared contract
      Given required arguments path, expected_revision, idempotency_key
      And optional arguments no optional arguments
      And declared defaults no declared defaults
      And policy scope curator role plus operation-specific visible/read/write namespace checks
      When an authorized client invokes memory_restore
      Then the response exposes status, data, warnings[], next_tools[], repo_revision, index_revision, index_stale, operation_id, path, restored_path, repo_revision, changed_paths[], operation_id
      And side effects are publishes a Git/control operation and updates derived state after commit
      And idempotency is required key; exact replay returns prior result; changed payload conflicts
      And pagination or range behavior is none

    @surface-51275a4cb14f_role_reader
    Scenario: memory_restore as reader
      Given the canonical reader profile
      When that principal discovers or invokes memory_restore
      Then the outcome is hidden_and_forbidden

    @surface-51275a4cb14f_role_proposer
    Scenario: memory_restore as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes memory_restore
      Then the outcome is hidden_and_forbidden

    @surface-51275a4cb14f_role_curator
    Scenario: memory_restore as curator
      Given the canonical curator profile
      When that principal discovers or invokes memory_restore
      Then the outcome is discover_and_call

    @surface-51275a4cb14f_role_admin
    Scenario: memory_restore as admin
      Given the canonical admin profile
      When that principal discovers or invokes memory_restore
      Then the outcome is discover_and_call

    @surface-51275a4cb14f_failure
    Scenario: memory_restore rejects invalid or conflicting requests
      Given required arguments path, expected_revision, idempotency_key and declared defaults no declared defaults
      And malformed, missing, out-of-scope, unavailable, replayed, or conflicting inputs
      When the client invokes memory_restore
      Then one of the specified failures is validation_error: schema/domain validation; forbidden: required role or namespace grant absent; not_found/conflict/idempotency_conflict where defined; unexpected internal failures are transport-redacted; observed JSON-RPC -32602: Required parameter 'path' is missing
      And failed pre-publication calls do not apply publishes a Git/control operation and updates derived state after commit

  Rule: purge workflow

    @surface-0334d36d2465
    Scenario: memory_purge succeeds with its declared contract
      Given required arguments path, expected_revision, idempotency_key
      And optional arguments confirm
      And declared defaults {"confirm":false}
      And policy scope curator role plus operation-specific visible/read/write namespace checks
      When an authorized client invokes memory_purge
      Then the response exposes status, data, warnings[], next_tools[], repo_revision, index_revision, index_stale, operation_id, path, repo_revision, changed_paths[], operation_id
      And side effects are publishes a Git/control operation and updates derived state after commit
      And idempotency is required key; exact replay returns prior result; changed payload conflicts
      And pagination or range behavior is none

    @surface-0334d36d2465_role_reader
    Scenario: memory_purge as reader
      Given the canonical reader profile
      When that principal discovers or invokes memory_purge
      Then the outcome is hidden_and_forbidden

    @surface-0334d36d2465_role_proposer
    Scenario: memory_purge as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes memory_purge
      Then the outcome is hidden_and_forbidden

    @surface-0334d36d2465_role_curator
    Scenario: memory_purge as curator
      Given the canonical curator profile
      When that principal discovers or invokes memory_purge
      Then the outcome is discover_and_call

    @surface-0334d36d2465_role_admin
    Scenario: memory_purge as admin
      Given the canonical admin profile
      When that principal discovers or invokes memory_purge
      Then the outcome is discover_and_call

    @surface-0334d36d2465_failure
    Scenario: memory_purge rejects invalid or conflicting requests
      Given required arguments path, expected_revision, idempotency_key and declared defaults {"confirm":false}
      And malformed, missing, out-of-scope, unavailable, replayed, or conflicting inputs
      When the client invokes memory_purge
      Then one of the specified failures is validation_error: schema/domain validation; forbidden: required role or namespace grant absent; not_found/conflict/idempotency_conflict where defined; unexpected internal failures are transport-redacted; observed JSON-RPC -32602: Required parameter 'path' is missing
      And failed pre-publication calls do not apply publishes a Git/control operation and updates derived state after commit

  Rule: rename workflow

    @surface-085fbec32750
    Scenario: memory_rename succeeds with its declared contract
      Given required arguments path, new_path, expected_revision, idempotency_key
      And optional arguments no optional arguments
      And declared defaults no declared defaults
      And policy scope curator role plus operation-specific visible/read/write namespace checks
      When an authorized client invokes memory_rename
      Then the response exposes status, data, warnings[], next_tools[], repo_revision, index_revision, index_stale, operation_id, path, new_path, repo_revision, changed_paths[], operation_id
      And side effects are publishes a Git/control operation and updates derived state after commit
      And idempotency is required key; exact replay returns prior result; changed payload conflicts
      And pagination or range behavior is none

    @surface-085fbec32750_role_reader
    Scenario: memory_rename as reader
      Given the canonical reader profile
      When that principal discovers or invokes memory_rename
      Then the outcome is hidden_and_forbidden

    @surface-085fbec32750_role_proposer
    Scenario: memory_rename as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes memory_rename
      Then the outcome is hidden_and_forbidden

    @surface-085fbec32750_role_curator
    Scenario: memory_rename as curator
      Given the canonical curator profile
      When that principal discovers or invokes memory_rename
      Then the outcome is discover_and_call

    @surface-085fbec32750_role_admin
    Scenario: memory_rename as admin
      Given the canonical admin profile
      When that principal discovers or invokes memory_rename
      Then the outcome is discover_and_call

    @surface-085fbec32750_failure
    Scenario: memory_rename rejects invalid or conflicting requests
      Given required arguments path, new_path, expected_revision, idempotency_key and declared defaults no declared defaults
      And malformed, missing, out-of-scope, unavailable, replayed, or conflicting inputs
      When the client invokes memory_rename
      Then one of the specified failures is validation_error: schema/domain validation; forbidden: required role or namespace grant absent; not_found/conflict/idempotency_conflict where defined; unexpected internal failures are transport-redacted; observed JSON-RPC -32602: Required parameter 'path' is missing
      And failed pre-publication calls do not apply publishes a Git/control operation and updates derived state after commit
