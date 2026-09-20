Feature: mcp/proposal lifecycle

  Each operation has a success contract, four canonical role outcomes, and a failure contract.
  Stable row tags link behavior to validation data without exposing implementation details.

  Rule: audit workflow

    @surface-b14276d52368
    Scenario: memory_audit succeeds with its declared contract
      Given required arguments no required arguments
      And optional arguments cursor, limit, path, rule, severity
      And declared defaults {"cursor":null,"limit":50,"path":null,"rule":null,"severity":null}
      And policy scope proposer role plus operation-specific visible/read/write namespace checks
      When an authorized client invokes memory_audit
      Then the response exposes status, data, warnings[], next_tools[], repo_revision, index_revision, index_stale, operation_id, ok, issues[], issues[].code, issues[].path, graph, next_cursor
      And side effects are none
      And idempotency is not_applicable
      And pagination or range behavior is bounded limit plus policy/filter/revision-bound cursor; next_cursor is null/omitted at end

    @surface-b14276d52368_role_reader
    Scenario: memory_audit as reader
      Given the canonical reader profile
      When that principal discovers or invokes memory_audit
      Then the outcome is hidden_and_forbidden

    @surface-b14276d52368_role_proposer
    Scenario: memory_audit as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes memory_audit
      Then the outcome is discover_and_call

    @surface-b14276d52368_role_curator
    Scenario: memory_audit as curator
      Given the canonical curator profile
      When that principal discovers or invokes memory_audit
      Then the outcome is discover_and_call

    @surface-b14276d52368_role_admin
    Scenario: memory_audit as admin
      Given the canonical admin profile
      When that principal discovers or invokes memory_audit
      Then the outcome is discover_and_call

    @surface-b14276d52368_failure
    Scenario: memory_audit rejects invalid or conflicting requests
      Given required arguments no required arguments and declared defaults {"cursor":null,"limit":50,"path":null,"rule":null,"severity":null}
      And malformed, missing, out-of-scope, unavailable, replayed, or conflicting inputs
      When the client invokes memory_audit
      Then one of the specified failures is validation_error: schema/domain validation; forbidden: required role or namespace grant absent; not_found/conflict/idempotency_conflict where defined; unexpected internal failures are transport-redacted; observed JSON-RPC -32602: Unrecognized parameter(s): principal
      And failed pre-publication calls do not apply none

  Rule: propose workflow

    @surface-2c384e774062
    Scenario: memory_propose succeeds with its declared contract
      Given required arguments intent, base_revision, changes
      And optional arguments rationale
      And declared defaults {"rationale":null}
      And policy scope proposer role plus operation-specific visible/read/write namespace checks
      When an authorized client invokes memory_propose
      Then the response exposes status, data, warnings[], next_tools[], repo_revision, index_revision, index_stale, operation_id, proposal_id, status, intent, base_revision, changes[]
      And side effects are creates or updates durable proposal/review state without publishing canonical Git unless apply is called
      And idempotency is proposal lifecycle constraints prevent invalid repeats
      And pagination or range behavior is none

    @surface-2c384e774062_role_reader
    Scenario: memory_propose as reader
      Given the canonical reader profile
      When that principal discovers or invokes memory_propose
      Then the outcome is hidden_and_forbidden

    @surface-2c384e774062_role_proposer
    Scenario: memory_propose as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes memory_propose
      Then the outcome is discover_and_call

    @surface-2c384e774062_role_curator
    Scenario: memory_propose as curator
      Given the canonical curator profile
      When that principal discovers or invokes memory_propose
      Then the outcome is discover_and_call

    @surface-2c384e774062_role_admin
    Scenario: memory_propose as admin
      Given the canonical admin profile
      When that principal discovers or invokes memory_propose
      Then the outcome is discover_and_call

    @surface-2c384e774062_failure
    Scenario: memory_propose rejects invalid or conflicting requests
      Given required arguments intent, base_revision, changes and declared defaults {"rationale":null}
      And malformed, missing, out-of-scope, unavailable, replayed, or conflicting inputs
      When the client invokes memory_propose
      Then one of the specified failures is validation_error: schema/domain validation; forbidden: required role or namespace grant absent; not_found/conflict/idempotency_conflict where defined; unexpected internal failures are transport-redacted; observed JSON-RPC -32602: Unrecognized parameter(s): principal
      And failed pre-publication calls do not apply creates or updates durable proposal/review state without publishing canonical Git unless apply is called

    @surface-831a46271491
    Scenario: memory_propose_freeform succeeds with its declared contract
      Given required arguments content
      And optional arguments intent, suggested_path
      And declared defaults {"intent":null,"suggested_path":null}
      And policy scope proposer role plus operation-specific visible/read/write namespace checks
      When an authorized client invokes memory_propose_freeform
      Then the response exposes status, data, warnings[], next_tools[], repo_revision, index_revision, index_stale, operation_id, proposal_id, status, model_attempts[], citations[]
      And side effects are creates or updates durable proposal/review state without publishing canonical Git unless apply is called
      And idempotency is proposal lifecycle constraints prevent invalid repeats
      And pagination or range behavior is none

    @surface-831a46271491_role_reader
    Scenario: memory_propose_freeform as reader
      Given the canonical reader profile
      When that principal discovers or invokes memory_propose_freeform
      Then the outcome is hidden_and_forbidden

    @surface-831a46271491_role_proposer
    Scenario: memory_propose_freeform as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes memory_propose_freeform
      Then the outcome is discover_and_call

    @surface-831a46271491_role_curator
    Scenario: memory_propose_freeform as curator
      Given the canonical curator profile
      When that principal discovers or invokes memory_propose_freeform
      Then the outcome is discover_and_call

    @surface-831a46271491_role_admin
    Scenario: memory_propose_freeform as admin
      Given the canonical admin profile
      When that principal discovers or invokes memory_propose_freeform
      Then the outcome is discover_and_call

    @surface-831a46271491_failure
    Scenario: memory_propose_freeform rejects invalid or conflicting requests
      Given required arguments content and declared defaults {"intent":null,"suggested_path":null}
      And malformed, missing, out-of-scope, unavailable, replayed, or conflicting inputs
      When the client invokes memory_propose_freeform
      Then one of the specified failures is validation_error: schema/domain validation; forbidden: required role or namespace grant absent; not_found/conflict/idempotency_conflict where defined; unexpected internal failures are transport-redacted
      And failed pre-publication calls do not apply creates or updates durable proposal/review state without publishing canonical Git unless apply is called

    @surface-990560045406
    Scenario: memory_propose_update succeeds with its declared contract
      Given required arguments instruction
      And optional arguments target_hint
      And declared defaults {"target_hint":null}
      And policy scope proposer role plus operation-specific visible/read/write namespace checks
      When an authorized client invokes memory_propose_update
      Then the response exposes status, data, warnings[], next_tools[], repo_revision, index_revision, index_stale, operation_id, proposal_id, status, model_attempts[], citations[]
      And side effects are creates or updates durable proposal/review state without publishing canonical Git unless apply is called
      And idempotency is proposal lifecycle constraints prevent invalid repeats
      And pagination or range behavior is none

    @surface-990560045406_role_reader
    Scenario: memory_propose_update as reader
      Given the canonical reader profile
      When that principal discovers or invokes memory_propose_update
      Then the outcome is hidden_and_forbidden

    @surface-990560045406_role_proposer
    Scenario: memory_propose_update as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes memory_propose_update
      Then the outcome is discover_and_call

    @surface-990560045406_role_curator
    Scenario: memory_propose_update as curator
      Given the canonical curator profile
      When that principal discovers or invokes memory_propose_update
      Then the outcome is discover_and_call

    @surface-990560045406_role_admin
    Scenario: memory_propose_update as admin
      Given the canonical admin profile
      When that principal discovers or invokes memory_propose_update
      Then the outcome is discover_and_call

    @surface-990560045406_failure
    Scenario: memory_propose_update rejects invalid or conflicting requests
      Given required arguments instruction and declared defaults {"target_hint":null}
      And malformed, missing, out-of-scope, unavailable, replayed, or conflicting inputs
      When the client invokes memory_propose_update
      Then one of the specified failures is validation_error: schema/domain validation; forbidden: required role or namespace grant absent; not_found/conflict/idempotency_conflict where defined; unexpected internal failures are transport-redacted
      And failed pre-publication calls do not apply creates or updates durable proposal/review state without publishing canonical Git unless apply is called

  Rule: proposal workflow

    @surface-edbf396f4e8e
    Scenario: memory_proposal_get succeeds with its declared contract
      Given required arguments proposal_id
      And optional arguments view
      And declared defaults {"view":"detailed"}
      And policy scope proposer role plus operation-specific visible/read/write namespace checks
      When an authorized client invokes memory_proposal_get
      Then the response exposes status, data, warnings[], next_tools[], repo_revision, index_revision, index_stale, operation_id, proposal, history[], preview, assets[]
      And side effects are none
      And idempotency is not_applicable
      And pagination or range behavior is none

    @surface-edbf396f4e8e_role_reader
    Scenario: memory_proposal_get as reader
      Given the canonical reader profile
      When that principal discovers or invokes memory_proposal_get
      Then the outcome is hidden_and_forbidden

    @surface-edbf396f4e8e_role_proposer
    Scenario: memory_proposal_get as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes memory_proposal_get
      Then the outcome is discover_and_call

    @surface-edbf396f4e8e_role_curator
    Scenario: memory_proposal_get as curator
      Given the canonical curator profile
      When that principal discovers or invokes memory_proposal_get
      Then the outcome is discover_and_call

    @surface-edbf396f4e8e_role_admin
    Scenario: memory_proposal_get as admin
      Given the canonical admin profile
      When that principal discovers or invokes memory_proposal_get
      Then the outcome is discover_and_call

    @surface-edbf396f4e8e_failure
    Scenario: memory_proposal_get rejects invalid or conflicting requests
      Given required arguments proposal_id and declared defaults {"view":"detailed"}
      And malformed, missing, out-of-scope, unavailable, replayed, or conflicting inputs
      When the client invokes memory_proposal_get
      Then one of the specified failures is validation_error: schema/domain validation; forbidden: required role or namespace grant absent; not_found/conflict/idempotency_conflict where defined; unexpected internal failures are transport-redacted; observed JSON-RPC -32602: Required parameter 'proposal_id' is missing
      And failed pre-publication calls do not apply none

    @surface-90db0603af2c
    Scenario: memory_proposal_list succeeds with its declared contract
      Given required arguments no required arguments
      And optional arguments cursor, limit, status
      And declared defaults {"cursor":null,"limit":50,"status":null}
      And policy scope proposer role plus operation-specific visible/read/write namespace checks
      When an authorized client invokes memory_proposal_list
      Then the response exposes status, data, warnings[], next_tools[], repo_revision, index_revision, index_stale, operation_id, proposals[], next_cursor
      And side effects are none
      And idempotency is not_applicable
      And pagination or range behavior is bounded limit plus policy/filter/revision-bound cursor; next_cursor is null/omitted at end

    @surface-90db0603af2c_role_reader
    Scenario: memory_proposal_list as reader
      Given the canonical reader profile
      When that principal discovers or invokes memory_proposal_list
      Then the outcome is hidden_and_forbidden

    @surface-90db0603af2c_role_proposer
    Scenario: memory_proposal_list as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes memory_proposal_list
      Then the outcome is discover_and_call

    @surface-90db0603af2c_role_curator
    Scenario: memory_proposal_list as curator
      Given the canonical curator profile
      When that principal discovers or invokes memory_proposal_list
      Then the outcome is discover_and_call

    @surface-90db0603af2c_role_admin
    Scenario: memory_proposal_list as admin
      Given the canonical admin profile
      When that principal discovers or invokes memory_proposal_list
      Then the outcome is discover_and_call

    @surface-90db0603af2c_failure
    Scenario: memory_proposal_list rejects invalid or conflicting requests
      Given required arguments no required arguments and declared defaults {"cursor":null,"limit":50,"status":null}
      And malformed, missing, out-of-scope, unavailable, replayed, or conflicting inputs
      When the client invokes memory_proposal_list
      Then one of the specified failures is validation_error: schema/domain validation; forbidden: required role or namespace grant absent; not_found/conflict/idempotency_conflict where defined; unexpected internal failures are transport-redacted
      And failed pre-publication calls do not apply none

    @surface-106498966aa2
    Scenario: memory_proposal_asset_get succeeds with its declared contract
      Given required arguments proposal_id, asset_id
      And optional arguments file_path, limit, offset
      And declared defaults no declared defaults
      And policy scope proposer role plus operation-specific visible/read/write namespace checks
      When an authorized client invokes memory_proposal_asset_get
      Then the response exposes status, data, warnings[], next_tools[], repo_revision, index_revision, index_stale, operation_id, proposal_id, asset_id, file_path, offset, returned_bytes, total_bytes, truncated, next_offset, content_sha256, content
      And side effects are none
      And idempotency is not_applicable
      And pagination or range behavior is bounded byte offset/limit; response reports total/returned/truncated and next_offset

    @surface-106498966aa2_role_reader
    Scenario: memory_proposal_asset_get as reader
      Given the canonical reader profile
      When that principal discovers or invokes memory_proposal_asset_get
      Then the outcome is hidden_and_forbidden

    @surface-106498966aa2_role_proposer
    Scenario: memory_proposal_asset_get as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes memory_proposal_asset_get
      Then the outcome is discover_and_call

    @surface-106498966aa2_role_curator
    Scenario: memory_proposal_asset_get as curator
      Given the canonical curator profile
      When that principal discovers or invokes memory_proposal_asset_get
      Then the outcome is discover_and_call

    @surface-106498966aa2_role_admin
    Scenario: memory_proposal_asset_get as admin
      Given the canonical admin profile
      When that principal discovers or invokes memory_proposal_asset_get
      Then the outcome is discover_and_call

    @surface-106498966aa2_failure
    Scenario: memory_proposal_asset_get rejects invalid or conflicting requests
      Given required arguments proposal_id, asset_id and declared defaults no declared defaults
      And malformed, missing, out-of-scope, unavailable, replayed, or conflicting inputs
      When the client invokes memory_proposal_asset_get
      Then one of the specified failures is validation_error: schema/domain validation; forbidden: required role or namespace grant absent; not_found/conflict/idempotency_conflict where defined; unexpected internal failures are transport-redacted
      And failed pre-publication calls do not apply none

    @surface-0b4811f722bf
    Scenario: memory_proposal_rebase succeeds with its declared contract
      Given required arguments proposal_id, expected_revision, idempotency_key
      And optional arguments no optional arguments
      And declared defaults no declared defaults
      And policy scope proposer or curator role plus operation-specific visible/read/write namespace checks
      When an authorized client invokes memory_proposal_rebase
      Then the response exposes status, data, warnings[], next_tools[], repo_revision, index_revision, index_stale, operation_id, proposal_id, status, base_revision, conflicts[]
      And side effects are creates or updates durable proposal/review state without publishing canonical Git unless apply is called
      And idempotency is required key with replay protection
      And pagination or range behavior is none

    @surface-0b4811f722bf_role_reader
    Scenario: memory_proposal_rebase as reader
      Given the canonical reader profile
      When that principal discovers or invokes memory_proposal_rebase
      Then the outcome is hidden_and_forbidden

    @surface-0b4811f722bf_role_proposer
    Scenario: memory_proposal_rebase as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes memory_proposal_rebase
      Then the outcome is discover_and_call

    @surface-0b4811f722bf_role_curator
    Scenario: memory_proposal_rebase as curator
      Given the canonical curator profile
      When that principal discovers or invokes memory_proposal_rebase
      Then the outcome is discover_and_call

    @surface-0b4811f722bf_role_admin
    Scenario: memory_proposal_rebase as admin
      Given the canonical admin profile
      When that principal discovers or invokes memory_proposal_rebase
      Then the outcome is discover_and_call

    @surface-0b4811f722bf_failure
    Scenario: memory_proposal_rebase rejects invalid or conflicting requests
      Given required arguments proposal_id, expected_revision, idempotency_key and declared defaults no declared defaults
      And malformed, missing, out-of-scope, unavailable, replayed, or conflicting inputs
      When the client invokes memory_proposal_rebase
      Then one of the specified failures is validation_error: schema/domain validation; forbidden: required role or namespace grant absent; not_found/conflict/idempotency_conflict where defined; unexpected internal failures are transport-redacted
      And failed pre-publication calls do not apply creates or updates durable proposal/review state without publishing canonical Git unless apply is called

    @surface-aa8ecc31f343
    Scenario: memory_proposal_revise succeeds with its declared contract
      Given required arguments proposal_id, selected_change_indexes, expected_revision
      And optional arguments intent, rationale
      And declared defaults no declared defaults
      And policy scope curator role plus operation-specific visible/read/write namespace checks
      When an authorized client invokes memory_proposal_revise
      Then the response exposes status, data, warnings[], next_tools[], repo_revision, index_revision, index_stale, operation_id, proposal_id, status, selected_change_indexes[], preview
      And side effects are creates or updates durable proposal/review state without publishing canonical Git unless apply is called
      And idempotency is proposal lifecycle constraints prevent invalid repeats
      And pagination or range behavior is none

    @surface-aa8ecc31f343_role_reader
    Scenario: memory_proposal_revise as reader
      Given the canonical reader profile
      When that principal discovers or invokes memory_proposal_revise
      Then the outcome is hidden_and_forbidden

    @surface-aa8ecc31f343_role_proposer
    Scenario: memory_proposal_revise as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes memory_proposal_revise
      Then the outcome is hidden_and_forbidden

    @surface-aa8ecc31f343_role_curator
    Scenario: memory_proposal_revise as curator
      Given the canonical curator profile
      When that principal discovers or invokes memory_proposal_revise
      Then the outcome is discover_and_call

    @surface-aa8ecc31f343_role_admin
    Scenario: memory_proposal_revise as admin
      Given the canonical admin profile
      When that principal discovers or invokes memory_proposal_revise
      Then the outcome is discover_and_call

    @surface-aa8ecc31f343_failure
    Scenario: memory_proposal_revise rejects invalid or conflicting requests
      Given required arguments proposal_id, selected_change_indexes, expected_revision and declared defaults no declared defaults
      And malformed, missing, out-of-scope, unavailable, replayed, or conflicting inputs
      When the client invokes memory_proposal_revise
      Then one of the specified failures is validation_error: schema/domain validation; forbidden: required role or namespace grant absent; not_found/conflict/idempotency_conflict where defined; unexpected internal failures are transport-redacted
      And failed pre-publication calls do not apply creates or updates durable proposal/review state without publishing canonical Git unless apply is called

    @surface-6101c4d10fb3
    Scenario: memory_proposal_review succeeds with its declared contract
      Given required arguments proposal_id, decision
      And optional arguments comment, idempotency_key
      And declared defaults {"comment":null,"idempotency_key":null}
      And policy scope curator role plus operation-specific visible/read/write namespace checks
      When an authorized client invokes memory_proposal_review
      Then the response exposes status, data, warnings[], next_tools[], repo_revision, index_revision, index_stale, operation_id, proposal_id, status, reviewed_by, review_comment
      And side effects are creates or updates durable proposal/review state without publishing canonical Git unless apply is called
      And idempotency is proposal lifecycle constraints prevent invalid repeats
      And pagination or range behavior is none

    @surface-6101c4d10fb3_role_reader
    Scenario: memory_proposal_review as reader
      Given the canonical reader profile
      When that principal discovers or invokes memory_proposal_review
      Then the outcome is hidden_and_forbidden

    @surface-6101c4d10fb3_role_proposer
    Scenario: memory_proposal_review as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes memory_proposal_review
      Then the outcome is hidden_and_forbidden

    @surface-6101c4d10fb3_role_curator
    Scenario: memory_proposal_review as curator
      Given the canonical curator profile
      When that principal discovers or invokes memory_proposal_review
      Then the outcome is discover_and_call

    @surface-6101c4d10fb3_role_admin
    Scenario: memory_proposal_review as admin
      Given the canonical admin profile
      When that principal discovers or invokes memory_proposal_review
      Then the outcome is discover_and_call

    @surface-6101c4d10fb3_failure
    Scenario: memory_proposal_review rejects invalid or conflicting requests
      Given required arguments proposal_id, decision and declared defaults {"comment":null,"idempotency_key":null}
      And malformed, missing, out-of-scope, unavailable, replayed, or conflicting inputs
      When the client invokes memory_proposal_review
      Then one of the specified failures is validation_error: schema/domain validation; forbidden: required role or namespace grant absent; not_found/conflict/idempotency_conflict where defined; unexpected internal failures are transport-redacted
      And failed pre-publication calls do not apply creates or updates durable proposal/review state without publishing canonical Git unless apply is called

    @surface-4e7107b7301d
    Scenario: memory_proposal_apply succeeds with its declared contract
      Given required arguments proposal_id, expected_revision, idempotency_key
      And optional arguments no optional arguments
      And declared defaults no declared defaults
      And policy scope curator role plus operation-specific visible/read/write namespace checks
      When an authorized client invokes memory_proposal_apply
      Then the response exposes status, data, warnings[], next_tools[], repo_revision, index_revision, index_stale, operation_id, proposal_id, status, operation_id, repo_revision, changed_paths[]
      And side effects are publishes a Git/control operation and updates derived state after commit
      And idempotency is required key; exact replay returns prior result; changed payload conflicts
      And pagination or range behavior is none

    @surface-4e7107b7301d_role_reader
    Scenario: memory_proposal_apply as reader
      Given the canonical reader profile
      When that principal discovers or invokes memory_proposal_apply
      Then the outcome is hidden_and_forbidden

    @surface-4e7107b7301d_role_proposer
    Scenario: memory_proposal_apply as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes memory_proposal_apply
      Then the outcome is hidden_and_forbidden

    @surface-4e7107b7301d_role_curator
    Scenario: memory_proposal_apply as curator
      Given the canonical curator profile
      When that principal discovers or invokes memory_proposal_apply
      Then the outcome is discover_and_call

    @surface-4e7107b7301d_role_admin
    Scenario: memory_proposal_apply as admin
      Given the canonical admin profile
      When that principal discovers or invokes memory_proposal_apply
      Then the outcome is discover_and_call

    @surface-4e7107b7301d_failure
    Scenario: memory_proposal_apply rejects invalid or conflicting requests
      Given required arguments proposal_id, expected_revision, idempotency_key and declared defaults no declared defaults
      And malformed, missing, out-of-scope, unavailable, replayed, or conflicting inputs
      When the client invokes memory_proposal_apply
      Then one of the specified failures is validation_error: schema/domain validation; forbidden: required role or namespace grant absent; not_found/conflict/idempotency_conflict where defined; unexpected internal failures are transport-redacted
      And failed pre-publication calls do not apply publishes a Git/control operation and updates derived state after commit

  Rule: operation workflow

    @surface-257627a91347
    Scenario: memory_operation_get succeeds with its declared contract
      Given required arguments no required arguments
      And optional arguments idempotency_key, operation_id
      And declared defaults no declared defaults
      And policy scope proposer or curator role plus operation-specific visible/read/write namespace checks
      When an authorized client invokes memory_operation_get
      Then the response exposes status, data, warnings[], next_tools[], repo_revision, index_revision, index_stale, operation_id, operation_id, state, safe_to_retry, result, error
      And side effects are none
      And idempotency is not_applicable
      And pagination or range behavior is none

    @surface-257627a91347_role_reader
    Scenario: memory_operation_get as reader
      Given the canonical reader profile
      When that principal discovers or invokes memory_operation_get
      Then the outcome is hidden_and_forbidden

    @surface-257627a91347_role_proposer
    Scenario: memory_operation_get as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes memory_operation_get
      Then the outcome is discover_and_call

    @surface-257627a91347_role_curator
    Scenario: memory_operation_get as curator
      Given the canonical curator profile
      When that principal discovers or invokes memory_operation_get
      Then the outcome is discover_and_call

    @surface-257627a91347_role_admin
    Scenario: memory_operation_get as admin
      Given the canonical admin profile
      When that principal discovers or invokes memory_operation_get
      Then the outcome is discover_and_call

    @surface-257627a91347_failure
    Scenario: memory_operation_get rejects invalid or conflicting requests
      Given required arguments no required arguments and declared defaults no declared defaults
      And malformed, missing, out-of-scope, unavailable, replayed, or conflicting inputs
      When the client invokes memory_operation_get
      Then one of the specified failures is validation_error: schema/domain validation; forbidden: required role or namespace grant absent; not_found/conflict/idempotency_conflict where defined; unexpected internal failures are transport-redacted
      And failed pre-publication calls do not apply none
