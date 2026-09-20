Feature: mcp/answer routing and execute

  Each operation has a success contract, four canonical role outcomes, and a failure contract.
  Stable row tags link behavior to validation data without exposing implementation details.

  Rule: answer workflow

    @surface-8aaf8ef92f9f
    Scenario: memory_answer succeeds with its declared contract
      Given required arguments question
      And optional arguments answer_mode
      And declared defaults {"answer_mode":"summary"}
      And policy scope reader role plus operation-specific visible/read/write namespace checks
      When an authorized client invokes memory_answer
      Then the response exposes status, data, warnings[], next_tools[], repo_revision, index_revision, index_stale, operation_id, answer, citations[], mode, model_attempts[], warnings[]
      And side effects are none
      And idempotency is not_applicable
      And pagination or range behavior is none

    @surface-8aaf8ef92f9f_role_reader
    Scenario: memory_answer as reader
      Given the canonical reader profile
      When that principal discovers or invokes memory_answer
      Then the outcome is discover_and_call

    @surface-8aaf8ef92f9f_role_proposer
    Scenario: memory_answer as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes memory_answer
      Then the outcome is discover_and_call

    @surface-8aaf8ef92f9f_role_curator
    Scenario: memory_answer as curator
      Given the canonical curator profile
      When that principal discovers or invokes memory_answer
      Then the outcome is discover_and_call

    @surface-8aaf8ef92f9f_role_admin
    Scenario: memory_answer as admin
      Given the canonical admin profile
      When that principal discovers or invokes memory_answer
      Then the outcome is discover_and_call

    @surface-8aaf8ef92f9f_failure
    Scenario: memory_answer rejects invalid or conflicting requests
      Given required arguments question and declared defaults {"answer_mode":"summary"}
      And malformed, missing, out-of-scope, unavailable, replayed, or conflicting inputs
      When the client invokes memory_answer
      Then one of the specified failures is validation_error: schema/domain validation; forbidden: required role or namespace grant absent; not_found/conflict/idempotency_conflict where defined; unexpected internal failures are transport-redacted
      And failed pre-publication calls do not apply none

  Rule: route workflow

    @surface-6d832731060b
    Scenario: memory_route succeeds with its declared contract
      Given required arguments request
      And optional arguments execute
      And declared defaults {"execute":true}
      And policy scope reader role plus operation-specific visible/read/write namespace checks
      When an authorized client invokes memory_route
      Then the response exposes status, data, warnings[], next_tools[], repo_revision, index_revision, index_stale, operation_id, action, tool, args, projection, result
      And side effects are none
      And idempotency is not_applicable
      And pagination or range behavior is none

    @surface-6d832731060b_role_reader
    Scenario: memory_route as reader
      Given the canonical reader profile
      When that principal discovers or invokes memory_route
      Then the outcome is discover_and_call

    @surface-6d832731060b_role_proposer
    Scenario: memory_route as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes memory_route
      Then the outcome is discover_and_call

    @surface-6d832731060b_role_curator
    Scenario: memory_route as curator
      Given the canonical curator profile
      When that principal discovers or invokes memory_route
      Then the outcome is discover_and_call

    @surface-6d832731060b_role_admin
    Scenario: memory_route as admin
      Given the canonical admin profile
      When that principal discovers or invokes memory_route
      Then the outcome is discover_and_call

    @surface-6d832731060b_failure
    Scenario: memory_route rejects invalid or conflicting requests
      Given required arguments request and declared defaults {"execute":true}
      And malformed, missing, out-of-scope, unavailable, replayed, or conflicting inputs
      When the client invokes memory_route
      Then one of the specified failures is validation_error: schema/domain validation; forbidden: required role or namespace grant absent; not_found/conflict/idempotency_conflict where defined; unexpected internal failures are transport-redacted
      And failed pre-publication calls do not apply none

  Rule: execute workflow

    @surface-21962f513767
    Scenario: memory_execute succeeds with its declared contract
      Given required arguments no required arguments
      And optional arguments operations, plan, returns, stop_on_error
      And declared defaults {"returns":[],"stop_on_error":true}
      And policy scope reader role plus operation-specific visible/read/write namespace checks
      When an authorized client invokes memory_execute
      Then the response exposes status, data, warnings[], next_tools[], repo_revision, index_revision, index_stale, operation_id, steps[], steps[].operation, steps[].status, returns, repo_revision, operation_id
      And side effects are none
      And idempotency is not_applicable
      And pagination or range behavior is none

    @surface-21962f513767_role_reader
    Scenario: memory_execute as reader
      Given the canonical reader profile
      When that principal discovers or invokes memory_execute
      Then the outcome is discover_and_call

    @surface-21962f513767_role_proposer
    Scenario: memory_execute as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes memory_execute
      Then the outcome is discover_and_call

    @surface-21962f513767_role_curator
    Scenario: memory_execute as curator
      Given the canonical curator profile
      When that principal discovers or invokes memory_execute
      Then the outcome is discover_and_call

    @surface-21962f513767_role_admin
    Scenario: memory_execute as admin
      Given the canonical admin profile
      When that principal discovers or invokes memory_execute
      Then the outcome is discover_and_call

    @surface-21962f513767_failure
    Scenario: memory_execute rejects invalid or conflicting requests
      Given required arguments no required arguments and declared defaults {"returns":[],"stop_on_error":true}
      And malformed, missing, out-of-scope, unavailable, replayed, or conflicting inputs
      When the client invokes memory_execute
      Then one of the specified failures is validation_error: schema/domain validation; forbidden: required role or namespace grant absent; not_found/conflict/idempotency_conflict where defined; unexpected internal failures are transport-redacted
      And failed pre-publication calls do not apply none
