Feature: http/staging

  Each operation has a success contract, four canonical role outcomes, and a failure contract.

  Rule: http endpoint workflow

    @surface-29d4cebf9364 @go_TestStagingHTTPReference
    Scenario: POST /assets/staging/upload preserves the Python success contract
      Given the Python request contract POST request with route-specific JSON/headers
      When an authorized client invokes POST /assets/staging/upload
      Then the response matches staged asset, 201/200 replay

    @surface-29d4cebf9364_role_reader @go_TestStagingHTTPReference
    Scenario: POST /assets/staging/upload as reader
      Given the canonical reader profile
      When that principal discovers or invokes POST /assets/staging/upload
      Then the Python outcome is route_available_with_route_specific_policy

    @surface-29d4cebf9364_role_proposer @go_TestStagingHTTPReference
    Scenario: POST /assets/staging/upload as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes POST /assets/staging/upload
      Then the Python outcome is route_available_with_route_specific_policy

    @surface-29d4cebf9364_role_curator @go_TestStagingHTTPReference
    Scenario: POST /assets/staging/upload as curator
      Given the canonical curator profile
      When that principal discovers or invokes POST /assets/staging/upload
      Then the Python outcome is route_available_with_route_specific_policy

    @surface-29d4cebf9364_role_admin @go_TestStagingHTTPReference
    Scenario: POST /assets/staging/upload as admin
      Given the canonical admin profile
      When that principal discovers or invokes POST /assets/staging/upload
      Then the Python outcome is route_available_with_route_specific_policy

    @surface-29d4cebf9364_failure @go_TestStagingHTTPReference
    Scenario: POST /assets/staging/upload preserves Python validation and failure behavior
      Given malformed, missing, out-of-scope, or unavailable inputs
      When the client invokes POST /assets/staging/upload
      Then 400 validation, 401 auth, 403 policy where applicable, 404 unknown, 405 method, 413/415 upload bounds, 503 unavailable

    @surface-a2ae439ee3df @go_TestStagingHTTPReference
    Scenario: POST /assets/staging preserves the Python success contract
      Given the Python request contract POST request with route-specific JSON/headers
      When an authorized client invokes POST /assets/staging
      Then the response matches upload ticket/staged asset

    @surface-a2ae439ee3df_role_reader @go_TestStagingHTTPReference
    Scenario: POST /assets/staging as reader
      Given the canonical reader profile
      When that principal discovers or invokes POST /assets/staging
      Then the Python outcome is http_401_or_403

    @surface-a2ae439ee3df_role_proposer @go_TestStagingHTTPReference
    Scenario: POST /assets/staging as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes POST /assets/staging
      Then the Python outcome is allowed_or_owner_scoped

    @surface-a2ae439ee3df_role_curator @go_TestStagingHTTPReference
    Scenario: POST /assets/staging as curator
      Given the canonical curator profile
      When that principal discovers or invokes POST /assets/staging
      Then the Python outcome is allowed_or_owner_scoped

    @surface-a2ae439ee3df_role_admin @go_TestStagingHTTPReference
    Scenario: POST /assets/staging as admin
      Given the canonical admin profile
      When that principal discovers or invokes POST /assets/staging
      Then the Python outcome is allowed_or_owner_scoped

    @surface-a2ae439ee3df_failure @go_TestStagingHTTPReference
    Scenario: POST /assets/staging preserves Python validation and failure behavior
      Given malformed, missing, out-of-scope, or unavailable inputs
      When the client invokes POST /assets/staging
      Then 400 validation, 401 auth, 403 policy where applicable, 404 unknown, 405 method, 413/415 upload bounds, 503 unavailable

    @surface-ea179a4b618b @go_TestStagingHTTPReference
    Scenario: GET /assets/staging/{id} preserves the Python success contract
      Given the Python request contract GET request with route-specific JSON/headers
      When an authorized client invokes GET /assets/staging/{id}
      Then the response matches staged asset status

    @surface-ea179a4b618b_role_reader @go_TestStagingHTTPReference
    Scenario: GET /assets/staging/{id} as reader
      Given the canonical reader profile
      When that principal discovers or invokes GET /assets/staging/{id}
      Then the Python outcome is http_401_or_403

    @surface-ea179a4b618b_role_proposer @go_TestStagingHTTPReference
    Scenario: GET /assets/staging/{id} as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes GET /assets/staging/{id}
      Then the Python outcome is allowed_or_owner_scoped

    @surface-ea179a4b618b_role_curator @go_TestStagingHTTPReference
    Scenario: GET /assets/staging/{id} as curator
      Given the canonical curator profile
      When that principal discovers or invokes GET /assets/staging/{id}
      Then the Python outcome is allowed_or_owner_scoped

    @surface-ea179a4b618b_role_admin @go_TestStagingHTTPReference
    Scenario: GET /assets/staging/{id} as admin
      Given the canonical admin profile
      When that principal discovers or invokes GET /assets/staging/{id}
      Then the Python outcome is allowed_or_owner_scoped

    @surface-ea179a4b618b_failure @go_TestStagingHTTPReference
    Scenario: GET /assets/staging/{id} preserves Python validation and failure behavior
      Given malformed, missing, out-of-scope, or unavailable inputs
      When the client invokes GET /assets/staging/{id}
      Then 400 validation, 401 auth, 403 policy where applicable, 404 unknown, 405 method, 413/415 upload bounds, 503 unavailable
