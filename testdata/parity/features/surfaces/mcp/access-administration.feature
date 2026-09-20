Feature: mcp/access administration

  Each operation has a success contract, four canonical role outcomes, and a failure contract.

  Rule: principal workflow

    @surface-0edd0cc21aa2 @go_TestAccessToolDefinitions @go_TestAccessToolArgumentValidation @go_TestAccessReadToolsDiscoveryAndCalls
    Scenario: access_principal_list preserves the Python success contract
      Given the Python request contract [object Object]
      When an authorized client invokes access_principal_list
      Then the response matches managed principal/audit data or one-time credential for create/rotate

    @surface-0edd0cc21aa2_role_reader @go_TestAccessToolDefinitions @go_TestAccessToolArgumentValidation @go_TestAccessReadToolsDiscoveryAndCalls
    Scenario: access_principal_list as reader
      Given the canonical reader profile
      When that principal discovers or invokes access_principal_list
      Then the Python outcome is hidden_and_forbidden

    @surface-0edd0cc21aa2_role_proposer @go_TestAccessToolDefinitions @go_TestAccessToolArgumentValidation @go_TestAccessReadToolsDiscoveryAndCalls
    Scenario: access_principal_list as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes access_principal_list
      Then the Python outcome is hidden_and_forbidden

    @surface-0edd0cc21aa2_role_curator @go_TestAccessToolDefinitions @go_TestAccessToolArgumentValidation @go_TestAccessReadToolsDiscoveryAndCalls
    Scenario: access_principal_list as curator
      Given the canonical curator profile
      When that principal discovers or invokes access_principal_list
      Then the Python outcome is hidden_and_forbidden

    @surface-0edd0cc21aa2_role_admin @go_TestAccessToolDefinitions @go_TestAccessToolArgumentValidation @go_TestAccessReadToolsDiscoveryAndCalls
    Scenario: access_principal_list as admin
      Given the canonical admin profile
      When that principal discovers or invokes access_principal_list
      Then the Python outcome is discover_and_call

    @surface-0edd0cc21aa2_failure @go_TestAccessToolDefinitions @go_TestAccessToolArgumentValidation @go_TestAccessReadToolsDiscoveryAndCalls
    Scenario: access_principal_list preserves Python validation and failure behavior
      Given malformed, missing, out-of-scope, or unavailable inputs
      When the client invokes access_principal_list
      Then 401/forbidden without explicit admin; argument validation; final-admin guard; idempotency conflict; credential plaintext returned once

    @surface-589e66ca921c @go_TestAccessToolDefinitions @go_TestAccessToolArgumentValidation @go_TestAccessMutationLifecycle
    Scenario: access_principal_create preserves the Python success contract
      Given the Python request contract [object Object]
      When an authorized client invokes access_principal_create
      Then the response matches managed principal/audit data or one-time credential for create/rotate

    @surface-589e66ca921c_role_reader @go_TestAccessToolDefinitions @go_TestAccessToolArgumentValidation @go_TestAccessMutationLifecycle
    Scenario: access_principal_create as reader
      Given the canonical reader profile
      When that principal discovers or invokes access_principal_create
      Then the Python outcome is hidden_and_forbidden

    @surface-589e66ca921c_role_proposer @go_TestAccessToolDefinitions @go_TestAccessToolArgumentValidation @go_TestAccessMutationLifecycle
    Scenario: access_principal_create as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes access_principal_create
      Then the Python outcome is hidden_and_forbidden

    @surface-589e66ca921c_role_curator @go_TestAccessToolDefinitions @go_TestAccessToolArgumentValidation @go_TestAccessMutationLifecycle
    Scenario: access_principal_create as curator
      Given the canonical curator profile
      When that principal discovers or invokes access_principal_create
      Then the Python outcome is hidden_and_forbidden

    @surface-589e66ca921c_role_admin @go_TestAccessToolDefinitions @go_TestAccessToolArgumentValidation @go_TestAccessMutationLifecycle
    Scenario: access_principal_create as admin
      Given the canonical admin profile
      When that principal discovers or invokes access_principal_create
      Then the Python outcome is discover_and_call

    @surface-589e66ca921c_failure @go_TestAccessToolDefinitions @go_TestAccessToolArgumentValidation @go_TestAccessMutationLifecycle
    Scenario: access_principal_create preserves Python validation and failure behavior
      Given malformed, missing, out-of-scope, or unavailable inputs
      When the client invokes access_principal_create
      Then 401/forbidden without explicit admin; argument validation; final-admin guard; idempotency conflict; credential plaintext returned once

    @surface-2c7b02ad6f21 @go_TestAccessToolDefinitions @go_TestAccessToolArgumentValidation @go_TestAccessMutationLifecycle
    Scenario: access_principal_update preserves the Python success contract
      Given the Python request contract [object Object]
      When an authorized client invokes access_principal_update
      Then the response matches managed principal/audit data or one-time credential for create/rotate

    @surface-2c7b02ad6f21_role_reader @go_TestAccessToolDefinitions @go_TestAccessToolArgumentValidation @go_TestAccessMutationLifecycle
    Scenario: access_principal_update as reader
      Given the canonical reader profile
      When that principal discovers or invokes access_principal_update
      Then the Python outcome is hidden_and_forbidden

    @surface-2c7b02ad6f21_role_proposer @go_TestAccessToolDefinitions @go_TestAccessToolArgumentValidation @go_TestAccessMutationLifecycle
    Scenario: access_principal_update as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes access_principal_update
      Then the Python outcome is hidden_and_forbidden

    @surface-2c7b02ad6f21_role_curator @go_TestAccessToolDefinitions @go_TestAccessToolArgumentValidation @go_TestAccessMutationLifecycle
    Scenario: access_principal_update as curator
      Given the canonical curator profile
      When that principal discovers or invokes access_principal_update
      Then the Python outcome is hidden_and_forbidden

    @surface-2c7b02ad6f21_role_admin @go_TestAccessToolDefinitions @go_TestAccessToolArgumentValidation @go_TestAccessMutationLifecycle
    Scenario: access_principal_update as admin
      Given the canonical admin profile
      When that principal discovers or invokes access_principal_update
      Then the Python outcome is discover_and_call

    @surface-2c7b02ad6f21_failure @go_TestAccessToolDefinitions @go_TestAccessToolArgumentValidation @go_TestAccessMutationLifecycle
    Scenario: access_principal_update preserves Python validation and failure behavior
      Given malformed, missing, out-of-scope, or unavailable inputs
      When the client invokes access_principal_update
      Then 401/forbidden without explicit admin; argument validation; final-admin guard; idempotency conflict; credential plaintext returned once

    @surface-313fae116d9d @go_TestAccessToolDefinitions @go_TestAccessToolArgumentValidation @go_TestAccessMutationLifecycle
    Scenario: access_principal_rename preserves the Python success contract
      Given the Python request contract [object Object]
      When an authorized client invokes access_principal_rename
      Then the response matches managed principal/audit data or one-time credential for create/rotate

    @surface-313fae116d9d_role_reader @go_TestAccessToolDefinitions @go_TestAccessToolArgumentValidation @go_TestAccessMutationLifecycle
    Scenario: access_principal_rename as reader
      Given the canonical reader profile
      When that principal discovers or invokes access_principal_rename
      Then the Python outcome is hidden_and_forbidden

    @surface-313fae116d9d_role_proposer @go_TestAccessToolDefinitions @go_TestAccessToolArgumentValidation @go_TestAccessMutationLifecycle
    Scenario: access_principal_rename as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes access_principal_rename
      Then the Python outcome is hidden_and_forbidden

    @surface-313fae116d9d_role_curator @go_TestAccessToolDefinitions @go_TestAccessToolArgumentValidation @go_TestAccessMutationLifecycle
    Scenario: access_principal_rename as curator
      Given the canonical curator profile
      When that principal discovers or invokes access_principal_rename
      Then the Python outcome is hidden_and_forbidden

    @surface-313fae116d9d_role_admin @go_TestAccessToolDefinitions @go_TestAccessToolArgumentValidation @go_TestAccessMutationLifecycle
    Scenario: access_principal_rename as admin
      Given the canonical admin profile
      When that principal discovers or invokes access_principal_rename
      Then the Python outcome is discover_and_call

    @surface-313fae116d9d_failure @go_TestAccessToolDefinitions @go_TestAccessToolArgumentValidation @go_TestAccessMutationLifecycle
    Scenario: access_principal_rename preserves Python validation and failure behavior
      Given malformed, missing, out-of-scope, or unavailable inputs
      When the client invokes access_principal_rename
      Then 401/forbidden without explicit admin; argument validation; final-admin guard; idempotency conflict; credential plaintext returned once

    @surface-43df6433a536 @go_TestAccessToolDefinitions @go_TestAccessToolArgumentValidation @go_TestAccessMutationLifecycle
    Scenario: access_principal_disable preserves the Python success contract
      Given the Python request contract [object Object]
      When an authorized client invokes access_principal_disable
      Then the response matches managed principal/audit data or one-time credential for create/rotate

    @surface-43df6433a536_role_reader @go_TestAccessToolDefinitions @go_TestAccessToolArgumentValidation @go_TestAccessMutationLifecycle
    Scenario: access_principal_disable as reader
      Given the canonical reader profile
      When that principal discovers or invokes access_principal_disable
      Then the Python outcome is hidden_and_forbidden

    @surface-43df6433a536_role_proposer @go_TestAccessToolDefinitions @go_TestAccessToolArgumentValidation @go_TestAccessMutationLifecycle
    Scenario: access_principal_disable as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes access_principal_disable
      Then the Python outcome is hidden_and_forbidden

    @surface-43df6433a536_role_curator @go_TestAccessToolDefinitions @go_TestAccessToolArgumentValidation @go_TestAccessMutationLifecycle
    Scenario: access_principal_disable as curator
      Given the canonical curator profile
      When that principal discovers or invokes access_principal_disable
      Then the Python outcome is hidden_and_forbidden

    @surface-43df6433a536_role_admin @go_TestAccessToolDefinitions @go_TestAccessToolArgumentValidation @go_TestAccessMutationLifecycle
    Scenario: access_principal_disable as admin
      Given the canonical admin profile
      When that principal discovers or invokes access_principal_disable
      Then the Python outcome is discover_and_call

    @surface-43df6433a536_failure @go_TestAccessToolDefinitions @go_TestAccessToolArgumentValidation @go_TestAccessMutationLifecycle
    Scenario: access_principal_disable preserves Python validation and failure behavior
      Given malformed, missing, out-of-scope, or unavailable inputs
      When the client invokes access_principal_disable
      Then 401/forbidden without explicit admin; argument validation; final-admin guard; idempotency conflict; credential plaintext returned once

    @surface-d5f2bac6e5bf @go_TestAccessToolDefinitions @go_TestAccessToolArgumentValidation @go_TestAccessMutationLifecycle
    Scenario: access_principal_enable preserves the Python success contract
      Given the Python request contract [object Object]
      When an authorized client invokes access_principal_enable
      Then the response matches managed principal/audit data or one-time credential for create/rotate

    @surface-d5f2bac6e5bf_role_reader @go_TestAccessToolDefinitions @go_TestAccessToolArgumentValidation @go_TestAccessMutationLifecycle
    Scenario: access_principal_enable as reader
      Given the canonical reader profile
      When that principal discovers or invokes access_principal_enable
      Then the Python outcome is hidden_and_forbidden

    @surface-d5f2bac6e5bf_role_proposer @go_TestAccessToolDefinitions @go_TestAccessToolArgumentValidation @go_TestAccessMutationLifecycle
    Scenario: access_principal_enable as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes access_principal_enable
      Then the Python outcome is hidden_and_forbidden

    @surface-d5f2bac6e5bf_role_curator @go_TestAccessToolDefinitions @go_TestAccessToolArgumentValidation @go_TestAccessMutationLifecycle
    Scenario: access_principal_enable as curator
      Given the canonical curator profile
      When that principal discovers or invokes access_principal_enable
      Then the Python outcome is hidden_and_forbidden

    @surface-d5f2bac6e5bf_role_admin @go_TestAccessToolDefinitions @go_TestAccessToolArgumentValidation @go_TestAccessMutationLifecycle
    Scenario: access_principal_enable as admin
      Given the canonical admin profile
      When that principal discovers or invokes access_principal_enable
      Then the Python outcome is discover_and_call

    @surface-d5f2bac6e5bf_failure @go_TestAccessToolDefinitions @go_TestAccessToolArgumentValidation @go_TestAccessMutationLifecycle
    Scenario: access_principal_enable preserves Python validation and failure behavior
      Given malformed, missing, out-of-scope, or unavailable inputs
      When the client invokes access_principal_enable
      Then 401/forbidden without explicit admin; argument validation; final-admin guard; idempotency conflict; credential plaintext returned once

    @surface-6509fdd0e990 @go_TestAccessToolDefinitions @go_TestAccessToolArgumentValidation @go_TestAccessMutationLifecycle
    Scenario: access_principal_revoke preserves the Python success contract
      Given the Python request contract [object Object]
      When an authorized client invokes access_principal_revoke
      Then the response matches managed principal/audit data or one-time credential for create/rotate

    @surface-6509fdd0e990_role_reader @go_TestAccessToolDefinitions @go_TestAccessToolArgumentValidation @go_TestAccessMutationLifecycle
    Scenario: access_principal_revoke as reader
      Given the canonical reader profile
      When that principal discovers or invokes access_principal_revoke
      Then the Python outcome is hidden_and_forbidden

    @surface-6509fdd0e990_role_proposer @go_TestAccessToolDefinitions @go_TestAccessToolArgumentValidation @go_TestAccessMutationLifecycle
    Scenario: access_principal_revoke as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes access_principal_revoke
      Then the Python outcome is hidden_and_forbidden

    @surface-6509fdd0e990_role_curator @go_TestAccessToolDefinitions @go_TestAccessToolArgumentValidation @go_TestAccessMutationLifecycle
    Scenario: access_principal_revoke as curator
      Given the canonical curator profile
      When that principal discovers or invokes access_principal_revoke
      Then the Python outcome is hidden_and_forbidden

    @surface-6509fdd0e990_role_admin @go_TestAccessToolDefinitions @go_TestAccessToolArgumentValidation @go_TestAccessMutationLifecycle
    Scenario: access_principal_revoke as admin
      Given the canonical admin profile
      When that principal discovers or invokes access_principal_revoke
      Then the Python outcome is discover_and_call

    @surface-6509fdd0e990_failure @go_TestAccessToolDefinitions @go_TestAccessToolArgumentValidation @go_TestAccessMutationLifecycle
    Scenario: access_principal_revoke preserves Python validation and failure behavior
      Given malformed, missing, out-of-scope, or unavailable inputs
      When the client invokes access_principal_revoke
      Then 401/forbidden without explicit admin; argument validation; final-admin guard; idempotency conflict; credential plaintext returned once

    @surface-bda80ecae438 @go_TestAccessToolDefinitions @go_TestAccessToolArgumentValidation @go_TestAccessMutationLifecycle
    Scenario: access_principal_delete preserves the Python success contract
      Given the Python request contract [object Object]
      When an authorized client invokes access_principal_delete
      Then the response matches managed principal/audit data or one-time credential for create/rotate

    @surface-bda80ecae438_role_reader @go_TestAccessToolDefinitions @go_TestAccessToolArgumentValidation @go_TestAccessMutationLifecycle
    Scenario: access_principal_delete as reader
      Given the canonical reader profile
      When that principal discovers or invokes access_principal_delete
      Then the Python outcome is hidden_and_forbidden

    @surface-bda80ecae438_role_proposer @go_TestAccessToolDefinitions @go_TestAccessToolArgumentValidation @go_TestAccessMutationLifecycle
    Scenario: access_principal_delete as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes access_principal_delete
      Then the Python outcome is hidden_and_forbidden

    @surface-bda80ecae438_role_curator @go_TestAccessToolDefinitions @go_TestAccessToolArgumentValidation @go_TestAccessMutationLifecycle
    Scenario: access_principal_delete as curator
      Given the canonical curator profile
      When that principal discovers or invokes access_principal_delete
      Then the Python outcome is hidden_and_forbidden

    @surface-bda80ecae438_role_admin @go_TestAccessToolDefinitions @go_TestAccessToolArgumentValidation @go_TestAccessMutationLifecycle
    Scenario: access_principal_delete as admin
      Given the canonical admin profile
      When that principal discovers or invokes access_principal_delete
      Then the Python outcome is discover_and_call

    @surface-bda80ecae438_failure @go_TestAccessToolDefinitions @go_TestAccessToolArgumentValidation @go_TestAccessMutationLifecycle
    Scenario: access_principal_delete preserves Python validation and failure behavior
      Given malformed, missing, out-of-scope, or unavailable inputs
      When the client invokes access_principal_delete
      Then 401/forbidden without explicit admin; argument validation; final-admin guard; idempotency conflict; credential plaintext returned once

  Rule: audit workflow

    @surface-997d587c4243 @go_TestAccessToolDefinitions @go_TestAccessToolArgumentValidation @go_TestAccessReadToolsDiscoveryAndCalls
    Scenario: access_audit_list preserves the Python success contract
      Given the Python request contract [object Object]
      When an authorized client invokes access_audit_list
      Then the response matches managed principal/audit data or one-time credential for create/rotate

    @surface-997d587c4243_role_reader @go_TestAccessToolDefinitions @go_TestAccessToolArgumentValidation @go_TestAccessReadToolsDiscoveryAndCalls
    Scenario: access_audit_list as reader
      Given the canonical reader profile
      When that principal discovers or invokes access_audit_list
      Then the Python outcome is hidden_and_forbidden

    @surface-997d587c4243_role_proposer @go_TestAccessToolDefinitions @go_TestAccessToolArgumentValidation @go_TestAccessReadToolsDiscoveryAndCalls
    Scenario: access_audit_list as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes access_audit_list
      Then the Python outcome is hidden_and_forbidden

    @surface-997d587c4243_role_curator @go_TestAccessToolDefinitions @go_TestAccessToolArgumentValidation @go_TestAccessReadToolsDiscoveryAndCalls
    Scenario: access_audit_list as curator
      Given the canonical curator profile
      When that principal discovers or invokes access_audit_list
      Then the Python outcome is hidden_and_forbidden

    @surface-997d587c4243_role_admin @go_TestAccessToolDefinitions @go_TestAccessToolArgumentValidation @go_TestAccessReadToolsDiscoveryAndCalls
    Scenario: access_audit_list as admin
      Given the canonical admin profile
      When that principal discovers or invokes access_audit_list
      Then the Python outcome is discover_and_call

    @surface-997d587c4243_failure @go_TestAccessToolDefinitions @go_TestAccessToolArgumentValidation @go_TestAccessReadToolsDiscoveryAndCalls
    Scenario: access_audit_list preserves Python validation and failure behavior
      Given malformed, missing, out-of-scope, or unavailable inputs
      When the client invokes access_audit_list
      Then 401/forbidden without explicit admin; argument validation; final-admin guard; idempotency conflict; credential plaintext returned once

  Rule: credential workflow

    @surface-155cf88caaea @go_TestAccessToolDefinitions @go_TestAccessToolArgumentValidation @go_TestAccessMutationLifecycle
    Scenario: access_credential_rotate preserves the Python success contract
      Given the Python request contract [object Object]
      When an authorized client invokes access_credential_rotate
      Then the response matches managed principal/audit data or one-time credential for create/rotate

    @surface-155cf88caaea_role_reader @go_TestAccessToolDefinitions @go_TestAccessToolArgumentValidation @go_TestAccessMutationLifecycle
    Scenario: access_credential_rotate as reader
      Given the canonical reader profile
      When that principal discovers or invokes access_credential_rotate
      Then the Python outcome is hidden_and_forbidden

    @surface-155cf88caaea_role_proposer @go_TestAccessToolDefinitions @go_TestAccessToolArgumentValidation @go_TestAccessMutationLifecycle
    Scenario: access_credential_rotate as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes access_credential_rotate
      Then the Python outcome is hidden_and_forbidden

    @surface-155cf88caaea_role_curator @go_TestAccessToolDefinitions @go_TestAccessToolArgumentValidation @go_TestAccessMutationLifecycle
    Scenario: access_credential_rotate as curator
      Given the canonical curator profile
      When that principal discovers or invokes access_credential_rotate
      Then the Python outcome is hidden_and_forbidden

    @surface-155cf88caaea_role_admin @go_TestAccessToolDefinitions @go_TestAccessToolArgumentValidation @go_TestAccessMutationLifecycle
    Scenario: access_credential_rotate as admin
      Given the canonical admin profile
      When that principal discovers or invokes access_credential_rotate
      Then the Python outcome is discover_and_call

    @surface-155cf88caaea_failure @go_TestAccessToolDefinitions @go_TestAccessToolArgumentValidation @go_TestAccessMutationLifecycle
    Scenario: access_credential_rotate preserves Python validation and failure behavior
      Given malformed, missing, out-of-scope, or unavailable inputs
      When the client invokes access_credential_rotate
      Then 401/forbidden without explicit admin; argument validation; final-admin guard; idempotency conflict; credential plaintext returned once
