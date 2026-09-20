Feature: mcp/assets and staging

  Each operation has a success contract, four canonical role outcomes, and a failure contract.

  Rule: asset workflow

    @surface-43a98ab93c7b @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_asset_metadata preserves the Python success contract
      Given the Python request contract [object Object]
      When an authorized client invokes memory_asset_metadata
      Then the response matches the captured Python behavior is exercised

    @surface-43a98ab93c7b_role_reader @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_asset_metadata as reader
      Given the canonical reader profile
      When that principal discovers or invokes memory_asset_metadata
      Then the Python outcome is discover_and_call

    @surface-43a98ab93c7b_role_proposer @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_asset_metadata as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes memory_asset_metadata
      Then the Python outcome is discover_and_call

    @surface-43a98ab93c7b_role_curator @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_asset_metadata as curator
      Given the canonical curator profile
      When that principal discovers or invokes memory_asset_metadata
      Then the Python outcome is discover_and_call

    @surface-43a98ab93c7b_role_admin @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_asset_metadata as admin
      Given the canonical admin profile
      When that principal discovers or invokes memory_asset_metadata
      Then the Python outcome is discover_and_call

    @surface-43a98ab93c7b_failure @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_asset_metadata preserves Python validation and failure behavior
      Given malformed, missing, out-of-scope, or unavailable inputs
      When the client invokes memory_asset_metadata
      Then role/policy validation, typed argument validation, domain error envelope, unexpected transport redaction

    @surface-225a9d544903 @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_asset_stage_begin preserves the Python success contract
      Given the Python request contract [object Object]
      When an authorized client invokes memory_asset_stage_begin
      Then the response matches the captured Python behavior is exercised

    @surface-225a9d544903_role_reader @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_asset_stage_begin as reader
      Given the canonical reader profile
      When that principal discovers or invokes memory_asset_stage_begin
      Then the Python outcome is hidden_and_forbidden

    @surface-225a9d544903_role_proposer @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_asset_stage_begin as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes memory_asset_stage_begin
      Then the Python outcome is discover_and_call

    @surface-225a9d544903_role_curator @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_asset_stage_begin as curator
      Given the canonical curator profile
      When that principal discovers or invokes memory_asset_stage_begin
      Then the Python outcome is discover_and_call

    @surface-225a9d544903_role_admin @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_asset_stage_begin as admin
      Given the canonical admin profile
      When that principal discovers or invokes memory_asset_stage_begin
      Then the Python outcome is discover_and_call

    @surface-225a9d544903_failure @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_asset_stage_begin preserves Python validation and failure behavior
      Given malformed, missing, out-of-scope, or unavailable inputs
      When the client invokes memory_asset_stage_begin
      Then role/policy validation, typed argument validation, domain error envelope, unexpected transport redaction

    @surface-125cc3f7941c @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_asset_stage_status preserves the Python success contract
      Given the Python request contract [object Object]
      When an authorized client invokes memory_asset_stage_status
      Then the response matches the captured Python behavior is exercised

    @surface-125cc3f7941c_role_reader @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_asset_stage_status as reader
      Given the canonical reader profile
      When that principal discovers or invokes memory_asset_stage_status
      Then the Python outcome is hidden_and_forbidden

    @surface-125cc3f7941c_role_proposer @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_asset_stage_status as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes memory_asset_stage_status
      Then the Python outcome is discover_and_call

    @surface-125cc3f7941c_role_curator @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_asset_stage_status as curator
      Given the canonical curator profile
      When that principal discovers or invokes memory_asset_stage_status
      Then the Python outcome is discover_and_call

    @surface-125cc3f7941c_role_admin @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_asset_stage_status as admin
      Given the canonical admin profile
      When that principal discovers or invokes memory_asset_stage_status
      Then the Python outcome is discover_and_call

    @surface-125cc3f7941c_failure @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_asset_stage_status preserves Python validation and failure behavior
      Given malformed, missing, out-of-scope, or unavailable inputs
      When the client invokes memory_asset_stage_status
      Then role/policy validation, typed argument validation, domain error envelope, unexpected transport redaction

    @surface-ae92a01f6f63 @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_asset_get preserves the Python success contract
      Given the Python request contract [object Object]
      When an authorized client invokes memory_asset_get
      Then the response matches the captured Python behavior is exercised

    @surface-ae92a01f6f63_role_reader @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_asset_get as reader
      Given the canonical reader profile
      When that principal discovers or invokes memory_asset_get
      Then the Python outcome is discover_and_call

    @surface-ae92a01f6f63_role_proposer @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_asset_get as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes memory_asset_get
      Then the Python outcome is discover_and_call

    @surface-ae92a01f6f63_role_curator @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_asset_get as curator
      Given the canonical curator profile
      When that principal discovers or invokes memory_asset_get
      Then the Python outcome is discover_and_call

    @surface-ae92a01f6f63_role_admin @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_asset_get as admin
      Given the canonical admin profile
      When that principal discovers or invokes memory_asset_get
      Then the Python outcome is discover_and_call

    @surface-ae92a01f6f63_failure @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_asset_get preserves Python validation and failure behavior
      Given malformed, missing, out-of-scope, or unavailable inputs
      When the client invokes memory_asset_get
      Then role/policy validation, typed argument validation, domain error envelope, unexpected transport redaction

    @surface-51bd9bde8934 @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_asset_prune preserves the Python success contract
      Given the Python request contract [object Object]
      When an authorized client invokes memory_asset_prune
      Then the response matches the captured Python behavior is exercised

    @surface-51bd9bde8934_role_reader @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_asset_prune as reader
      Given the canonical reader profile
      When that principal discovers or invokes memory_asset_prune
      Then the Python outcome is hidden_and_forbidden

    @surface-51bd9bde8934_role_proposer @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_asset_prune as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes memory_asset_prune
      Then the Python outcome is hidden_and_forbidden

    @surface-51bd9bde8934_role_curator @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_asset_prune as curator
      Given the canonical curator profile
      When that principal discovers or invokes memory_asset_prune
      Then the Python outcome is discover_and_call

    @surface-51bd9bde8934_role_admin @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_asset_prune as admin
      Given the canonical admin profile
      When that principal discovers or invokes memory_asset_prune
      Then the Python outcome is discover_and_call

    @surface-51bd9bde8934_failure @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_asset_prune preserves Python validation and failure behavior
      Given malformed, missing, out-of-scope, or unavailable inputs
      When the client invokes memory_asset_prune
      Then role/policy validation, typed argument validation, domain error envelope, unexpected transport redaction
