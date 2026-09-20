Feature: http/mcp transport

  Each operation has a success contract, four canonical role outcomes, and a failure contract.

  Rule: http endpoint workflow

    @surface-cdbe1d5b6a22 @go_TestModelsOffRuntimeHTTPHooks
    Scenario: POST /mcp preserves the Python success contract
      Given the Python request contract POST request with route-specific JSON/headers
      When an authorized client invokes POST /mcp
      Then the response matches MCP response or SSE

    @surface-cdbe1d5b6a22_role_reader @go_TestModelsOffRuntimeHTTPHooks
    Scenario: POST /mcp as reader
      Given the canonical reader profile
      When that principal discovers or invokes POST /mcp
      Then the Python outcome is route_specific_authentication_and_policy

    @surface-cdbe1d5b6a22_role_proposer @go_TestModelsOffRuntimeHTTPHooks
    Scenario: POST /mcp as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes POST /mcp
      Then the Python outcome is route_specific_authentication_and_policy

    @surface-cdbe1d5b6a22_role_curator @go_TestModelsOffRuntimeHTTPHooks
    Scenario: POST /mcp as curator
      Given the canonical curator profile
      When that principal discovers or invokes POST /mcp
      Then the Python outcome is route_specific_authentication_and_policy

    @surface-cdbe1d5b6a22_role_admin @go_TestModelsOffRuntimeHTTPHooks
    Scenario: POST /mcp as admin
      Given the canonical admin profile
      When that principal discovers or invokes POST /mcp
      Then the Python outcome is route_specific_authentication_and_policy

    @surface-cdbe1d5b6a22_failure @go_TestModelsOffRuntimeHTTPHooks
    Scenario: POST /mcp preserves Python validation and failure behavior
      Given malformed, missing, out-of-scope, or unavailable inputs
      When the client invokes POST /mcp
      Then 400 validation, 401 auth, 403 policy where applicable, 404 unknown, 405 method, 413/415 upload bounds, 503 unavailable

    @surface-9bb9207af645 @go_TestHTTPInitialiseSessionAndStateless
    Scenario: GET /mcp preserves the Python success contract
      Given the Python request contract GET request with route-specific JSON/headers
      When an authorized client invokes GET /mcp
      Then the response matches SSE event stream

    @surface-9bb9207af645_role_reader @go_TestHTTPInitialiseSessionAndStateless
    Scenario: GET /mcp as reader
      Given the canonical reader profile
      When that principal discovers or invokes GET /mcp
      Then the Python outcome is route_specific_authentication_and_policy

    @surface-9bb9207af645_role_proposer @go_TestHTTPInitialiseSessionAndStateless
    Scenario: GET /mcp as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes GET /mcp
      Then the Python outcome is route_specific_authentication_and_policy

    @surface-9bb9207af645_role_curator @go_TestHTTPInitialiseSessionAndStateless
    Scenario: GET /mcp as curator
      Given the canonical curator profile
      When that principal discovers or invokes GET /mcp
      Then the Python outcome is route_specific_authentication_and_policy

    @surface-9bb9207af645_role_admin @go_TestHTTPInitialiseSessionAndStateless
    Scenario: GET /mcp as admin
      Given the canonical admin profile
      When that principal discovers or invokes GET /mcp
      Then the Python outcome is route_specific_authentication_and_policy

    @surface-9bb9207af645_failure @go_TestHTTPInitialiseSessionAndStateless
    Scenario: GET /mcp preserves Python validation and failure behavior
      Given malformed, missing, out-of-scope, or unavailable inputs
      When the client invokes GET /mcp
      Then 400 validation, 401 auth, 403 policy where applicable, 404 unknown, 405 method, 413/415 upload bounds, 503 unavailable

    @surface-8297b7f3e421 @go_TestSessionLifecycle
    Scenario: DELETE /mcp preserves the Python success contract
      Given the Python request contract DELETE request with route-specific JSON/headers
      When an authorized client invokes DELETE /mcp
      Then the response matches session deletion

    @surface-8297b7f3e421_role_reader @go_TestSessionLifecycle
    Scenario: DELETE /mcp as reader
      Given the canonical reader profile
      When that principal discovers or invokes DELETE /mcp
      Then the Python outcome is route_specific_authentication_and_policy

    @surface-8297b7f3e421_role_proposer @go_TestSessionLifecycle
    Scenario: DELETE /mcp as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes DELETE /mcp
      Then the Python outcome is route_specific_authentication_and_policy

    @surface-8297b7f3e421_role_curator @go_TestSessionLifecycle
    Scenario: DELETE /mcp as curator
      Given the canonical curator profile
      When that principal discovers or invokes DELETE /mcp
      Then the Python outcome is route_specific_authentication_and_policy

    @surface-8297b7f3e421_role_admin @go_TestSessionLifecycle
    Scenario: DELETE /mcp as admin
      Given the canonical admin profile
      When that principal discovers or invokes DELETE /mcp
      Then the Python outcome is route_specific_authentication_and_policy

    @surface-8297b7f3e421_failure @go_TestSessionLifecycle
    Scenario: DELETE /mcp preserves Python validation and failure behavior
      Given malformed, missing, out-of-scope, or unavailable inputs
      When the client invokes DELETE /mcp
      Then 400 validation, 401 auth, 403 policy where applicable, 404 unknown, 405 method, 413/415 upload bounds, 503 unavailable
