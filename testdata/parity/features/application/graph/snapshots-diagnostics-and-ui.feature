Feature: graph/snapshots diagnostics and ui

  These scenarios describe observable behavior independently of its implementation.
  Stable row tags link each scenario to versioned evidence and executable validation data.

  Rule: Graph debug

    @py-6d597626a314
    Scenario: Disabled graph routes are indistinguishable 404s
      Given the controlled domain state and principal described by this behavior
      When the actor performs: disabled graph routes are indistinguishable 404s
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-504e22c85bef
    Scenario: Enabled graph boundary serves ui status and assets
      Given the controlled domain state and principal described by this behavior
      When the actor performs: enabled graph boundary serves ui status and assets
      Then the bounded result and continuation state match the specified contract

    @py-cedb7964bc62
    Scenario: Graph api decodes url encoded ids
      Given the controlled domain state and principal described by this behavior
      When the actor performs: graph api decodes url encoded ids
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-9d94c261b69d
    Scenario: Graph boundary rejects methods and bodies without touching mcp
      Given the controlled domain state and principal described by this behavior
      When the actor performs: graph boundary rejects methods and bodies without touching mcp
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @py-453c4545197d
    Scenario: Graph principal simulation exposes safe metadata and scopes requests
      Given the controlled domain state and principal described by this behavior
      When the actor performs: graph principal simulation exposes safe metadata and scopes requests
      Then the authorization decision and visible result match the specified principal scope

    @py-a5d86f41895c
    Scenario: Graph route prefix is strict and configurable
      Given the controlled domain state and principal described by this behavior
      When the actor performs: graph route prefix is strict and configurable
      Then the authorization decision and visible result match the specified principal scope

    @py-15c8abc6bce1
    Scenario: Graph search post returns results
      Given the controlled domain state and principal described by this behavior
      When the actor performs: graph search post returns results
      Then the observable response, ordering, warnings and resulting state match the specified contract

  Rule: Graph diagnostics

    @py-d80c013a66b9
    Scenario: Diagnostics are explainable and keep semantics derived
      Given the controlled domain state and principal described by this behavior
      When the actor performs: diagnostics are explainable and keep semantics derived
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-acd72734954f
    Scenario: Namespace outlier uses only explicit edges
      Given the controlled domain state and principal described by this behavior
      When the actor performs: namespace outlier uses only explicit edges
      Then the observable response, ordering, warnings and resulting state match the specified contract

  Rule: Graph export

    @py-4550a7b1b9bf
    Scenario: Json and svg exports are bounded and safe
      Given the controlled domain state and principal described by this behavior
      When the actor performs: json and svg exports are bounded and safe
      Then the bounded result and continuation state match the specified contract

  Rule: Graph layout

    @py-042337a8f00c
    Scenario: Aggregate layout groups sparse namespace isolates
      Given the controlled domain state and principal described by this behavior
      When the actor performs: aggregate layout groups sparse namespace isolates
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-79fc1de84e1e
    Scenario: Aggregate layout handles isolates and cluster bound
      Given the controlled domain state and principal described by this behavior
      When the actor performs: aggregate layout handles isolates and cluster bound
      Then the bounded result and continuation state match the specified contract

    @py-4df4552c7f20
    Scenario: Aggregate layout is deterministic and input order independent
      Given the controlled domain state and principal described by this behavior
      When the actor performs: aggregate layout is deterministic and input order independent
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-ff450127bc69
    Scenario: Aggregate semantic edges retain similarity
      Given the controlled domain state and principal described by this behavior
      When the actor performs: aggregate semantic edges retain similarity
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-ee3f6b20c9de
    Scenario: All shared forces have bounded relationships and do not affect canonical layout
      Given the controlled domain state and principal described by this behavior
      When the actor performs: all shared forces have bounded relationships and do not affect canonical layout
      Then the bounded result and continuation state match the specified contract

    @py-3a715df4347a
    Scenario: Overlay edges connect adjacent members with shared tags
      Given the controlled domain state and principal described by this behavior
      When the actor performs: overlay edges connect adjacent members with shared tags
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-843036da6e5a
    Scenario: Scale fixtures are bounded
      Given the controlled domain state and principal described by this behavior
      When the actor performs: scale fixtures are bounded
      Then the bounded result and continuation state match the specified contract

    @py-0f69b2bde618
    Scenario: Sparse overview detection requires many orphaned nodes
      Given the controlled domain state and principal described by this behavior
      When the actor performs: sparse overview detection requires many orphaned nodes
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-6d0b1ca67b67
    Scenario: Trash is named and reserved when clusters overflow
      Given the controlled domain state and principal described by this behavior
      When the actor performs: trash is named and reserved when clusters overflow
      Then the response and durable state transition match the specified lifecycle

  Rule: Graph refresh

    @py-96dbdf27b222
    Scenario: Dead worker is unavailable and enqueue rejected
      Given the controlled domain state and principal described by this behavior
      When the actor performs: dead worker is unavailable and enqueue rejected
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @py-5fcd3772d35e
    Scenario: Refresh rejects unknown bounds and unavailable worker
      Given the controlled domain state and principal described by this behavior
      When the actor performs: refresh rejects unknown bounds and unavailable worker
      Then the request is rejected at the specified boundary and prohibited state is unchanged

    @py-138c7f2eca26
    Scenario: Selected visible and confirmed full refresh
      Given the controlled domain state and principal described by this behavior
      When the actor performs: selected visible and confirmed full refresh
      Then the authorization decision and visible result match the specified principal scope

  Rule: Graph snapshot

    @py-1b3b638c29fb
    Scenario: Detail and neighbourhood are bounded and revision aware
      Given the controlled domain state and principal described by this behavior
      When the actor performs: detail and neighbourhood are bounded and revision aware
      Then the bounded result and continuation state match the specified contract

    @py-5de1ddab867c
    Scenario: Graph search uses fts and returns bounded metadata
      Given the controlled domain state and principal described by this behavior
      When the actor performs: graph search uses fts and returns bounded metadata
      Then the bounded result and continuation state match the specified contract

    @py-73b6b14f4889
    Scenario: Overview adds bounded semantic edges without vectors
      Given the controlled domain state and principal described by this behavior
      When the actor performs: overview adds bounded semantic edges without vectors
      Then the bounded result and continuation state match the specified contract

    @py-5aa00ead5437
    Scenario: Overview is bounded deterministic and omits vectors
      Given the controlled domain state and principal described by this behavior
      When the actor performs: overview is bounded deterministic and omits vectors
      Then the bounded result and continuation state match the specified contract

    @py-62ab0aa7f402
    Scenario: Simulated prefix scope hides nodes search links and metrics
      Given the controlled domain state and principal described by this behavior
      When the actor performs: simulated prefix scope hides nodes search links and metrics
      Then the authorization decision and visible result match the specified principal scope

    @py-46746714b87a
    Scenario: Snapshot reports stale revisions
      Given the controlled domain state and principal described by this behavior
      When the actor performs: snapshot reports stale revisions
      Then the observable response, ordering, warnings and resulting state match the specified contract

  Rule: Graph vendor

    @py-07f1211e94bc
    Scenario: Graph application assets are available as package resources
      Given the controlled domain state and principal described by this behavior
      When the actor performs: graph application assets are available as package resources
      Then the observable response, ordering, warnings and resulting state match the specified contract

    @py-a7ef4dd27ab2
    Scenario: Vendored graph modules match manifest and ship licences
      Given the controlled domain state and principal described by this behavior
      When the actor performs: vendored graph modules match manifest and ship licences
      Then the observable response, ordering, warnings and resulting state match the specified contract
