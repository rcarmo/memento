Feature: mcp/schema tools and coercion

  These scenarios describe observable behavior independently of its implementation.
  Stable row tags link each scenario to versioned evidence and executable validation data.

  Rule: Annotations

    @umcp-08f4eff712d5
    Scenario: Read prefix marks read only
      Given a configured MCP client, server and transport state
      When the client performs: read prefix marks read only
      Then the authorization decision and visible result match the specified principal scope

    @umcp-99f835143941
    Scenario: Destructive prefix marks destructive
      Given a configured MCP client, server and transport state
      When the client performs: destructive prefix marks destructive
      Then the authorization decision and visible result match the specified principal scope

    @umcp-968dde3b88d9
    Scenario: Open world prefix marks open world
      Given a configured MCP client, server and transport state
      When the client performs: open world prefix marks open world
      Then the authorization decision and visible result match the specified principal scope

    @umcp-c4b5bf0564fb
    Scenario: No recognised prefix yields neutral annotations
      Given a configured MCP client, server and transport state
      When the client performs: no recognised prefix yields neutral annotations
      Then the authorization decision and visible result match the specified principal scope

    @umcp-75ee031900a3
    Scenario: Explicit mcp annotations override inference
      Given a configured MCP client, server and transport state
      When the client performs: explicit mcp annotations override inference
      Then the observable response, ordering, warnings and resulting state match the specified contract

  Rule: Coercion

    @umcp-7e650ebb8906
    Scenario: String to int is coerced
      Given a configured MCP client, server and transport state
      When the client performs: string to int is coerced
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-def2781caea5
    Scenario: String to float is coerced
      Given a configured MCP client, server and transport state
      When the client performs: string to float is coerced
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-0dda614a55cc
    Scenario: String true to bool is coerced
      Given a configured MCP client, server and transport state
      When the client performs: string true to bool is coerced
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-43ccc67a3c96
    Scenario: Native types pass through unchanged
      Given a configured MCP client, server and transport state
      When the client performs: native types pass through unchanged
      Then the observable response, ordering, warnings and resulting state match the specified contract

  Rule: Introspection

    @umcp-1de69fe9cf78
    Scenario: Introspected movie server
      Given a configured MCP client, server and transport state
      When the client performs: introspected movie server
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-a2fff1eb6c8a
    Scenario: Calculator server
      Given a configured MCP client, server and transport state
      When the client performs: calculator server
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-0daefb9d4efb
    Scenario: Basic movie server
      Given a configured MCP client, server and transport state
      When the client performs: basic movie server
      Then the observable response, ordering, warnings and resulting state match the specified contract

  Rule: Schema fallbacks

    @umcp-f16004011a79
    Scenario: Sync schema falls back to signature annotation and string default
      Given a configured MCP client, server and transport state
      When the client performs: sync schema falls back to signature annotation and string default
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-fed3c6620137
    Scenario: Async schema falls back to signature annotation and string default
      Given a configured MCP client, server and transport state
      When the client performs: async schema falls back to signature annotation and string default
      Then the observable response, ordering, warnings and resulting state match the specified contract

  Rule: Schema generation

    @umcp-2736f909764a
    Scenario: Primitive types map to jsonschema
      Given a configured MCP client, server and transport state
      When the client performs: primitive types map to jsonschema
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-d127f00e7982
    Scenario: Required vs optional by default
      Given a configured MCP client, server and transport state
      When the client performs: required vs optional by default
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-55b34e18d68a
    Scenario: Additional properties is false
      Given a configured MCP client, server and transport state
      When the client performs: additional properties is false
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-175bd839a7e5
    Scenario: No args tool gets empty object schema
      Given a configured MCP client, server and transport state
      When the client performs: no args tool gets empty object schema
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-9e04f0c1ce1a
    Scenario: Optional via pep604 union with none
      Given a configured MCP client, server and transport state
      When the client performs: optional via pep604 union with none
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-241bd43df7e5
    Scenario: Union of two concrete types maps to array of types
      Given a configured MCP client, server and transport state
      When the client performs: union of two concrete types maps to array of types
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-fbab508c1a62
    Scenario: Literal becomes enum
      Given a configured MCP client, server and transport state
      When the client performs: literal becomes enum
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-d03e46383738
    Scenario: Typeddict required keys map to object schema
      Given a configured MCP client, server and transport state
      When the client performs: typeddict required keys map to object schema
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-6d6d81cb1090
    Scenario: Typeddict total false has no required keys
      Given a configured MCP client, server and transport state
      When the client performs: typeddict total false has no required keys
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-6e838d6eefb8
    Scenario: List of strings
      Given a configured MCP client, server and transport state
      When the client performs: list of strings
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-64ecadf74c38
    Scenario: Dict of str to int
      Given a configured MCP client, server and transport state
      When the client performs: dict of str to int
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-909c23d4e9e9
    Scenario: Async base generates same shape
      Given a configured MCP client, server and transport state
      When the client performs: async base generates same shape
      Then the observable response, ordering, warnings and resulting state match the specified contract

  Rule: Tools

    @umcp-c24948c22632
    Scenario: Tools list returns full metadata
      Given a configured MCP client, server and transport state
      When the client performs: tools list returns full metadata
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-43deb75697f9
    Scenario: Tools list includes every tool method
      Given a configured MCP client, server and transport state
      When the client performs: tools list includes every tool method
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-94bdccadcf75
    Scenario: Tool args section populates param descriptions
      Given a configured MCP client, server and transport state
      When the client performs: tool args section populates param descriptions
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-0cd5ea99ad49
    Scenario: Call with string return wraps as text content
      Given a configured MCP client, server and transport state
      When the client performs: call with string return wraps as text content
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-3fa23df60de2
    Scenario: Call with dict return serialises to json text
      Given a configured MCP client, server and transport state
      When the client performs: call with dict return serialises to json text
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-7fc61548e318
    Scenario: Call with list return serialises to json text
      Given a configured MCP client, server and transport state
      When the client performs: call with list return serialises to json text
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-989b9a7059b0
    Scenario: Call with scalar returns are stringified
      Given a configured MCP client, server and transport state
      When the client performs: call with scalar returns are stringified
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-031c65d64bbe
    Scenario: Call with none return serialises as json null
      Given a configured MCP client, server and transport state
      When the client performs: call with none return serialises as json null
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-92bea91449db
    Scenario: Call uses default when argument omitted
      Given a configured MCP client, server and transport state
      When the client performs: call uses default when argument omitted
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-1119dfd14779
    Scenario: Call with no args section at all
      Given a configured MCP client, server and transport state
      When the client performs: call with no args section at all
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-de44b3857c8d
    Scenario: Listed tools can all be called round trip
      Given a configured MCP client, server and transport state
      When the client performs: listed tools can all be called round trip
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-19089cb16f2b
    Scenario: Async tool dispatch returns text content
      Given a configured MCP client, server and transport state
      When the client performs: async tool dispatch returns text content
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-8ea8fab9ff49
    Scenario: Async tool with int return
      Given a configured MCP client, server and transport state
      When the client performs: async tool with int return
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-0a2444b2beef
    Scenario: Async base supports sync tool methods
      Given a configured MCP client, server and transport state
      When the client performs: async base supports sync tool methods
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @umcp-6820afec87ce
    Scenario: Async tools can run concurrently
      Given a configured MCP client, server and transport state
      When the client performs: async tools can run concurrently
      Then the result and persisted state remain deterministic under concurrent execution

    @umcp-4504e1b19c32
    Scenario: Async initialize includes tools capability
      Given a configured MCP client, server and transport state
      When the client performs: async initialize includes tools capability
      Then the observable response, ordering, warnings and resulting state match the specified contract
