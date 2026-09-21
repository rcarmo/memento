Feature: http/staging

  Each operation has a success contract, four canonical role outcomes, and a failure contract.
  Stable row tags link behavior to validation data without exposing implementation details.

  Rule: http endpoint workflow

    @surface-29d4cebf9364
    Scenario: POST /assets/staging/upload succeeds with its declared contract
      Given required arguments no required arguments
      And optional arguments no optional arguments
      And declared defaults no declared defaults
      And policy scope one-time upload ticket
      When an authorized client invokes POST /assets/staging/upload
      Then the response exposes HTTP 201 new or 200 exact replay, staged_asset_id, state, sha256, size_bytes
      And side effects are queues or persists the endpoint-specific staging, refresh, access, or session transition
      And idempotency is not_applicable
      And pagination or range behavior is none

    @surface-29d4cebf9364_role_reader
    Scenario: POST /assets/staging/upload as reader
      Given the canonical reader profile
      When that principal uses POST /assets/staging/upload
      Then the outcome is upload ticket authenticates the bounded application/zip request; ordinary bearer role is not used here

    @surface-29d4cebf9364_role_proposer
    Scenario: POST /assets/staging/upload as proposer
      Given the canonical proposer profile
      When that principal uses POST /assets/staging/upload
      Then the outcome is upload ticket authenticates the bounded application/zip request; ordinary bearer role is not used here

    @surface-29d4cebf9364_role_curator
    Scenario: POST /assets/staging/upload as curator
      Given the canonical curator profile
      When that principal uses POST /assets/staging/upload
      Then the outcome is upload ticket authenticates the bounded application/zip request; ordinary bearer role is not used here

    @surface-29d4cebf9364_role_admin
    Scenario: POST /assets/staging/upload as admin
      Given the canonical admin profile
      When that principal uses POST /assets/staging/upload
      Then the outcome is upload ticket authenticates the bounded application/zip request; ordinary bearer role is not used here

    @surface-29d4cebf9364_failure
    Scenario: POST /assets/staging/upload rejects invalid or conflicting requests
      Given required arguments no required arguments and declared defaults no declared defaults
      And malformed, missing, out-of-scope, unavailable, replayed, or conflicting inputs
      When the client invokes POST /assets/staging/upload
      Then one of the specified failures is 400 validation, 401 auth, 403 policy where applicable, 404 unknown, 405 method, 413/415 upload bounds, 503 unavailable
      And failed pre-publication calls do not apply queues or persists the endpoint-specific staging, refresh, access, or session transition

    @surface-a2ae439ee3df
    Scenario: POST /assets/staging succeeds with its declared contract
      Given required arguments no required arguments
      And optional arguments no optional arguments
      And declared defaults no declared defaults
      And policy scope proposer
      When an authorized client invokes POST /assets/staging
      Then the response exposes HTTP 201 new or 200 exact replay, idempotency_key, upload_ticket, upload_url, expires_at, state
      And side effects are queues or persists the endpoint-specific staging, refresh, access, or session transition
      And idempotency is not_applicable
      And pagination or range behavior is none

    @surface-a2ae439ee3df_role_reader
    Scenario: POST /assets/staging as reader
      Given the canonical reader profile
      When that principal uses POST /assets/staging
      Then the outcome is HTTP 401/403 proposer role required

    @surface-a2ae439ee3df_role_proposer
    Scenario: POST /assets/staging as proposer
      Given the canonical proposer profile
      When that principal uses POST /assets/staging
      Then the outcome is authenticated proposer-scoped staging response

    @surface-a2ae439ee3df_role_curator
    Scenario: POST /assets/staging as curator
      Given the canonical curator profile
      When that principal uses POST /assets/staging
      Then the outcome is authenticated proposer-scoped staging response

    @surface-a2ae439ee3df_role_admin
    Scenario: POST /assets/staging as admin
      Given the canonical admin profile
      When that principal uses POST /assets/staging
      Then the outcome is authenticated proposer-scoped staging response

    @surface-a2ae439ee3df_failure
    Scenario: POST /assets/staging rejects invalid or conflicting requests
      Given required arguments no required arguments and declared defaults no declared defaults
      And malformed, missing, out-of-scope, unavailable, replayed, or conflicting inputs
      When the client invokes POST /assets/staging
      Then one of the specified failures is 400 validation, 401 auth, 403 policy where applicable, 404 unknown, 405 method, 413/415 upload bounds, 503 unavailable
      And failed pre-publication calls do not apply queues or persists the endpoint-specific staging, refresh, access, or session transition

    @surface-ea179a4b618b
    Scenario: GET /assets/staging/{id} succeeds with its declared contract
      Given required arguments no required arguments
      And optional arguments no optional arguments
      And declared defaults no declared defaults
      And policy scope owner
      When an authorized client invokes GET /assets/staging/{id}
      Then the response exposes HTTP 200, staged_asset_id, state, owner, asset_kind, version, expires_at
      And side effects are none
      And idempotency is not_applicable
      And pagination or range behavior is none

    @surface-ea179a4b618b_role_reader
    Scenario: GET /assets/staging/{id} as reader
      Given the canonical reader profile
      When that principal uses GET /assets/staging/{id}
      Then the outcome is HTTP 401/403 unless the principal owns the staged asset

    @surface-ea179a4b618b_role_proposer
    Scenario: GET /assets/staging/{id} as proposer
      Given the canonical proposer profile
      When that principal uses GET /assets/staging/{id}
      Then the outcome is owner-scoped staged asset status

    @surface-ea179a4b618b_role_curator
    Scenario: GET /assets/staging/{id} as curator
      Given the canonical curator profile
      When that principal uses GET /assets/staging/{id}
      Then the outcome is owner-scoped staged asset status

    @surface-ea179a4b618b_role_admin
    Scenario: GET /assets/staging/{id} as admin
      Given the canonical admin profile
      When that principal uses GET /assets/staging/{id}
      Then the outcome is owner-scoped staged asset status

    @surface-ea179a4b618b_failure
    Scenario: GET /assets/staging/{id} rejects invalid or conflicting requests
      Given required arguments no required arguments and declared defaults no declared defaults
      And malformed, missing, out-of-scope, unavailable, replayed, or conflicting inputs
      When the client invokes GET /assets/staging/{id}
      Then one of the specified failures is 400 validation, 401 auth, 403 policy where applicable, 404 unknown, 405 method, 413/415 upload bounds, 503 unavailable
      And failed pre-publication calls do not apply none
