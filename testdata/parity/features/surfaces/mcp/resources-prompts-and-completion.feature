Feature: mcp/resources prompts and completion

  Each operation has a success contract, four canonical role outcomes, and a failure contract.
  Stable row tags link behavior to validation data without exposing implementation details.

  Rule: mcp resource workflow

    @surface-3c11e2ff2637
    Scenario: memory://catalog succeeds with its declared contract
      Given required arguments no required arguments
      And optional arguments no optional arguments
      And declared defaults no declared defaults
      And policy scope authenticated session/principal where transport requires it
      When an authorized client invokes memory://catalog
      Then the response exposes uri/name/description metadata or deterministic prompt/resource content declared by the Python registry
      And side effects are none
      And idempotency is not_applicable
      And pagination or range behavior is none

    @surface-3c11e2ff2637_role_reader
    Scenario: memory://catalog as reader
      Given the canonical reader profile
      When that principal discovers or invokes memory://catalog
      Then the outcome is read_or_get

    @surface-3c11e2ff2637_role_proposer
    Scenario: memory://catalog as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes memory://catalog
      Then the outcome is read_or_get

    @surface-3c11e2ff2637_role_curator
    Scenario: memory://catalog as curator
      Given the canonical curator profile
      When that principal discovers or invokes memory://catalog
      Then the outcome is read_or_get

    @surface-3c11e2ff2637_role_admin
    Scenario: memory://catalog as admin
      Given the canonical admin profile
      When that principal discovers or invokes memory://catalog
      Then the outcome is read_or_get

    @surface-3c11e2ff2637_failure
    Scenario: memory://catalog rejects invalid or conflicting requests
      Given required arguments no required arguments and declared defaults no declared defaults
      And malformed, missing, out-of-scope, unavailable, replayed, or conflicting inputs
      When the client invokes memory://catalog
      Then one of the specified failures is JSON-RPC protocol/invalid-params/method-not-found/internal error as observed
      And failed pre-publication calls do not apply none

    @surface-04b7216c645e
    Scenario: memory://help succeeds with its declared contract
      Given required arguments no required arguments
      And optional arguments no optional arguments
      And declared defaults no declared defaults
      And policy scope authenticated session/principal where transport requires it
      When an authorized client invokes memory://help
      Then the response exposes uri/name/description metadata or deterministic prompt/resource content declared by the Python registry
      And side effects are none
      And idempotency is not_applicable
      And pagination or range behavior is none

    @surface-04b7216c645e_role_reader
    Scenario: memory://help as reader
      Given the canonical reader profile
      When that principal discovers or invokes memory://help
      Then the outcome is read_or_get

    @surface-04b7216c645e_role_proposer
    Scenario: memory://help as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes memory://help
      Then the outcome is read_or_get

    @surface-04b7216c645e_role_curator
    Scenario: memory://help as curator
      Given the canonical curator profile
      When that principal discovers or invokes memory://help
      Then the outcome is read_or_get

    @surface-04b7216c645e_role_admin
    Scenario: memory://help as admin
      Given the canonical admin profile
      When that principal discovers or invokes memory://help
      Then the outcome is read_or_get

    @surface-04b7216c645e_failure
    Scenario: memory://help rejects invalid or conflicting requests
      Given required arguments no required arguments and declared defaults no declared defaults
      And malformed, missing, out-of-scope, unavailable, replayed, or conflicting inputs
      When the client invokes memory://help
      Then one of the specified failures is JSON-RPC protocol/invalid-params/method-not-found/internal error as observed
      And failed pre-publication calls do not apply none

    @surface-207481a8a1b4
    Scenario: memory://status succeeds with its declared contract
      Given required arguments no required arguments
      And optional arguments no optional arguments
      And declared defaults no declared defaults
      And policy scope authenticated session/principal where transport requires it
      When an authorized client invokes memory://status
      Then the response exposes uri/name/description metadata or deterministic prompt/resource content declared by the Python registry
      And side effects are none
      And idempotency is not_applicable
      And pagination or range behavior is none

    @surface-207481a8a1b4_role_reader
    Scenario: memory://status as reader
      Given the canonical reader profile
      When that principal discovers or invokes memory://status
      Then the outcome is read_or_get

    @surface-207481a8a1b4_role_proposer
    Scenario: memory://status as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes memory://status
      Then the outcome is read_or_get

    @surface-207481a8a1b4_role_curator
    Scenario: memory://status as curator
      Given the canonical curator profile
      When that principal discovers or invokes memory://status
      Then the outcome is read_or_get

    @surface-207481a8a1b4_role_admin
    Scenario: memory://status as admin
      Given the canonical admin profile
      When that principal discovers or invokes memory://status
      Then the outcome is read_or_get

    @surface-207481a8a1b4_failure
    Scenario: memory://status rejects invalid or conflicting requests
      Given required arguments no required arguments and declared defaults no declared defaults
      And malformed, missing, out-of-scope, unavailable, replayed, or conflicting inputs
      When the client invokes memory://status
      Then one of the specified failures is JSON-RPC protocol/invalid-params/method-not-found/internal error as observed
      And failed pre-publication calls do not apply none

  Rule: mcp resource template workflow

    @surface-1c393ba53964
    Scenario: memory://catalog/{operation} succeeds with its declared contract
      Given required arguments no required arguments
      And optional arguments no optional arguments
      And declared defaults no declared defaults
      And policy scope authenticated session/principal where transport requires it
      When an authorized client invokes memory://catalog/{operation}
      Then the response exposes uri/name/description metadata or deterministic prompt/resource content declared by the Python registry
      And side effects are none
      And idempotency is not_applicable
      And pagination or range behavior is none

    @surface-1c393ba53964_role_reader
    Scenario: memory://catalog/{operation} as reader
      Given the canonical reader profile
      When that principal discovers or invokes memory://catalog/{operation}
      Then the outcome is read_or_get

    @surface-1c393ba53964_role_proposer
    Scenario: memory://catalog/{operation} as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes memory://catalog/{operation}
      Then the outcome is read_or_get

    @surface-1c393ba53964_role_curator
    Scenario: memory://catalog/{operation} as curator
      Given the canonical curator profile
      When that principal discovers or invokes memory://catalog/{operation}
      Then the outcome is read_or_get

    @surface-1c393ba53964_role_admin
    Scenario: memory://catalog/{operation} as admin
      Given the canonical admin profile
      When that principal discovers or invokes memory://catalog/{operation}
      Then the outcome is read_or_get

    @surface-1c393ba53964_failure
    Scenario: memory://catalog/{operation} rejects invalid or conflicting requests
      Given required arguments no required arguments and declared defaults no declared defaults
      And malformed, missing, out-of-scope, unavailable, replayed, or conflicting inputs
      When the client invokes memory://catalog/{operation}
      Then one of the specified failures is JSON-RPC protocol/invalid-params/method-not-found/internal error as observed
      And failed pre-publication calls do not apply none

    @surface-b444c275502a
    Scenario: memory://workflow/{goal} succeeds with its declared contract
      Given required arguments no required arguments
      And optional arguments no optional arguments
      And declared defaults no declared defaults
      And policy scope authenticated session/principal where transport requires it
      When an authorized client invokes memory://workflow/{goal}
      Then the response exposes uri/name/description metadata or deterministic prompt/resource content declared by the Python registry
      And side effects are none
      And idempotency is not_applicable
      And pagination or range behavior is none

    @surface-b444c275502a_role_reader
    Scenario: memory://workflow/{goal} as reader
      Given the canonical reader profile
      When that principal discovers or invokes memory://workflow/{goal}
      Then the outcome is read_or_get

    @surface-b444c275502a_role_proposer
    Scenario: memory://workflow/{goal} as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes memory://workflow/{goal}
      Then the outcome is read_or_get

    @surface-b444c275502a_role_curator
    Scenario: memory://workflow/{goal} as curator
      Given the canonical curator profile
      When that principal discovers or invokes memory://workflow/{goal}
      Then the outcome is read_or_get

    @surface-b444c275502a_role_admin
    Scenario: memory://workflow/{goal} as admin
      Given the canonical admin profile
      When that principal discovers or invokes memory://workflow/{goal}
      Then the outcome is read_or_get

    @surface-b444c275502a_failure
    Scenario: memory://workflow/{goal} rejects invalid or conflicting requests
      Given required arguments no required arguments and declared defaults no declared defaults
      And malformed, missing, out-of-scope, unavailable, replayed, or conflicting inputs
      When the client invokes memory://workflow/{goal}
      Then one of the specified failures is JSON-RPC protocol/invalid-params/method-not-found/internal error as observed
      And failed pre-publication calls do not apply none

  Rule: mcp prompt workflow

    @surface-467db6735b44
    Scenario: publish_asset_pack succeeds with its declared contract
      Given required arguments no required arguments
      And optional arguments asset_kind, target_path, version
      And declared defaults no declared defaults
      And policy scope authenticated session/principal where transport requires it
      When an authorized client invokes publish_asset_pack
      Then the response exposes uri/name/description metadata or deterministic prompt/resource content declared by the Python registry
      And side effects are none
      And idempotency is not_applicable
      And pagination or range behavior is none

    @surface-467db6735b44_role_reader
    Scenario: publish_asset_pack as reader
      Given the canonical reader profile
      When that principal discovers or invokes publish_asset_pack
      Then the outcome is read_or_get

    @surface-467db6735b44_role_proposer
    Scenario: publish_asset_pack as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes publish_asset_pack
      Then the outcome is read_or_get

    @surface-467db6735b44_role_curator
    Scenario: publish_asset_pack as curator
      Given the canonical curator profile
      When that principal discovers or invokes publish_asset_pack
      Then the outcome is read_or_get

    @surface-467db6735b44_role_admin
    Scenario: publish_asset_pack as admin
      Given the canonical admin profile
      When that principal discovers or invokes publish_asset_pack
      Then the outcome is read_or_get

    @surface-467db6735b44_failure
    Scenario: publish_asset_pack rejects invalid or conflicting requests
      Given required arguments no required arguments and declared defaults no declared defaults
      And malformed, missing, out-of-scope, unavailable, replayed, or conflicting inputs
      When the client invokes publish_asset_pack
      Then one of the specified failures is JSON-RPC protocol/invalid-params/method-not-found/internal error as observed
      And failed pre-publication calls do not apply none

  Rule: mcp protocol workflow

    @surface-14b778244dc4
    Scenario: resources/list succeeds with its declared contract
      Given required arguments no required arguments
      And optional arguments no optional arguments
      And declared defaults no declared defaults
      And policy scope authenticated session/principal where transport requires it
      When an authorized client invokes resources/list
      Then the response exposes resources[], nextCursor
      And side effects are none
      And idempotency is not_applicable
      And pagination or range behavior is none

    @surface-14b778244dc4_role_reader
    Scenario: resources/list as reader
      Given the canonical reader profile
      When that principal discovers or invokes resources/list
      Then the outcome is available_after_successful_initialize_and_authentication

    @surface-14b778244dc4_role_proposer
    Scenario: resources/list as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes resources/list
      Then the outcome is available_after_successful_initialize_and_authentication

    @surface-14b778244dc4_role_curator
    Scenario: resources/list as curator
      Given the canonical curator profile
      When that principal discovers or invokes resources/list
      Then the outcome is available_after_successful_initialize_and_authentication

    @surface-14b778244dc4_role_admin
    Scenario: resources/list as admin
      Given the canonical admin profile
      When that principal discovers or invokes resources/list
      Then the outcome is available_after_successful_initialize_and_authentication

    @surface-14b778244dc4_failure
    Scenario: resources/list rejects invalid or conflicting requests
      Given required arguments no required arguments and declared defaults no declared defaults
      And malformed, missing, out-of-scope, unavailable, replayed, or conflicting inputs
      When the client invokes resources/list
      Then one of the specified failures is JSON-RPC protocol/invalid-params/method-not-found/internal error as observed
      And failed pre-publication calls do not apply none

    @surface-71d2cfba6cf4
    Scenario: resources/templates/list succeeds with its declared contract
      Given required arguments no required arguments
      And optional arguments no optional arguments
      And declared defaults no declared defaults
      And policy scope authenticated session/principal where transport requires it
      When an authorized client invokes resources/templates/list
      Then the response exposes resourceTemplates[], nextCursor
      And side effects are none
      And idempotency is not_applicable
      And pagination or range behavior is none

    @surface-71d2cfba6cf4_role_reader
    Scenario: resources/templates/list as reader
      Given the canonical reader profile
      When that principal discovers or invokes resources/templates/list
      Then the outcome is available_after_successful_initialize_and_authentication

    @surface-71d2cfba6cf4_role_proposer
    Scenario: resources/templates/list as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes resources/templates/list
      Then the outcome is available_after_successful_initialize_and_authentication

    @surface-71d2cfba6cf4_role_curator
    Scenario: resources/templates/list as curator
      Given the canonical curator profile
      When that principal discovers or invokes resources/templates/list
      Then the outcome is available_after_successful_initialize_and_authentication

    @surface-71d2cfba6cf4_role_admin
    Scenario: resources/templates/list as admin
      Given the canonical admin profile
      When that principal discovers or invokes resources/templates/list
      Then the outcome is available_after_successful_initialize_and_authentication

    @surface-71d2cfba6cf4_failure
    Scenario: resources/templates/list rejects invalid or conflicting requests
      Given required arguments no required arguments and declared defaults no declared defaults
      And malformed, missing, out-of-scope, unavailable, replayed, or conflicting inputs
      When the client invokes resources/templates/list
      Then one of the specified failures is JSON-RPC protocol/invalid-params/method-not-found/internal error as observed
      And failed pre-publication calls do not apply none

    @surface-cd015dc35a80
    Scenario: resources/read succeeds with its declared contract
      Given required arguments no required arguments
      And optional arguments no optional arguments
      And declared defaults no declared defaults
      And policy scope authenticated session/principal where transport requires it
      When an authorized client invokes resources/read
      Then the response exposes contents[], contents[].uri, contents[].mimeType, contents[].text or blob
      And side effects are none
      And idempotency is not_applicable
      And pagination or range behavior is none

    @surface-cd015dc35a80_role_reader
    Scenario: resources/read as reader
      Given the canonical reader profile
      When that principal discovers or invokes resources/read
      Then the outcome is available_after_successful_initialize_and_authentication

    @surface-cd015dc35a80_role_proposer
    Scenario: resources/read as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes resources/read
      Then the outcome is available_after_successful_initialize_and_authentication

    @surface-cd015dc35a80_role_curator
    Scenario: resources/read as curator
      Given the canonical curator profile
      When that principal discovers or invokes resources/read
      Then the outcome is available_after_successful_initialize_and_authentication

    @surface-cd015dc35a80_role_admin
    Scenario: resources/read as admin
      Given the canonical admin profile
      When that principal discovers or invokes resources/read
      Then the outcome is available_after_successful_initialize_and_authentication

    @surface-cd015dc35a80_failure
    Scenario: resources/read rejects invalid or conflicting requests
      Given required arguments no required arguments and declared defaults no declared defaults
      And malformed, missing, out-of-scope, unavailable, replayed, or conflicting inputs
      When the client invokes resources/read
      Then one of the specified failures is JSON-RPC protocol/invalid-params/method-not-found/internal error as observed
      And failed pre-publication calls do not apply none

    @surface-43c3e58569f9
    Scenario: resources/subscribe succeeds with its declared contract
      Given required arguments no required arguments
      And optional arguments no optional arguments
      And declared defaults no declared defaults
      And policy scope authenticated session/principal where transport requires it
      When an authorized client invokes resources/subscribe
      Then the response exposes empty JSON-RPC result; later notifications/resources/updated
      And side effects are none
      And idempotency is not_applicable
      And pagination or range behavior is none

    @surface-43c3e58569f9_role_reader
    Scenario: resources/subscribe as reader
      Given the canonical reader profile
      When that principal discovers or invokes resources/subscribe
      Then the outcome is available_after_successful_initialize_and_authentication

    @surface-43c3e58569f9_role_proposer
    Scenario: resources/subscribe as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes resources/subscribe
      Then the outcome is available_after_successful_initialize_and_authentication

    @surface-43c3e58569f9_role_curator
    Scenario: resources/subscribe as curator
      Given the canonical curator profile
      When that principal discovers or invokes resources/subscribe
      Then the outcome is available_after_successful_initialize_and_authentication

    @surface-43c3e58569f9_role_admin
    Scenario: resources/subscribe as admin
      Given the canonical admin profile
      When that principal discovers or invokes resources/subscribe
      Then the outcome is available_after_successful_initialize_and_authentication

    @surface-43c3e58569f9_failure
    Scenario: resources/subscribe rejects invalid or conflicting requests
      Given required arguments no required arguments and declared defaults no declared defaults
      And malformed, missing, out-of-scope, unavailable, replayed, or conflicting inputs
      When the client invokes resources/subscribe
      Then one of the specified failures is JSON-RPC protocol/invalid-params/method-not-found/internal error as observed
      And failed pre-publication calls do not apply none

    @surface-88921c268313
    Scenario: resources/unsubscribe succeeds with its declared contract
      Given required arguments no required arguments
      And optional arguments no optional arguments
      And declared defaults no declared defaults
      And policy scope authenticated session/principal where transport requires it
      When an authorized client invokes resources/unsubscribe
      Then the response exposes empty JSON-RPC result
      And side effects are none
      And idempotency is not_applicable
      And pagination or range behavior is none

    @surface-88921c268313_role_reader
    Scenario: resources/unsubscribe as reader
      Given the canonical reader profile
      When that principal discovers or invokes resources/unsubscribe
      Then the outcome is available_after_successful_initialize_and_authentication

    @surface-88921c268313_role_proposer
    Scenario: resources/unsubscribe as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes resources/unsubscribe
      Then the outcome is available_after_successful_initialize_and_authentication

    @surface-88921c268313_role_curator
    Scenario: resources/unsubscribe as curator
      Given the canonical curator profile
      When that principal discovers or invokes resources/unsubscribe
      Then the outcome is available_after_successful_initialize_and_authentication

    @surface-88921c268313_role_admin
    Scenario: resources/unsubscribe as admin
      Given the canonical admin profile
      When that principal discovers or invokes resources/unsubscribe
      Then the outcome is available_after_successful_initialize_and_authentication

    @surface-88921c268313_failure
    Scenario: resources/unsubscribe rejects invalid or conflicting requests
      Given required arguments no required arguments and declared defaults no declared defaults
      And malformed, missing, out-of-scope, unavailable, replayed, or conflicting inputs
      When the client invokes resources/unsubscribe
      Then one of the specified failures is JSON-RPC protocol/invalid-params/method-not-found/internal error as observed
      And failed pre-publication calls do not apply none

    @surface-6065f5d4ac68
    Scenario: prompts/list succeeds with its declared contract
      Given required arguments no required arguments
      And optional arguments no optional arguments
      And declared defaults no declared defaults
      And policy scope authenticated session/principal where transport requires it
      When an authorized client invokes prompts/list
      Then the response exposes prompts[], nextCursor
      And side effects are none
      And idempotency is not_applicable
      And pagination or range behavior is none

    @surface-6065f5d4ac68_role_reader
    Scenario: prompts/list as reader
      Given the canonical reader profile
      When that principal discovers or invokes prompts/list
      Then the outcome is available_after_successful_initialize_and_authentication

    @surface-6065f5d4ac68_role_proposer
    Scenario: prompts/list as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes prompts/list
      Then the outcome is available_after_successful_initialize_and_authentication

    @surface-6065f5d4ac68_role_curator
    Scenario: prompts/list as curator
      Given the canonical curator profile
      When that principal discovers or invokes prompts/list
      Then the outcome is available_after_successful_initialize_and_authentication

    @surface-6065f5d4ac68_role_admin
    Scenario: prompts/list as admin
      Given the canonical admin profile
      When that principal discovers or invokes prompts/list
      Then the outcome is available_after_successful_initialize_and_authentication

    @surface-6065f5d4ac68_failure
    Scenario: prompts/list rejects invalid or conflicting requests
      Given required arguments no required arguments and declared defaults no declared defaults
      And malformed, missing, out-of-scope, unavailable, replayed, or conflicting inputs
      When the client invokes prompts/list
      Then one of the specified failures is JSON-RPC protocol/invalid-params/method-not-found/internal error as observed
      And failed pre-publication calls do not apply none

    @surface-6c46d6f12550
    Scenario: prompts/get succeeds with its declared contract
      Given required arguments no required arguments
      And optional arguments no optional arguments
      And declared defaults no declared defaults
      And policy scope authenticated session/principal where transport requires it
      When an authorized client invokes prompts/get
      Then the response exposes description, messages[], messages[].role, messages[].content
      And side effects are none
      And idempotency is not_applicable
      And pagination or range behavior is none

    @surface-6c46d6f12550_role_reader
    Scenario: prompts/get as reader
      Given the canonical reader profile
      When that principal discovers or invokes prompts/get
      Then the outcome is available_after_successful_initialize_and_authentication

    @surface-6c46d6f12550_role_proposer
    Scenario: prompts/get as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes prompts/get
      Then the outcome is available_after_successful_initialize_and_authentication

    @surface-6c46d6f12550_role_curator
    Scenario: prompts/get as curator
      Given the canonical curator profile
      When that principal discovers or invokes prompts/get
      Then the outcome is available_after_successful_initialize_and_authentication

    @surface-6c46d6f12550_role_admin
    Scenario: prompts/get as admin
      Given the canonical admin profile
      When that principal discovers or invokes prompts/get
      Then the outcome is available_after_successful_initialize_and_authentication

    @surface-6c46d6f12550_failure
    Scenario: prompts/get rejects invalid or conflicting requests
      Given required arguments no required arguments and declared defaults no declared defaults
      And malformed, missing, out-of-scope, unavailable, replayed, or conflicting inputs
      When the client invokes prompts/get
      Then one of the specified failures is JSON-RPC protocol/invalid-params/method-not-found/internal error as observed
      And failed pre-publication calls do not apply none

    @surface-f17b3fa09404
    Scenario: completion/complete succeeds with its declared contract
      Given required arguments no required arguments
      And optional arguments no optional arguments
      And declared defaults no declared defaults
      And policy scope authenticated session/principal where transport requires it
      When an authorized client invokes completion/complete
      Then the response exposes completion.values[], completion.total, completion.hasMore
      And side effects are none
      And idempotency is not_applicable
      And pagination or range behavior is none

    @surface-f17b3fa09404_role_reader
    Scenario: completion/complete as reader
      Given the canonical reader profile
      When that principal discovers or invokes completion/complete
      Then the outcome is available_after_successful_initialize_and_authentication

    @surface-f17b3fa09404_role_proposer
    Scenario: completion/complete as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes completion/complete
      Then the outcome is available_after_successful_initialize_and_authentication

    @surface-f17b3fa09404_role_curator
    Scenario: completion/complete as curator
      Given the canonical curator profile
      When that principal discovers or invokes completion/complete
      Then the outcome is available_after_successful_initialize_and_authentication

    @surface-f17b3fa09404_role_admin
    Scenario: completion/complete as admin
      Given the canonical admin profile
      When that principal discovers or invokes completion/complete
      Then the outcome is available_after_successful_initialize_and_authentication

    @surface-f17b3fa09404_failure
    Scenario: completion/complete rejects invalid or conflicting requests
      Given required arguments no required arguments and declared defaults no declared defaults
      And malformed, missing, out-of-scope, unavailable, replayed, or conflicting inputs
      When the client invokes completion/complete
      Then one of the specified failures is JSON-RPC protocol/invalid-params/method-not-found/internal error as observed
      And failed pre-publication calls do not apply none
