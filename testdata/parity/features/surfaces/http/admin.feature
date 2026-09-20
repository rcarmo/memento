Feature: http/admin

  Each operation has a success contract, four canonical role outcomes, and a failure contract.
  Stable row tags link behavior to validation data without exposing implementation details.

  Rule: http endpoint workflow

    @surface-e2485ab26cbb
    Scenario: GET /admin succeeds with its declared contract
      Given required arguments no required arguments
      And optional arguments no optional arguments
      And declared defaults no declared defaults
      And policy scope none
      When an authorized client invokes GET /admin
      Then the response exposes HTTP 200, Content-Type text/html, Cache-Control no-store, X-Content-Type-Options nosniff
      And side effects are none
      And idempotency is not_applicable
      And pagination or range behavior is none

    @surface-e2485ab26cbb_role_reader
    Scenario: GET /admin as reader
      Given the canonical reader profile
      When that principal discovers or invokes GET /admin
      Then the outcome is HTTP 200 static no-store/nosniff response when managed access is configured

    @surface-e2485ab26cbb_role_proposer
    Scenario: GET /admin as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes GET /admin
      Then the outcome is HTTP 200 static no-store/nosniff response when managed access is configured

    @surface-e2485ab26cbb_role_curator
    Scenario: GET /admin as curator
      Given the canonical curator profile
      When that principal discovers or invokes GET /admin
      Then the outcome is HTTP 200 static no-store/nosniff response when managed access is configured

    @surface-e2485ab26cbb_role_admin
    Scenario: GET /admin as admin
      Given the canonical admin profile
      When that principal discovers or invokes GET /admin
      Then the outcome is HTTP 200 static no-store/nosniff response when managed access is configured

    @surface-e2485ab26cbb_failure
    Scenario: GET /admin rejects invalid or conflicting requests
      Given required arguments no required arguments and declared defaults no declared defaults
      And malformed, missing, out-of-scope, unavailable, replayed, or conflicting inputs
      When the client invokes GET /admin
      Then one of the specified failures is 400 validation, 401 auth, 403 policy where applicable, 404 unknown, 405 method, 413/415 upload bounds, 503 unavailable
      And failed pre-publication calls do not apply none

    @surface-2c59ca52a630
    Scenario: GET /admin/app.js succeeds with its declared contract
      Given required arguments no required arguments
      And optional arguments no optional arguments
      And declared defaults no declared defaults
      And policy scope none
      When an authorized client invokes GET /admin/app.js
      Then the response exposes HTTP 200, Content-Type text/javascript, Cache-Control no-store, X-Content-Type-Options nosniff
      And side effects are none
      And idempotency is not_applicable
      And pagination or range behavior is none

    @surface-2c59ca52a630_role_reader
    Scenario: GET /admin/app.js as reader
      Given the canonical reader profile
      When that principal discovers or invokes GET /admin/app.js
      Then the outcome is HTTP 200 static no-store/nosniff response when managed access is configured

    @surface-2c59ca52a630_role_proposer
    Scenario: GET /admin/app.js as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes GET /admin/app.js
      Then the outcome is HTTP 200 static no-store/nosniff response when managed access is configured

    @surface-2c59ca52a630_role_curator
    Scenario: GET /admin/app.js as curator
      Given the canonical curator profile
      When that principal discovers or invokes GET /admin/app.js
      Then the outcome is HTTP 200 static no-store/nosniff response when managed access is configured

    @surface-2c59ca52a630_role_admin
    Scenario: GET /admin/app.js as admin
      Given the canonical admin profile
      When that principal discovers or invokes GET /admin/app.js
      Then the outcome is HTTP 200 static no-store/nosniff response when managed access is configured

    @surface-2c59ca52a630_failure
    Scenario: GET /admin/app.js rejects invalid or conflicting requests
      Given required arguments no required arguments and declared defaults no declared defaults
      And malformed, missing, out-of-scope, unavailable, replayed, or conflicting inputs
      When the client invokes GET /admin/app.js
      Then one of the specified failures is 400 validation, 401 auth, 403 policy where applicable, 404 unknown, 405 method, 413/415 upload bounds, 503 unavailable
      And failed pre-publication calls do not apply none

    @surface-527a5f06b0d9
    Scenario: GET /admin/api/principals succeeds with its declared contract
      Given required arguments no required arguments
      And optional arguments no optional arguments
      And declared defaults no declared defaults
      And policy scope admin
      When an authorized client invokes GET /admin/api/principals
      Then the response exposes HTTP 200, principals[], principals[].name, roles[], read_prefixes[], write_prefixes[], enabled, revoked, deleted, warnings[]
      And side effects are none
      And idempotency is not_applicable
      And pagination or range behavior is none

    @surface-527a5f06b0d9_role_reader
    Scenario: GET /admin/api/principals as reader
      Given the canonical reader profile
      When that principal discovers or invokes GET /admin/api/principals
      Then the outcome is HTTP 401 admin bearer credential required

    @surface-527a5f06b0d9_role_proposer
    Scenario: GET /admin/api/principals as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes GET /admin/api/principals
      Then the outcome is HTTP 401 admin bearer credential required

    @surface-527a5f06b0d9_role_curator
    Scenario: GET /admin/api/principals as curator
      Given the canonical curator profile
      When that principal discovers or invokes GET /admin/api/principals
      Then the outcome is HTTP 401 admin bearer credential required

    @surface-527a5f06b0d9_role_admin
    Scenario: GET /admin/api/principals as admin
      Given the canonical admin profile
      When that principal discovers or invokes GET /admin/api/principals
      Then the outcome is authenticated HTTP 200/201 operation

    @surface-527a5f06b0d9_failure
    Scenario: GET /admin/api/principals rejects invalid or conflicting requests
      Given required arguments no required arguments and declared defaults no declared defaults
      And malformed, missing, out-of-scope, unavailable, replayed, or conflicting inputs
      When the client invokes GET /admin/api/principals
      Then one of the specified failures is 400 validation, 401 auth, 403 policy where applicable, 404 unknown, 405 method, 413/415 upload bounds, 503 unavailable
      And failed pre-publication calls do not apply none

    @surface-9308312ce583
    Scenario: GET /admin/api/activity succeeds with its declared contract
      Given required arguments no required arguments
      And optional arguments no optional arguments
      And declared defaults no declared defaults
      And policy scope admin
      When an authorized client invokes GET /admin/api/activity
      Then the response exposes HTTP 200, events[0:50], events[].action, events[].actor, events[].target, events[].created_at
      And side effects are none
      And idempotency is not_applicable
      And pagination or range behavior is none

    @surface-9308312ce583_role_reader
    Scenario: GET /admin/api/activity as reader
      Given the canonical reader profile
      When that principal discovers or invokes GET /admin/api/activity
      Then the outcome is HTTP 401 admin bearer credential required

    @surface-9308312ce583_role_proposer
    Scenario: GET /admin/api/activity as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes GET /admin/api/activity
      Then the outcome is HTTP 401 admin bearer credential required

    @surface-9308312ce583_role_curator
    Scenario: GET /admin/api/activity as curator
      Given the canonical curator profile
      When that principal discovers or invokes GET /admin/api/activity
      Then the outcome is HTTP 401 admin bearer credential required

    @surface-9308312ce583_role_admin
    Scenario: GET /admin/api/activity as admin
      Given the canonical admin profile
      When that principal discovers or invokes GET /admin/api/activity
      Then the outcome is authenticated HTTP 200/201 operation

    @surface-9308312ce583_failure
    Scenario: GET /admin/api/activity rejects invalid or conflicting requests
      Given required arguments no required arguments and declared defaults no declared defaults
      And malformed, missing, out-of-scope, unavailable, replayed, or conflicting inputs
      When the client invokes GET /admin/api/activity
      Then one of the specified failures is 400 validation, 401 auth, 403 policy where applicable, 404 unknown, 405 method, 413/415 upload bounds, 503 unavailable
      And failed pre-publication calls do not apply none

    @surface-a7495156c5e1
    Scenario: POST /admin/api/principals succeeds with its declared contract
      Given required arguments no required arguments
      And optional arguments no optional arguments
      And declared defaults no declared defaults
      And policy scope admin
      When an authorized client invokes POST /admin/api/principals
      Then the response exposes HTTP 201, principal, credential returned once
      And side effects are queues or persists the endpoint-specific staging, refresh, access, or session transition
      And idempotency is not_applicable
      And pagination or range behavior is none

    @surface-a7495156c5e1_role_reader
    Scenario: POST /admin/api/principals as reader
      Given the canonical reader profile
      When that principal discovers or invokes POST /admin/api/principals
      Then the outcome is HTTP 401 admin bearer credential required

    @surface-a7495156c5e1_role_proposer
    Scenario: POST /admin/api/principals as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes POST /admin/api/principals
      Then the outcome is HTTP 401 admin bearer credential required

    @surface-a7495156c5e1_role_curator
    Scenario: POST /admin/api/principals as curator
      Given the canonical curator profile
      When that principal discovers or invokes POST /admin/api/principals
      Then the outcome is HTTP 401 admin bearer credential required

    @surface-a7495156c5e1_role_admin
    Scenario: POST /admin/api/principals as admin
      Given the canonical admin profile
      When that principal discovers or invokes POST /admin/api/principals
      Then the outcome is authenticated HTTP 200/201 operation

    @surface-a7495156c5e1_failure
    Scenario: POST /admin/api/principals rejects invalid or conflicting requests
      Given required arguments no required arguments and declared defaults no declared defaults
      And malformed, missing, out-of-scope, unavailable, replayed, or conflicting inputs
      When the client invokes POST /admin/api/principals
      Then one of the specified failures is 400 validation, 401 auth, 403 policy where applicable, 404 unknown, 405 method, 413/415 upload bounds, 503 unavailable
      And failed pre-publication calls do not apply queues or persists the endpoint-specific staging, refresh, access, or session transition

    @surface-30b7e1ddfa50
    Scenario: POST /admin/api/principals/{name}/update succeeds with its declared contract
      Given required arguments no required arguments
      And optional arguments no optional arguments
      And declared defaults no declared defaults
      And policy scope admin
      When an authorized client invokes POST /admin/api/principals/{name}/update
      Then the response exposes HTTP 200, principal
      And side effects are queues or persists the endpoint-specific staging, refresh, access, or session transition
      And idempotency is not_applicable
      And pagination or range behavior is none

    @surface-30b7e1ddfa50_role_reader
    Scenario: POST /admin/api/principals/{name}/update as reader
      Given the canonical reader profile
      When that principal discovers or invokes POST /admin/api/principals/{name}/update
      Then the outcome is HTTP 401 admin bearer credential required

    @surface-30b7e1ddfa50_role_proposer
    Scenario: POST /admin/api/principals/{name}/update as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes POST /admin/api/principals/{name}/update
      Then the outcome is HTTP 401 admin bearer credential required

    @surface-30b7e1ddfa50_role_curator
    Scenario: POST /admin/api/principals/{name}/update as curator
      Given the canonical curator profile
      When that principal discovers or invokes POST /admin/api/principals/{name}/update
      Then the outcome is HTTP 401 admin bearer credential required

    @surface-30b7e1ddfa50_role_admin
    Scenario: POST /admin/api/principals/{name}/update as admin
      Given the canonical admin profile
      When that principal discovers or invokes POST /admin/api/principals/{name}/update
      Then the outcome is authenticated HTTP 200/201 operation

    @surface-30b7e1ddfa50_failure
    Scenario: POST /admin/api/principals/{name}/update rejects invalid or conflicting requests
      Given required arguments no required arguments and declared defaults no declared defaults
      And malformed, missing, out-of-scope, unavailable, replayed, or conflicting inputs
      When the client invokes POST /admin/api/principals/{name}/update
      Then one of the specified failures is 400 validation, 401 auth, 403 policy where applicable, 404 unknown, 405 method, 413/415 upload bounds, 503 unavailable
      And failed pre-publication calls do not apply queues or persists the endpoint-specific staging, refresh, access, or session transition

    @surface-4c5e4a822549
    Scenario: POST /admin/api/principals/{name}/rename succeeds with its declared contract
      Given required arguments no required arguments
      And optional arguments no optional arguments
      And declared defaults no declared defaults
      And policy scope admin
      When an authorized client invokes POST /admin/api/principals/{name}/rename
      Then the response exposes HTTP 200, principal
      And side effects are queues or persists the endpoint-specific staging, refresh, access, or session transition
      And idempotency is not_applicable
      And pagination or range behavior is none

    @surface-4c5e4a822549_role_reader
    Scenario: POST /admin/api/principals/{name}/rename as reader
      Given the canonical reader profile
      When that principal discovers or invokes POST /admin/api/principals/{name}/rename
      Then the outcome is HTTP 401 admin bearer credential required

    @surface-4c5e4a822549_role_proposer
    Scenario: POST /admin/api/principals/{name}/rename as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes POST /admin/api/principals/{name}/rename
      Then the outcome is HTTP 401 admin bearer credential required

    @surface-4c5e4a822549_role_curator
    Scenario: POST /admin/api/principals/{name}/rename as curator
      Given the canonical curator profile
      When that principal discovers or invokes POST /admin/api/principals/{name}/rename
      Then the outcome is HTTP 401 admin bearer credential required

    @surface-4c5e4a822549_role_admin
    Scenario: POST /admin/api/principals/{name}/rename as admin
      Given the canonical admin profile
      When that principal discovers or invokes POST /admin/api/principals/{name}/rename
      Then the outcome is authenticated HTTP 200/201 operation

    @surface-4c5e4a822549_failure
    Scenario: POST /admin/api/principals/{name}/rename rejects invalid or conflicting requests
      Given required arguments no required arguments and declared defaults no declared defaults
      And malformed, missing, out-of-scope, unavailable, replayed, or conflicting inputs
      When the client invokes POST /admin/api/principals/{name}/rename
      Then one of the specified failures is 400 validation, 401 auth, 403 policy where applicable, 404 unknown, 405 method, 413/415 upload bounds, 503 unavailable
      And failed pre-publication calls do not apply queues or persists the endpoint-specific staging, refresh, access, or session transition

    @surface-c45eccf925c8
    Scenario: POST /admin/api/principals/{name}/disable succeeds with its declared contract
      Given required arguments no required arguments
      And optional arguments no optional arguments
      And declared defaults no declared defaults
      And policy scope admin
      When an authorized client invokes POST /admin/api/principals/{name}/disable
      Then the response exposes HTTP 200, principal.enabled=false
      And side effects are queues or persists the endpoint-specific staging, refresh, access, or session transition
      And idempotency is not_applicable
      And pagination or range behavior is none

    @surface-c45eccf925c8_role_reader
    Scenario: POST /admin/api/principals/{name}/disable as reader
      Given the canonical reader profile
      When that principal discovers or invokes POST /admin/api/principals/{name}/disable
      Then the outcome is HTTP 401 admin bearer credential required

    @surface-c45eccf925c8_role_proposer
    Scenario: POST /admin/api/principals/{name}/disable as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes POST /admin/api/principals/{name}/disable
      Then the outcome is HTTP 401 admin bearer credential required

    @surface-c45eccf925c8_role_curator
    Scenario: POST /admin/api/principals/{name}/disable as curator
      Given the canonical curator profile
      When that principal discovers or invokes POST /admin/api/principals/{name}/disable
      Then the outcome is HTTP 401 admin bearer credential required

    @surface-c45eccf925c8_role_admin
    Scenario: POST /admin/api/principals/{name}/disable as admin
      Given the canonical admin profile
      When that principal discovers or invokes POST /admin/api/principals/{name}/disable
      Then the outcome is authenticated HTTP 200/201 operation

    @surface-c45eccf925c8_failure
    Scenario: POST /admin/api/principals/{name}/disable rejects invalid or conflicting requests
      Given required arguments no required arguments and declared defaults no declared defaults
      And malformed, missing, out-of-scope, unavailable, replayed, or conflicting inputs
      When the client invokes POST /admin/api/principals/{name}/disable
      Then one of the specified failures is 400 validation, 401 auth, 403 policy where applicable, 404 unknown, 405 method, 413/415 upload bounds, 503 unavailable
      And failed pre-publication calls do not apply queues or persists the endpoint-specific staging, refresh, access, or session transition

    @surface-f8177e84f7ee
    Scenario: POST /admin/api/principals/{name}/enable succeeds with its declared contract
      Given required arguments no required arguments
      And optional arguments no optional arguments
      And declared defaults no declared defaults
      And policy scope admin
      When an authorized client invokes POST /admin/api/principals/{name}/enable
      Then the response exposes HTTP 200, principal.enabled=true
      And side effects are queues or persists the endpoint-specific staging, refresh, access, or session transition
      And idempotency is not_applicable
      And pagination or range behavior is none

    @surface-f8177e84f7ee_role_reader
    Scenario: POST /admin/api/principals/{name}/enable as reader
      Given the canonical reader profile
      When that principal discovers or invokes POST /admin/api/principals/{name}/enable
      Then the outcome is HTTP 401 admin bearer credential required

    @surface-f8177e84f7ee_role_proposer
    Scenario: POST /admin/api/principals/{name}/enable as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes POST /admin/api/principals/{name}/enable
      Then the outcome is HTTP 401 admin bearer credential required

    @surface-f8177e84f7ee_role_curator
    Scenario: POST /admin/api/principals/{name}/enable as curator
      Given the canonical curator profile
      When that principal discovers or invokes POST /admin/api/principals/{name}/enable
      Then the outcome is HTTP 401 admin bearer credential required

    @surface-f8177e84f7ee_role_admin
    Scenario: POST /admin/api/principals/{name}/enable as admin
      Given the canonical admin profile
      When that principal discovers or invokes POST /admin/api/principals/{name}/enable
      Then the outcome is authenticated HTTP 200/201 operation

    @surface-f8177e84f7ee_failure
    Scenario: POST /admin/api/principals/{name}/enable rejects invalid or conflicting requests
      Given required arguments no required arguments and declared defaults no declared defaults
      And malformed, missing, out-of-scope, unavailable, replayed, or conflicting inputs
      When the client invokes POST /admin/api/principals/{name}/enable
      Then one of the specified failures is 400 validation, 401 auth, 403 policy where applicable, 404 unknown, 405 method, 413/415 upload bounds, 503 unavailable
      And failed pre-publication calls do not apply queues or persists the endpoint-specific staging, refresh, access, or session transition

    @surface-907e8a310559
    Scenario: POST /admin/api/principals/{name}/rotate succeeds with its declared contract
      Given required arguments no required arguments
      And optional arguments no optional arguments
      And declared defaults no declared defaults
      And policy scope admin
      When an authorized client invokes POST /admin/api/principals/{name}/rotate
      Then the response exposes HTTP 200, name, credential returned once
      And side effects are queues or persists the endpoint-specific staging, refresh, access, or session transition
      And idempotency is not_applicable
      And pagination or range behavior is none

    @surface-907e8a310559_role_reader
    Scenario: POST /admin/api/principals/{name}/rotate as reader
      Given the canonical reader profile
      When that principal discovers or invokes POST /admin/api/principals/{name}/rotate
      Then the outcome is HTTP 401 admin bearer credential required

    @surface-907e8a310559_role_proposer
    Scenario: POST /admin/api/principals/{name}/rotate as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes POST /admin/api/principals/{name}/rotate
      Then the outcome is HTTP 401 admin bearer credential required

    @surface-907e8a310559_role_curator
    Scenario: POST /admin/api/principals/{name}/rotate as curator
      Given the canonical curator profile
      When that principal discovers or invokes POST /admin/api/principals/{name}/rotate
      Then the outcome is HTTP 401 admin bearer credential required

    @surface-907e8a310559_role_admin
    Scenario: POST /admin/api/principals/{name}/rotate as admin
      Given the canonical admin profile
      When that principal discovers or invokes POST /admin/api/principals/{name}/rotate
      Then the outcome is authenticated HTTP 200/201 operation

    @surface-907e8a310559_failure
    Scenario: POST /admin/api/principals/{name}/rotate rejects invalid or conflicting requests
      Given required arguments no required arguments and declared defaults no declared defaults
      And malformed, missing, out-of-scope, unavailable, replayed, or conflicting inputs
      When the client invokes POST /admin/api/principals/{name}/rotate
      Then one of the specified failures is 400 validation, 401 auth, 403 policy where applicable, 404 unknown, 405 method, 413/415 upload bounds, 503 unavailable
      And failed pre-publication calls do not apply queues or persists the endpoint-specific staging, refresh, access, or session transition

    @surface-72ace281970a
    Scenario: POST /admin/api/principals/{name}/revoke succeeds with its declared contract
      Given required arguments no required arguments
      And optional arguments no optional arguments
      And declared defaults no declared defaults
      And policy scope admin
      When an authorized client invokes POST /admin/api/principals/{name}/revoke
      Then the response exposes HTTP 200, principal.revoked=true
      And side effects are queues or persists the endpoint-specific staging, refresh, access, or session transition
      And idempotency is not_applicable
      And pagination or range behavior is none

    @surface-72ace281970a_role_reader
    Scenario: POST /admin/api/principals/{name}/revoke as reader
      Given the canonical reader profile
      When that principal discovers or invokes POST /admin/api/principals/{name}/revoke
      Then the outcome is HTTP 401 admin bearer credential required

    @surface-72ace281970a_role_proposer
    Scenario: POST /admin/api/principals/{name}/revoke as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes POST /admin/api/principals/{name}/revoke
      Then the outcome is HTTP 401 admin bearer credential required

    @surface-72ace281970a_role_curator
    Scenario: POST /admin/api/principals/{name}/revoke as curator
      Given the canonical curator profile
      When that principal discovers or invokes POST /admin/api/principals/{name}/revoke
      Then the outcome is HTTP 401 admin bearer credential required

    @surface-72ace281970a_role_admin
    Scenario: POST /admin/api/principals/{name}/revoke as admin
      Given the canonical admin profile
      When that principal discovers or invokes POST /admin/api/principals/{name}/revoke
      Then the outcome is authenticated HTTP 200/201 operation

    @surface-72ace281970a_failure
    Scenario: POST /admin/api/principals/{name}/revoke rejects invalid or conflicting requests
      Given required arguments no required arguments and declared defaults no declared defaults
      And malformed, missing, out-of-scope, unavailable, replayed, or conflicting inputs
      When the client invokes POST /admin/api/principals/{name}/revoke
      Then one of the specified failures is 400 validation, 401 auth, 403 policy where applicable, 404 unknown, 405 method, 413/415 upload bounds, 503 unavailable
      And failed pre-publication calls do not apply queues or persists the endpoint-specific staging, refresh, access, or session transition

    @surface-43af9f6a33f7
    Scenario: POST /admin/api/principals/{name}/delete succeeds with its declared contract
      Given required arguments no required arguments
      And optional arguments no optional arguments
      And declared defaults no declared defaults
      And policy scope admin
      When an authorized client invokes POST /admin/api/principals/{name}/delete
      Then the response exposes HTTP 200, principal.deleted=true
      And side effects are queues or persists the endpoint-specific staging, refresh, access, or session transition
      And idempotency is not_applicable
      And pagination or range behavior is none

    @surface-43af9f6a33f7_role_reader
    Scenario: POST /admin/api/principals/{name}/delete as reader
      Given the canonical reader profile
      When that principal discovers or invokes POST /admin/api/principals/{name}/delete
      Then the outcome is HTTP 401 admin bearer credential required

    @surface-43af9f6a33f7_role_proposer
    Scenario: POST /admin/api/principals/{name}/delete as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes POST /admin/api/principals/{name}/delete
      Then the outcome is HTTP 401 admin bearer credential required

    @surface-43af9f6a33f7_role_curator
    Scenario: POST /admin/api/principals/{name}/delete as curator
      Given the canonical curator profile
      When that principal discovers or invokes POST /admin/api/principals/{name}/delete
      Then the outcome is HTTP 401 admin bearer credential required

    @surface-43af9f6a33f7_role_admin
    Scenario: POST /admin/api/principals/{name}/delete as admin
      Given the canonical admin profile
      When that principal discovers or invokes POST /admin/api/principals/{name}/delete
      Then the outcome is authenticated HTTP 200/201 operation

    @surface-43af9f6a33f7_failure
    Scenario: POST /admin/api/principals/{name}/delete rejects invalid or conflicting requests
      Given required arguments no required arguments and declared defaults no declared defaults
      And malformed, missing, out-of-scope, unavailable, replayed, or conflicting inputs
      When the client invokes POST /admin/api/principals/{name}/delete
      Then one of the specified failures is 400 validation, 401 auth, 403 policy where applicable, 404 unknown, 405 method, 413/415 upload bounds, 503 unavailable
      And failed pre-publication calls do not apply queues or persists the endpoint-specific staging, refresh, access, or session transition
