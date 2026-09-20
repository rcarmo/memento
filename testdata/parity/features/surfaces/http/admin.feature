Feature: http/admin

  Each operation has a success contract, four canonical role outcomes, and a failure contract.

  Rule: http endpoint workflow

    @surface-e2485ab26cbb @go_TestAdminHTTPThroughUMCP
    Scenario: GET /admin preserves the Python success contract
      Given the Python request contract GET request with route-specific JSON/headers
      When an authorized client invokes GET /admin
      Then the response matches HTML no-store/nosniff

    @surface-e2485ab26cbb_role_reader @go_TestAdminHTTPThroughUMCP
    Scenario: GET /admin as reader
      Given the canonical reader profile
      When that principal discovers or invokes GET /admin
      Then the Python outcome is route_available_with_route_specific_policy

    @surface-e2485ab26cbb_role_proposer @go_TestAdminHTTPThroughUMCP
    Scenario: GET /admin as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes GET /admin
      Then the Python outcome is route_available_with_route_specific_policy

    @surface-e2485ab26cbb_role_curator @go_TestAdminHTTPThroughUMCP
    Scenario: GET /admin as curator
      Given the canonical curator profile
      When that principal discovers or invokes GET /admin
      Then the Python outcome is route_available_with_route_specific_policy

    @surface-e2485ab26cbb_role_admin @go_TestAdminHTTPThroughUMCP
    Scenario: GET /admin as admin
      Given the canonical admin profile
      When that principal discovers or invokes GET /admin
      Then the Python outcome is route_available_with_route_specific_policy

    @surface-e2485ab26cbb_failure @go_TestAdminHTTPThroughUMCP
    Scenario: GET /admin preserves Python validation and failure behavior
      Given malformed, missing, out-of-scope, or unavailable inputs
      When the client invokes GET /admin
      Then 400 validation, 401 auth, 403 policy where applicable, 404 unknown, 405 method, 413/415 upload bounds, 503 unavailable

    @surface-2c59ca52a630 @go_TestAdminHTTPThroughUMCP
    Scenario: GET /admin/app.js preserves the Python success contract
      Given the Python request contract GET request with route-specific JSON/headers
      When an authorized client invokes GET /admin/app.js
      Then the response matches JavaScript no-store/nosniff

    @surface-2c59ca52a630_role_reader @go_TestAdminHTTPThroughUMCP
    Scenario: GET /admin/app.js as reader
      Given the canonical reader profile
      When that principal discovers or invokes GET /admin/app.js
      Then the Python outcome is route_available_with_route_specific_policy

    @surface-2c59ca52a630_role_proposer @go_TestAdminHTTPThroughUMCP
    Scenario: GET /admin/app.js as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes GET /admin/app.js
      Then the Python outcome is route_available_with_route_specific_policy

    @surface-2c59ca52a630_role_curator @go_TestAdminHTTPThroughUMCP
    Scenario: GET /admin/app.js as curator
      Given the canonical curator profile
      When that principal discovers or invokes GET /admin/app.js
      Then the Python outcome is route_available_with_route_specific_policy

    @surface-2c59ca52a630_role_admin @go_TestAdminHTTPThroughUMCP
    Scenario: GET /admin/app.js as admin
      Given the canonical admin profile
      When that principal discovers or invokes GET /admin/app.js
      Then the Python outcome is route_available_with_route_specific_policy

    @surface-2c59ca52a630_failure @go_TestAdminHTTPThroughUMCP
    Scenario: GET /admin/app.js preserves Python validation and failure behavior
      Given malformed, missing, out-of-scope, or unavailable inputs
      When the client invokes GET /admin/app.js
      Then 400 validation, 401 auth, 403 policy where applicable, 404 unknown, 405 method, 413/415 upload bounds, 503 unavailable

    @surface-527a5f06b0d9 @go_TestAdminHTTPHandleCRUDAndActivity
    Scenario: GET /admin/api/principals preserves the Python success contract
      Given the Python request contract GET request with route-specific JSON/headers
      When an authorized client invokes GET /admin/api/principals
      Then the response matches principal list with broad-read warnings

    @surface-527a5f06b0d9_role_reader @go_TestAdminHTTPHandleCRUDAndActivity
    Scenario: GET /admin/api/principals as reader
      Given the canonical reader profile
      When that principal discovers or invokes GET /admin/api/principals
      Then the Python outcome is http_401

    @surface-527a5f06b0d9_role_proposer @go_TestAdminHTTPHandleCRUDAndActivity
    Scenario: GET /admin/api/principals as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes GET /admin/api/principals
      Then the Python outcome is http_401

    @surface-527a5f06b0d9_role_curator @go_TestAdminHTTPHandleCRUDAndActivity
    Scenario: GET /admin/api/principals as curator
      Given the canonical curator profile
      When that principal discovers or invokes GET /admin/api/principals
      Then the Python outcome is http_401

    @surface-527a5f06b0d9_role_admin @go_TestAdminHTTPHandleCRUDAndActivity
    Scenario: GET /admin/api/principals as admin
      Given the canonical admin profile
      When that principal discovers or invokes GET /admin/api/principals
      Then the Python outcome is allowed

    @surface-527a5f06b0d9_failure @go_TestAdminHTTPHandleCRUDAndActivity
    Scenario: GET /admin/api/principals preserves Python validation and failure behavior
      Given malformed, missing, out-of-scope, or unavailable inputs
      When the client invokes GET /admin/api/principals
      Then 400 validation, 401 auth, 403 policy where applicable, 404 unknown, 405 method, 413/415 upload bounds, 503 unavailable

    @surface-9308312ce583 @go_TestAdminHTTPHandleCRUDAndActivity
    Scenario: GET /admin/api/activity preserves the Python success contract
      Given the Python request contract GET request with route-specific JSON/headers
      When an authorized client invokes GET /admin/api/activity
      Then the response matches latest 50 access events

    @surface-9308312ce583_role_reader @go_TestAdminHTTPHandleCRUDAndActivity
    Scenario: GET /admin/api/activity as reader
      Given the canonical reader profile
      When that principal discovers or invokes GET /admin/api/activity
      Then the Python outcome is http_401

    @surface-9308312ce583_role_proposer @go_TestAdminHTTPHandleCRUDAndActivity
    Scenario: GET /admin/api/activity as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes GET /admin/api/activity
      Then the Python outcome is http_401

    @surface-9308312ce583_role_curator @go_TestAdminHTTPHandleCRUDAndActivity
    Scenario: GET /admin/api/activity as curator
      Given the canonical curator profile
      When that principal discovers or invokes GET /admin/api/activity
      Then the Python outcome is http_401

    @surface-9308312ce583_role_admin @go_TestAdminHTTPHandleCRUDAndActivity
    Scenario: GET /admin/api/activity as admin
      Given the canonical admin profile
      When that principal discovers or invokes GET /admin/api/activity
      Then the Python outcome is allowed

    @surface-9308312ce583_failure @go_TestAdminHTTPHandleCRUDAndActivity
    Scenario: GET /admin/api/activity preserves Python validation and failure behavior
      Given malformed, missing, out-of-scope, or unavailable inputs
      When the client invokes GET /admin/api/activity
      Then 400 validation, 401 auth, 403 policy where applicable, 404 unknown, 405 method, 413/415 upload bounds, 503 unavailable

    @surface-a7495156c5e1 @go_TestAdminHTTPHandleCRUDAndActivity
    Scenario: POST /admin/api/principals preserves the Python success contract
      Given the Python request contract POST request with route-specific JSON/headers
      When an authorized client invokes POST /admin/api/principals
      Then the response matches principal plus one-time credential, 201

    @surface-a7495156c5e1_role_reader @go_TestAdminHTTPHandleCRUDAndActivity
    Scenario: POST /admin/api/principals as reader
      Given the canonical reader profile
      When that principal discovers or invokes POST /admin/api/principals
      Then the Python outcome is http_401

    @surface-a7495156c5e1_role_proposer @go_TestAdminHTTPHandleCRUDAndActivity
    Scenario: POST /admin/api/principals as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes POST /admin/api/principals
      Then the Python outcome is http_401

    @surface-a7495156c5e1_role_curator @go_TestAdminHTTPHandleCRUDAndActivity
    Scenario: POST /admin/api/principals as curator
      Given the canonical curator profile
      When that principal discovers or invokes POST /admin/api/principals
      Then the Python outcome is http_401

    @surface-a7495156c5e1_role_admin @go_TestAdminHTTPHandleCRUDAndActivity
    Scenario: POST /admin/api/principals as admin
      Given the canonical admin profile
      When that principal discovers or invokes POST /admin/api/principals
      Then the Python outcome is allowed

    @surface-a7495156c5e1_failure @go_TestAdminHTTPHandleCRUDAndActivity
    Scenario: POST /admin/api/principals preserves Python validation and failure behavior
      Given malformed, missing, out-of-scope, or unavailable inputs
      When the client invokes POST /admin/api/principals
      Then 400 validation, 401 auth, 403 policy where applicable, 404 unknown, 405 method, 413/415 upload bounds, 503 unavailable

    @surface-30b7e1ddfa50 @go_TestAdminHTTPHandleCRUDAndActivity
    Scenario: POST /admin/api/principals/{name}/update preserves the Python success contract
      Given the Python request contract POST request with route-specific JSON/headers
      When an authorized client invokes POST /admin/api/principals/{name}/update
      Then the response matches updated principal or one-time credential

    @surface-30b7e1ddfa50_role_reader @go_TestAdminHTTPHandleCRUDAndActivity
    Scenario: POST /admin/api/principals/{name}/update as reader
      Given the canonical reader profile
      When that principal discovers or invokes POST /admin/api/principals/{name}/update
      Then the Python outcome is http_401

    @surface-30b7e1ddfa50_role_proposer @go_TestAdminHTTPHandleCRUDAndActivity
    Scenario: POST /admin/api/principals/{name}/update as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes POST /admin/api/principals/{name}/update
      Then the Python outcome is http_401

    @surface-30b7e1ddfa50_role_curator @go_TestAdminHTTPHandleCRUDAndActivity
    Scenario: POST /admin/api/principals/{name}/update as curator
      Given the canonical curator profile
      When that principal discovers or invokes POST /admin/api/principals/{name}/update
      Then the Python outcome is http_401

    @surface-30b7e1ddfa50_role_admin @go_TestAdminHTTPHandleCRUDAndActivity
    Scenario: POST /admin/api/principals/{name}/update as admin
      Given the canonical admin profile
      When that principal discovers or invokes POST /admin/api/principals/{name}/update
      Then the Python outcome is allowed

    @surface-30b7e1ddfa50_failure @go_TestAdminHTTPHandleCRUDAndActivity
    Scenario: POST /admin/api/principals/{name}/update preserves Python validation and failure behavior
      Given malformed, missing, out-of-scope, or unavailable inputs
      When the client invokes POST /admin/api/principals/{name}/update
      Then 400 validation, 401 auth, 403 policy where applicable, 404 unknown, 405 method, 413/415 upload bounds, 503 unavailable

    @surface-4c5e4a822549 @go_TestAdminHTTPHandleCRUDAndActivity
    Scenario: POST /admin/api/principals/{name}/rename preserves the Python success contract
      Given the Python request contract POST request with route-specific JSON/headers
      When an authorized client invokes POST /admin/api/principals/{name}/rename
      Then the response matches updated principal or one-time credential

    @surface-4c5e4a822549_role_reader @go_TestAdminHTTPHandleCRUDAndActivity
    Scenario: POST /admin/api/principals/{name}/rename as reader
      Given the canonical reader profile
      When that principal discovers or invokes POST /admin/api/principals/{name}/rename
      Then the Python outcome is http_401

    @surface-4c5e4a822549_role_proposer @go_TestAdminHTTPHandleCRUDAndActivity
    Scenario: POST /admin/api/principals/{name}/rename as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes POST /admin/api/principals/{name}/rename
      Then the Python outcome is http_401

    @surface-4c5e4a822549_role_curator @go_TestAdminHTTPHandleCRUDAndActivity
    Scenario: POST /admin/api/principals/{name}/rename as curator
      Given the canonical curator profile
      When that principal discovers or invokes POST /admin/api/principals/{name}/rename
      Then the Python outcome is http_401

    @surface-4c5e4a822549_role_admin @go_TestAdminHTTPHandleCRUDAndActivity
    Scenario: POST /admin/api/principals/{name}/rename as admin
      Given the canonical admin profile
      When that principal discovers or invokes POST /admin/api/principals/{name}/rename
      Then the Python outcome is allowed

    @surface-4c5e4a822549_failure @go_TestAdminHTTPHandleCRUDAndActivity
    Scenario: POST /admin/api/principals/{name}/rename preserves Python validation and failure behavior
      Given malformed, missing, out-of-scope, or unavailable inputs
      When the client invokes POST /admin/api/principals/{name}/rename
      Then 400 validation, 401 auth, 403 policy where applicable, 404 unknown, 405 method, 413/415 upload bounds, 503 unavailable

    @surface-c45eccf925c8 @go_TestAdminHTTPHandleCRUDAndActivity
    Scenario: POST /admin/api/principals/{name}/disable preserves the Python success contract
      Given the Python request contract POST request with route-specific JSON/headers
      When an authorized client invokes POST /admin/api/principals/{name}/disable
      Then the response matches updated principal or one-time credential

    @surface-c45eccf925c8_role_reader @go_TestAdminHTTPHandleCRUDAndActivity
    Scenario: POST /admin/api/principals/{name}/disable as reader
      Given the canonical reader profile
      When that principal discovers or invokes POST /admin/api/principals/{name}/disable
      Then the Python outcome is http_401

    @surface-c45eccf925c8_role_proposer @go_TestAdminHTTPHandleCRUDAndActivity
    Scenario: POST /admin/api/principals/{name}/disable as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes POST /admin/api/principals/{name}/disable
      Then the Python outcome is http_401

    @surface-c45eccf925c8_role_curator @go_TestAdminHTTPHandleCRUDAndActivity
    Scenario: POST /admin/api/principals/{name}/disable as curator
      Given the canonical curator profile
      When that principal discovers or invokes POST /admin/api/principals/{name}/disable
      Then the Python outcome is http_401

    @surface-c45eccf925c8_role_admin @go_TestAdminHTTPHandleCRUDAndActivity
    Scenario: POST /admin/api/principals/{name}/disable as admin
      Given the canonical admin profile
      When that principal discovers or invokes POST /admin/api/principals/{name}/disable
      Then the Python outcome is allowed

    @surface-c45eccf925c8_failure @go_TestAdminHTTPHandleCRUDAndActivity
    Scenario: POST /admin/api/principals/{name}/disable preserves Python validation and failure behavior
      Given malformed, missing, out-of-scope, or unavailable inputs
      When the client invokes POST /admin/api/principals/{name}/disable
      Then 400 validation, 401 auth, 403 policy where applicable, 404 unknown, 405 method, 413/415 upload bounds, 503 unavailable

    @surface-f8177e84f7ee @go_TestAdminHTTPHandleCRUDAndActivity
    Scenario: POST /admin/api/principals/{name}/enable preserves the Python success contract
      Given the Python request contract POST request with route-specific JSON/headers
      When an authorized client invokes POST /admin/api/principals/{name}/enable
      Then the response matches updated principal or one-time credential

    @surface-f8177e84f7ee_role_reader @go_TestAdminHTTPHandleCRUDAndActivity
    Scenario: POST /admin/api/principals/{name}/enable as reader
      Given the canonical reader profile
      When that principal discovers or invokes POST /admin/api/principals/{name}/enable
      Then the Python outcome is http_401

    @surface-f8177e84f7ee_role_proposer @go_TestAdminHTTPHandleCRUDAndActivity
    Scenario: POST /admin/api/principals/{name}/enable as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes POST /admin/api/principals/{name}/enable
      Then the Python outcome is http_401

    @surface-f8177e84f7ee_role_curator @go_TestAdminHTTPHandleCRUDAndActivity
    Scenario: POST /admin/api/principals/{name}/enable as curator
      Given the canonical curator profile
      When that principal discovers or invokes POST /admin/api/principals/{name}/enable
      Then the Python outcome is http_401

    @surface-f8177e84f7ee_role_admin @go_TestAdminHTTPHandleCRUDAndActivity
    Scenario: POST /admin/api/principals/{name}/enable as admin
      Given the canonical admin profile
      When that principal discovers or invokes POST /admin/api/principals/{name}/enable
      Then the Python outcome is allowed

    @surface-f8177e84f7ee_failure @go_TestAdminHTTPHandleCRUDAndActivity
    Scenario: POST /admin/api/principals/{name}/enable preserves Python validation and failure behavior
      Given malformed, missing, out-of-scope, or unavailable inputs
      When the client invokes POST /admin/api/principals/{name}/enable
      Then 400 validation, 401 auth, 403 policy where applicable, 404 unknown, 405 method, 413/415 upload bounds, 503 unavailable

    @surface-907e8a310559 @go_TestAdminHTTPHandleCRUDAndActivity
    Scenario: POST /admin/api/principals/{name}/rotate preserves the Python success contract
      Given the Python request contract POST request with route-specific JSON/headers
      When an authorized client invokes POST /admin/api/principals/{name}/rotate
      Then the response matches updated principal or one-time credential

    @surface-907e8a310559_role_reader @go_TestAdminHTTPHandleCRUDAndActivity
    Scenario: POST /admin/api/principals/{name}/rotate as reader
      Given the canonical reader profile
      When that principal discovers or invokes POST /admin/api/principals/{name}/rotate
      Then the Python outcome is http_401

    @surface-907e8a310559_role_proposer @go_TestAdminHTTPHandleCRUDAndActivity
    Scenario: POST /admin/api/principals/{name}/rotate as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes POST /admin/api/principals/{name}/rotate
      Then the Python outcome is http_401

    @surface-907e8a310559_role_curator @go_TestAdminHTTPHandleCRUDAndActivity
    Scenario: POST /admin/api/principals/{name}/rotate as curator
      Given the canonical curator profile
      When that principal discovers or invokes POST /admin/api/principals/{name}/rotate
      Then the Python outcome is http_401

    @surface-907e8a310559_role_admin @go_TestAdminHTTPHandleCRUDAndActivity
    Scenario: POST /admin/api/principals/{name}/rotate as admin
      Given the canonical admin profile
      When that principal discovers or invokes POST /admin/api/principals/{name}/rotate
      Then the Python outcome is allowed

    @surface-907e8a310559_failure @go_TestAdminHTTPHandleCRUDAndActivity
    Scenario: POST /admin/api/principals/{name}/rotate preserves Python validation and failure behavior
      Given malformed, missing, out-of-scope, or unavailable inputs
      When the client invokes POST /admin/api/principals/{name}/rotate
      Then 400 validation, 401 auth, 403 policy where applicable, 404 unknown, 405 method, 413/415 upload bounds, 503 unavailable

    @surface-72ace281970a @go_TestAdminHTTPHandleCRUDAndActivity
    Scenario: POST /admin/api/principals/{name}/revoke preserves the Python success contract
      Given the Python request contract POST request with route-specific JSON/headers
      When an authorized client invokes POST /admin/api/principals/{name}/revoke
      Then the response matches updated principal or one-time credential

    @surface-72ace281970a_role_reader @go_TestAdminHTTPHandleCRUDAndActivity
    Scenario: POST /admin/api/principals/{name}/revoke as reader
      Given the canonical reader profile
      When that principal discovers or invokes POST /admin/api/principals/{name}/revoke
      Then the Python outcome is http_401

    @surface-72ace281970a_role_proposer @go_TestAdminHTTPHandleCRUDAndActivity
    Scenario: POST /admin/api/principals/{name}/revoke as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes POST /admin/api/principals/{name}/revoke
      Then the Python outcome is http_401

    @surface-72ace281970a_role_curator @go_TestAdminHTTPHandleCRUDAndActivity
    Scenario: POST /admin/api/principals/{name}/revoke as curator
      Given the canonical curator profile
      When that principal discovers or invokes POST /admin/api/principals/{name}/revoke
      Then the Python outcome is http_401

    @surface-72ace281970a_role_admin @go_TestAdminHTTPHandleCRUDAndActivity
    Scenario: POST /admin/api/principals/{name}/revoke as admin
      Given the canonical admin profile
      When that principal discovers or invokes POST /admin/api/principals/{name}/revoke
      Then the Python outcome is allowed

    @surface-72ace281970a_failure @go_TestAdminHTTPHandleCRUDAndActivity
    Scenario: POST /admin/api/principals/{name}/revoke preserves Python validation and failure behavior
      Given malformed, missing, out-of-scope, or unavailable inputs
      When the client invokes POST /admin/api/principals/{name}/revoke
      Then 400 validation, 401 auth, 403 policy where applicable, 404 unknown, 405 method, 413/415 upload bounds, 503 unavailable

    @surface-43af9f6a33f7 @go_TestAdminHTTPHandleCRUDAndActivity
    Scenario: POST /admin/api/principals/{name}/delete preserves the Python success contract
      Given the Python request contract POST request with route-specific JSON/headers
      When an authorized client invokes POST /admin/api/principals/{name}/delete
      Then the response matches updated principal or one-time credential

    @surface-43af9f6a33f7_role_reader @go_TestAdminHTTPHandleCRUDAndActivity
    Scenario: POST /admin/api/principals/{name}/delete as reader
      Given the canonical reader profile
      When that principal discovers or invokes POST /admin/api/principals/{name}/delete
      Then the Python outcome is http_401

    @surface-43af9f6a33f7_role_proposer @go_TestAdminHTTPHandleCRUDAndActivity
    Scenario: POST /admin/api/principals/{name}/delete as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes POST /admin/api/principals/{name}/delete
      Then the Python outcome is http_401

    @surface-43af9f6a33f7_role_curator @go_TestAdminHTTPHandleCRUDAndActivity
    Scenario: POST /admin/api/principals/{name}/delete as curator
      Given the canonical curator profile
      When that principal discovers or invokes POST /admin/api/principals/{name}/delete
      Then the Python outcome is http_401

    @surface-43af9f6a33f7_role_admin @go_TestAdminHTTPHandleCRUDAndActivity
    Scenario: POST /admin/api/principals/{name}/delete as admin
      Given the canonical admin profile
      When that principal discovers or invokes POST /admin/api/principals/{name}/delete
      Then the Python outcome is allowed

    @surface-43af9f6a33f7_failure @go_TestAdminHTTPHandleCRUDAndActivity
    Scenario: POST /admin/api/principals/{name}/delete preserves Python validation and failure behavior
      Given malformed, missing, out-of-scope, or unavailable inputs
      When the client invokes POST /admin/api/principals/{name}/delete
      Then 400 validation, 401 auth, 403 policy where applicable, 404 unknown, 405 method, 413/415 upload bounds, 503 unavailable
