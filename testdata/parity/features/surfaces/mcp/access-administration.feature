Feature: mcp/access administration

  Each operation has a success contract, four canonical role outcomes, and a failure contract.
  Stable row tags link behavior to validation data without exposing implementation details.

  Rule: principal workflow

    @surface-0edd0cc21aa2
    Scenario: access_principal_list succeeds with its declared contract
      Given required arguments no required arguments
      And optional arguments no optional arguments
      And declared defaults no declared defaults
      And policy scope explicit admin role; managed identity is re-resolved for every request
      When an authorized client invokes access_principal_list
      Then the response exposes principals[], principals[].name, principals[].roles, principals[].read_prefixes, principals[].write_prefixes, principals[].enabled, principals[].revoked, principals[].deleted
      And side effects are none
      And idempotency is not_applicable
      And pagination or range behavior is none

    @surface-0edd0cc21aa2_role_reader
    Scenario: access_principal_list as reader
      Given the operation is present on the configured tool surface and the canonical reader profile
      When that principal lists and calls access_principal_list
      Then the outcome is hidden_and_forbidden

    @surface-0edd0cc21aa2_role_proposer
    Scenario: access_principal_list as proposer
      Given the operation is present on the configured tool surface and the canonical proposer profile
      When that principal lists and calls access_principal_list
      Then the outcome is hidden_and_forbidden

    @surface-0edd0cc21aa2_role_curator
    Scenario: access_principal_list as curator
      Given the operation is present on the configured tool surface and the canonical curator profile
      When that principal lists and calls access_principal_list
      Then the outcome is hidden_and_forbidden

    @surface-0edd0cc21aa2_role_admin
    Scenario: access_principal_list as admin
      Given the operation is present on the configured tool surface and the canonical admin profile
      When that principal lists and calls access_principal_list
      Then the outcome is discover_and_call

    @surface-0edd0cc21aa2_failure
    Scenario: access_principal_list rejects invalid or conflicting requests
      Given required arguments no required arguments and declared defaults no declared defaults
      And malformed, missing, out-of-scope, unavailable, replayed, or conflicting inputs
      When the client invokes access_principal_list
      Then one of the specified failures is forbidden: explicit admin role required; validation_error: malformed arguments or invalid lifecycle transition
      And failed pre-publication calls do not apply none

    @surface-589e66ca921c
    Scenario: access_principal_create succeeds with its declared contract
      Given required arguments name, roles, read_prefixes, write_prefixes, idempotency_key
      And optional arguments no optional arguments
      And declared defaults no declared defaults
      And policy scope explicit admin role; managed identity is re-resolved for every request
      When an authorized client invokes access_principal_create
      Then the response exposes principal, credential
      And side effects are updates managed principal/credential state and appends access audit event
      And idempotency is required key; exact replay returns prior result; changed payload conflicts
      And pagination or range behavior is none

    @surface-589e66ca921c_role_reader
    Scenario: access_principal_create as reader
      Given the operation is present on the configured tool surface and the canonical reader profile
      When that principal lists and calls access_principal_create
      Then the outcome is hidden_and_forbidden

    @surface-589e66ca921c_role_proposer
    Scenario: access_principal_create as proposer
      Given the operation is present on the configured tool surface and the canonical proposer profile
      When that principal lists and calls access_principal_create
      Then the outcome is hidden_and_forbidden

    @surface-589e66ca921c_role_curator
    Scenario: access_principal_create as curator
      Given the operation is present on the configured tool surface and the canonical curator profile
      When that principal lists and calls access_principal_create
      Then the outcome is hidden_and_forbidden

    @surface-589e66ca921c_role_admin
    Scenario: access_principal_create as admin
      Given the operation is present on the configured tool surface and the canonical admin profile
      When that principal lists and calls access_principal_create
      Then the outcome is discover_and_call

    @surface-589e66ca921c_failure
    Scenario: access_principal_create rejects invalid or conflicting requests
      Given required arguments name, roles, read_prefixes, write_prefixes, idempotency_key and declared defaults no declared defaults
      And malformed, missing, out-of-scope, unavailable, replayed, or conflicting inputs
      When the client invokes access_principal_create
      Then one of the specified failures is forbidden: explicit admin role required; validation_error: malformed arguments or invalid lifecycle transition
      And failed pre-publication calls do not apply updates managed principal/credential state and appends access audit event

    @surface-2c7b02ad6f21
    Scenario: access_principal_update succeeds with its declared contract
      Given required arguments name, roles, read_prefixes, write_prefixes
      And optional arguments no optional arguments
      And declared defaults no declared defaults
      And policy scope explicit admin role; managed identity is re-resolved for every request
      When an authorized client invokes access_principal_update
      Then the response exposes principal
      And side effects are updates managed principal/credential state and appends access audit event
      And idempotency is operation-specific lifecycle guard
      And pagination or range behavior is none

    @surface-2c7b02ad6f21_role_reader
    Scenario: access_principal_update as reader
      Given the operation is present on the configured tool surface and the canonical reader profile
      When that principal lists and calls access_principal_update
      Then the outcome is hidden_and_forbidden

    @surface-2c7b02ad6f21_role_proposer
    Scenario: access_principal_update as proposer
      Given the operation is present on the configured tool surface and the canonical proposer profile
      When that principal lists and calls access_principal_update
      Then the outcome is hidden_and_forbidden

    @surface-2c7b02ad6f21_role_curator
    Scenario: access_principal_update as curator
      Given the operation is present on the configured tool surface and the canonical curator profile
      When that principal lists and calls access_principal_update
      Then the outcome is hidden_and_forbidden

    @surface-2c7b02ad6f21_role_admin
    Scenario: access_principal_update as admin
      Given the operation is present on the configured tool surface and the canonical admin profile
      When that principal lists and calls access_principal_update
      Then the outcome is discover_and_call

    @surface-2c7b02ad6f21_failure
    Scenario: access_principal_update rejects invalid or conflicting requests
      Given required arguments name, roles, read_prefixes, write_prefixes and declared defaults no declared defaults
      And malformed, missing, out-of-scope, unavailable, replayed, or conflicting inputs
      When the client invokes access_principal_update
      Then one of the specified failures is forbidden: explicit admin role required; validation_error: malformed arguments or invalid lifecycle transition
      And failed pre-publication calls do not apply updates managed principal/credential state and appends access audit event

    @surface-313fae116d9d
    Scenario: access_principal_rename succeeds with its declared contract
      Given required arguments name, new_name
      And optional arguments no optional arguments
      And declared defaults no declared defaults
      And policy scope explicit admin role; managed identity is re-resolved for every request
      When an authorized client invokes access_principal_rename
      Then the response exposes principal
      And side effects are updates managed principal/credential state and appends access audit event
      And idempotency is operation-specific lifecycle guard
      And pagination or range behavior is none

    @surface-313fae116d9d_role_reader
    Scenario: access_principal_rename as reader
      Given the operation is present on the configured tool surface and the canonical reader profile
      When that principal lists and calls access_principal_rename
      Then the outcome is hidden_and_forbidden

    @surface-313fae116d9d_role_proposer
    Scenario: access_principal_rename as proposer
      Given the operation is present on the configured tool surface and the canonical proposer profile
      When that principal lists and calls access_principal_rename
      Then the outcome is hidden_and_forbidden

    @surface-313fae116d9d_role_curator
    Scenario: access_principal_rename as curator
      Given the operation is present on the configured tool surface and the canonical curator profile
      When that principal lists and calls access_principal_rename
      Then the outcome is hidden_and_forbidden

    @surface-313fae116d9d_role_admin
    Scenario: access_principal_rename as admin
      Given the operation is present on the configured tool surface and the canonical admin profile
      When that principal lists and calls access_principal_rename
      Then the outcome is discover_and_call

    @surface-313fae116d9d_failure
    Scenario: access_principal_rename rejects invalid or conflicting requests
      Given required arguments name, new_name and declared defaults no declared defaults
      And malformed, missing, out-of-scope, unavailable, replayed, or conflicting inputs
      When the client invokes access_principal_rename
      Then one of the specified failures is forbidden: explicit admin role required; validation_error: malformed arguments or invalid lifecycle transition
      And failed pre-publication calls do not apply updates managed principal/credential state and appends access audit event

    @surface-43df6433a536
    Scenario: access_principal_disable succeeds with its declared contract
      Given required arguments name
      And optional arguments no optional arguments
      And declared defaults no declared defaults
      And policy scope explicit admin role; managed identity is re-resolved for every request
      When an authorized client invokes access_principal_disable
      Then the response exposes principal
      And side effects are updates managed principal/credential state and appends access audit event
      And idempotency is operation-specific lifecycle guard
      And pagination or range behavior is none

    @surface-43df6433a536_role_reader
    Scenario: access_principal_disable as reader
      Given the operation is present on the configured tool surface and the canonical reader profile
      When that principal lists and calls access_principal_disable
      Then the outcome is hidden_and_forbidden

    @surface-43df6433a536_role_proposer
    Scenario: access_principal_disable as proposer
      Given the operation is present on the configured tool surface and the canonical proposer profile
      When that principal lists and calls access_principal_disable
      Then the outcome is hidden_and_forbidden

    @surface-43df6433a536_role_curator
    Scenario: access_principal_disable as curator
      Given the operation is present on the configured tool surface and the canonical curator profile
      When that principal lists and calls access_principal_disable
      Then the outcome is hidden_and_forbidden

    @surface-43df6433a536_role_admin
    Scenario: access_principal_disable as admin
      Given the operation is present on the configured tool surface and the canonical admin profile
      When that principal lists and calls access_principal_disable
      Then the outcome is discover_and_call

    @surface-43df6433a536_failure
    Scenario: access_principal_disable rejects invalid or conflicting requests
      Given required arguments name and declared defaults no declared defaults
      And malformed, missing, out-of-scope, unavailable, replayed, or conflicting inputs
      When the client invokes access_principal_disable
      Then one of the specified failures is forbidden: explicit admin role required; validation_error: malformed arguments or invalid lifecycle transition
      And failed pre-publication calls do not apply updates managed principal/credential state and appends access audit event

    @surface-d5f2bac6e5bf
    Scenario: access_principal_enable succeeds with its declared contract
      Given required arguments name
      And optional arguments no optional arguments
      And declared defaults no declared defaults
      And policy scope explicit admin role; managed identity is re-resolved for every request
      When an authorized client invokes access_principal_enable
      Then the response exposes principal
      And side effects are updates managed principal/credential state and appends access audit event
      And idempotency is operation-specific lifecycle guard
      And pagination or range behavior is none

    @surface-d5f2bac6e5bf_role_reader
    Scenario: access_principal_enable as reader
      Given the operation is present on the configured tool surface and the canonical reader profile
      When that principal lists and calls access_principal_enable
      Then the outcome is hidden_and_forbidden

    @surface-d5f2bac6e5bf_role_proposer
    Scenario: access_principal_enable as proposer
      Given the operation is present on the configured tool surface and the canonical proposer profile
      When that principal lists and calls access_principal_enable
      Then the outcome is hidden_and_forbidden

    @surface-d5f2bac6e5bf_role_curator
    Scenario: access_principal_enable as curator
      Given the operation is present on the configured tool surface and the canonical curator profile
      When that principal lists and calls access_principal_enable
      Then the outcome is hidden_and_forbidden

    @surface-d5f2bac6e5bf_role_admin
    Scenario: access_principal_enable as admin
      Given the operation is present on the configured tool surface and the canonical admin profile
      When that principal lists and calls access_principal_enable
      Then the outcome is discover_and_call

    @surface-d5f2bac6e5bf_failure
    Scenario: access_principal_enable rejects invalid or conflicting requests
      Given required arguments name and declared defaults no declared defaults
      And malformed, missing, out-of-scope, unavailable, replayed, or conflicting inputs
      When the client invokes access_principal_enable
      Then one of the specified failures is forbidden: explicit admin role required; validation_error: malformed arguments or invalid lifecycle transition
      And failed pre-publication calls do not apply updates managed principal/credential state and appends access audit event

    @surface-6509fdd0e990
    Scenario: access_principal_revoke succeeds with its declared contract
      Given required arguments name
      And optional arguments no optional arguments
      And declared defaults no declared defaults
      And policy scope explicit admin role; managed identity is re-resolved for every request
      When an authorized client invokes access_principal_revoke
      Then the response exposes principal
      And side effects are updates managed principal/credential state and appends access audit event
      And idempotency is operation-specific lifecycle guard
      And pagination or range behavior is none

    @surface-6509fdd0e990_role_reader
    Scenario: access_principal_revoke as reader
      Given the operation is present on the configured tool surface and the canonical reader profile
      When that principal lists and calls access_principal_revoke
      Then the outcome is hidden_and_forbidden

    @surface-6509fdd0e990_role_proposer
    Scenario: access_principal_revoke as proposer
      Given the operation is present on the configured tool surface and the canonical proposer profile
      When that principal lists and calls access_principal_revoke
      Then the outcome is hidden_and_forbidden

    @surface-6509fdd0e990_role_curator
    Scenario: access_principal_revoke as curator
      Given the operation is present on the configured tool surface and the canonical curator profile
      When that principal lists and calls access_principal_revoke
      Then the outcome is hidden_and_forbidden

    @surface-6509fdd0e990_role_admin
    Scenario: access_principal_revoke as admin
      Given the operation is present on the configured tool surface and the canonical admin profile
      When that principal lists and calls access_principal_revoke
      Then the outcome is discover_and_call

    @surface-6509fdd0e990_failure
    Scenario: access_principal_revoke rejects invalid or conflicting requests
      Given required arguments name and declared defaults no declared defaults
      And malformed, missing, out-of-scope, unavailable, replayed, or conflicting inputs
      When the client invokes access_principal_revoke
      Then one of the specified failures is forbidden: explicit admin role required; validation_error: malformed arguments or invalid lifecycle transition
      And failed pre-publication calls do not apply updates managed principal/credential state and appends access audit event

    @surface-bda80ecae438
    Scenario: access_principal_delete succeeds with its declared contract
      Given required arguments name
      And optional arguments no optional arguments
      And declared defaults no declared defaults
      And policy scope explicit admin role; managed identity is re-resolved for every request
      When an authorized client invokes access_principal_delete
      Then the response exposes principal
      And side effects are updates managed principal/credential state and appends access audit event
      And idempotency is operation-specific lifecycle guard
      And pagination or range behavior is none

    @surface-bda80ecae438_role_reader
    Scenario: access_principal_delete as reader
      Given the operation is present on the configured tool surface and the canonical reader profile
      When that principal lists and calls access_principal_delete
      Then the outcome is hidden_and_forbidden

    @surface-bda80ecae438_role_proposer
    Scenario: access_principal_delete as proposer
      Given the operation is present on the configured tool surface and the canonical proposer profile
      When that principal lists and calls access_principal_delete
      Then the outcome is hidden_and_forbidden

    @surface-bda80ecae438_role_curator
    Scenario: access_principal_delete as curator
      Given the operation is present on the configured tool surface and the canonical curator profile
      When that principal lists and calls access_principal_delete
      Then the outcome is hidden_and_forbidden

    @surface-bda80ecae438_role_admin
    Scenario: access_principal_delete as admin
      Given the operation is present on the configured tool surface and the canonical admin profile
      When that principal lists and calls access_principal_delete
      Then the outcome is discover_and_call

    @surface-bda80ecae438_failure
    Scenario: access_principal_delete rejects invalid or conflicting requests
      Given required arguments name and declared defaults no declared defaults
      And malformed, missing, out-of-scope, unavailable, replayed, or conflicting inputs
      When the client invokes access_principal_delete
      Then one of the specified failures is forbidden: explicit admin role required; validation_error: malformed arguments or invalid lifecycle transition
      And failed pre-publication calls do not apply updates managed principal/credential state and appends access audit event

  Rule: audit workflow

    @surface-997d587c4243
    Scenario: access_audit_list succeeds with its declared contract
      Given required arguments no required arguments
      And optional arguments limit
      And declared defaults no declared defaults
      And policy scope explicit admin role; managed identity is re-resolved for every request
      When an authorized client invokes access_audit_list
      Then the response exposes events[], events[].action, events[].actor, events[].target, events[].created_at
      And side effects are none
      And idempotency is not_applicable
      And pagination or range behavior is none

    @surface-997d587c4243_role_reader
    Scenario: access_audit_list as reader
      Given the operation is present on the configured tool surface and the canonical reader profile
      When that principal lists and calls access_audit_list
      Then the outcome is hidden_and_forbidden

    @surface-997d587c4243_role_proposer
    Scenario: access_audit_list as proposer
      Given the operation is present on the configured tool surface and the canonical proposer profile
      When that principal lists and calls access_audit_list
      Then the outcome is hidden_and_forbidden

    @surface-997d587c4243_role_curator
    Scenario: access_audit_list as curator
      Given the operation is present on the configured tool surface and the canonical curator profile
      When that principal lists and calls access_audit_list
      Then the outcome is hidden_and_forbidden

    @surface-997d587c4243_role_admin
    Scenario: access_audit_list as admin
      Given the operation is present on the configured tool surface and the canonical admin profile
      When that principal lists and calls access_audit_list
      Then the outcome is discover_and_call

    @surface-997d587c4243_failure
    Scenario: access_audit_list rejects invalid or conflicting requests
      Given required arguments no required arguments and declared defaults no declared defaults
      And malformed, missing, out-of-scope, unavailable, replayed, or conflicting inputs
      When the client invokes access_audit_list
      Then one of the specified failures is forbidden: explicit admin role required; validation_error: malformed arguments or invalid lifecycle transition
      And failed pre-publication calls do not apply none

  Rule: credential workflow

    @surface-155cf88caaea
    Scenario: access_credential_rotate succeeds with its declared contract
      Given required arguments name, idempotency_key
      And optional arguments no optional arguments
      And declared defaults no declared defaults
      And policy scope explicit admin role; managed identity is re-resolved for every request
      When an authorized client invokes access_credential_rotate
      Then the response exposes name, credential
      And side effects are updates managed principal/credential state and appends access audit event
      And idempotency is required key; exact replay returns prior result; changed payload conflicts
      And pagination or range behavior is none

    @surface-155cf88caaea_role_reader
    Scenario: access_credential_rotate as reader
      Given the operation is present on the configured tool surface and the canonical reader profile
      When that principal lists and calls access_credential_rotate
      Then the outcome is hidden_and_forbidden

    @surface-155cf88caaea_role_proposer
    Scenario: access_credential_rotate as proposer
      Given the operation is present on the configured tool surface and the canonical proposer profile
      When that principal lists and calls access_credential_rotate
      Then the outcome is hidden_and_forbidden

    @surface-155cf88caaea_role_curator
    Scenario: access_credential_rotate as curator
      Given the operation is present on the configured tool surface and the canonical curator profile
      When that principal lists and calls access_credential_rotate
      Then the outcome is hidden_and_forbidden

    @surface-155cf88caaea_role_admin
    Scenario: access_credential_rotate as admin
      Given the operation is present on the configured tool surface and the canonical admin profile
      When that principal lists and calls access_credential_rotate
      Then the outcome is discover_and_call

    @surface-155cf88caaea_failure
    Scenario: access_credential_rotate rejects invalid or conflicting requests
      Given required arguments name, idempotency_key and declared defaults no declared defaults
      And malformed, missing, out-of-scope, unavailable, replayed, or conflicting inputs
      When the client invokes access_credential_rotate
      Then one of the specified failures is forbidden: explicit admin role required; validation_error: malformed arguments or invalid lifecycle transition
      And failed pre-publication calls do not apply updates managed principal/credential state and appends access audit event
