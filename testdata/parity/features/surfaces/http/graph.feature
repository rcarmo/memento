Feature: http/graph

  Each operation has a success contract, four canonical role outcomes, and a failure contract.
  Stable row tags link behavior to validation data without exposing implementation details.

  Rule: http endpoint workflow

    @surface-87701b11921b
    Scenario: GET /graph succeeds with its declared contract
      Given required arguments no required arguments
      And optional arguments no optional arguments
      And declared defaults no declared defaults
      And policy scope trusted network
      When an authorized client invokes GET /graph
      Then the response exposes HTTP 200, Content-Type text/html, embedded graph debugger
      And side effects are none
      And idempotency is not_applicable
      And pagination or range behavior is none

    @surface-87701b11921b_role_reader
    Scenario: GET /graph as reader
      Given the canonical reader profile
      When that principal uses GET /graph
      Then the outcome is trusted-network route; optional simulated principal applies that profile read policy before output

    @surface-87701b11921b_role_proposer
    Scenario: GET /graph as proposer
      Given the canonical proposer profile
      When that principal uses GET /graph
      Then the outcome is trusted-network route; optional simulated principal applies that profile read policy before output

    @surface-87701b11921b_role_curator
    Scenario: GET /graph as curator
      Given the canonical curator profile
      When that principal uses GET /graph
      Then the outcome is trusted-network route; optional simulated principal applies that profile read policy before output

    @surface-87701b11921b_role_admin
    Scenario: GET /graph as admin
      Given the canonical admin profile
      When that principal uses GET /graph
      Then the outcome is trusted-network route; optional simulated principal applies that profile read policy before output

    @surface-87701b11921b_failure
    Scenario: GET /graph rejects invalid or conflicting requests
      Given required arguments no required arguments and declared defaults no declared defaults
      And malformed, missing, out-of-scope, unavailable, replayed, or conflicting inputs
      When the client invokes GET /graph
      Then one of the specified failures is 400 validation, 401 auth, 403 policy where applicable, 404 unknown, 405 method, 413/415 upload bounds, 503 unavailable
      And failed pre-publication calls do not apply none

    @surface-7841b06d34c9
    Scenario: GET /graph/assets/{asset} succeeds with its declared contract
      Given required arguments no required arguments
      And optional arguments no optional arguments
      And declared defaults no declared defaults
      And policy scope trusted network
      When an authorized client invokes GET /graph/assets/{asset}
      Then the response exposes HTTP 200, asset MIME type, Cache-Control no-store, nosniff
      And side effects are none
      And idempotency is not_applicable
      And pagination or range behavior is none

    @surface-7841b06d34c9_role_reader
    Scenario: GET /graph/assets/{asset} as reader
      Given the canonical reader profile
      When that principal uses GET /graph/assets/{asset}
      Then the outcome is trusted-network route; optional simulated principal applies that profile read policy before output

    @surface-7841b06d34c9_role_proposer
    Scenario: GET /graph/assets/{asset} as proposer
      Given the canonical proposer profile
      When that principal uses GET /graph/assets/{asset}
      Then the outcome is trusted-network route; optional simulated principal applies that profile read policy before output

    @surface-7841b06d34c9_role_curator
    Scenario: GET /graph/assets/{asset} as curator
      Given the canonical curator profile
      When that principal uses GET /graph/assets/{asset}
      Then the outcome is trusted-network route; optional simulated principal applies that profile read policy before output

    @surface-7841b06d34c9_role_admin
    Scenario: GET /graph/assets/{asset} as admin
      Given the canonical admin profile
      When that principal uses GET /graph/assets/{asset}
      Then the outcome is trusted-network route; optional simulated principal applies that profile read policy before output

    @surface-7841b06d34c9_failure
    Scenario: GET /graph/assets/{asset} rejects invalid or conflicting requests
      Given required arguments no required arguments and declared defaults no declared defaults
      And malformed, missing, out-of-scope, unavailable, replayed, or conflicting inputs
      When the client invokes GET /graph/assets/{asset}
      Then one of the specified failures is 400 validation, 401 auth, 403 policy where applicable, 404 unknown, 405 method, 413/415 upload bounds, 503 unavailable
      And failed pre-publication calls do not apply none

    @surface-56754c9cb122
    Scenario: GET /graph/api/v1/status succeeds with its declared contract
      Given required arguments no required arguments
      And optional arguments no optional arguments
      And declared defaults no declared defaults
      And policy scope trusted network
      When an authorized client invokes GET /graph/api/v1/status
      Then the response exposes HTTP 200, enabled, route_prefix, schema_version, warning
      And side effects are none
      And idempotency is not_applicable
      And pagination or range behavior is none

    @surface-56754c9cb122_role_reader
    Scenario: GET /graph/api/v1/status as reader
      Given the canonical reader profile
      When that principal uses GET /graph/api/v1/status
      Then the outcome is trusted-network route; optional simulated principal applies that profile read policy before output

    @surface-56754c9cb122_role_proposer
    Scenario: GET /graph/api/v1/status as proposer
      Given the canonical proposer profile
      When that principal uses GET /graph/api/v1/status
      Then the outcome is trusted-network route; optional simulated principal applies that profile read policy before output

    @surface-56754c9cb122_role_curator
    Scenario: GET /graph/api/v1/status as curator
      Given the canonical curator profile
      When that principal uses GET /graph/api/v1/status
      Then the outcome is trusted-network route; optional simulated principal applies that profile read policy before output

    @surface-56754c9cb122_role_admin
    Scenario: GET /graph/api/v1/status as admin
      Given the canonical admin profile
      When that principal uses GET /graph/api/v1/status
      Then the outcome is trusted-network route; optional simulated principal applies that profile read policy before output

    @surface-56754c9cb122_failure
    Scenario: GET /graph/api/v1/status rejects invalid or conflicting requests
      Given required arguments no required arguments and declared defaults no declared defaults
      And malformed, missing, out-of-scope, unavailable, replayed, or conflicting inputs
      When the client invokes GET /graph/api/v1/status
      Then one of the specified failures is 400 validation, 401 auth, 403 policy where applicable, 404 unknown, 405 method, 413/415 upload bounds, 503 unavailable
      And failed pre-publication calls do not apply none

    @surface-77ca624d98cc
    Scenario: GET /graph/api/v1/principals succeeds with its declared contract
      Given required arguments no required arguments
      And optional arguments no optional arguments
      And declared defaults no declared defaults
      And policy scope trusted network
      When an authorized client invokes GET /graph/api/v1/principals
      Then the response exposes HTTP 200, principals[].name, roles[], read_prefixes[], write_prefixes[], protected_read_prefixes[]
      And side effects are none
      And idempotency is not_applicable
      And pagination or range behavior is none

    @surface-77ca624d98cc_role_reader
    Scenario: GET /graph/api/v1/principals as reader
      Given the canonical reader profile
      When that principal uses GET /graph/api/v1/principals
      Then the outcome is trusted-network route; optional simulated principal applies that profile read policy before output

    @surface-77ca624d98cc_role_proposer
    Scenario: GET /graph/api/v1/principals as proposer
      Given the canonical proposer profile
      When that principal uses GET /graph/api/v1/principals
      Then the outcome is trusted-network route; optional simulated principal applies that profile read policy before output

    @surface-77ca624d98cc_role_curator
    Scenario: GET /graph/api/v1/principals as curator
      Given the canonical curator profile
      When that principal uses GET /graph/api/v1/principals
      Then the outcome is trusted-network route; optional simulated principal applies that profile read policy before output

    @surface-77ca624d98cc_role_admin
    Scenario: GET /graph/api/v1/principals as admin
      Given the canonical admin profile
      When that principal uses GET /graph/api/v1/principals
      Then the outcome is trusted-network route; optional simulated principal applies that profile read policy before output

    @surface-77ca624d98cc_failure
    Scenario: GET /graph/api/v1/principals rejects invalid or conflicting requests
      Given required arguments no required arguments and declared defaults no declared defaults
      And malformed, missing, out-of-scope, unavailable, replayed, or conflicting inputs
      When the client invokes GET /graph/api/v1/principals
      Then one of the specified failures is 400 validation, 401 auth, 403 policy where applicable, 404 unknown, 405 method, 413/415 upload bounds, 503 unavailable
      And failed pre-publication calls do not apply none

    @surface-9c590b85aa90
    Scenario: GET /graph/api/v1/overview succeeds with its declared contract
      Given required arguments no required arguments
      And optional arguments no optional arguments
      And declared defaults no declared defaults
      And policy scope trusted network/simulated policy
      When an authorized client invokes GET /graph/api/v1/overview
      Then the response exposes HTTP 200, mode, nodes[] or clusters[], edges[] or cluster_edges[], diagnostics[], revisions, truncated
      And side effects are none
      And idempotency is not_applicable
      And pagination or range behavior is none

    @surface-9c590b85aa90_role_reader
    Scenario: GET /graph/api/v1/overview as reader
      Given the canonical reader profile
      When that principal uses GET /graph/api/v1/overview
      Then the outcome is trusted-network route; optional simulated principal applies that profile read policy before output

    @surface-9c590b85aa90_role_proposer
    Scenario: GET /graph/api/v1/overview as proposer
      Given the canonical proposer profile
      When that principal uses GET /graph/api/v1/overview
      Then the outcome is trusted-network route; optional simulated principal applies that profile read policy before output

    @surface-9c590b85aa90_role_curator
    Scenario: GET /graph/api/v1/overview as curator
      Given the canonical curator profile
      When that principal uses GET /graph/api/v1/overview
      Then the outcome is trusted-network route; optional simulated principal applies that profile read policy before output

    @surface-9c590b85aa90_role_admin
    Scenario: GET /graph/api/v1/overview as admin
      Given the canonical admin profile
      When that principal uses GET /graph/api/v1/overview
      Then the outcome is trusted-network route; optional simulated principal applies that profile read policy before output

    @surface-9c590b85aa90_failure
    Scenario: GET /graph/api/v1/overview rejects invalid or conflicting requests
      Given required arguments no required arguments and declared defaults no declared defaults
      And malformed, missing, out-of-scope, unavailable, replayed, or conflicting inputs
      When the client invokes GET /graph/api/v1/overview
      Then one of the specified failures is 400 validation, 401 auth, 403 policy where applicable, 404 unknown, 405 method, 413/415 upload bounds, 503 unavailable
      And failed pre-publication calls do not apply none

    @surface-36b7cee1143b
    Scenario: GET /graph/api/v1/embeddings/status succeeds with its declared contract
      Given required arguments no required arguments
      And optional arguments no optional arguments
      And declared defaults no declared defaults
      And policy scope trusted network
      When an authorized client invokes GET /graph/api/v1/embeddings/status
      Then the response exposes HTTP 200, available, alive, running, pending, pause_reason, completed, repository_revision, embedding_revision, last_error
      And side effects are none
      And idempotency is not_applicable
      And pagination or range behavior is none

    @surface-36b7cee1143b_role_reader
    Scenario: GET /graph/api/v1/embeddings/status as reader
      Given the canonical reader profile
      When that principal uses GET /graph/api/v1/embeddings/status
      Then the outcome is trusted-network route; optional simulated principal applies that profile read policy before output

    @surface-36b7cee1143b_role_proposer
    Scenario: GET /graph/api/v1/embeddings/status as proposer
      Given the canonical proposer profile
      When that principal uses GET /graph/api/v1/embeddings/status
      Then the outcome is trusted-network route; optional simulated principal applies that profile read policy before output

    @surface-36b7cee1143b_role_curator
    Scenario: GET /graph/api/v1/embeddings/status as curator
      Given the canonical curator profile
      When that principal uses GET /graph/api/v1/embeddings/status
      Then the outcome is trusted-network route; optional simulated principal applies that profile read policy before output

    @surface-36b7cee1143b_role_admin
    Scenario: GET /graph/api/v1/embeddings/status as admin
      Given the canonical admin profile
      When that principal uses GET /graph/api/v1/embeddings/status
      Then the outcome is trusted-network route; optional simulated principal applies that profile read policy before output

    @surface-36b7cee1143b_failure
    Scenario: GET /graph/api/v1/embeddings/status rejects invalid or conflicting requests
      Given required arguments no required arguments and declared defaults no declared defaults
      And malformed, missing, out-of-scope, unavailable, replayed, or conflicting inputs
      When the client invokes GET /graph/api/v1/embeddings/status
      Then one of the specified failures is 400 validation, 401 auth, 403 policy where applicable, 404 unknown, 405 method, 413/415 upload bounds, 503 unavailable
      And failed pre-publication calls do not apply none

    @surface-8b2f29ba0ef9
    Scenario: POST /graph/api/v1/embeddings/refresh succeeds with its declared contract
      Given required arguments no required arguments
      And optional arguments no optional arguments
      And declared defaults no declared defaults
      And policy scope full diagnostic only
      When an authorized client invokes POST /graph/api/v1/embeddings/refresh
      Then the response exposes HTTP 202, worker state, queued scope/path count
      And side effects are queues or persists the endpoint-specific staging, refresh, access, or session transition
      And idempotency is not_applicable
      And pagination or range behavior is none

    @surface-8b2f29ba0ef9_role_reader
    Scenario: POST /graph/api/v1/embeddings/refresh as reader
      Given the canonical reader profile
      When that principal uses POST /graph/api/v1/embeddings/refresh
      Then the outcome is HTTP 202 only in full diagnostic view; simulated-principal requests return HTTP 400

    @surface-8b2f29ba0ef9_role_proposer
    Scenario: POST /graph/api/v1/embeddings/refresh as proposer
      Given the canonical proposer profile
      When that principal uses POST /graph/api/v1/embeddings/refresh
      Then the outcome is HTTP 202 only in full diagnostic view; simulated-principal requests return HTTP 400

    @surface-8b2f29ba0ef9_role_curator
    Scenario: POST /graph/api/v1/embeddings/refresh as curator
      Given the canonical curator profile
      When that principal uses POST /graph/api/v1/embeddings/refresh
      Then the outcome is HTTP 202 only in full diagnostic view; simulated-principal requests return HTTP 400

    @surface-8b2f29ba0ef9_role_admin
    Scenario: POST /graph/api/v1/embeddings/refresh as admin
      Given the canonical admin profile
      When that principal uses POST /graph/api/v1/embeddings/refresh
      Then the outcome is HTTP 202 only in full diagnostic view; simulated-principal requests return HTTP 400

    @surface-8b2f29ba0ef9_failure
    Scenario: POST /graph/api/v1/embeddings/refresh rejects invalid or conflicting requests
      Given required arguments no required arguments and declared defaults no declared defaults
      And malformed, missing, out-of-scope, unavailable, replayed, or conflicting inputs
      When the client invokes POST /graph/api/v1/embeddings/refresh
      Then one of the specified failures is 400 validation, 401 auth, 403 policy where applicable, 404 unknown, 405 method, 413/415 upload bounds, 503 unavailable
      And failed pre-publication calls do not apply queues or persists the endpoint-specific staging, refresh, access, or session transition

    @surface-03d8b8c261e0
    Scenario: POST /graph/api/v1/search succeeds with its declared contract
      Given required arguments no required arguments
      And optional arguments no optional arguments
      And declared defaults no declared defaults
      And policy scope trusted network/simulated policy
      When an authorized client invokes POST /graph/api/v1/search
      Then the response exposes HTTP 200, results[], results[].id, results[].path, results[].title, results[].tags, results[].snippet
      And side effects are none
      And idempotency is not_applicable
      And pagination or range behavior is none

    @surface-03d8b8c261e0_role_reader
    Scenario: POST /graph/api/v1/search as reader
      Given the canonical reader profile
      When that principal uses POST /graph/api/v1/search
      Then the outcome is trusted-network route; optional simulated principal applies that profile read policy before output

    @surface-03d8b8c261e0_role_proposer
    Scenario: POST /graph/api/v1/search as proposer
      Given the canonical proposer profile
      When that principal uses POST /graph/api/v1/search
      Then the outcome is trusted-network route; optional simulated principal applies that profile read policy before output

    @surface-03d8b8c261e0_role_curator
    Scenario: POST /graph/api/v1/search as curator
      Given the canonical curator profile
      When that principal uses POST /graph/api/v1/search
      Then the outcome is trusted-network route; optional simulated principal applies that profile read policy before output

    @surface-03d8b8c261e0_role_admin
    Scenario: POST /graph/api/v1/search as admin
      Given the canonical admin profile
      When that principal uses POST /graph/api/v1/search
      Then the outcome is trusted-network route; optional simulated principal applies that profile read policy before output

    @surface-03d8b8c261e0_failure
    Scenario: POST /graph/api/v1/search rejects invalid or conflicting requests
      Given required arguments no required arguments and declared defaults no declared defaults
      And malformed, missing, out-of-scope, unavailable, replayed, or conflicting inputs
      When the client invokes POST /graph/api/v1/search
      Then one of the specified failures is 400 validation, 401 auth, 403 policy where applicable, 404 unknown, 405 method, 413/415 upload bounds, 503 unavailable
      And failed pre-publication calls do not apply none

    @surface-a98c056ba8ec
    Scenario: GET /graph/api/v1/clusters/{id} succeeds with its declared contract
      Given required arguments no required arguments
      And optional arguments no optional arguments
      And declared defaults no declared defaults
      And policy scope trusted network/simulated policy
      When an authorized client invokes GET /graph/api/v1/clusters/{id}
      Then the response exposes HTTP 200, nodes[], edges[], truncated
      And side effects are none
      And idempotency is not_applicable
      And pagination or range behavior is none

    @surface-a98c056ba8ec_role_reader
    Scenario: GET /graph/api/v1/clusters/{id} as reader
      Given the canonical reader profile
      When that principal uses GET /graph/api/v1/clusters/{id}
      Then the outcome is trusted-network route; optional simulated principal applies that profile read policy before output

    @surface-a98c056ba8ec_role_proposer
    Scenario: GET /graph/api/v1/clusters/{id} as proposer
      Given the canonical proposer profile
      When that principal uses GET /graph/api/v1/clusters/{id}
      Then the outcome is trusted-network route; optional simulated principal applies that profile read policy before output

    @surface-a98c056ba8ec_role_curator
    Scenario: GET /graph/api/v1/clusters/{id} as curator
      Given the canonical curator profile
      When that principal uses GET /graph/api/v1/clusters/{id}
      Then the outcome is trusted-network route; optional simulated principal applies that profile read policy before output

    @surface-a98c056ba8ec_role_admin
    Scenario: GET /graph/api/v1/clusters/{id} as admin
      Given the canonical admin profile
      When that principal uses GET /graph/api/v1/clusters/{id}
      Then the outcome is trusted-network route; optional simulated principal applies that profile read policy before output

    @surface-a98c056ba8ec_failure
    Scenario: GET /graph/api/v1/clusters/{id} rejects invalid or conflicting requests
      Given required arguments no required arguments and declared defaults no declared defaults
      And malformed, missing, out-of-scope, unavailable, replayed, or conflicting inputs
      When the client invokes GET /graph/api/v1/clusters/{id}
      Then one of the specified failures is 400 validation, 401 auth, 403 policy where applicable, 404 unknown, 405 method, 413/415 upload bounds, 503 unavailable
      And failed pre-publication calls do not apply none

    @surface-859546743957
    Scenario: GET /graph/api/v1/memories/{id} succeeds with its declared contract
      Given required arguments no required arguments
      And optional arguments no optional arguments
      And declared defaults no declared defaults
      And policy scope trusted network/simulated policy
      When an authorized client invokes GET /graph/api/v1/memories/{id}
      Then the response exposes HTTP 200, node, preview, inbound[], outbound[], assets[], proposals[]
      And side effects are none
      And idempotency is not_applicable
      And pagination or range behavior is none

    @surface-859546743957_role_reader
    Scenario: GET /graph/api/v1/memories/{id} as reader
      Given the canonical reader profile
      When that principal uses GET /graph/api/v1/memories/{id}
      Then the outcome is trusted-network route; optional simulated principal applies that profile read policy before output

    @surface-859546743957_role_proposer
    Scenario: GET /graph/api/v1/memories/{id} as proposer
      Given the canonical proposer profile
      When that principal uses GET /graph/api/v1/memories/{id}
      Then the outcome is trusted-network route; optional simulated principal applies that profile read policy before output

    @surface-859546743957_role_curator
    Scenario: GET /graph/api/v1/memories/{id} as curator
      Given the canonical curator profile
      When that principal uses GET /graph/api/v1/memories/{id}
      Then the outcome is trusted-network route; optional simulated principal applies that profile read policy before output

    @surface-859546743957_role_admin
    Scenario: GET /graph/api/v1/memories/{id} as admin
      Given the canonical admin profile
      When that principal uses GET /graph/api/v1/memories/{id}
      Then the outcome is trusted-network route; optional simulated principal applies that profile read policy before output

    @surface-859546743957_failure
    Scenario: GET /graph/api/v1/memories/{id} rejects invalid or conflicting requests
      Given required arguments no required arguments and declared defaults no declared defaults
      And malformed, missing, out-of-scope, unavailable, replayed, or conflicting inputs
      When the client invokes GET /graph/api/v1/memories/{id}
      Then one of the specified failures is 400 validation, 401 auth, 403 policy where applicable, 404 unknown, 405 method, 413/415 upload bounds, 503 unavailable
      And failed pre-publication calls do not apply none

    @surface-12fd0752fde9
    Scenario: GET /graph/api/v1/neighbourhood/{id} succeeds with its declared contract
      Given required arguments no required arguments
      And optional arguments no optional arguments
      And declared defaults no declared defaults
      And policy scope trusted network/simulated policy
      When an authorized client invokes GET /graph/api/v1/neighbourhood/{id}
      Then the response exposes HTTP 200, center_id, nodes[], edges[], revisions, truncated
      And side effects are none
      And idempotency is not_applicable
      And pagination or range behavior is none

    @surface-12fd0752fde9_role_reader
    Scenario: GET /graph/api/v1/neighbourhood/{id} as reader
      Given the canonical reader profile
      When that principal uses GET /graph/api/v1/neighbourhood/{id}
      Then the outcome is trusted-network route; optional simulated principal applies that profile read policy before output

    @surface-12fd0752fde9_role_proposer
    Scenario: GET /graph/api/v1/neighbourhood/{id} as proposer
      Given the canonical proposer profile
      When that principal uses GET /graph/api/v1/neighbourhood/{id}
      Then the outcome is trusted-network route; optional simulated principal applies that profile read policy before output

    @surface-12fd0752fde9_role_curator
    Scenario: GET /graph/api/v1/neighbourhood/{id} as curator
      Given the canonical curator profile
      When that principal uses GET /graph/api/v1/neighbourhood/{id}
      Then the outcome is trusted-network route; optional simulated principal applies that profile read policy before output

    @surface-12fd0752fde9_role_admin
    Scenario: GET /graph/api/v1/neighbourhood/{id} as admin
      Given the canonical admin profile
      When that principal uses GET /graph/api/v1/neighbourhood/{id}
      Then the outcome is trusted-network route; optional simulated principal applies that profile read policy before output

    @surface-12fd0752fde9_failure
    Scenario: GET /graph/api/v1/neighbourhood/{id} rejects invalid or conflicting requests
      Given required arguments no required arguments and declared defaults no declared defaults
      And malformed, missing, out-of-scope, unavailable, replayed, or conflicting inputs
      When the client invokes GET /graph/api/v1/neighbourhood/{id}
      Then one of the specified failures is 400 validation, 401 auth, 403 policy where applicable, 404 unknown, 405 method, 413/415 upload bounds, 503 unavailable
      And failed pre-publication calls do not apply none

    @surface-b0340ab6792a
    Scenario: GET /graph/api/v1/assets/{path} succeeds with its declared contract
      Given required arguments no required arguments
      And optional arguments no optional arguments
      And declared defaults no declared defaults
      And policy scope trusted network
      When an authorized client invokes GET /graph/api/v1/assets/{path}
      Then the response exposes HTTP 200, static asset bytes, asset MIME type
      And side effects are none
      And idempotency is not_applicable
      And pagination or range behavior is none

    @surface-b0340ab6792a_role_reader
    Scenario: GET /graph/api/v1/assets/{path} as reader
      Given the canonical reader profile
      When that principal uses GET /graph/api/v1/assets/{path}
      Then the outcome is trusted-network route; optional simulated principal applies that profile read policy before output

    @surface-b0340ab6792a_role_proposer
    Scenario: GET /graph/api/v1/assets/{path} as proposer
      Given the canonical proposer profile
      When that principal uses GET /graph/api/v1/assets/{path}
      Then the outcome is trusted-network route; optional simulated principal applies that profile read policy before output

    @surface-b0340ab6792a_role_curator
    Scenario: GET /graph/api/v1/assets/{path} as curator
      Given the canonical curator profile
      When that principal uses GET /graph/api/v1/assets/{path}
      Then the outcome is trusted-network route; optional simulated principal applies that profile read policy before output

    @surface-b0340ab6792a_role_admin
    Scenario: GET /graph/api/v1/assets/{path} as admin
      Given the canonical admin profile
      When that principal uses GET /graph/api/v1/assets/{path}
      Then the outcome is trusted-network route; optional simulated principal applies that profile read policy before output

    @surface-b0340ab6792a_failure
    Scenario: GET /graph/api/v1/assets/{path} rejects invalid or conflicting requests
      Given required arguments no required arguments and declared defaults no declared defaults
      And malformed, missing, out-of-scope, unavailable, replayed, or conflicting inputs
      When the client invokes GET /graph/api/v1/assets/{path}
      Then one of the specified failures is 400 validation, 401 auth, 403 policy where applicable, 404 unknown, 405 method, 413/415 upload bounds, 503 unavailable
      And failed pre-publication calls do not apply none

    @surface-3baee52bbbef
    Scenario: POST /graph/api/v1/export/{json|svg} succeeds with its declared contract
      Given required arguments no required arguments
      And optional arguments no optional arguments
      And declared defaults no declared defaults
      And policy scope trusted network/simulated policy
      When an authorized client invokes POST /graph/api/v1/export/{json|svg}
      Then the response exposes HTTP 200, JSON schema/nodes/edges/settings/revisions or SVG bytes
      And side effects are none
      And idempotency is not_applicable
      And pagination or range behavior is none

    @surface-3baee52bbbef_role_reader
    Scenario: POST /graph/api/v1/export/{json|svg} as reader
      Given the canonical reader profile
      When that principal uses POST /graph/api/v1/export/{json|svg}
      Then the outcome is trusted-network route; optional simulated principal applies that profile read policy before output

    @surface-3baee52bbbef_role_proposer
    Scenario: POST /graph/api/v1/export/{json|svg} as proposer
      Given the canonical proposer profile
      When that principal uses POST /graph/api/v1/export/{json|svg}
      Then the outcome is trusted-network route; optional simulated principal applies that profile read policy before output

    @surface-3baee52bbbef_role_curator
    Scenario: POST /graph/api/v1/export/{json|svg} as curator
      Given the canonical curator profile
      When that principal uses POST /graph/api/v1/export/{json|svg}
      Then the outcome is trusted-network route; optional simulated principal applies that profile read policy before output

    @surface-3baee52bbbef_role_admin
    Scenario: POST /graph/api/v1/export/{json|svg} as admin
      Given the canonical admin profile
      When that principal uses POST /graph/api/v1/export/{json|svg}
      Then the outcome is trusted-network route; optional simulated principal applies that profile read policy before output

    @surface-3baee52bbbef_failure
    Scenario: POST /graph/api/v1/export/{json|svg} rejects invalid or conflicting requests
      Given required arguments no required arguments and declared defaults no declared defaults
      And malformed, missing, out-of-scope, unavailable, replayed, or conflicting inputs
      When the client invokes POST /graph/api/v1/export/{json|svg}
      Then one of the specified failures is 400 validation, 401 auth, 403 policy where applicable, 404 unknown, 405 method, 413/415 upload bounds, 503 unavailable
      And failed pre-publication calls do not apply none
