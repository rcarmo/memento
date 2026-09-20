Feature: models/needle and model transport

  These scenarios describe observable behavior independently of its implementation.
  Stable row tags link each scenario to versioned evidence and executable validation data.

  Rule: Model transport

    @py-153148b58aa8
    Scenario: Model response is bounded
      Given the controlled domain state and principal described by this behavior
      When the actor performs: model response is bounded
      Then the bounded result and continuation state match the specified contract

  Rule: Needle corpus

    @py-9d8385eacc14
    Scenario: Router v2 generator matches vendored corpus manifest
      Given the controlled domain state and principal described by this behavior
      When the actor performs: router v2 generator matches vendored corpus manifest
      Then the observable response, ordering, warnings and resulting state match the specified contract

  Rule: Needle ffi

    @py-1b3b0de5736b
    Scenario: Needle router config defaults are disabled with default paths
      Given the controlled domain state and principal described by this behavior
      When the actor performs: needle router config defaults are disabled with default paths
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-fe2dbf04711e
    Scenario: Python ctypes wrapper router lifecycle generate cancel and errors
      Given the controlled domain state and principal described by this behavior
      When the actor performs: python ctypes wrapper router lifecycle generate cancel and errors
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @py-b7e9d7fada49
    Scenario: Real ffi router output parses to one action
      Given the controlled domain state and principal described by this behavior
      When the actor performs: real ffi router output parses to one action
      Then the observable response, ordering, warnings and resulting state match the specified contract

  Rule: Needle router generator

    @py-fc8d34f2c437
    Scenario: Generated answers validate against router adapter
      Given the controlled domain state and principal described by this behavior
      When the actor performs: generated answers validate against router adapter
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-90c9dcc01c34
    Scenario: Generator uses valid historical router field enums
      Given the controlled domain state and principal described by this behavior
      When the actor performs: generator uses valid historical router field enums
      Then the observable response, ordering, warnings and resulting state match the specified contract
