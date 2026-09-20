Feature: mcp/discovery notifications and progress

  These scenarios describe observable behavior independently of its implementation.
  Stable row tags link each scenario to versioned evidence and executable validation data.

  Rule: Discovery pagination

    @umcp-9652351e4dd1
    Scenario: Sync dynamic discovery registration and pagination
      Given a configured MCP client, server and transport state
      When the client performs: sync dynamic discovery registration and pagination
      Then the bounded result and continuation state match the specified contract

    @umcp-6d898f2a530e
    Scenario: Sync invalid cursor and default no cursor compatibility
      Given a configured MCP client, server and transport state
      When the client performs: sync invalid cursor and default no cursor compatibility
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @umcp-c6053ea3387d
    Scenario: Sync cursor is principal safe
      Given a configured MCP client, server and transport state
      When the client performs: sync cursor is principal safe
      Then the bounded result and continuation state match the specified contract

    @umcp-3b167e4ca97c
    Scenario: Async dynamic discovery registration and pagination
      Given a configured MCP client, server and transport state
      When the client performs: async dynamic discovery registration and pagination
      Then the bounded result and continuation state match the specified contract

    @umcp-52a6cac0624a
    Scenario: List changed notifications target existing transports
      Given a configured MCP client, server and transport state
      When the client performs: list changed notifications target existing transports
      Then the observable response, ordering, warnings and resulting state match the specified contract

  Rule: Notifications

    @umcp-3e632c214980
    Scenario: Sync list changed always emits
      Given a configured MCP client, server and transport state
      When the client performs: sync list changed always emits
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-7906537937b8
    Scenario: Sync updated skipped without subscription
      Given a configured MCP client, server and transport state
      When the client performs: sync updated skipped without subscription
      Then the response and durable state transition match the specified lifecycle

    @umcp-feb4fb28cdc3
    Scenario: Sync updated emits only for subscribed uri
      Given a configured MCP client, server and transport state
      When the client performs: sync updated emits only for subscribed uri
      Then the response and durable state transition match the specified lifecycle

    @umcp-5b429a0381e9
    Scenario: Sync unsubscribe stops notifications
      Given a configured MCP client, server and transport state
      When the client performs: sync unsubscribe stops notifications
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-cfab471aecff
    Scenario: Sync subscribe via protocol then notify
      Given a configured MCP client, server and transport state
      When the client performs: sync subscribe via protocol then notify
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-efcc8ceaa07e
    Scenario: Sync sse updated targets only subscribed session
      Given a configured MCP client, server and transport state
      When the client performs: sync sse updated targets only subscribed session
      Then the response and durable state transition match the specified lifecycle

    @umcp-96cfa7100f4c
    Scenario: Sync sse list changed broadcasts to all sessions
      Given a configured MCP client, server and transport state
      When the client performs: sync sse list changed broadcasts to all sessions
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-97efaf1cb633
    Scenario: Async list changed always emits
      Given a configured MCP client, server and transport state
      When the client performs: async list changed always emits
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-3e657e0a4e5e
    Scenario: Async updated gated by subscription
      Given a configured MCP client, server and transport state
      When the client performs: async updated gated by subscription
      Then the response and durable state transition match the specified lifecycle

    @umcp-deac04a69846
    Scenario: Async sse updated targets only subscribed session
      Given a configured MCP client, server and transport state
      When the client performs: async sse updated targets only subscribed session
      Then the response and durable state transition match the specified lifecycle

    @umcp-8f2b816160e6
    Scenario: Async sse list changed broadcasts to all sessions
      Given a configured MCP client, server and transport state
      When the client performs: async sse list changed broadcasts to all sessions
      Then the observable response, ordering, warnings and resulting state match the specified contract

  Rule: Progress cancellation

    @umcp-d71811f1f416
    Scenario: Sync progress exact payload and absent token noop
      Given a configured MCP client, server and transport state
      When the client performs: sync progress exact payload and absent token noop
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-f529c0092aa7
    Scenario: Async progress exact payload for integer token
      Given a configured MCP client, server and transport state
      When the client performs: async progress exact payload for integer token
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-7e523580cef8
    Scenario: Invalid progress token and invalid progress are rejected
      Given a configured MCP client, server and transport state
      When the client performs: invalid progress token and invalid progress are rejected
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @umcp-21b26bec488b
    Scenario: Sync cooperative cancellation and cleanup
      Given a configured MCP client, server and transport state
      When the client performs: sync cooperative cancellation and cleanup
      Then cancellation or timeout preserves the specified completion and reconciliation state

    @umcp-92233c31da1e
    Scenario: Sync cancellation isolated between concurrent requests
      Given a configured MCP client, server and transport state
      When the client performs: sync cancellation isolated between concurrent requests
      Then the result and persisted state remain deterministic under concurrent execution

    @umcp-6af42ebab4ee
    Scenario: Async active cancellation and cleanup
      Given a configured MCP client, server and transport state
      When the client performs: async active cancellation and cleanup
      Then cancellation or timeout preserves the specified completion and reconciliation state

    @umcp-cdcdaa3e924e
    Scenario: Async cancellation isolated between concurrent requests
      Given a configured MCP client, server and transport state
      When the client performs: async cancellation isolated between concurrent requests
      Then the result and persisted state remain deterministic under concurrent execution

    @umcp-c850acb753f4
    Scenario: Sync progress uses sse transport when present
      Given a configured MCP client, server and transport state
      When the client performs: sync progress uses sse transport when present
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-c61cc4016ca8
    Scenario: Async progress uses sse transport when present
      Given a configured MCP client, server and transport state
      When the client performs: async progress uses sse transport when present
      Then the observable response, ordering, warnings and resulting state match the specified contract

  Rule: Tool outputs and runtime notifications

    @umcp-871d3f58ee43
    Scenario: Sync register unregister and notify stdio
      Given a configured MCP client, server and transport state
      When the client performs: sync register unregister and notify stdio
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-347478781a5b
    Scenario: Sync register unregister and notify sse
      Given a configured MCP client, server and transport state
      When the client performs: sync register unregister and notify sse
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-db1301a590a1
    Scenario: Async register unregister and notify stdio
      Given a configured MCP client, server and transport state
      When the client performs: async register unregister and notify stdio
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-2dec70e66b20
    Scenario: Async register unregister and notify sse
      Given a configured MCP client, server and transport state
      When the client performs: async register unregister and notify sse
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-bf2124757038
    Scenario: Tools list advertises inferred and explicit output schemas
      Given a configured MCP client, server and transport state
      When the client performs: tools list advertises inferred and explicit output schemas
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-48e8eb3b4ad8
    Scenario: Sync mapping return adds structured content
      Given a configured MCP client, server and transport state
      When the client performs: sync mapping return adds structured content
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-ed1fea538be7
    Scenario: Sync typed return adds structured content
      Given a configured MCP client, server and transport state
      When the client performs: sync typed return adds structured content
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-faae02c4fda0
    Scenario: Async mapping return adds structured content
      Given a configured MCP client, server and transport state
      When the client performs: async mapping return adds structured content
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-45da0c68685a
    Scenario: Async typed return adds structured content
      Given a configured MCP client, server and transport state
      When the client performs: async typed return adds structured content
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-af1d7f1db12e
    Scenario: Sync static malformed output is local detailed
      Given a configured MCP client, server and transport state
      When the client performs: sync static malformed output is local detailed
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-da87e528ef40
    Scenario: Sync dynamic malformed output is remote safe
      Given a configured MCP client, server and transport state
      When the client performs: sync dynamic malformed output is remote safe
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-aa42cdcf60d1
    Scenario: Async static malformed output is local detailed
      Given a configured MCP client, server and transport state
      When the client performs: async static malformed output is local detailed
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-55edd23a3ac5
    Scenario: Async dynamic malformed output is remote safe
      Given a configured MCP client, server and transport state
      When the client performs: async dynamic malformed output is remote safe
      Then the observable response, ordering, warnings and resulting state match the specified contract
