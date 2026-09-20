Feature: mcp/answer routing and execute

  Each operation has a success contract, four canonical role outcomes, and a failure contract.

  Rule: answer workflow

    @surface-8aaf8ef92f9f @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_answer preserves the Python success contract
      Given the Python request contract [object Object]
      When an authorized client invokes memory_answer
      Then the response matches the captured Python behavior is exercised

    @surface-8aaf8ef92f9f_role_reader @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_answer as reader
      Given the canonical reader profile
      When that principal discovers or invokes memory_answer
      Then the Python outcome is discover_and_call

    @surface-8aaf8ef92f9f_role_proposer @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_answer as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes memory_answer
      Then the Python outcome is discover_and_call

    @surface-8aaf8ef92f9f_role_curator @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_answer as curator
      Given the canonical curator profile
      When that principal discovers or invokes memory_answer
      Then the Python outcome is discover_and_call

    @surface-8aaf8ef92f9f_role_admin @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_answer as admin
      Given the canonical admin profile
      When that principal discovers or invokes memory_answer
      Then the Python outcome is discover_and_call

    @surface-8aaf8ef92f9f_failure @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_answer preserves Python validation and failure behavior
      Given malformed, missing, out-of-scope, or unavailable inputs
      When the client invokes memory_answer
      Then role/policy validation, typed argument validation, domain error envelope, unexpected transport redaction

  Rule: route workflow

    @surface-6d832731060b @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_route preserves the Python success contract
      Given the Python request contract [object Object]
      When an authorized client invokes memory_route
      Then the response matches the captured Python behavior is exercised

    @surface-6d832731060b_role_reader @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_route as reader
      Given the canonical reader profile
      When that principal discovers or invokes memory_route
      Then the Python outcome is discover_and_call

    @surface-6d832731060b_role_proposer @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_route as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes memory_route
      Then the Python outcome is discover_and_call

    @surface-6d832731060b_role_curator @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_route as curator
      Given the canonical curator profile
      When that principal discovers or invokes memory_route
      Then the Python outcome is discover_and_call

    @surface-6d832731060b_role_admin @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_route as admin
      Given the canonical admin profile
      When that principal discovers or invokes memory_route
      Then the Python outcome is discover_and_call

    @surface-6d832731060b_failure @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_route preserves Python validation and failure behavior
      Given malformed, missing, out-of-scope, or unavailable inputs
      When the client invokes memory_route
      Then role/policy validation, typed argument validation, domain error envelope, unexpected transport redaction

  Rule: execute workflow

    @surface-21962f513767 @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch @go_TestExecuteEndpointProtocol @go_TestMCPContractRoutesExecuteRealPlans
    Scenario: memory_execute preserves the Python success contract
      Given the Python request contract [object Object]
      When an authorized client invokes memory_execute
      Then the response matches the captured Python behavior is exercised

    @surface-21962f513767_role_reader @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch @go_TestExecuteEndpointProtocol @go_TestMCPContractRoutesExecuteRealPlans
    Scenario: memory_execute as reader
      Given the canonical reader profile
      When that principal discovers or invokes memory_execute
      Then the Python outcome is discover_and_call

    @surface-21962f513767_role_proposer @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch @go_TestExecuteEndpointProtocol @go_TestMCPContractRoutesExecuteRealPlans
    Scenario: memory_execute as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes memory_execute
      Then the Python outcome is discover_and_call

    @surface-21962f513767_role_curator @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch @go_TestExecuteEndpointProtocol @go_TestMCPContractRoutesExecuteRealPlans
    Scenario: memory_execute as curator
      Given the canonical curator profile
      When that principal discovers or invokes memory_execute
      Then the Python outcome is discover_and_call

    @surface-21962f513767_role_admin @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch @go_TestExecuteEndpointProtocol @go_TestMCPContractRoutesExecuteRealPlans
    Scenario: memory_execute as admin
      Given the canonical admin profile
      When that principal discovers or invokes memory_execute
      Then the Python outcome is discover_and_call

    @surface-21962f513767_failure @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch @go_TestExecuteEndpointProtocol @go_TestMCPContractRoutesExecuteRealPlans
    Scenario: memory_execute preserves Python validation and failure behavior
      Given malformed, missing, out-of-scope, or unavailable inputs
      When the client invokes memory_execute
      Then role/policy validation, typed argument validation, domain error envelope, unexpected transport redaction
