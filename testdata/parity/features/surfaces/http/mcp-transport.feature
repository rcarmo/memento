Feature: http/mcp transport

  Each operation has a success contract, four canonical role outcomes, and a failure contract.
  Stable row tags link behavior to validation data without exposing implementation details.

  Rule: http endpoint workflow

    @surface-cdbe1d5b6a22
    Scenario: POST /mcp succeeds with its declared contract
      Given required arguments no required arguments
      And optional arguments no optional arguments
      And declared defaults no declared defaults
      And policy scope bearer principal
      When an authorized client invokes POST /mcp
      Then the response exposes HTTP 200/202 or SSE according to JSON-RPC request kind, Mcp-Session-Id after initialize, MCP JSON-RPC response
      And side effects are queues or persists the endpoint-specific staging, refresh, access, or session transition
      And idempotency is not_applicable
      And pagination or range behavior is none

    @surface-cdbe1d5b6a22_role_reader
    Scenario: POST /mcp as reader
      Given the canonical reader profile
      When that principal uses POST /mcp
      Then the outcome is bearer-authenticated MCP transport; called tool applies the profile role and namespace policy

    @surface-cdbe1d5b6a22_role_proposer
    Scenario: POST /mcp as proposer
      Given the canonical proposer profile
      When that principal uses POST /mcp
      Then the outcome is bearer-authenticated MCP transport; called tool applies the profile role and namespace policy

    @surface-cdbe1d5b6a22_role_curator
    Scenario: POST /mcp as curator
      Given the canonical curator profile
      When that principal uses POST /mcp
      Then the outcome is bearer-authenticated MCP transport; called tool applies the profile role and namespace policy

    @surface-cdbe1d5b6a22_role_admin
    Scenario: POST /mcp as admin
      Given the canonical admin profile
      When that principal uses POST /mcp
      Then the outcome is bearer-authenticated MCP transport; called tool applies the profile role and namespace policy

    @surface-cdbe1d5b6a22_failure
    Scenario: POST /mcp rejects invalid or conflicting requests
      Given required arguments no required arguments and declared defaults no declared defaults
      And malformed, missing, out-of-scope, unavailable, replayed, or conflicting inputs
      When the client invokes POST /mcp
      Then one of the specified failures is 400 validation, 401 auth, 403 policy where applicable, 404 unknown, 405 method, 413/415 upload bounds, 503 unavailable
      And failed pre-publication calls do not apply queues or persists the endpoint-specific staging, refresh, access, or session transition

    @surface-9bb9207af645
    Scenario: GET /mcp succeeds with its declared contract
      Given required arguments no required arguments
      And optional arguments no optional arguments
      And declared defaults no declared defaults
      And policy scope bound session
      When an authorized client invokes GET /mcp
      Then the response exposes HTTP 200 text/event-stream, session-bound SSE events
      And side effects are none
      And idempotency is not_applicable
      And pagination or range behavior is none

    @surface-9bb9207af645_role_reader
    Scenario: GET /mcp as reader
      Given the canonical reader profile
      When that principal uses GET /mcp
      Then the outcome is bearer-authenticated MCP transport; called tool applies the profile role and namespace policy

    @surface-9bb9207af645_role_proposer
    Scenario: GET /mcp as proposer
      Given the canonical proposer profile
      When that principal uses GET /mcp
      Then the outcome is bearer-authenticated MCP transport; called tool applies the profile role and namespace policy

    @surface-9bb9207af645_role_curator
    Scenario: GET /mcp as curator
      Given the canonical curator profile
      When that principal uses GET /mcp
      Then the outcome is bearer-authenticated MCP transport; called tool applies the profile role and namespace policy

    @surface-9bb9207af645_role_admin
    Scenario: GET /mcp as admin
      Given the canonical admin profile
      When that principal uses GET /mcp
      Then the outcome is bearer-authenticated MCP transport; called tool applies the profile role and namespace policy

    @surface-9bb9207af645_failure
    Scenario: GET /mcp rejects invalid or conflicting requests
      Given required arguments no required arguments and declared defaults no declared defaults
      And malformed, missing, out-of-scope, unavailable, replayed, or conflicting inputs
      When the client invokes GET /mcp
      Then one of the specified failures is 400 validation, 401 auth, 403 policy where applicable, 404 unknown, 405 method, 413/415 upload bounds, 503 unavailable
      And failed pre-publication calls do not apply none

    @surface-8297b7f3e421
    Scenario: DELETE /mcp succeeds with its declared contract
      Given required arguments no required arguments
      And optional arguments no optional arguments
      And declared defaults no declared defaults
      And policy scope bound session
      When an authorized client invokes DELETE /mcp
      Then the response exposes HTTP 200, session removed
      And side effects are none
      And idempotency is not_applicable
      And pagination or range behavior is none

    @surface-8297b7f3e421_role_reader
    Scenario: DELETE /mcp as reader
      Given the canonical reader profile
      When that principal uses DELETE /mcp
      Then the outcome is bearer-authenticated MCP transport; called tool applies the profile role and namespace policy

    @surface-8297b7f3e421_role_proposer
    Scenario: DELETE /mcp as proposer
      Given the canonical proposer profile
      When that principal uses DELETE /mcp
      Then the outcome is bearer-authenticated MCP transport; called tool applies the profile role and namespace policy

    @surface-8297b7f3e421_role_curator
    Scenario: DELETE /mcp as curator
      Given the canonical curator profile
      When that principal uses DELETE /mcp
      Then the outcome is bearer-authenticated MCP transport; called tool applies the profile role and namespace policy

    @surface-8297b7f3e421_role_admin
    Scenario: DELETE /mcp as admin
      Given the canonical admin profile
      When that principal uses DELETE /mcp
      Then the outcome is bearer-authenticated MCP transport; called tool applies the profile role and namespace policy

    @surface-8297b7f3e421_failure
    Scenario: DELETE /mcp rejects invalid or conflicting requests
      Given required arguments no required arguments and declared defaults no declared defaults
      And malformed, missing, out-of-scope, unavailable, replayed, or conflicting inputs
      When the client invokes DELETE /mcp
      Then one of the specified failures is 400 validation, 401 auth, 403 policy where applicable, 404 unknown, 405 method, 413/415 upload bounds, 503 unavailable
      And failed pre-publication calls do not apply none
