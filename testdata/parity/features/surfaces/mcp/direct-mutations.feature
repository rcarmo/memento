Feature: mcp/direct mutations

  Each operation has a success contract, four canonical role outcomes, and a failure contract.

  Rule: create workflow

    @surface-125c710ec91b @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_create preserves the Python success contract
      Given the Python request contract [object Object]
      When an authorized client invokes memory_create
      Then the response matches the captured Python behavior is exercised

    @surface-125c710ec91b_role_reader @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_create as reader
      Given the canonical reader profile
      When that principal discovers or invokes memory_create
      Then the Python outcome is hidden_and_forbidden

    @surface-125c710ec91b_role_proposer @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_create as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes memory_create
      Then the Python outcome is hidden_and_forbidden

    @surface-125c710ec91b_role_curator @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_create as curator
      Given the canonical curator profile
      When that principal discovers or invokes memory_create
      Then the Python outcome is discover_and_call

    @surface-125c710ec91b_role_admin @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_create as admin
      Given the canonical admin profile
      When that principal discovers or invokes memory_create
      Then the Python outcome is discover_and_call

    @surface-125c710ec91b_failure @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_create preserves Python validation and failure behavior
      Given malformed, missing, out-of-scope, or unavailable inputs
      When the client invokes memory_create
      Then role/policy validation, typed argument validation, domain error envelope, unexpected transport redaction

  Rule: patch workflow

    @surface-69ad95b805dc @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_patch preserves the Python success contract
      Given the Python request contract [object Object]
      When an authorized client invokes memory_patch
      Then the response matches the captured Python behavior is exercised

    @surface-69ad95b805dc_role_reader @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_patch as reader
      Given the canonical reader profile
      When that principal discovers or invokes memory_patch
      Then the Python outcome is hidden_and_forbidden

    @surface-69ad95b805dc_role_proposer @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_patch as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes memory_patch
      Then the Python outcome is hidden_and_forbidden

    @surface-69ad95b805dc_role_curator @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_patch as curator
      Given the canonical curator profile
      When that principal discovers or invokes memory_patch
      Then the Python outcome is discover_and_call

    @surface-69ad95b805dc_role_admin @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_patch as admin
      Given the canonical admin profile
      When that principal discovers or invokes memory_patch
      Then the Python outcome is discover_and_call

    @surface-69ad95b805dc_failure @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_patch preserves Python validation and failure behavior
      Given malformed, missing, out-of-scope, or unavailable inputs
      When the client invokes memory_patch
      Then role/policy validation, typed argument validation, domain error envelope, unexpected transport redaction

  Rule: trash workflow

    @surface-4c8b7b60fd20 @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_trash preserves the Python success contract
      Given the Python request contract [object Object]
      When an authorized client invokes memory_trash
      Then the response matches the captured Python behavior is exercised

    @surface-4c8b7b60fd20_role_reader @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_trash as reader
      Given the canonical reader profile
      When that principal discovers or invokes memory_trash
      Then the Python outcome is hidden_and_forbidden

    @surface-4c8b7b60fd20_role_proposer @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_trash as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes memory_trash
      Then the Python outcome is hidden_and_forbidden

    @surface-4c8b7b60fd20_role_curator @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_trash as curator
      Given the canonical curator profile
      When that principal discovers or invokes memory_trash
      Then the Python outcome is discover_and_call

    @surface-4c8b7b60fd20_role_admin @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_trash as admin
      Given the canonical admin profile
      When that principal discovers or invokes memory_trash
      Then the Python outcome is discover_and_call

    @surface-4c8b7b60fd20_failure @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_trash preserves Python validation and failure behavior
      Given malformed, missing, out-of-scope, or unavailable inputs
      When the client invokes memory_trash
      Then role/policy validation, typed argument validation, domain error envelope, unexpected transport redaction

  Rule: restore workflow

    @surface-51275a4cb14f @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_restore preserves the Python success contract
      Given the Python request contract [object Object]
      When an authorized client invokes memory_restore
      Then the response matches the captured Python behavior is exercised

    @surface-51275a4cb14f_role_reader @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_restore as reader
      Given the canonical reader profile
      When that principal discovers or invokes memory_restore
      Then the Python outcome is hidden_and_forbidden

    @surface-51275a4cb14f_role_proposer @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_restore as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes memory_restore
      Then the Python outcome is hidden_and_forbidden

    @surface-51275a4cb14f_role_curator @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_restore as curator
      Given the canonical curator profile
      When that principal discovers or invokes memory_restore
      Then the Python outcome is discover_and_call

    @surface-51275a4cb14f_role_admin @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_restore as admin
      Given the canonical admin profile
      When that principal discovers or invokes memory_restore
      Then the Python outcome is discover_and_call

    @surface-51275a4cb14f_failure @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_restore preserves Python validation and failure behavior
      Given malformed, missing, out-of-scope, or unavailable inputs
      When the client invokes memory_restore
      Then role/policy validation, typed argument validation, domain error envelope, unexpected transport redaction

  Rule: purge workflow

    @surface-0334d36d2465 @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_purge preserves the Python success contract
      Given the Python request contract [object Object]
      When an authorized client invokes memory_purge
      Then the response matches the captured Python behavior is exercised

    @surface-0334d36d2465_role_reader @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_purge as reader
      Given the canonical reader profile
      When that principal discovers or invokes memory_purge
      Then the Python outcome is hidden_and_forbidden

    @surface-0334d36d2465_role_proposer @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_purge as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes memory_purge
      Then the Python outcome is hidden_and_forbidden

    @surface-0334d36d2465_role_curator @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_purge as curator
      Given the canonical curator profile
      When that principal discovers or invokes memory_purge
      Then the Python outcome is discover_and_call

    @surface-0334d36d2465_role_admin @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_purge as admin
      Given the canonical admin profile
      When that principal discovers or invokes memory_purge
      Then the Python outcome is discover_and_call

    @surface-0334d36d2465_failure @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_purge preserves Python validation and failure behavior
      Given malformed, missing, out-of-scope, or unavailable inputs
      When the client invokes memory_purge
      Then role/policy validation, typed argument validation, domain error envelope, unexpected transport redaction

  Rule: rename workflow

    @surface-085fbec32750 @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_rename preserves the Python success contract
      Given the Python request contract [object Object]
      When an authorized client invokes memory_rename
      Then the response matches the captured Python behavior is exercised

    @surface-085fbec32750_role_reader @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_rename as reader
      Given the canonical reader profile
      When that principal discovers or invokes memory_rename
      Then the Python outcome is hidden_and_forbidden

    @surface-085fbec32750_role_proposer @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_rename as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes memory_rename
      Then the Python outcome is hidden_and_forbidden

    @surface-085fbec32750_role_curator @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_rename as curator
      Given the canonical curator profile
      When that principal discovers or invokes memory_rename
      Then the Python outcome is discover_and_call

    @surface-085fbec32750_role_admin @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_rename as admin
      Given the canonical admin profile
      When that principal discovers or invokes memory_rename
      Then the Python outcome is discover_and_call

    @surface-085fbec32750_failure @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_rename preserves Python validation and failure behavior
      Given malformed, missing, out-of-scope, or unavailable inputs
      When the client invokes memory_rename
      Then role/policy validation, typed argument validation, domain error envelope, unexpected transport redaction
