Feature: http/graph

  Each operation has a success contract, four canonical role outcomes, and a failure contract.

  Rule: http endpoint workflow

    @surface-87701b11921b @go_TestGraphHTTPStatic
    Scenario: GET /graph preserves the Python success contract
      Given the Python request contract GET request with route-specific JSON/headers
      When an authorized client invokes GET /graph
      Then the response matches embedded HTML

    @surface-87701b11921b_role_reader @go_TestGraphHTTPStatic
    Scenario: GET /graph as reader
      Given the canonical reader profile
      When that principal discovers or invokes GET /graph
      Then the Python outcome is route_available_with_route_specific_policy

    @surface-87701b11921b_role_proposer @go_TestGraphHTTPStatic
    Scenario: GET /graph as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes GET /graph
      Then the Python outcome is route_available_with_route_specific_policy

    @surface-87701b11921b_role_curator @go_TestGraphHTTPStatic
    Scenario: GET /graph as curator
      Given the canonical curator profile
      When that principal discovers or invokes GET /graph
      Then the Python outcome is route_available_with_route_specific_policy

    @surface-87701b11921b_role_admin @go_TestGraphHTTPStatic
    Scenario: GET /graph as admin
      Given the canonical admin profile
      When that principal discovers or invokes GET /graph
      Then the Python outcome is route_available_with_route_specific_policy

    @surface-87701b11921b_failure @go_TestGraphHTTPStatic
    Scenario: GET /graph preserves Python validation and failure behavior
      Given malformed, missing, out-of-scope, or unavailable inputs
      When the client invokes GET /graph
      Then 400 validation, 401 auth, 403 policy where applicable, 404 unknown, 405 method, 413/415 upload bounds, 503 unavailable

    @surface-7841b06d34c9 @go_TestGraphStaticAssets
    Scenario: GET /graph/assets/{asset} preserves the Python success contract
      Given the Python request contract GET request with route-specific JSON/headers
      When an authorized client invokes GET /graph/assets/{asset}
      Then the response matches embedded JS/CSS/vendor

    @surface-7841b06d34c9_role_reader @go_TestGraphStaticAssets
    Scenario: GET /graph/assets/{asset} as reader
      Given the canonical reader profile
      When that principal discovers or invokes GET /graph/assets/{asset}
      Then the Python outcome is route_available_with_route_specific_policy

    @surface-7841b06d34c9_role_proposer @go_TestGraphStaticAssets
    Scenario: GET /graph/assets/{asset} as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes GET /graph/assets/{asset}
      Then the Python outcome is route_available_with_route_specific_policy

    @surface-7841b06d34c9_role_curator @go_TestGraphStaticAssets
    Scenario: GET /graph/assets/{asset} as curator
      Given the canonical curator profile
      When that principal discovers or invokes GET /graph/assets/{asset}
      Then the Python outcome is route_available_with_route_specific_policy

    @surface-7841b06d34c9_role_admin @go_TestGraphStaticAssets
    Scenario: GET /graph/assets/{asset} as admin
      Given the canonical admin profile
      When that principal discovers or invokes GET /graph/assets/{asset}
      Then the Python outcome is route_available_with_route_specific_policy

    @surface-7841b06d34c9_failure @go_TestGraphStaticAssets
    Scenario: GET /graph/assets/{asset} preserves Python validation and failure behavior
      Given malformed, missing, out-of-scope, or unavailable inputs
      When the client invokes GET /graph/assets/{asset}
      Then 400 validation, 401 auth, 403 policy where applicable, 404 unknown, 405 method, 413/415 upload bounds, 503 unavailable

    @surface-56754c9cb122 @go_TestGraphHTTPThroughUMCP
    Scenario: GET /graph/api/v1/status preserves the Python success contract
      Given the Python request contract GET request with route-specific JSON/headers
      When an authorized client invokes GET /graph/api/v1/status
      Then the response matches graph enabled/status

    @surface-56754c9cb122_role_reader @go_TestGraphHTTPThroughUMCP
    Scenario: GET /graph/api/v1/status as reader
      Given the canonical reader profile
      When that principal discovers or invokes GET /graph/api/v1/status
      Then the Python outcome is route_available_with_route_specific_policy

    @surface-56754c9cb122_role_proposer @go_TestGraphHTTPThroughUMCP
    Scenario: GET /graph/api/v1/status as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes GET /graph/api/v1/status
      Then the Python outcome is route_available_with_route_specific_policy

    @surface-56754c9cb122_role_curator @go_TestGraphHTTPThroughUMCP
    Scenario: GET /graph/api/v1/status as curator
      Given the canonical curator profile
      When that principal discovers or invokes GET /graph/api/v1/status
      Then the Python outcome is route_available_with_route_specific_policy

    @surface-56754c9cb122_role_admin @go_TestGraphHTTPThroughUMCP
    Scenario: GET /graph/api/v1/status as admin
      Given the canonical admin profile
      When that principal discovers or invokes GET /graph/api/v1/status
      Then the Python outcome is route_available_with_route_specific_policy

    @surface-56754c9cb122_failure @go_TestGraphHTTPThroughUMCP
    Scenario: GET /graph/api/v1/status preserves Python validation and failure behavior
      Given malformed, missing, out-of-scope, or unavailable inputs
      When the client invokes GET /graph/api/v1/status
      Then 400 validation, 401 auth, 403 policy where applicable, 404 unknown, 405 method, 413/415 upload bounds, 503 unavailable

    @surface-77ca624d98cc @go_TestGraphHTTPThroughUMCP
    Scenario: GET /graph/api/v1/principals preserves the Python success contract
      Given the Python request contract GET request with route-specific JSON/headers
      When an authorized client invokes GET /graph/api/v1/principals
      Then the response matches simulation policy list

    @surface-77ca624d98cc_role_reader @go_TestGraphHTTPThroughUMCP
    Scenario: GET /graph/api/v1/principals as reader
      Given the canonical reader profile
      When that principal discovers or invokes GET /graph/api/v1/principals
      Then the Python outcome is route_available_with_route_specific_policy

    @surface-77ca624d98cc_role_proposer @go_TestGraphHTTPThroughUMCP
    Scenario: GET /graph/api/v1/principals as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes GET /graph/api/v1/principals
      Then the Python outcome is route_available_with_route_specific_policy

    @surface-77ca624d98cc_role_curator @go_TestGraphHTTPThroughUMCP
    Scenario: GET /graph/api/v1/principals as curator
      Given the canonical curator profile
      When that principal discovers or invokes GET /graph/api/v1/principals
      Then the Python outcome is route_available_with_route_specific_policy

    @surface-77ca624d98cc_role_admin @go_TestGraphHTTPThroughUMCP
    Scenario: GET /graph/api/v1/principals as admin
      Given the canonical admin profile
      When that principal discovers or invokes GET /graph/api/v1/principals
      Then the Python outcome is route_available_with_route_specific_policy

    @surface-77ca624d98cc_failure @go_TestGraphHTTPThroughUMCP
    Scenario: GET /graph/api/v1/principals preserves Python validation and failure behavior
      Given malformed, missing, out-of-scope, or unavailable inputs
      When the client invokes GET /graph/api/v1/principals
      Then 400 validation, 401 auth, 403 policy where applicable, 404 unknown, 405 method, 413/415 upload bounds, 503 unavailable

    @surface-9c590b85aa90 @go_TestOverviewFixture
    Scenario: GET /graph/api/v1/overview preserves the Python success contract
      Given the Python request contract GET request with route-specific JSON/headers
      When an authorized client invokes GET /graph/api/v1/overview
      Then the response matches bounded graph snapshot

    @surface-9c590b85aa90_role_reader @go_TestOverviewFixture
    Scenario: GET /graph/api/v1/overview as reader
      Given the canonical reader profile
      When that principal discovers or invokes GET /graph/api/v1/overview
      Then the Python outcome is route_available_with_route_specific_policy

    @surface-9c590b85aa90_role_proposer @go_TestOverviewFixture
    Scenario: GET /graph/api/v1/overview as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes GET /graph/api/v1/overview
      Then the Python outcome is route_available_with_route_specific_policy

    @surface-9c590b85aa90_role_curator @go_TestOverviewFixture
    Scenario: GET /graph/api/v1/overview as curator
      Given the canonical curator profile
      When that principal discovers or invokes GET /graph/api/v1/overview
      Then the Python outcome is route_available_with_route_specific_policy

    @surface-9c590b85aa90_role_admin @go_TestOverviewFixture
    Scenario: GET /graph/api/v1/overview as admin
      Given the canonical admin profile
      When that principal discovers or invokes GET /graph/api/v1/overview
      Then the Python outcome is route_available_with_route_specific_policy

    @surface-9c590b85aa90_failure @go_TestOverviewFixture
    Scenario: GET /graph/api/v1/overview preserves Python validation and failure behavior
      Given malformed, missing, out-of-scope, or unavailable inputs
      When the client invokes GET /graph/api/v1/overview
      Then 400 validation, 401 auth, 403 policy where applicable, 404 unknown, 405 method, 413/415 upload bounds, 503 unavailable

    @surface-36b7cee1143b @go_TestGraphHTTPThroughUMCP
    Scenario: GET /graph/api/v1/embeddings/status preserves the Python success contract
      Given the Python request contract GET request with route-specific JSON/headers
      When an authorized client invokes GET /graph/api/v1/embeddings/status
      Then the response matches worker status

    @surface-36b7cee1143b_role_reader @go_TestGraphHTTPThroughUMCP
    Scenario: GET /graph/api/v1/embeddings/status as reader
      Given the canonical reader profile
      When that principal discovers or invokes GET /graph/api/v1/embeddings/status
      Then the Python outcome is route_available_with_route_specific_policy

    @surface-36b7cee1143b_role_proposer @go_TestGraphHTTPThroughUMCP
    Scenario: GET /graph/api/v1/embeddings/status as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes GET /graph/api/v1/embeddings/status
      Then the Python outcome is route_available_with_route_specific_policy

    @surface-36b7cee1143b_role_curator @go_TestGraphHTTPThroughUMCP
    Scenario: GET /graph/api/v1/embeddings/status as curator
      Given the canonical curator profile
      When that principal discovers or invokes GET /graph/api/v1/embeddings/status
      Then the Python outcome is route_available_with_route_specific_policy

    @surface-36b7cee1143b_role_admin @go_TestGraphHTTPThroughUMCP
    Scenario: GET /graph/api/v1/embeddings/status as admin
      Given the canonical admin profile
      When that principal discovers or invokes GET /graph/api/v1/embeddings/status
      Then the Python outcome is route_available_with_route_specific_policy

    @surface-36b7cee1143b_failure @go_TestGraphHTTPThroughUMCP
    Scenario: GET /graph/api/v1/embeddings/status preserves Python validation and failure behavior
      Given malformed, missing, out-of-scope, or unavailable inputs
      When the client invokes GET /graph/api/v1/embeddings/status
      Then 400 validation, 401 auth, 403 policy where applicable, 404 unknown, 405 method, 413/415 upload bounds, 503 unavailable

    @surface-8b2f29ba0ef9 @go_TestGraphHTTPExport @go_TestGraphHTTPRefresh
    Scenario: POST /graph/api/v1/embeddings/refresh preserves the Python success contract
      Given the Python request contract POST request with route-specific JSON/headers
      When an authorized client invokes POST /graph/api/v1/embeddings/refresh
      Then the response matches 202 worker status

    @surface-8b2f29ba0ef9_role_reader @go_TestGraphHTTPExport @go_TestGraphHTTPRefresh
    Scenario: POST /graph/api/v1/embeddings/refresh as reader
      Given the canonical reader profile
      When that principal discovers or invokes POST /graph/api/v1/embeddings/refresh
      Then the Python outcome is route_available_with_route_specific_policy

    @surface-8b2f29ba0ef9_role_proposer @go_TestGraphHTTPExport @go_TestGraphHTTPRefresh
    Scenario: POST /graph/api/v1/embeddings/refresh as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes POST /graph/api/v1/embeddings/refresh
      Then the Python outcome is route_available_with_route_specific_policy

    @surface-8b2f29ba0ef9_role_curator @go_TestGraphHTTPExport @go_TestGraphHTTPRefresh
    Scenario: POST /graph/api/v1/embeddings/refresh as curator
      Given the canonical curator profile
      When that principal discovers or invokes POST /graph/api/v1/embeddings/refresh
      Then the Python outcome is route_available_with_route_specific_policy

    @surface-8b2f29ba0ef9_role_admin @go_TestGraphHTTPExport @go_TestGraphHTTPRefresh
    Scenario: POST /graph/api/v1/embeddings/refresh as admin
      Given the canonical admin profile
      When that principal discovers or invokes POST /graph/api/v1/embeddings/refresh
      Then the Python outcome is route_available_with_route_specific_policy

    @surface-8b2f29ba0ef9_failure @go_TestGraphHTTPExport @go_TestGraphHTTPRefresh
    Scenario: POST /graph/api/v1/embeddings/refresh preserves Python validation and failure behavior
      Given malformed, missing, out-of-scope, or unavailable inputs
      When the client invokes POST /graph/api/v1/embeddings/refresh
      Then 400 validation, 401 auth, 403 policy where applicable, 404 unknown, 405 method, 413/415 upload bounds, 503 unavailable

    @surface-03d8b8c261e0 @go_TestGraphHTTPThroughUMCP
    Scenario: POST /graph/api/v1/search preserves the Python success contract
      Given the Python request contract POST request with route-specific JSON/headers
      When an authorized client invokes POST /graph/api/v1/search
      Then the response matches scoped search results

    @surface-03d8b8c261e0_role_reader @go_TestGraphHTTPThroughUMCP
    Scenario: POST /graph/api/v1/search as reader
      Given the canonical reader profile
      When that principal discovers or invokes POST /graph/api/v1/search
      Then the Python outcome is route_available_with_route_specific_policy

    @surface-03d8b8c261e0_role_proposer @go_TestGraphHTTPThroughUMCP
    Scenario: POST /graph/api/v1/search as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes POST /graph/api/v1/search
      Then the Python outcome is route_available_with_route_specific_policy

    @surface-03d8b8c261e0_role_curator @go_TestGraphHTTPThroughUMCP
    Scenario: POST /graph/api/v1/search as curator
      Given the canonical curator profile
      When that principal discovers or invokes POST /graph/api/v1/search
      Then the Python outcome is route_available_with_route_specific_policy

    @surface-03d8b8c261e0_role_admin @go_TestGraphHTTPThroughUMCP
    Scenario: POST /graph/api/v1/search as admin
      Given the canonical admin profile
      When that principal discovers or invokes POST /graph/api/v1/search
      Then the Python outcome is route_available_with_route_specific_policy

    @surface-03d8b8c261e0_failure @go_TestGraphHTTPThroughUMCP
    Scenario: POST /graph/api/v1/search preserves Python validation and failure behavior
      Given malformed, missing, out-of-scope, or unavailable inputs
      When the client invokes POST /graph/api/v1/search
      Then 400 validation, 401 auth, 403 policy where applicable, 404 unknown, 405 method, 413/415 upload bounds, 503 unavailable

    @surface-a98c056ba8ec @go_TestGraphHTTPThroughUMCP
    Scenario: GET /graph/api/v1/clusters/{id} preserves the Python success contract
      Given the Python request contract GET request with route-specific JSON/headers
      When an authorized client invokes GET /graph/api/v1/clusters/{id}
      Then the response matches cluster expansion

    @surface-a98c056ba8ec_role_reader @go_TestGraphHTTPThroughUMCP
    Scenario: GET /graph/api/v1/clusters/{id} as reader
      Given the canonical reader profile
      When that principal discovers or invokes GET /graph/api/v1/clusters/{id}
      Then the Python outcome is route_available_with_route_specific_policy

    @surface-a98c056ba8ec_role_proposer @go_TestGraphHTTPThroughUMCP
    Scenario: GET /graph/api/v1/clusters/{id} as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes GET /graph/api/v1/clusters/{id}
      Then the Python outcome is route_available_with_route_specific_policy

    @surface-a98c056ba8ec_role_curator @go_TestGraphHTTPThroughUMCP
    Scenario: GET /graph/api/v1/clusters/{id} as curator
      Given the canonical curator profile
      When that principal discovers or invokes GET /graph/api/v1/clusters/{id}
      Then the Python outcome is route_available_with_route_specific_policy

    @surface-a98c056ba8ec_role_admin @go_TestGraphHTTPThroughUMCP
    Scenario: GET /graph/api/v1/clusters/{id} as admin
      Given the canonical admin profile
      When that principal discovers or invokes GET /graph/api/v1/clusters/{id}
      Then the Python outcome is route_available_with_route_specific_policy

    @surface-a98c056ba8ec_failure @go_TestGraphHTTPThroughUMCP
    Scenario: GET /graph/api/v1/clusters/{id} preserves Python validation and failure behavior
      Given malformed, missing, out-of-scope, or unavailable inputs
      When the client invokes GET /graph/api/v1/clusters/{id}
      Then 400 validation, 401 auth, 403 policy where applicable, 404 unknown, 405 method, 413/415 upload bounds, 503 unavailable

    @surface-859546743957 @go_TestGraphHTTPThroughUMCP
    Scenario: GET /graph/api/v1/memories/{id} preserves the Python success contract
      Given the Python request contract GET request with route-specific JSON/headers
      When an authorized client invokes GET /graph/api/v1/memories/{id}
      Then the response matches detail/links/assets/proposals

    @surface-859546743957_role_reader @go_TestGraphHTTPThroughUMCP
    Scenario: GET /graph/api/v1/memories/{id} as reader
      Given the canonical reader profile
      When that principal discovers or invokes GET /graph/api/v1/memories/{id}
      Then the Python outcome is route_available_with_route_specific_policy

    @surface-859546743957_role_proposer @go_TestGraphHTTPThroughUMCP
    Scenario: GET /graph/api/v1/memories/{id} as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes GET /graph/api/v1/memories/{id}
      Then the Python outcome is route_available_with_route_specific_policy

    @surface-859546743957_role_curator @go_TestGraphHTTPThroughUMCP
    Scenario: GET /graph/api/v1/memories/{id} as curator
      Given the canonical curator profile
      When that principal discovers or invokes GET /graph/api/v1/memories/{id}
      Then the Python outcome is route_available_with_route_specific_policy

    @surface-859546743957_role_admin @go_TestGraphHTTPThroughUMCP
    Scenario: GET /graph/api/v1/memories/{id} as admin
      Given the canonical admin profile
      When that principal discovers or invokes GET /graph/api/v1/memories/{id}
      Then the Python outcome is route_available_with_route_specific_policy

    @surface-859546743957_failure @go_TestGraphHTTPThroughUMCP
    Scenario: GET /graph/api/v1/memories/{id} preserves Python validation and failure behavior
      Given malformed, missing, out-of-scope, or unavailable inputs
      When the client invokes GET /graph/api/v1/memories/{id}
      Then 400 validation, 401 auth, 403 policy where applicable, 404 unknown, 405 method, 413/415 upload bounds, 503 unavailable

    @surface-12fd0752fde9 @go_TestGraphHTTPThroughUMCP
    Scenario: GET /graph/api/v1/neighbourhood/{id} preserves the Python success contract
      Given the Python request contract GET request with route-specific JSON/headers
      When an authorized client invokes GET /graph/api/v1/neighbourhood/{id}
      Then the response matches bounded neighbourhood

    @surface-12fd0752fde9_role_reader @go_TestGraphHTTPThroughUMCP
    Scenario: GET /graph/api/v1/neighbourhood/{id} as reader
      Given the canonical reader profile
      When that principal discovers or invokes GET /graph/api/v1/neighbourhood/{id}
      Then the Python outcome is route_available_with_route_specific_policy

    @surface-12fd0752fde9_role_proposer @go_TestGraphHTTPThroughUMCP
    Scenario: GET /graph/api/v1/neighbourhood/{id} as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes GET /graph/api/v1/neighbourhood/{id}
      Then the Python outcome is route_available_with_route_specific_policy

    @surface-12fd0752fde9_role_curator @go_TestGraphHTTPThroughUMCP
    Scenario: GET /graph/api/v1/neighbourhood/{id} as curator
      Given the canonical curator profile
      When that principal discovers or invokes GET /graph/api/v1/neighbourhood/{id}
      Then the Python outcome is route_available_with_route_specific_policy

    @surface-12fd0752fde9_role_admin @go_TestGraphHTTPThroughUMCP
    Scenario: GET /graph/api/v1/neighbourhood/{id} as admin
      Given the canonical admin profile
      When that principal discovers or invokes GET /graph/api/v1/neighbourhood/{id}
      Then the Python outcome is route_available_with_route_specific_policy

    @surface-12fd0752fde9_failure @go_TestGraphHTTPThroughUMCP
    Scenario: GET /graph/api/v1/neighbourhood/{id} preserves Python validation and failure behavior
      Given malformed, missing, out-of-scope, or unavailable inputs
      When the client invokes GET /graph/api/v1/neighbourhood/{id}
      Then 400 validation, 401 auth, 403 policy where applicable, 404 unknown, 405 method, 413/415 upload bounds, 503 unavailable

    @surface-b0340ab6792a @go_TestGraphHTTPStatic
    Scenario: GET /graph/api/v1/assets/{path} preserves the Python success contract
      Given the Python request contract GET request with route-specific JSON/headers
      When an authorized client invokes GET /graph/api/v1/assets/{path}
      Then the response matches static asset

    @surface-b0340ab6792a_role_reader @go_TestGraphHTTPStatic
    Scenario: GET /graph/api/v1/assets/{path} as reader
      Given the canonical reader profile
      When that principal discovers or invokes GET /graph/api/v1/assets/{path}
      Then the Python outcome is route_available_with_route_specific_policy

    @surface-b0340ab6792a_role_proposer @go_TestGraphHTTPStatic
    Scenario: GET /graph/api/v1/assets/{path} as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes GET /graph/api/v1/assets/{path}
      Then the Python outcome is route_available_with_route_specific_policy

    @surface-b0340ab6792a_role_curator @go_TestGraphHTTPStatic
    Scenario: GET /graph/api/v1/assets/{path} as curator
      Given the canonical curator profile
      When that principal discovers or invokes GET /graph/api/v1/assets/{path}
      Then the Python outcome is route_available_with_route_specific_policy

    @surface-b0340ab6792a_role_admin @go_TestGraphHTTPStatic
    Scenario: GET /graph/api/v1/assets/{path} as admin
      Given the canonical admin profile
      When that principal discovers or invokes GET /graph/api/v1/assets/{path}
      Then the Python outcome is route_available_with_route_specific_policy

    @surface-b0340ab6792a_failure @go_TestGraphHTTPStatic
    Scenario: GET /graph/api/v1/assets/{path} preserves Python validation and failure behavior
      Given malformed, missing, out-of-scope, or unavailable inputs
      When the client invokes GET /graph/api/v1/assets/{path}
      Then 400 validation, 401 auth, 403 policy where applicable, 404 unknown, 405 method, 413/415 upload bounds, 503 unavailable

    @surface-3baee52bbbef @go_TestGraphHTTPExport @go_TestGraphHTTPRefresh
    Scenario: POST /graph/api/v1/export/{json|svg} preserves the Python success contract
      Given the Python request contract POST request with route-specific JSON/headers
      When an authorized client invokes POST /graph/api/v1/export/{json|svg}
      Then the response matches bounded export

    @surface-3baee52bbbef_role_reader @go_TestGraphHTTPExport @go_TestGraphHTTPRefresh
    Scenario: POST /graph/api/v1/export/{json|svg} as reader
      Given the canonical reader profile
      When that principal discovers or invokes POST /graph/api/v1/export/{json|svg}
      Then the Python outcome is route_available_with_route_specific_policy

    @surface-3baee52bbbef_role_proposer @go_TestGraphHTTPExport @go_TestGraphHTTPRefresh
    Scenario: POST /graph/api/v1/export/{json|svg} as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes POST /graph/api/v1/export/{json|svg}
      Then the Python outcome is route_available_with_route_specific_policy

    @surface-3baee52bbbef_role_curator @go_TestGraphHTTPExport @go_TestGraphHTTPRefresh
    Scenario: POST /graph/api/v1/export/{json|svg} as curator
      Given the canonical curator profile
      When that principal discovers or invokes POST /graph/api/v1/export/{json|svg}
      Then the Python outcome is route_available_with_route_specific_policy

    @surface-3baee52bbbef_role_admin @go_TestGraphHTTPExport @go_TestGraphHTTPRefresh
    Scenario: POST /graph/api/v1/export/{json|svg} as admin
      Given the canonical admin profile
      When that principal discovers or invokes POST /graph/api/v1/export/{json|svg}
      Then the Python outcome is route_available_with_route_specific_policy

    @surface-3baee52bbbef_failure @go_TestGraphHTTPExport @go_TestGraphHTTPRefresh
    Scenario: POST /graph/api/v1/export/{json|svg} preserves Python validation and failure behavior
      Given malformed, missing, out-of-scope, or unavailable inputs
      When the client invokes POST /graph/api/v1/export/{json|svg}
      Then 400 validation, 401 auth, 403 policy where applicable, 404 unknown, 405 method, 413/415 upload bounds, 503 unavailable
