Feature: access/managed access

  These scenarios describe observable behavior independently of its implementation.
  Stable row tags link each scenario to versioned evidence and executable validation data.

  Rule: Access

    @py-1380ae25e6c2
    Scenario: Bootstrap renames initial admin and authenticates
      Given the controlled domain state and principal described by this behavior
      When the actor performs: bootstrap renames initial admin and authenticates
      Then the authorization decision and visible result match the specified principal scope

    @py-7817571ef4e3
    Scenario: Create returns one time token and validates scope
      Given the controlled domain state and principal described by this behavior
      When the actor performs: create returns one time token and validates scope
      Then the authorization decision and visible result match the specified principal scope

    @py-78754534ea44
    Scenario: Lifecycle and last admin guard
      Given the controlled domain state and principal described by this behavior
      When the actor performs: lifecycle and last admin guard
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @py-1b4338fe4a5a
    Scenario: Managed principal policy inherits protected namespaces
      Given the controlled domain state and principal described by this behavior
      When the actor performs: managed principal policy inherits protected namespaces
      Then the authorization decision and visible result match the specified principal scope

    @py-81566e51cdcb
    Scenario: Master key rotation preserves credentials
      Given the controlled domain state and principal described by this behavior
      When the actor performs: master key rotation preserves credentials
      Then the observable response, ordering, warnings and resulting state match the specified contract

  Rule: Admin http

    @py-7cbcbcbe6d53
    Scenario: Admin create returns one time credential
      Given the controlled domain state and principal described by this behavior
      When the actor performs: admin create returns one time credential
      Then the response and durable state transition match the specified lifecycle

    @py-980566bf5992
    Scenario: Admin page and api auth
      Given the controlled domain state and principal described by this behavior
      When the actor performs: admin page and api auth
      Then the bounded result and continuation state match the specified contract
