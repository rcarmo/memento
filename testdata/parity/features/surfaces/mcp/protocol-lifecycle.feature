Feature: mcp/protocol lifecycle

  Each operation has a success contract, four canonical role outcomes, and a failure contract.
  Stable row tags link behavior to validation data without exposing implementation details.

  Rule: mcp protocol workflow

    @surface-b762c46cbf97
    Scenario: initialize succeeds with its declared contract
      Given required arguments no required arguments
      And optional arguments no optional arguments
      And declared defaults no declared defaults
      And policy scope authenticated session/principal where transport requires it
      When an authorized client invokes initialize
      Then the response exposes protocolVersion, serverInfo.name, serverInfo.version, capabilities, instructions
      And side effects are none
      And idempotency is not_applicable
      And pagination or range behavior is none

    @surface-b762c46cbf97_role_reader
    Scenario: initialize as reader
      Given the canonical reader profile
      When that principal uses initialize
      Then the outcome is available_after_successful_initialize_and_authentication

    @surface-b762c46cbf97_role_proposer
    Scenario: initialize as proposer
      Given the canonical proposer profile
      When that principal uses initialize
      Then the outcome is available_after_successful_initialize_and_authentication

    @surface-b762c46cbf97_role_curator
    Scenario: initialize as curator
      Given the canonical curator profile
      When that principal uses initialize
      Then the outcome is available_after_successful_initialize_and_authentication

    @surface-b762c46cbf97_role_admin
    Scenario: initialize as admin
      Given the canonical admin profile
      When that principal uses initialize
      Then the outcome is available_after_successful_initialize_and_authentication

    @surface-b762c46cbf97_failure
    Scenario: initialize rejects invalid or conflicting requests
      Given required arguments no required arguments and declared defaults no declared defaults
      And malformed, missing, out-of-scope, unavailable, replayed, or conflicting inputs
      When the client invokes initialize
      Then one of the specified failures is JSON-RPC protocol/invalid-params/method-not-found/internal error as observed
      And failed pre-publication calls do not apply none

    @surface-38abc25b376e
    Scenario: tools/list succeeds with its declared contract
      Given required arguments no required arguments
      And optional arguments no optional arguments
      And declared defaults no declared defaults
      And policy scope authenticated session/principal where transport requires it
      When an authorized client invokes tools/list
      Then the response exposes tools[], tools[].name, tools[].description, tools[].inputSchema, nextCursor
      And side effects are none
      And idempotency is not_applicable
      And pagination or range behavior is none

    @surface-38abc25b376e_role_reader
    Scenario: tools/list as reader
      Given the canonical reader profile
      When that principal uses tools/list
      Then the outcome is available_after_successful_initialize_and_authentication

    @surface-38abc25b376e_role_proposer
    Scenario: tools/list as proposer
      Given the canonical proposer profile
      When that principal uses tools/list
      Then the outcome is available_after_successful_initialize_and_authentication

    @surface-38abc25b376e_role_curator
    Scenario: tools/list as curator
      Given the canonical curator profile
      When that principal uses tools/list
      Then the outcome is available_after_successful_initialize_and_authentication

    @surface-38abc25b376e_role_admin
    Scenario: tools/list as admin
      Given the canonical admin profile
      When that principal uses tools/list
      Then the outcome is available_after_successful_initialize_and_authentication

    @surface-38abc25b376e_failure
    Scenario: tools/list rejects invalid or conflicting requests
      Given required arguments no required arguments and declared defaults no declared defaults
      And malformed, missing, out-of-scope, unavailable, replayed, or conflicting inputs
      When the client invokes tools/list
      Then one of the specified failures is JSON-RPC protocol/invalid-params/method-not-found/internal error as observed
      And failed pre-publication calls do not apply none

    @surface-86e3a96daa92
    Scenario: tools/call succeeds with its declared contract
      Given required arguments no required arguments
      And optional arguments no optional arguments
      And declared defaults no declared defaults
      And policy scope authenticated session/principal where transport requires it
      When an authorized client invokes tools/call
      Then the response exposes content[], structuredContent, isError
      And side effects are none
      And idempotency is not_applicable
      And pagination or range behavior is none

    @surface-86e3a96daa92_role_reader
    Scenario: tools/call as reader
      Given the canonical reader profile
      When that principal uses tools/call
      Then the outcome is available_after_successful_initialize_and_authentication

    @surface-86e3a96daa92_role_proposer
    Scenario: tools/call as proposer
      Given the canonical proposer profile
      When that principal uses tools/call
      Then the outcome is available_after_successful_initialize_and_authentication

    @surface-86e3a96daa92_role_curator
    Scenario: tools/call as curator
      Given the canonical curator profile
      When that principal uses tools/call
      Then the outcome is available_after_successful_initialize_and_authentication

    @surface-86e3a96daa92_role_admin
    Scenario: tools/call as admin
      Given the canonical admin profile
      When that principal uses tools/call
      Then the outcome is available_after_successful_initialize_and_authentication

    @surface-86e3a96daa92_failure
    Scenario: tools/call rejects invalid or conflicting requests
      Given required arguments no required arguments and declared defaults no declared defaults
      And malformed, missing, out-of-scope, unavailable, replayed, or conflicting inputs
      When the client invokes tools/call
      Then one of the specified failures is JSON-RPC protocol/invalid-params/method-not-found/internal error as observed
      And failed pre-publication calls do not apply none

    @surface-9dc4d7adc272
    Scenario: logging/setLevel succeeds with its declared contract
      Given required arguments no required arguments
      And optional arguments no optional arguments
      And declared defaults no declared defaults
      And policy scope authenticated session/principal where transport requires it
      When an authorized client invokes logging/setLevel
      Then the response exposes empty JSON-RPC result; later notifications/message
      And side effects are none
      And idempotency is not_applicable
      And pagination or range behavior is none

    @surface-9dc4d7adc272_role_reader
    Scenario: logging/setLevel as reader
      Given the canonical reader profile
      When that principal uses logging/setLevel
      Then the outcome is available_after_successful_initialize_and_authentication

    @surface-9dc4d7adc272_role_proposer
    Scenario: logging/setLevel as proposer
      Given the canonical proposer profile
      When that principal uses logging/setLevel
      Then the outcome is available_after_successful_initialize_and_authentication

    @surface-9dc4d7adc272_role_curator
    Scenario: logging/setLevel as curator
      Given the canonical curator profile
      When that principal uses logging/setLevel
      Then the outcome is available_after_successful_initialize_and_authentication

    @surface-9dc4d7adc272_role_admin
    Scenario: logging/setLevel as admin
      Given the canonical admin profile
      When that principal uses logging/setLevel
      Then the outcome is available_after_successful_initialize_and_authentication

    @surface-9dc4d7adc272_failure
    Scenario: logging/setLevel rejects invalid or conflicting requests
      Given required arguments no required arguments and declared defaults no declared defaults
      And malformed, missing, out-of-scope, unavailable, replayed, or conflicting inputs
      When the client invokes logging/setLevel
      Then one of the specified failures is JSON-RPC protocol/invalid-params/method-not-found/internal error as observed
      And failed pre-publication calls do not apply none
