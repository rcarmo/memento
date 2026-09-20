Feature: cli/operations

  Each operation has a success contract, four canonical role outcomes, and a failure contract.

  Rule: cli workflow

    @surface-39eaef7c37c5 @go_TestRunSyntax @go_TestRunStatus @go_TestRunDream
    Scenario: serve preserves the Python success contract
      Given the Python request contract CLI flags/config/environment
      When an authorized client invokes serve
      Then the response matches configured daemon/stdio or HTTP

    @surface-39eaef7c37c5_role_reader @go_TestRunSyntax @go_TestRunStatus @go_TestRunDream
    Scenario: serve as reader
      Given the canonical reader profile
      When that principal discovers or invokes serve
      Then the Python outcome is local_operator_not_role_scoped

    @surface-39eaef7c37c5_role_proposer @go_TestRunSyntax @go_TestRunStatus @go_TestRunDream
    Scenario: serve as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes serve
      Then the Python outcome is local_operator_not_role_scoped

    @surface-39eaef7c37c5_role_curator @go_TestRunSyntax @go_TestRunStatus @go_TestRunDream
    Scenario: serve as curator
      Given the canonical curator profile
      When that principal discovers or invokes serve
      Then the Python outcome is local_operator_not_role_scoped

    @surface-39eaef7c37c5_role_admin @go_TestRunSyntax @go_TestRunStatus @go_TestRunDream
    Scenario: serve as admin
      Given the canonical admin profile
      When that principal discovers or invokes serve
      Then the Python outcome is local_operator_not_role_scoped

    @surface-39eaef7c37c5_failure @go_TestRunSyntax @go_TestRunStatus @go_TestRunDream
    Scenario: serve preserves Python validation and failure behavior
      Given malformed, missing, out-of-scope, or unavailable inputs
      When the client invokes serve
      Then usage exit 2, runtime/validation failure exit 1

    @surface-43009273020f @go_TestRunSyntax @go_TestRunStatus @go_TestRunDream
    Scenario: audit preserves the Python success contract
      Given the Python request contract CLI flags/config/environment
      When an authorized client invokes audit
      Then the response matches repository issues and policy warnings

    @surface-43009273020f_role_reader @go_TestRunSyntax @go_TestRunStatus @go_TestRunDream
    Scenario: audit as reader
      Given the canonical reader profile
      When that principal discovers or invokes audit
      Then the Python outcome is local_operator_not_role_scoped

    @surface-43009273020f_role_proposer @go_TestRunSyntax @go_TestRunStatus @go_TestRunDream
    Scenario: audit as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes audit
      Then the Python outcome is local_operator_not_role_scoped

    @surface-43009273020f_role_curator @go_TestRunSyntax @go_TestRunStatus @go_TestRunDream
    Scenario: audit as curator
      Given the canonical curator profile
      When that principal discovers or invokes audit
      Then the Python outcome is local_operator_not_role_scoped

    @surface-43009273020f_role_admin @go_TestRunSyntax @go_TestRunStatus @go_TestRunDream
    Scenario: audit as admin
      Given the canonical admin profile
      When that principal discovers or invokes audit
      Then the Python outcome is local_operator_not_role_scoped

    @surface-43009273020f_failure @go_TestRunSyntax @go_TestRunStatus @go_TestRunDream
    Scenario: audit preserves Python validation and failure behavior
      Given malformed, missing, out-of-scope, or unavailable inputs
      When the client invokes audit
      Then usage exit 2, runtime/validation failure exit 1

    @surface-00d140452a2a @go_TestRunSyntax @go_TestRunStatus @go_TestRunDream
    Scenario: rebuild-index preserves the Python success contract
      Given the Python request contract CLI flags/config/environment
      When an authorized client invokes rebuild-index
      Then the response matches live/temporary parity report

    @surface-00d140452a2a_role_reader @go_TestRunSyntax @go_TestRunStatus @go_TestRunDream
    Scenario: rebuild-index as reader
      Given the canonical reader profile
      When that principal discovers or invokes rebuild-index
      Then the Python outcome is local_operator_not_role_scoped

    @surface-00d140452a2a_role_proposer @go_TestRunSyntax @go_TestRunStatus @go_TestRunDream
    Scenario: rebuild-index as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes rebuild-index
      Then the Python outcome is local_operator_not_role_scoped

    @surface-00d140452a2a_role_curator @go_TestRunSyntax @go_TestRunStatus @go_TestRunDream
    Scenario: rebuild-index as curator
      Given the canonical curator profile
      When that principal discovers or invokes rebuild-index
      Then the Python outcome is local_operator_not_role_scoped

    @surface-00d140452a2a_role_admin @go_TestRunSyntax @go_TestRunStatus @go_TestRunDream
    Scenario: rebuild-index as admin
      Given the canonical admin profile
      When that principal discovers or invokes rebuild-index
      Then the Python outcome is local_operator_not_role_scoped

    @surface-00d140452a2a_failure @go_TestRunSyntax @go_TestRunStatus @go_TestRunDream
    Scenario: rebuild-index preserves Python validation and failure behavior
      Given malformed, missing, out-of-scope, or unavailable inputs
      When the client invokes rebuild-index
      Then usage exit 2, runtime/validation failure exit 1

    @surface-09741af9a2d6 @go_TestRunSyntax @go_TestRunStatus @go_TestRunDream
    Scenario: backup preserves the Python success contract
      Given the Python request contract CLI flags/config/environment
      When an authorized client invokes backup
      Then the response matches deterministic archive

    @surface-09741af9a2d6_role_reader @go_TestRunSyntax @go_TestRunStatus @go_TestRunDream
    Scenario: backup as reader
      Given the canonical reader profile
      When that principal discovers or invokes backup
      Then the Python outcome is local_operator_not_role_scoped

    @surface-09741af9a2d6_role_proposer @go_TestRunSyntax @go_TestRunStatus @go_TestRunDream
    Scenario: backup as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes backup
      Then the Python outcome is local_operator_not_role_scoped

    @surface-09741af9a2d6_role_curator @go_TestRunSyntax @go_TestRunStatus @go_TestRunDream
    Scenario: backup as curator
      Given the canonical curator profile
      When that principal discovers or invokes backup
      Then the Python outcome is local_operator_not_role_scoped

    @surface-09741af9a2d6_role_admin @go_TestRunSyntax @go_TestRunStatus @go_TestRunDream
    Scenario: backup as admin
      Given the canonical admin profile
      When that principal discovers or invokes backup
      Then the Python outcome is local_operator_not_role_scoped

    @surface-09741af9a2d6_failure @go_TestRunSyntax @go_TestRunStatus @go_TestRunDream
    Scenario: backup preserves Python validation and failure behavior
      Given malformed, missing, out-of-scope, or unavailable inputs
      When the client invokes backup
      Then usage exit 2, runtime/validation failure exit 1

    @surface-6cc618194fbc @go_TestRunSyntax @go_TestRunStatus @go_TestRunDream
    Scenario: restore preserves the Python success contract
      Given the Python request contract CLI flags/config/environment
      When an authorized client invokes restore
      Then the response matches verified safe restore

    @surface-6cc618194fbc_role_reader @go_TestRunSyntax @go_TestRunStatus @go_TestRunDream
    Scenario: restore as reader
      Given the canonical reader profile
      When that principal discovers or invokes restore
      Then the Python outcome is local_operator_not_role_scoped

    @surface-6cc618194fbc_role_proposer @go_TestRunSyntax @go_TestRunStatus @go_TestRunDream
    Scenario: restore as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes restore
      Then the Python outcome is local_operator_not_role_scoped

    @surface-6cc618194fbc_role_curator @go_TestRunSyntax @go_TestRunStatus @go_TestRunDream
    Scenario: restore as curator
      Given the canonical curator profile
      When that principal discovers or invokes restore
      Then the Python outcome is local_operator_not_role_scoped

    @surface-6cc618194fbc_role_admin @go_TestRunSyntax @go_TestRunStatus @go_TestRunDream
    Scenario: restore as admin
      Given the canonical admin profile
      When that principal discovers or invokes restore
      Then the Python outcome is local_operator_not_role_scoped

    @surface-6cc618194fbc_failure @go_TestRunSyntax @go_TestRunStatus @go_TestRunDream
    Scenario: restore preserves Python validation and failure behavior
      Given malformed, missing, out-of-scope, or unavailable inputs
      When the client invokes restore
      Then usage exit 2, runtime/validation failure exit 1

    @surface-fed523668854 @go_TestRunSyntax @go_TestRunStatus @go_TestRunDream
    Scenario: status preserves the Python success contract
      Given the Python request contract CLI flags/config/environment
      When an authorized client invokes status
      Then the response matches JSON/Prometheus/Graphite operational state

    @surface-fed523668854_role_reader @go_TestRunSyntax @go_TestRunStatus @go_TestRunDream
    Scenario: status as reader
      Given the canonical reader profile
      When that principal discovers or invokes status
      Then the Python outcome is local_operator_not_role_scoped

    @surface-fed523668854_role_proposer @go_TestRunSyntax @go_TestRunStatus @go_TestRunDream
    Scenario: status as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes status
      Then the Python outcome is local_operator_not_role_scoped

    @surface-fed523668854_role_curator @go_TestRunSyntax @go_TestRunStatus @go_TestRunDream
    Scenario: status as curator
      Given the canonical curator profile
      When that principal discovers or invokes status
      Then the Python outcome is local_operator_not_role_scoped

    @surface-fed523668854_role_admin @go_TestRunSyntax @go_TestRunStatus @go_TestRunDream
    Scenario: status as admin
      Given the canonical admin profile
      When that principal discovers or invokes status
      Then the Python outcome is local_operator_not_role_scoped

    @surface-fed523668854_failure @go_TestRunSyntax @go_TestRunStatus @go_TestRunDream
    Scenario: status preserves Python validation and failure behavior
      Given malformed, missing, out-of-scope, or unavailable inputs
      When the client invokes status
      Then usage exit 2, runtime/validation failure exit 1

    @surface-1354f1967675 @go_TestRunSyntax @go_TestRunStatus @go_TestRunDream
    Scenario: dream preserves the Python success contract
      Given the Python request contract CLI flags/config/environment
      When an authorized client invokes dream
      Then the response matches report/propose scheduler run

    @surface-1354f1967675_role_reader @go_TestRunSyntax @go_TestRunStatus @go_TestRunDream
    Scenario: dream as reader
      Given the canonical reader profile
      When that principal discovers or invokes dream
      Then the Python outcome is local_operator_not_role_scoped

    @surface-1354f1967675_role_proposer @go_TestRunSyntax @go_TestRunStatus @go_TestRunDream
    Scenario: dream as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes dream
      Then the Python outcome is local_operator_not_role_scoped

    @surface-1354f1967675_role_curator @go_TestRunSyntax @go_TestRunStatus @go_TestRunDream
    Scenario: dream as curator
      Given the canonical curator profile
      When that principal discovers or invokes dream
      Then the Python outcome is local_operator_not_role_scoped

    @surface-1354f1967675_role_admin @go_TestRunSyntax @go_TestRunStatus @go_TestRunDream
    Scenario: dream as admin
      Given the canonical admin profile
      When that principal discovers or invokes dream
      Then the Python outcome is local_operator_not_role_scoped

    @surface-1354f1967675_failure @go_TestRunSyntax @go_TestRunStatus @go_TestRunDream
    Scenario: dream preserves Python validation and failure behavior
      Given malformed, missing, out-of-scope, or unavailable inputs
      When the client invokes dream
      Then usage exit 2, runtime/validation failure exit 1

    @surface-0334e05ca7bd @go_TestRunSyntax @go_TestRunStatus @go_TestRunDream
    Scenario: rotate-master-key preserves the Python success contract
      Given the Python request contract CLI flags/config/environment
      When an authorized client invokes rotate-master-key
      Then the response matches offline verifier rewrap

    @surface-0334e05ca7bd_role_reader @go_TestRunSyntax @go_TestRunStatus @go_TestRunDream
    Scenario: rotate-master-key as reader
      Given the canonical reader profile
      When that principal discovers or invokes rotate-master-key
      Then the Python outcome is local_operator_not_role_scoped

    @surface-0334e05ca7bd_role_proposer @go_TestRunSyntax @go_TestRunStatus @go_TestRunDream
    Scenario: rotate-master-key as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes rotate-master-key
      Then the Python outcome is local_operator_not_role_scoped

    @surface-0334e05ca7bd_role_curator @go_TestRunSyntax @go_TestRunStatus @go_TestRunDream
    Scenario: rotate-master-key as curator
      Given the canonical curator profile
      When that principal discovers or invokes rotate-master-key
      Then the Python outcome is local_operator_not_role_scoped

    @surface-0334e05ca7bd_role_admin @go_TestRunSyntax @go_TestRunStatus @go_TestRunDream
    Scenario: rotate-master-key as admin
      Given the canonical admin profile
      When that principal discovers or invokes rotate-master-key
      Then the Python outcome is local_operator_not_role_scoped

    @surface-0334e05ca7bd_failure @go_TestRunSyntax @go_TestRunStatus @go_TestRunDream
    Scenario: rotate-master-key preserves Python validation and failure behavior
      Given malformed, missing, out-of-scope, or unavailable inputs
      When the client invokes rotate-master-key
      Then usage exit 2, runtime/validation failure exit 1

    @surface-8cf2b70cf50b @go_TestHealthcheckCloseFailure
    Scenario: healthcheck preserves the Python success contract
      Given the Python request contract CLI flags/config/environment
      When an authorized client invokes healthcheck
      Then the response matches Go-only native health probe

    @surface-8cf2b70cf50b_role_reader @go_TestHealthcheckCloseFailure
    Scenario: healthcheck as reader
      Given the canonical reader profile
      When that principal discovers or invokes healthcheck
      Then the Python outcome is local_operator_not_role_scoped

    @surface-8cf2b70cf50b_role_proposer @go_TestHealthcheckCloseFailure
    Scenario: healthcheck as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes healthcheck
      Then the Python outcome is local_operator_not_role_scoped

    @surface-8cf2b70cf50b_role_curator @go_TestHealthcheckCloseFailure
    Scenario: healthcheck as curator
      Given the canonical curator profile
      When that principal discovers or invokes healthcheck
      Then the Python outcome is local_operator_not_role_scoped

    @surface-8cf2b70cf50b_role_admin @go_TestHealthcheckCloseFailure
    Scenario: healthcheck as admin
      Given the canonical admin profile
      When that principal discovers or invokes healthcheck
      Then the Python outcome is local_operator_not_role_scoped

    @surface-8cf2b70cf50b_failure @go_TestHealthcheckCloseFailure
    Scenario: healthcheck preserves Python validation and failure behavior
      Given malformed, missing, out-of-scope, or unavailable inputs
      When the client invokes healthcheck
      Then usage exit 2, runtime/validation failure exit 1
