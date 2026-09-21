Feature: cli/operations

  Each operation has a success contract, four canonical role outcomes, and a failure contract.
  Stable row tags link behavior to validation data without exposing implementation details.

  Rule: cli workflow

    @surface-39eaef7c37c5
    Scenario: serve succeeds with its declared contract
      Given required arguments no required arguments
      And optional arguments no optional arguments
      And declared defaults no declared defaults
      And policy scope local operator filesystem/configuration permissions
      When an authorized client invokes serve
      Then the response exposes listener address/endpoint, ordered shutdown on signal
      And side effects are none
      And idempotency is not_applicable
      And pagination or range behavior is none

    @surface-39eaef7c37c5_role_reader
    Scenario: serve as reader
      Given the canonical reader profile
      When that principal uses serve
      Then the outcome is local_operator_not_role_scoped

    @surface-39eaef7c37c5_role_proposer
    Scenario: serve as proposer
      Given the canonical proposer profile
      When that principal uses serve
      Then the outcome is local_operator_not_role_scoped

    @surface-39eaef7c37c5_role_curator
    Scenario: serve as curator
      Given the canonical curator profile
      When that principal uses serve
      Then the outcome is local_operator_not_role_scoped

    @surface-39eaef7c37c5_role_admin
    Scenario: serve as admin
      Given the canonical admin profile
      When that principal uses serve
      Then the outcome is local_operator_not_role_scoped

    @surface-39eaef7c37c5_failure
    Scenario: serve rejects invalid or conflicting requests
      Given required arguments no required arguments and declared defaults no declared defaults
      And malformed, missing, out-of-scope, unavailable, replayed, or conflicting inputs
      When the client invokes serve
      Then one of the specified failures is usage error exits 2; runtime/configuration failure exits 1
      And failed pre-publication calls do not apply none

    @surface-43009273020f
    Scenario: audit succeeds with its declared contract
      Given required arguments no required arguments
      And optional arguments no optional arguments
      And declared defaults no declared defaults
      And policy scope local operator filesystem/configuration permissions
      When an authorized client invokes audit
      Then the response exposes ok, issues[], warnings[]
      And side effects are none
      And idempotency is not_applicable
      And pagination or range behavior is none

    @surface-43009273020f_role_reader
    Scenario: audit as reader
      Given the canonical reader profile
      When that principal uses audit
      Then the outcome is local_operator_not_role_scoped

    @surface-43009273020f_role_proposer
    Scenario: audit as proposer
      Given the canonical proposer profile
      When that principal uses audit
      Then the outcome is local_operator_not_role_scoped

    @surface-43009273020f_role_curator
    Scenario: audit as curator
      Given the canonical curator profile
      When that principal uses audit
      Then the outcome is local_operator_not_role_scoped

    @surface-43009273020f_role_admin
    Scenario: audit as admin
      Given the canonical admin profile
      When that principal uses audit
      Then the outcome is local_operator_not_role_scoped

    @surface-43009273020f_failure
    Scenario: audit rejects invalid or conflicting requests
      Given required arguments no required arguments and declared defaults no declared defaults
      And malformed, missing, out-of-scope, unavailable, replayed, or conflicting inputs
      When the client invokes audit
      Then one of the specified failures is usage error exits 2; runtime/configuration failure exits 1
      And failed pre-publication calls do not apply none

    @surface-00d140452a2a
    Scenario: rebuild-index succeeds with its declared contract
      Given required arguments no required arguments
      And optional arguments no optional arguments
      And declared defaults no declared defaults
      And policy scope local operator filesystem/configuration permissions
      When an authorized client invokes rebuild-index
      Then the response exposes repo_revision, index_revision, parity_matches, parity_details
      And side effects are performs the named bounded filesystem, database, index, scheduler, or key-rotation transition
      And idempotency is not_applicable
      And pagination or range behavior is none

    @surface-00d140452a2a_role_reader
    Scenario: rebuild-index as reader
      Given the canonical reader profile
      When that principal uses rebuild-index
      Then the outcome is local_operator_not_role_scoped

    @surface-00d140452a2a_role_proposer
    Scenario: rebuild-index as proposer
      Given the canonical proposer profile
      When that principal uses rebuild-index
      Then the outcome is local_operator_not_role_scoped

    @surface-00d140452a2a_role_curator
    Scenario: rebuild-index as curator
      Given the canonical curator profile
      When that principal uses rebuild-index
      Then the outcome is local_operator_not_role_scoped

    @surface-00d140452a2a_role_admin
    Scenario: rebuild-index as admin
      Given the canonical admin profile
      When that principal uses rebuild-index
      Then the outcome is local_operator_not_role_scoped

    @surface-00d140452a2a_failure
    Scenario: rebuild-index rejects invalid or conflicting requests
      Given required arguments no required arguments and declared defaults no declared defaults
      And malformed, missing, out-of-scope, unavailable, replayed, or conflicting inputs
      When the client invokes rebuild-index
      Then one of the specified failures is usage error exits 2; runtime/configuration failure exits 1
      And failed pre-publication calls do not apply performs the named bounded filesystem, database, index, scheduler, or key-rotation transition

    @surface-09741af9a2d6
    Scenario: backup succeeds with its declared contract
      Given required arguments no required arguments
      And optional arguments no optional arguments
      And declared defaults no declared defaults
      And policy scope local operator filesystem/configuration permissions
      When an authorized client invokes backup
      Then the response exposes archive path, manifest/checksums
      And side effects are performs the named bounded filesystem, database, index, scheduler, or key-rotation transition
      And idempotency is not_applicable
      And pagination or range behavior is none

    @surface-09741af9a2d6_role_reader
    Scenario: backup as reader
      Given the canonical reader profile
      When that principal uses backup
      Then the outcome is local_operator_not_role_scoped

    @surface-09741af9a2d6_role_proposer
    Scenario: backup as proposer
      Given the canonical proposer profile
      When that principal uses backup
      Then the outcome is local_operator_not_role_scoped

    @surface-09741af9a2d6_role_curator
    Scenario: backup as curator
      Given the canonical curator profile
      When that principal uses backup
      Then the outcome is local_operator_not_role_scoped

    @surface-09741af9a2d6_role_admin
    Scenario: backup as admin
      Given the canonical admin profile
      When that principal uses backup
      Then the outcome is local_operator_not_role_scoped

    @surface-09741af9a2d6_failure
    Scenario: backup rejects invalid or conflicting requests
      Given required arguments no required arguments and declared defaults no declared defaults
      And malformed, missing, out-of-scope, unavailable, replayed, or conflicting inputs
      When the client invokes backup
      Then one of the specified failures is usage error exits 2; runtime/configuration failure exits 1
      And failed pre-publication calls do not apply performs the named bounded filesystem, database, index, scheduler, or key-rotation transition

    @surface-6cc618194fbc
    Scenario: restore succeeds with its declared contract
      Given required arguments no required arguments
      And optional arguments no optional arguments
      And declared defaults no declared defaults
      And policy scope local operator filesystem/configuration permissions
      When an authorized client invokes restore
      Then the response exposes restored repository/control/derived state, revision
      And side effects are performs the named bounded filesystem, database, index, scheduler, or key-rotation transition
      And idempotency is not_applicable
      And pagination or range behavior is none

    @surface-6cc618194fbc_role_reader
    Scenario: restore as reader
      Given the canonical reader profile
      When that principal uses restore
      Then the outcome is local_operator_not_role_scoped

    @surface-6cc618194fbc_role_proposer
    Scenario: restore as proposer
      Given the canonical proposer profile
      When that principal uses restore
      Then the outcome is local_operator_not_role_scoped

    @surface-6cc618194fbc_role_curator
    Scenario: restore as curator
      Given the canonical curator profile
      When that principal uses restore
      Then the outcome is local_operator_not_role_scoped

    @surface-6cc618194fbc_role_admin
    Scenario: restore as admin
      Given the canonical admin profile
      When that principal uses restore
      Then the outcome is local_operator_not_role_scoped

    @surface-6cc618194fbc_failure
    Scenario: restore rejects invalid or conflicting requests
      Given required arguments no required arguments and declared defaults no declared defaults
      And malformed, missing, out-of-scope, unavailable, replayed, or conflicting inputs
      When the client invokes restore
      Then one of the specified failures is usage error exits 2; runtime/configuration failure exits 1
      And failed pre-publication calls do not apply performs the named bounded filesystem, database, index, scheduler, or key-rotation transition

    @surface-fed523668854
    Scenario: status succeeds with its declared contract
      Given required arguments no required arguments
      And optional arguments no optional arguments
      And declared defaults no declared defaults
      And policy scope local operator filesystem/configuration permissions
      When an authorized client invokes status
      Then the response exposes service_version, schema_version, repo_revision, index_revision, readiness, visible_concepts, proposal_backlog
      And side effects are none
      And idempotency is not_applicable
      And pagination or range behavior is none

    @surface-fed523668854_role_reader
    Scenario: status as reader
      Given the canonical reader profile
      When that principal uses status
      Then the outcome is local_operator_not_role_scoped

    @surface-fed523668854_role_proposer
    Scenario: status as proposer
      Given the canonical proposer profile
      When that principal uses status
      Then the outcome is local_operator_not_role_scoped

    @surface-fed523668854_role_curator
    Scenario: status as curator
      Given the canonical curator profile
      When that principal uses status
      Then the outcome is local_operator_not_role_scoped

    @surface-fed523668854_role_admin
    Scenario: status as admin
      Given the canonical admin profile
      When that principal uses status
      Then the outcome is local_operator_not_role_scoped

    @surface-fed523668854_failure
    Scenario: status rejects invalid or conflicting requests
      Given required arguments no required arguments and declared defaults no declared defaults
      And malformed, missing, out-of-scope, unavailable, replayed, or conflicting inputs
      When the client invokes status
      Then one of the specified failures is usage error exits 2; runtime/configuration failure exits 1
      And failed pre-publication calls do not apply none

    @surface-1354f1967675
    Scenario: dream succeeds with its declared contract
      Given required arguments no required arguments
      And optional arguments no optional arguments
      And declared defaults no declared defaults
      And policy scope local operator filesystem/configuration permissions
      When an authorized client invokes dream
      Then the response exposes state, run_id, signals_detected, actionable_signals, proposals_created, model_attempts
      And side effects are performs the named bounded filesystem, database, index, scheduler, or key-rotation transition
      And idempotency is not_applicable
      And pagination or range behavior is none

    @surface-1354f1967675_role_reader
    Scenario: dream as reader
      Given the canonical reader profile
      When that principal uses dream
      Then the outcome is local_operator_not_role_scoped

    @surface-1354f1967675_role_proposer
    Scenario: dream as proposer
      Given the canonical proposer profile
      When that principal uses dream
      Then the outcome is local_operator_not_role_scoped

    @surface-1354f1967675_role_curator
    Scenario: dream as curator
      Given the canonical curator profile
      When that principal uses dream
      Then the outcome is local_operator_not_role_scoped

    @surface-1354f1967675_role_admin
    Scenario: dream as admin
      Given the canonical admin profile
      When that principal uses dream
      Then the outcome is local_operator_not_role_scoped

    @surface-1354f1967675_failure
    Scenario: dream rejects invalid or conflicting requests
      Given required arguments no required arguments and declared defaults no declared defaults
      And malformed, missing, out-of-scope, unavailable, replayed, or conflicting inputs
      When the client invokes dream
      Then one of the specified failures is usage error exits 2; runtime/configuration failure exits 1
      And failed pre-publication calls do not apply performs the named bounded filesystem, database, index, scheduler, or key-rotation transition

    @surface-0334e05ca7bd
    Scenario: rotate-master-key succeeds with its declared contract
      Given required arguments no required arguments
      And optional arguments no optional arguments
      And declared defaults no declared defaults
      And policy scope local operator filesystem/configuration permissions
      When an authorized client invokes rotate-master-key
      Then the response exposes rewrapped verifier count, control DB path
      And side effects are performs the named bounded filesystem, database, index, scheduler, or key-rotation transition
      And idempotency is not_applicable
      And pagination or range behavior is none

    @surface-0334e05ca7bd_role_reader
    Scenario: rotate-master-key as reader
      Given the canonical reader profile
      When that principal uses rotate-master-key
      Then the outcome is local_operator_not_role_scoped

    @surface-0334e05ca7bd_role_proposer
    Scenario: rotate-master-key as proposer
      Given the canonical proposer profile
      When that principal uses rotate-master-key
      Then the outcome is local_operator_not_role_scoped

    @surface-0334e05ca7bd_role_curator
    Scenario: rotate-master-key as curator
      Given the canonical curator profile
      When that principal uses rotate-master-key
      Then the outcome is local_operator_not_role_scoped

    @surface-0334e05ca7bd_role_admin
    Scenario: rotate-master-key as admin
      Given the canonical admin profile
      When that principal uses rotate-master-key
      Then the outcome is local_operator_not_role_scoped

    @surface-0334e05ca7bd_failure
    Scenario: rotate-master-key rejects invalid or conflicting requests
      Given required arguments no required arguments and declared defaults no declared defaults
      And malformed, missing, out-of-scope, unavailable, replayed, or conflicting inputs
      When the client invokes rotate-master-key
      Then one of the specified failures is usage error exits 2; runtime/configuration failure exits 1
      And failed pre-publication calls do not apply performs the named bounded filesystem, database, index, scheduler, or key-rotation transition

    @surface-8cf2b70cf50b
    Scenario: healthcheck succeeds with its declared contract
      Given required arguments no required arguments
      And optional arguments no optional arguments
      And declared defaults no declared defaults
      And policy scope local operator filesystem/configuration permissions
      When an authorized client invokes healthcheck
      Then the response exposes exit 0 when TCP service accepts connection
      And side effects are none
      And idempotency is not_applicable
      And pagination or range behavior is none

    @surface-8cf2b70cf50b_role_reader
    Scenario: healthcheck as reader
      Given the canonical reader profile
      When that principal uses healthcheck
      Then the outcome is local_operator_not_role_scoped

    @surface-8cf2b70cf50b_role_proposer
    Scenario: healthcheck as proposer
      Given the canonical proposer profile
      When that principal uses healthcheck
      Then the outcome is local_operator_not_role_scoped

    @surface-8cf2b70cf50b_role_curator
    Scenario: healthcheck as curator
      Given the canonical curator profile
      When that principal uses healthcheck
      Then the outcome is local_operator_not_role_scoped

    @surface-8cf2b70cf50b_role_admin
    Scenario: healthcheck as admin
      Given the canonical admin profile
      When that principal uses healthcheck
      Then the outcome is local_operator_not_role_scoped

    @surface-8cf2b70cf50b_failure
    Scenario: healthcheck rejects invalid or conflicting requests
      Given required arguments no required arguments and declared defaults no declared defaults
      And malformed, missing, out-of-scope, unavailable, replayed, or conflicting inputs
      When the client invokes healthcheck
      Then one of the specified failures is usage error exits 2; runtime/configuration failure exits 1
      And failed pre-publication calls do not apply none
