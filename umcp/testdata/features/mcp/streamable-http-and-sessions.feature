Feature: mcp/streamable http and sessions

  These scenarios describe observable behavior independently of its implementation.
  Stable row tags link each scenario to versioned evidence and executable validation data.

  Rule: Streamable http regressions

    @umcp-b19b2ba5d202
    Scenario: Sync non dict json rpcs return invalid request
      Given a configured MCP client, server and transport state
      When the client performs: sync non dict json rpcs return invalid request
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @umcp-73770948dc48
    Scenario: Async non dict json rpcs return invalid request
      Given a configured MCP client, server and transport state
      When the client performs: async non dict json rpcs return invalid request
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @umcp-6fa69aa4b7db
    Scenario: Context isolated across threads and async tasks
      Given a configured MCP client, server and transport state
      When the client performs: context isolated across threads and async tasks
      Then the result and persisted state remain deterministic under concurrent execution

    @umcp-f8f60ecfc44e
    Scenario: Stdio and file modes expose transport context
      Given a configured MCP client, server and transport state
      When the client performs: stdio and file modes expose transport context
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-354cba428ef7
    Scenario: Async context propagates into sync tool
      Given a configured MCP client, server and transport state
      When the client performs: async context propagates into sync tool
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-643814a3336e
    Scenario: Async http 202 for notification and response object
      Given a configured MCP client, server and transport state
      When the client performs: async http 202 for notification and response object
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-cf94f8a3b054
    Scenario: Async http tool sees authenticated context
      Given a configured MCP client, server and transport state
      When the client performs: async http tool sees authenticated context
      Then the authorization decision and visible result match the specified principal scope

    @umcp-225ab47c2df9
    Scenario: Origin validation is exact and remote safe
      Given a configured MCP client, server and transport state
      When the client performs: origin validation is exact and remote safe
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-a87a4c12bfb8
    Scenario: Accept negotiation matches actual response type
      Given a configured MCP client, server and transport state
      When the client performs: accept negotiation matches actual response type
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-5187d68210ec
    Scenario: Cli rejects unknown conflicting and incomplete transports
      Given a configured MCP client, server and transport state
      When the client performs: cli rejects unknown conflicting and incomplete transports
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @umcp-312e9b4f43c0
    Scenario: Async cors headers are returned on preflight and post
      Given a configured MCP client, server and transport state
      When the client performs: async cors headers are returned on preflight and post
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-ecd6f451c9ed
    Scenario: Async auxiliary http routes are bounded and mcp is unchanged
      Given a configured MCP client, server and transport state
      When the client performs: async auxiliary http routes are bounded and mcp is unchanged
      Then the bounded result and continuation state match the specified contract

    @umcp-3412055144c3
    Scenario: Async http rejects bad content length and oversize
      Given a configured MCP client, server and transport state
      When the client performs: async http rejects bad content length and oversize
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @umcp-a79845283665
    Scenario: Sync auth hook aliases and new hooks are bidirectional
      Given a configured MCP client, server and transport state
      When the client performs: sync auth hook aliases and new hooks are bidirectional
      Then the authorization decision and visible result match the specified principal scope

    @umcp-1e54ba12d421
    Scenario: Async auth hook aliases and new hooks are bidirectional
      Given a configured MCP client, server and transport state
      When the client performs: async auth hook aliases and new hooks are bidirectional
      Then the authorization decision and visible result match the specified principal scope

    @umcp-c5f5bf16c9e0
    Scenario: Async http rejects missing version bad accept and unauthorized
      Given a configured MCP client, server and transport state
      When the client performs: async http rejects missing version bad accept and unauthorized
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @umcp-9759303d185d
    Scenario: Async http preflight requires valid origin and authorization can forbid
      Given a configured MCP client, server and transport state
      When the client performs: async http preflight requires valid origin and authorization can forbid
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @umcp-1fad2ac487e0
    Scenario: Async http allowed origin gets cors on terminal responses (GET /mcp HTTP/1.1\nOrigin: http://allowed\nContent Length: 0  401 Unauthorized)
      Given a configured MCP client, server and transport state
      When the client performs: async http allowed origin gets cors on terminal responses (get /mcp http/1.1\norigin: http://allowed\ncontent length: 0  401 unauthorized)
      Then the authorization decision and visible result match the specified principal scope

    @umcp-73527eb67563
    Scenario: Async http allowed origin gets cors on terminal responses (POST /mcp HTTP/1.1\nOrigin: http://allowed\nContent Type: text/plain\nAccept: application/json\nAuthorization: Bearer ok\nContent Length: 0  415 Unsupported Media Type)
      Given a configured MCP client, server and transport state
      When the client performs: async http allowed origin gets cors on terminal responses (post /mcp http/1.1\norigin: http://allowed\ncontent type: text/plain\naccept: application/json\nauthorization: bearer ok\ncontent length: 0  415 unsupported media type)
      Then the authorization decision and visible result match the specified principal scope

    @umcp-bd485b8eb58d
    Scenario: Async http allowed origin gets cors on terminal responses (POST /mcp HTTP/1.1\nOrigin: http://allowed\nContent Type: application/json\nAccept: text/plain\nAuthorization: Bearer ok\nContent Length: 0  406 Not Acceptable)
      Given a configured MCP client, server and transport state
      When the client performs: async http allowed origin gets cors on terminal responses (post /mcp http/1.1\norigin: http://allowed\ncontent type: application/json\naccept: text/plain\nauthorization: bearer ok\ncontent length: 0  406 not acceptable)
      Then the authorization decision and visible result match the specified principal scope

    @umcp-94994abbfb6b
    Scenario: Async http allowed origin gets cors on terminal responses (POST /mcp HTTP/1.1\nOrigin: http://allowed\nContent Type: application/json\nAccept: application/json\nMCP Protocol Version: 2025 03 26\nContent Length: 0  401 Unauthorized)
      Given a configured MCP client, server and transport state
      When the client performs: async http allowed origin gets cors on terminal responses (post /mcp http/1.1\norigin: http://allowed\ncontent type: application/json\naccept: application/json\nmcp protocol version: 2025 03 26\ncontent length: 0  401 unauthorized)
      Then the authorization decision and visible result match the specified principal scope

    @umcp-6cdc615c2bbd
    Scenario: Async http allowed origin gets cors on terminal responses (POST /mcp HTTP/1.1\nOrigin: http://allowed\nContent Type: application/json\nAccept: application/json\nAuthorization: Bearer ok\nMCP Protocol Version: 2025 03 26\nContent Length: 0  200 OK)
      Given a configured MCP client, server and transport state
      When the client performs: async http allowed origin gets cors on terminal responses (post /mcp http/1.1\norigin: http://allowed\ncontent type: application/json\naccept: application/json\nauthorization: bearer ok\nmcp protocol version: 2025 03 26\ncontent length: 0  200 ok)
      Then the authorization decision and visible result match the specified principal scope

    @umcp-9a9465aaa3ec
    Scenario: Async http allowed origin gets cors on terminal responses (POST /mcp HTTP/1.1\nOrigin: http://allowed\nContent Type: application/json\nAccept: application/json\nAuthorization: Bearer ok\nContent Length: nope  400 Bad Request)
      Given a configured MCP client, server and transport state
      When the client performs: async http allowed origin gets cors on terminal responses (post /mcp http/1.1\norigin: http://allowed\ncontent type: application/json\naccept: application/json\nauthorization: bearer ok\ncontent length: nope  400 bad request)
      Then the authorization decision and visible result match the specified principal scope

    @umcp-168e7079151e
    Scenario: Async http allowed origin gets cors on terminal responses (POST /mcp HTTP/1.1\nOrigin: http://allowed\nContent Type: application/json\nAccept: application/json\nAuthorization: Bearer ok\nContent Length: 9  413 Payload Too Large)
      Given a configured MCP client, server and transport state
      When the client performs: async http allowed origin gets cors on terminal responses (post /mcp http/1.1\norigin: http://allowed\ncontent type: application/json\naccept: application/json\nauthorization: bearer ok\ncontent length: 9  413 payload too large)
      Then the authorization decision and visible result match the specified principal scope

    @umcp-178eec2548ef
    Scenario: Async http allowed origin gets cors on json errors and duplicate length and transfer encoding
      Given a configured MCP client, server and transport state
      When the client performs: async http allowed origin gets cors on json errors and duplicate length and transfer encoding
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @umcp-259293ad90bf
    Scenario: Origin validation rejects malformed loopback forms
      Given a configured MCP client, server and transport state
      When the client performs: origin validation rejects malformed loopback forms
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @umcp-c980620186c3
    Scenario: Remote safe prompt errors
      Given a configured MCP client, server and transport state
      When the client performs: remote safe prompt errors
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @umcp-54f33871e7f0
    Scenario: Remote transports hide internal errors
      Given a configured MCP client, server and transport state
      When the client performs: remote transports hide internal errors
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @umcp-6f5a6036dd95
    Scenario: Async http rejects invalid utf8 version and ambiguous host
      Given a configured MCP client, server and transport state
      When the client performs: async http rejects invalid utf8 version and ambiguous host
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @umcp-8c309ca383b3
    Scenario: Async streamable http rejects invalid host and duplicate singleton headers
      Given a configured MCP client, server and transport state
      When the client performs: async streamable http rejects invalid host and duplicate singleton headers
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @umcp-b40142829680
    Scenario: Async streamable http http10 allows missing host and query path
      Given a configured MCP client, server and transport state
      When the client performs: async streamable http http10 allows missing host and query path
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-521dcdedbdce
    Scenario: Async streamable http options only on endpoint and hook failures are 500
      Given a configured MCP client, server and transport state
      When the client performs: async streamable http options only on endpoint and hook failures are 500
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @umcp-580d7856c919
    Scenario: Async sse origin auth media and body rules
      Given a configured MCP client, server and transport state
      When the client performs: async sse origin auth media and body rules
      Then the authorization decision and visible result match the specified principal scope

    @umcp-8965446b01f6
    Scenario: Async sse binds sessions to authenticated principal
      Given a configured MCP client, server and transport state
      When the client performs: async sse binds sessions to authenticated principal
      Then the authorization decision and visible result match the specified principal scope

    @umcp-8f81489be868
    Scenario: Async sse rejects invalid utf8 body
      Given a configured MCP client, server and transport state
      When the client performs: async sse rejects invalid utf8 body
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @umcp-b6480e1d464a
    Scenario: Async sse session cleanup race returns 404
      Given a configured MCP client, server and transport state
      When the client performs: async sse session cleanup race returns 404
      Then the result and persisted state remain deterministic under concurrent execution

    @umcp-94741595cd46
    Scenario: Async sse hook failures and duplicate host are 500 or 400
      Given a configured MCP client, server and transport state
      When the client performs: async sse hook failures and duplicate host are 500 or 400
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @umcp-d750a7160468
    Scenario: Authentication and authorization paths
      Given a configured MCP client, server and transport state
      When the client performs: authentication and authorization paths
      Then the authorization decision and visible result match the specified principal scope

  Rule: Streamable http sessions

    @umcp-0348d33ff9d2
    Scenario: Http 11 reuses connection and explicitly closes (sync)
      Given a configured MCP client, server and transport state
      When the client performs: http 11 reuses connection and explicitly closes (sync)
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-f8a9df7dc32a
    Scenario: Http 11 reuses connection and explicitly closes (async)
      Given a configured MCP client, server and transport state
      When the client performs: http 11 reuses connection and explicitly closes (async)
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-3175fe76c8cc
    Scenario: Http 10 close and keep alive are explicit (sync)
      Given a configured MCP client, server and transport state
      When the client performs: http 10 close and keep alive are explicit (sync)
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-5dd798240c6d
    Scenario: Http 10 close and keep alive are explicit (async)
      Given a configured MCP client, server and transport state
      When the client performs: http 10 close and keep alive are explicit (async)
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-b88301ad99a5
    Scenario: Session binding notifications reconnect and delete (sync)
      Given a configured MCP client, server and transport state
      When the client performs: session binding notifications reconnect and delete (sync)
      Then the response and durable state transition match the specified lifecycle

    @umcp-1782646e1950
    Scenario: Session binding notifications reconnect and delete (async)
      Given a configured MCP client, server and transport state
      When the client performs: session binding notifications reconnect and delete (async)
      Then the response and durable state transition match the specified lifecycle

    @umcp-794d1107467e
    Scenario: Cors exposes session and allows stream headers (sync)
      Given a configured MCP client, server and transport state
      When the client performs: cors exposes session and allows stream headers (sync)
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-1a8e88cab8ba
    Scenario: Cors exposes session and allows stream headers (async)
      Given a configured MCP client, server and transport state
      When the client performs: cors exposes session and allows stream headers (async)
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-1a1060947435
    Scenario: Concurrent session posts are isolated (sync)
      Given a configured MCP client, server and transport state
      When the client performs: concurrent session posts are isolated (sync)
      Then the result and persisted state remain deterministic under concurrent execution

    @umcp-c64e6912db47
    Scenario: Concurrent session posts are isolated (async)
      Given a configured MCP client, server and transport state
      When the client performs: concurrent session posts are isolated (async)
      Then the result and persisted state remain deterministic under concurrent execution

    @umcp-2c548d3ab60d
    Scenario: Idle timeout expiry and duplicate session headers (sync)
      Given a configured MCP client, server and transport state
      When the client performs: idle timeout expiry and duplicate session headers (sync)
      Then cancellation or timeout preserves the specified completion and reconciliation state

    @umcp-9b04872fc9fa
    Scenario: Idle timeout expiry and duplicate session headers (async)
      Given a configured MCP client, server and transport state
      When the client performs: idle timeout expiry and duplicate session headers (async)
      Then cancellation or timeout preserves the specified completion and reconciliation state

    @umcp-bdafdc8f6343
    Scenario: Event stream requires session owner and version (sync)
      Given a configured MCP client, server and transport state
      When the client performs: event stream requires session owner and version (sync)
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-5d33113c880e
    Scenario: Event stream requires session owner and version (async)
      Given a configured MCP client, server and transport state
      When the client performs: event stream requires session owner and version (async)
      Then the observable response, ordering, warnings and resulting state match the specified contract

  Rule: Streamable http sync async

    @umcp-e5ed2b6845ab
    Scenario: Sync tool context from process request
      Given a configured MCP client, server and transport state
      When the client performs: sync tool context from process request
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-9ad23774c0f2
    Scenario: Async tool context from process request
      Given a configured MCP client, server and transport state
      When the client performs: async tool context from process request
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-cf4ba520596c
    Scenario: Async streamable http accepts json and context
      Given a configured MCP client, server and transport state
      When the client performs: async streamable http accepts json and context
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-4bbab1c7c978
    Scenario: Async streamable http explains missing and unsupported protocol versions
      Given a configured MCP client, server and transport state
      When the client performs: async streamable http explains missing and unsupported protocol versions
      Then the observable response, ordering, warnings and resulting state match the specified contract
