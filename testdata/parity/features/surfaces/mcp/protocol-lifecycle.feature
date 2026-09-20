Feature: mcp/protocol lifecycle

  Each operation has a success contract, four canonical role outcomes, and a failure contract.

  Rule: mcp protocol workflow

    @surface-b762c46cbf97 @go_TestMCPContractResourcesAndProtocol
    Scenario: initialize preserves the Python success contract
      Given the Python request contract JSON-RPC 2.0 MCP request
      When an authorized client invokes initialize
      Then the response matches source-compatible result/error and ordered discovery

    @surface-b762c46cbf97_role_reader @go_TestMCPContractResourcesAndProtocol
    Scenario: initialize as reader
      Given the canonical reader profile
      When that principal discovers or invokes initialize
      Then the Python outcome is available_after_successful_initialize_and_authentication

    @surface-b762c46cbf97_role_proposer @go_TestMCPContractResourcesAndProtocol
    Scenario: initialize as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes initialize
      Then the Python outcome is available_after_successful_initialize_and_authentication

    @surface-b762c46cbf97_role_curator @go_TestMCPContractResourcesAndProtocol
    Scenario: initialize as curator
      Given the canonical curator profile
      When that principal discovers or invokes initialize
      Then the Python outcome is available_after_successful_initialize_and_authentication

    @surface-b762c46cbf97_role_admin @go_TestMCPContractResourcesAndProtocol
    Scenario: initialize as admin
      Given the canonical admin profile
      When that principal discovers or invokes initialize
      Then the Python outcome is available_after_successful_initialize_and_authentication

    @surface-b762c46cbf97_failure @go_TestMCPContractResourcesAndProtocol
    Scenario: initialize preserves Python validation and failure behavior
      Given malformed, missing, out-of-scope, or unavailable inputs
      When the client invokes initialize
      Then -32600/-32601/-32602/-32603 as observed

    @surface-38abc25b376e @go_TestMCPContractRuntimeSurfaces
    Scenario: tools/list preserves the Python success contract
      Given the Python request contract JSON-RPC 2.0 MCP request
      When an authorized client invokes tools/list
      Then the response matches source-compatible result/error and ordered discovery

    @surface-38abc25b376e_role_reader @go_TestMCPContractRuntimeSurfaces
    Scenario: tools/list as reader
      Given the canonical reader profile
      When that principal discovers or invokes tools/list
      Then the Python outcome is available_after_successful_initialize_and_authentication

    @surface-38abc25b376e_role_proposer @go_TestMCPContractRuntimeSurfaces
    Scenario: tools/list as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes tools/list
      Then the Python outcome is available_after_successful_initialize_and_authentication

    @surface-38abc25b376e_role_curator @go_TestMCPContractRuntimeSurfaces
    Scenario: tools/list as curator
      Given the canonical curator profile
      When that principal discovers or invokes tools/list
      Then the Python outcome is available_after_successful_initialize_and_authentication

    @surface-38abc25b376e_role_admin @go_TestMCPContractRuntimeSurfaces
    Scenario: tools/list as admin
      Given the canonical admin profile
      When that principal discovers or invokes tools/list
      Then the Python outcome is available_after_successful_initialize_and_authentication

    @surface-38abc25b376e_failure @go_TestMCPContractRuntimeSurfaces
    Scenario: tools/list preserves Python validation and failure behavior
      Given malformed, missing, out-of-scope, or unavailable inputs
      When the client invokes tools/list
      Then -32600/-32601/-32602/-32603 as observed

    @surface-86e3a96daa92 @go_TestMCPContractEveryToolSchemaDefaultsAndValidation
    Scenario: tools/call preserves the Python success contract
      Given the Python request contract JSON-RPC 2.0 MCP request
      When an authorized client invokes tools/call
      Then the response matches source-compatible result/error and ordered discovery

    @surface-86e3a96daa92_role_reader @go_TestMCPContractEveryToolSchemaDefaultsAndValidation
    Scenario: tools/call as reader
      Given the canonical reader profile
      When that principal discovers or invokes tools/call
      Then the Python outcome is available_after_successful_initialize_and_authentication

    @surface-86e3a96daa92_role_proposer @go_TestMCPContractEveryToolSchemaDefaultsAndValidation
    Scenario: tools/call as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes tools/call
      Then the Python outcome is available_after_successful_initialize_and_authentication

    @surface-86e3a96daa92_role_curator @go_TestMCPContractEveryToolSchemaDefaultsAndValidation
    Scenario: tools/call as curator
      Given the canonical curator profile
      When that principal discovers or invokes tools/call
      Then the Python outcome is available_after_successful_initialize_and_authentication

    @surface-86e3a96daa92_role_admin @go_TestMCPContractEveryToolSchemaDefaultsAndValidation
    Scenario: tools/call as admin
      Given the canonical admin profile
      When that principal discovers or invokes tools/call
      Then the Python outcome is available_after_successful_initialize_and_authentication

    @surface-86e3a96daa92_failure @go_TestMCPContractEveryToolSchemaDefaultsAndValidation
    Scenario: tools/call preserves Python validation and failure behavior
      Given malformed, missing, out-of-scope, or unavailable inputs
      When the client invokes tools/call
      Then -32600/-32601/-32602/-32603 as observed

    @surface-9dc4d7adc272 @go_TestServerLoggingAndNotifications
    Scenario: logging/setLevel preserves the Python success contract
      Given the Python request contract JSON-RPC 2.0 MCP request
      When an authorized client invokes logging/setLevel
      Then the response matches source-compatible result/error and ordered discovery

    @surface-9dc4d7adc272_role_reader @go_TestServerLoggingAndNotifications
    Scenario: logging/setLevel as reader
      Given the canonical reader profile
      When that principal discovers or invokes logging/setLevel
      Then the Python outcome is available_after_successful_initialize_and_authentication

    @surface-9dc4d7adc272_role_proposer @go_TestServerLoggingAndNotifications
    Scenario: logging/setLevel as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes logging/setLevel
      Then the Python outcome is available_after_successful_initialize_and_authentication

    @surface-9dc4d7adc272_role_curator @go_TestServerLoggingAndNotifications
    Scenario: logging/setLevel as curator
      Given the canonical curator profile
      When that principal discovers or invokes logging/setLevel
      Then the Python outcome is available_after_successful_initialize_and_authentication

    @surface-9dc4d7adc272_role_admin @go_TestServerLoggingAndNotifications
    Scenario: logging/setLevel as admin
      Given the canonical admin profile
      When that principal discovers or invokes logging/setLevel
      Then the Python outcome is available_after_successful_initialize_and_authentication

    @surface-9dc4d7adc272_failure @go_TestServerLoggingAndNotifications
    Scenario: logging/setLevel preserves Python validation and failure behavior
      Given malformed, missing, out-of-scope, or unavailable inputs
      When the client invokes logging/setLevel
      Then -32600/-32601/-32602/-32603 as observed
