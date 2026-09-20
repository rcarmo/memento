Feature: mcp/transports and integrated servers

  These scenarios describe observable behavior independently of its implementation.
  Stable row tags link each scenario to versioned evidence and executable validation data.

  Rule: Async servers

    @umcp-b13443a5d78e
    Scenario: Introspection
      Given a configured MCP client, server and transport state
      When the client performs: introspection
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-947957ba3a3b
    Scenario: Async performance
      Given a configured MCP client, server and transport state
      When the client performs: async performance
      Then the observable response, ordering, warnings and resulting state match the specified contract

  Rule: Movieserver

    @umcp-1014a9d135a6
    Scenario: Basic functionality
      Given a configured MCP client, server and transport state
      When the client performs: basic functionality
      Then the observable response, ordering, warnings and resulting state match the specified contract

  Rule: Transports

    @umcp-4ea8e852b72a
    Scenario: Sync stdio streaming round trip
      Given a configured MCP client, server and transport state
      When the client performs: sync stdio streaming round trip
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-d44ac184a19a
    Scenario: Sync stdio resources round trip
      Given a configured MCP client, server and transport state
      When the client performs: sync stdio resources round trip
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-7a8871616f0b
    Scenario: Sync tcp transport round trip (calculator server)
      Given a configured MCP client, server and transport state
      When the client performs: sync tcp transport round trip (calculator server)
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-3672760045e8
    Scenario: Sync streamable http round trip and error paths
      Given a configured MCP client, server and transport state
      When the client performs: sync streamable http round trip and error paths
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @umcp-6b5670e5a8e3
    Scenario: Sync streamable http auxiliary routes and mcp auth boundary
      Given a configured MCP client, server and transport state
      When the client performs: sync streamable http auxiliary routes and mcp auth boundary
      Then the bounded result and continuation state match the specified contract

    @umcp-e38e82651c1f
    Scenario: Sync streamable http auth 401 and 403
      Given a configured MCP client, server and transport state
      When the client performs: sync streamable http auth 401 and 403
      Then the authorization decision and visible result match the specified principal scope

    @umcp-3518b90916a3
    Scenario: Sync streamable http legacy auth alias cors errors and length guards
      Given a configured MCP client, server and transport state
      When the client performs: sync streamable http legacy auth alias cors errors and length guards
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @umcp-f8a0c4a20b43
    Scenario: Sync streamable http query host duplicate headers and options path
      Given a configured MCP client, server and transport state
      When the client performs: sync streamable http query host duplicate headers and options path
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-e5ee4bb2149b
    Scenario: Sync streamable http rejects async auth hooks cleanly
      Given a configured MCP client, server and transport state
      When the client performs: sync streamable http rejects async auth hooks cleanly
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @umcp-3c4a9bdb305b
    Scenario: Sync sse security origin and body guards
      Given a configured MCP client, server and transport state
      When the client performs: sync sse security origin and body guards
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @umcp-b95032096897
    Scenario: Sync sse transport handshake and request
      Given a configured MCP client, server and transport state
      When the client performs: sync sse transport handshake and request
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-310b17c3ad15
    Scenario: Sync sse session cleanup race returns 404
      Given a configured MCP client, server and transport state
      When the client performs: sync sse session cleanup race returns 404
      Then the result and persisted state remain deterministic under concurrent execution

    @umcp-1ee3757945b5
    Scenario: Help flag does not crash
      Given a configured MCP client, server and transport state
      When the client performs: help flag does not crash
      Then the observable response, ordering, warnings and resulting state match the specified contract
