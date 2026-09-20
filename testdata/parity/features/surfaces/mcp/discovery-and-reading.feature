Feature: mcp/discovery and reading

  Each operation has a success contract, four canonical role outcomes, and a failure contract.

  Rule: help workflow

    @surface-c6906a04e6cb @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch @go_TestRegisteredModelsOffStatusHTTP
    Scenario: memory_help preserves the Python success contract
      Given the Python request contract [object Object]
      When an authorized client invokes memory_help
      Then the response matches the captured Python behavior is exercised

    @surface-c6906a04e6cb_role_reader @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch @go_TestRegisteredModelsOffStatusHTTP
    Scenario: memory_help as reader
      Given the canonical reader profile
      When that principal discovers or invokes memory_help
      Then the Python outcome is discover_and_call

    @surface-c6906a04e6cb_role_proposer @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch @go_TestRegisteredModelsOffStatusHTTP
    Scenario: memory_help as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes memory_help
      Then the Python outcome is discover_and_call

    @surface-c6906a04e6cb_role_curator @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch @go_TestRegisteredModelsOffStatusHTTP
    Scenario: memory_help as curator
      Given the canonical curator profile
      When that principal discovers or invokes memory_help
      Then the Python outcome is discover_and_call

    @surface-c6906a04e6cb_role_admin @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch @go_TestRegisteredModelsOffStatusHTTP
    Scenario: memory_help as admin
      Given the canonical admin profile
      When that principal discovers or invokes memory_help
      Then the Python outcome is discover_and_call

    @surface-c6906a04e6cb_failure @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch @go_TestRegisteredModelsOffStatusHTTP
    Scenario: memory_help preserves Python validation and failure behavior
      Given malformed, missing, out-of-scope, or unavailable inputs
      When the client invokes memory_help
      Then role/policy validation, typed argument validation, domain error envelope, unexpected transport redaction

  Rule: status workflow

    @surface-213a4c43ed32 @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch @go_TestRegisteredModelsOffStatusHTTP
    Scenario: memory_status preserves the Python success contract
      Given the Python request contract [object Object]
      When an authorized client invokes memory_status
      Then the response matches the captured Python behavior is exercised

    @surface-213a4c43ed32_role_reader @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch @go_TestRegisteredModelsOffStatusHTTP
    Scenario: memory_status as reader
      Given the canonical reader profile
      When that principal discovers or invokes memory_status
      Then the Python outcome is discover_and_call

    @surface-213a4c43ed32_role_proposer @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch @go_TestRegisteredModelsOffStatusHTTP
    Scenario: memory_status as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes memory_status
      Then the Python outcome is discover_and_call

    @surface-213a4c43ed32_role_curator @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch @go_TestRegisteredModelsOffStatusHTTP
    Scenario: memory_status as curator
      Given the canonical curator profile
      When that principal discovers or invokes memory_status
      Then the Python outcome is discover_and_call

    @surface-213a4c43ed32_role_admin @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch @go_TestRegisteredModelsOffStatusHTTP
    Scenario: memory_status as admin
      Given the canonical admin profile
      When that principal discovers or invokes memory_status
      Then the Python outcome is discover_and_call

    @surface-213a4c43ed32_failure @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch @go_TestRegisteredModelsOffStatusHTTP
    Scenario: memory_status preserves Python validation and failure behavior
      Given malformed, missing, out-of-scope, or unavailable inputs
      When the client invokes memory_status
      Then role/policy validation, typed argument validation, domain error envelope, unexpected transport redaction

  Rule: search workflow

    @surface-6c54892a4a07 @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch @go_TestReadToolReference
    Scenario: memory_search preserves the Python success contract
      Given the Python request contract [object Object]
      When an authorized client invokes memory_search
      Then the response matches the captured Python behavior is exercised

    @surface-6c54892a4a07_role_reader @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch @go_TestReadToolReference
    Scenario: memory_search as reader
      Given the canonical reader profile
      When that principal discovers or invokes memory_search
      Then the Python outcome is discover_and_call

    @surface-6c54892a4a07_role_proposer @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch @go_TestReadToolReference
    Scenario: memory_search as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes memory_search
      Then the Python outcome is discover_and_call

    @surface-6c54892a4a07_role_curator @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch @go_TestReadToolReference
    Scenario: memory_search as curator
      Given the canonical curator profile
      When that principal discovers or invokes memory_search
      Then the Python outcome is discover_and_call

    @surface-6c54892a4a07_role_admin @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch @go_TestReadToolReference
    Scenario: memory_search as admin
      Given the canonical admin profile
      When that principal discovers or invokes memory_search
      Then the Python outcome is discover_and_call

    @surface-6c54892a4a07_failure @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch @go_TestReadToolReference
    Scenario: memory_search preserves Python validation and failure behavior
      Given malformed, missing, out-of-scope, or unavailable inputs
      When the client invokes memory_search
      Then role/policy validation, typed argument validation, domain error envelope, unexpected transport redaction

  Rule: read workflow

    @surface-4447ab44f238 @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch @go_TestReadToolReference
    Scenario: memory_read preserves the Python success contract
      Given the Python request contract [object Object]
      When an authorized client invokes memory_read
      Then the response matches the captured Python behavior is exercised

    @surface-4447ab44f238_role_reader @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch @go_TestReadToolReference
    Scenario: memory_read as reader
      Given the canonical reader profile
      When that principal discovers or invokes memory_read
      Then the Python outcome is discover_and_call

    @surface-4447ab44f238_role_proposer @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch @go_TestReadToolReference
    Scenario: memory_read as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes memory_read
      Then the Python outcome is discover_and_call

    @surface-4447ab44f238_role_curator @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch @go_TestReadToolReference
    Scenario: memory_read as curator
      Given the canonical curator profile
      When that principal discovers or invokes memory_read
      Then the Python outcome is discover_and_call

    @surface-4447ab44f238_role_admin @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch @go_TestReadToolReference
    Scenario: memory_read as admin
      Given the canonical admin profile
      When that principal discovers or invokes memory_read
      Then the Python outcome is discover_and_call

    @surface-4447ab44f238_failure @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch @go_TestReadToolReference
    Scenario: memory_read preserves Python validation and failure behavior
      Given malformed, missing, out-of-scope, or unavailable inputs
      When the client invokes memory_read
      Then role/policy validation, typed argument validation, domain error envelope, unexpected transport redaction

  Rule: list workflow

    @surface-729f7a949314 @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch @go_TestReadToolReference
    Scenario: memory_list preserves the Python success contract
      Given the Python request contract [object Object]
      When an authorized client invokes memory_list
      Then the response matches the captured Python behavior is exercised

    @surface-729f7a949314_role_reader @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch @go_TestReadToolReference
    Scenario: memory_list as reader
      Given the canonical reader profile
      When that principal discovers or invokes memory_list
      Then the Python outcome is discover_and_call

    @surface-729f7a949314_role_proposer @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch @go_TestReadToolReference
    Scenario: memory_list as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes memory_list
      Then the Python outcome is discover_and_call

    @surface-729f7a949314_role_curator @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch @go_TestReadToolReference
    Scenario: memory_list as curator
      Given the canonical curator profile
      When that principal discovers or invokes memory_list
      Then the Python outcome is discover_and_call

    @surface-729f7a949314_role_admin @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch @go_TestReadToolReference
    Scenario: memory_list as admin
      Given the canonical admin profile
      When that principal discovers or invokes memory_list
      Then the Python outcome is discover_and_call

    @surface-729f7a949314_failure @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch @go_TestReadToolReference
    Scenario: memory_list preserves Python validation and failure behavior
      Given malformed, missing, out-of-scope, or unavailable inputs
      When the client invokes memory_list
      Then role/policy validation, typed argument validation, domain error envelope, unexpected transport redaction

  Rule: inventory workflow

    @surface-0e9b58d5dc55 @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_inventory preserves the Python success contract
      Given the Python request contract [object Object]
      When an authorized client invokes memory_inventory
      Then the response matches the captured Python behavior is exercised

    @surface-0e9b58d5dc55_role_reader @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_inventory as reader
      Given the canonical reader profile
      When that principal discovers or invokes memory_inventory
      Then the Python outcome is discover_and_call

    @surface-0e9b58d5dc55_role_proposer @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_inventory as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes memory_inventory
      Then the Python outcome is discover_and_call

    @surface-0e9b58d5dc55_role_curator @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_inventory as curator
      Given the canonical curator profile
      When that principal discovers or invokes memory_inventory
      Then the Python outcome is discover_and_call

    @surface-0e9b58d5dc55_role_admin @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_inventory as admin
      Given the canonical admin profile
      When that principal discovers or invokes memory_inventory
      Then the Python outcome is discover_and_call

    @surface-0e9b58d5dc55_failure @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_inventory preserves Python validation and failure behavior
      Given malformed, missing, out-of-scope, or unavailable inputs
      When the client invokes memory_inventory
      Then role/policy validation, typed argument validation, domain error envelope, unexpected transport redaction

  Rule: compare workflow

    @surface-b5cbb09a4d48 @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_compare_manifest preserves the Python success contract
      Given the Python request contract [object Object]
      When an authorized client invokes memory_compare_manifest
      Then the response matches the captured Python behavior is exercised

    @surface-b5cbb09a4d48_role_reader @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_compare_manifest as reader
      Given the canonical reader profile
      When that principal discovers or invokes memory_compare_manifest
      Then the Python outcome is discover_and_call

    @surface-b5cbb09a4d48_role_proposer @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_compare_manifest as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes memory_compare_manifest
      Then the Python outcome is discover_and_call

    @surface-b5cbb09a4d48_role_curator @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_compare_manifest as curator
      Given the canonical curator profile
      When that principal discovers or invokes memory_compare_manifest
      Then the Python outcome is discover_and_call

    @surface-b5cbb09a4d48_role_admin @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_compare_manifest as admin
      Given the canonical admin profile
      When that principal discovers or invokes memory_compare_manifest
      Then the Python outcome is discover_and_call

    @surface-b5cbb09a4d48_failure @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_compare_manifest preserves Python validation and failure behavior
      Given malformed, missing, out-of-scope, or unavailable inputs
      When the client invokes memory_compare_manifest
      Then role/policy validation, typed argument validation, domain error envelope, unexpected transport redaction

  Rule: graph workflow

    @surface-0f2a0410c00b @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch @go_TestReadToolReference
    Scenario: memory_graph preserves the Python success contract
      Given the Python request contract [object Object]
      When an authorized client invokes memory_graph
      Then the response matches the captured Python behavior is exercised

    @surface-0f2a0410c00b_role_reader @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch @go_TestReadToolReference
    Scenario: memory_graph as reader
      Given the canonical reader profile
      When that principal discovers or invokes memory_graph
      Then the Python outcome is discover_and_call

    @surface-0f2a0410c00b_role_proposer @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch @go_TestReadToolReference
    Scenario: memory_graph as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes memory_graph
      Then the Python outcome is discover_and_call

    @surface-0f2a0410c00b_role_curator @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch @go_TestReadToolReference
    Scenario: memory_graph as curator
      Given the canonical curator profile
      When that principal discovers or invokes memory_graph
      Then the Python outcome is discover_and_call

    @surface-0f2a0410c00b_role_admin @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch @go_TestReadToolReference
    Scenario: memory_graph as admin
      Given the canonical admin profile
      When that principal discovers or invokes memory_graph
      Then the Python outcome is discover_and_call

    @surface-0f2a0410c00b_failure @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch @go_TestReadToolReference
    Scenario: memory_graph preserves Python validation and failure behavior
      Given malformed, missing, out-of-scope, or unavailable inputs
      When the client invokes memory_graph
      Then role/policy validation, typed argument validation, domain error envelope, unexpected transport redaction
