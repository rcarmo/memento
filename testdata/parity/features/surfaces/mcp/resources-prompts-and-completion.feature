Feature: mcp/resources prompts and completion

  Each operation has a success contract, four canonical role outcomes, and a failure contract.

  Rule: mcp resource workflow

    @surface-3c11e2ff2637 @go_TestCatalogResourceProtocolReference @go_TestCatalogResourcesHTTPAndHandlerOwnership
    Scenario: memory://catalog preserves the Python success contract
      Given the Python request contract [object Object]
      When an authorized client invokes memory://catalog
      Then the response matches [object Object]

    @surface-3c11e2ff2637_role_reader @go_TestCatalogResourceProtocolReference @go_TestCatalogResourcesHTTPAndHandlerOwnership
    Scenario: memory://catalog as reader
      Given the canonical reader profile
      When that principal discovers or invokes memory://catalog
      Then the Python outcome is read_or_get

    @surface-3c11e2ff2637_role_proposer @go_TestCatalogResourceProtocolReference @go_TestCatalogResourcesHTTPAndHandlerOwnership
    Scenario: memory://catalog as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes memory://catalog
      Then the Python outcome is read_or_get

    @surface-3c11e2ff2637_role_curator @go_TestCatalogResourceProtocolReference @go_TestCatalogResourcesHTTPAndHandlerOwnership
    Scenario: memory://catalog as curator
      Given the canonical curator profile
      When that principal discovers or invokes memory://catalog
      Then the Python outcome is read_or_get

    @surface-3c11e2ff2637_role_admin @go_TestCatalogResourceProtocolReference @go_TestCatalogResourcesHTTPAndHandlerOwnership
    Scenario: memory://catalog as admin
      Given the canonical admin profile
      When that principal discovers or invokes memory://catalog
      Then the Python outcome is read_or_get

    @surface-3c11e2ff2637_failure @go_TestCatalogResourceProtocolReference @go_TestCatalogResourcesHTTPAndHandlerOwnership
    Scenario: memory://catalog preserves Python validation and failure behavior
      Given malformed, missing, out-of-scope, or unavailable inputs
      When the client invokes memory://catalog
      Then authentication, unknown resource, serialization failure

    @surface-04b7216c645e @go_TestCatalogResourceProtocolReference @go_TestCatalogResourcesHTTPAndHandlerOwnership
    Scenario: memory://help preserves the Python success contract
      Given the Python request contract [object Object]
      When an authorized client invokes memory://help
      Then the response matches [object Object]

    @surface-04b7216c645e_role_reader @go_TestCatalogResourceProtocolReference @go_TestCatalogResourcesHTTPAndHandlerOwnership
    Scenario: memory://help as reader
      Given the canonical reader profile
      When that principal discovers or invokes memory://help
      Then the Python outcome is read_or_get

    @surface-04b7216c645e_role_proposer @go_TestCatalogResourceProtocolReference @go_TestCatalogResourcesHTTPAndHandlerOwnership
    Scenario: memory://help as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes memory://help
      Then the Python outcome is read_or_get

    @surface-04b7216c645e_role_curator @go_TestCatalogResourceProtocolReference @go_TestCatalogResourcesHTTPAndHandlerOwnership
    Scenario: memory://help as curator
      Given the canonical curator profile
      When that principal discovers or invokes memory://help
      Then the Python outcome is read_or_get

    @surface-04b7216c645e_role_admin @go_TestCatalogResourceProtocolReference @go_TestCatalogResourcesHTTPAndHandlerOwnership
    Scenario: memory://help as admin
      Given the canonical admin profile
      When that principal discovers or invokes memory://help
      Then the Python outcome is read_or_get

    @surface-04b7216c645e_failure @go_TestCatalogResourceProtocolReference @go_TestCatalogResourcesHTTPAndHandlerOwnership
    Scenario: memory://help preserves Python validation and failure behavior
      Given malformed, missing, out-of-scope, or unavailable inputs
      When the client invokes memory://help
      Then authentication, unknown resource, serialization failure

    @surface-207481a8a1b4 @go_TestCatalogResourceProtocolReference @go_TestCatalogResourcesHTTPAndHandlerOwnership
    Scenario: memory://status preserves the Python success contract
      Given the Python request contract [object Object]
      When an authorized client invokes memory://status
      Then the response matches [object Object]

    @surface-207481a8a1b4_role_reader @go_TestCatalogResourceProtocolReference @go_TestCatalogResourcesHTTPAndHandlerOwnership
    Scenario: memory://status as reader
      Given the canonical reader profile
      When that principal discovers or invokes memory://status
      Then the Python outcome is read_or_get

    @surface-207481a8a1b4_role_proposer @go_TestCatalogResourceProtocolReference @go_TestCatalogResourcesHTTPAndHandlerOwnership
    Scenario: memory://status as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes memory://status
      Then the Python outcome is read_or_get

    @surface-207481a8a1b4_role_curator @go_TestCatalogResourceProtocolReference @go_TestCatalogResourcesHTTPAndHandlerOwnership
    Scenario: memory://status as curator
      Given the canonical curator profile
      When that principal discovers or invokes memory://status
      Then the Python outcome is read_or_get

    @surface-207481a8a1b4_role_admin @go_TestCatalogResourceProtocolReference @go_TestCatalogResourcesHTTPAndHandlerOwnership
    Scenario: memory://status as admin
      Given the canonical admin profile
      When that principal discovers or invokes memory://status
      Then the Python outcome is read_or_get

    @surface-207481a8a1b4_failure @go_TestCatalogResourceProtocolReference @go_TestCatalogResourcesHTTPAndHandlerOwnership
    Scenario: memory://status preserves Python validation and failure behavior
      Given malformed, missing, out-of-scope, or unavailable inputs
      When the client invokes memory://status
      Then authentication, unknown resource, serialization failure

  Rule: mcp resource template workflow

    @surface-1c393ba53964 @go_TestCatalogResourceProtocolReference @go_TestCatalogResourceProtocolReference
    Scenario: memory://catalog/{operation} preserves the Python success contract
      Given the Python request contract template variable constrained to source enum
      When an authorized client invokes memory://catalog/{operation}
      Then the response matches [object Object]

    @surface-1c393ba53964_role_reader @go_TestCatalogResourceProtocolReference @go_TestCatalogResourceProtocolReference
    Scenario: memory://catalog/{operation} as reader
      Given the canonical reader profile
      When that principal discovers or invokes memory://catalog/{operation}
      Then the Python outcome is read_or_get

    @surface-1c393ba53964_role_proposer @go_TestCatalogResourceProtocolReference @go_TestCatalogResourceProtocolReference
    Scenario: memory://catalog/{operation} as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes memory://catalog/{operation}
      Then the Python outcome is read_or_get

    @surface-1c393ba53964_role_curator @go_TestCatalogResourceProtocolReference @go_TestCatalogResourceProtocolReference
    Scenario: memory://catalog/{operation} as curator
      Given the canonical curator profile
      When that principal discovers or invokes memory://catalog/{operation}
      Then the Python outcome is read_or_get

    @surface-1c393ba53964_role_admin @go_TestCatalogResourceProtocolReference @go_TestCatalogResourceProtocolReference
    Scenario: memory://catalog/{operation} as admin
      Given the canonical admin profile
      When that principal discovers or invokes memory://catalog/{operation}
      Then the Python outcome is read_or_get

    @surface-1c393ba53964_failure @go_TestCatalogResourceProtocolReference @go_TestCatalogResourceProtocolReference
    Scenario: memory://catalog/{operation} preserves Python validation and failure behavior
      Given malformed, missing, out-of-scope, or unavailable inputs
      When the client invokes memory://catalog/{operation}
      Then unknown enum value/resource

    @surface-b444c275502a @go_TestCatalogResourceProtocolReference @go_TestCatalogResourceProtocolReference
    Scenario: memory://workflow/{goal} preserves the Python success contract
      Given the Python request contract template variable constrained to source enum
      When an authorized client invokes memory://workflow/{goal}
      Then the response matches [object Object]

    @surface-b444c275502a_role_reader @go_TestCatalogResourceProtocolReference @go_TestCatalogResourceProtocolReference
    Scenario: memory://workflow/{goal} as reader
      Given the canonical reader profile
      When that principal discovers or invokes memory://workflow/{goal}
      Then the Python outcome is read_or_get

    @surface-b444c275502a_role_proposer @go_TestCatalogResourceProtocolReference @go_TestCatalogResourceProtocolReference
    Scenario: memory://workflow/{goal} as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes memory://workflow/{goal}
      Then the Python outcome is read_or_get

    @surface-b444c275502a_role_curator @go_TestCatalogResourceProtocolReference @go_TestCatalogResourceProtocolReference
    Scenario: memory://workflow/{goal} as curator
      Given the canonical curator profile
      When that principal discovers or invokes memory://workflow/{goal}
      Then the Python outcome is read_or_get

    @surface-b444c275502a_role_admin @go_TestCatalogResourceProtocolReference @go_TestCatalogResourceProtocolReference
    Scenario: memory://workflow/{goal} as admin
      Given the canonical admin profile
      When that principal discovers or invokes memory://workflow/{goal}
      Then the Python outcome is read_or_get

    @surface-b444c275502a_failure @go_TestCatalogResourceProtocolReference @go_TestCatalogResourceProtocolReference
    Scenario: memory://workflow/{goal} preserves Python validation and failure behavior
      Given malformed, missing, out-of-scope, or unavailable inputs
      When the client invokes memory://workflow/{goal}
      Then unknown enum value/resource

  Rule: mcp prompt workflow

    @surface-467db6735b44 @go_TestCatalogReference @go_TestCatalogResourceProtocolReference
    Scenario: publish_asset_pack preserves the Python success contract
      Given the Python request contract [object Object]
      When an authorized client invokes publish_asset_pack
      Then the response matches one user prompt message with deterministic asset workflow instructions

    @surface-467db6735b44_role_reader @go_TestCatalogReference @go_TestCatalogResourceProtocolReference
    Scenario: publish_asset_pack as reader
      Given the canonical reader profile
      When that principal discovers or invokes publish_asset_pack
      Then the Python outcome is read_or_get

    @surface-467db6735b44_role_proposer @go_TestCatalogReference @go_TestCatalogResourceProtocolReference
    Scenario: publish_asset_pack as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes publish_asset_pack
      Then the Python outcome is read_or_get

    @surface-467db6735b44_role_curator @go_TestCatalogReference @go_TestCatalogResourceProtocolReference
    Scenario: publish_asset_pack as curator
      Given the canonical curator profile
      When that principal discovers or invokes publish_asset_pack
      Then the Python outcome is read_or_get

    @surface-467db6735b44_role_admin @go_TestCatalogReference @go_TestCatalogResourceProtocolReference
    Scenario: publish_asset_pack as admin
      Given the canonical admin profile
      When that principal discovers or invokes publish_asset_pack
      Then the Python outcome is read_or_get

    @surface-467db6735b44_failure @go_TestCatalogReference @go_TestCatalogResourceProtocolReference
    Scenario: publish_asset_pack preserves Python validation and failure behavior
      Given malformed, missing, out-of-scope, or unavailable inputs
      When the client invokes publish_asset_pack
      Then invalid path/kind/version/UTF-8

  Rule: mcp protocol workflow

    @surface-14b778244dc4 @go_TestMCPContractResourcesAndProtocol
    Scenario: resources/list preserves the Python success contract
      Given the Python request contract JSON-RPC 2.0 MCP request
      When an authorized client invokes resources/list
      Then the response matches source-compatible result/error and ordered discovery

    @surface-14b778244dc4_role_reader @go_TestMCPContractResourcesAndProtocol
    Scenario: resources/list as reader
      Given the canonical reader profile
      When that principal discovers or invokes resources/list
      Then the Python outcome is available_after_successful_initialize_and_authentication

    @surface-14b778244dc4_role_proposer @go_TestMCPContractResourcesAndProtocol
    Scenario: resources/list as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes resources/list
      Then the Python outcome is available_after_successful_initialize_and_authentication

    @surface-14b778244dc4_role_curator @go_TestMCPContractResourcesAndProtocol
    Scenario: resources/list as curator
      Given the canonical curator profile
      When that principal discovers or invokes resources/list
      Then the Python outcome is available_after_successful_initialize_and_authentication

    @surface-14b778244dc4_role_admin @go_TestMCPContractResourcesAndProtocol
    Scenario: resources/list as admin
      Given the canonical admin profile
      When that principal discovers or invokes resources/list
      Then the Python outcome is available_after_successful_initialize_and_authentication

    @surface-14b778244dc4_failure @go_TestMCPContractResourcesAndProtocol
    Scenario: resources/list preserves Python validation and failure behavior
      Given malformed, missing, out-of-scope, or unavailable inputs
      When the client invokes resources/list
      Then -32600/-32601/-32602/-32603 as observed

    @surface-71d2cfba6cf4 @go_TestMCPContractResourcesAndProtocol
    Scenario: resources/templates/list preserves the Python success contract
      Given the Python request contract JSON-RPC 2.0 MCP request
      When an authorized client invokes resources/templates/list
      Then the response matches source-compatible result/error and ordered discovery

    @surface-71d2cfba6cf4_role_reader @go_TestMCPContractResourcesAndProtocol
    Scenario: resources/templates/list as reader
      Given the canonical reader profile
      When that principal discovers or invokes resources/templates/list
      Then the Python outcome is available_after_successful_initialize_and_authentication

    @surface-71d2cfba6cf4_role_proposer @go_TestMCPContractResourcesAndProtocol
    Scenario: resources/templates/list as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes resources/templates/list
      Then the Python outcome is available_after_successful_initialize_and_authentication

    @surface-71d2cfba6cf4_role_curator @go_TestMCPContractResourcesAndProtocol
    Scenario: resources/templates/list as curator
      Given the canonical curator profile
      When that principal discovers or invokes resources/templates/list
      Then the Python outcome is available_after_successful_initialize_and_authentication

    @surface-71d2cfba6cf4_role_admin @go_TestMCPContractResourcesAndProtocol
    Scenario: resources/templates/list as admin
      Given the canonical admin profile
      When that principal discovers or invokes resources/templates/list
      Then the Python outcome is available_after_successful_initialize_and_authentication

    @surface-71d2cfba6cf4_failure @go_TestMCPContractResourcesAndProtocol
    Scenario: resources/templates/list preserves Python validation and failure behavior
      Given malformed, missing, out-of-scope, or unavailable inputs
      When the client invokes resources/templates/list
      Then -32600/-32601/-32602/-32603 as observed

    @surface-cd015dc35a80 @go_TestCatalogResourceProtocolReference
    Scenario: resources/read preserves the Python success contract
      Given the Python request contract JSON-RPC 2.0 MCP request
      When an authorized client invokes resources/read
      Then the response matches source-compatible result/error and ordered discovery

    @surface-cd015dc35a80_role_reader @go_TestCatalogResourceProtocolReference
    Scenario: resources/read as reader
      Given the canonical reader profile
      When that principal discovers or invokes resources/read
      Then the Python outcome is available_after_successful_initialize_and_authentication

    @surface-cd015dc35a80_role_proposer @go_TestCatalogResourceProtocolReference
    Scenario: resources/read as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes resources/read
      Then the Python outcome is available_after_successful_initialize_and_authentication

    @surface-cd015dc35a80_role_curator @go_TestCatalogResourceProtocolReference
    Scenario: resources/read as curator
      Given the canonical curator profile
      When that principal discovers or invokes resources/read
      Then the Python outcome is available_after_successful_initialize_and_authentication

    @surface-cd015dc35a80_role_admin @go_TestCatalogResourceProtocolReference
    Scenario: resources/read as admin
      Given the canonical admin profile
      When that principal discovers or invokes resources/read
      Then the Python outcome is available_after_successful_initialize_and_authentication

    @surface-cd015dc35a80_failure @go_TestCatalogResourceProtocolReference
    Scenario: resources/read preserves Python validation and failure behavior
      Given malformed, missing, out-of-scope, or unavailable inputs
      When the client invokes resources/read
      Then -32600/-32601/-32602/-32603 as observed

    @surface-43c3e58569f9 @go_TestResourceEdges
    Scenario: resources/subscribe preserves the Python success contract
      Given the Python request contract JSON-RPC 2.0 MCP request
      When an authorized client invokes resources/subscribe
      Then the response matches source-compatible result/error and ordered discovery

    @surface-43c3e58569f9_role_reader @go_TestResourceEdges
    Scenario: resources/subscribe as reader
      Given the canonical reader profile
      When that principal discovers or invokes resources/subscribe
      Then the Python outcome is available_after_successful_initialize_and_authentication

    @surface-43c3e58569f9_role_proposer @go_TestResourceEdges
    Scenario: resources/subscribe as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes resources/subscribe
      Then the Python outcome is available_after_successful_initialize_and_authentication

    @surface-43c3e58569f9_role_curator @go_TestResourceEdges
    Scenario: resources/subscribe as curator
      Given the canonical curator profile
      When that principal discovers or invokes resources/subscribe
      Then the Python outcome is available_after_successful_initialize_and_authentication

    @surface-43c3e58569f9_role_admin @go_TestResourceEdges
    Scenario: resources/subscribe as admin
      Given the canonical admin profile
      When that principal discovers or invokes resources/subscribe
      Then the Python outcome is available_after_successful_initialize_and_authentication

    @surface-43c3e58569f9_failure @go_TestResourceEdges
    Scenario: resources/subscribe preserves Python validation and failure behavior
      Given malformed, missing, out-of-scope, or unavailable inputs
      When the client invokes resources/subscribe
      Then -32600/-32601/-32602/-32603 as observed

    @surface-88921c268313 @go_TestResourceEdges
    Scenario: resources/unsubscribe preserves the Python success contract
      Given the Python request contract JSON-RPC 2.0 MCP request
      When an authorized client invokes resources/unsubscribe
      Then the response matches source-compatible result/error and ordered discovery

    @surface-88921c268313_role_reader @go_TestResourceEdges
    Scenario: resources/unsubscribe as reader
      Given the canonical reader profile
      When that principal discovers or invokes resources/unsubscribe
      Then the Python outcome is available_after_successful_initialize_and_authentication

    @surface-88921c268313_role_proposer @go_TestResourceEdges
    Scenario: resources/unsubscribe as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes resources/unsubscribe
      Then the Python outcome is available_after_successful_initialize_and_authentication

    @surface-88921c268313_role_curator @go_TestResourceEdges
    Scenario: resources/unsubscribe as curator
      Given the canonical curator profile
      When that principal discovers or invokes resources/unsubscribe
      Then the Python outcome is available_after_successful_initialize_and_authentication

    @surface-88921c268313_role_admin @go_TestResourceEdges
    Scenario: resources/unsubscribe as admin
      Given the canonical admin profile
      When that principal discovers or invokes resources/unsubscribe
      Then the Python outcome is available_after_successful_initialize_and_authentication

    @surface-88921c268313_failure @go_TestResourceEdges
    Scenario: resources/unsubscribe preserves Python validation and failure behavior
      Given malformed, missing, out-of-scope, or unavailable inputs
      When the client invokes resources/unsubscribe
      Then -32600/-32601/-32602/-32603 as observed

    @surface-6065f5d4ac68 @go_TestMCPContractResourcesAndProtocol
    Scenario: prompts/list preserves the Python success contract
      Given the Python request contract JSON-RPC 2.0 MCP request
      When an authorized client invokes prompts/list
      Then the response matches source-compatible result/error and ordered discovery

    @surface-6065f5d4ac68_role_reader @go_TestMCPContractResourcesAndProtocol
    Scenario: prompts/list as reader
      Given the canonical reader profile
      When that principal discovers or invokes prompts/list
      Then the Python outcome is available_after_successful_initialize_and_authentication

    @surface-6065f5d4ac68_role_proposer @go_TestMCPContractResourcesAndProtocol
    Scenario: prompts/list as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes prompts/list
      Then the Python outcome is available_after_successful_initialize_and_authentication

    @surface-6065f5d4ac68_role_curator @go_TestMCPContractResourcesAndProtocol
    Scenario: prompts/list as curator
      Given the canonical curator profile
      When that principal discovers or invokes prompts/list
      Then the Python outcome is available_after_successful_initialize_and_authentication

    @surface-6065f5d4ac68_role_admin @go_TestMCPContractResourcesAndProtocol
    Scenario: prompts/list as admin
      Given the canonical admin profile
      When that principal discovers or invokes prompts/list
      Then the Python outcome is available_after_successful_initialize_and_authentication

    @surface-6065f5d4ac68_failure @go_TestMCPContractResourcesAndProtocol
    Scenario: prompts/list preserves Python validation and failure behavior
      Given malformed, missing, out-of-scope, or unavailable inputs
      When the client invokes prompts/list
      Then -32600/-32601/-32602/-32603 as observed

    @surface-6c46d6f12550 @go_TestMCPContractResourcesAndProtocol
    Scenario: prompts/get preserves the Python success contract
      Given the Python request contract JSON-RPC 2.0 MCP request
      When an authorized client invokes prompts/get
      Then the response matches source-compatible result/error and ordered discovery

    @surface-6c46d6f12550_role_reader @go_TestMCPContractResourcesAndProtocol
    Scenario: prompts/get as reader
      Given the canonical reader profile
      When that principal discovers or invokes prompts/get
      Then the Python outcome is available_after_successful_initialize_and_authentication

    @surface-6c46d6f12550_role_proposer @go_TestMCPContractResourcesAndProtocol
    Scenario: prompts/get as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes prompts/get
      Then the Python outcome is available_after_successful_initialize_and_authentication

    @surface-6c46d6f12550_role_curator @go_TestMCPContractResourcesAndProtocol
    Scenario: prompts/get as curator
      Given the canonical curator profile
      When that principal discovers or invokes prompts/get
      Then the Python outcome is available_after_successful_initialize_and_authentication

    @surface-6c46d6f12550_role_admin @go_TestMCPContractResourcesAndProtocol
    Scenario: prompts/get as admin
      Given the canonical admin profile
      When that principal discovers or invokes prompts/get
      Then the Python outcome is available_after_successful_initialize_and_authentication

    @surface-6c46d6f12550_failure @go_TestMCPContractResourcesAndProtocol
    Scenario: prompts/get preserves Python validation and failure behavior
      Given malformed, missing, out-of-scope, or unavailable inputs
      When the client invokes prompts/get
      Then -32600/-32601/-32602/-32603 as observed

    @surface-f17b3fa09404 @go_TestCompletionSyncAsyncParity
    Scenario: completion/complete preserves the Python success contract
      Given the Python request contract JSON-RPC 2.0 MCP request
      When an authorized client invokes completion/complete
      Then the response matches source-compatible result/error and ordered discovery

    @surface-f17b3fa09404_role_reader @go_TestCompletionSyncAsyncParity
    Scenario: completion/complete as reader
      Given the canonical reader profile
      When that principal discovers or invokes completion/complete
      Then the Python outcome is available_after_successful_initialize_and_authentication

    @surface-f17b3fa09404_role_proposer @go_TestCompletionSyncAsyncParity
    Scenario: completion/complete as proposer
      Given the canonical proposer profile
      When that principal discovers or invokes completion/complete
      Then the Python outcome is available_after_successful_initialize_and_authentication

    @surface-f17b3fa09404_role_curator @go_TestCompletionSyncAsyncParity
    Scenario: completion/complete as curator
      Given the canonical curator profile
      When that principal discovers or invokes completion/complete
      Then the Python outcome is available_after_successful_initialize_and_authentication

    @surface-f17b3fa09404_role_admin @go_TestCompletionSyncAsyncParity
    Scenario: completion/complete as admin
      Given the canonical admin profile
      When that principal discovers or invokes completion/complete
      Then the Python outcome is available_after_successful_initialize_and_authentication

    @surface-f17b3fa09404_failure @go_TestCompletionSyncAsyncParity
    Scenario: completion/complete preserves Python validation and failure behavior
      Given malformed, missing, out-of-scope, or unavailable inputs
      When the client invokes completion/complete
      Then -32600/-32601/-32602/-32603 as observed
