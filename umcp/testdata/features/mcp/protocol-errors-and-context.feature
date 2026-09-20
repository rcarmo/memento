Feature: mcp/protocol errors and context

  These scenarios describe observable behavior independently of its implementation.
  Stable row tags link each scenario to versioned evidence and executable validation data.

  Rule: Protocol errors

    @umcp-dc6daba2cf4e
    Scenario: Sync invalid json returns parse error
      Given a configured MCP client, server and transport state
      When the client performs: sync invalid json returns parse error
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @umcp-8456f692a67f
    Scenario: Sync non object top level json is invalid request
      Given a configured MCP client, server and transport state
      When the client performs: sync non object top level json is invalid request
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @umcp-5ff0b12e40d7
    Scenario: Sync wrong jsonrpc version rejected
      Given a configured MCP client, server and transport state
      When the client performs: sync wrong jsonrpc version rejected
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @umcp-da145c530a0d
    Scenario: Sync invalid ids and malformed responses are rejected
      Given a configured MCP client, server and transport state
      When the client performs: sync invalid ids and malformed responses are rejected
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @umcp-56f18ebea82c
    Scenario: Sync missing or non string method is invalid request
      Given a configured MCP client, server and transport state
      When the client performs: sync missing or non string method is invalid request
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @umcp-4301bc7f2c70
    Scenario: Sync client response returns none
      Given a configured MCP client, server and transport state
      When the client performs: sync client response returns none
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-67c6ac8366ed
    Scenario: Sync unknown method returns minus 32601
      Given a configured MCP client, server and transport state
      When the client performs: sync unknown method returns minus 32601
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-4180b1d97b21
    Scenario: Sync non object params returns invalid params
      Given a configured MCP client, server and transport state
      When the client performs: sync non object params returns invalid params
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @umcp-3a946d70f06e
    Scenario: Sync tools call missing name
      Given a configured MCP client, server and transport state
      When the client performs: sync tools call missing name
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-7f36e90e5429
    Scenario: Sync tools call unknown tool
      Given a configured MCP client, server and transport state
      When the client performs: sync tools call unknown tool
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-abb8166665a1
    Scenario: Sync tools call with unknown argument is rejected
      Given a configured MCP client, server and transport state
      When the client performs: sync tools call with unknown argument is rejected
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @umcp-854be0d8a9b2
    Scenario: Sync tool that raises does not crash server
      Given a configured MCP client, server and transport state
      When the client performs: sync tool that raises does not crash server
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-803e77a40ede
    Scenario: Sync prompts get unknown returns error
      Given a configured MCP client, server and transport state
      When the client performs: sync prompts get unknown returns error
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @umcp-7358f042ceac
    Scenario: Sync notifications initialized returns none
      Given a configured MCP client, server and transport state
      When the client performs: sync notifications initialized returns none
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-45265eb98d4d
    Scenario: Async invalid json returns parse error
      Given a configured MCP client, server and transport state
      When the client performs: async invalid json returns parse error
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @umcp-7770742a87fd
    Scenario: Async non object top level json is invalid request
      Given a configured MCP client, server and transport state
      When the client performs: async non object top level json is invalid request
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @umcp-96014de5bb34
    Scenario: Async invalid ids and malformed responses are rejected
      Given a configured MCP client, server and transport state
      When the client performs: async invalid ids and malformed responses are rejected
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @umcp-13f5555d9c54
    Scenario: Async missing or non string method is invalid request
      Given a configured MCP client, server and transport state
      When the client performs: async missing or non string method is invalid request
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @umcp-7ef12e827d2a
    Scenario: Async client response returns none
      Given a configured MCP client, server and transport state
      When the client performs: async client response returns none
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-100762c25600
    Scenario: Async unknown method returns minus 32601
      Given a configured MCP client, server and transport state
      When the client performs: async unknown method returns minus 32601
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-a94a9ff2e05a
    Scenario: Async non object params returns invalid params
      Given a configured MCP client, server and transport state
      When the client performs: async non object params returns invalid params
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @umcp-5809fa868dba
    Scenario: Async tools call unknown tool
      Given a configured MCP client, server and transport state
      When the client performs: async tools call unknown tool
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-dc27bd9c5bd2
    Scenario: Async tool that raises does not crash server
      Given a configured MCP client, server and transport state
      When the client performs: async tool that raises does not crash server
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-90dfe4758253
    Scenario: Async notifications initialized returns none
      Given a configured MCP client, server and transport state
      When the client performs: async notifications initialized returns none
      Then the observable response, ordering, warnings and resulting state match the specified contract

  Rule: Shared negotiation context

    @umcp-d9e0176d7ae9
    Scenario: Exact or fallback prefers supported version
      Given a configured MCP client, server and transport state
      When the client performs: exact or fallback prefers supported version
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-36fc32ec4acb
    Scenario: Request context roundtrip
      Given a configured MCP client, server and transport state
      When the client performs: request context roundtrip
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-c76afbbb5c0f
    Scenario: Sync initialize negotiates supported and falls back
      Given a configured MCP client, server and transport state
      When the client performs: sync initialize negotiates supported and falls back
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-5fac35c71574
    Scenario: Async initialize negotiation matches sync
      Given a configured MCP client, server and transport state
      When the client performs: async initialize negotiation matches sync
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-b0e2d9e99a61
    Scenario: Sync initialize capabilities are exact
      Given a configured MCP client, server and transport state
      When the client performs: sync initialize capabilities are exact
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-928e5a585175
    Scenario: Async initialize capabilities match sync exactly
      Given a configured MCP client, server and transport state
      When the client performs: async initialize capabilities match sync exactly
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-62785dca6999
    Scenario: Request context headers are defensively immutable
      Given a configured MCP client, server and transport state
      When the client performs: request context headers are defensively immutable
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-615dac5bf8fd
    Scenario: Principal metadata is defensively immutable
      Given a configured MCP client, server and transport state
      When the client performs: principal metadata is defensively immutable
      Then the observable response, ordering, warnings and resulting state match the specified contract
