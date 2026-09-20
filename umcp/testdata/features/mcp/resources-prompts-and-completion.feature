Feature: mcp/resources prompts and completion

  These scenarios describe observable behavior independently of its implementation.
  Stable row tags link each scenario to versioned evidence and executable validation data.

  Rule: Async prompts

    @umcp-272bab2fca9a
    Scenario: Async prompts list
      Given a configured MCP client, server and transport state
      When the client performs: async prompts list
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-744264bc0329
    Scenario: Async prompt get sync return
      Given a configured MCP client, server and transport state
      When the client performs: async prompt get sync return
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-d5051bec5c94
    Scenario: Async prompt get async return
      Given a configured MCP client, server and transport state
      When the client performs: async prompt get async return
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-9c049739c356
    Scenario: Async prompts missing required argument returns invalid params
      Given a configured MCP client, server and transport state
      When the client performs: async prompts missing required argument returns invalid params
      Then the request is rejected at the specified boundary and prohibited state is unchanged

  Rule: Completion logging

    @umcp-21d34e1828c0
    Scenario: Sync initialize only advertises logging without completion support
      Given a configured MCP client, server and transport state
      When the client performs: sync initialize only advertises logging without completion support
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-3c26188c4bb3
    Scenario: Sync initialize advertises completions exactly when available
      Given a configured MCP client, server and transport state
      When the client performs: sync initialize advertises completions exactly when available
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-782a17b97200
    Scenario: Async initialize advertises completions exactly when available
      Given a configured MCP client, server and transport state
      When the client performs: async initialize advertises completions exactly when available
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-28db721b0126
    Scenario: Sync completion supports prompt and resource refs literal and enum
      Given a configured MCP client, server and transport state
      When the client performs: sync completion supports prompt and resource refs literal and enum
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-d0979968f941
    Scenario: Completion supports registered sync and async providers and context (False)
      Given a configured MCP client, server and transport state
      When the client performs: completion supports registered sync and async providers and context (false)
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-7facb781e10d
    Scenario: Completion supports registered sync and async providers and context (True)
      Given a configured MCP client, server and transport state
      When the client performs: completion supports registered sync and async providers and context (true)
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-9c39b3b32a0d
    Scenario: Completion supports schema enums for registered prompts (False)
      Given a configured MCP client, server and transport state
      When the client performs: completion supports schema enums for registered prompts (false)
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-f6562fd6dd04
    Scenario: Completion supports schema enums for registered prompts (True)
      Given a configured MCP client, server and transport state
      When the client performs: completion supports schema enums for registered prompts (true)
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-66cbfffe7d5c
    Scenario: Completion limits to max 100 and sets total and has more (False)
      Given a configured MCP client, server and transport state
      When the client performs: completion limits to max 100 and sets total and has more (false)
      Then the bounded result and continuation state match the specified contract

    @umcp-5f965d32bee1
    Scenario: Completion limits to max 100 and sets total and has more (True)
      Given a configured MCP client, server and transport state
      When the client performs: completion limits to max 100 and sets total and has more (true)
      Then the bounded result and continuation state match the specified contract

    @umcp-02cb98105cc2
    Scenario: Completion invalid refs args and outputs are remote safe (False)
      Given a configured MCP client, server and transport state
      When the client performs: completion invalid refs args and outputs are remote safe (false)
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @umcp-cd96b78d0b31
    Scenario: Completion invalid refs args and outputs are remote safe (True)
      Given a configured MCP client, server and transport state
      When the client performs: completion invalid refs args and outputs are remote safe (true)
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @umcp-6fc09c396965
    Scenario: Logging set level accepts standard levels and threshold ordering (False)
      Given a configured MCP client, server and transport state
      When the client performs: logging set level accepts standard levels and threshold ordering (false)
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-50b508cb90a3
    Scenario: Logging set level accepts standard levels and threshold ordering (True)
      Given a configured MCP client, server and transport state
      When the client performs: logging set level accepts standard levels and threshold ordering (true)
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-b13d9a679db3
    Scenario: Sync logging stdio payload redacts sensitive keys recursively
      Given a configured MCP client, server and transport state
      When the client performs: sync logging stdio payload redacts sensitive keys recursively
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-f1d7dd57275b
    Scenario: Sync logging stdio payload preserves data when sanitize false
      Given a configured MCP client, server and transport state
      When the client performs: sync logging stdio payload preserves data when sanitize false
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-94d21ab49f39
    Scenario: Sync logging sse payload exact and logger optional
      Given a configured MCP client, server and transport state
      When the client performs: sync logging sse payload exact and logger optional
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-5da7a47f7bde
    Scenario: Async logging stdio and sse payloads
      Given a configured MCP client, server and transport state
      When the client performs: async logging stdio and sse payloads
      Then the observable response, ordering, warnings and resulting state match the specified contract

  Rule: Prompts

    @umcp-9ea07661246b
    Scenario: Prompts list
      Given a configured MCP client, server and transport state
      When the client performs: prompts list
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-a88fbd8ee45b
    Scenario: Prompts get missing required argument returns invalid params
      Given a configured MCP client, server and transport state
      When the client performs: prompts get missing required argument returns invalid params
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @umcp-fe003cc0bcd5
    Scenario: Prompts get with arguments
      Given a configured MCP client, server and transport state
      When the client performs: prompts get with arguments
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-46974c62c915
    Scenario: Prompts get list messages
      Given a configured MCP client, server and transport state
      When the client performs: prompts get list messages
      Then the observable response, ordering, warnings and resulting state match the specified contract

  Rule: Prompts extra

    @umcp-05f03b0975bf
    Scenario: Prompts list returns full metadata
      Given a configured MCP client, server and transport state
      When the client performs: prompts list returns full metadata
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-7722ddd67393
    Scenario: Prompts categories are parsed from docstring
      Given a configured MCP client, server and transport state
      When the client performs: prompts categories are parsed from docstring
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-d8e3e675e9d7
    Scenario: Prompts arguments track required vs optional
      Given a configured MCP client, server and transport state
      When the client performs: prompts arguments track required vs optional
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-c5b6413129fa
    Scenario: Prompts get string return wraps as user message
      Given a configured MCP client, server and transport state
      When the client performs: prompts get string return wraps as user message
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-37a56c76dd35
    Scenario: Prompts get list return passes messages through
      Given a configured MCP client, server and transport state
      When the client performs: prompts get list return passes messages through
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-ffc79ac4ea9b
    Scenario: Prompts get dict return preserves top level fields
      Given a configured MCP client, server and transport state
      When the client performs: prompts get dict return preserves top level fields
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-1a2a13a6425b
    Scenario: Prompts get missing required argument returns invalid params
      Given a configured MCP client, server and transport state
      When the client performs: prompts get missing required argument returns invalid params
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @umcp-df6a0fd59368
    Scenario: Prompts get unknown name returns error
      Given a configured MCP client, server and transport state
      When the client performs: prompts get unknown name returns error
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @umcp-3b6eb9ca3d8e
    Scenario: Prompts get unknown argument returns invalid params
      Given a configured MCP client, server and transport state
      When the client performs: prompts get unknown argument returns invalid params
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @umcp-8f3be6bf1230
    Scenario: Initialize declares prompts capability
      Given a configured MCP client, server and transport state
      When the client performs: initialize declares prompts capability
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-924b5bc25605
    Scenario: Async prompts list includes categories
      Given a configured MCP client, server and transport state
      When the client performs: async prompts list includes categories
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-fdcd16ab87fc
    Scenario: Async prompt with async return
      Given a configured MCP client, server and transport state
      When the client performs: async prompt with async return
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-0bb0d01d5072
    Scenario: Async base supports sync prompt methods
      Given a configured MCP client, server and transport state
      When the client performs: async base supports sync prompt methods
      Then the observable response, ordering, warnings and resulting state match the specified contract

  Rule: Resources

    @umcp-7fd3b8a764eb
    Scenario: Sync resources list includes static
      Given a configured MCP client, server and transport state
      When the client performs: sync resources list includes static
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-b56073bf6692
    Scenario: Sync templates list includes template
      Given a configured MCP client, server and transport state
      When the client performs: sync templates list includes template
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-f545b6c5359f
    Scenario: Sync read text resource
      Given a configured MCP client, server and transport state
      When the client performs: sync read text resource
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-e22566b4a914
    Scenario: Sync read binary resource is base64
      Given a configured MCP client, server and transport state
      When the client performs: sync read binary resource is base64
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-cd713efd951b
    Scenario: Sync read template resource binds param
      Given a configured MCP client, server and transport state
      When the client performs: sync read template resource binds param
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-99547675c3db
    Scenario: Sync read unknown uri returns minus 32002
      Given a configured MCP client, server and transport state
      When the client performs: sync read unknown uri returns minus 32002
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-8c276f3df296
    Scenario: Sync subscribe unsubscribe roundtrip
      Given a configured MCP client, server and transport state
      When the client performs: sync subscribe unsubscribe roundtrip
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-4d884f0d86d9
    Scenario: Sync dynamic register resource
      Given a configured MCP client, server and transport state
      When the client performs: sync dynamic register resource
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-7e2688d21dd1
    Scenario: Sync initialize declares resources capability
      Given a configured MCP client, server and transport state
      When the client performs: sync initialize declares resources capability
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-f378d046baa3
    Scenario: Async resources list
      Given a configured MCP client, server and transport state
      When the client performs: async resources list
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-5f1475655df8
    Scenario: Async read async text resource
      Given a configured MCP client, server and transport state
      When the client performs: async read async text resource
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-08b93ea1c7ed
    Scenario: Async read sync method in async server
      Given a configured MCP client, server and transport state
      When the client performs: async read sync method in async server
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-e69e95994aff
    Scenario: Async read template resource
      Given a configured MCP client, server and transport state
      When the client performs: async read template resource
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-1a586e5d548d
    Scenario: Async unknown uri returns minus 32002
      Given a configured MCP client, server and transport state
      When the client performs: async unknown uri returns minus 32002
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-0dcb44633e60
    Scenario: Async initialize declares resources capability
      Given a configured MCP client, server and transport state
      When the client performs: async initialize declares resources capability
      Then the observable response, ordering, warnings and resulting state match the specified contract

  Rule: Resources extra

    @umcp-707ba85853c6
    Scenario: Multi param template listed with both placeholders
      Given a configured MCP client, server and transport state
      When the client performs: multi param template listed with both placeholders
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-396c27b69cff
    Scenario: Multi param template read binds both groups
      Given a configured MCP client, server and transport state
      When the client performs: multi param template read binds both groups
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-fee3fe0db209
    Scenario: Static resource raise is caught as minus 32603
      Given a configured MCP client, server and transport state
      When the client performs: static resource raise is caught as minus 32603
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-dd09bd78fa9d
    Scenario: Template resource raise is caught as minus 32603
      Given a configured MCP client, server and transport state
      When the client performs: template resource raise is caught as minus 32603
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-968952d03327
    Scenario: List of dicts return yields multiple content entries
      Given a configured MCP client, server and transport state
      When the client performs: list of dicts return yields multiple content entries
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-761738ff1deb
    Scenario: Register resource template then read
      Given a configured MCP client, server and transport state
      When the client performs: register resource template then read
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-72ca34191bfd
    Scenario: Custom uri schemes round trip
      Given a configured MCP client, server and transport state
      When the client performs: custom uri schemes round trip
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-a330c44eadea
    Scenario: Resource annotations are exposed on list
      Given a configured MCP client, server and transport state
      When the client performs: resource annotations are exposed on list
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-0ea7aaa5a97b
    Scenario: Override attribute replaces default uri and name
      Given a configured MCP client, server and transport state
      When the client performs: override attribute replaces default uri and name
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-90a028e8bd78
    Scenario: Resources list paginates when requested
      Given a configured MCP client, server and transport state
      When the client performs: resources list paginates when requested
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-9d31157d9146
    Scenario: Resources templates list without cursor preserves compatibility
      Given a configured MCP client, server and transport state
      When the client performs: resources templates list without cursor preserves compatibility
      Then the bounded result and continuation state match the specified contract

    @umcp-cde9a436c57d
    Scenario: Async register resource then read
      Given a configured MCP client, server and transport state
      When the client performs: async register resource then read
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-a2a9f747368d
    Scenario: Async resource that raises is caught as minus 32603
      Given a configured MCP client, server and transport state
      When the client performs: async resource that raises is caught as minus 32603
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-2662a7fd1b88
    Scenario: Subscribe to unknown uri is still accepted
      Given a configured MCP client, server and transport state
      When the client performs: subscribe to unknown uri is still accepted
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-d4c8bb66214c
    Scenario: Unsubscribe unknown uri is a noop
      Given a configured MCP client, server and transport state
      When the client performs: unsubscribe unknown uri is a noop
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-b1213eb0676a
    Scenario: Subscribe without uri returns invalid params
      Given a configured MCP client, server and transport state
      When the client performs: subscribe without uri returns invalid params
      Then the request is rejected at the specified boundary and prohibited state is unchanged
