Feature: mcp/proposal lifecycle

  Each operation has a success contract, four canonical role outcomes, and a failure contract.

  Rule: audit workflow

    @surface-b14276d52368 @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_audit preserves the Python success contract
      Given the Python request contract [object Object]
      When an authorized client invokes memory_audit
      Then the response matches the captured Python behavior is exercised

    @surface-b14276d52368_role_reader @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_audit as reader
      Given the canonical reader profile
      When that principal discovers or invokes memory_audit
      Then the Python outcome is hidden_and_forbidden

    @surface-b14276d52368_role_proposer @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_audit as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes memory_audit
      Then the Python outcome is discover_and_call

    @surface-b14276d52368_role_curator @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_audit as curator
      Given the canonical curator profile
      When that principal discovers or invokes memory_audit
      Then the Python outcome is discover_and_call

    @surface-b14276d52368_role_admin @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_audit as admin
      Given the canonical admin profile
      When that principal discovers or invokes memory_audit
      Then the Python outcome is discover_and_call

    @surface-b14276d52368_failure @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_audit preserves Python validation and failure behavior
      Given malformed, missing, out-of-scope, or unavailable inputs
      When the client invokes memory_audit
      Then role/policy validation, typed argument validation, domain error envelope, unexpected transport redaction

  Rule: propose workflow

    @surface-2c384e774062 @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_propose preserves the Python success contract
      Given the Python request contract [object Object]
      When an authorized client invokes memory_propose
      Then the response matches the captured Python behavior is exercised

    @surface-2c384e774062_role_reader @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_propose as reader
      Given the canonical reader profile
      When that principal discovers or invokes memory_propose
      Then the Python outcome is hidden_and_forbidden

    @surface-2c384e774062_role_proposer @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_propose as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes memory_propose
      Then the Python outcome is discover_and_call

    @surface-2c384e774062_role_curator @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_propose as curator
      Given the canonical curator profile
      When that principal discovers or invokes memory_propose
      Then the Python outcome is discover_and_call

    @surface-2c384e774062_role_admin @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_propose as admin
      Given the canonical admin profile
      When that principal discovers or invokes memory_propose
      Then the Python outcome is discover_and_call

    @surface-2c384e774062_failure @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_propose preserves Python validation and failure behavior
      Given malformed, missing, out-of-scope, or unavailable inputs
      When the client invokes memory_propose
      Then role/policy validation, typed argument validation, domain error envelope, unexpected transport redaction

    @surface-831a46271491 @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_propose_freeform preserves the Python success contract
      Given the Python request contract [object Object]
      When an authorized client invokes memory_propose_freeform
      Then the response matches the captured Python behavior is exercised

    @surface-831a46271491_role_reader @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_propose_freeform as reader
      Given the canonical reader profile
      When that principal discovers or invokes memory_propose_freeform
      Then the Python outcome is hidden_and_forbidden

    @surface-831a46271491_role_proposer @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_propose_freeform as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes memory_propose_freeform
      Then the Python outcome is discover_and_call

    @surface-831a46271491_role_curator @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_propose_freeform as curator
      Given the canonical curator profile
      When that principal discovers or invokes memory_propose_freeform
      Then the Python outcome is discover_and_call

    @surface-831a46271491_role_admin @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_propose_freeform as admin
      Given the canonical admin profile
      When that principal discovers or invokes memory_propose_freeform
      Then the Python outcome is discover_and_call

    @surface-831a46271491_failure @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_propose_freeform preserves Python validation and failure behavior
      Given malformed, missing, out-of-scope, or unavailable inputs
      When the client invokes memory_propose_freeform
      Then role/policy validation, typed argument validation, domain error envelope, unexpected transport redaction

    @surface-990560045406 @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_propose_update preserves the Python success contract
      Given the Python request contract [object Object]
      When an authorized client invokes memory_propose_update
      Then the response matches the captured Python behavior is exercised

    @surface-990560045406_role_reader @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_propose_update as reader
      Given the canonical reader profile
      When that principal discovers or invokes memory_propose_update
      Then the Python outcome is hidden_and_forbidden

    @surface-990560045406_role_proposer @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_propose_update as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes memory_propose_update
      Then the Python outcome is discover_and_call

    @surface-990560045406_role_curator @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_propose_update as curator
      Given the canonical curator profile
      When that principal discovers or invokes memory_propose_update
      Then the Python outcome is discover_and_call

    @surface-990560045406_role_admin @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_propose_update as admin
      Given the canonical admin profile
      When that principal discovers or invokes memory_propose_update
      Then the Python outcome is discover_and_call

    @surface-990560045406_failure @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_propose_update preserves Python validation and failure behavior
      Given malformed, missing, out-of-scope, or unavailable inputs
      When the client invokes memory_propose_update
      Then role/policy validation, typed argument validation, domain error envelope, unexpected transport redaction

  Rule: proposal workflow

    @surface-edbf396f4e8e @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_proposal_get preserves the Python success contract
      Given the Python request contract [object Object]
      When an authorized client invokes memory_proposal_get
      Then the response matches the captured Python behavior is exercised

    @surface-edbf396f4e8e_role_reader @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_proposal_get as reader
      Given the canonical reader profile
      When that principal discovers or invokes memory_proposal_get
      Then the Python outcome is hidden_and_forbidden

    @surface-edbf396f4e8e_role_proposer @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_proposal_get as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes memory_proposal_get
      Then the Python outcome is discover_and_call

    @surface-edbf396f4e8e_role_curator @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_proposal_get as curator
      Given the canonical curator profile
      When that principal discovers or invokes memory_proposal_get
      Then the Python outcome is discover_and_call

    @surface-edbf396f4e8e_role_admin @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_proposal_get as admin
      Given the canonical admin profile
      When that principal discovers or invokes memory_proposal_get
      Then the Python outcome is discover_and_call

    @surface-edbf396f4e8e_failure @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_proposal_get preserves Python validation and failure behavior
      Given malformed, missing, out-of-scope, or unavailable inputs
      When the client invokes memory_proposal_get
      Then role/policy validation, typed argument validation, domain error envelope, unexpected transport redaction

    @surface-90db0603af2c @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_proposal_list preserves the Python success contract
      Given the Python request contract [object Object]
      When an authorized client invokes memory_proposal_list
      Then the response matches the captured Python behavior is exercised

    @surface-90db0603af2c_role_reader @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_proposal_list as reader
      Given the canonical reader profile
      When that principal discovers or invokes memory_proposal_list
      Then the Python outcome is hidden_and_forbidden

    @surface-90db0603af2c_role_proposer @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_proposal_list as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes memory_proposal_list
      Then the Python outcome is discover_and_call

    @surface-90db0603af2c_role_curator @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_proposal_list as curator
      Given the canonical curator profile
      When that principal discovers or invokes memory_proposal_list
      Then the Python outcome is discover_and_call

    @surface-90db0603af2c_role_admin @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_proposal_list as admin
      Given the canonical admin profile
      When that principal discovers or invokes memory_proposal_list
      Then the Python outcome is discover_and_call

    @surface-90db0603af2c_failure @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_proposal_list preserves Python validation and failure behavior
      Given malformed, missing, out-of-scope, or unavailable inputs
      When the client invokes memory_proposal_list
      Then role/policy validation, typed argument validation, domain error envelope, unexpected transport redaction

    @surface-106498966aa2 @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_proposal_asset_get preserves the Python success contract
      Given the Python request contract [object Object]
      When an authorized client invokes memory_proposal_asset_get
      Then the response matches the captured Python behavior is exercised

    @surface-106498966aa2_role_reader @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_proposal_asset_get as reader
      Given the canonical reader profile
      When that principal discovers or invokes memory_proposal_asset_get
      Then the Python outcome is hidden_and_forbidden

    @surface-106498966aa2_role_proposer @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_proposal_asset_get as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes memory_proposal_asset_get
      Then the Python outcome is discover_and_call

    @surface-106498966aa2_role_curator @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_proposal_asset_get as curator
      Given the canonical curator profile
      When that principal discovers or invokes memory_proposal_asset_get
      Then the Python outcome is discover_and_call

    @surface-106498966aa2_role_admin @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_proposal_asset_get as admin
      Given the canonical admin profile
      When that principal discovers or invokes memory_proposal_asset_get
      Then the Python outcome is discover_and_call

    @surface-106498966aa2_failure @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_proposal_asset_get preserves Python validation and failure behavior
      Given malformed, missing, out-of-scope, or unavailable inputs
      When the client invokes memory_proposal_asset_get
      Then role/policy validation, typed argument validation, domain error envelope, unexpected transport redaction

    @surface-0b4811f722bf @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_proposal_rebase preserves the Python success contract
      Given the Python request contract [object Object]
      When an authorized client invokes memory_proposal_rebase
      Then the response matches the captured Python behavior is exercised

    @surface-0b4811f722bf_role_reader @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_proposal_rebase as reader
      Given the canonical reader profile
      When that principal discovers or invokes memory_proposal_rebase
      Then the Python outcome is hidden_and_forbidden

    @surface-0b4811f722bf_role_proposer @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_proposal_rebase as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes memory_proposal_rebase
      Then the Python outcome is discover_and_call

    @surface-0b4811f722bf_role_curator @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_proposal_rebase as curator
      Given the canonical curator profile
      When that principal discovers or invokes memory_proposal_rebase
      Then the Python outcome is discover_and_call

    @surface-0b4811f722bf_role_admin @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_proposal_rebase as admin
      Given the canonical admin profile
      When that principal discovers or invokes memory_proposal_rebase
      Then the Python outcome is discover_and_call

    @surface-0b4811f722bf_failure @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_proposal_rebase preserves Python validation and failure behavior
      Given malformed, missing, out-of-scope, or unavailable inputs
      When the client invokes memory_proposal_rebase
      Then role/policy validation, typed argument validation, domain error envelope, unexpected transport redaction

    @surface-aa8ecc31f343 @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_proposal_revise preserves the Python success contract
      Given the Python request contract [object Object]
      When an authorized client invokes memory_proposal_revise
      Then the response matches the captured Python behavior is exercised

    @surface-aa8ecc31f343_role_reader @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_proposal_revise as reader
      Given the canonical reader profile
      When that principal discovers or invokes memory_proposal_revise
      Then the Python outcome is hidden_and_forbidden

    @surface-aa8ecc31f343_role_proposer @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_proposal_revise as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes memory_proposal_revise
      Then the Python outcome is hidden_and_forbidden

    @surface-aa8ecc31f343_role_curator @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_proposal_revise as curator
      Given the canonical curator profile
      When that principal discovers or invokes memory_proposal_revise
      Then the Python outcome is discover_and_call

    @surface-aa8ecc31f343_role_admin @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_proposal_revise as admin
      Given the canonical admin profile
      When that principal discovers or invokes memory_proposal_revise
      Then the Python outcome is discover_and_call

    @surface-aa8ecc31f343_failure @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_proposal_revise preserves Python validation and failure behavior
      Given malformed, missing, out-of-scope, or unavailable inputs
      When the client invokes memory_proposal_revise
      Then role/policy validation, typed argument validation, domain error envelope, unexpected transport redaction

    @surface-6101c4d10fb3 @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_proposal_review preserves the Python success contract
      Given the Python request contract [object Object]
      When an authorized client invokes memory_proposal_review
      Then the response matches the captured Python behavior is exercised

    @surface-6101c4d10fb3_role_reader @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_proposal_review as reader
      Given the canonical reader profile
      When that principal discovers or invokes memory_proposal_review
      Then the Python outcome is hidden_and_forbidden

    @surface-6101c4d10fb3_role_proposer @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_proposal_review as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes memory_proposal_review
      Then the Python outcome is hidden_and_forbidden

    @surface-6101c4d10fb3_role_curator @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_proposal_review as curator
      Given the canonical curator profile
      When that principal discovers or invokes memory_proposal_review
      Then the Python outcome is discover_and_call

    @surface-6101c4d10fb3_role_admin @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_proposal_review as admin
      Given the canonical admin profile
      When that principal discovers or invokes memory_proposal_review
      Then the Python outcome is discover_and_call

    @surface-6101c4d10fb3_failure @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_proposal_review preserves Python validation and failure behavior
      Given malformed, missing, out-of-scope, or unavailable inputs
      When the client invokes memory_proposal_review
      Then role/policy validation, typed argument validation, domain error envelope, unexpected transport redaction

    @surface-4e7107b7301d @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_proposal_apply preserves the Python success contract
      Given the Python request contract [object Object]
      When an authorized client invokes memory_proposal_apply
      Then the response matches the captured Python behavior is exercised

    @surface-4e7107b7301d_role_reader @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_proposal_apply as reader
      Given the canonical reader profile
      When that principal discovers or invokes memory_proposal_apply
      Then the Python outcome is hidden_and_forbidden

    @surface-4e7107b7301d_role_proposer @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_proposal_apply as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes memory_proposal_apply
      Then the Python outcome is hidden_and_forbidden

    @surface-4e7107b7301d_role_curator @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_proposal_apply as curator
      Given the canonical curator profile
      When that principal discovers or invokes memory_proposal_apply
      Then the Python outcome is discover_and_call

    @surface-4e7107b7301d_role_admin @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_proposal_apply as admin
      Given the canonical admin profile
      When that principal discovers or invokes memory_proposal_apply
      Then the Python outcome is discover_and_call

    @surface-4e7107b7301d_failure @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_proposal_apply preserves Python validation and failure behavior
      Given malformed, missing, out-of-scope, or unavailable inputs
      When the client invokes memory_proposal_apply
      Then role/policy validation, typed argument validation, domain error envelope, unexpected transport redaction

  Rule: operation workflow

    @surface-257627a91347 @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_operation_get preserves the Python success contract
      Given the Python request contract [object Object]
      When an authorized client invokes memory_operation_get
      Then the response matches the captured Python behavior is exercised

    @surface-257627a91347_role_reader @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_operation_get as reader
      Given the canonical reader profile
      When that principal discovers or invokes memory_operation_get
      Then the Python outcome is hidden_and_forbidden

    @surface-257627a91347_role_proposer @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_operation_get as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes memory_operation_get
      Then the Python outcome is discover_and_call

    @surface-257627a91347_role_curator @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_operation_get as curator
      Given the canonical curator profile
      When that principal discovers or invokes memory_operation_get
      Then the Python outcome is discover_and_call

    @surface-257627a91347_role_admin @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_operation_get as admin
      Given the canonical admin profile
      When that principal discovers or invokes memory_operation_get
      Then the Python outcome is discover_and_call

    @surface-257627a91347_failure @go_TestMCPContractEveryToolSchemaDefaultsAndValidation @go_TestConfiguredServerDispatch
    Scenario: memory_operation_get preserves Python validation and failure behavior
      Given malformed, missing, out-of-scope, or unavailable inputs
      When the client invokes memory_operation_get
      Then role/policy validation, typed argument validation, domain error envelope, unexpected transport redaction
